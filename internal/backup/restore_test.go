package backup

import (
	"archive/zip"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readFile(t *testing.T, parts ...string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(parts...))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestRestoreRoundTrip(t *testing.T) {
	info := fakeServer(t)

	if _, err := Create(info, ReasonManual); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(info.Dir, "world", "level.dat"), []byte("RUINED"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(info.Dir, "world_nether")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(info.Dir, "ops.json"), []byte("[]"), 0o644); err != nil {
		t.Fatal(err)
	}

	archives, err := List(info)
	if err != nil {
		t.Fatal(err)
	}
	if len(archives) != 1 {
		t.Fatalf("expected one backup, got %d", len(archives))
	}

	if _, err := Restore(info, archives[0]); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, info.Dir, "world", "level.dat"); got != "the overworld" {
		t.Errorf("world/level.dat = %q, want the backed-up contents", got)
	}
	if got := readFile(t, info.Dir, "world_nether", "level.dat"); got != "the nether" {
		t.Errorf("the deleted nether was not restored, got %q", got)
	}
	if got := readFile(t, info.Dir, "ops.json"); got != `[{"name":"evanb"}]` {
		t.Errorf("ops.json = %q, want the backed-up operators", got)
	}
	if got := readFile(t, info.Dir, "world", "region", "r.0.0.mca"); !strings.HasPrefix(got, "REGION") {
		t.Errorf("region file was not restored intact")
	}
}

func TestRestoreLeavesASafetyCopy(t *testing.T) {
	info := fakeServer(t)
	if _, err := Create(info, ReasonManual); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(info.Dir, "world", "level.dat"), []byte("THE CURRENT WORLD"), 0o644)

	archives, _ := List(info)
	safety, err := Restore(info, archives[len(archives)-1])
	if err != nil {
		t.Fatal(err)
	}

	if safety.Manifest.Reason != ReasonPreRestore {
		t.Errorf("safety copy reason = %q, want %q", safety.Manifest.Reason, ReasonPreRestore)
	}
	entries := zipEntries(t, safety.Path)
	if got := entries["world/level.dat"]; got != "THE CURRENT WORLD" {
		t.Errorf("the safety copy holds %q, not the state that was replaced", got)
	}

	after, _ := List(info)
	found := false
	for _, a := range after {
		if a.Path == safety.Path {
			found = true
		}
	}
	if !found {
		t.Error("the safety copy does not appear in List")
	}
}

func TestRestoreDoesNotRestoreTheServerJar(t *testing.T) {

	info := fakeServer(t)
	Create(info, ReasonManual)

	archives, _ := List(info)
	if _, err := Restore(info, archives[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(info.Dir, "paper-1.21.11-132.jar")); err != nil {
		t.Errorf("the server jar went missing after a restore: %v", err)
	}
}

func writeHostileZip(t *testing.T, dir, entryName string) string {
	t.Helper()
	path := filepath.Join(dir, "hostile.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	zw := zip.NewWriter(file)
	w, err := zw.Create(entryName)
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("pwned"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRestoreRefusesZipSlip(t *testing.T) {
	hostile := []string{
		"../escaped.txt",
		"../../escaped.txt",
		"world/../../escaped.txt",
		`..\escaped.txt`,
		"/absolute.txt",
	}

	for _, name := range hostile {
		t.Run(name, func(t *testing.T) {
			_ = fakeServer(t)
			staging := t.TempDir()
			path := writeHostileZip(t, t.TempDir(), name)

			if _, err := extract(path, staging); err == nil {
				t.Fatalf("extract accepted the entry %q", name)
			}

			outside := filepath.Join(filepath.Dir(staging), "escaped.txt")
			if _, err := os.Stat(outside); err == nil {
				t.Errorf("%q wrote outside the staging directory", name)
			}
		})
	}
}

func TestSafeJoinAcceptsOrdinaryNames(t *testing.T) {
	base := filepath.Join("C:", "servers", "mine")
	for _, name := range []string{
		"world/level.dat",
		"plugins/EssentialsX.jar",
		"server.properties",
		"world/region/r.0.0.mca",
	} {
		got, err := safeJoin(base, name)
		if err != nil {
			t.Errorf("safeJoin(%q) = %v, want it accepted", name, err)
			continue
		}
		if !strings.HasPrefix(got, base) {
			t.Errorf("safeJoin(%q) = %q, which is outside %q", name, got, base)
		}
	}
}

func TestRestoreRefusesWhileRunning(t *testing.T) {

	info := fakeServer(t)
	Create(info, ReasonManual)
	archives, _ := List(info)

	listener, addr := listenSomewhere(t)
	defer listener.Close()
	info.Port = addr

	if _, err := Restore(info, archives[0]); err == nil {
		t.Fatal("Restore replaced the world of a running server")
	} else if !strings.Contains(err.Error(), "running") {
		t.Errorf("error %q should say the server is running", err)
	}
}

func listenSomewhere(t *testing.T) (net.Listener, int) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return listener, listener.Addr().(*net.TCPAddr).Port
}

func TestRestorePreservesEmptyDirectories(t *testing.T) {
	info := fakeServer(t)

	empty := []string{
		filepath.Join("world", "playerdata"),
		filepath.Join("world", "data"),
		filepath.Join("world", "datapacks"),
	}
	for _, rel := range empty {
		if err := os.MkdirAll(filepath.Join(info.Dir, rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Create(info, ReasonManual); err != nil {
		t.Fatal(err)
	}

	if err := os.RemoveAll(filepath.Join(info.Dir, "world")); err != nil {
		t.Fatal(err)
	}

	archives, _ := List(info)
	if _, err := Restore(info, archives[0]); err != nil {
		t.Fatal(err)
	}

	for _, rel := range empty {
		st, err := os.Stat(filepath.Join(info.Dir, rel))
		if err != nil {
			t.Errorf("%s did not survive the round trip: %v", rel, err)
			continue
		}
		if !st.IsDir() {
			t.Errorf("%s came back as a file", rel)
		}
	}
}
