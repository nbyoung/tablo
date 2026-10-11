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

// Cell is one task at one gate. A cell at the gate the status names shows
// the status; one before it is empty, or shows its junction where the view
// shows historical junctions; one after it shows its junction's marks, or
// reads exempt where the gate does not apply.
type Cell struct {
	Kind       string   `json:"kind"`       // empty, status, marks or exempt
	Historical bool     `json:"historical"` // the gate lies before the gate the status names
	State      string   `json:"state"`      // status: a key of Legend.States; else ""
	Reason     string   `json:"reason"`     // status: a key of Legend.Reasons, or ""
	Marks      []string `json:"marks"`      // marks: keys of Legend.Marks, the contributor's first, a parent's as its subtree inherits them; else empty
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
