// Package schemas embeds the tableaux schema files that the module validates
// a project against.
package schemas

import "embed"

// FS holds the schema files copied from the tableaux repository, one per
// Tableaux file kind: version, gates, task, status and history, each named
// <kind>.schema.yaml after its $id. The tableaux schema-files task (e3cb)
// produces them; until a copy lands here the pattern matches nothing and this
// package does not build. README.md in this directory records the copy.
//
//go:embed *.schema.yaml
var FS embed.FS
