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

// Cause is one root of the tree. A snapshot is a cause in two ways: the pin
// is behind its trunk and its advance changes what the project sees, or the
// task read has passed the next gate of the leaf that reads it, and that
// leaf's own gate advances (design 886d, decision 11).
type Cause struct {
	Kind     string  `json:"kind"`
	Task     TaskRef `json:"task"`     // the task the cause stands at; a snapshot of a pin: the first task it holds
	URL      string  `json:"url"`      // a cross-project requirement: the project of Task; a snapshot: the link; else ""
	Gate     string  `json:"gate"`     // requirement: the gate not passed; review: the gate under review; else the gate the status names
	Status   Status  `json:"status"`   // of Task
	Pin      string  `json:"pin"`      // snapshot: the commit the link reads, in full
	Tip      string  `json:"tip"`      // snapshot: the tip of the subproject's trunk, in full
	Resolver string  `json:"resolver"` // the email of the person who acts; "" where the files name none
	Mark     string  `json:"mark"`     // the resolver's mark: person, agent, reviewer or subproject; "" for none
	Act      string  `json:"act"`      // contributes, records, reviews, authorises or advances
	Advance  string  `json:"advance"`  // snapshot: what advances, pin or gate; else ""
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
