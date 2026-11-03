//go:build windows

package javart

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanb/mcinit/internal/ui"
)

const wingetTimeout = 15 * time.Minute

func platformInstall(ctx context.Context, major int) []string {
	var attempts []string

	if _, err := exec.LookPath("winget"); err == nil {

		for _, pkg := range []string{
			fmt.Sprintf("EclipseAdoptium.Temurin.%d.JRE", major),
			fmt.Sprintf("EclipseAdoptium.Temurin.%d.JDK", major),
		} {
			err := ui.Spin(fmt.Sprintf("Installing %s with winget", pkg), func() error {
				return wingetInstall(ctx, pkg)
			})
			if err != nil {
				attempts = append(attempts, fmt.Sprintf("winget %s: %v", pkg, err))
				continue
			}
			if _, ok := Detect(major); ok {
				return nil
			}
			attempts = append(attempts, fmt.Sprintf("winget %s: installed but no java.exe turned up", pkg))
		}
	} else {
		attempts = append(attempts, "winget is not available")
	}

	ui.Info("Trying the official Adoptium installer")
	if err := msiInstall(ctx, major); err != nil {
		attempts = append(attempts, fmt.Sprintf("MSI installer: %v", err))
	} else if _, ok := Detect(major); ok {
		return nil
	} else {
		attempts = append(attempts, "MSI installer: finished but no java.exe turned up")
	}
	return attempts
}

func wingetInstall(ctx context.Context, pkg string) error {
	ctx, cancel := context.WithTimeout(ctx, wingetTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "winget", "install",
		"--id", pkg,
		"--exact",
		"--source", "winget",
		"--silent",
		"--accept-package-agreements",
		"--accept-source-agreements",
		"--disable-interactivity",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s (%s)", err, firstLine(strings.TrimSpace(string(out))))
	}
	return nil
}

func msiInstall(ctx context.Context, major int) error {
	file, _, err := findAsset(ctx, major, pickInstaller)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "mcinit-java-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	msi := filepath.Join(dir, file.Name)
	if err := fetchAdoptium(ctx, file, msi, major); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "msiexec", "/i", msi, "/qn", "/norestart",
		"ADDLOCAL=FeatureMain,FeatureEnvironment,FeatureJavaHome")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("msiexec failed: %s %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
