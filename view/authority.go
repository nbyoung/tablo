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
