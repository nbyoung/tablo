package model

import "testing"

// TestAccepts covers the version rule where it now lives: major 0 and minor
// at most 4 pass, a higher minor or another major fails, and a malformed
// string fails. The root package's version_test.go covers its wrappers.
func TestAccepts(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"0.0.0", true},
		{"0.3.0", true},
		{"0.3.4", true},
		{"0.4.0", true},
		{"0.4.9", true},
		{"0.5.0", false},
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

// TestBefore is T1: a version precedes a release by major, minor and patch,
// and a nil or malformed version precedes nothing.
func TestBefore(t *testing.T) {
	tests := []struct {
		text string
		r    Release
		want bool
	}{
		{"0.2.0", ModelTrailer, true},
		{"0.2.1", ModelTrailer, false},
		{"0.1.9", ModelTrailer, true},
		{"0.3.0", ModelTrailer, false},
		{"0.3.2", AgentIdentity, true},
		{"0.4.0", AgentIdentity, false},
		{"0.4.1", AgentIdentity, false},
		{"1.0.0", AgentIdentity, false},
		{"", AgentIdentity, false},
		{"0.3", AgentIdentity, false},
		{"v0.3.2", AgentIdentity, false},
	}
	for _, tt := range tests {
		if got := Before(tt.text, tt.r); got != tt.want {
			t.Errorf("Before(%q, %v) = %v, want %v", tt.text, tt.r, got, tt.want)
		}
		major, minor, patch, ok := ParseVersion(tt.text)
		v := &Version{Major: major, Minor: minor, Patch: patch, WellFormed: ok}
		if got := v.Before(tt.r); got != tt.want {
			t.Errorf("(%q).Before(%v) = %v, want %v", tt.text, tt.r, got, tt.want)
		}
	}
	var none *Version
	if none.Before(AgentIdentity) {
		t.Error("a nil version precedes 0.4.0")
	}
}
