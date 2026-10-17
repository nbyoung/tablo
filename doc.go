// Package tablo is the Tableaux backend: a library that loads, validates,
// audits and derives a Tableaux project from a Git repository and serves
// every abstract view as data. Command tablo, in cmd/tablo, is a thin shell
// over the library: plumbing that reads a repository and emits data.
//
// A caller opens a Repository once and loads a Snapshot per change: Load
// reads the files and the history once, and the snapshot then derives every
// view from memory. View returns one view's Envelope, Validate the
// diagnostics and CheckTrailer the check of a commit trailer; the command
// encodes the same envelope as JSON or YAML, so the two cannot disagree. The
// data of each view has its types in package view.
//
// The library reads neither the environment nor the clock: the caller
// passes the viewer and the date staleness counts from in Params.
package tablo
