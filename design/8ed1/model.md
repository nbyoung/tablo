# Design 8ed1: the declarations

The Go surface of [design 8ed1](../8ed1.md). The task adds one file to the public package `view`, two files to `internal/views`, and two fields to `views.Query`. A trial outside the repository compiles these blocks as they stand against `main` at `85c3e85`, with the bodies the design states, and produces the files under [`golden/`](golden/).

| Block                                                      | Lands as                                  |
|------------------------------------------------------------|-------------------------------------------|
| [`view`: the history](#view-the-history)                   | `view/history.go`                         |
| [`internal/views`: the query](#internalviews-the-query)    | `internal/views/views.go`                 |
| [`internal/views`: the two functions](#internalviews-the-two-functions) | `internal/views/history.go`, `internal/views/audit.go` |
| [`snapshot.go` of 4ed9](#snapshotgo-of-4ed9)               | Two entries of `derivers`                 |

A type the first block names and does not declare stands on `main`: `Legend`, `TaskRef`, `Status`, `Event`, `ModelCheck`, `Commit`, `Command`. The audit's types stand in `view/audit.go` on `main` (task `dada`), and this task declares none of them again.

## `view`: the history

```go
package view

// History is the history view: what each commit does to each task in view,
// oldest first. The counts cover the project's own lines, and they are the
// same at every level.
type History struct {
	Legend  Legend       `json:"legend"`
	Subject TaskRef      `json:"subject"` // the task whose subtree is in view: the task parameter, the root by default
	Events  int          `json:"events"`  // the events the project's own lines hold
	Commits int          `json:"commits"` // the distinct commits of those lines
	Kinds   []EventCount `json:"kinds"`   // the six kinds of SYNTAX.md#history, in its order, zeros included
	Days    []Day        `json:"days"`    // one per author date of those lines, in the order the lines first name it
	Lines   []Line       `json:"lines"`   // oldest first; a subproject's lines from detail on
	*HistoryProvenance
}

// HistoryProvenance is what the history adds at provenance. There each line
// also carries its LineProvenance.
type HistoryProvenance struct {
	Commands []Command `json:"commands"` // the two git log commands of README.md#history, over the subtree and the range
}

// EventCount counts the events of one kind.
type EventCount struct {
	Kind  string `json:"kind"` // task, authorised, status, reaffirmed, reviewed or pin
	Count int    `json:"count"`
}

// Day counts the project's own lines of one author date.
type Day struct {
	Date    string       `json:"date"` // YYYY-MM-DD
	Events  int          `json:"events"`
	Commits int          `json:"commits"`
	Kinds   []EventCount `json:"kinds"` // the kinds the day holds, in the order of SYNTAX.md#history; no zero
}

// Line is what one commit does to one task: its events there, each in the
// shape the task definition gives an event. The events of a line share the
// commit, the author, the task and, for a line taken from a subproject, the
// subproject task.
type Line struct {
	Task     TaskRef `json:"task"`     // the task of the project in view
	Proposal bool    `json:"proposal"` // off the trunk: the trunk's tip does not reach the commit
	Events   []Event `json:"events"`   // never empty; in the order of the kinds
	*LineDetail
	*LineProvenance
}

// LineDetail is what a line adds at detail.
type LineDetail struct {
	After        *Status     `json:"after"`         // the task's status as the files stand at the commit; a taken line's, of the subproject task; null where those files lack the task
	Unchanged    bool        `json:"unchanged"`     // the files at one of the commit's parents state the same status
	SubjectAfter *Status     `json:"subject_after"` // the subject's status at the commit, on a line of another task of the project; else null
	Effect       string      `json:"effect"`        // what the line does to the task's acceptance or review; "" for nothing
	Model        string      `json:"model"`         // the commit's first Model: trailer; "" for none
	Check        *ModelCheck `json:"check"`         // the model check of the line's status or reaffirmed event; null where it reads no model
	Committer    string      `json:"committer"`     // the committer's email where it differs from the author's; else ""
}

// LineProvenance is what a line adds at provenance.
type LineProvenance struct {
	Commit *Commit `json:"commit"` // the commit, of the project or of the subproject the line comes from
}
```

The words, each an `enum` in [`data.schema.yaml`](data.schema.yaml): an event's `kind` stays `view_event_kind`, as on `main`; `effect` is `view_line_effect`.

| `effect`             | The line holds, first rule that holds                                                                          |
|----------------------|----------------------------------------------------------------------------------------------------------------|
| `authorised_commit`, `authorised_merge`, `authorised_trailer` | `task` or `authorised`, and `AuthorisationAt(task, commit)` is authorised with this commit deciding, by `ByCommit`, `ByMerge` or `ByTrailer` |
| No word of acceptance | `task` or `authorised`, and that authorisation is proposed with `Why` `NoHistory` or `NoTrunk`: the rows below apply |
| `proposed`           | `task`, and that authorisation is proposed, or nil                                                             |
| `none`               | `authorised` and no `task`: the trailer decides nothing                                                        |
| `handoff`            | `status`, recording the reason `review`                                                                        |
| `accepts`, `stands`, `none` | `reviewed`: the `Effect` of its first reviewed event, `Accepts`, `AtDefined`, or any other                |
| `""`                 | Anything else                                                                                                  |

A taken line reads `AuthorisationAt` of the subproject's facts, `Facts.Sub(Link)`.

## `internal/views`: the query

```go
// Query is what a view takes: the parameters, checked and resolved.
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
```

`views.go` gains the import of `internal/model`. `Stale` and `Now` are no fields of `Query`: they are `audit.Input`'s, and the report the audit view reads already holds them (dada A1).

## `internal/views`: the two functions

```go
// History returns the history of the subtree under q.Task, the root when it
// is "": each event of f.History() whose task lies there, whose author is
// q.Person when it names one, and that the range from q.From holds when it
// names one, joined into lines. An event taken from a subproject joins from
// Detail on. It fails in no case.
func History(f *derive.Facts, q Query) *view.History

// Audit returns the audit of the subtree under q.Task, the root when it is
// "", from r, the report of the project f holds: under the root every
// finding, under another task those whose task lies in its subtree; of them
// those whose resolver is q.Person when it names one. The counts, Most and
// Silent read every finding kept; the rows list those at or above
// q.Minimum. r is not nil.
func Audit(f *derive.Facts, r *audit.Report, q Query) *view.Audit
```

The unexported surface, each a method of 493e's `builder` or a plain function, as the files of 886d hold theirs:

```go
// lineKey is what the events of one line share: the commit, the task, and
// for a taken event the subproject task and the link's url.
type lineKey struct{ commit, task, sub, url string }

// inRange reports whether the range from q.From holds an event: always
// without a start; for an event of the project, when f.Log().Reaches(From,
// commit) is false; for a taken event, when the commit the link reads at the
// start does not reach it in the subproject's pass, or when no link reads a
// commit at the start.
func (b builder) inRange(e *derive.Event) bool

// pinnedAt returns the commit a task's link to a subproject reads as the
// files stand at a commit of the project: the Where.Commit of the facts of
// the snapshot of f.At(commit).Snapshots(task) with the link's url; "" for
// none.
func (b builder) pinnedAt(commit, task string, link *model.Link) string

// line returns one line from its events, with the subject's id.
func (b builder) line(subject string, events []*derive.Event) view.Line

// effect returns a line's effect by the table of model.md, reading the
// facts of the project the line comes from.
func effect(f *derive.Facts, task string, events []*derive.Event) string

// count fills Events, Commits, Kinds and Days from the project's own lines.
func (b builder) count(h *view.History)

// historyCommands returns the two commands of the history's provenance.
func (b builder) historyCommands(top string) []view.Command

// finding returns one row of the audit, through the builders, as dada's
// model.md, "From a finding to the data", states.
func (b builder) finding(row audit.Row) view.Finding

// findingFacts returns the typed facts of one finding, and false where it
// carries none.
func (b builder) findingFacts(x *audit.Finding) (view.FindingFacts, bool)
```

`event(e)` of `task.go` builds each event of a line, `reading(v)` each check, and `status`, `commit`, `ref`, `legend`, `junction`, `requirement`, `authorisation` and `linkages` the shared types; the two files write none again.

## `snapshot.go` of 4ed9

```go
History: func(_ context.Context, s *Snapshot, p Params) (any, error) {
	return views.History(s.facts, query(p)), nil
},
Audit: func(_ context.Context, s *Snapshot, p Params) (any, error) {
	r, err := audit.Run(audit.Input{Facts: s.facts, Diagnostics: s.diagnostics, Now: p.Now, Stale: p.Stale})
	if err != nil {
		return nil, usage(err) // a *DateError as a *UsageError (dada A1)
	}
	if r == nil {
		return nil, nil // the Derivation refuses: data is null
	}
	return views.Audit(s.facts, r, query(p)), nil
},
```

`query(p)` copies `From` as the commit `snapshot.go` resolves the range's start to, and maps `p.Minimum` to `model.Warning` when it is `""`.
