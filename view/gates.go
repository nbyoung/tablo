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
