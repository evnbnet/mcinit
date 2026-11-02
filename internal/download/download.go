// Package download fetches a file, verifies it against a published digest and
// only then puts it in place. Nothing half-downloaded ever reaches the final
// path, so a dropped connection cannot leave behind a jar that looks installed
// but is truncated.
package download

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/ui"
)

// Supported digest algorithms. Paper and Adoptium publish SHA-256; Purpur
// publishes MD5 and nothing else.
const (
	AlgoSHA256 = "sha256"
	AlgoMD5    = "md5"
)

// partSuffix names the in-progress file. It sits next to the destination so the
// final step is a rename on the same volume, which is atomic.
const partSuffix = ".part"

// Request describes one file to fetch.
type Request struct {
	URL  string
	Dest string // final absolute path
	Size int64  // expected bytes; 0 falls back to Content-Length

	// Algo and Checksum come from the provider's API. Both must be set:
	// an unverified download is not worth the risk of a corrupted install.
	Algo     string
	Checksum string

	// Label is what the progress bar shows. Defaults to the destination's
	// base name.
	Label string
}

// client has no overall timeout — a large jar on a slow line is normal. The
// context passed to Fetch is what cancels it.
var client = &http.Client{}

// Fetch downloads, verifies and installs the file. On any failure the partial
// download is removed and the destination is left untouched.
func Fetch(ctx context.Context, req Request) error {
	hasher, err := hasherFor(req.Algo)
	if err != nil {
		return err
	}
	if req.Checksum == "" {
		return fmt.Errorf("no published checksum for %s", req.URL)
	}
	if err := os.MkdirAll(filepath.Dir(req.Dest), 0o755); err != nil {
		return err
	}

	label := req.Label
	if label == "" {
		label = filepath.Base(req.Dest)
	}

	resp, err := get(ctx, req.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	total := req.Size
	if total <= 0 {
		total = resp.ContentLength
	}

	part := req.Dest + partSuffix
	file, err := os.Create(part)
	if err != nil {
		return err
	}
	// Any early return below must not leave the partial file behind.
	installed := false
	defer func() {
		if !installed {
			file.Close()
			os.Remove(part)
		}
	}()

	bar := ui.NewProgress(label, total)
	written, err := io.Copy(io.MultiWriter(file, hasher, bar), resp.Body)
	bar.Finish()
	if err != nil {
		return fmt.Errorf("the download of %s was cut short: %w", label, err)
	}
	if total > 0 && written != total {
		return fmt.Errorf("%s arrived incomplete (%d of %d bytes)", label, written, total)
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	want := strings.ToLower(strings.TrimSpace(req.Checksum))
	if got != want {
		return fmt.Errorf("el %s de %s no coincide\n      esperado %s\n      obtenido %s",
			req.Algo, label, want, got)
	}

	if err := file.Close(); err != nil {
		return err
	}
	// Windows will not rename onto an existing file.
	os.Remove(req.Dest)
	if err := os.Rename(part, req.Dest); err != nil {
		return err
	}

	installed = true
	return nil
}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", buildinfo.UserAgent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not download %s: %w", url, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s replied %s", url, resp.Status)
	}
	return resp, nil
}

func hasherFor(algo string) (hash.Hash, error) {
	switch strings.ToLower(algo) {
	case AlgoSHA256:
		return sha256.New(), nil
	case AlgoMD5:
		// Weak against deliberate collisions, but it is the only digest
		// Purpur publishes, and here it does its job: catching corruption.
		return md5.New(), nil
	}
	return nil, fmt.Errorf("unsupported checksum algorithm %q", algo)
}
