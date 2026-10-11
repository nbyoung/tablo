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
