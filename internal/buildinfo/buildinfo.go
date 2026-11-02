// Package buildinfo carries the binary's identity. Version is overwritten at
// link time by goreleaser.
package buildinfo

import "fmt"

// Version is the release tag, injected with
// -ldflags "-X github.com/evanb/mcinit/internal/buildinfo.Version=v0.1.0".
var Version = "dev"

// Repo is where the project lives; it appears in the User-Agent so that API
// operators can reach us.
const Repo = "https://github.com/evanb/mcinit"

// UserAgent identifies mcinit to the download APIs. PaperMC rejects requests
// that do not carry a descriptive one, so this is not optional.
func UserAgent() string {
	return fmt.Sprintf("mcinit/%s (+%s)", Version, Repo)
}
