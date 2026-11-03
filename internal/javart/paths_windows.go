//go:build windows

package javart

import (
	"os"
	"path/filepath"
	"sort"
)

// platformJavaPaths lists the java.exe files in the places Windows installers
// use, newest-looking first.
//
// Both machine-wide and per-user locations are searched because winget can
// install to either, and the per-user one is what an unelevated install gets.
func platformJavaPaths() []string {
	var roots []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		roots = append(roots,
			filepath.Join(base, "Eclipse Adoptium"),
			filepath.Join(base, "Programs", "Eclipse Adoptium"),
			filepath.Join(base, "Java"),
			filepath.Join(base, "Microsoft", "jdk"),
		)
	}

	var out []string
	for _, root := range roots {
		matches, err := filepath.Glob(filepath.Join(root, "*", "bin", "java.exe"))
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		out = append(out, matches...)
	}
	return out
}

// adoptiumOS is the name Adoptium's API uses for this platform.
func adoptiumOS() string { return "windows" }
