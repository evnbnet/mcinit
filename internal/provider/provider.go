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

const (
	SHA256 = "sha256"
	MD5    = "md5"
)

type Artifact struct {
	Filename string
	URL      string
	Size     int64
	Build    string
	Channel  string
	Algo     string
	Checksum string
}

func (a Artifact) Experimental() bool {
	return a.Channel != "" && a.Channel != "STABLE"
}

type Provider interface {
	Software() config.Software

	Versions(ctx context.Context) ([]mcver.Version, error)

	Resolve(ctx context.Context, version string) (Artifact, error)
}

func For(s config.Software) (Provider, error) {
	switch s {
	case config.Paper:
		return paper{}, nil
	case config.Purpur:
		return purpur{}, nil
	}
	return nil, fmt.Errorf("unknown server software %q", s)
}

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

var errNotFound = errors.New("not found")

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
