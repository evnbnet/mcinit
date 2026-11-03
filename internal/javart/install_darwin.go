//go:build darwin

package javart

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanb/mcinit/internal/ui"
)

// brewTimeout bounds a single Homebrew install. Downloading a JRE over a slow
// line is normal; hanging forever is not.
const brewTimeout = 15 * time.Minute

// platformInstall installs Temurin, returning a note for each attempt that did
// not work. It stops early once Detect can see a usable runtime.
//
// Homebrew is tried first because somebody who has it expects their packages
// to be managed by it. The archive is the fallback that always works: it needs
// neither Homebrew nor an administrator password, which keeps installing Java
// on a Mac as unceremonious as it is on Windows.
func platformInstall(ctx context.Context, major int) []string {
	var attempts []string

	if _, err := exec.LookPath("brew"); err == nil {
		cask := fmt.Sprintf("temurin@%d", major)
		err := ui.Spin(fmt.Sprintf("Installing %s with Homebrew", cask), func() error {
			return brewInstall(ctx, cask)
		})
		if err != nil {
			attempts = append(attempts, fmt.Sprintf("brew %s: %v", cask, err))
		} else if _, ok := Detect(major); ok {
			return nil
		} else {
			attempts = append(attempts, fmt.Sprintf("brew %s: installed but no java turned up", cask))
		}
	} else {
		attempts = append(attempts, "Homebrew is not installed")
	}

	// The archive path draws its own download bar, so it gets no spinner.
	ui.Info("Downloading Temurin from Adoptium")
	if err := archiveInstall(ctx, major); err != nil {
		attempts = append(attempts, fmt.Sprintf("Adoptium archive: %v", err))
	} else if _, ok := Detect(major); ok {
		return nil
	} else {
		attempts = append(attempts, "Adoptium archive: unpacked but no java turned up")
	}
	return attempts
}

func brewInstall(ctx context.Context, cask string) error {
	ctx, cancel := context.WithTimeout(ctx, brewTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "brew", "install", "--cask", cask)
	// Homebrew prompts for confirmation and for a password on some casks;
	// neither can be answered from here.
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "NONINTERACTIVE=1")

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s (%s)", err, firstLine(strings.TrimSpace(string(out))))
	}
	return nil
}

// archiveInstall downloads Temurin's .tar.gz and unpacks it into the user's own
// Java directory, which needs no password and leaves the system untouched.
func archiveInstall(ctx context.Context, major int) error {
	file, release, err := findAsset(ctx, major, pickArchive)
	if err != nil {
		return err
	}

	temp, err := os.MkdirTemp("", "mcinit-java-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)

	archive := filepath.Join(temp, file.Name)
	if err := fetchAdoptium(ctx, file, archive, major); err != nil {
		return err
	}

	target := userJavaDir()
	if target == "" {
		return fmt.Errorf("could not work out your home directory")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	// Unpack beside the final location first, so a half-extracted runtime is
	// never visible to the detector as if it were installed.
	staging, err := os.MkdirTemp(target, ".mcinit-unpack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	var root string
	err = ui.Spin(fmt.Sprintf("Unpacking %s", release), func() error {
		root, err = extractTarGz(archive, staging)
		return err
	})
	if err != nil {
		return err
	}
	if root == "" {
		return fmt.Errorf("%s did not contain a runtime", file.Name)
	}

	final := filepath.Join(target, root)
	os.RemoveAll(final) // a previous half-install, or the same version again
	if err := os.Rename(filepath.Join(staging, root), final); err != nil {
		return fmt.Errorf("could not move the runtime into %s: %w", target, err)
	}
	return nil
}

// extractTarGz unpacks an archive into dir and returns the single top-level
// directory it contained, which is the runtime's bundle.
func extractTarGz(archivePath, dir string) (string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("%s is not a gzip archive: %w", filepath.Base(archivePath), err)
	}
	defer gz.Close()

	root := ""
	reader := tar.NewReader(gz)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("could not read %s: %w", filepath.Base(archivePath), err)
		}

		dest, err := safeJoin(dir, header.Name)
		if err != nil {
			return "", err
		}
		if top := topLevel(header.Name); top != "" && root == "" {
			root = top
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return "", err
			}
			// The mode from the archive matters here: java itself has to come
			// out executable.
			if err := writeFile(reader, dest, header.FileInfo().Mode()); err != nil {
				return "", err
			}
		case tar.TypeSymlink:
			// A JDK bundle is full of symlinks. A link target that escapes the
			// destination is refused for the same reason a path is.
			if _, err := safeJoin(dir, path.Join(path.Dir(header.Name), header.Linkname)); err != nil {
				return "", fmt.Errorf("the archive contains a symlink pointing outside it (%s)", header.Name)
			}
			os.MkdirAll(filepath.Dir(dest), 0o755)
			os.Remove(dest)
			if err := os.Symlink(header.Linkname, dest); err != nil {
				return "", err
			}
		}
	}
	return root, nil
}

func writeFile(src io.Reader, dest string, mode os.FileMode) error {
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, src); err != nil {
		return err
	}
	return out.Close()
}

// topLevel returns the first path segment of an archive entry.
func topLevel(name string) string {
	cleaned := path.Clean(strings.ReplaceAll(name, `\`, "/"))
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	top, _, _ := strings.Cut(strings.TrimPrefix(cleaned, "/"), "/")
	return top
}

// safeJoin resolves an archive entry name against a destination directory and
// refuses anything that would escape it.
//
// This is the same class of bug as zip slip: an entry named "../../evil" would
// otherwise let a downloaded archive write anywhere on the machine.
func safeJoin(base, name string) (string, error) {
	cleaned := path.Clean(strings.ReplaceAll(name, `\`, "/"))
	if path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("the archive contains an absolute path (%s); refusing to unpack it", name)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("the archive tries to write outside its folder (%s); refusing to unpack it", name)
	}
	return filepath.Join(base, filepath.FromSlash(cleaned)), nil
}
