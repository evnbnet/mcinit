// Package provider resolves a Minecraft version into a downloadable server
// jar. Each supported distribution has its own API with its own shape, and the
// differences that matter — notably which checksum algorithm is published —
// are normalised into Artifact.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/mcver"
)

// Checksum algorithms. Paper publishes SHA-256; Purpur publishes only MD5.
// MD5 is weak against deliberate collisions, but here it is used for what the
// upstream project offers — catching a truncated or corrupted download — so we
// verify with whatever the API gives us rather than skipping the check.
const (
	SHA256 = "sha256"
	MD5    = "md5"
)

// Artifact is a specific server jar, ready to download and verify.
type Artifact struct {
	Filename string
	URL      string
	Size     int64  // 0 when the API does not report it
	Build    string // provider build number, shown in the summary
	Channel  string // "STABLE", "EXPERIMENTAL", or "" when unreported
	Algo     string // SHA256 or MD5
	Checksum string // lowercase hex digest
}

// Experimental reports whether the provider flagged this build as not stable.
func (a Artifact) Experimental() bool {
	return a.Channel != "" && a.Channel != "STABLE"
}

// Provider is one server distribution's download API.
type Provider interface {
	// Software identifies the distribution.
	Software() config.Software
	// Versions lists the stable Minecraft versions on offer, newest first.
	Versions(ctx context.Context) ([]mcver.Version, error)
	// Resolve returns the newest build for a version. The version must be one
	// of the strings returned by Versions.
	Resolve(ctx context.Context, version string) (Artifact, error)
}

// For returns the provider for a distribution.
func For(s config.Software) (Provider, error) {
	switch s {
	case config.Paper:
		return paper{}, nil
	case config.Purpur:
		return purpur{}, nil
	}
	return nil, fmt.Errorf("unknown server software %q", s)
}

// LatestStable returns the newest stable version a provider offers.
func LatestStable(ctx context.Context, p Provider) (string, error) {
	vs, err := p.Versions(ctx)
	if err != nil {
		return "", err
	}
	if len(vs) == 0 {
		return "", fmt.Errorf("%s has no stable versions available", p.Software().Display())
	}
	return vs[0].Raw, nil
}

// errNotFound marks a 404, which for these APIs always means "that version or
// build does not exist" rather than a real failure. Callers turn it into
// something the user can act on.
var errNotFound = errors.New("not found")

// apiClient is used for the metadata calls only; jar downloads get their own
// client with no overall timeout.
var apiClient = &http.Client{Timeout: 20 * time.Second}

// getJSON fetches a URL and decodes it, always sending the User-Agent that
// PaperMC requires.
func getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", buildinfo.UserAgent())
	req.Header.Set("Accept", "application/json")

	resp, err := apiClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not reach %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return errNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s replied %s", url, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("unexpected response from %s: %w", url, err)
	}
	return nil
}

// sortNewestFirst keeps only parseable stable versions and orders them newest
// first, which is the order every menu wants.
func sortNewestFirst(raw []string) []mcver.Version {
	var out []mcver.Version
	for _, s := range raw {
		v, err := mcver.Parse(s)
		if err != nil || !v.Stable() {
			continue
		}
		out = append(out, v)
	}
	mcver.Sort(out)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}
