//go:build windows

package javart

import (
	"os"
	"path/filepath"
	"sort"
)

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

func adoptiumOS() string { return "windows" }
