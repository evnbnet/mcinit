//go:build darwin

package javart

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// macJavaHomeTool is the system utility that knows where Java is installed. It
// is the authority on macOS, so it is asked first; the globs below then cover
// anything it does not report, such as a runtime that is present but not
// registered.
const macJavaHomeTool = "/usr/libexec/java_home"

// platformJavaPaths lists the java binaries installed on this Mac.
//
// A macOS runtime is a bundle: the executable is at
// <jdk>/Contents/Home/bin/java rather than <jdk>/bin/java, which is why this
// cannot share the Windows glob.
func platformJavaPaths() []string {
	var out []string

	if home, err := exec.Command(macJavaHomeTool).Output(); err == nil {
		if trimmed := strings.TrimSpace(string(home)); trimmed != "" {
			out = append(out, filepath.Join(trimmed, "bin", "java"))
		}
	}

	patterns := []string{
		// Where installers and Homebrew casks put runtimes, and where mcinit
		// extracts the ones it downloads itself.
		"/Library/Java/JavaVirtualMachines/*/Contents/Home/bin/java",
		filepath.Join(userJavaDir(), "*", "Contents", "Home", "bin", "java"),
		// Homebrew formulae, Apple Silicon and Intel prefixes.
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

// userJavaDir is where a runtime can be installed without administrator
// rights, and where mcinit puts the ones it downloads.
func userJavaDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Library", "Java", "JavaVirtualMachines")
}

// adoptiumOS is the name Adoptium's API uses for this platform.
func adoptiumOS() string { return "mac" }
