# Design dada: the declarations

The Go surface of [design dada](../dada.md): one package, `internal/audit`. Every name is internal to the module. A trial builds the exported declarations against `main` at `250ef85`, with bodies, and produces [`corpus.expected.txt`](corpus.expected.txt) and [`audit.json`](audit.json).

```go
// Package audit turns the Validator's diagnostics and two facts of the
// Derivation into findings: each names its tasks, its gate, its commits, the
// action that resolves it and the person who takes it. It is a pure function
// of its input: it runs no process and reads no file, clock or environment.
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
	Advance       Act = "advance"        // the originating task passes the gate a requirement names
	None          Act = "none"           // nothing resolves it: the commit stands in the history
)

// The keys of the two kinds of finding that no rule states.
const (
	Proposed = "proposed"
	Stale    = "stale"
)

// DefaultStale is the age in days beyond which a status is stale.
const DefaultStale = 7

// Input is what one audit reads.
type Input struct {
	Facts       *derive.Facts      // the facts of the project in view
	Diagnostics []model.Diagnostic // the Loader's and the Validator's, as snapshot.go sorts them
	Now         string             // the date staleness counts from, YYYY-MM-DD; "" is the source commit's author date
	Stale       int                // the age in days; 0 is DefaultStale
}

// Run audits a project. It returns nil when in.Facts is nil or refuses, and
// never fails or panics otherwise. It changes nothing of its input, and
// the findings point into in.Diagnostics.
func Run(in Input) *Report

// Report is the audit of one project at one source. It is immutable.
type Report struct {
	Now      string    // the date staleness counts from, as resolved; "" when nothing gives one
	Stale    int       // the age in days, as resolved
	Findings []Finding // one per diagnostic, proposed task and stale status, in the order of the design
	// and, unexported, the kinds the run examines
}

// Keep returns the report with the findings keep accepts, in their order. Now,
// Stale and the kinds examined stay.
func (r *Report) Keep(keep func(*Finding) bool) *Report

// Rows returns the findings with the tasks that one kind hits at one gate for
// one reason joined in one row, in the order of the first finding of each.
func (r *Report) Rows() []Finding

// Counts returns the findings by severity, one per finding of Findings.
func (r *Report) Counts() Counts

// Most returns the person who resolves the most findings, the first in the
// findings' order among equals, or nil when no finding names a resolver.
func (r *Report) Most() *Resolving

// Silent returns the kinds that the run examines and that no finding of
// Findings has, in the order of the design.
func (r *Report) Silent() []Silence

// Finding is one disagreement between the files and the history. Run gives
// each one task or none; Rows gives a row several. No list is nil.
type Finding struct {
	Key      string    `json:"key"`      // the rule id, or "proposed" or "stale"
	Rule     string    `json:"rule"`     // the rule id of RULES.md; "" for the two kinds no rule states
	Severity string    `json:"severity"` // "error", "warning" or "information"
	Kind     string    `json:"kind"`     // the kind in a few words: the rule's title
	Tasks    []Task    `json:"tasks"`
	Gate     string    `json:"gate"`     // "" when the finding names none
	Files    []string  `json:"files"`    // below .tableaux, with forward slashes
	Message  string    `json:"message"`  // a rule finding: the diagnostic's, byte for byte
	Act      Act       `json:"act"`
	Action   string    `json:"action"`   // worded text
	Resolver string    `json:"resolver"` // an email; "" when the files name nobody
	Sentence string    `json:"sentence"` // the sentence of the method the kind rests on
	Source   Source    `json:"source"`   // where the sentence stands
	Facts    []Fact    `json:"facts"`    // worded pairs
	Commands []Command `json:"commands"` // what shows the finding, then what resolves it

	Commits    []*history.Commit `json:"-"` // the commits involved, each once, the oldest first
	Diagnostic *model.Diagnostic `json:"-"` // the diagnostic a rule finding carries; nil for the two kinds; a row's first
}

// Task names a task. Title is "" for an id that names no task of the project.
type Task struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Source says where a sentence stands: "README.md, Status" and its address.
type Source struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Fact is a name and a value in the Audit's words.
type Fact struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Command is a line a reader runs, and what it does.
type Command struct {
	Text    string `json:"text"`
	Comment string `json:"comment"` // "shows the finding" or "resolves it"
}

// Counts are the findings by severity.
type Counts struct {
	Errors      int `json:"errors"`
	Warnings    int `json:"warnings"`
	Information int `json:"information"`
}

// Resolving is the number of findings one person resolves.
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

The unexported surface, one entry per rule in `rules.go`:

```go
// role names who resolves a finding of a rule: "keeper", "owner", "assignee",
// "contributor", "reviewer", "author", "origin" or "nobody".
type role string

// rule is one row of design/dada/rules.md.
type rule struct {
	act      Act
	who      role
	action   string // the text; J8, H2, H4, H5, S11 and R9 fill theirs in code
	source   Source
	sentence string
}

// table holds a row for every rule of validate.Rules.
var table map[string]rule

// silent lists the rules the audit view names in VIEWS.md#audit, in the
// order of validate.Rules: P5, R9, R12, R13, J8, J9, J13 to J17, H1 to H6.
var silent []string
```
