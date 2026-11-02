package backup

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/evanb/mcinit/internal/buildinfo"
	"github.com/evanb/mcinit/internal/server"
	"github.com/evanb/mcinit/internal/ui"
)

// stampLayout names backups so that sorting them as text sorts them by time.
const stampLayout = "2006-01-02_1504"

// partSuffix is the in-progress name, renamed only once the zip is complete.
// Same idea as internal/download: a half-written file must never look like a
// usable one.
const partSuffix = ".part"

// Manifest is stored inside each zip so a backup can describe itself, rather
// than the listing having to guess from the file name.
type Manifest struct {
	McinitVersion string    `json:"mcinit_version"`
	Created       time.Time `json:"created"`
	ServerName    string    `json:"server_name"`
	LevelName     string    `json:"level_name"`
	Worlds        []string  `json:"worlds"`
	Files         int       `json:"files"`
	Bytes         int64     `json:"bytes"`
	// Reason distinguishes a backup the user asked for from the safety copy
	// restore takes before overwriting.
	Reason string `json:"reason"`
}

// Reasons recorded in a manifest.
const (
	ReasonManual     = "manual"
	ReasonPreRestore = "pre-restore"
)

// Result reports what a backup ended up containing.
type Result struct {
	Path     string
	Manifest Manifest
	Size     int64 // size of the zip on disk
}

// entry is one file queued for the archive.
type entry struct {
	abs  string
	rel  string // forward-slash path, used as the zip entry name
	size int64
}

// Create writes a backup of the server in dir. Nothing is written to the final
// path unless the whole archive succeeds.
func Create(info server.Info, reason string) (Result, error) {
	// Config alone is not worth an archive: the point of a backup is the world,
	// and a zip that restores nothing is worse than no zip at all because it
	// looks like protection.
	if len(info.Worlds()) == 0 {
		return Result{}, fmt.Errorf("there is no world to back up yet — start the server once so it creates one")
	}

	entries, dirs, total, err := collect(info)
	if err != nil {
		return Result{}, err
	}
	if len(entries) == 0 {
		return Result{}, fmt.Errorf("there is nothing to back up in %s", info.Dir)
	}

	dir := filepath.Join(info.Dir, BackupsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("could not create %s: %w", dir, err)
	}

	name := time.Now().Format(stampLayout)
	if reason == ReasonPreRestore {
		name += "_pre-restore"
	}
	final := filepath.Join(dir, name+".zip")
	part := final + partSuffix

	manifest := Manifest{
		McinitVersion: buildinfo.Version,
		Created:       time.Now(),
		ServerName:    info.Name,
		LevelName:     info.LevelName,
		Worlds:        info.Worlds(),
		Files:         len(entries),
		Bytes:         total,
		Reason:        reason,
	}

	if err := writeArchive(part, entries, dirs, manifest); err != nil {
		os.Remove(part)
		return Result{}, err
	}

	os.Remove(final) // Windows will not rename onto an existing file
	if err := os.Rename(part, final); err != nil {
		os.Remove(part)
		return Result{}, err
	}

	size := int64(0)
	if st, err := os.Stat(final); err == nil {
		size = st.Size()
	}
	return Result{Path: final, Manifest: manifest, Size: size}, nil
}

// collect walks the server directory and returns everything the rules keep,
// along with the total byte count so the progress bar can be accurate.
func collect(info server.Info) ([]entry, []string, int64, error) {
	selection := NewSelection(info)

	var entries []entry
	var dirs []string
	var total int64

	err := filepath.Walk(info.Dir, func(abs string, fi os.FileInfo, err error) error {
		if err != nil {
			// A file that vanished mid-walk is not worth aborting over.
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		rel, relErr := filepath.Rel(info.Dir, abs)
		if relErr != nil || rel == "." {
			return nil
		}
		slashed := filepath.ToSlash(rel)

		if !selection.Include(slashed, fi.IsDir()) {
			if fi.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if fi.IsDir() {
			// Directories are recorded too. An empty one — playerdata before
			// anybody joins, a datapacks folder with nothing in it — would
			// otherwise vanish on restore, because a zip built only from file
			// paths cannot describe it.
			dirs = append(dirs, slashed)
			return nil
		}
		if !fi.Mode().IsRegular() {
			return nil
		}

		entries = append(entries, entry{abs: abs, rel: slashed, size: fi.Size()})
		total += fi.Size()
		return nil
	})
	if err != nil {
		return nil, nil, 0, fmt.Errorf("could not read %s: %w", info.Dir, err)
	}
	return entries, dirs, total, nil
}

// writeArchive compresses every entry plus the manifest, showing progress.
func writeArchive(path string, entries []entry, dirs []string, manifest Manifest) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	zw := zip.NewWriter(file)

	for _, dir := range dirs {
		if _, err := zw.Create(dir + "/"); err != nil {
			zw.Close()
			return err
		}
	}

	bar := ui.NewProgress(fmt.Sprintf("Packing %d files", len(entries)), manifest.Bytes)

	for _, e := range entries {
		if err := addFile(zw, e, bar); err != nil {
			bar.Finish()
			zw.Close()
			return err
		}
	}
	bar.Finish()

	if err := addManifest(zw, manifest); err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

func addFile(zw *zip.Writer, e entry, bar io.Writer) error {
	src, err := os.Open(e.abs)
	if err != nil {
		// Minecraft may hold a file open; say which one rather than failing
		// with a bare path.
		return fmt.Errorf("could not read %s: %w", e.rel, err)
	}
	defer src.Close()

	w, err := zw.CreateHeader(&zip.FileHeader{
		Name:   e.rel,
		Method: zip.Deflate,
	})
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, io.TeeReader(src, bar)); err != nil {
		return fmt.Errorf("could not pack %s: %w", e.rel, err)
	}
	return nil
}

func addManifest(zw *zip.Writer, manifest Manifest) error {
	w, err := zw.Create(manifestName)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(manifest)
}

// Describe summarises what a backup holds, for the closing lines of the
// command and for the restore picker.
func (m Manifest) Describe() string {
	worlds := "no worlds"
	if len(m.Worlds) > 0 {
		worlds = strings.Join(m.Worlds, ", ")
	}
	return fmt.Sprintf("%s · %d files", worlds, m.Files)
}

// HumanSize renders a byte count for display.
func HumanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	value := float64(n)
	for _, suffix := range []string{"KB", "MB", "GB", "TB"} {
		value /= unit
		if value < unit {
			return fmt.Sprintf("%.1f %s", value, suffix)
		}
	}
	return fmt.Sprintf("%.1f PB", value/unit)
}
