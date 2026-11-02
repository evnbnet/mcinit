// Package javart finds a usable Java runtime and, when there is none, installs
// one. Everything here deals in absolute paths to java.exe: a freshly installed
// JRE is not on the PATH of the running process, so relying on the bare name
// "java" would break the one case this package exists to handle.
package javart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Runtime is one java.exe found on this machine.
type Runtime struct {
	Path    string // absolute path to java.exe
	Major   int    // feature version: 8, 17, 21, 25
	Version string // as printed by java -version, e.g. "21.0.12"
}

func (r Runtime) String() string { return fmt.Sprintf("Java %s", r.Version) }

// Detect returns the most suitable installed runtime for a server that needs
// Java minMajor or newer, or false when there is none.
//
// Among the candidates that qualify it picks the *lowest* major version: if the
// machine has both 21 and 25 and the server needs 21, 21 is the closer match
// and the safer choice. Ties go to whatever the user configured explicitly
// (JAVA_HOME, then PATH).
func Detect(minMajor int) (Runtime, bool) {
	var best Runtime
	found := false

	for _, path := range candidates() {
		rt, err := probe(path)
		if err != nil || rt.Major < minMajor {
			continue
		}
		if !found || rt.Major < best.Major {
			best, found = rt, true
		}
	}
	return best, found
}

// candidates lists every java.exe worth probing, in order of preference, with
// duplicates removed.
func candidates() []string {
	var out []string
	seen := map[string]bool{}

	add := func(path string) {
		if path == "" {
			return
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		key := strings.ToLower(path)
		if seen[key] {
			return
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return
		}
		seen[key] = true
		out = append(out, path)
	}

	// What the user configured wins.
	if home := os.Getenv("JAVA_HOME"); home != "" {
		add(filepath.Join(home, "bin", "java"+exeSuffix))
	}
	if path, err := exec.LookPath("java"); err == nil {
		add(path)
	}

	// Then anything installed in the usual places, newest first so that a
	// machine with several JDKs does not have to probe all of them in a silly
	// order.
	for _, dir := range installRoots() {
		matches, err := filepath.Glob(filepath.Join(dir, "*", "bin", "java"+exeSuffix))
		if err != nil {
			continue
		}
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		for _, m := range matches {
			add(m)
		}
	}
	return out
}

// exeSuffix keeps the package testable on non-Windows CI even though the
// product targets Windows.
var exeSuffix = func() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}()

// installRoots are the directories Java installers use on Windows: Adoptium's
// machine-wide and per-user locations (winget can pick either), plus the path
// Oracle's installers use.
func installRoots() []string {
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
	return roots
}

// probe runs java -version and reads the feature version out of it.
func probe(path string) (Runtime, error) {
	// java -version writes to stderr, not stdout — the single most common
	// mistake when shelling out to it.
	out, err := exec.Command(path, "-version").CombinedOutput()
	if err != nil {
		return Runtime{}, fmt.Errorf("%s did not answer -version: %w", path, err)
	}
	major, version, err := parseVersion(string(out))
	if err != nil {
		return Runtime{}, err
	}
	return Runtime{Path: path, Major: major, Version: version}, nil
}

// parseVersion reads the feature version from java -version output, handling
// both the modern form (openjdk version "21.0.12") and the legacy 1.x form
// (java version "1.8.0_412", which is Java 8).
func parseVersion(output string) (int, string, error) {
	start := strings.Index(output, `version "`)
	if start < 0 {
		return 0, "", fmt.Errorf("could not read a Java version from: %s", firstLine(output))
	}
	rest := output[start+len(`version "`):]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return 0, "", fmt.Errorf("could not read a Java version from: %s", firstLine(output))
	}
	raw := rest[:end]

	numbers := strings.FieldsFunc(raw, func(r rune) bool { return !unicode.IsDigit(r) })
	if len(numbers) == 0 {
		return 0, "", fmt.Errorf("unreadable Java version %q", raw)
	}
	major, err := strconv.Atoi(numbers[0])
	if err != nil {
		return 0, "", fmt.Errorf("unreadable Java version %q", raw)
	}
	// "1.8.0_412" means Java 8: the feature version is the second component.
	if major == 1 && len(numbers) > 1 {
		if second, err := strconv.Atoi(numbers[1]); err == nil {
			major = second
		}
	}
	return major, raw, nil
}

func firstLine(s string) string {
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		return s[:i]
	}
	return s
}
