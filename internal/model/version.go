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
	AcceptedMinor = 3
)

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
