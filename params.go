package tablo

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// A View names one of the ten views by its command name.
type View string

// The ten views, each named as its mockup file is named.
const (
	Gates      View = "gates"
	Task       View = "task"
	Authority  View = "authority"
	Assignment View = "assignment"
	Queue      View = "queue"
	Blockage   View = "blockage"
	Tableau    View = "tableau"
	Context    View = "context"
	History    View = "history"
	Audit      View = "audit"
)

// Level is "glance", "detail" or "provenance".
type Level string

// The three levels, from the least to the most a view shows.
const (
	LevelGlance     Level = "glance"
	LevelDetail     Level = "detail"
	LevelProvenance Level = "provenance"
)

// Role is "owner", "authority", "assignee", "contributor", "agent",
// "reviewer" or "observer".
type Role string

// The seven roles, in the order README.md lists them; observer is the role of
// an email that holds none.
const (
	RoleOwner       Role = "owner"
	RoleAuthority   Role = "authority"
	RoleAssignee    Role = "assignee"
	RoleContributor Role = "contributor"
	RoleAgent       Role = "agent"
	RoleReviewer    Role = "reviewer"
	RoleObserver    Role = "observer"
)

// Params focus a view. The zero value asks for every default. Check returns
// a *UsageError when p sets a field v does not take or holds a malformed
// value, by the rules of the command table. Narrow returns p without the
// fields v does not take, for a front end that carries one state across views.
type Params struct {
	Task       string   // a task id
	Person     string   // an email
	Viewer     string   // whoever runs the tool; "" is an observer
	Role       Role     // the role whose level the view opens at
	Level      Level    // the level, overriding the role's
	Window     *int     // tableau, context: columns either side of the next gates; nil is 1
	Columns    []string // tableau, context: the gate keys to show, in place of a window
	Historical bool     // tableau, context: show the marks of historical junctions
	Proposed   bool     // authority: proposed tasks only
	Brief      *Brief   // queue: write this one item as a brief
	Stale      int      // audit: days after which a status is stale; 0 is 7
	Now        string   // audit: the date staleness counts from, YYYY-MM-DD; required
	From       string   // history: the ref the range starts after; "" is the whole history
}

// A Brief names the one queue item the queue view writes as a brief.
type Brief struct{ Task, Gate string }

// field is one bit per parameter that only some views take.
type field uint16

const (
	fTask field = 1 << iota
	fPerson
	fWindow
	fColumns
	fHistorical
	fProposed
	fBrief
	fStale
	fNow
	fFrom
)

// fields lists the optional parameters in the order of Resolved's keys, with
// the option that sets each, for the messages.
var fields = []struct {
	bit field
	opt string
}{
	{fTask, "--task"},
	{fPerson, "--person"},
	{fWindow, "--window"},
	{fColumns, "--columns"},
	{fHistorical, "--historical"},
	{fProposed, "--proposed"},
	{fBrief, "--brief"},
	{fStale, "--stale"},
	{fNow, "--now"},
	{fFrom, "a range in --ref"},
}

// A viewSpec is one row of the command table: the optional parameters the view
// takes. Every view also takes Viewer, Role and Level.
type viewSpec struct {
	view      View
	takes     field
	needsTask bool // the task id is an argument and no default stands in
}

// viewTable is the command table of design 4ed9, one row per view. Check,
// Narrow and Resolved read it.
var viewTable = []viewSpec{
	{Gates, fTask, false},
	{Task, fTask | fPerson, true},
	{Authority, fTask | fPerson | fProposed, false},
	{Assignment, fTask | fPerson, false},
	{Queue, fTask | fPerson | fBrief, false},
	{Blockage, fTask | fPerson, false},
	{Tableau, fPerson | fWindow | fColumns | fHistorical, false},
	{Context, fTask | fPerson | fWindow | fColumns | fHistorical, false},
	{History, fTask | fPerson | fFrom, false},
	{Audit, fTask | fPerson | fStale | fNow, false},
}

// specOf returns the row of v.
func specOf(v View) (viewSpec, bool) {
	for _, s := range viewTable {
		if s.view == v {
			return s, true
		}
	}
	return viewSpec{}, false
}

// set returns the optional parameters p sets: a non-zero value, a non-nil
// pointer or a non-empty list.
func (p Params) set() field {
	var s field
	flag := func(on bool, f field) {
		if on {
			s |= f
		}
	}
	flag(p.Task != "", fTask)
	flag(p.Person != "", fPerson)
	flag(p.Window != nil, fWindow)
	flag(len(p.Columns) > 0, fColumns)
	flag(p.Historical, fHistorical)
	flag(p.Proposed, fProposed)
	flag(p.Brief != nil, fBrief)
	flag(p.Stale != 0, fStale)
	flag(p.Now != "", fNow)
	flag(p.From != "", fFrom)
	return s
}

var (
	idPattern    = regexp.MustCompile(`^[0-9a-f]{4}$`)
	keyPattern   = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+$`)
)

func usagef(format string, a ...any) error {
	return &UsageError{Msg: fmt.Sprintf(format, a...)}
}

// Check returns a *UsageError when p sets a field v does not take or holds a
// malformed value, or sets fields that exclude each other: --window with
// --columns, --task with --person on context, and --brief with --level or
// --role. It reads no environment, so it does not require a person or a
// viewer; the resolution of defaults does.
func (p Params) Check(v View) error {
	spec, ok := specOf(v)
	if !ok {
		return usagef("%q is not a view", string(v))
	}
	set := p.set()
	for _, f := range fields {
		if set&f.bit != 0 && spec.takes&f.bit == 0 {
			return usagef("%s is not taken by %s", f.opt, v)
		}
	}
	taskOpt := "--task"
	if spec.needsTask {
		taskOpt = "the task id"
		if p.Task == "" {
			return usagef("%s requires %s", v, taskOpt)
		}
	}
	if p.Task != "" && !idPattern.MatchString(p.Task) {
		return usagef("%s %q is not four lowercase hexadecimal digits", taskOpt, p.Task)
	}
	if p.Person != "" && !emailPattern.MatchString(p.Person) {
		return usagef("--person %q is not an email address", p.Person)
	}
	if p.Viewer != "" && !emailPattern.MatchString(p.Viewer) {
		return usagef("--viewer %q is not an email address", p.Viewer)
	}
	if p.Window != nil && *p.Window < 0 {
		return usagef("--window %d is negative", *p.Window)
	}
	seen := map[string]bool{}
	for _, c := range p.Columns {
		if !keyPattern.MatchString(c) {
			return usagef("--columns %q is not a gate key", c)
		}
		if seen[c] {
			return usagef("--columns names %q twice", c)
		}
		seen[c] = true
	}
	if b := p.Brief; b != nil {
		if !idPattern.MatchString(b.Task) {
			return usagef("--brief %q is not four lowercase hexadecimal digits", b.Task)
		}
		if !keyPattern.MatchString(b.Gate) {
			return usagef("--brief %q is not a gate key", b.Gate)
		}
	}
	if p.Stale < 0 {
		return usagef("--stale %d is below one", p.Stale)
	}
	if p.Now != "" {
		if _, err := time.Parse("2006-01-02", p.Now); err != nil {
			return usagef("--now %q is not a date of the form YYYY-MM-DD", p.Now)
		}
	}
	if strings.HasPrefix(p.From, "-") {
		return usagef("the range start %q starts with a hyphen", p.From)
	}
	switch p.Role {
	case "", RoleOwner, RoleAuthority, RoleAssignee, RoleContributor, RoleAgent, RoleReviewer, RoleObserver:
	default:
		return usagef("--role %q is not owner, authority, assignee, contributor, agent, reviewer or observer", string(p.Role))
	}
	switch p.Level {
	case "", LevelGlance, LevelDetail, LevelProvenance:
	default:
		return usagef("--level %q is not glance, detail or provenance", string(p.Level))
	}
	if p.Window != nil && len(p.Columns) > 0 {
		return usagef("--window and --columns exclude each other")
	}
	if v == Context && p.Task != "" && p.Person != "" {
		return usagef("context takes --task or --person, not both")
	}
	if p.Brief != nil && p.Level != "" {
		return usagef("--brief and --level exclude each other")
	}
	if p.Brief != nil && p.Role != "" {
		return usagef("--brief and --role exclude each other")
	}
	return nil
}

// Narrow returns p without the fields v does not take. It keeps Viewer, Role
// and Level, which every view takes, and it changes nothing else: a malformed
// value or a conflict between fields v takes stays for Check to report. For a
// name that is no view it returns the zero value.
func (p Params) Narrow(v View) Params {
	spec, ok := specOf(v)
	if !ok {
		return Params{}
	}
	q := p
	drop := func(f field, clear func()) {
		if spec.takes&f == 0 {
			clear()
		}
	}
	drop(fTask, func() { q.Task = "" })
	drop(fPerson, func() { q.Person = "" })
	drop(fWindow, func() { q.Window = nil })
	drop(fColumns, func() { q.Columns = nil })
	drop(fHistorical, func() { q.Historical = false })
	drop(fProposed, func() { q.Proposed = false })
	drop(fBrief, func() { q.Brief = nil })
	drop(fStale, func() { q.Stale = 0 })
	drop(fNow, func() { q.Now = "" })
	drop(fFrom, func() { q.From = "" })
	return q
}
