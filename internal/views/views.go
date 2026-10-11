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

// Level is a level of disclosure; each includes the one before.
type Level int

// The levels.
const (
	Glance Level = iota
	Detail
	Provenance
)

// Query is what a view takes: the parameters, checked and resolved. Task
// 886d adds Window, Columns, Historical and Brief; task 8ed1 adds From,
// Stale and Now.
type Query struct {
	Task     string // a task of the project; "" is the root and, on the gate definition, no task
	Person   string // an email; "" is nobody
	Level    Level
	Proposed bool // authority: proposed tasks only
}
