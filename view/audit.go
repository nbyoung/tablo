package view

// Audit is the audit view. The three counts and Most cover every finding in
// view, whatever the minimum; Findings lists the rows at or above it.
type Audit struct {
	Legend      Legend     `json:"legend"`
	Errors      int        `json:"errors"`
	Warnings    int        `json:"warnings"`
	Information int        `json:"information"`
	Most        *Resolving `json:"most"`     // who has the most to resolve; null when no finding asks an act of a person
	Findings    []Finding  `json:"findings"` // the rows at or above the minimum, in the order of the design
	*AuditDetail
}

// AuditDetail is what the audit adds at detail. There each finding also
// carries its FindingDetail, and at provenance its FindingProvenance.
type AuditDetail struct {
	Silent []Silence `json:"silent"` // the kinds examined that no finding in view has
}

// Finding is one row of the audit: the tasks that one kind hits at one gate
// for one reason.
type Finding struct {
	Key      string    `json:"key"`      // a rule id of RULES.md, or proposed, stale or sole_review
	Rule     string    `json:"rule"`     // the rule id; "" for the three kinds no rule states
	Severity string    `json:"severity"` // error, warning or information, as the audit lists it
	Demoted  bool      `json:"demoted"`  // the rule's own severity is graver: the junction is historical, or the commit is old
	Kind     string    `json:"kind"`     // the rule's title, or the kind's: tablo's words
	Tasks    []TaskRef `json:"tasks"`    // in display order; empty for a finding that names none
	Gate     string    `json:"gate"`     // "" when the finding names none
	Act      string    `json:"act"`      // one of the ten acts
	Resolver string    `json:"resolver"` // an email; "" when the files name nobody
	*FindingDetail
	*FindingProvenance
}

// FindingDetail is what a finding adds at detail.
type FindingDetail struct {
	Files   []string `json:"files"`   // relative to the project directory: .tableaux/tasks/9f31.yaml
	Message string   `json:"message"` // a rule finding: the diagnostic's, byte for byte; tablo's words
}

// FindingProvenance is what a finding adds at provenance.
type FindingProvenance struct {
	Sentence string         `json:"sentence"` // the sentence of the method the kind rests on
	Source   Reference      `json:"source"`   // where the sentence stands; the url is absolute
	Facts    []FindingFacts `json:"facts"`    // one per task of the row that has a typed fact, in the order of Tasks
	Commits  []Commit       `json:"commits"`  // the commits involved, each once, the oldest first
	Commands []Command      `json:"commands"` // what shows the finding, then what resolves it
}

// FindingFacts is what the Derivation knows of one task of a finding, each
// fact in the type every other view gives it. A fact the finding does not
// carry is null.
type FindingFacts struct {
	Task          string         `json:"task"`          // the task's id
	Junction      *Junction      `json:"junction"`      // the junction at the finding's gate, with From
	Status        *Status        `json:"status"`        // the task's status
	Requirement   *Requirement   `json:"requirement"`   // the requires entry the finding stands at
	Authorisation *Authorisation `json:"authorisation"` // a proposed task: who decides, and why nothing does
	Linkage       *Linkage       `json:"linkage"`       // the subproject the junction reads, with its pin
	Model         *ModelCheck    `json:"model"`         // the commit's Model: trailer against the model stated
}

// Resolving is the number of findings that ask an act of one person.
type Resolving struct {
	Email string `json:"email"`
	Count int    `json:"count"`
}

// Silence is a kind the audit examines and finds nothing of.
type Silence struct {
	Key  string `json:"key"`
	Rule string `json:"rule"`
	Kind string `json:"kind"`
}
