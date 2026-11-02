package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeServer(t *testing.T, properties string, worlds ...string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, propertiesFile), []byte(properties), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, w := range worlds {
		if err := os.MkdirAll(filepath.Join(dir, w), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestInspectDefaults(t *testing.T) {
	dir := writeServer(t, "motd=hello\n")

	info, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.LevelName != "world" {
		t.Errorf("LevelName = %q, want the Minecraft default world", info.LevelName)
	}
	if info.Port != 25565 {
		t.Errorf("Port = %d, want the Minecraft default 25565", info.Port)
	}
	if info.Name != filepath.Base(dir) {
		t.Errorf("Name = %q, want the directory name", info.Name)
	}
}

func TestInspectReadsProperties(t *testing.T) {
	dir := writeServer(t, `# a comment
! another comment style

level-name=survival
server-port=25570
motd=my server
malformed line with no equals
`)

	info, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.LevelName != "survival" {
		t.Errorf("LevelName = %q, want survival", info.LevelName)
	}
	if info.Port != 25570 {
		t.Errorf("Port = %d, want 25570", info.Port)
	}
}

func TestInspectIgnoresNonsensePort(t *testing.T) {

	for _, line := range []string{"server-port=abc", "server-port=", "server-port=99999", "server-port=0"} {
		info, err := Inspect(writeServer(t, line+"\n"))
		if err != nil {
			t.Fatal(err)
		}
		if info.Port != 25565 {
			t.Errorf("with %q the port became %d, want the default", line, info.Port)
		}
	}
}

func TestInspectRejectsNonServer(t *testing.T) {
	_, err := Inspect(t.TempDir())
	if err == nil {
		t.Fatal("Inspect accepted a directory with no server.properties")
	}

	if !strings.Contains(err.Error(), "server.properties") {
		t.Errorf("error %q does not mention the missing file", err)
	}
}

func TestWorldsFollowLevelNameAndExistence(t *testing.T) {
	dir := writeServer(t, "level-name=survival\n", "survival", "survival_nether")

	info, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := info.Worlds()
	want := []string{"survival", "survival_nether"}
	if len(got) != len(want) {
		t.Fatalf("Worlds() = %v, want %v (the end was never created)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Worlds()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWorldsEmptyBeforeFirstStart(t *testing.T) {
	info, err := Inspect(writeServer(t, "level-name=world\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Worlds()) != 0 {
		t.Errorf("Worlds() = %v, want none before the server has ever started", info.Worlds())
	}
}

func TestSessionLockedOnUnlockedFiles(t *testing.T) {
	dir := t.TempDir()

	missing := filepath.Join(dir, "session.lock")
	if sessionLocked(missing) {
		t.Error("a missing session.lock was reported as locked")
	}

	normal := filepath.Join(dir, "normal.lock")
	os.WriteFile(normal, []byte("snowman"), 0o644)
	if sessionLocked(normal) {
		t.Error("an ordinary readable file was reported as locked")
	}

	empty := filepath.Join(dir, "empty.lock")
	os.WriteFile(empty, nil, 0o644)
	if sessionLocked(empty) {
		t.Error("an empty file was reported as locked; EOF is not a lock")
	}
}

func TestIsRunningOnIdleServer(t *testing.T) {

	dir := writeServer(t, "level-name=world\nserver-port=1\n", "world")
	info, err := Inspect(dir)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "world", "session.lock"), []byte("x"), 0o644)

	if running, why := info.IsRunning(); running {
		t.Errorf("IsRunning() = true (%s) for a server that is not running", why)
	}
}
