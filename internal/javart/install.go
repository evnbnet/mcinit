package javart

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/download"
	"github.com/evanb/mcinit/internal/ui"
)

// Ensure returns a runtime that can run a server needing Java minMajor,
// installing Temurin first if nothing suitable is present.
//
// The returned path is always absolute and always re-detected after an install,
// because a package manager cannot add anything to the PATH of a process that
// is already running.
func Ensure(ctx context.Context, minMajor int) (Runtime, bool, error) {
	if rt, ok := Detect(minMajor); ok {
		return rt, false, nil
	}

	rt, err := install(ctx, minMajor)
	if err != nil {
		return Runtime{}, false, err
	}
	return rt, true, nil
}

// install tries winget first because it is already on every supported Windows
// version, then falls back to Adoptium's own MSI.
func install(ctx context.Context, major int) (Runtime, error) {
	var attempts []string

	if _, err := exec.LookPath("winget"); err == nil {
		// JRE first: it is roughly half the size of the JDK and a server needs
		// nothing more. Not every feature version ships a JRE package, so the
		// JDK is the second try.
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
			if rt, ok := Detect(major); ok {
				return rt, nil
			}
			attempts = append(attempts, fmt.Sprintf("winget %s: installed but no java.exe turned up", pkg))
		}
	} else {
		attempts = append(attempts, "winget is not available")
	}

	// The MSI path draws its own download bar, so it gets no spinner around it.
	ui.Info("Trying the official Adoptium installer")
	if err := msiInstall(ctx, major); err != nil {
		attempts = append(attempts, fmt.Sprintf("MSI installer: %v", err))
	} else if rt, ok := Detect(major); ok {
		return rt, nil
	} else {
		attempts = append(attempts, "MSI installer: finished but no java.exe turned up")
	}

	return Runtime{}, fmt.Errorf("no pude instalar Java %d\n      %s",
		major, strings.Join(attempts, "\n      "))
}

// wingetTimeout bounds a single winget install. Downloading a JRE over a slow
// line is normal; hanging forever is not.
const wingetTimeout = 15 * time.Minute

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

// adoptiumAPI serves the official binaries, including each installer's SHA-256,
// so the MSI fallback is verified the same way the server jar is.
var adoptiumAPI = "https://api.adoptium.net/v3"

type adoptiumAsset struct {
	Binary struct {
		Installer struct {
			Name     string `json:"name"`
			Link     string `json:"link"`
			Checksum string `json:"checksum"`
			Size     int64  `json:"size"`
		} `json:"installer"`
	} `json:"binary"`
	ReleaseName string `json:"release_name"`
}

func msiInstall(ctx context.Context, major int) error {
	asset, err := findInstaller(ctx, major)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "mcinit-java-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	msi := filepath.Join(dir, asset.Binary.Installer.Name)
	err = download.Fetch(ctx, download.Request{
		URL:      asset.Binary.Installer.Link,
		Dest:     msi,
		Size:     asset.Binary.Installer.Size,
		Algo:     download.AlgoSHA256,
		Checksum: asset.Binary.Installer.Checksum,
		Label:    fmt.Sprintf("Temurin %d", major),
	})
	if err != nil {
		return err
	}

	// /qn is a fully silent install, which needs an elevated process to write
	// into Program Files. FeatureEnvironment and FeatureJavaHome put java on
	// the PATH for future terminals; mcinit itself does not depend on that,
	// since the generated start script uses the absolute path.
	cmd := exec.CommandContext(ctx, "msiexec", "/i", msi, "/qn", "/norestart",
		"ADDLOCAL=FeatureMain,FeatureEnvironment,FeatureJavaHome")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("msiexec failed: %s %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func findInstaller(ctx context.Context, major int) (adoptiumAsset, error) {
	// A JRE is enough to run a server; fall back to the JDK for feature
	// versions that do not publish one.
	for _, imageType := range []string{"jre", "jdk"} {
		url := fmt.Sprintf("%s/assets/latest/%d/hotspot?os=windows&architecture=%s&image_type=%s",
			adoptiumAPI, major, adoptiumArch(), imageType)

		var assets []adoptiumAsset
		if err := getJSON(ctx, url, &assets); err != nil {
			return adoptiumAsset{}, err
		}
		for _, a := range assets {
			if a.Binary.Installer.Link != "" && a.Binary.Installer.Checksum != "" {
				return a, nil
			}
		}
	}
	return adoptiumAsset{}, fmt.Errorf("Adoptium publishes no Java %d installer for windows/%s", major, adoptiumArch())
}

// adoptiumArch maps Go's architecture names onto Adoptium's.
func adoptiumArch() string {
	switch runtime.GOARCH {
	case "arm64":
		return "aarch64"
	case "386":
		return "x86"
	default:
		return "x64"
	}
}

var apiClient = &http.Client{Timeout: 20 * time.Second}

func getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach Adoptium: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Adoptium replied %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
