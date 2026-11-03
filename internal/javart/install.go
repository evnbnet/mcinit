package javart

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/download"
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

// install delegates to the platform, which knows its own package manager and
// package format, and turns a pile of failed attempts into one readable error.
func install(ctx context.Context, major int) (Runtime, error) {
	attempts := platformInstall(ctx, major)

	if rt, ok := Detect(major); ok {
		return rt, nil
	}
	if len(attempts) == 0 {
		attempts = []string{"the install reported success but no java turned up"}
	}
	return Runtime{}, fmt.Errorf("could not install Java %d\n      %s",
		major, strings.Join(attempts, "\n      "))
}

var adoptiumAPI = "https://api.adoptium.net/v3"

// adoptiumFile is one downloadable artefact: an installer (.msi, .pkg) or an
// archive (.zip, .tar.gz). Both carry a SHA-256, so either can be verified.
type adoptiumFile struct {
	Name     string `json:"name"`
	Link     string `json:"link"`
	Checksum string `json:"checksum"`
	Size     int64  `json:"size"`
}

func (f adoptiumFile) usable() bool { return f.Link != "" && f.Checksum != "" }

type adoptiumAsset struct {
	Binary struct {
		// Installer is the platform's native installer, which generally wants
		// administrator rights.
		Installer adoptiumFile `json:"installer"`
		// Package is the plain archive, which can be unpacked anywhere — the
		// basis for installing without a password.
		Package adoptiumFile `json:"package"`
	} `json:"binary"`
	ReleaseName string `json:"release_name"`
}

// pick selects which of an asset's two downloads a platform wants.
type pick func(adoptiumAsset) adoptiumFile

func pickInstaller(a adoptiumAsset) adoptiumFile { return a.Binary.Installer }
func pickArchive(a adoptiumAsset) adoptiumFile   { return a.Binary.Package }

// findAsset asks Adoptium for a build of Java major for this machine, and
// returns the download the caller asked for.
//
// A JRE is tried before a JDK: it is roughly half the size and a server needs
// nothing more. Not every feature version publishes a JRE, so the JDK is the
// fallback.
func findAsset(ctx context.Context, major int, want pick) (adoptiumFile, string, error) {
	return findAssetFor(ctx, major, adoptiumOS(), adoptiumArch(), want)
}

// findAssetFor is findAsset with the platform spelled out, so the packages for
// every target can be checked from any machine.
func findAssetFor(ctx context.Context, major int, goos, arch string, want pick) (adoptiumFile, string, error) {
	for _, imageType := range []string{"jre", "jdk"} {
		url := fmt.Sprintf("%s/assets/latest/%d/hotspot?os=%s&architecture=%s&image_type=%s",
			adoptiumAPI, major, goos, arch, imageType)

		var assets []adoptiumAsset
		if err := getJSON(ctx, url, &assets); err != nil {
			return adoptiumFile{}, "", err
		}
		for _, asset := range assets {
			if file := want(asset); file.usable() {
				return file, asset.ReleaseName, nil
			}
		}
	}
	return adoptiumFile{}, "", fmt.Errorf(
		"Adoptium publishes no suitable Java %d package for %s/%s", major, goos, arch)
}

// fetchAdoptium downloads and verifies one Adoptium file.
func fetchAdoptium(ctx context.Context, file adoptiumFile, dest string, major int) error {
	return download.Fetch(ctx, download.Request{
		URL:      file.Link,
		Dest:     dest,
		Size:     file.Size,
		Algo:     download.AlgoSHA256,
		Checksum: file.Checksum,
		Label:    fmt.Sprintf("Temurin %d", major),
	})
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
