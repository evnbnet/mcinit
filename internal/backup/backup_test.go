package backup

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evanb/mcinit/internal/server"
)

func fakeServer(t *testing.T) server.Info {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		// Port 1 is privileged and never listening, so IsRunning's port check
		// cannot be tripped by whatever else happens to be running on the
		// machine — including a real Minecraft server on 25565.
		"server.properties":          "level-name=world\nserver-port=1\nmotd=test\n",
		"ops.json":                   `[{"name":"evanb"}]`,
		"whitelist.json":             `[]`,
		"eula.txt":                   "eula=true\n",
		"start.bat":                  "@echo off\n",
		"paper-1.21.11-132.jar":      strings.Repeat("JAR", 200),
		"world/level.dat":            "the overworld",
		"world/session.lock":         "snowman",
		"world/region/r.0.0.mca":     strings.Repeat("REGION", 100),
		"world_nether/level.dat":     "the nether",
		"world_the_end/level.dat":    "the end",
		"plugins/EssentialsX.jar":    strings.Repeat("PLUGIN", 50),
		"plugins/Essentials/cfg.yml": "a plugin setting",
		"libraries/com/foo/bar.jar":  strings.Repeat("LIB", 300),
		"logs/latest.log":            "noise",
		"cache/mojang.json":          "cached",
	}
	for rel, body := range files {
		abs := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	info, err := server.Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func zipEntries(t *testing.T, path string) map[string]string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	out := map[string]string{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		body := make([]byte, f.UncompressedSize64)
		if len(body) > 0 {
			if _, err := rc.Read(body); err != nil && err.Error() != "EOF" {
				t.Fatalf("read %s: %v", f.Name, err)
			}
		}
		rc.Close()
		out[f.Name] = string(body)
	}
	return out
}

func TestCreateContents(t *testing.T) {
	info := fakeServer(t)

	result, err := Create(info, ReasonManual)
	if err != nil {
		t.Fatal(err)
	}
	entries := zipEntries(t, result.Path)

	for _, want := range []string{
		"world/level.dat",
		"world/region/r.0.0.mca",
		"world_nether/level.dat",
		"world_the_end/level.dat",
		"server.properties",
		"ops.json",
		"whitelist.json",

		"plugins/EssentialsX.jar",
		"plugins/Essentials/cfg.yml",
		manifestName,
	} {
		if _, ok := entries[want]; !ok {
			t.Errorf("%s is missing from the backup", want)
		}
	}

	for _, unwanted := range []string{
		"paper-1.21.11-132.jar",
		"libraries/com/foo/bar.jar",
		"logs/latest.log",
		"cache/mojang.json",
	} {
		if _, ok := entries[unwanted]; ok {
			t.Errorf("%s should not be in the backup", unwanted)
		}
	}

	if got := entries["world/level.dat"]; got != "the overworld" {
		t.Errorf("world/level.dat = %q, want the original contents", got)
	}
}

func TestCreateExcludesEarlierBackups(t *testing.T) {
	info := fakeServer(t)

	first, err := Create(info, ReasonManual)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Create(info, ReasonPreRestore)
	if err != nil {
		t.Fatal(err)
	}

	for name := range zipEntries(t, second.Path) {
		if strings.HasPrefix(name, BackupsDir+"/") {
			t.Errorf("the second backup contains %s", name)
		}
	}
	if second.Size > first.Size*2 {
		t.Errorf("second backup is %d bytes vs %d; it probably swallowed the first", second.Size, first.Size)
	}
}

func TestCreateManifest(t *testing.T) {
	info := fakeServer(t)
	result, err := Create(info, ReasonManual)
	if err != nil {
		t.Fatal(err)
	}

	m, err := readManifest(result.Path)
	if err != nil {
		t.Fatal(err)
	}
	if m.LevelName != "world" {
		t.Errorf("LevelName = %q", m.LevelName)
	}
	if len(m.Worlds) != 3 {
		t.Errorf("Worlds = %v, want all three", m.Worlds)
	}
	if m.Reason != ReasonManual {
		t.Errorf("Reason = %q, want %q", m.Reason, ReasonManual)
	}
	if m.Files == 0 || m.Bytes == 0 {
		t.Errorf("manifest counts are empty: %d files, %d bytes", m.Files, m.Bytes)
	}
	if m.Created.IsZero() {
		t.Error("Created is zero")
	}
}

func TestCreateRefusesEmptyServer(t *testing.T) {

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "server.properties"), []byte("level-name=world\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "paper.jar"), []byte("jar"), 0o644)

	info, err := server.Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Create(info, ReasonManual); err == nil {
		t.Fatal("Create produced a backup of a server with nothing in it")
	}
}

func TestCreateLeavesNoPartFile(t *testing.T) {
	info := fakeServer(t)
	if _, err := Create(info, ReasonManual); err != nil {
		t.Fatal(err)
	}

	files, _ := os.ReadDir(filepath.Join(info.Dir, BackupsDir))
	for _, f := range files {
		if strings.HasSuffix(f.Name(), partSuffix) {
			t.Errorf("%s was left behind", f.Name())
		}
	}
}

func TestListNewestFirst(t *testing.T) {
	info := fakeServer(t)

	if archives, err := List(info); err != nil || len(archives) != 0 {
		t.Fatalf("List on a server with no backups = %v, %v", archives, err)
	}

	first, _ := Create(info, ReasonManual)

	os.Rename(first.Path, filepath.Join(info.Dir, BackupsDir, "2026-01-01_0900.zip"))
	Create(info, ReasonPreRestore)

	archives, err := List(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 2 {
		t.Fatalf("List returned %d archives, want 2", len(archives))
	}
	if archives[0].Created.Before(archives[1].Created) {
		t.Error("List did not return the newest backup first")
	}
	if !archives[0].HasManifest {
		t.Error("the newest archive has no manifest")
	}
	if !strings.Contains(archives[0].Label(), "saved before a restore") {
		t.Errorf("Label() = %q, should mark a pre-restore copy", archives[0].Label())
	}
}
