package views

import (
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// builder holds the facts and the query of one call and builds the shared
// types. Its methods are the rows of the table of shared types of design
// 493e, one each.
type builder struct {
	f *derive.Facts
	q Query
}

// planDir is the directory that holds a project's files, as a path of the
// data names it.
const planDir = ".tableaux"

// The words a view writes for where a junction stands.
const (
	standsPassed = "passed"
	standsHere   = "here"
	standsNext   = "next"
	standsLater  = "later"
	standsExempt = "exempt"
)

// marks holds, per mark of the method, the phrase of VIEWS.md's gate
// definition and the kind of junction that shows the mark.
var marks = []struct {
	mark     derive.Mark
	meaning  string
	junction model.JunctionKind
}{
	{derive.MarkPerson, "A person contributes", model.Plain},
	{derive.MarkAgent, "An agent contributes", model.Plain},
	{derive.MarkReviewer, "A reviewer accepts", model.Plain},
	{derive.MarkSubproject, "A subproject does the work", model.Recursive},
	{derive.MarkExempt, "The gate does not apply", model.NotApplicable},
}

// why returns the word of a reason a fact is undetermined: "" when it is
// determined, else the reason's name with an underscore for each space.
func why(w derive.Why) string {
	if w == derive.Determined {
		return ""
	}
	return strings.ReplaceAll(w.String(), " ", "_")
}

// statusKind returns the word of a status's kind.
func statusKind(k derive.StatusKind) string {
	switch k {
	case derive.Absent:
		return "absent"
	case derive.RolledUp:
		return "rollup"
	case derive.Snapshotted:
		return "snapshot"
	}
	return "recorded"
}

// refIn returns the reference of a task of the project f holds; the title is
// "" when the project lacks the task.
func refIn(f *derive.Facts, id string) view.TaskRef {
	ref := view.TaskRef{ID: id}
	if p := f.Project(); p != nil {
		if task := p.Tasks[id]; task != nil {
			ref.Title = task.Title.V
		}
	}
	return ref
}

// taskFile returns the path of a task's file, relative to the project
// directory.
func taskFile(id string) string {
	return path.Join(planDir, "tasks", id+".yaml")
}

// ref returns the reference of a task of the project in view.
func (b builder) ref(id string) view.TaskRef { return refIn(b.f, id) }

// legend returns gates.yaml and the method's marks.
func (b builder) legend() view.Legend {
	l := view.Legend{Gates: []view.Gate{}, States: []view.State{}, Reasons: []view.Reason{}, Marks: []view.Mark{}}
	gating := b.f.Project().Gating
	for _, key := range b.f.Gates() {
		for _, g := range gating.Gates {
			if g.Key.V == key {
				l.Gates = append(l.Gates, view.Gate{Key: key, Symbol: g.Symbol.V, Name: g.Name.V, Criteria: g.Criteria.V})
				break
			}
		}
	}
	for _, s := range gating.States {
		l.States = append(l.States, view.State{Key: s.Key.V, Symbol: s.Symbol.V, Severity: b.f.Severity(s.Key.V), Synopsis: s.Synopsis.V})
	}
	for _, r := range gating.Reasons {
		l.Reasons = append(l.Reasons, view.Reason{Key: r.Key.V, Symbol: r.Symbol.V, Synopsis: r.Synopsis.V, Reserved: r.Key.V == "review"})
	}
	for _, m := range marks {
		l.Marks = append(l.Marks, view.Mark{Key: string(m.mark), Symbol: m.mark.Symbol(), Meaning: m.meaning, Junction: m.junction.String()})
	}
	return l
}

// refs returns a list of references; it is never nil.
func (b builder) refs(list []*model.Reference) []view.Reference {
	out := make([]view.Reference, 0, len(list))
	for _, r := range list {
		out = append(out, view.Reference{Text: r.Text.V, URL: r.URL.V})
	}
	return out
}

// status returns a status as a view shows it. f holds the status's task: a
// roll-up's child and a snapshot's title read the project the task is of.
func (b builder) status(f *derive.Facts, s *derive.Status) view.Status {
	if s == nil {
		return view.Status{}
	}
	out := view.Status{
		Kind: statusKind(s.Kind), Gate: s.Gate, State: s.State, Reason: s.Reason, Note: s.Note,
		Date: s.Date, Recorder: s.Recorder, Derived: s.Derived(), Why: why(s.Why), Uncommitted: s.Uncommitted,
	}
	switch {
	case s.Kind == derive.RolledUp && s.From != "":
		out.From = &view.Origin{Kind: "rollup", Task: refIn(f, s.From)}
	case s.Kind == derive.Snapshotted && s.Snapshot != nil:
		origin := &view.Origin{Kind: "snapshot", Task: refIn(s.Snapshot.Facts, s.Snapshot.Target)}
		if link := s.Snapshot.Link; link != nil {
			origin.URL, origin.Pin = link.URL, link.Commit
		}
		if s.Of != nil {
			origin.Gate = s.Of.Gate
		}
		out.From = origin
	}
	return out
}

// chain returns the statuses a task's status comes through, its own first.
// A step after a snapshot reads the subproject, and names its url.
func (b builder) chain(id string) []view.Step {
	steps := []view.Step{}
	at, url := b.f, ""
	for _, s := range b.f.Chain(id) {
		steps = append(steps, view.Step{Task: refIn(at, s.Task), URL: url, Status: b.status(at, s)})
		if s.Kind == derive.Snapshotted && s.Snapshot != nil {
			at = s.Snapshot.Facts
			if s.Snapshot.Link != nil {
				url = s.Snapshot.Link.URL
			}
		}
	}
	return steps
}

// commit returns a commit fact, or nil for nil.
func (b builder) commit(c *history.Commit) *view.Commit {
	if c == nil {
		return nil
	}
	out := &view.Commit{
		Hash: c.ID, Subject: c.Subject, Date: c.Date(),
		AuthorTime: time.Unix(c.Time, 0).In(time.FixedZone("", c.Zone*60)).Format(time.RFC3339),
		Author:     view.Person{Name: c.Author.Name, Email: c.Author.Email},
		Committer:  view.Person{Name: c.Committer.Name, Email: c.Committer.Email},
		Parents:    append([]string{}, c.Parents...),
		Trailers:   make([]string, 0, len(c.Trailers)),
		Files:      make([]view.File, 0, len(c.Changes)),
	}
	for _, t := range c.Trailers {
		out.Trailers = append(out.Trailers, t.Key+": "+t.Value)
	}
	for _, change := range c.Changes {
		word := "changed"
		switch {
		case change.Gitlink:
			word = "pin"
		case change.Old == "":
			word = "added"
		case change.New == "":
			word = "removed"
		}
		out.Files = append(out.Files, view.File{Path: change.Path, Change: word})
	}
	return out
}

// junction returns a resolved junction of a task of the project in view.
// from says whether the view shows suppliers, and the junction then carries
// the tasks whose entries supply it.
func (b builder) junction(j *derive.Junction, from bool) view.Junction {
	out := view.Junction{
		Gate: j.Gate, Kind: j.Kind.String(), Marks: []string{},
		Contributor: j.Contributor.V, Model: j.Model.V, Reviewer: j.Reviewer.V,
		ReviewerByDefault: j.Reviewer.By == derive.ByAssignee,
		References:        b.refs(j.References),
		Stands:            b.stands(j),
	}
	for _, m := range j.Marks() {
		out.Marks = append(out.Marks, string(m))
	}
	if person := b.q.Person; person != "" && j.Kind == model.Plain {
		out.Acts = j.Contributor.V == person || j.Reviewer.V == person
	}
	if j.Kind == model.Recursive {
		out.Subproject = b.subRef(j.Snapshot)
	}
	if review := b.f.Accepted(j.Task, j.Gate); review != nil {
		out.Reviewed = review.Commit != nil
	}
	if from {
		out.From = append([]string{}, j.Sources()...)
	}
	return out
}

// stands returns where a junction stands against its task's status: "" on a
// parent, whose junctions are defaults.
func (b builder) stands(j *derive.Junction) string {
	if !b.f.Leaf(j.Task) {
		return ""
	}
	at := ""
	if s := b.f.Status(j.Task); s != nil {
		at = s.Gate
	}
	switch {
	case j.Kind == model.NotApplicable:
		return standsExempt
	case j.Gate == at:
		return standsHere
	case b.f.Index(j.Gate) < b.f.Index(at):
		return standsPassed
	case j.Gate == b.f.Next(j.Task, at):
		return standsNext
	}
	return standsLater
}

// subRef names the task a snapshot reads, or returns nil for nil.
func (b builder) subRef(n *derive.Snapshot) *view.SubRef {
	if n == nil {
		return nil
	}
	out := &view.SubRef{Form: model.NoForm.String(), Why: why(n.Why)}
	if n.Entry != nil {
		out.URL, out.ID = n.Entry.URL.V, n.Entry.ID.V
	}
	if n.Link != nil {
		out.URL, out.Form, out.Commit = n.Link.URL, n.Link.Form.String(), n.Link.Commit
	}
	if n.Why == derive.Determined && n.Facts != nil {
		out.Task = refIn(n.Facts, n.Target)
	}
	return out
}

// statedField says where a field of a resolved junction stands.
func statedField(gate, field string, f derive.Field) view.Stated {
	switch f.By {
	case derive.ByTask:
		out := view.Stated{By: f.Task, Key: "junctions." + gate + "." + field, File: taskFile(f.Task)}
		if f.Node != nil {
			out.Line, out.Col = f.Node.Pos.Line, f.Node.Pos.Col
		}
		return out
	case derive.ByAssignee:
		return view.Stated{By: "assignee", Key: "assignee"}
	}
	return view.Stated{By: "default"}
}

// statedEntry says where the entry of a task's file at a gate states one of
// its keys: at the entry's own place in the file.
func (b builder) statedEntry(task, gate, field string) view.Stated {
	out := view.Stated{By: task, Key: "junctions." + gate + "." + field, File: taskFile(task)}
	if t := b.f.Project().Tasks[task]; t != nil {
		for _, e := range t.Junctions {
			if e.Gate == gate {
				out.Line, out.Col = e.KeyPos.Line, e.KeyPos.Col
				break
			}
		}
	}
	return out
}

// suppliers returns what supplies each field of a resolved junction, in the
// order contributor, model, reviewer, references, subproject, applies. It
// is never nil.
func (b builder) suppliers(j *derive.Junction) []view.Supplier {
	out := []view.Supplier{}
	add := func(field, value string, stated view.Stated) {
		out = append(out, view.Supplier{Gate: j.Gate, Field: field, Value: value, Stated: stated})
	}
	switch j.Kind {
	case model.Plain:
		if j.Contributor.By == derive.ByTask {
			add("contributor", j.Contributor.V, statedField(j.Gate, "contributor", j.Contributor))
		}
		if j.Model.By == derive.ByTask {
			add("model", j.Model.V, statedField(j.Gate, "model", j.Model))
		}
		if j.Reviewer.By != derive.ByDefault {
			add("reviewer", j.Reviewer.V, statedField(j.Gate, "reviewer", j.Reviewer))
		}
		if j.ReferencesFrom != "" {
			add("references", strconv.Itoa(len(j.References)), b.statedEntry(j.ReferencesFrom, j.Gate, "references"))
		}
	case model.Recursive:
		url := ""
		if ref := b.subRef(j.Snapshot); ref != nil {
			url = ref.URL
		}
		add("subproject", url, b.statedEntry(j.Entry, j.Gate, "subproject"))
	case model.NotApplicable:
		add("applies", "false", b.statedEntry(j.Entry, j.Gate, "applies"))
	}
	return out
}

// requirement returns a requires entry with its condition. A requirement
// names its originating task, with that task's status from the project that
// holds it; a dependent names the terminating task, of the project in view.
func (b builder) requirement(c *derive.Condition, dependent bool) view.Requirement {
	out := view.Requirement{
		From: c.From, To: c.To, Met: c.Met, Due: c.Due,
		Condition: c.Word(), Why: why(c.Why), AtTrunk: c.AtTrunk.String(),
	}
	if c.Entry != nil {
		out.Text = c.Entry.Text.V
	}
	if c.Link != nil {
		out.Subproject, out.Commit = c.Link.URL, c.Link.Commit
	}
	if dependent {
		out.Task = b.ref(c.Task)
		return out
	}
	out.Task = refIn(c.Facts, c.Origin)
	if s := c.Facts.Status(c.Origin); s != nil {
		status := b.status(c.Facts, s)
		out.Status = &status
	}
	return out
}

// subtree returns a task and its descendants, in display order.
func (b builder) subtree(top string) []string {
	list := []string{top}
	for _, child := range b.f.Children(top) {
		list = append(list, b.subtree(child)...)
	}
	return list
}

// parent returns the id of a task's parent, or "" for the root.
func (b builder) parent(id string) string {
	if authorities := b.f.Authorities(id); len(authorities) > 0 {
		return authorities[0].Task
	}
	return ""
}
