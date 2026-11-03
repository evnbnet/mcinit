//go:build darwin

package javart

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const macJavaHomeTool = "/usr/libexec/java_home"

func platformJavaPaths() []string {
	var out []string

	if home, err := exec.Command(macJavaHomeTool).Output(); err == nil {
		if trimmed := strings.TrimSpace(string(home)); trimmed != "" {
			out = append(out, filepath.Join(trimmed, "bin", "java"))
		}
	}

	patterns := []string{

		"/Library/Java/JavaVirtualMachines/*/Contents/Home/bin/java",
		filepath.Join(userJavaDir(), "*", "Contents", "Home", "bin", "java"),

		"/opt/homebrew/opt/openjdk*/bin/java",
		"/opt/homebrew/opt/openjdk*/libexec/openjdk.jdk/Contents/Home/bin/java",
		"/usr/local/opt/openjdk*/bin/java",
		"/usr/local/opt/openjdk*/libexec/openjdk.jdk/Contents/Home/bin/java",
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		out = append(out, matches...)
	}
	return out
}

func userJavaDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Java", "JavaVirtualMachines")
}

func adoptiumOS() string { return "mac" }
