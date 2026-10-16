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
	Why    string  `json:"why"`    // authorisation: the task's Authorisation.Why, "" where a commit decides but not by an authority; else ""
	*ItemDetail
}

// Wait is one reason a work item waits.
type Wait struct {
	Kind     string       `json:"kind"`     // requirement, reason, authorisation or review
	Requires *Requirement `json:"requires"` // requirement: the unmet entry; else null
	Reason   string       `json:"reason"`   // reason: the status's reason, a key of Legend.Reasons other than review; else ""
	Note     string       `json:"note"`     // reason: the status's note; else ""
	By       string       `json:"by"`       // authorisation: the nearest authority; review: the reviewer; else ""
}

// ItemDetail is what an item adds at detail.
type ItemDetail struct {
	Junction       *Junction      `json:"junction"`         // the task's next junction, with From; null for a parent and at the last gate
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
// tabloio's public package brief reads; the envelope states the person and
// the ref.
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
