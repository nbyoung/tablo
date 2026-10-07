// Package tablo is the Tableaux backend: a library that loads, validates,
// audits and derives a Tableaux project from a Git repository and serves
// every abstract view as data. Command tablo, in cmd/tablo, exposes the
// library as plumbing that reads a repository and emits data.
//
// The Loader, in internal/load, reads a project into the model of
// internal/model. Later tasks add the validator, the derivation, the audit
// and the views.
package tablo
