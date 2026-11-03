package javart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

type Runtime struct {
	Path    string
	Major   int
	Version string
}

func (r Runtime) String() string { return fmt.Sprintf("Java %s", r.Version) }

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

	if home := os.Getenv("JAVA_HOME"); home != "" {
		add(filepath.Join(home, "bin", "java"+exeSuffix))
	}
	if path, err := exec.LookPath("java"); err == nil {
		add(path)
	}

	for _, path := range platformJavaPaths() {
		add(path)
	}
	return out
}

var exeSuffix = func() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}()

func probe(path string) (Runtime, error) {

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
