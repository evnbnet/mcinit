package mcver

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type Version struct {
	Raw   string
	parts []int
	pre   string
}

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

func MustParse(s string) Version {
	v, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return v
}

func (v Version) Yearly() bool { return len(v.parts) > 0 && v.parts[0] != 1 }

func (v Version) Stable() bool { return v.pre == "" }

func (v Version) String() string { return v.Raw }

func (v Version) at(i int) int {
	if i < len(v.parts) {
		return v.parts[i]
	}
	return 0
}

func Compare(a, b Version) int {

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

func Sort(vs []Version) {
	sort.Slice(vs, func(i, j int) bool { return Compare(vs[i], vs[j]) < 0 })
}

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

func (v Version) core() Version { return Version{Raw: v.Raw, parts: v.parts} }
