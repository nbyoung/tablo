// Package model holds the typed model of a Tableaux project as its files
// state it: the version, the gating, every task and status as written, every
// subproject a file links to, and the position of every value. It holds the
// version rule too. The package does no input or output: the Loader in
// internal/load builds the model, and the validator and the derivation are
// functions of it.
package model

import "sort"

// File is one file of the layout that the Loader read.
type File struct {
	Path string // below .tableaux
	Root *Value // nil when the file is empty or does not parse
}

// Project is one Tableaux project as its files state it at one place and moment.
type Project struct {
	Where       Location
	Exists      bool             // a .tableaux directory is there
	Files       []*File          // by path
	Stray       []string         // paths under .tableaux the layout does not name, by path
	Version     *Version         // nil when version.yaml is absent or does not parse
	Gating      *Gating          // nil when gates.yaml is absent or does not parse
	Tasks       map[string]*Task // by id: the file's name without .yaml, whatever it is
	Statuses    map[string]*Status
	Links       []*Link      // each distinct subproject the files name, by URL then commit
	Diagnostics []Diagnostic // the Loader's own, by file, line, column, code
}

// TaskIDs returns the keys of Tasks in byte order.
func (p *Project) TaskIDs() []string {
	ids := make([]string, 0, len(p.Tasks))
	for id := range p.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Location says where a project was read.
type Location struct {
	GitDir   string // the repository's common Git directory, absolute: the repository's identity
	Top      string // the working tree's root, absolute; "" in a bare repository
	Dir      string // the directory that holds .tableaux, relative to the repository root; "" at the root
	Ref      string // the revision asked for; "" for the working tree
	Commit   string // the commit read, in full; HEAD's for the working tree; "" for a bare tree or an unborn branch
	Worktree bool   // the files come from disk
}

// Version is version.yaml.
type Version struct {
	File                *File
	Tableaux, Trunk     StrField
	Major, Minor, Patch int
	WellFormed          bool // Tableaux is major.minor.patch
	Accepted            bool // WellFormed, and the module accepts it
}

// Gating is gates.yaml. Each list is in file order and keeps an entry for every item written.
type Gating struct {
	File    *File
	Gates   []*Gate
	States  []*State
	Reasons []*Reason
}

// Gate is one entry under gates.
type Gate struct {
	Key, Symbol, Name, Criteria StrField
	Node                        *Value
}

// State is one entry under states.
type State struct {
	Key, Symbol, Synopsis StrField
	Severity              IntField
	Node                  *Value
}

// Reason is one entry under reasons.
type Reason struct {
	Key, Symbol, Synopsis StrField
	Node                  *Value
}

// Task is tasks/<id>.yaml as written. Nothing here is inherited, defaulted or resolved.
type Task struct {
	ID                           string
	File                         *File
	Title, Description, Assignee StrField
	References                   []*Reference
	Requires                     []*Requirement
	Junctions                    []*Junction // in file order
	Parent                       *Parent     // nil exactly when the file has no parent key
}

// Reference is one entry under references, on a task or a junction.
type Reference struct {
	URL, Text StrField
	Node      *Value
}

// Requirement is one entry under requires.
type Requirement struct {
	ID             StrField
	Subproject     *Subproject
	From, To, Text StrField
	Node           *Value
}

// Parent is the parent field of a task.
type Parent struct {
	ID    StrField
	Order IntField
	Node  *Value
}

// Junction is one entry under junctions, with every field it states, whatever its kind.
type Junction struct {
	Gate                         string // the key as written
	KeyPos                       Pos
	Contributor, Model, Reviewer StrField
	References                   []*Reference
	Subproject                   *Subproject
	Applies                      BoolField
	Node                         *Value
}

// JunctionKind is what the fields of an entry make it.
type JunctionKind int

// The kinds of a junction entry.
const (
	Plain         JunctionKind = iota // no subproject and no applies; {} is plain
	Recursive                         // subproject alone
	NotApplicable                     // applies alone, whatever its value
	Mixed                             // fields of more than one kind
)

// String returns the kind's name as the corpus writes it.
func (k JunctionKind) String() string {
	switch k {
	case Plain:
		return "plain"
	case Recursive:
		return "recursive"
	case NotApplicable:
		return "not_applicable"
	}
	return "mixed"
}

// Kind returns the kind the entry's fields make it. The fields of a plain
// junction are contributor, model, reviewer and references.
func (j *Junction) Kind() JunctionKind {
	plain := j.Contributor.Node != nil || j.Model.Node != nil || j.Reviewer.Node != nil ||
		j.Node.Get("references") != nil
	recursive := j.Node.Get("subproject") != nil
	notApplicable := j.Applies.Node != nil
	switch {
	case !recursive && !notApplicable:
		return Plain
	case recursive && !notApplicable && !plain:
		return Recursive
	case notApplicable && !recursive && !plain:
		return NotApplicable
	}
	return Mixed
}

// Subproject is a subproject field, on a junction or a requirement, and what it leads to.
type Subproject struct {
	URL, ID, Commit StrField
	Node            *Value
	Link            *Link // nil when URL is not a scalar
}

// Status is status/<id>.yaml as written.
type Status struct {
	ID                        string
	File                      *File
	Gate, State, Reason, Note StrField
}
