# Design 27a3: the declarations

The Go surface of [design 27a3](../27a3.md), package by package. Every name here is internal to the module: nothing joins the exported surface of `github.com/nbyoung/tablo`. A trial compiles these blocks against `main` at `0777a93` with a body that panics in each function.

## `internal/git`: two commands more

```go
package git

// Ref is one branch or remote-tracking ref.
type Ref struct {
	Name   string // in full: refs/heads/main
	Commit string // the commit it names, in full
	Target string // for a symbolic ref, the ref it names; else ""
}

// Refs lists the refs under the prefixes given, by name, in one for-each-ref.
func (r Runner) Refs(ctx context.Context, repo Repo, prefixes ...string) ([]Ref, error)

// LogChange is one path a commit changes against its first parent.
type LogChange struct {
	Path             string // relative to the repository root
	OldMode, NewMode string // 000000 where the path is absent
	Old, New         string // the object ids, in full; all zeros where the path is absent
}

// LogCommit is one commit as Log prints it.
type LogCommit struct {
	ID                            string
	Parents                       []string
	AuthorName, AuthorEmail       string
	AuthorTime                    int64 // seconds since the epoch
	AuthorZone                    int   // minutes east of UTC
	CommitterName, CommitterEmail string
	Subject                       string
	Trailers                      []string // the lines of the trailer block, unfolded, in order
	Changes                       []LogChange
}

// Log walks the history of tips in one git log and returns every commit, the
// newest first in Git's date order, each with the changes it makes among
// paths against its first parent. It prunes no commit: one that changes none
// of the paths returns with no change.
func (r Runner) Log(ctx context.Context, repo Repo, tips, paths []string) ([]LogCommit, error)
```

## `internal/load`: one file at a time

`assemble` becomes `Compose` over `Parse`, and behaves as it does today.

```go
package load

// Piece is one file of the layout, parsed: the file, what it builds and the
// diagnostics it earns. A Piece is immutable.
type Piece struct{ /* unexported */ }

// Parse reads data as the file at path, a path below .tableaux.
func Parse(path string, data []byte) *Piece

// Compose joins pieces into the project they state, without its links: the
// project's Links is nil and each Subproject.Link is nil.
func Compose(where model.Location, pieces []*Piece, stray []string) *model.Project
```

## `internal/history`: the pass

```go
package history

// Person is an identity as a commit records it, with no mailmap applied.
type Person struct{ Name, Email string }

// Trailer is one line of a commit's trailer block.
type Trailer struct{ Key, Value string }

// Change is one watched path a commit changes against its first parent.
type Change struct {
	Path     string // relative to the repository root
	Old, New string // the object ids, in full; "" where the path is absent
	Gitlink  bool   // either side is a submodule's pin
}

// Commit is one commit of a pass. It is immutable.
type Commit struct {
	ID        string   // in full
	Parents   []string // in full, the first parent first
	Author    Person
	Committer Person
	Time      int64 // the author time, in seconds since the epoch
	Zone      int   // the author's offset from UTC, in minutes east
	Subject   string
	Trailers  []Trailer // the trailer block as Git reads it, in order
	Changes   []Change  // by path
	Seq       int       // its place in the pass; 0 is the newest
	InSource  bool      // the source reaches it
	Trunk     int       // its place on the trunk's first-parent line, 0 at the tip; -1 off the line
}

// Date returns the author date as YYYY-MM-DD in the author's own zone.
func (c *Commit) Date() string

// Merge reports whether the commit has more than one parent.
func (c *Commit) Merge() bool

// Values returns the values of the trailers whose key is key, in order.
func (c *Commit) Values(key string) []string

// From says what names the trunk.
type From int

// The sources of a trunk's name, in the order README.md#project reads them.
const (
	Unnamed  From = iota // nothing names a branch
	Stated               // version.yaml
	Inferred             // refs/remotes/origin/HEAD
	Caller               // the branch the caller names
)

// Trunk is the trunk of one project in one repository.
type Trunk struct {
	Name string // the branch's name; "" when From is Unnamed
	From From
	Ref  string // the ref that carries the name, in full; "" when none does
	Tip  string // the commit that ref names; "" when none does
}

// How returns "stated", "inferred" or "caller", or "undetermined" when no
// ref carries a name.
func (t Trunk) How() string

// Log is one pass: the history of one project's source and of its trunk. It
// is immutable and safe for concurrent use.
type Log struct {
	GitDir  string // the repository's common Git directory
	Dir     string // the directory that holds .tableaux
	Source  string // the commit the view's history ends at, in full
	Trunk   Trunk
	Shallow bool      // the repository is a shallow clone: the pass may end early
	Commits []*Commit // the newest first, in Git's date order
	// unexported: the commits by id, the blobs, the tables, the pieces
}

// Commit returns the commit of the pass with the id, or nil.
func (l *Log) Commit(id string) *Commit

// Object returns the id of the object at path as the tree of a commit of the
// pass holds it, or "".
func (l *Log) Object(commit, path string) string

// Changed reports whether c changes path: against its parent or, for a
// merge, against every parent.
func (l *Log) Changed(c *Commit, path string) bool

// Project returns the project in Dir as its files stand at a commit of the
// pass, without links, or nil when the pass holds no such commit.
func (l *Log) Project(commit string) *model.Project

// Reaches reports whether to is from or one of its ancestors in the pass.
func (l *Log) Reaches(from, to string) bool

// Options configure a Reader.
type Options struct {
	Git string // the executable; "" is "git"
}

// Reader reads passes and remembers the last 64. It is safe for concurrent use.
type Reader struct{ /* unexported */ }

// NewReader returns a Reader.
func NewReader(Options) *Reader

// Set holds the pass of every project of one load.
type Set struct{ /* unexported */ }

// Of returns the pass of p, or nil when p has no commit to read history from.
func (s *Set) Of(p *model.Project) *Log

// Read makes the passes for p, the project a Load returns, and for every
// project its links reach. trunk is the branch the caller names, read for p
// alone. extra names further commits of p's repository to walk, as a range's
// start. It fails only when git fails.
func (r *Reader) Read(ctx context.Context, p *model.Project, trunk string, extra ...string) (*Set, error)
```

## `internal/derive`: the facts

```go
package derive

// New derives the facts of p, the project a Load returns, and of every
// project its links reach. logs is nil for a load with no history to read.
func New(p *model.Project, logs *history.Set) *Facts

// Facts are the derived facts of one project at one source. A Facts is
// immutable and safe for concurrent use; it computes each fact at its first
// call and keeps it. After a refusal every method below Refused returns the
// zero value of its result.
type Facts struct{ /* unexported */ }

func (f *Facts) Project() *model.Project
func (f *Facts) Log() *history.Log // nil without history
func (f *Facts) Trunk() history.Trunk
func (f *Facts) OnTrunk() bool // the source is on the trunk's first-parent line
func (f *Facts) Refused() *Refusal

// Sub returns the facts of the project a link leads to, or nil when the link
// has none.
func (f *Facts) Sub(l *model.Link) *Facts

// At returns the facts of the project as its files stand at a commit of the
// pass: every fact the files alone give, and no fact of the history. It
// follows a link, by the url as the project in view links it, to the
// subproject's own past at the commit the linkage fixes there. It returns
// nil when the pass holds no such commit.
func (f *Facts) At(commit string) *Facts

// Refusal says why the files give no structure to derive from.
type Refusal struct {
	Part  string   // "project", "version", "gates", "states" or "tree"
	Rules []string // the rules of RULES.md that report it
}

// Why says why a fact is undetermined.
type Why int

// The reasons. Determined is none.
const (
	Determined Why = iota
	NoHistory      // no commit to read, or the pass does not hold the commit
	NoTrunk        // the trunk is undetermined
	OffTrunk       // the source is off the trunk's first-parent line
	NoCommit       // no commit of the history decides
	NoLink         // the link has no project
	NoTask         // the project holds no such task
	NoGate         // the key names no gate of gates.yaml, or none applies
	NoState        // the file states no state where one is due
	NoChildren     // no child of the parent has a status to roll up
	Cycle          // the statuses of two projects read each other
)

// Known is a fact that may be undetermined.
type Known int

// The values of a Known.
const (
	Unknown Known = iota
	No
	Yes
)

// The tree.

func (f *Facts) Root() string
func (f *Facts) Owner() string
func (f *Facts) Order() []string             // every task, depth first, siblings in display order
func (f *Facts) Children(id string) []string // in display order
func (f *Facts) Leaf(id string) bool
func (f *Facts) Authorities(id string) []Authority // nearest first

// Authority is one authority of a task and the ancestor that makes it one.
type Authority struct{ Email, Task string }

// Role is a role of README.md#roles.
type Role string

// The roles, in README.md's order.
const (
	Owner       Role = "owner"
	AuthorityOf Role = "authority"
	Assignee    Role = "assignee"
	Contributor Role = "contributor"
	Agent       Role = "agent"
	Reviewer    Role = "reviewer"
	Observer    Role = "observer"
)

// Roles returns the roles the email holds anywhere in the project, in
// README.md's order, or Observer alone.
func (f *Facts) Roles(email string) []Role

// The gates.

func (f *Facts) Gates() []string           // the keys of gates.yaml, in order
func (f *Facts) Index(gate string) int     // -1 for a key gates.yaml lacks
func (f *Facts) Severity(state string) int // 0 for a key gates.yaml lacks

// The junctions.

// By says what supplies a field of a resolved junction.
type By int

// The suppliers. e4c7 A3 names them default, task and assignee.
const (
	ByDefault  By = iota // no entry states the field, and the plain default gives none
	ByTask               // the entry of Field.Task states it
	ByAssignee           // no entry states it, and the task's assignee takes it
)

// Field is one field of a resolved plain junction.
type Field struct {
	V    string
	By   By
	Task string       // the task whose entry states it; "" unless By is ByTask
	Node *model.Value // the value in that entry; nil unless By is ByTask
}

// Junction is a task's junction at one gate after undefined, resolved.
type Junction struct {
	Task, Gate                   string
	Kind                         model.JunctionKind // Plain, Recursive or NotApplicable
	Contributor, Model, Reviewer Field              // of a plain junction
	References                   []*model.Reference // of a plain junction
	ReferencesFrom               string             // the task whose entry states them; "" for none
	Entry                        string             // the task whose entry makes it recursive or exempt
	Snapshot                     *Snapshot          // of a recursive junction
}

// Sources returns the tasks whose entries supply the junction, nearest first.
func (j *Junction) Sources() []string

// Mark is a junction mark of the method, by the key tableaud 438a A8 gives it.
type Mark string

// The marks.
const (
	MarkPerson     Mark = "person"     // 🧑
	MarkAgent      Mark = "agent"      // 🤖
	MarkReviewer   Mark = "reviewer"   // 👀
	MarkSubproject Mark = "subproject" // 🪆
	MarkExempt     Mark = "exempt"     // —
)

// Symbol returns the mark as README.md#junctions draws it.
func (m Mark) Symbol() string

// Marks returns the marks of the junction, the contributor's first.
func (j *Junction) Marks() []Mark

func (f *Facts) Junction(id, gate string) *Junction // nil at undefined
func (f *Facts) Junctions(id string) []*Junction    // every gate after undefined, in order
func (f *Facts) Applicable(id string) []string      // undefined first
func (f *Facts) First(id string) string             // the first applicable gate after undefined
func (f *Facts) Last(id string) string
func (f *Facts) Next(id, gate string) string // the next applicable gate after gate; "" at the last

// The subprojects.

// Snapshot is the task a recursive junction reads, in the project its url fixes.
type Snapshot struct {
	Task   string            // the task that holds the junction
	Gates  []string          // the gates whose junctions name this target, in order
	Entry  *model.Subproject // the field of the first of them
	Link   *model.Link
	Target string // the task read there: the id stated, or that project's root
	Facts  *Facts // the facts of that project; nil when the link has none
	Why    Why    // NoLink or NoTask
}

// Snapshots returns one snapshot per distinct link and target, in gate order.
func (f *Facts) Snapshots(id string) []*Snapshot

// Pin is the commit a link reads and where it stands in that repository.
type Pin struct {
	Link     *model.Link
	Commit   string // the commit read; "" for a directory of the same repository
	Recorded string // a submodule: the gitlink of the source commit
	Moved    bool   // a submodule: the checkout's HEAD is not Recorded
	Tip      string // the tip of that project's trunk
	OnTrunk  Known  // Commit is on that trunk's first-parent line
	Behind   Known  // OnTrunk, and Commit is not Tip
}

func (f *Facts) Pin(l *model.Link) *Pin

// The statuses.

// StatusKind says where a status comes from.
type StatusKind int

// The kinds of a status.
const (
	Recorded    StatusKind = iota // a leaf's file
	Absent                        // a leaf without a file: undefined at undefined
	RolledUp                      // a parent's, from its children
	Snapshotted                   // a leaf whose next junction is recursive
)

// Status is a task's status.
type Status struct {
	Task                      string
	Kind                      StatusKind
	Gate, State, Reason, Note string
	Why                       Why           // why the state is undetermined
	File                      *model.Status // nil for Absent and RolledUp
	From                      string        // RolledUp: the child the state comes from
	Considered                []string      // RolledUp: the children considered, in display order
	Snapshot                  *Snapshot     // Snapshotted
	Of                        *Status       // Snapshotted: the status of the task read there

	Commit      *history.Commit // the deciding commit; nil for RolledUp
	Date        string          // YYYY-MM-DD; "" when undetermined
	Recorder    string          // "" for Absent and RolledUp
	Own         *history.Commit // Snapshotted: the deciding commit of the task's own file
	Uncommitted bool            // the file in view is not the source commit's
}

// Derived reports whether a roll-up gives the state: a parent's status, or a
// snapshot of a parent.
func (s *Status) Derived() bool

func (f *Facts) Status(id string) *Status

// Chain returns the statuses a status comes through, the task's first: each
// roll-up's child and each snapshot's task, to the leaf whose file states it.
func (f *Facts) Chain(id string) []*Status

// The requirements.

// Condition is one requires entry with its gates resolved and its condition.
type Condition struct {
	Task     string // the terminating task
	Entry    *model.Requirement
	Link     *model.Link // nil for an entry by id
	Origin   string      // the originating task
	Facts    *Facts      // the facts of the originating project; nil when the link has none
	From, To string
	Stands   string // the gate the originating task stands at
	Met, Due bool
	Why      Why   // NoLink, NoTask or NoGate: Met and Due are then false
	AtTrunk  Known // a cross-project entry: met at the tip of the originating project's trunk
}

// Word returns "met", "unmet" or "pending", as the corpus writes the
// condition, or "" when it is undetermined.
func (c *Condition) Word() string

func (f *Facts) Requires(id string) []*Condition   // in file order
func (f *Facts) Dependents(id string) []*Condition // by terminating task in display order, then file order

// The authorisation.

// Way is how a deciding commit accepts.
type Way int

// The ways.
const (
	NoWay     Way = iota
	ByCommit      // it changes the task's file
	ByMerge       // it merges a change to the task's file
	ByTrailer     // it carries Authorised: and leaves the file alone
)

// Authorisation is a task's authorisation.
type Authorisation struct {
	Task        string
	Authorised  bool
	Why         Why             // proposed with no deciding commit: NoHistory, NoTrunk, OffTrunk or NoCommit
	Commit      *history.Commit // the deciding commit
	Way         Way
	Judges      []string // the emails that may accept, as the files stand at the deciding commit, nearest first
	Author      bool     // the author is one of them
	Committer   bool     // the committer is one of them
	By          string   // the authority that accepts: the author's email before the committer's; else the author's
	Differs     bool     // the task's file in view is not the one at the trunk's tip
	Uncommitted bool     // the task's file in view is not the source commit's
}

func (f *Facts) Authorisation(id string) *Authorisation

// AuthorisationAt returns the authorisation as it stands at a commit of the
// trunk's first-parent line, read from the line up to that commit.
func (f *Facts) AuthorisationAt(id, commit string) *Authorisation

// The reviews.

// Review is one reviewed junction and the commit that accepts it.
type Review struct {
	Task, Gate      string
	Reviewer        string          // the junction's reviewer in view
	Commit          *history.Commit // nil while nothing accepts
	By              string          // the reviewer's email on that commit
	ByAuthorisation bool            // at defined: the authorisation stands as the review
}

// Reviews returns the reviewed junctions the task's status passes, in gate order.
func (f *Facts) Reviews(id string) []*Review

// Accepted returns the review of one junction, passed or not, or nil when
// the junction has no reviewer.
func (f *Facts) Accepted(id, gate string) *Review

// HandoffKind is the form of a hand-off.
type HandoffKind int

// The forms.
const (
	NoHandoff HandoffKind = iota
	Stated                // the status states the reason review
	Implied               // the history implies it and the status does not state it
	Stale                 // the status states it after the reviewer's Reviewed: commit
)

// Handoff is the hand-off of a leaf's next junction to its reviewer.
type Handoff struct {
	Task, Gate string
	Kind       HandoffKind
	Reviewer   string
	Self       bool            // the reviewer is the junction's contributor
	Commit     *history.Commit // Implied: the newest event's; Stale: the accepting commit
}

func (f *Facts) Handoff(id string) *Handoff

// The history.

// EventKind is an event of SYNTAX.md#history.
type EventKind string

// The kinds, in the order one commit's events of one task take.
const (
	TaskEvent  EventKind = "task"
	Authorised EventKind = "authorised"
	StatusSet  EventKind = "status"
	Reaffirmed EventKind = "reaffirmed"
	Reviewed   EventKind = "reviewed"
	Pinned     EventKind = "pin"
)

// Effect is what a Reviewed: trailer does.
type Effect int

// The effects.
const (
	NoEffect   Effect = iota // not a reviewed event
	Accepts                  // the reviewer's first: it completes the junction
	Repeats                  // the reviewer's, after one that accepts
	WrongHand                // neither author nor committer is the junction's reviewer
	NoReviewer               // the junction has no reviewer
	AtDefined                // the authorisation stands as the review of defined
)

// Verdict is how a commit's Model: trailer reads against its junction.
type Verdict int

// The verdicts.
const (
	NoModel  Verdict = iota // the junction states no model and the commit carries no trailer, or another hand commits
	Unstated                // a trailer where the junction states no model
	Match                   // the trailer is the stated model or starts with it
	Mismatch                // the trailer names a model outside the one stated
	Missing                 // the contributor's commit carries no trailer
	Exempt                  // as Missing, where version.yaml at the commit states a language before 0.2.1
)

// Reading is the model check of one commit at one junction.
type Reading struct {
	Gate        string // the junction the commit is at
	Contributor string // its contributor, as the files stand at the commit
	Stated      string // the model it states there; "" for none
	Trailer     string // the commit's first Model: trailer; "" for none
	Verdict     Verdict
}

// Event is one event of one task.
type Event struct {
	Commit                    *history.Commit
	Task                      string // the task of this project the event concerns
	Kind                      EventKind
	Gate, State, Reason, Note string      // status: as the commit records them; reviewed: the gate
	URL, Old, New             string      // pin
	Link                      *model.Link // pin, and an event taken from a subproject
	Sub                       string      // an event taken from a subproject: the task there; else ""
	Reading                   *Reading    // status and reaffirmed
	Effect                    Effect      // reviewed
}

// Events returns the events of a task: its own and, for each snapshot, those
// of the task read there and of its descendants up to the pin.
func (f *Facts) Events(id string) []*Event

// History returns the events of every task of the project.
func (f *Facts) History() []*Event

// Unread is a task trailer the method cannot read (H1).
type Unread struct {
	Commit *history.Commit
	Line   string // the trailer as written
	NoTask bool   // it names no task of the project
	NoGate bool   // Reviewed: it names no gate that applies to the task
}

func (f *Facts) Unread() []*Unread
```
