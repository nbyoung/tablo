package tablo

import (
	"os"
	"strings"
	"testing"
)

// TestAccepts covers the version rule: major 0 and minor at most 1 pass, a
// higher minor or another major fails, and every malformed string fails.
func TestAccepts(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"0.0.0", true},
		{"0.1.0", true},
		{"0.1.9", true},
		{"0.0.12", true},
		{"0.2.0", true},
		{"0.2.7", true},
		{"0.3.0", false},
		{"1.0.0", false},
		{"1.1.0", false},
		{"0.1", false},
		{"0.1.0.0", false},
		{"", false},
		{"0.01.0", false},
		{"v0.1.0", false},
		{"0.1.0-rc1", false},
		{"0.+1.0", false},
		{"0.1.x", false},
		{" 0.1.0", false},
	}
	for _, tt := range tests {
		if got := Accepts(tt.version); got != tt.want {
			t.Errorf("Accepts(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version string
		major   int
		minor   int
		patch   int
		ok      bool
	}{
		{"0.1.0", 0, 1, 0, true},
		{"12.34.56", 12, 34, 56, true},
		{"0.1", 0, 0, 0, false},
		{"a.b.c", 0, 0, 0, false},
	}
	for _, tt := range tests {
		major, minor, patch, ok := ParseVersion(tt.version)
		if major != tt.major || minor != tt.minor || patch != tt.patch || ok != tt.ok {
			t.Errorf("ParseVersion(%q) = %d, %d, %d, %v; want %d, %d, %d, %v",
				tt.version, major, minor, patch, ok, tt.major, tt.minor, tt.patch, tt.ok)
		}
	}
}

// TestAcceptsOwnPlan checks that the module accepts the version of its own
// Tableaux plan. version.yaml holds flat scalar fields, so a line scan
// suffices here; the loader task parses it properly.
func TestAcceptsOwnPlan(t *testing.T) {
	data, err := os.ReadFile(".tableaux/version.yaml")
	if err != nil {
		t.Fatal(err)
	}
	v := ""
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "tableaux:"); ok {
			v = strings.TrimSpace(rest)
		}
	}
	if !Accepts(v) {
		t.Errorf("Accepts(%q) = false for the module's own plan", v)
	}
}
