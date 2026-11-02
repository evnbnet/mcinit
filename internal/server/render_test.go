package server

import (
	"os"
	"path/filepath"
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
	// Aikar's tuning changes at 12 GB; picking the wrong side is a silent
	// performance regression, which is exactly why this is tested.
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
	// --add-modules=jdk.incubator.vector does not exist before Java 9; passing
	// it to a Java 8 runtime makes the server refuse to start.
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

	// The absolute java path is the whole point: a freshly installed JRE is
	// not on the PATH of an already-running shell.
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
	// Batch files get CRLF.
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
	// The expectations are built from constants rather than written as literal
	// backslashes: a `\\` typed inline here is easy to mangle, and when the
	// same mangling hits both the function and its test the test silently
	// stops asserting anything.
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

// TestPropertyRoundTrip is the guarantee that actually matters: whatever goes
// into server.properties must come back out unchanged when mcinit reads it.
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

func TestCheckTarget(t *testing.T) {
	base := t.TempDir()

	// Missing is fine.
	if err := CheckTarget(filepath.Join(base, "nope")); err != nil {
		t.Errorf("missing dir rejected: %v", err)
	}

	// Existing but empty is fine.
	empty := filepath.Join(base, "empty")
	os.Mkdir(empty, 0o755)
	if err := CheckTarget(empty); err != nil {
		t.Errorf("empty dir rejected: %v", err)
	}

	// Existing with contents must be refused — it could hold a world.
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
