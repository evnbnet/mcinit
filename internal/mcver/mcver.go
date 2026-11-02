// Package mcver parses and compares Minecraft version numbers.
//
// Minecraft uses two incompatible naming schemes: the classic 1.x line
// (1.21.4, 1.8.8) and the year-based line introduced with 26.1 (26.3,
// 26.3-rc-3). They cannot be compared as plain semver — 26.3 ships *after*
// 1.21.11 even though 26 > 1 — so the scheme is detected first and always
// dominates the comparison.
package mcver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Version is a parsed Minecraft version number.
type Version struct {
	Raw   string
	parts []int
	pre   string // pre-release suffix ("rc-3", "pre1"); empty for stable
}

// Parse reads a version string such as "1.21.4", "26.3" or "26.3-rc-3".
func Parse(s string) (Version, error) {
	v := Version{Raw: strings.TrimSpace(s)}
	if v.Raw == "" {
		return v, fmt.Errorf("empty version")
	}

	num := v.Raw
	if i := strings.IndexAny(num, "-+"); i >= 0 {
		v.pre, num = num[i+1:], num[:i]
	}

	for _, field := range strings.Split(num, ".") {
		n, err := strconv.Atoi(field)
		if err != nil {
			return Version{Raw: v.Raw}, fmt.Errorf("invalid version %q", s)
		}
		v.parts = append(v.parts, n)
	}
	return v, nil
}

// MustParse is Parse for literals known to be valid; it panics otherwise.
func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

// Yearly reports whether the version uses the year-based scheme (26.x and up)
// rather than the classic 1.x scheme.
func (v Version) Yearly() bool { return len(v.parts) > 0 && v.parts[0] != 1 }

// Stable reports whether this is a full release rather than a pre-release,
// release candidate or snapshot.
func (v Version) Stable() bool { return v.pre == "" }

func (v Version) String() string { return v.Raw }

func (v Version) at(i int) int {
	if i < len(v.parts) {
		return v.parts[i]
	}
	return 0
}

// Compare returns -1 if a precedes b, +1 if a follows b, and 0 if equal.
func Compare(a, b Version) int {
	// The scheme outranks everything: every year-based version is newer than
	// every 1.x version.
	if a.Yearly() != b.Yearly() {
		if a.Yearly() {
			return 1
		}
		return -1
	}

	n := len(a.parts)
	if len(b.parts) > n {
		n = len(b.parts)
	}
	for i := 0; i < n; i++ {
		switch av, bv := a.at(i), b.at(i); {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}

	// Same number: a stable release outranks a pre-release of it.
	switch {
	case a.pre == b.pre:
		return 0
	case a.pre == "":
		return 1
	case b.pre == "":
		return -1
	}
	return strings.Compare(a.pre, b.pre)
}

// Sort orders versions oldest to newest.
func Sort(vs []Version) {
	sort.Slice(vs, func(i, j int) bool { return Compare(vs[i], vs[j]) < 0 })
}

// RequiredJava returns the lowest Java major version that can run a server of
// this Minecraft version.
//
//	26.1 and up      Java 25
//	1.20.5 – 1.21.x  Java 21
//	1.17 – 1.20.4    Java 17
//	older            Java 8
func (v Version) RequiredJava() int {
	if v.Yearly() {
		return 25
	}
	switch {
	case Compare(v.core(), MustParse("1.20.5")) >= 0:
		return 21
	case Compare(v.core(), MustParse("1.17")) >= 0:
		return 17
	default:
		return 8
	}
}

// core drops the pre-release suffix so that 1.20.5-pre1 maps to the same Java
// requirement as 1.20.5.
func (v Version) core() Version { return Version{Raw: v.Raw, parts: v.parts} }
