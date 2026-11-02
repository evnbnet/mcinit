package javart

import (
	"context"
	"testing"
)

func TestParseVersion(t *testing.T) {
	// Real `java -version` output, which goes to stderr. Each block is what a
	// different vendor and generation actually prints.
	tests := []struct {
		name      string
		output    string
		wantMajor int
		wantRaw   string
	}{
		{
			name: "temurin 21",
			output: `openjdk version "21.0.12" 2026-07-21 LTS
OpenJDK Runtime Environment Temurin-21.0.12+8 (build 21.0.12+8-LTS)
OpenJDK 64-Bit Server VM Temurin-21.0.12+8 (build 21.0.12+8-LTS, mixed mode, sharing)`,
			wantMajor: 21,
			wantRaw:   "21.0.12",
		},
		{
			name: "temurin 25",
			output: `openjdk version "25.0.5" 2026-10-21 LTS
OpenJDK Runtime Environment Temurin-25.0.5+8 (build 25.0.5+8-LTS)`,
			wantMajor: 25,
			wantRaw:   "25.0.5",
		},
		{
			name: "temurin 17",
			output: `openjdk version "17.0.13" 2024-10-15
OpenJDK Runtime Environment Temurin-17.0.13+11 (build 17.0.13+11)`,
			wantMajor: 17,
			wantRaw:   "17.0.13",
		},
		{
			name: "legacy oracle 8 is Java 8, not Java 1",
			output: `java version "1.8.0_412"
Java(TM) SE Runtime Environment (build 1.8.0_412-b08)
Java HotSpot(TM) 64-Bit Server VM (build 25.412-b08, mixed mode)`,
			wantMajor: 8,
			wantRaw:   "1.8.0_412",
		},
		{
			name:      "early access build",
			output:    `openjdk version "26-ea" 2027-03-16`,
			wantMajor: 26,
			wantRaw:   "26-ea",
		},
		{
			name:      "no patch component",
			output:    `openjdk version "21" 2023-09-19`,
			wantMajor: 21,
			wantRaw:   "21",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			major, raw, err := parseVersion(tt.output)
			if err != nil {
				t.Fatal(err)
			}
			if major != tt.wantMajor {
				t.Errorf("major = %d, want %d", major, tt.wantMajor)
			}
			if raw != tt.wantRaw {
				t.Errorf("raw = %q, want %q", raw, tt.wantRaw)
			}
		})
	}
}

func TestParseVersionRejectsGarbage(t *testing.T) {
	for _, output := range []string{
		"",
		"command not found",
		`openjdk version "`,
		`openjdk version "oops"`,
	} {
		if _, _, err := parseVersion(output); err == nil {
			t.Errorf("parseVersion(%q) succeeded, want error", output)
		}
	}
}

// TestDetectOnThisMachine is informational: it reports what mcinit sees here.
// It does not assert a specific version, because that depends on the machine.
func TestDetectOnThisMachine(t *testing.T) {
	all := candidates()
	t.Logf("%d java candidate(s) found", len(all))
	for _, path := range all {
		if rt, err := probe(path); err == nil {
			t.Logf("  Java %d at %s", rt.Major, rt.Path)
		} else {
			t.Logf("  unprobeable: %s (%v)", path, err)
		}
	}

	// Whatever is installed, asking for an impossible version must fail
	// rather than hand back something too old.
	if rt, ok := Detect(999); ok {
		t.Errorf("Detect(999) returned %s; nothing should satisfy that", rt)
	}
}

func TestDetectPicksClosestMatch(t *testing.T) {
	// Detect's contract: with 21 and 25 installed and 21 required, it must
	// choose 21. Verified against whatever this machine has.
	if _, ok := Detect(8); !ok {
		t.Skip("no Java installed here; nothing to compare")
	}
	low, _ := Detect(8)
	for _, path := range candidates() {
		if rt, err := probe(path); err == nil && rt.Major >= 8 && rt.Major < low.Major {
			t.Errorf("Detect(8) chose Java %d but Java %d is also installed", low.Major, rt.Major)
		}
	}
}

func TestAdoptiumArch(t *testing.T) {
	// Adoptium's names differ from Go's; a wrong mapping yields an empty asset
	// list and a confusing "no installer published" error.
	if got := adoptiumArch(); got != "x64" && got != "aarch64" && got != "x86" {
		t.Errorf("adoptiumArch() = %q, want one of x64/aarch64/x86", got)
	}
}

// TestFindInstallerLive checks that Adoptium still publishes a verifiable
// Windows installer for the Java versions mcinit may need to install.
func TestFindInstallerLive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Adoptium check in -short mode")
	}
	for _, major := range []int{17, 21, 25} {
		asset, err := findInstaller(context.Background(), major)
		if err != nil {
			t.Errorf("findInstaller(%d): %v", major, err)
			continue
		}
		if len(asset.Binary.Installer.Checksum) != 64 {
			t.Errorf("Java %d installer checksum = %q, want a sha256",
				major, asset.Binary.Installer.Checksum)
		}
		t.Logf("Java %d → %s (%s)", major, asset.Binary.Installer.Name, asset.ReleaseName)
	}
}
