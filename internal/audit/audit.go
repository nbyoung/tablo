// Package audit turns the Validator's diagnostics and three facts of the
// Derivation into findings: each names its task, its gate, its commits, the
// act that resolves it and the person who takes it, and holds the facts of
// the Derivation it rests on. It is a pure function of its input: it runs no
// process and reads no file, clock or environment.
package audit

// Act is the kind of action that resolves a finding.
type Act string

// The acts: the eight of VIEWS.md#audit, then the two this design adds.
const (
	Authorise     Act = "authorise"      // an authority accepts the task
	Review        Act = "review"         // the reviewer accepts the junction
	Reaffirm      Act = "reaffirm"       // the recorder confirms the status
	RecordHandoff Act = "record_handoff" // the contributor states the reason review
	ClearHandoff  Act = "clear_handoff"  // the contributor records the status at the gate
	Revise        Act = "revise"         // someone revises the file
	MovePin       Act = "move_pin"       // someone moves the pin or the commit field
	Checkout      Act = "checkout"       // whoever holds the clone gives the tool the subproject
	Await         Act = "await"          // the originating task passes the gate a requirement names
	None          Act = "none"           // nothing resolves it
)

// The keys of the three kinds of finding that no rule states.
const (
	Proposed   = "proposed"
	Stale      = "stale"
	SoleReview = "sole_review"
)

// DefaultStale is the age in days beyond which a status is stale.
const DefaultStale = 7

// Source says where a sentence stands: "README.md, Status" and its address.
type Source struct{ Text, URL string }
