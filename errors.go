package tablo

import "fmt"

// A UsageError reports parameters a command or a view does not accept: the
// command exits 2. Msg names the option or the value at fault.
type UsageError struct{ Msg string }

// Error returns the message.
func (e *UsageError) Error() string { return e.Msg }

// A NotFoundError reports a well-formed ref, task or gate that the repository
// or the project lacks: the command exits 3. Kind is "ref", "task" or "gate".
type NotFoundError struct{ Kind, Name string }

// Error names what is missing.
func (e *NotFoundError) Error() string {
	switch e.Kind {
	case "ref":
		return fmt.Sprintf("the repository has no ref %q", e.Name)
	case "gate":
		return fmt.Sprintf("the project has no gate %q for this use", e.Name)
	}
	return fmt.Sprintf("the project has no %s %q", e.Kind, e.Name)
}

// A ReadError reports a repository that cannot be read: the command exits 3.
// Op names what failed and Err says why.
type ReadError struct {
	Op  string
	Err error
}

// Error joins the operation and its cause.
func (e *ReadError) Error() string {
	if e.Err == nil {
		return e.Op
	}
	return e.Op + ": " + e.Err.Error()
}

// Unwrap returns the cause.
func (e *ReadError) Unwrap() error { return e.Err }
