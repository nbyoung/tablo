package model

// Severity is a diagnostic's weight, as RULES.md defines the three.
type Severity int

// The severities: an error makes the project invalid, a warning leaves it
// valid, and information marks nothing wrong.
const (
	Error Severity = iota
	Warning
	Information
)

// String returns the severity's name as RULES.md writes it.
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	}
	return "information"
}

// Diagnostic is one finding of the Loader or the Validator.
type Diagnostic struct {
	Code     string // a rule id from RULES.md
	Severity Severity
	Pos      Pos
	Task     string // the task it concerns, when it concerns one
	Gate     string // the gate it concerns, when it concerns one
	Message  string
}
