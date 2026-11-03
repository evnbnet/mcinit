package server

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/evanb/mcinit/internal/config"
	"github.com/evanb/mcinit/internal/javart"
	"github.com/evanb/mcinit/internal/provider"
)

func testData(ram, javaMajor int) templateData {
	cfg := config.Default()
	cfg.Name = "mi-servidor"
	cfg.Software = config.Paper
	cfg.Version = "26.3"
	cfg.RAM = ram
	cfg.AcceptedEULA = true

	art := provider.Artifact{
		Filename: "paper-26.3-237.jar",
		Build:    "237",
		Algo:     provider.SHA256,
		Checksum: strings.Repeat("a", 64),
	}
	return newTemplateData(cfg, art, `C:\Program Files\Eclipse Adoptium\jre-25\bin\java.exe`, javaMajor)
}

func TestAikarFlagsSmallHeap(t *testing.T) {
	d := testData(4, 25)
	checks := map[string]int{
		"G1NewSizePercent": d.G1NewSizePercent, "G1MaxNewSizePercent": d.G1MaxNewSizePercent,
		"G1ReservePercent": d.G1ReservePercent, "InitiatingHeapOccupancyPercent": d.InitiatingHeapOccupancyPercent,
	}
	want := map[string]int{
		"G1NewSizePercent": 30, "G1MaxNewSizePercent": 40,
		"G1ReservePercent": 20, "InitiatingHeapOccupancyPercent": 15,
	}
	for k, got := range checks {
		if got != want[k] {
			t.Errorf("%s = %d, want %d for a 4 GB heap", k, got, want[k])
		}
	}
	if d.G1HeapRegionSize != "8M" {
		t.Errorf("G1HeapRegionSize = %q, want 8M", d.G1HeapRegionSize)
	}
}

func TestAikarFlagsLargeHeap(t *testing.T) {

	d := testData(16, 25)
	if d.G1NewSizePercent != 40 || d.G1MaxNewSizePercent != 50 {
		t.Errorf("new size = %d/%d, want 40/50 for a 16 GB heap", d.G1NewSizePercent, d.G1MaxNewSizePercent)
	}
	if d.G1HeapRegionSize != "16M" {
		t.Errorf("G1HeapRegionSize = %q, want 16M", d.G1HeapRegionSize)
	}
	if d.G1ReservePercent != 15 || d.InitiatingHeapOccupancyPercent != 20 {
		t.Errorf("reserve/IHOP = %d/%d, want 15/20", d.G1ReservePercent, d.InitiatingHeapOccupancyPercent)
	}
}

func TestAikarFlagsBoundaryIsTwelve(t *testing.T) {
	if testData(11, 25).G1HeapRegionSize != "8M" {
		t.Error("11 GB should use the small-heap tuning")
	}
	if testData(12, 25).G1HeapRegionSize != "16M" {
		t.Error("12 GB should use the large-heap tuning")
	}
}

func TestVectorModuleGatedByJavaVersion(t *testing.T) {

	if testData(4, 8).VectorModule {
		t.Error("Java 8 must not get --add-modules=jdk.incubator.vector")
	}
	if !testData(4, 17).VectorModule {
		t.Error("Java 17 should get --add-modules=jdk.incubator.vector")
	}
}

func TestStartBatContents(t *testing.T) {
	dir := t.TempDir()
	if err := renderAll(dir, testData(4, 25)); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "start.bat"))
	if err != nil {
		t.Fatal(err)
	}
	bat := string(raw)

	if !strings.Contains(bat, `set "JAVA=C:\Program Files\Eclipse Adoptium\jre-25\bin\java.exe"`) {
		t.Error("start.bat does not embed the absolute java path")
	}
	for _, want := range []string{
		"-Xms4G -Xmx4G",
		"-XX:+UseG1GC",
		"-Dusing.aikars.flags=https://mcflags.emc.gs",
		`-jar "paper-26.3-237.jar" nogui`,
		"pause",
	} {
		if !strings.Contains(bat, want) {
			t.Errorf("start.bat is missing %q", want)
		}
	}

	if strings.Contains(strings.ReplaceAll(bat, "\r\n", ""), "\n") {
		t.Error("start.bat contains bare LF line endings")
	}
}

func TestEulaAndPropertiesContents(t *testing.T) {
	dir := t.TempDir()
	if err := renderAll(dir, testData(4, 25)); err != nil {
		t.Fatal(err)
	}

	eula, err := os.ReadFile(filepath.Join(dir, "eula.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(eula), "eula=true") {
		t.Error("eula.txt does not accept the EULA")
	}

	props, err := os.ReadFile(filepath.Join(dir, "server.properties"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"gamemode=survival",
		"difficulty=normal",
		"max-players=20",
		"server-port=25565",
		"online-mode=true",
	} {
		if !strings.Contains(string(props), want) {
			t.Errorf("server.properties is missing %q", want)
		}
	}
}

func TestEscapeProperty(t *testing.T) {
	// The expectations are built from a constant rather than written as literal
	// backslashes. A `\\` typed inline is easy to mangle, and when the same
	// mangling hits both the function and its test, the test silently stops
	// asserting anything — which is exactly what happened here once.
	const bs = `\`
	tests := []struct{ in, want string }{
		{"normal motd", "normal motd"},
		{"C:" + bs + "path", "C:" + bs + bs + "path"},
		{bs, bs + bs},
		{"two\nlines", "two" + bs + "nlines"},
		{"with\r\nCRLF", "with" + bs + "nCRLF"},
	}
	for _, tt := range tests {
		if got := escapeProperty(tt.in); got != tt.want {
			t.Errorf("escapeProperty(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// TestPropertyRoundTrip is the guarantee that matters: whatever mcinit writes
// into server.properties has to come back out unchanged when it reads it.
func TestPropertyRoundTrip(t *testing.T) {
	const bs = `\`
	for _, original := range []string{
		"A server created with mcinit",
		"C:" + bs + "Users" + bs + "evanb",
		bs + bs + "share" + bs + "path",
		"line one\nline two",
		"nothing special",
	} {
		if got := unescapeProperty(escapeProperty(original)); got != original {
			t.Errorf("round trip of %q gave %q", original, got)
		}
	}
}

// TestUnescapePropertyHandlesWhatMinecraftWrites covers the forms a real
// server.properties contains but mcinit never produces: Minecraft escapes the
// colon in level-type and writes non-ASCII as \uXXXX.
func TestUnescapePropertyHandlesWhatMinecraftWrites(t *testing.T) {
	tests := []struct{ in, want string }{
		{`minecraft\:normal`, "minecraft:normal"},
		{`a\=b`, "a=b"},
		{`\` + "u00f1", "ñ"},
		{`tab\there`, "tab\there"},
		{"nothing to do", "nothing to do"},
		// Malformed escapes are kept rather than turned into garbage.
		{`\u00`, `\u00`},
		{`trailing\`, `trailing\`},
	}
	for _, tt := range tests {
		if got := unescapeProperty(tt.in); got != tt.want {
			t.Errorf("unescapeProperty(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCheckTarget(t *testing.T) {
	base := t.TempDir()

	if err := CheckTarget(filepath.Join(base, "nope")); err != nil {
		t.Errorf("missing dir rejected: %v", err)
	}

	empty := filepath.Join(base, "empty")
	os.Mkdir(empty, 0o755)
	if err := CheckTarget(empty); err != nil {
		t.Errorf("empty dir rejected: %v", err)
	}

	full := filepath.Join(base, "full")
	os.Mkdir(full, 0o755)
	os.WriteFile(filepath.Join(full, "level.dat"), []byte("x"), 0o644)
	if err := CheckTarget(full); err == nil {
		t.Error("CheckTarget accepted a non-empty directory")
	}
}

func TestCreateRefusesWithoutEULA(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "srv")
	cfg := config.Default()
	cfg.Name = "x"
	cfg.Software = config.Paper
	cfg.Version = "26.3"
	cfg.AcceptedEULA = false

	_, err := Create(nil, Plan{Config: cfg, Dir: dir, Java: javart.Runtime{Major: 25}})
	if err == nil {
		t.Fatal("Create ran without EULA acceptance")
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Error("Create created the directory despite refusing")
	}
}

// TestStartCommandContents renders the macOS launch script from whatever
// machine the tests run on. The start script is the most important file mcinit
// writes and the hardest to eyeball on a platform you are not using, so it is
// checked here rather than left to a Mac.
func TestStartCommandContents(t *testing.T) {
	dir := t.TempDir()
	if err := renderAllFor("darwin", dir, testData(4, 25)); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "start.command"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)

	if !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Error("start.command has no shebang on its first line")
	}
	// The absolute java path is the whole point: a freshly installed Java is
	// not on the PATH of an already-open shell.
	if !strings.Contains(script, `JAVA="C:\Program Files\Eclipse Adoptium\jre-25\bin\java.exe"`) {
		t.Error("start.command does not embed the absolute java path")
	}
	for _, want := range []string{
		`cd "$(dirname "$0")"`, // Finder starts scripts in the home directory
		"-Xms4G -Xmx4G",
		"-XX:+UseG1GC",
		"-Dusing.aikars.flags=https://mcflags.emc.gs",
		`-jar "paper-26.3-237.jar" nogui`,
		"read -r _unused", // keeps the Finder window open on a crash
	} {
		if !strings.Contains(script, want) {
			t.Errorf("start.command is missing %q", want)
		}
	}
	// A shell script with CRLF fails with "bad interpreter".
	if strings.Contains(script, "\r") {
		t.Error("start.command contains carriage returns")
	}
}

func TestStartCommandIsExecutable(t *testing.T) {
	dir := t.TempDir()
	if err := renderAllFor("darwin", dir, testData(4, 25)); err != nil {
		t.Fatal(err)
	}

	st, err := os.Stat(filepath.Join(dir, "start.command"))
	if err != nil {
		t.Fatal(err)
	}
	// Windows does not model the execute bit, so the assertion only means
	// something where it does.
	if runtime.GOOS != "windows" && st.Mode().Perm()&0o111 == 0 {
		t.Errorf("start.command mode is %v; Finder and the shell both need the execute bit", st.Mode().Perm())
	}
}

func TestEachPlatformGetsOneStartScript(t *testing.T) {
	for goos, want := range map[string]string{
		"windows": "start.bat",
		"darwin":  "start.command",
	} {
		dir := t.TempDir()
		if err := renderAllFor(goos, dir, testData(4, 25)); err != nil {
			t.Fatal(err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var scripts []string
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "start.") {
				scripts = append(scripts, e.Name())
			}
		}
		if len(scripts) != 1 || scripts[0] != want {
			t.Errorf("%s produced %v, want exactly [%s]", goos, scripts, want)
		}
		// eula.txt and server.properties are written on every platform.
		for _, shared := range []string{"eula.txt", "server.properties"} {
			if _, err := os.Stat(filepath.Join(dir, shared)); err != nil {
				t.Errorf("%s did not get %s", goos, shared)
			}
		}
	}
}

func TestVectorModuleOmittedForJava8OnMac(t *testing.T) {
	dir := t.TempDir()
	if err := renderAllFor("darwin", dir, testData(4, 8)); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "start.command"))
	if strings.Contains(string(raw), "add-modules") {
		t.Error("Java 8 must not get --add-modules=jdk.incubator.vector")
	}
}

// TestStartCommandSurvivesAFailingServer runs the generated macOS script with a
// stand-in java that exits non-zero, and checks the script still reaches its
// closing prompt.
//
// This is the behaviour a "set -e" at the top silently breaks: the script would
// exit the moment java failed, closing the Finder window on exactly the crash
// the user needs to read. A static check for the absence of "set -e" would
// not catch an equivalent mistake, so the script is actually executed.
func TestStartCommandSurvivesAFailingServer(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no POSIX shell available to run the script")
	}

	dir := t.TempDir()

	// A java that reports failure, like a server that crashes on startup.
	fakeJava := filepath.Join(dir, "fake-java")
	script := "#!/bin/sh\necho 'simulated crash' >&2\nexit 3\n"
	if err := os.WriteFile(fakeJava, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	data := testData(4, 25)
	data.JavaPath = toShellPath(t, fakeJava)
	if err := renderAllFor("darwin", dir, data); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(sh, toShellPath(t, filepath.Join(dir, "start.command")))
	cmd.Stdin = strings.NewReader("\n") // the closing prompt
	out, runErr := cmd.CombinedOutput()
	output := string(out)

	if !strings.Contains(output, "The server stopped with an error (exit 3)") {
		t.Errorf("the script did not report the failure and reach its prompt:\n%s", output)
	}
	if !strings.Contains(output, "simulated crash") {
		t.Errorf("the server's own output was swallowed:\n%s", output)
	}
	// The script must pass the server's exit code on, not hide it.
	var exitErr *exec.ExitError
	if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 3 {
		t.Errorf("script exit code = %v, want 3", runErr)
	}
}

// TestStartCommandRunsFromAnyDirectory checks the cd that makes a double click
// work: Finder launches scripts from the home directory, not from the folder
// the script lives in.
func TestStartCommandRunsFromAnyDirectory(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no POSIX shell available to run the script")
	}

	dir := t.TempDir()

	// A java that proves which directory it was started in.
	fakeJava := filepath.Join(dir, "fake-java")
	script := "#!/bin/sh\necho \"cwd=$(pwd)\"\nexit 0\n"
	if err := os.WriteFile(fakeJava, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	data := testData(4, 25)
	data.JavaPath = toShellPath(t, fakeJava)
	if err := renderAllFor("darwin", dir, data); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(sh, toShellPath(t, filepath.Join(dir, "start.command")))
	cmd.Dir = os.TempDir() // started from somewhere else entirely
	cmd.Stdin = strings.NewReader("\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}

	// The jar is referenced by bare name, so the server must run with the
	// server folder as its working directory.
	if !strings.Contains(string(out), "cwd=") {
		t.Fatalf("the stand-in java did not report a directory:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(string(out)), strings.ToLower(filepath.Base(dir))) {
		t.Errorf("the script ran from the wrong directory:\n%s", out)
	}
}

// toShellPath converts a path for the POSIX shell running the test. On Windows
// that shell is MSYS/Git Bash, which wants /c/... rather than C:\...
func toShellPath(t *testing.T, path string) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		return path
	}
	converted, err := exec.Command("cygpath", "-u", path).Output()
	if err != nil {
		t.Skipf("cygpath unavailable, cannot run shell scripts here: %v", err)
	}
	return strings.TrimSpace(string(converted))
}
