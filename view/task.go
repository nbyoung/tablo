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
	Snapshot    *Snapshot     `json:"snapshot"`  // a snapshot status: the task read; else null
}

// TaskProvenance is what the task definition adds at provenance. There the
// junctions also carry From.
type TaskProvenance struct {
	StatusCommit  *Commit       `json:"status_commit"` // the deciding commit of the last status of the chain; null without one
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
