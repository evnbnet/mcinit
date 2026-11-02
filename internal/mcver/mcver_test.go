package mcver

import "testing"

func TestCompareAcrossSchemes(t *testing.T) {

	tests := []struct {
		a, b string
		want int
	}{
		{"26.3", "1.21.11", 1},
		{"1.21.11", "26.3", -1},
		{"26.1", "1.8.8", 1},
		{"26.3", "26.1.2", 1},
		{"26.3", "26.3", 0},
		{"1.21.4", "1.21", 1},
		{"1.21", "1.21.4", -1},
		{"1.21.11", "1.21.9", 1},
		{"1.20.5", "1.20.4", 1},
		{"1.8.8", "1.8.8", 0},

		{"26.3", "26.3-rc-3", 1},
		{"26.3-rc-3", "26.3", -1},
		{"26.3-rc-3", "26.3-rc-2", 1},
		{"1.21.2-pre1", "1.21.2", -1},

		{"1.21.0", "1.21", 0},
	}
	for _, tt := range tests {
		got := Compare(MustParse(tt.a), MustParse(tt.b))
		if got != tt.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestRequiredJava(t *testing.T) {
	tests := []struct {
		version string
		want    int
	}{

		{"26.1", 25},
		{"26.2", 25},
		{"26.3", 25},
		{"26.3-rc-3", 25},
		{"27.1", 25},

		{"1.20.4", 17},
		{"1.20.5", 21},
		{"1.20.6", 21},
		{"1.21", 21},
		{"1.21.11", 21},
		{"1.20.5-pre1", 21},

		{"1.16.5", 8},
		{"1.17", 17},
		{"1.18", 17},
		{"1.19.4", 17},
		{"1.20", 17},
		{"1.12.2", 8},
		{"1.8.8", 8},
	}
	for _, tt := range tests {
		if got := MustParse(tt.version).RequiredJava(); got != tt.want {
			t.Errorf("%s.RequiredJava() = %d, want %d", tt.version, got, tt.want)
		}
	}
}

func TestYearlyAndStable(t *testing.T) {
	tests := []struct {
		version string
		yearly  bool
		stable  bool
	}{
		{"26.3", true, true},
		{"26.3-rc-3", true, false},
		{"1.21.4", false, true},
		{"1.21.2-pre1", false, false},
	}
	for _, tt := range tests {
		v := MustParse(tt.version)
		if v.Yearly() != tt.yearly {
			t.Errorf("%s.Yearly() = %v, want %v", tt.version, v.Yearly(), tt.yearly)
		}
		if v.Stable() != tt.stable {
			t.Errorf("%s.Stable() = %v, want %v", tt.version, v.Stable(), tt.stable)
		}
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "   ", "latest", "1.x", "1..2", "v1.21"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", s)
		}
	}
}

func TestSort(t *testing.T) {
	vs := []Version{
		MustParse("26.3"), MustParse("1.21.11"), MustParse("1.8.8"),
		MustParse("26.1"), MustParse("1.21.9"), MustParse("26.3-rc-3"),
	}
	Sort(vs)
	want := []string{"1.8.8", "1.21.9", "1.21.11", "26.1", "26.3-rc-3", "26.3"}
	for i, w := range want {
		if vs[i].Raw != w {
			t.Fatalf("Sort()[%d] = %s, want %s (full order: %v)", i, vs[i].Raw, w, vs)
		}
	}
}
