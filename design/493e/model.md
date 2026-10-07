# Design 493e: the declarations

The Go surface of [design 493e](../493e.md). Two packages are new and one gains a method. A trial outside the repository compiles these blocks as they stand against `main` at `250ef85`, with a body for the gate definition, the task definition, the authority delegation and the glance of the task assignment, and produces the files under [`golden/`](golden/).

**Shared** marks what the three view tasks build on: `886d` and `8ed1` add files beside these and change no shared declaration without a row in their own "Assumed interfaces". **Own** marks what the four structural views alone use.

| Block                                                  | Lands as                                                      | Shared or own                                    |
|--------------------------------------------------------|---------------------------------------------------------------|--------------------------------------------------|
| [`view`: what every view shares](#view-what-every-view-shares) | `view/shared.go`                                      | Shared                                           |
| [`view`: the gate definition](#view-the-gate-definition) | `view/gates.go`                                             | Own                                              |
| [`view`: the task definition](#view-the-task-definition) | `view/task.go`                                              | Own                                              |
| [`view`: the authority delegation](#view-the-authority-delegation) | `view/authority.go`                               | Own                                              |
| [`view`: the task assignment](#view-the-task-assignment) | `view/assignment.go`                                        | Own                                              |
| [`internal/views`: the functions](#internalviews-the-functions) | `internal/views/views.go`, `shared.go`, one file per view | `Level`, `Query` and the builders are shared |
| [`internal/derive`: one method more](#internalderive-one-method-more) | `internal/derive/authorise.go`                 | Shared                                           |

## `view`: what every view shares

```go
// Package view holds the data of each view as tablo emits it: the Go types
// that an envelope's Data holds and that marshal to its JSON. The JSON tags
// are the contract of the front ends. The package holds types alone and
// imports nothing; internal/views builds the values.
//
// A view's struct holds its glance and embeds a pointer to its detail and a
// pointer to its provenance. A level the data does not reach is a nil
// pointer, and its keys are absent from the JSON; a level the data reaches
// writes every one of its keys, a list as an array, an absent object as null
// and an absent text as "".
package view

// TaskRef names a task of the project in view, or of a subproject where the
// field says so.
type TaskRef struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// Person is an identity as a commit records it.
type Person struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Legend is gates.yaml and the method's marks: what every key and symbol in
// a view means. Every view carries it whole at every level. A view names a
// gate, a state, a reason and a mark by its key alone.
type Legend struct {
	Gates   []Gate   `json:"gates"`   // in file order
	States  []State  `json:"states"`  // in file order
	Reasons []Reason `json:"reasons"` // in file order
	Marks   []Mark   `json:"marks"`   // person, agent, reviewer, subproject, exempt
}

// Gate is one gate of gates.yaml.
type Gate struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Criteria string `json:"criteria"`
}

// State is one state of gates.yaml.
type State struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Severity int    `json:"severity"`
	Synopsis string `json:"synopsis"`
}

// Reason is one reason of gates.yaml. Reserved is true for review, the
// reason the method reserves for the hand-off.
type Reason struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Synopsis string `json:"synopsis"`
	Reserved bool   `json:"reserved"`
}

// Mark is a junction mark of the method.
type Mark struct {
	Key      string `json:"key"`      // person, agent, reviewer, subproject or exempt
	Symbol   string `json:"symbol"`   // as README.md#junctions draws it
	Meaning  string `json:"meaning"`  // the phrase of VIEWS.md's gate definition
	Junction string `json:"junction"` // the kind of junction that shows it: plain, recursive or not_applicable
}

// Reference is a reference of a task or of a junction.
type Reference struct {
	Text string `json:"text"`
	URL  string `json:"url"`
}

// Status is a task's status as a view shows it.
type Status struct {
	Kind        string  `json:"kind"` // recorded, absent, rollup or snapshot
	Gate        string  `json:"gate"`
	State       string  `json:"state"`
	Reason      string  `json:"reason"`
	Note        string  `json:"note"`
	Date        string  `json:"date"`     // YYYY-MM-DD; "" when undetermined
	Recorder    string  `json:"recorder"` // an email; "" for an absent status and a roll-up
	Derived     bool    `json:"derived"`  // a roll-up gives the state
	Why         string  `json:"why"`      // why the state is undetermined; "" when it is not
	Uncommitted bool    `json:"uncommitted"`
	From        *Origin `json:"from"` // null for a recorded and an absent status
}

// Origin says where a rolled-up or a snapshot status comes from.
type Origin struct {
	Kind string  `json:"kind"` // rollup or snapshot
	Task TaskRef `json:"task"` // rollup: the child; snapshot: the task read in the subproject
	URL  string  `json:"url"`  // snapshot: the subproject
	Pin  string  `json:"pin"`  // snapshot: the commit read, in full; "" for a directory of the same repository
	Gate string  `json:"gate"` // snapshot: the gate the task read there stands at
}

// Step is one status of a chain: the statuses a status comes through, the
// task's own first, to the leaf whose file states it.
type Step struct {
	Task   TaskRef `json:"task"`
	URL    string  `json:"url"` // the subproject the task is of; "" in the project in view
	Status Status  `json:"status"`
}

// Commit is a commit fact.
type Commit struct {
	Hash       string   `json:"hash"` // in full
	Subject    string   `json:"subject"`
	Date       string   `json:"date"`        // the author date, YYYY-MM-DD, in the author's zone
	AuthorTime string   `json:"author_time"` // the author time, RFC 3339 with the author's offset
	Author     Person   `json:"author"`
	Committer  Person   `json:"committer"`
	Parents    []string `json:"parents"`  // in full, the first parent first
	Trailers   []string `json:"trailers"` // "Key: value", in order
	Files      []File   `json:"files"`    // the watched paths it changes against its first parent, by path
}

// File is a path a commit changes.
type File struct {
	Path   string `json:"path"`   // relative to the repository root
	Change string `json:"change"` // added, changed, removed or pin
}

// Command is a line a reader runs to reproduce a fact.
type Command struct {
	Text    string `json:"text"`
	Comment string `json:"comment"`
}

// Junction is a task's junction at one gate after undefined, resolved.
type Junction struct {
	Gate              string      `json:"gate"`
	Kind              string      `json:"kind"`  // plain, recursive or not_applicable
	Marks             []string    `json:"marks"` // keys of Legend.Marks, the contributor's first
	Acts              bool        `json:"acts"`  // the person in view contributes or reviews here
	Contributor       string      `json:"contributor"`
	Model             string      `json:"model"`
	Reviewer          string      `json:"reviewer"`
	ReviewerByDefault bool        `json:"reviewer_by_default"` // the assignee reviews, since no entry states a reviewer
	Subproject        *SubRef     `json:"subproject"`          // a recursive junction; else null
	References        []Reference `json:"references"`
	Stands            string      `json:"stands"`        // passed, here, next, later or exempt; "" on a parent
	Reviewed          bool        `json:"reviewed"`      // a commit accepts the junction
	From              []string    `json:"from,omitzero"` // where the view shows suppliers: the tasks whose entries supply the junction, nearest first
}

// SubRef names the task a recursive junction or a cross-project requirement
// reads.
type SubRef struct {
	URL    string  `json:"url"`    // as the link resolves it
	ID     string  `json:"id"`     // the id the entry states; "" when it reads the root
	Task   TaskRef `json:"task"`   // the task read there; zero when undetermined
	Form   string  `json:"form"`   // submodule, directory, url or none
	Commit string  `json:"commit"` // the commit the link reads, in full; "" for a directory
	Why    string  `json:"why"`    // no_link or no_task; "" when the task is read
}

// Stated says where a value stands in the files.
type Stated struct {
	By   string `json:"by"`   // a task id; "own" for each task's own file; "assignee"; or "default"
	Key  string `json:"key"`  // the key in that task's file: junctions.<gate>.<field> or assignee; "" for a default
	File string `json:"file"` // .tableaux/tasks/<id>.yaml; "" unless By is a task id
	Line int    `json:"line"` // from 1; 0 unless By is a task id
	Col  int    `json:"col"`
}

// Supplier says what supplies one field of one resolved junction.
type Supplier struct {
	Gate  string `json:"gate"`
	Field string `json:"field"` // contributor, model, reviewer, references, subproject or applies
	Value string `json:"value"`
	Stated
}

// Requirement is one requires entry with its condition, seen from either end.
type Requirement struct {
	Task       TaskRef `json:"task"` // the other task
	From       string  `json:"from"`
	To         string  `json:"to"`
	Text       string  `json:"text"`
	Met        bool    `json:"met"`
	Due        bool    `json:"due"`
	Condition  string  `json:"condition"`  // met, unmet or pending; "" when undetermined
	Why        string  `json:"why"`        // no_link, no_task or no_gate; "" when determined
	Status     *Status `json:"status"`     // of the originating task; null for a dependent and when undetermined
	Subproject string  `json:"subproject"` // a cross-project entry: its url; else ""
	Commit     string  `json:"commit"`     // a cross-project entry: the commit it reads, in full
	AtTrunk    string  `json:"at_trunk"`   // a cross-project entry: met at the tip of that project's trunk: unknown, no or yes
}

// Column is one column of a tableau's window: a gate, or the run of gates a
// fold stands for. The two tableaux alone carry columns; task 886d computes
// them.
type Column struct {
	Gates  []string `json:"gates"` // one gate, or the run a folded column stands for
	Folded bool     `json:"folded"`
	Count  int      `json:"count"`   // folded: the tasks in view whose current gate lies in the run
	ByGate []int    `json:"by_gate"` // folded: the same count per gate of the run
}
```

What fills each shared type. `f` is the `*derive.Facts` of the project that holds the task: a status, a title or a child of a subproject's task reads the facts of that project, `Snapshot.Facts` or `Condition.Facts`, never the home project's.

| Type, field                    | Filled from                                                                                                        |
|--------------------------------|--------------------------------------------------------------------------------------------------------------------|
| `TaskRef`                      | The id; `f.Project().Tasks[id].Title.V`, `""` when the project lacks the task                                        |
| `Person`                       | `history.Commit.Author`, `.Committer`                                                                                |
| `Legend.Gates`                 | For each key of `f.Gates()`, the first entry of `f.Project().Gating.Gates` with that key: `Symbol.V`, `Name.V`, `Criteria.V` |
| `Legend.States`                | Each entry of `Gating.States` in file order; `Severity` is `f.Severity(key)`                                         |
| `Legend.Reasons`               | Each entry of `Gating.Reasons` in file order; `Reserved` is `key == "review"`                                        |
| `Legend.Marks`                 | The five `derive.Mark` constants in their order; `Symbol` is `Mark.Symbol()`; `Meaning` and `Junction` from the table of marks below |
| `Reference`                    | `model.Reference.Text.V`, `.URL.V`                                                                                   |
| `Status`                       | `f.Status(id)`: `Kind` by the table of words below; `Gate`, `State`, `Reason`, `Note`, `Date`, `Recorder`, `Uncommitted` as they stand; `Derived` is `Status.Derived()`; `Why` is the word of `Status.Why` |
| `Status.From`, a roll-up       | `Task` is `Status.From` with its title; null when `From` is `""`. The other fields stay `""`                         |
| `Status.From`, a snapshot      | `Task` is `Status.Snapshot.Target`, its title from `Snapshot.Facts`; `URL` and `Pin` are `Snapshot.Link.URL` and `.Commit`; `Gate` is `Status.Of.Gate` |
| `Step`                         | Each status of `f.Chain(id)`, in order. `URL` is `""` up to and including a `Snapshotted` status, and `Snapshot.Link.URL` of that status on the steps after it, which read `Snapshot.Facts` |
| `Commit`                       | A `*history.Commit`: `ID`, `Subject`, `Date()`, `Author`, `Committer`, `Parents`; `AuthorTime` is `time.Unix(Time, 0).In(time.FixedZone("", Zone*60)).Format(time.RFC3339)`; each of `Trailers` is `Key + ": " + Value`; `Files` from `Changes` |
| `File.Change`                  | `pin` when `Change.Gitlink`; else `added` when `Old` is `""`; else `removed` when `New` is `""`; else `changed`        |
| `Junction`                     | A `*derive.Junction`: `Gate`; `Kind` is `Kind.String()`; `Marks` is `Marks()`; `Contributor.V`, `Model.V`, `Reviewer.V`; `ReviewerByDefault` is `Reviewer.By == derive.ByAssignee`; `References`; `Subproject` from `Snapshot`, null unless the junction is recursive |
| `Junction.Acts`                | The query names a person, the junction is plain, and `Contributor.V` or `Reviewer.V` is that person                  |
| `Junction.Stands`              | `""` when `!f.Leaf(task)`. Else, with `at` the gate of `f.Status(task)`: `exempt` for a not-applicable junction; `here` when the gate is `at`; `passed` when `f.Index(gate) < f.Index(at)`; `next` when the gate is `f.Next(task, at)`; else `later` |
| `Junction.Reviewed`            | `f.Accepted(task, gate)` is non-nil and has a `Commit`                                                               |
| `Junction.From`                | `Junction.Sources()`, an empty array when it gives none; nil, so absent, where the view shows no suppliers           |
| `SubRef`                       | A `*derive.Snapshot`: `URL` is `Link.URL`, or `Entry.URL.V` when `Link` is nil; `ID` is `Entry.ID.V`; `Task` is `Target` with its title from `Snapshot.Facts`; `Form` is `Link.Form.String()`, `none` when `Link` is nil; `Commit` is `Link.Commit`; `Why` is the word of `Snapshot.Why` |
| `Stated`, a field `ByTask`     | `By` is `Field.Task`; `Key` is `junctions.<gate>.<field>`; `File` is `.tableaux/tasks/<Field.Task>.yaml`; `Line` and `Col` from `Field.Node.Pos` |
| `Stated`, a field `ByAssignee` | `By` is `assignee`, `Key` is `assignee`; the rest zero                                                               |
| `Stated`, a field `ByDefault`  | `By` is `default`; the rest zero                                                                                     |
| `Stated`, an assignee          | In a position of the assignment: `By` is `own` and `Key` is `assignee` for the row that groups every task assigned; for an authority, `By` is the task, `Key` is `assignee`, and `Line` and `Col` come from the `Assignee.Node.Pos` of its file |
| `Stated`, an entry             | For `references`, `subproject` and `applies`: `By` is `Junction.ReferencesFrom` or `Junction.Entry`; `Key` and `File` as above; `Line` and `Col` from `KeyPos` of that task's `model.Junction` at the gate |
| `Supplier`                     | Per junction, in this order: `contributor` when `Contributor.By` is `ByTask`; `model` when `Model.By` is `ByTask`; `reviewer` when `Reviewer.By` is not `ByDefault`; `references` when `ReferencesFrom` is not `""`, the value their count in digits; `subproject` for a recursive junction, the value its url; `applies` for a not-applicable one, the value `false` |
| `Requirement`                  | A `*derive.Condition` of `f.Requires` or `f.Dependents`: `From`, `To`, `Met`, `Due`; `Text` is `Entry.Text.V`; `Condition` is `Word()`; `Why` its word; `AtTrunk` is `AtTrunk.String()`; `Subproject` and `Commit` are `Link.URL` and `Link.Commit`, `""` when `Link` is nil |
| `Requirement.Task`, `.Status`  | A requirement: `Origin`, with its title and `Status(Origin)` from `Condition.Facts`; the status is null when `Facts` is nil or lacks the task. A dependent: `Condition.Task`, of the project in view, and a null status |
| `Column`                       | Task `886d`                                                                                                          |

The words. Each list is closed in the sense of design 4ed9's schema version: a new value raises `tablo/<n>`.

| Fact                         | Words                                                                                                              |
|------------------------------|--------------------------------------------------------------------------------------------------------------------|
| `derive.StatusKind`          | `Recorded` `recorded`, `Absent` `absent`, `RolledUp` `rollup`, `Snapshotted` `snapshot`                              |
| `derive.Why`                 | `""` for `Determined`; else `Why.String()` with each space as an underscore: `no_history`, `no_trunk`, `off_trunk`, `no_commit`, `no_link`, `no_task`, `no_gate`, `no_state`, `no_children`, `cycle` |
| `model.JunctionKind`         | `String()`: `plain`, `recursive`, `not_applicable`                                                                  |
| `model.Form`                 | `String()`: `submodule`, `directory`, `url`, `none`                                                                 |
| `derive.Known`               | `String()`: `unknown`, `no`, `yes`                                                                                  |
| `derive.Way`                 | `ByCommit` `change`, `ByMerge` `merge`, `ByTrailer` `trailer`, `NoWay` `""`                                         |
| The hand of an authorisation | `""` when the task is proposed; else `both` when `Author` and `Committer`; else `author`; else `committer`           |
| `derive.Verdict`             | `Match` `agrees`, `Mismatch` `differs`, `Missing` `missing`, `Exempt` `exempt`, `Unstated` `unstated`; `NoModel` makes no row |
| A review's effect            | `authorisation` when `Review.ByAuthorisation`; else `accepts` when `Review.Commit` is non-nil; else `none`           |
| `derive.EventKind`           | As it stands: `task`, `authorised`, `status`, `reaffirmed`, `reviewed`, `pin`                                        |
| Where a junction stands      | `passed`, `here`, `next`, `later`, `exempt`, and `""` on a parent                                                    |

The marks. `Meaning` is the phrase of VIEWS.md's gate definition with a capital; `Junction` is the kind of junction that shows the mark.

| Key          | Meaning                    | Junction         |
|--------------|----------------------------|------------------|
| `person`     | A person contributes       | `plain`          |
| `agent`      | An agent contributes       | `plain`          |
| `reviewer`   | A reviewer accepts         | `plain`          |
| `subproject` | A subproject does the work | `recursive`      |
| `exempt`     | The gate does not apply    | `not_applicable` |

## `view`: the gate definition

```go
package view

// Gates is the gate definition view.
type Gates struct {
	Legend Legend   `json:"legend"`
	Task   *TaskRef `json:"task"` // the task parameter; null without one
	*GatesDetail
	*GatesProvenance
}

// GatesDetail is what the gate definition adds at detail.
type GatesDetail struct {
	ForTask []GateFor `json:"for_task"` // with a task: one per gate, undefined first; else empty
}

// GatesProvenance is what the gate definition adds at provenance.
type GatesProvenance struct {
	Files    []FileCommit `json:"files"` // gates.yaml, then version.yaml
	Commands []Command    `json:"commands"`
}

// GateFor says how one task's junction expands a gate.
type GateFor struct {
	Gate       string      `json:"gate"`
	Applies    bool        `json:"applies"`
	References []Reference `json:"references"`
	StatedBy   string      `json:"stated_by"` // the task whose entry states them; "" for none
}

// FileCommit is a file with the newest commit that changes it.
type FileCommit struct {
	Path   string  `json:"path"`   // relative to the project directory: .tableaux/gates.yaml
	Commit *Commit `json:"commit"` // null without history, or when no commit of the source changes it
}
```

## `view`: the task definition

```go
package view

// Task is the task definition view.
type Task struct {
	Legend     Legend  `json:"legend"`
	Task       TaskRef `json:"task"`
	Assignee   string  `json:"assignee"`
	Parent     *Parent `json:"parent"` // null for the root
	Status     Status  `json:"status"`
	Authorised bool    `json:"authorised"`
	*TaskDetail
	*TaskProvenance
}

// TaskDetail is what the task definition adds at detail.
type TaskDetail struct {
	Description string        `json:"description"`
	References  []Reference   `json:"references"`
	Path        []TaskRef     `json:"path"`        // the root first, the task last
	Siblings    int           `json:"siblings"`    // the children of the parent, the task among them; 0 for the root
	Children    []Child       `json:"children"`    // in display order
	Authorities []Authority   `json:"authorities"` // nearest first
	Requires    []Requirement `json:"requires"`    // in file order
	Dependents  []Requirement `json:"dependents"`
	Junctions   []Junction    `json:"junctions"` // every gate after undefined, in order
	Snapshot    *Snapshot     `json:"snapshot"`  // a recursive next junction; else null
}

// TaskProvenance is what the task definition adds at provenance. There the
// junctions also carry From.
type TaskProvenance struct {
	StatusCommit  *Commit       `json:"status_commit"` // the deciding commit; null for a roll-up and without history
	OwnCommit     *Commit       `json:"own_commit"`    // a snapshot: the deciding commit of the task's own file
	Authorisation Authorisation `json:"authorisation"`
	Considered    []string      `json:"considered"` // a parent: the children its roll-up considers
	Chain         []Step        `json:"chain"`      // the statuses the status comes through
	Linkages      []Linkage     `json:"linkages"`   // one per subproject a recursive junction reads
	Sources       []Supplier    `json:"sources"`    // by gate, then field
	Reviews       []Review      `json:"reviews"`    // the reviewed junctions the status passes
	Models        []ModelCheck  `json:"models"`     // oldest first
	Events        []Event       `json:"events"`     // the newest five, newest first
	EventCount    int           `json:"event_count"`
	Commands      []Command     `json:"commands"`
}

// Parent is the parent of a task and the task's place under it.
type Parent struct {
	TaskRef
	Order *int `json:"order"` // null when the file states none
}

// Child is a child with its place and its status.
type Child struct {
	TaskRef
	Order  *int   `json:"order"`
	Status Status `json:"status"`
}

// Authority is an authority of a task and the ancestor that makes it one.
type Authority struct {
	Email string `json:"email"`
	By    string `json:"by"` // the ancestor's id
}

// Snapshot is a subproject's task as a recursive junction reads it.
type Snapshot struct {
	SubRef
	Assignee string  `json:"assignee"`
	Status   *Status `json:"status"` // null when the task is not read
	Children []Child `json:"children"`
}

// Authorisation is a task's authorisation and the commit that decides it.
type Authorisation struct {
	Commit           *Commit  `json:"commit"` // the deciding commit; null when none decides
	Way              string   `json:"way"`    // change, merge or trailer; "" with no commit
	By               string   `json:"by"`     // author, committer or both: which of them is an authority; "" for a proposed task
	Email            string   `json:"email"`  // the authority that accepts; else the author; "" with no commit
	Judges           []string `json:"judges"` // the emails that may accept, nearest first
	Why              string   `json:"why"`    // proposed with no deciding commit: no_history, no_trunk, off_trunk or no_commit
	DiffersFromTrunk bool     `json:"differs_from_trunk"`
	Uncommitted      bool     `json:"uncommitted"`
}

// Linkage is one subproject a recursive junction reads, and what fixes the
// commit it reads.
type Linkage struct {
	Gates []string `json:"gates"` // the gates whose junctions read it, in order
	SubRef
	Stated
	StatedCommit string  `json:"stated_commit"` // the commit field as the entry writes it; "" for none
	Recorded     string  `json:"recorded"`      // a submodule: the gitlink of the source commit
	Moved        bool    `json:"moved"`         // a submodule: the checkout stands elsewhere
	Trunk        string  `json:"trunk"`         // the name of the subproject's trunk; "" when undetermined
	Tip          string  `json:"tip"`           // the tip of that trunk, in full
	OnTrunk      string  `json:"on_trunk"`      // unknown, no or yes
	Behind       string  `json:"behind"`        // unknown, no or yes
	Pinned       *Commit `json:"pinned"`        // the commit read, as the subproject's history gives it
	SetBy        *Commit `json:"set_by"`        // the commit of the newest pin event of the link
}

// Review is a reviewed junction the status passes and what accepts it.
type Review struct {
	Gate     string  `json:"gate"`
	Reviewer string  `json:"reviewer"`
	Effect   string  `json:"effect"` // accepts, authorisation or none
	By       string  `json:"by"`     // the reviewer's email on the commit; "" for none
	Commit   *Commit `json:"commit"` // null while nothing accepts
}

// ModelCheck compares a junction's model with a commit's Model: trailer.
type ModelCheck struct {
	Commit  string `json:"commit"` // in full
	Gate    string `json:"gate"`
	Stated  string `json:"stated"`
	Trailer string `json:"trailer"`
	Reading string `json:"reading"` // agrees, differs, missing, exempt or unstated
}

// Event is one event of the task.
type Event struct {
	Date       string       `json:"date"`
	Commit     string       `json:"commit"`     // in full
	By         string       `json:"by"`         // the author's email
	Kind       string       `json:"kind"`       // task, authorised, status, reaffirmed, reviewed or pin
	Recorded   *EventStatus `json:"recorded"`   // status: what the commit records
	Gate       string       `json:"gate"`       // reviewed
	Pin        *PinMove     `json:"pin"`        // pin
	Subproject *SubEvent    `json:"subproject"` // an event taken from a subproject
}

// EventStatus is what a status event records.
type EventStatus struct {
	Gate   string `json:"gate"`
	State  string `json:"state"`
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

// PinMove is a pin that moves from one commit to another.
type PinMove struct {
	URL string `json:"url"`
	Old string `json:"old"` // "" when the linkage first appears
	New string `json:"new"`
}

// SubEvent names the subproject task an event comes from.
type SubEvent struct {
	URL  string `json:"url"`
	Task string `json:"task"`
}
```

## `view`: the authority delegation

```go
package view

// Delegation is the authority delegation view, the data of the command
// authority. Authority names one authority of a task.
type Delegation struct {
	Legend Legend          `json:"legend"`
	Rows   []DelegationRow `json:"rows"` // every task in view, in display order
	*DelegationDetail
	*DelegationProvenance
}

// DelegationDetail is what the authority delegation adds at detail.
type DelegationDetail struct {
	Parents []Chain `json:"parents"` // every row that has children, in display order
}

// DelegationProvenance is what the authority delegation adds at provenance.
type DelegationProvenance struct {
	Deciding  []Deciding  `json:"deciding"`  // newest first
	Undecided []Undecided `json:"undecided"` // the rows no commit decides
	Commands  []Command   `json:"commands"`
}

// DelegationRow is one task of the tree.
type DelegationRow struct {
	TaskRef
	Depth            int    `json:"depth"` // 0 for the task the view starts at
	Assignee         string `json:"assignee"`
	Parent           bool   `json:"parent"`    // the task has children
	Delegated        bool   `json:"delegated"` // its assignee differs from its parent's
	Proposed         bool   `json:"proposed"`
	DiffersFromTrunk bool   `json:"differs_from_trunk"`
	Uncommitted      bool   `json:"uncommitted"`
}

// Chain is what one parent gives its subtree.
type Chain struct {
	TaskRef
	Assignee    string      `json:"assignee"`
	Authorities []Authority `json:"authorities"` // of its children, nearest first
	Children    []string    `json:"children"`    // in display order
	Defaults    []Junction  `json:"defaults"`    // the gates at which the parent's file states an entry, resolved
	Sources     []Supplier  `json:"sources"`     // of those junctions, by gate, then field
}

// Deciding groups the tasks one commit decides in one way.
type Deciding struct {
	Commit Commit   `json:"commit"`
	Way    string   `json:"way"`    // change, merge or trailer
	Merged string   `json:"merged"` // a merge: the commit it brings in, in full; else ""
	By     string   `json:"by"`     // author, committer or both; "" where the tasks stay proposed
	Email  string   `json:"email"`  // the authority that accepts; else the author
	Tasks  []string `json:"tasks"`  // in display order
}

// Undecided groups the rows that have no deciding commit for one reason.
type Undecided struct {
	Why   string   `json:"why"` // no_history, no_trunk, off_trunk or no_commit
	Tasks []string `json:"tasks"`
}
```

## `view`: the task assignment

```go
package view

// Assignment is the task assignment view.
type Assignment struct {
	Legend    Legend      `json:"legend"`
	People    []PersonRow `json:"people"`    // one per email in view, in order of first appearance
	Recursive []string    `json:"recursive"` // the leaves in view whose next junction is recursive
	*AssignmentDetail
}

// AssignmentDetail is what the task assignment adds at detail. At provenance
// each section also carries its positions.
type AssignmentDetail struct {
	Sections []PersonSection `json:"sections"` // one per row of People, in its order
}

// PersonRow is the glance of one email.
type PersonRow struct {
	Email           string   `json:"email"`
	Owner           bool     `json:"owner"`
	Assigned        int      `json:"assigned"`
	ContributesNext int      `json:"contributes_next"`
	ReviewsNext     int      `json:"reviews_next"`
	Models          []string `json:"models"`
}

// PersonSection is what one email carries.
type PersonSection struct {
	Email       string         `json:"email"`
	Assigned    []AssignedRow  `json:"assigned"`
	Recursive   []RecursiveRow `json:"recursive"`
	Contributes []JunctionRow  `json:"contributes"` // gate order, then display order
	Reviews     []JunctionRow  `json:"reviews"`
	Models      []ModelRow     `json:"models"`
	Counts      []GateCount    `json:"counts"` // every gate, undefined first
	Authority   []Subtree      `json:"authority"`
	*SectionProvenance
}

// SectionProvenance is what a section adds at provenance.
type SectionProvenance struct {
	Positions []Position `json:"positions"`
}

// AssignedRow is a task assigned to an email.
type AssignedRow struct {
	TaskRef
	Under    string    `json:"under"` // the parent's id; "" for the root
	Status   Status    `json:"status"`
	Next     *Junction `json:"next"`      // null for a parent and at the last gate
	NextGate string    `json:"next_gate"` // "" where Next is null
}

// RecursiveRow is an assigned task whose next junction a subproject carries.
type RecursiveRow struct {
	Task       TaskRef `json:"task"`
	Gate       string  `json:"gate"`
	Subproject SubRef  `json:"subproject"`
	Status     *Status `json:"status"` // of the task read there; null when it is not read
}

// JunctionRow is one junction where an email contributes or reviews.
type JunctionRow struct {
	Task TaskRef `json:"task"`
	Junction
}

// ModelRow counts the junctions at one gate where an agent runs one model.
type ModelRow struct {
	Gate      string `json:"gate"`
	Model     string `json:"model"`
	StatedBy  string `json:"stated_by"` // the task whose entry states it
	Junctions int    `json:"junctions"`
	Next      int    `json:"next"`
}

// GateCount is what an email holds at one gate.
type GateCount struct {
	Gate            string `json:"gate"`
	Standing        int    `json:"standing"` // the tasks assigned whose status stands at the gate
	Contributes     int    `json:"contributes"`
	ContributesNext int    `json:"contributes_next"`
	Reviews         int    `json:"reviews"`
	ReviewsNext     int    `json:"reviews_next"`
}

// Subtree is a task an email has authority under, with its descendants.
type Subtree struct {
	Task        TaskRef `json:"task"`
	Descendants int     `json:"descendants"`
}

// Position groups the tasks at which one source gives an email a position.
type Position struct {
	Kind         string   `json:"kind"` // assignee, authority, contributes, reviews or recursive
	Gate         string   `json:"gate"` // "" for assignee and authority
	Tasks        []string `json:"tasks"`
	Reviewer     string   `json:"reviewer"`      // contributes: the junction's reviewer
	From         Stated   `json:"from"`          // the assignee, the contributor with its model, or the subproject
	ReviewerFrom Stated   `json:"reviewer_from"` // contributes and reviews
	URL          string   `json:"url"`           // recursive
	Pin          string   `json:"pin"`           // recursive
}
```

## `internal/views`: the functions

```go
// Package views builds each view's data from the derived facts. It selects,
// counts and words; it derives no fact and reads no file, process, clock or
// environment.
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

// Gates returns the gate definition. The Derivation does not refuse f, and
// q.Task is "" or a task of the project.
func Gates(f *derive.Facts, q Query) *view.Gates

// Task returns the task definition of q.Task, a task of the project.
func Task(f *derive.Facts, q Query) *view.Task

// Authority returns the authority delegation of the subtree under q.Task.
func Authority(f *derive.Facts, q Query) *view.Delegation

// Assignment returns the task assignment of the subtree under q.Task.
func Assignment(f *derive.Facts, q Query) *view.Assignment

// builder holds the facts and the query of one call and builds the shared
// types. Its methods are the rows of the table of shared types, one each;
// the files of 886d and 8ed1 call them and write none again.
type builder struct {
	f *derive.Facts
	q Query
}

func (b builder) ref(id string) view.TaskRef                           // of the project in view
func (b builder) legend() view.Legend
func (b builder) refs(list []*model.Reference) []view.Reference        // never nil
func (b builder) status(f *derive.Facts, s *derive.Status) view.Status // f holds the status's task
func (b builder) chain(id string) []view.Step
func (b builder) commit(c *history.Commit) *view.Commit                // nil for nil
func (b builder) junction(j *derive.Junction, from bool) view.Junction // from: carry From
func (b builder) subRef(n *derive.Snapshot) *view.SubRef               // nil for nil
func (b builder) suppliers(j *derive.Junction) []view.Supplier
func (b builder) requirement(c *derive.Condition, dependent bool) view.Requirement
func (b builder) subtree(top string) []string // top and its descendants, in display order
func (b builder) parent(id string) string     // f.Authorities(id)[0].Task; "" for the root
func why(w derive.Why) string
```

`snapshot.go` of task `4ed9` holds the one place that joins a view to its function (design 4ed9, `derivers`). `query` copies `Task`, `Person` and `Proposed` from the resolved `Params` and maps the resolved `tablo.Level` to `views.Level`.

```go
var derivers = map[View]deriver{
	Gates:      func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Gates(s.facts, query(p)), nil },
	Task:       func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Task(s.facts, query(p)), nil },
	Authority:  func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Authority(s.facts, query(p)), nil },
	Assignment: func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Assignment(s.facts, query(p)), nil },
}
```

## `internal/derive`: one method more

```go
// LastChange returns the newest commit the source reaches that changes the
// file at a path below .tableaux, as "gates.yaml" or "tasks/9f31.yaml", or
// nil when none does and without history.
func (f *Facts) LastChange(below string) *history.Commit
```

Its body is `f.newest(path.Join(f.p.Where.Dir, ".tableaux", below), "")` between `f.enter()` and `f.leave()`, as every method of `Facts` runs.
