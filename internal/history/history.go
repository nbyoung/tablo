// Package history is the one place that reads a project's history. A pass is
// one git log over the commits a view's source and its trunk reach, bounded
// to the project's files and the pins of its submodules, with every blob the
// pass names under .tableaux. A Log answers from it what a path holds at any
// commit and what the project states there, and a Reader makes one pass per
// project of a load and remembers the last of them. The derivation in
// internal/derive is a function of the model and these passes.
package history

import (
	"strings"
	"time"
)

// Person is an identity as a commit records it, with no mailmap applied.
type Person struct{ Name, Email string }

// Trailer is one line of a commit's trailer block.
type Trailer struct{ Key, Value string }

// Change is one watched path a commit changes against its first parent.
type Change struct {
	Path     string // relative to the repository root
	Old, New string // the object ids, in full; "" where the path is absent
	Gitlink  bool   // either side is a submodule's pin
}

// Commit is one commit of a pass. It is immutable.
type Commit struct {
	ID        string   // in full
	Parents   []string // in full, the first parent first
	Author    Person
	Committer Person
	Time      int64 // the author time, in seconds since the epoch
	Zone      int   // the author's offset from UTC, in minutes east
	Subject   string
	Trailers  []Trailer // the trailer block as Git reads it, in order
	Changes   []Change  // by path
	Seq       int       // its place in the pass; 0 is the newest
	InSource  bool      // the source reaches it
	Trunk     int       // its place on the trunk's first-parent line, 0 at the tip; -1 off the line
}

// Date returns the author date as YYYY-MM-DD in the author's own zone.
func (c *Commit) Date() string {
	return time.Unix(c.Time, 0).In(time.FixedZone("", c.Zone*60)).Format("2006-01-02")
}

// Merge reports whether the commit has more than one parent.
func (c *Commit) Merge() bool { return len(c.Parents) > 1 }

// Values returns the values of the trailers whose key is key, in order.
func (c *Commit) Values(key string) []string {
	var values []string
	for _, t := range c.Trailers {
		if t.Key == key {
			values = append(values, t.Value)
		}
	}
	return values
}

// From says what names the trunk.
type From int

// The sources of a trunk's name, in the order README.md#project reads them.
const (
	Unnamed  From = iota // nothing names a branch
	Stated               // version.yaml
	Inferred             // refs/remotes/origin/HEAD
	Caller               // the branch the caller names
)

// Trunk is the trunk of one project in one repository.
type Trunk struct {
	Name string // the branch's name; "" when From is Unnamed
	From From
	Ref  string // the ref that carries the name, in full; "" when none does
	Tip  string // the commit that ref names; "" when none does
}

// How returns "stated", "inferred" or "caller", or "undetermined" when no
// ref carries a name.
func (t Trunk) How() string {
	if t.Tip == "" {
		return "undetermined"
	}
	switch t.From {
	case Stated:
		return "stated"
	case Inferred:
		return "inferred"
	case Caller:
		return "caller"
	}
	return "undetermined"
}

// trailer splits one line of a trailer block at its first colon.
func trailer(line string) Trailer {
	key, value, _ := strings.Cut(line, ":")
	return Trailer{Key: strings.TrimSpace(key), Value: strings.TrimSpace(value)}
}
