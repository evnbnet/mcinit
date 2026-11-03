package javart

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {

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

	if rt, ok := Detect(999); ok {
		t.Errorf("Detect(999) returned %s; nothing should satisfy that", rt)
	}
}

func TestDetectPicksClosestMatch(t *testing.T) {

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
	// Adoptium names architectures differently from Go; a wrong value yields an
	// empty asset list and a confusing "no package published" error.
	if got := adoptiumArch(); got != "x64" && got != "aarch64" && got != "x86" {
		t.Errorf("adoptiumArch() = %q, want one of x64/aarch64/x86", got)
	}
}

func TestAdoptiumOSIsOneTheAPIKnows(t *testing.T) {
	switch got := adoptiumOS(); got {
	case "windows", "mac", "linux":
	default:
		t.Errorf("adoptiumOS() = %q, which the API does not recognise", got)
	}
}

// TestAdoptiumPackagesLiveEveryPlatform asks Adoptium for the exact downloads
// mcinit would use on each platform it targets, including the ones this machine
// is not.
//
// That last part is the point: a Windows developer has no other way to notice
// that the mac tarball stopped being published, and the macOS install path
// cannot work without it.
func TestAdoptiumPackagesLiveEveryPlatform(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live Adoptium check in -short mode")
	}

	cases := []struct {
		goos, arch string
		want       pick
		kind       string
		suffix     string
		// upstreamGap marks a combination Adoptium does not publish at all.
		// Recorded rather than asserted, because it is not ours to fix: on
		// Windows ARM the only route to a modern Java is winget installing the
		// x64 build, which runs under emulation.
		upstreamGap bool
	}{
		// Windows installs from the MSI.
		{goos: "windows", arch: "x64", want: pickInstaller, kind: "installer", suffix: ".msi"},
		{goos: "windows", arch: "aarch64", want: pickInstaller, kind: "installer", suffix: ".msi", upstreamGap: true},
		// macOS unpacks the archive, so no password is needed.
		{goos: "mac", arch: "x64", want: pickArchive, kind: "archive", suffix: ".tar.gz"},
		{goos: "mac", arch: "aarch64", want: pickArchive, kind: "archive", suffix: ".tar.gz"},
	}

	for _, c := range cases {
		for _, major := range []int{21, 25} {
			name := fmt.Sprintf("%s-%s-java%d-%s", c.goos, c.arch, major, c.kind)
			t.Run(name, func(t *testing.T) {
				file, release, err := findAssetFor(context.Background(), major, c.goos, c.arch, c.want)
				if err != nil {
					if c.upstreamGap {
						t.Logf("%s: not published by Adoptium (known gap): %v", name, err)
						return
					}
					t.Errorf("%s: %v", name, err)
					return
				}
				if len(file.Checksum) != 64 {
					t.Errorf("%s: checksum = %q, want a sha256", name, file.Checksum)
				}
				if !strings.HasSuffix(file.Name, c.suffix) {
					t.Errorf("%s: got %q, want a %s", name, file.Name, c.suffix)
				}
				if file.Size <= 0 {
					t.Errorf("%s: size = %d", name, file.Size)
				}
				t.Logf("%-28s %s (%s)", name, file.Name, release)
			})
		}
	}
}
