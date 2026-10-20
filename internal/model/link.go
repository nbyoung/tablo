package model

import "regexp"

// absoluteURL matches a url with a scheme and ://, as the Loader reads one;
// any other url is a path relative to the repository root.
var absoluteURL = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)

// Form is which of the method's three url forms a link takes.
type Form int

// The forms of a link.
const (
	NoForm    Form = iota // the path leads nowhere
	Submodule             // a path that is a gitlink
	Directory             // a path that is a directory of the same repository
	URL                   // an absolute URL
)

// String returns the form's name in lower case.
func (f Form) String() string {
	switch f {
	case Submodule:
		return "submodule"
	case Directory:
		return "directory"
	case URL:
		return "url"
	}
	return "none"
}

// Problem is why a link has no project.
type Problem int

// The problems of a link. Resolved is none.
const (
	Resolved     Problem = iota
	Outside              // the path is empty, absolute or leaves the repository
	Missing              // nothing is at the path
	NoClone              // a submodule with no clone here; a URL with no mapping and no cache
	BadCommit            // a URL with no commit field, or one that is no full hash
	CommitAbsent         // the clone does not hold the commit
	NoProject            // no .tableaux is there
	TooDeep              // more than eight links from the project first asked for
	NoTrunk              // an unpinned URL whose clone holds no trunk to read
)

// String returns the problem's name in lower case.
func (p Problem) String() string {
	switch p {
	case Resolved:
		return "resolved"
	case Outside:
		return "outside"
	case Missing:
		return "missing"
	case NoClone:
		return "no clone"
	case BadCommit:
		return "bad commit"
	case CommitAbsent:
		return "commit absent"
	case NoProject:
		return "no project"
	case NoTrunk:
		return "no trunk"
	}
	return "too deep"
}

// Link is one subproject as the Loader found it.
type Link struct {
	URL      string // an absolute URL as written, or the path cleaned
	Form     Form
	Commit   string   // the commit the project is read at; "" for a Directory
	Project  *Project // nil unless Problem is Resolved
	Problem  Problem
	Detail   string
	Checkout string // a Submodule whose pin the Loader read on disk: the checkout's directory, absolute; else ""
	Unpinned bool   // a requirement's absolute URL with no commit: Commit is the tip of Trunk
	Trunk    string // Unpinned: the trunk's name, or "HEAD" where version.yaml names none; else ""
}
