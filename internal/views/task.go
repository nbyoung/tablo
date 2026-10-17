package views

import (
	"path"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// newestEvents is the number of events the task definition holds.
const newestEvents = 5

// Task returns the task definition of q.Task, a task of the project.
func Task(f *derive.Facts, q Query) *view.Task {
	b := builder{f, q}
	id := q.Task
	out := &view.Task{Legend: b.legend(), Task: b.ref(id), Status: b.status(f, f.Status(id))}
	task := f.Project().Tasks[id]
	if task != nil {
		out.Assignee = task.Assignee.V
	}
	if parent := b.parent(id); parent != "" {
		out.Parent = &view.Parent{TaskRef: b.ref(parent), Order: order(task)}
	}
	if a := f.Authorisation(id); a != nil {
		out.Authorised = a.Authorised
	}
	if q.Level < Detail {
		return out
	}
	out.TaskDetail = b.taskDetail(id, task)
	if q.Level < Provenance {
		return out
	}
	out.TaskProvenance = b.taskProvenance(id)
	return out
}

// order returns the place a task's file states under its parent, or nil
// when it states none.
func order(task *model.Task) *int {
	if task == nil || task.Parent == nil || !task.Parent.Order.OK {
		return nil
	}
	n := task.Parent.Order.V
	return &n
}

// children returns the children of a task of the project f holds, each with
// its place and its status, in display order.
func (b builder) children(f *derive.Facts, id string) []view.Child {
	out := []view.Child{}
	for _, child := range f.Children(id) {
		out = append(out, view.Child{TaskRef: refIn(f, child), Order: order(f.Project().Tasks[child]), Status: b.status(f, f.Status(child))})
	}
	return out
}

// taskDetail returns what the task definition adds at detail.
func (b builder) taskDetail(id string, task *model.Task) *view.TaskDetail {
	f := b.f
	d := &view.TaskDetail{
		References:  []view.Reference{},
		Path:        []view.TaskRef{},
		Children:    b.children(f, id),
		Authorities: []view.Authority{},
		Requires:    []view.Requirement{},
		Dependents:  []view.Requirement{},
		Junctions:   []view.Junction{},
	}
	if task != nil {
		d.Description = task.Description.V
		d.References = b.refs(task.References)
	}
	authorities := f.Authorities(id)
	for i := len(authorities) - 1; i >= 0; i-- {
		d.Path = append(d.Path, b.ref(authorities[i].Task))
	}
	d.Path = append(d.Path, b.ref(id))
	for _, a := range authorities {
		d.Authorities = append(d.Authorities, view.Authority{Email: a.Email, By: a.Task})
	}
	if parent := b.parent(id); parent != "" {
		d.Siblings = len(f.Children(parent))
	}
	for _, c := range f.Requires(id) {
		d.Requires = append(d.Requires, b.requirement(c, false))
	}
	for _, c := range f.Dependents(id) {
		d.Dependents = append(d.Dependents, b.requirement(c, true))
	}
	for _, j := range f.Junctions(id) {
		d.Junctions = append(d.Junctions, b.junction(j, b.q.Level >= Provenance))
	}
	if s := f.Status(id); s != nil && s.Kind == derive.Snapshotted && s.Snapshot != nil {
		d.Snapshot = b.snapshot(s.Snapshot)
	}
	return d
}

// snapshot returns the task a snapshot reads, with its assignee, its status
// and its children as the subproject's facts give them.
func (b builder) snapshot(n *derive.Snapshot) *view.Snapshot {
	out := &view.Snapshot{SubRef: *b.subRef(n), Children: []view.Child{}}
	sub := n.Facts
	if sub == nil || sub.Project() == nil {
		return out
	}
	if task := sub.Project().Tasks[n.Target]; task != nil {
		out.Assignee = task.Assignee.V
	}
	if s := sub.Status(n.Target); s != nil {
		status := b.status(sub, s)
		out.Status = &status
	}
	out.Children = b.children(sub, n.Target)
	return out
}

// taskProvenance returns what the task definition adds at provenance.
func (b builder) taskProvenance(id string) *view.TaskProvenance {
	f := b.f
	p := &view.TaskProvenance{
		StatusCommit:  b.statusCommit(id),
		Authorisation: b.authorisation(id),
		Considered:    []string{},
		Chain:         b.chain(id),
		Linkages:      b.linkages(id),
		Sources:       []view.Supplier{},
		Reviews:       []view.Review{},
		Models:        []view.ModelCheck{},
		Events:        []view.Event{},
		Commands:      []view.Command{},
	}
	if s := f.Status(id); s != nil {
		p.OwnCommit = b.commit(s.Own)
		p.Considered = append(p.Considered, s.Considered...)
	}
	for _, j := range f.Junctions(id) {
		p.Sources = append(p.Sources, b.suppliers(j)...)
	}
	for _, r := range f.Reviews(id) {
		effect := "none"
		switch {
		case r.ByAuthorisation:
			effect = "authorisation"
		case r.Commit != nil:
			effect = "accepts"
		}
		p.Reviews = append(p.Reviews, view.Review{Gate: r.Gate, Reviewer: r.Reviewer, Effect: effect, By: r.By, Commit: b.commit(r.Commit)})
	}
	events := f.Events(id)
	p.EventCount = len(events)
	for _, e := range events {
		own := e.Sub == "" && (e.Kind == derive.StatusSet || e.Kind == derive.Reaffirmed)
		if !own || e.Reading == nil || e.Reading.Verdict == derive.NoModel || e.Commit == nil {
			continue
		}
		p.Models = append(p.Models, view.ModelCheck{
			Commit: e.Commit.ID, Gate: e.Reading.Gate, Stated: e.Reading.Stated, Trailer: e.Reading.Trailer, Reading: reading(e.Reading.Verdict),
		})
	}
	for i := len(events) - 1; i >= 0 && len(p.Events) < newestEvents; i-- {
		p.Events = append(p.Events, event(events[i]))
	}
	if log := f.Log(); log != nil {
		text := "git log --format='%H %as %ae %s' " + log.Source + " -- " + b.fromRoot(path.Join("tasks", id+".yaml")) + " " + b.fromRoot(path.Join("status", id+".yaml"))
		for _, n := range f.Snapshots(id) {
			if n.Link != nil && n.Link.Form == model.Submodule {
				text += " " + n.Link.URL
			}
		}
		p.Commands = append(p.Commands, view.Command{Text: text})
	}
	return p
}

// reading returns the word of a model check's verdict.
func reading(v derive.Verdict) string {
	switch v {
	case derive.Match:
		return "agrees"
	case derive.Mismatch:
		return "differs"
	case derive.Missing:
		return "missing"
	case derive.Exempt:
		return "exempt"
	case derive.Unstated:
		return "unstated"
	}
	return ""
}

// event returns one event of a task as a row.
func event(e *derive.Event) view.Event {
	out := view.Event{Kind: string(e.Kind)}
	if c := e.Commit; c != nil {
		out.Date, out.Commit, out.By = c.Date(), c.ID, c.Author.Email
	}
	switch e.Kind {
	case derive.StatusSet:
		out.Recorded = &view.EventStatus{Gate: e.Gate, State: e.State, Reason: e.Reason, Note: e.Note}
	case derive.Reviewed:
		out.Gate = e.Gate
	case derive.Pinned:
		out.Pin = &view.PinMove{URL: e.URL, Old: e.Old, New: e.New}
	}
	if e.Sub != "" {
		sub := &view.SubEvent{Task: e.Sub, Title: e.SubTitle}
		if e.Link != nil {
			sub.URL = e.Link.URL
		}
		out.Subproject = sub
	}
	return out
}

// way returns the word of the way a deciding commit accepts.
func way(w derive.Way) string {
	switch w {
	case derive.ByCommit:
		return "change"
	case derive.ByMerge:
		return "merge"
	case derive.ByTrailer:
		return "trailer"
	}
	return ""
}

// hand says which of a deciding commit's two identities is an authority:
// "" when the task is proposed.
func hand(a *derive.Authorisation) string {
	switch {
	case !a.Authorised:
		return ""
	case a.Author && a.Committer:
		return "both"
	case a.Author:
		return "author"
	}
	return "committer"
}

// statusCommit returns the deciding commit of a task's status: the commit
// of the last status of its chain, so a roll-up shows its leaf's. It is nil
// without one.
func (b builder) statusCommit(id string) *view.Commit {
	chain := b.f.Chain(id)
	if len(chain) == 0 {
		return nil
	}
	return b.commit(chain[len(chain)-1].Commit)
}

// authorisation returns a task's authorisation and the commit that decides
// it.
func (b builder) authorisation(id string) view.Authorisation {
	out := view.Authorisation{Judges: []string{}}
	a := b.f.Authorisation(id)
	if a == nil {
		return out
	}
	out.Commit, out.Way, out.By, out.Why = b.commit(a.Commit), way(a.Way), hand(a), why(a.Why)
	if a.Commit != nil {
		out.Email = a.By
	}
	out.Judges = append(out.Judges, a.Judges...)
	out.DiffersFromTrunk, out.Uncommitted = a.Differs, a.Uncommitted
	return out
}

// linkages returns one linkage per snapshot of a task: the subproject its
// recursive junctions read and what fixes the commit they read. It is never
// nil.
func (b builder) linkages(id string) []view.Linkage {
	out := []view.Linkage{}
	f := b.f
	events := f.Events(id)
	for _, n := range f.Snapshots(id) {
		l := view.Linkage{Gates: append([]string{}, n.Gates...), SubRef: *b.subRef(n), OnTrunk: derive.Unknown.String(), Behind: derive.Unknown.String()}
		if len(n.Gates) > 0 {
			if j := f.Junction(id, n.Gates[0]); j != nil {
				l.Stated = view.Stated{By: j.Entry, Key: "junctions." + j.Gate + ".subproject.url", File: taskFile(j.Entry)}
			}
		}
		if n.Entry != nil {
			if node := n.Entry.URL.Node; node != nil {
				l.Line, l.Col = node.Pos.Line, node.Pos.Col
			}
			l.StatedCommit = n.Entry.Commit.V
		}
		if pin := f.Pin(n.Link); pin != nil {
			l.Recorded, l.Moved, l.Tip = pin.Recorded, pin.Moved, pin.Tip
			l.OnTrunk, l.Behind = pin.OnTrunk.String(), pin.Behind.String()
			if n.Facts != nil {
				l.Trunk = n.Facts.Trunk().Name
				if log := n.Facts.Log(); log != nil {
					l.Pinned = b.commit(log.Commit(pin.Commit))
				}
			}
			for i := len(events) - 1; i >= 0; i-- {
				if e := events[i]; e.Kind == derive.Pinned && e.Sub == "" && e.URL == n.Link.URL {
					l.SetBy = b.commit(e.Commit)
					break
				}
			}
		}
		out = append(out, l)
	}
	return out
}
