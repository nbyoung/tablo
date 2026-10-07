# Design 886d: the declarations

The Go surface of [design 886d](../886d.md). It adds three files to the package `view` and three to `internal/views`, both of [design 493e](../493e/model.md), and changes no declaration there but the four its [Assumed interfaces](../886d.md#assumed-interfaces) name. A trial outside the repository compiles the three `view` blocks as they stand, beside the five of 493e, against the code at `5779017`, with a body for each of the four functions, and produces the files under [`golden/`](golden/).

| Block                                                    | Lands as                                               |
|----------------------------------------------------------|--------------------------------------------------------|
| [`view`: the two tableaux](#view-the-two-tableaux)       | `view/tableau.go`                                      |
| [`view`: the work queue](#view-the-work-queue)           | `view/queue.go`                                        |
| [`view`: the work-blockage tree](#view-the-work-blockage-tree) | `view/blockage.go`                               |
| [`internal/views`: the functions](#internalviews-the-functions) | `internal/views/views.go` (changed), `tableau.go`, `queue.go`, `blockage.go` |

A type this file names and does not declare is one of 493e: `Legend`, `Gate`, `TaskRef`, `Column`, `Status`, `Step`, `Commit`, `Command`, `Junction`, `Supplier`, `Requirement`, `Reference`, `Linkage`, `Authorisation`.

## `view`: the two tableaux

```go
package view

// Tableau is the global tableau and the contextual tableau: the commands
// tableau and context both return it. Detail adds rows and no key.
type Tableau struct {
	Legend  Legend    `json:"legend"`
	Subject *TaskRef  `json:"subject"` // context with a task: that task; else null
	Columns []Column  `json:"columns"` // every gate once, in order, alone or in a folded run
	Rows    []GridRow `json:"rows"`    // the rows of the level, in display order
	*TableauProvenance
}

// TableauProvenance is what a tableau adds at provenance. There each row
// also carries its RowProvenance.
type TableauProvenance struct {
	Commands []Command `json:"commands"`
}

// GridRow is one task of a tableau.
type GridRow struct {
	TaskRef
	Under  string `json:"under"`  // the parent's id; "" for the root
	Depth  int    `json:"depth"`  // the number of its ancestors that the data holds a row for
	Parent bool   `json:"parent"` // the task has children
	Hidden int    `json:"hidden"` // the tasks beneath it with no row, whose nearest ancestor with a row it is
	Label  string `json:"label"`  // context: "", spine or sibling; tableau: ""
	Status Status `json:"status"`
	Leaf   *Leaf  `json:"leaf"`  // a roll-up or a snapshot: the task whose file states the status; else null
	Cells  []Cell `json:"cells"` // one per gate of Legend.Gates, in its order, whatever the columns
	*RowProvenance
}

// Leaf names the task at the end of a status's chain.
type Leaf struct {
	Task TaskRef `json:"task"`
	URL  string  `json:"url"` // the subproject the task is of; "" in the project in view
}

// Cell is one task at one gate.
type Cell struct {
	Kind       string   `json:"kind"`       // empty, status, marks or exempt
	Historical bool     `json:"historical"` // the gate lies before the gate the status names
	State      string   `json:"state"`      // status: a key of Legend.States; else ""
	Reason     string   `json:"reason"`     // status: a key of Legend.Reasons, or ""
	Marks      []string `json:"marks"`      // marks: keys of Legend.Marks, the contributor's first; else empty
	Acts       bool     `json:"acts"`       // the person in view contributes or reviews here, and the work is to come
}

// RowProvenance is what a row adds at provenance: the fields of the same
// names of the task definition, for this row's task.
type RowProvenance struct {
	StatusCommit *Commit    `json:"status_commit"` // the deciding commit of the last status of the chain
	OwnCommit    *Commit    `json:"own_commit"`    // a snapshot: the deciding commit of the task's own file
	Chain        []Step     `json:"chain"`         // the statuses the status comes through
	Junctions    []Junction `json:"junctions"`     // every gate after undefined, in order, each with From
	Sources      []Supplier `json:"sources"`       // of those junctions, by gate, then field
	Linkages     []Linkage  `json:"linkages"`      // one per subproject a recursive junction reads
}
```

## `view`: the work queue

```go
package view

// Queue is the contributor work queue of one person.
type Queue struct {
	Legend Legend      `json:"legend"`
	Items  []QueueItem `json:"items"` // kind order, then display order; reaffirmations oldest first; empty with a brief
	*QueueProvenance
}

// QueueProvenance is what the queue adds at provenance.
type QueueProvenance struct {
	Brief *Brief `json:"brief"` // with the brief parameter, when the queue holds that work; else null
}

// QueueItem is one thing the person does.
type QueueItem struct {
	Kind   string  `json:"kind"` // review, authorisation, ready, reaffirmation or waiting
	Task   TaskRef `json:"task"`
	Gate   string  `json:"gate"`   // review, ready, waiting: the next gate; authorisation, reaffirmation: the gate the status names
	Model  string  `json:"model"`  // review, ready, waiting: the model the junction states; else ""
	Since  string  `json:"since"`  // the date of the status; "" when undetermined
	Age    int     `json:"age"`    // reaffirmation: days from Since to the date of the source commit; else 0
	Causes []Wait  `json:"causes"` // waiting: why, in the order requirement, reason, authorisation, review; else empty
	*ItemDetail
}

// Wait is one reason a work item waits.
type Wait struct {
	Kind     string       `json:"kind"`     // requirement, reason, authorisation or review
	Requires *Requirement `json:"requires"` // requirement: the unmet entry; else null
	Reason   string       `json:"reason"`   // reason: blocked or overloaded; else ""
	By       string       `json:"by"`       // authorisation: the nearest authority; review: the reviewer; else ""
}

// ItemDetail is what an item adds at detail.
type ItemDetail struct {
	Junction       *Junction      `json:"junction"`         // review, ready, waiting: the next junction, with From; else null
	Sources        []Supplier     `json:"sources"`          // of that junction, by field
	TaskReferences []Reference    `json:"task_references"`  // the task's own
	Requires       []Requirement  `json:"requires"`         // in file order
	Unblocks       []Requirement  `json:"unblocks"`         // the dependents that passing Gate meets
	AlsoRequiredBy []Requirement  `json:"also_required_by"` // the other dependents
	ParentUnblocks []ParentEdge   `json:"parent_unblocks"`  // the same for the parent's dependents
	Status         Status         `json:"status"`
	StatusCommit   *Commit        `json:"status_commit"` // the deciding commit of the status; null without one
	Authorisation  *Authorisation `json:"authorisation"` // authorisation: the task's; else null
}

// ParentEdge is a requirement on an item's parent that the item's gate
// would let pass.
type ParentEdge struct {
	Parent   TaskRef     `json:"parent"`
	Requires Requirement `json:"requires"` // Task is the dependent
}

// Brief is one work item with every fact a brief states. Its keys are those
// tabloio's internal/brief reads; the envelope states the person and the ref.
type Brief struct {
	Kind       string         `json:"kind"`   // ready or waiting
	Causes     []Wait         `json:"causes"` // as the item's
	Task       BriefTask      `json:"task"`
	Gate       BriefGate      `json:"gate"`
	Junction   BriefJunction  `json:"junction"`
	Supplier   *BriefSupplier `json:"supplier"`   // the task whose entry states the contributor; null for the task itself and for no entry
	Requires   []BriefEdge    `json:"requires"`   // in file order
	Dependents []BriefEdge    `json:"dependents"` // in the Derivation's order
	Parent     *BriefParent   `json:"parent"`     // null for the root
	Status     BriefStatus    `json:"status"`
}

// BriefTask is the task of a brief.
type BriefTask struct {
	TaskRef
	Description string      `json:"description"`
	References  []Reference `json:"references"`
	Path        []TaskRef   `json:"path"` // the ancestors, the root first, without the task
}

// BriefGate is the gate the work reaches, with its legend.
type BriefGate struct {
	Gate
	Follows string `json:"follows"` // the kind of the task's junction at its next applicable gate after this one; "" at the last
}

// BriefJunction is the junction, each field with its source.
type BriefJunction struct {
	Contributor BriefField       `json:"contributor"`
	Model       BriefField       `json:"model"`
	Reviewer    BriefField       `json:"reviewer"`
	References  []BriefReference `json:"references"`
}

// BriefField is one resolved value and its source.
type BriefField struct {
	Value  string      `json:"value"`
	Source BriefSource `json:"source"`
}

// BriefSource names where a field resolves from.
type BriefSource struct {
	Kind  string `json:"kind"` // task, assignee or default
	ID    string `json:"id"`   // task: the task whose entry states it; else ""
	Title string `json:"title"`
}

// BriefReference is a junction reference with its source.
type BriefReference struct {
	URL    string      `json:"url"`
	Text   string      `json:"text"`
	Source BriefSource `json:"source"`
}

// BriefSupplier is the task whose entry sends the work to the contributor.
type BriefSupplier struct {
	TaskRef
	Description string `json:"description"`
}

// BriefEdge is one requirement, seen from either end.
type BriefEdge struct {
	TaskRef          // the other task
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text"`
	Condition string `json:"condition"` // met, unmet or pending; "" when undetermined
	URL       string `json:"url"`       // a cross-project entry: its url; else ""
	Commit    string `json:"commit"`    // a cross-project entry: the commit it reads, in full
}

// BriefParent is the task's parent with the tasks that require it.
type BriefParent struct {
	TaskRef
	Dependents []BriefEdge `json:"dependents"`
}

// BriefStatus is the task's status with its symbols and its file.
type BriefStatus struct {
	Path         string `json:"path"` // the status file, from the repository root
	Gate         string `json:"gate"`
	GateSymbol   string `json:"gate_symbol"`
	State        string `json:"state"`
	StateSymbol  string `json:"state_symbol"`
	Reason       string `json:"reason"`
	ReasonSymbol string `json:"reason_symbol"`
	Note         string `json:"note"`
	Date         string `json:"date"`
	Recorder     string `json:"recorder"`
	Commit       string `json:"commit"` // the deciding commit, in full; "" without one
}
```

What fills a brief. `id` and `gate` are the brief parameter's, `j` is `f.Junction(id, gate)` and `s` is `f.Status(id)`.

| Field                         | Filled from                                                                                                   |
|-------------------------------|---------------------------------------------------------------------------------------------------------------|
| `kind`, `causes`              | The item's                                                                                                    |
| `task`                        | `ref(id)`; `Description.V`; `refs(References)`; `path` is `f.Authorities(id)` reversed, each as a `TaskRef`    |
| `gate`                        | The item of `Legend.Gates` with the key; `follows` is `f.Junction(id, f.Next(id, gate)).Kind.String()`, `""` when `Next` gives none |
| `junction.contributor`, `.model`, `.reviewer` | `value` is `Field.V`; `source` is `task` with the id and title of `Field.Task` for `ByTask`, `assignee` for `ByAssignee`, `default` for `ByDefault`, the last two with no id |
| `junction.references`         | `j.References`, each with the source `task` and `j.ReferencesFrom`                                             |
| `supplier`                    | When `j.Contributor.By` is `ByTask` and `j.Contributor.Task` is not `id`: that task with its `Description.V`; else null |
| `requires`                    | `f.Requires(id)`: the originating task with its title from `Condition.Facts`; `From`, `To`; `Entry.Text.V`; `Word()`; `Link.URL` and `Link.Commit`, `""` without a link |
| `dependents`                  | `f.Dependents(id)`, the same with `Condition.Task` as the other task                                           |
| `parent`                      | `parent(id)` with `f.Dependents` of it; null for the root                                                      |
| `status`                      | `path` is `path.Join(f.Project().Where.Dir, ".tableaux", "status", id+".yaml")`; `Gate`, `State`, `Reason`, `Note`, `Date`, `Recorder` of `s`; each symbol from the legend, `""` for a key it lacks; `commit` is `s.Commit.ID`, `""` for nil |

## `view`: the work-blockage tree

```go
package view

// Blockage is the work-blockage tree.
type Blockage struct {
	Legend      Legend      `json:"legend"`
	Causes      []Cause     `json:"causes"`        // largest first
	Kinds       []KindCount `json:"kinds"`         // the five kinds, in VIEWS.md's order, each with its causes in view
	NotDueCount int         `json:"not_due_count"` // the requirements not yet due
	NotDueUnmet int         `json:"not_due_unmet"` // those of them not met
	*BlockageDetail
	*BlockageProvenance
}

// BlockageDetail is what the tree adds at detail. There each cause also
// carries its CauseDetail.
type BlockageDetail struct {
	NotDue []Waiting `json:"not_due"` // by task, in display order
}

// BlockageProvenance is what the tree adds at provenance. There each cause
// also carries its CauseProvenance.
type BlockageProvenance struct {
	Commands []Command `json:"commands"`
}

// KindCount counts the causes of one kind.
type KindCount struct {
	Kind  string `json:"kind"` // requirement, status, review, authorisation or snapshot
	Count int    `json:"count"`
}

// Cause is one root of the tree.
type Cause struct {
	Kind     string  `json:"kind"`
	Task     TaskRef `json:"task"`     // the task the cause stands at; a snapshot: the first task it holds
	URL      string  `json:"url"`      // a cross-project requirement: the project of Task; a snapshot: the link; else ""
	Gate     string  `json:"gate"`     // requirement: the gate not passed; review: the gate under review; else the gate the status names
	Status   Status  `json:"status"`   // of Task
	Pin      string  `json:"pin"`      // snapshot: the commit the link reads, in full
	Tip      string  `json:"tip"`      // snapshot: the tip of the subproject's trunk, in full
	Resolver string  `json:"resolver"` // the email of the person who acts; "" when no person of this project does
	Mark     string  `json:"mark"`     // the resolver's mark: person, agent, reviewer or subproject; "" for none
	Act      string  `json:"act"`      // contributes, records, reviews, authorises or advances
	Holds    int     `json:"holds"`    // the distinct tasks beneath, at every depth
	*CauseDetail
	*CauseProvenance
}

// CauseDetail is what a cause adds at detail.
type CauseDetail struct {
	Held []Held `json:"held"`
}

// CauseProvenance is what a cause adds at provenance.
type CauseProvenance struct {
	StatusCommit  *Commit        `json:"status_commit"` // the deciding commit of Status; null without one
	Junction      *Junction      `json:"junction"`      // the next junction of Task, with From; null when it has none
	Authorisation *Authorisation `json:"authorisation"` // authorisation: the task's; else null
	Linkage       *Linkage       `json:"linkage"`       // snapshot: the link; else null
	Resolve       []Command      `json:"resolve"`       // review, authorisation: the commit that resolves; else empty
}

// Held is a task a cause holds, and what it holds in turn.
type Held struct {
	Task        TaskRef      `json:"task"`
	Gate        string       `json:"gate"`        // the gate it waits to pass
	Parent      bool         `json:"parent"`      // the task has children
	Via         string       `json:"via"`         // self, parent or requirement
	Child       string       `json:"child"`       // parent: the child that holds it
	Requires    *Requirement `json:"requires"`    // requirement: the entry; else null
	Contributor string       `json:"contributor"` // who contributes at Gate; "" on a parent and where the junction is not plain
	Also        []int        `json:"also"`        // the other causes it stands under, counted from 1 in Causes
	Held        []Held       `json:"held"`
}

// Waiting is a task with requirements not yet due.
type Waiting struct {
	Task     TaskRef       `json:"task"`
	Status   Status        `json:"status"`
	Next     string        `json:"next"` // its next gate; "" at the last
	Requires []Requirement `json:"requires"`
}
```

## `internal/views`: the functions

`views.go` of 493e gains four fields of `Query` and one type; the four functions and the methods below land in the three new files.

```go
// BriefOf names the one queue item the queue writes as a brief.
type BriefOf struct{ Task, Gate string }

// Query is what a view takes: the parameters, checked and resolved.
type Query struct {
	Task       string // a task of the project; "" is the root, no task on the gate definition, and the person form on context
	Person     string // an email; "" is nobody
	Level      Level
	Proposed   bool     // authority: proposed tasks only
	Window     int      // tableau, context: the columns either side of the next gates
	Columns    []string // tableau, context: the gates to show, each a gate of the project, in place of the window; empty for none
	Historical bool     // tableau, context: show the marks of historical junctions
	Brief      *BriefOf // queue: write this one item as a brief; its task and gate are of the project
}

// Tableau returns the global tableau. It reads no q.Task.
func Tableau(f *derive.Facts, q Query) *view.Tableau

// Context returns the contextual tableau: of the subtree under q.Task, or,
// with q.Task empty, of q.Person.
func Context(f *derive.Facts, q Query) *view.Tableau

// Queue returns the work queue of q.Person within the subtree under q.Task,
// or, with q.Brief, the brief of one of its items.
func Queue(f *derive.Facts, q Query) *view.Queue

// Blockage returns the work-blockage tree, selected by q.Task and q.Person.
func Blockage(f *derive.Facts, q Query) *view.Blockage

func (b builder) next(id string) string                 // f.Next(id, f.Status(id).Gate): the task's next gate, or ""
func (b builder) nextJunction(id string) *derive.Junction // f.Junction(id, b.next(id)), or nil
func (b builder) own(person string) []string            // the person's own tasks, in display order
func (b builder) proposed(id string) bool               // the trunk's history decides the task proposed
func (b builder) authority(id string) string            // f.Authorities(id)[0].Email; f.Owner() for the root
func (b builder) columns(focus, counted []string) []view.Column // the window from the focus, the counts from the tasks counted
func (b builder) cells(id string) []view.Cell           // one per gate
func (b builder) leaf(id string) *view.Leaf             // the end of f.Chain(id); nil for a chain of one
func (b builder) grid(subject *view.TaskRef, focus, counted, rows []string, labels map[string]string) *view.Tableau
func (b builder) items() []view.QueueItem               // the person's items, in the queue's order
func (b builder) waits(id string) []view.Wait           // why the work at a leaf's next junction waits
func (b builder) brief(item view.QueueItem) *view.Brief
```

Three values that 493e's `task.go` builds for the task definition become methods of `builder`, since the status views carry the same facts for other tasks:

```go
func (b builder) statusCommit(id string) *view.Commit        // the Commit of the last status of f.Chain(id); nil without one
func (b builder) authorisation(id string) view.Authorisation // as TaskProvenance.Authorisation
func (b builder) linkages(id string) []view.Linkage          // as TaskProvenance.Linkages; never nil
```

`snapshot.go` of task `4ed9` gains four entries. `query` also copies `Columns` and `Historical`, writes `Window` as 1 for a nil `Params.Window`, and maps `Params.Brief` to a `*views.BriefOf`.

```go
	Tableau:  func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Tableau(s.facts, query(p)), nil },
	Context:  func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Context(s.facts, query(p)), nil },
	Queue:    func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Queue(s.facts, query(p)), nil },
	Blockage: func(_ context.Context, s *Snapshot, p Params) (any, error) { return views.Blockage(s.facts, query(p)), nil },
```
