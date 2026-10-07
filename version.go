package tablo

import "github.com/nbyoung/tablo/internal/model"

// Version is the tablo release version. The tag v<Version> releases it, so a
// release bumps this constant first.
const Version = "0.1.0"

// The tableaux language versions the module accepts. The method's version rule
// accepts a project whose major version equals the tool's own and whose minor
// version does not exceed it; the patch version never matters. The rule itself
// lives in internal/model, where the Loader applies it.
const (
	AcceptedMajor = model.AcceptedMajor
	AcceptedMinor = model.AcceptedMinor
)

// Accepts reports whether the module accepts a project whose version.yaml
// states the tableaux language version v, written as major.minor.patch.
// A malformed version is not accepted.
func Accepts(v string) bool { return model.Accepts(v) }

// ParseVersion splits a tableaux language version into its numbers. It
// accepts exactly the form version.schema.yaml accepts: three decimal
// integers without leading zeros, joined by dots. ok is false otherwise.
func ParseVersion(v string) (major, minor, patch int, ok bool) { return model.ParseVersion(v) }
