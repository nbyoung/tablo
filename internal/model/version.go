package model

import (
	"strconv"
	"strings"
)

// The tableaux language versions the module accepts. The method's version rule
// accepts a project whose major version equals the tool's own and whose minor
// version does not exceed it; the patch version never matters.
const (
	AcceptedMajor = 0
	AcceptedMinor = 4
)

// Release is a language version, as major, minor and patch.
type Release struct{ Major, Minor, Patch int }

// The releases from which a reading of the method changes. A commit whose
// version.yaml states a language before one reads as that language did.
var (
	// ModelTrailer introduced the Model: trailer: H6 exempts a commit before it.
	ModelTrailer = Release{0, 2, 1}
	// AgentIdentity introduced agents.yaml, the pairing of an email with a
	// model and the nearest person as an agent's reviewer: before it a
	// junction is an agent's only where it states a model, an agent's
	// default reviewer is the assignee, and H7 exempts the commit.
	AgentIdentity = Release{0, 4, 0}
)

// Before reports whether version.yaml states a well-formed language before
// r. A nil or malformed version precedes nothing, so the newest reading
// applies to it.
func (v *Version) Before(r Release) bool {
	if v == nil || !v.WellFormed {
		return false
	}
	return Release{v.Major, v.Minor, v.Patch}.less(r)
}

// Before reports whether the text of a language version, as version.yaml
// writes it, names one before r; "" and a malformed text precede nothing.
func Before(text string, r Release) bool {
	major, minor, patch, ok := ParseVersion(text)
	return ok && Release{major, minor, patch}.less(r)
}

// less orders two releases by major, minor and patch.
func (a Release) less(b Release) bool {
	switch {
	case a.Major != b.Major:
		return a.Major < b.Major
	case a.Minor != b.Minor:
		return a.Minor < b.Minor
	}
	return a.Patch < b.Patch
}

// Accepts reports whether the module accepts a project whose version.yaml
// states the tableaux language version v, written as major.minor.patch.
// A malformed version is not accepted.
func Accepts(v string) bool {
	major, minor, _, ok := ParseVersion(v)
	return ok && major == AcceptedMajor && minor <= AcceptedMinor
}

// ParseVersion splits a tableaux language version into its numbers. It
// accepts exactly the form version.schema.yaml accepts: three decimal
// integers without leading zeros, joined by dots. ok is false otherwise.
func ParseVersion(v string) (major, minor, patch int, ok bool) {
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var n [3]int
	for i, part := range parts {
		x, valid := number(part)
		if !valid {
			return 0, 0, 0, false
		}
		n[i] = x
	}
	return n[0], n[1], n[2], true
}

// number parses a decimal integer of the form 0 or [1-9][0-9]*.
func number(s string) (int, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}
