# Design dada: the declarations

The Go surface of [design dada](../dada.md), as the owner's review of 2026-10-07 leaves it. Two blocks: the data a front end reads, in the public package `view` of [design 493e](https://github.com/nbyoung/tablo/blob/main/design/493e/model.md), and the functions, in `internal/audit`. A trial outside the repository compiles both blocks as they stand, the first beside 493e's `view/shared.go` and `view/task.go`, the second against `main` at `0883ec1`, with empty bodies; the first trial, of the first draft, has bodies and produces what the drafts keep from it.

| Block                                              | Lands as                              |
|----------------------------------------------------|---------------------------------------|
| [`view`: the audit](#view-the-audit)               | `view/audit.go`                       |
| [`internal/audit`](#internalaudit)                 | `internal/audit/{audit,rules,facts,report}.go` |

A type the first block names and does not declare is one of 493e: `Legend`, `TaskRef`, `Reference`, `Commit`, `Command`, `Junction`, `Status`, `Requirement`, `Authorisation`, `Linkage`, `ModelCheck`.

## `view`: the audit

```go
package view

// Audit is the audit view. The three counts and Most cover every finding in
// view, whatever the minimum; Findings lists the rows at or above it.
type Audit struct {
	Legend      Legend     `json:"legend"`
	Errors      int        `json:"errors"`
	Warnings    int        `json:"warnings"`
	Information int        `json:"information"`
	Most        *Resolving `json:"most"`     // who has the most to resolve; null when no finding asks an act of a person
	Findings    []Finding  `json:"findings"` // the rows at or above the minimum, in the order of the design
	*AuditDetail
}

// AuditDetail is what the audit adds at detail. There each finding also
// carries its FindingDetail, and at provenance its FindingProvenance.
type AuditDetail struct {
	Silent []Silence `json:"silent"` // the kinds examined that no finding in view has
}

// Finding is one row of the audit: the tasks that one kind hits at one gate
// for one reason.
type Finding struct {
	Key      string    `json:"key"`      // a rule id of RULES.md, or proposed, stale or sole_review
	Rule     string    `json:"rule"`     // the rule id; "" for the three kinds no rule states
	Severity string    `json:"severity"` // error, warning or information, as the audit lists it
	Demoted  bool      `json:"demoted"`  // the rule's own severity is graver: the junction is historical, or the commit is old
	Kind     string    `json:"kind"`     // the rule's title, or the kind's: tablo's words
	Tasks    []TaskRef `json:"tasks"`    // in display order; empty for a finding that names none
	Gate     string    `json:"gate"`     // "" when the finding names none
	Act      string    `json:"act"`      // one of the ten acts
	Resolver string    `json:"resolver"` // an email; "" when the files name nobody
	*FindingDetail
	*FindingProvenance
}

// FindingDetail is what a finding adds at detail.
type FindingDetail struct {
	Files   []string `json:"files"`   // relative to the project directory: .tableaux/tasks/9f31.yaml
	Message string   `json:"message"` // a rule finding: the diagnostic's, byte for byte; tablo's words
}

// FindingProvenance is what a finding adds at provenance.
type FindingProvenance struct {
	Sentence string         `json:"sentence"` // the sentence of the method the kind rests on
	Source   Reference      `json:"source"`   // where the sentence stands; the url is absolute
	Facts    []FindingFacts `json:"facts"`    // one per task of the row that has a typed fact, in the order of Tasks
	Commits  []Commit       `json:"commits"`  // the commits involved, each once, the oldest first
	Commands []Command      `json:"commands"` // what shows the finding, then what resolves it
}

// FindingFacts is what the Derivation knows of one task of a finding, each
// fact in the type every other view gives it. A fact the finding does not
// carry is null.
type FindingFacts struct {
	Task          string         `json:"task"`          // the task's id
	Junction      *Junction      `json:"junction"`      // the junction at the finding's gate, with From
	Status        *Status        `json:"status"`        // the task's status
	Requirement   *Requirement   `json:"requirement"`   // the requires entry the finding stands at
	Authorisation *Authorisation `json:"authorisation"` // a proposed task: who decides, and why nothing does
	Linkage       *Linkage       `json:"linkage"`       // the subproject the junction reads, with its pin
	Model         *ModelCheck    `json:"model"`         // the commit's Model: trailer against the model stated
}

// Resolving is the number of findings that ask an act of one person.
type Resolving struct {
	Email string `json:"email"`
	Count int    `json:"count"`
}

// Silence is a kind the audit examines and finds nothing of.
type Silence struct {
	Key  string `json:"key"`
	Rule string `json:"rule"`
	Kind string `json:"kind"`
}
```

The closed sets, each an `enum` in the schema (493e decision 11). The `$defs` join `envelope.schema.yaml` under the prefix `view_`, with this task; the `allOf` entries that tie a level to its keys join with the view (8ed1).

| Key                         | Words                                                                                                              |
|-----------------------------|--------------------------------------------------------------------------------------------------------------------|
| `severity`                  | `error`, `warning`, `information`                                                                                   |
| `act`                       | `authorise`, `review`, `reaffirm`, `record_handoff`, `clear_handoff`, `revise`, `move_pin`, `checkout`, `await`, `none` |
| `key`                       | A rule id, by the pattern `^[A-Z][0-9]+$`, or one of `proposed`, `stale`, `sole_review`                             |
| `params.minimum` (4ed9)     | `error`, `warning`, `information`                                                                                   |

## `internal/audit`

```go
// Package audit turns the Validator's diagnostics and three facts of the
// Derivation into findings: each names its task, its gate, its commits, the
// act that resolves it and the person who takes it, and holds the facts of
// the Derivation it rests on. It is a pure function of its input: it runs no
// process and reads no file, clock or environment.
package audit

import (
	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
)

// Act is the kind of action that resolves a finding.
type Act string

// The acts: the eight of VIEWS.md#audit, then the two this design adds.
const (
	Authorise     Act = "authorise"      // an authority accepts the task
	Review        Act = "review"         // the reviewer accepts the junction
	Reaffirm      Act = "reaffirm"       // the recorder confirms the status
	RecordHandoff Act = "record_handoff" // the contributor states the reason review
	ClearHandoff  Act = "clear_handoff"  // the contributor records the status at the gate
	Revise        Act = "revise"         // someone revises the file
	MovePin       Act = "move_pin"       // someone moves the pin or the commit field
	Checkout      Act = "checkout"       // whoever holds the clone gives the tool the subproject
	Await         Act = "await"          // the originating task passes the gate a requirement names
	None          Act = "none"           // nothing resolves it
)

// The keys of the three kinds of finding that no rule states.
const (
	Proposed   = "proposed"
	Stale      = "stale"
	SoleReview = "sole_review"
)

// DefaultStale is the age in days beyond which a status is stale.
const DefaultStale = 7

// Input is what one audit reads.
type Input struct {
	Facts       *derive.Facts      // the facts of the project in view
	Diagnostics []model.Diagnostic // the Loader's and the Validator's, as snapshot.go sorts them
	Now         string             // the date staleness counts from, YYYY-MM-DD; required
	Stale       int                // the age in days; 0 is DefaultStale
}

// A DateError reports an Input whose Now is empty or no date of the form
// YYYY-MM-DD. snapshot.go returns it as a *tablo.UsageError.
type DateError struct{ Now string }

func (e *DateError) Error() string

// Run audits a project. It returns a *DateError for a Now that is no date,
// before it reads anything else; nil and no error when in.Facts is nil or
// refuses; and never fails or panics otherwise. It changes nothing of its
// input, and the findings point into in.Diagnostics and into the facts.
func Run(in Input) (*Report, error)

// Report is the audit of one project at one source. It is immutable.
type Report struct {
	Now      string    // the date staleness counts from, as given
	Stale    int       // the age in days, as resolved
	Findings []Finding // one per diagnostic, proposed task, stale status and sole review, in the order of the design
	// and, unexported, the facts and the kinds the run examines
}

// Keep returns the report with the findings keep accepts, in their order. Now,
// Stale and the kinds examined stay.
func (r *Report) Keep(keep func(*Finding) bool) *Report

// Rows returns the findings whose listed severity is minimum or graver, with
// those that one kind hits at one gate for one reason joined in one row, in
// the order of the first finding of each.
func (r *Report) Rows(minimum model.Severity) []Row

// Counts returns the findings by listed severity, one per finding of
// Findings, whatever a caller's minimum.
func (r *Report) Counts() Counts

// Most returns the person the most findings ask an act of, the first in the
// findings' order among equals, or nil when none does. A finding whose act
// is None asks nothing and counts for nobody.
func (r *Report) Most() *Resolving

// Silent returns the kinds that the run examines and that no finding of
// Findings has, in the order of the design.
func (r *Report) Silent() []Silence

// Finding is one disagreement between the files and the history, of one task
// or of none.
type Finding struct {
	Key      string         // the rule id, or Proposed, Stale or SoleReview
	Rule     string         // the rule id of RULES.md; "" for the three kinds
	Severity model.Severity // as the audit lists it: the rule's, or Information after the demotion
	Demoted  bool           // Severity is Information and the rule's is graver
	Kind     string         // the rule's title, or the kind's
	Task     string         // "" when the finding names none
	Gate     string         // "" when the finding names none
	File     string         // relative to the project directory, .tableaux/…; "" for none
	Message  string         // a rule finding: the diagnostic's, byte for byte
	Act      Act
	Resolver string // an email; "" when the files name nobody
	Sentence string // the sentence of the method the kind rests on
	Source   Source // where the sentence stands
	Facts    Facts
	Commits  []*history.Commit // the commits involved, each once, the oldest first
	Commands []Command         // what shows the finding, then what resolves it

	Diagnostic *model.Diagnostic // the diagnostic a rule finding carries, its severity the rule's own; nil for the three kinds
}

// Facts are the typed facts of one finding: values of the Derivation, never
// copies and never words. A fact the finding does not carry is nil.
type Facts struct {
	Junction      *derive.Junction      // Facts.Junction(task, gate)
	Status        *derive.Status        // Facts.Status(task)
	Requirement   *derive.Condition     // the entry of Facts.Requires(task) the diagnostic stands at
	Authorisation *derive.Authorisation // Facts.Authorisation(task)
	Snapshot      *derive.Snapshot      // Junction.Snapshot: the link, whose pin is Facts.Pin(Snapshot.Link)
	Reading       *derive.Reading       // the model check of the finding's commit at its gate
}

// Row is the findings that agree in key, listed severity, gate, message, act
// and resolver, each of another task, in the order Run raises them. It is
// never empty.
type Row []*Finding

// Source says where a sentence stands: "README.md, Status" and its address.
type Source struct{ Text, URL string }

// Command is a line a reader runs, and what it does: "shows the finding" or
// "resolves it".
type Command struct{ Text, Comment string }

// Counts are the findings by listed severity.
type Counts struct{ Errors, Warnings, Information int }

// Resolving is the number of findings that ask an act of one person.
type Resolving struct {
	Email string
	Count int
}

// Silence is a kind the audit examines and finds nothing of.
type Silence struct{ Key, Rule, Kind string }
```

The unexported surface, one entry per rule in `rules.go`:

```go
// role names who resolves a finding of a rule: "keeper", "owner", "assignee",
// "contributor", "reviewer", "author", "origin", "authority" or "nobody".
type role string

// rule is one row of design/dada/rules.md.
type rule struct {
	act      Act
	who      role
	source   Source
	sentence string
}

// table holds a row for every rule of validate.Rules. J8 and H2 take their
// act and their role in code, by the condition rules.md states.
var table map[string]rule

// silent lists the rules the audit view names in VIEWS.md#audit, in the
// order of validate.Rules: P5, R9, R12, R13, J8, J9, J13 to J17, H1 to H6.
var silent []string

// historical reports whether the junction at the gate is one of the task's
// historical junctions, or the task is complete: the one predicate of the
// demotion and of a sole review.
func historical(f *derive.Facts, task, gate string) bool
```

## From a finding to the data

The audit view (8ed1) writes `view.Audit` from a `*Report`, through the builders of `internal/views` and no code of its own per fact:

| `view`                              | From                                                                                                   |
|-------------------------------------|--------------------------------------------------------------------------------------------------------|
| `errors`, `warnings`, `information` | `Counts()` of the report after `Keep`                                                                    |
| `most`, `silent`                    | `Most()`, `Silent()` of the same report                                                                  |
| `findings`                          | `Rows(minimum)`, one `Finding` per row; the fields of its first finding, `Severity.String()`, `string(Act)` |
| `tasks`                             | `builder.ref` of each finding's `Task`, in the row's order; empty for a finding with none                |
| `files`                             | Each finding's `File`, once, in the row's order                                                          |
| `source`                            | `Reference{Text, URL}` of `Source`                                                                       |
| `facts[].junction`                  | `builder.junction(Facts.Junction, true)`                                                                 |
| `facts[].status`                    | `builder.status(f, Facts.Status)`                                                                        |
| `facts[].requirement`               | `builder.requirement(Facts.Requirement, false)`                                                          |
| `facts[].authorisation`             | `builder.authorisation(task)` (886d)                                                                     |
| `facts[].linkage`                   | The item of `builder.linkages(task)` (886d) whose `gates` hold the finding's gate                        |
| `facts[].model`                     | A `ModelCheck` as the task definition writes one: the finding's newest commit, `Reading.Gate`, `.Stated`, `.Trailer`, and the word of `.Verdict` |
| `commits`                           | `builder.commit` of each commit of the row, once, by `Commit.Seq`, the oldest first                       |
| `commands`                          | `view.Command{Text, Comment}` of the first finding's                                                     |

## `internal/derive`: one method more

```go
// ByAuthorisation reports whether the authorisation stands as the review of
// a task's junction at a gate: a plain junction at the gate the method names
// for that exception, whether or not it states a reviewer. It agrees with
// Accepted(id, gate).ByAuthorisation wherever that review is not nil.
func (f *Facts) ByAuthorisation(id, gate string) bool
```

Its body reads the junction between `f.enter()` and `f.leave()` and compares the gate with the constant `defined` of `review.go`, as `accepted` does; it adds no Git call.
