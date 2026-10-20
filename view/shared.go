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
	Key  string `json:"key"`  // the key in that task's file: junctions.<gate>.<field>, assignee or subproject, the task-level field; "" for a default
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
// fold stands for. The two tableaux alone carry columns. The window and the
// fold say which gates show; a row holds one cell per gate whatever they
// say. The counts hold on every column, folded or not, and are those of the
// rows of the detail level at every level.
type Column struct {
	Gates  []string `json:"gates"` // one gate, or the run a folded column stands for
	Folded bool     `json:"folded"`
	Count  int      `json:"count"`   // the rows of the detail level whose status names a gate of the column
	ByGate []int    `json:"by_gate"` // the same count per gate of the column
}
