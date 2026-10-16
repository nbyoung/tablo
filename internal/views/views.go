// Package views builds each view's data from the derived facts. It selects,
// counts and words; it derives no fact and reads no file, process, clock or
// environment.
//
// Each view is one function of the facts and a Query, and returns a value of
// the public package view. A view function fails in no case: its caller
// guarantees that the Derivation does not refuse the facts and that the task
// the query names is one of the project. The types every view shares come
// from the methods of builder, one per type, and the view tasks that follow
// call them and write none again.
package views

import "github.com/nbyoung/tablo/internal/model"

// Level is a level of disclosure; each includes the one before.
type Level int

// The levels.
const (
	Glance Level = iota
	Detail
	Provenance
)

// BriefOf names the one queue item the queue writes as a brief.
type BriefOf struct{ Task, Gate string }

// Query is what a view takes: the parameters, checked and resolved. The
// audit's date and age are no fields of it: audit.Input takes them, and the
// report the audit view reads holds them.
type Query struct {
	Task       string // a task of the project; "" is the root, no task on the gate definition, and the person form on context
	Person     string // an email; "" is nobody
	Level      Level
	Proposed   bool           // authority: proposed tasks only
	Window     int            // tableau, context: the columns either side of the next gates
	Columns    []string       // tableau, context: the gates to show, each a gate of the project, in place of the window; empty for none
	Historical bool           // tableau, context: show the marks of historical junctions
	Brief      *BriefOf       // queue: write this one item as a brief; its task and gate are of the project
	From       string         // history: the commit the range starts after, in full, one the pass holds; "" for the whole history
	Minimum    model.Severity // audit: the lowest severity listed; the caller resolves the default, model.Warning
}
