package backup

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/evanb/mcinit/internal/server"
	"github.com/evanb/mcinit/internal/ui"
)

type Archive struct {
	Path     string
	Name     string
	Size     int64
	Created  time.Time
	Manifest Manifest

	HasManifest bool
}

func (a Archive) Label() string {
	stamp := a.Created.Format("2006-01-02 15:04")
	label := fmt.Sprintf("%s   %s", stamp, HumanSize(a.Size))
	if a.Manifest.Reason == ReasonPreRestore {
		label += "   (saved before a restore)"
	}
	return label
}

func List(info server.Info) ([]Archive, error) {
	dir := filepath.Join(info.Dir, BackupsDir)
	files, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", dir, err)
	}

	var archives []Archive
	for _, f := range files {
		if f.IsDir() || !strings.EqualFold(filepath.Ext(f.Name()), ".zip") {
			continue
		}
		path := filepath.Join(dir, f.Name())
		st, err := f.Info()
		if err != nil {
			continue
		}

		archive := Archive{
			Path:    path,
			Name:    strings.TrimSuffix(f.Name(), filepath.Ext(f.Name())),
			Size:    st.Size(),
			Created: st.ModTime(),
		}

		if m, err := readManifest(path); err == nil {
			archive.Manifest = m
			archive.HasManifest = true
			if !m.Created.IsZero() {
				archive.Created = m.Created
			}
		}
		archives = append(archives, archive)
	}

	sort.Slice(archives, func(i, j int) bool {
		return archives[i].Created.After(archives[j].Created)
	})
	return archives, nil
}

func readManifest(path string) (Manifest, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Manifest{}, err
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name != manifestName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return Manifest{}, err
		}
		defer rc.Close()

		var m Manifest
		if err := json.NewDecoder(rc).Decode(&m); err != nil {
			return Manifest{}, err
		}
		return m, nil
	}
	return Manifest{}, fmt.Errorf("%s has no manifest", filepath.Base(path))
}

func Restore(info server.Info, archive Archive) (safety Result, err error) {
	if running, why := info.IsRunning(); running {
		return Result{}, fmt.Errorf("the server is running (%s); stop it before restoring", why)
	}

	safety, err = Create(info, ReasonPreRestore)
	if err != nil {
		return Result{}, fmt.Errorf("could not save the current state, so nothing was restored: %w", err)
	}

	staging, err := os.MkdirTemp(info.Dir, ".mcinit-restore-")
	if err != nil {
		return safety, err
	}
	defer os.RemoveAll(staging)

	roots, err := extract(archive.Path, staging)
	if err != nil {
		return safety, err
	}
	if len(roots) == 0 {
		return safety, fmt.Errorf("%s is empty", filepath.Base(archive.Path))
	}

	err = ui.Spin("Replacing worlds and configuration", func() error {
		for _, name := range roots {
			target := filepath.Join(info.Dir, name)
			if err := os.RemoveAll(target); err != nil {
				return fmt.Errorf("could not remove %s: %w", name, err)
			}
			if err := os.Rename(filepath.Join(staging, name), target); err != nil {
				return fmt.Errorf("could not restore %s: %w", name, err)
			}
		}
		return nil
	})
	return safety, err
}

func extract(archivePath, staging string) ([]string, error) {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", filepath.Base(archivePath), err)
	}
	defer zr.Close()

	var total int64
	for _, f := range zr.File {
		if f.Name != manifestName {
			total += int64(f.UncompressedSize64)
		}
	}
	bar := ui.NewProgress("Unpacking "+filepath.Base(archivePath), total)
	defer bar.Finish()

	seen := map[string]bool{}
	var roots []string

	for _, f := range zr.File {
		if f.Name == manifestName {
			continue
		}
		dest, err := safeJoin(staging, f.Name)
		if err != nil {
			return nil, err
		}

		top, _, _ := strings.Cut(filepath.ToSlash(f.Name), "/")
		if top != "" && !seen[top] {
			seen[top] = true
			roots = append(roots, top)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				return nil, err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, err
		}
		if err := writeEntry(f, dest, bar); err != nil {
			return nil, err
		}
	}
	return roots, nil
}

func writeEntry(f *zip.File, dest string, bar io.Writer) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("could not read %s from the backup: %w", f.Name, err)
	}
	defer rc.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, io.TeeReader(rc, bar)); err != nil {
		return fmt.Errorf("could not unpack %s: %w", f.Name, err)
	}
	return out.Close()
}

func safeJoin(base, name string) (string, error) {
	cleaned := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "/") {
		return "", fmt.Errorf("the backup contains an absolute path (%s); refusing to unpack it", name)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("the backup tries to write outside the server folder (%s); refusing to unpack it", name)
	}

	if vol := filepath.VolumeName(filepath.FromSlash(cleaned)); vol != "" {
		return "", fmt.Errorf("the backup contains a drive-qualified path (%s); refusing to unpack it", name)
	}
	return filepath.Join(base, filepath.FromSlash(cleaned)), nil
}
