package view

// History is the history view: what each commit does to each task in view,
// oldest first. The counts cover the project's own lines, and they are the
// same at every level.
type History struct {
	Legend  Legend       `json:"legend"`
	Subject TaskRef      `json:"subject"` // the task whose subtree is in view: the task parameter, the root by default
	Events  int          `json:"events"`  // the events the project's own lines hold
	Commits int          `json:"commits"` // the distinct commits of those lines
	Kinds   []EventCount `json:"kinds"`   // the six kinds of SYNTAX.md#history, in its order, zeros included
	Days    []Day        `json:"days"`    // one per author date of those lines, in the order the lines first name it
	Lines   []Line       `json:"lines"`   // oldest first; a subproject's lines from detail on
	*HistoryProvenance
}

// HistoryProvenance is what the history adds at provenance. There each line
// also carries its LineProvenance.
type HistoryProvenance struct {
	Commands []Command `json:"commands"` // the two git log commands of README.md#history, over the subtree and the range
}

// EventCount counts the events of one kind.
type EventCount struct {
	Kind  string `json:"kind"` // task, authorised, status, reaffirmed, reviewed or pin
	Count int    `json:"count"`
}

// Day counts the project's own lines of one author date.
type Day struct {
	Date    string       `json:"date"` // YYYY-MM-DD
	Events  int          `json:"events"`
	Commits int          `json:"commits"`
	Kinds   []EventCount `json:"kinds"` // the kinds the day holds, in the order of SYNTAX.md#history; no zero
}

// Line is what one commit does to one task: its events there, each in the
// shape the task definition gives an event. The events of a line share the
// commit, the author, the task and, for a line taken from a subproject, the
// subproject task.
type Line struct {
	Task     TaskRef `json:"task"`     // the task of the project in view
	Proposal bool    `json:"proposal"` // off the trunk: the trunk's tip does not reach the commit
	Events   []Event `json:"events"`   // never empty; in the order of the kinds
	*LineDetail
	*LineProvenance
}

// LineDetail is what a line adds at detail.
type LineDetail struct {
	After        *Status     `json:"after"`         // the task's status as the files stand at the commit; a taken line's, of the subproject task; null where those files lack the task
	Unchanged    bool        `json:"unchanged"`     // the files at one of the commit's parents state the same status
	SubjectAfter *Status     `json:"subject_after"` // the subject's status at the commit, on a line of another task of the project; else null
	Effect       string      `json:"effect"`        // what the line does to the task's acceptance or review; "" for nothing
	Model        string      `json:"model"`         // the commit's first Model: trailer; "" for none
	Check        *ModelCheck `json:"check"`         // the model check of the line's status or reaffirmed event; null where it reads no model
	Committer    string      `json:"committer"`     // the committer's email where it differs from the author's; else ""
	CoAuthor     string      `json:"co_author"`     // the first agent a Co-Authored-By: trailer names on a person's commit; else ""
}

// LineProvenance is what a line adds at provenance.
type LineProvenance struct {
	Commit *Commit `json:"commit"` // the commit, of the project or of the subproject the line comes from
}
