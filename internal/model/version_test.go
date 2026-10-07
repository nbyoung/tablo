package model

import "testing"

// TestAccepts covers the version rule where it now lives: major 0 and minor
// at most 3 pass, a higher minor or another major fails, and a malformed
// string fails. The root package's version_test.go covers its wrappers.
func TestAccepts(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"0.0.0", true},
		{"0.3.0", true},
		{"0.3.4", true},
		{"0.4.0", false},
		{"1.0.0", false},
		{"0.1", false},
		{"", false},
		{"0.01.0", false},
		{"v0.1.0", false},
		{" 0.1.0", false},
	}
	for _, tt := range tests {
		if got := Accepts(tt.version); got != tt.want {
			t.Errorf("Accepts(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}

func TestParseVersion(t *testing.T) {
	major, minor, patch, ok := ParseVersion("12.34.56")
	if major != 12 || minor != 34 || patch != 56 || !ok {
		t.Errorf("ParseVersion(12.34.56) = %d, %d, %d, %v", major, minor, patch, ok)
	}
	if _, _, _, ok := ParseVersion("a.b.c"); ok {
		t.Error("ParseVersion(a.b.c) is ok")
	}
}
