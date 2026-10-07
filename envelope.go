package tablo

import (
	"bytes"
	_ "embed"
	"encoding/json"
)

// SchemaVersion names the output schema: the envelope and every command's data.
const SchemaVersion = "tablo/1"

// EnvelopeSchema is envelope.schema.yaml, the JSON Schema of the envelope.
//
//go:embed envelope.schema.yaml
var EnvelopeSchema []byte

// An Envelope is the result of one command: what the library returns and
// what the command encodes. Field order is the key order of the output.
type Envelope struct {
	Schema      string       `json:"schema"`
	Command     string       `json:"command"`
	Source      *Source      `json:"source"`
	Project     *ProjectInfo `json:"project"`
	Viewer      *Viewer      `json:"viewer"`
	Params      Resolved     `json:"params"`
	Data        any          `json:"data"` // a view's typed data (A5 to A8), or nil
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// A Source states what a command read. Commit and Date are nil when the
// source is a bare tree.
type Source struct {
	Ref      string      `json:"ref"`      // the name given; for the working tree, the branch HEAD names, or HEAD
	Worktree bool        `json:"worktree"` // the files come from disk and the history from HEAD
	Commit   *string     `json:"commit"`   // the commit in full
	Date     *string     `json:"date"`     // the commit's author date, YYYY-MM-DD
	From     *SourceFrom `json:"from"`     // the start of a range, for history alone
	Trunk    SourceTrunk `json:"trunk"`
	OnTrunk  bool        `json:"on_trunk"`
}

// A SourceFrom is the start of a range: the ref as given and its commit.
type SourceFrom struct {
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

// A SourceTrunk names the trunk and how it resolves: "stated", "inferred",
// "caller" or "undetermined". Name is nil when it is undetermined.
type SourceTrunk struct {
	Name     *string `json:"name"`
	Resolved string  `json:"resolved"`
}

// ProjectInfo names the project: the language version version.yaml states, the
// root task, its title and its assignee.
type ProjectInfo struct {
	Tableaux string `json:"tableaux"`
	Root     string `json:"root"`
	Title    string `json:"title"`
	Owner    string `json:"owner"`
}

// A Viewer is whoever runs the tool, with the roles the email holds anywhere
// in the project. Email is nil for nobody, an observer.
type Viewer struct {
	Email *string `json:"email"`
	Roles []Role  `json:"roles"`
}

// A Diagnostic is one finding of the loader or the validator. Path is relative
// to the directory -C names, starts with .tableaux/ and uses / on every host;
// Line and Col are 1-based. A field that does not apply is empty or zero and
// the encodings omit it.
type Diagnostic struct {
	Severity string `json:"severity"` // "error", "warning" or "information"
	Code     string `json:"code"`     // the rule id of corpus/RULES.md
	Path     string `json:"path,omitempty"`
	Line     int    `json:"line,omitempty"`
	Col      int    `json:"col,omitempty"`
	Task     string `json:"task,omitempty"`
	Gate     string `json:"gate,omitempty"`
	Commit   string `json:"commit,omitempty"`
	Trailer  string `json:"trailer,omitempty"`
	Message  string `json:"message"`
}

// Resolved holds a command's parameters with every default written in. It
// marshals the keys its command takes, each present and null when unset, in
// the order task, person, window, columns, historical, proposed, brief,
// stale, now, role, level; for a command that is no view it marshals {}.
type Resolved struct {
	View View
	Params
}

// MarshalJSON writes the keys View takes, each present, in the fixed order.
func (r Resolved) MarshalJSON() ([]byte, error) {
	spec, ok := specOf(r.View)
	if !ok {
		return []byte("{}"), nil
	}
	var b bytes.Buffer
	b.WriteByte('{')
	var err error
	put := func(key string, v any) {
		if err != nil {
			return
		}
		var val []byte
		if val, err = marshalNoEscape(v); err != nil {
			return
		}
		if b.Len() > 1 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		b.WriteString(key)
		b.WriteString(`":`)
		b.Write(bytes.TrimRight(val, "\n"))
	}
	str := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	takes := func(f field) bool { return spec.takes&f != 0 }
	if takes(fTask) {
		put("task", str(r.Task))
	}
	if takes(fPerson) {
		put("person", str(r.Person))
	}
	if takes(fWindow) {
		put("window", r.Window)
	}
	if takes(fColumns) {
		var cols any
		if len(r.Columns) > 0 {
			cols = r.Columns
		}
		put("columns", cols)
	}
	if takes(fHistorical) {
		put("historical", r.Historical)
	}
	if takes(fProposed) {
		put("proposed", r.Proposed)
	}
	if takes(fBrief) {
		var brief any
		if r.Brief != nil {
			brief = struct {
				Task string `json:"task"`
				Gate string `json:"gate"`
			}{r.Brief.Task, r.Brief.Gate}
		}
		put("brief", brief)
	}
	if takes(fStale) {
		var stale any
		if r.Stale != 0 {
			stale = r.Stale
		}
		put("stale", stale)
	}
	if takes(fNow) {
		put("now", str(r.Now))
	}
	put("role", str(string(r.Role)))
	put("level", str(string(r.Level)))
	b.WriteByte('}')
	return b.Bytes(), err
}

// marshalNoEscape marshals v without HTML escaping.
func marshalNoEscape(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// JSON returns the JSON encoding: a two-space indent, no HTML escaping, one
// final newline, and the keys in the declaration order of the Go types. A nil
// list of diagnostics or roles encodes as []. It panics when Data does not
// marshal, which is a fault of the view that built it.
func (e *Envelope) JSON() []byte {
	c := *e
	if c.Diagnostics == nil {
		c.Diagnostics = []Diagnostic{}
	}
	if c.Viewer != nil && c.Viewer.Roles == nil {
		v := *c.Viewer
		v.Roles = []Role{}
		c.Viewer = &v
	}
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&c); err != nil {
		panic("tablo: encoding an envelope: " + err.Error())
	}
	return b.Bytes()
}

// YAML returns the YAML respelling of JSON(): the same document, every string
// double-quoted, no library involved in writing it.
func (e *Envelope) YAML() []byte { return yamlOfJSON(e.JSON()) }

// ExitCode returns 1 when a diagnostic is an error, else 0.
func (e *Envelope) ExitCode() int {
	for _, d := range e.Diagnostics {
		if d.Severity == "error" {
			return 1
		}
	}
	return 0
}
