package views

import (
	"path"
	"sort"
	"time"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// The kinds of a queue item, in the queue's order.
const (
	itemReview        = "review"
	itemAuthorisation = "authorisation"
	itemReady         = "ready"
	itemReaffirmation = "reaffirmation"
	itemWaiting       = "waiting"
)

// The kinds of a reason a work item waits, in the order an item lists them.
const (
	waitRequirement   = "requirement"
	waitReason        = "reason"
	waitAuthorisation = "authorisation"
	waitReview        = "review"
)

// reserved is the reason the method reserves for the hand-off.
const reserved = "review"

// Queue returns the work queue of q.Person within the subtree under q.Task,
// or, with q.Brief, the brief of one of its items. With q.Brief the items
// are empty, and the brief is null unless the person's queue holds work,
// ready or waiting, for that task at that gate.
func Queue(f *derive.Facts, q Query) *view.Queue {
	b := builder{f, q}
	out := &view.Queue{Legend: b.legend(), Items: []view.QueueItem{}}
	items := b.items()
	if q.Brief == nil {
		out.Items = items
	}
	if q.Level < Provenance {
		return out
	}
	out.QueueProvenance = &view.QueueProvenance{}
	if q.Brief == nil {
		return out
	}
	for _, item := range items {
		if (item.Kind == itemReady || item.Kind == itemWaiting) && item.Task.ID == q.Brief.Task && item.Gate == q.Brief.Gate {
			out.Brief = b.brief(item)
			break
		}
	}
	return out
}

// proposed reports whether a task waits for its authority. On the trunk the
// history decides: the task is not authorised, by a deciding commit that is
// no authority's or for want of one. Off the trunk every task reads as
// proposed, and those whose file differs from the trunk's tip are the
// branch's proposals. With no trunk and with no history, no task is
// (decision 9).
func (b builder) proposed(id string) bool {
	a := b.f.Authorisation(id)
	if a == nil || a.Authorised {
		return false
	}
	switch a.Why {
	case derive.Determined, derive.NoCommit:
		return true
	case derive.OffTrunk:
		return a.Differs
	}
	return false
}

// authority returns the nearest authority of a task: the assignee of its
// parent, and the owner for the root.
func (b builder) authority(id string) string {
	if authorities := b.f.Authorities(id); len(authorities) > 0 {
		return authorities[0].Email
	}
	return b.f.Owner()
}

// judges reports whether a person is an authority of a task: the assignee
// of one of its ancestors, or the owner.
func (b builder) judges(id, person string) bool {
	if person == b.f.Owner() {
		return true
	}
	for _, a := range b.f.Authorities(id) {
		if a.Email == person {
			return true
		}
	}
	return false
}

// waits returns why the work at a leaf's next junction waits, in the order
// requirement, reason, authorisation, review: each due requirement that is
// not met; a reason on the status other than the reserved one (decision
// 12); a task that is proposed, which waits for its nearest authority; and
// a hand-off the status states, which waits for the reviewer (decision 7).
// A reason carries the status's note. It is never nil.
func (b builder) waits(id string) []view.Wait {
	f := b.f
	out := []view.Wait{}
	for _, c := range f.Requires(id) {
		if c.Due && !c.Met && c.Why == derive.Determined {
			r := b.requirement(c, false)
			out = append(out, view.Wait{Kind: waitRequirement, Requires: &r})
		}
	}
	if s := f.Status(id); s != nil && s.Reason != "" && s.Reason != reserved {
		out = append(out, view.Wait{Kind: waitReason, Reason: s.Reason, Note: s.Note})
	}
	if b.proposed(id) {
		out = append(out, view.Wait{Kind: waitAuthorisation, By: b.authority(id)})
	}
	if h := f.Handoff(id); h != nil && h.Kind == derive.Stated {
		out = append(out, view.Wait{Kind: waitReview, By: h.Reviewer})
	}
	return out
}

// age returns the days from a date to the date of the source commit, or 0
// without one and for a date that does not read.
func (b builder) age(since string) int {
	log := b.f.Log()
	if log == nil {
		return 0
	}
	source := log.Commit(log.Source)
	if source == nil {
		return 0
	}
	from, err := time.Parse(time.DateOnly, since)
	if err != nil {
		return 0
	}
	to, err := time.Parse(time.DateOnly, source.Date())
	if err != nil || to.Before(from) {
		return 0
	}
	return int(to.Sub(from).Hours() / 24)
}

// items returns the person's items within the subtree under the query's
// task, in the queue's order: the kinds as listed; within a kind display
// order; the reaffirmations by the author time of the status's deciding
// commit, the oldest first. From detail on each item carries its detail.
// It is never nil.
func (b builder) items() []view.QueueItem {
	f := b.f
	person := b.q.Person
	byKind := map[string][]view.QueueItem{}
	recorded := map[string]int64{} // by task: the author time of a reaffirmation's commit
	if person == "" {
		return []view.QueueItem{}
	}
	for _, id := range b.subtree(b.top()) {
		s := f.Status(id)
		if s == nil {
			continue
		}
		j := b.nextJunction(id)
		add := func(kind, gate string, causes []view.Wait) {
			item := view.QueueItem{Kind: kind, Task: b.ref(id), Gate: gate, Since: s.Date, Causes: causes}
			if j != nil && (kind == itemReview || kind == itemReady || kind == itemWaiting) {
				item.Model = j.Model.V
			}
			if kind == itemReaffirmation {
				item.Age = b.age(s.Date)
			}
			if a := f.Authorisation(id); kind == itemAuthorisation && a != nil {
				item.Why = why(a.Why)
			}
			if b.q.Level >= Detail && b.q.Brief == nil {
				item.ItemDetail = b.itemDetail(id, item)
			}
			byKind[kind] = append(byKind[kind], item)
		}
		if h := f.Handoff(id); h != nil && h.Kind == derive.Stated && h.Reviewer == person {
			add(itemReview, h.Gate, []view.Wait{})
		}
		if b.proposed(id) && b.judges(id, person) {
			add(itemAuthorisation, s.Gate, []view.Wait{})
		}
		// A recursive next junction is no work of this project: the
		// subproject's own queue holds it (decision 8).
		if j != nil && j.Kind == model.Plain && j.Contributor.V == person {
			if waits := b.waits(id); len(waits) > 0 {
				add(itemWaiting, j.Gate, waits)
			} else {
				add(itemReady, j.Gate, waits)
			}
		}
		if f.Leaf(id) && s.Kind == derive.Recorded && s.Recorder == person {
			if s.Commit != nil {
				recorded[id] = s.Commit.Time
			}
			add(itemReaffirmation, s.Gate, []view.Wait{})
		}
	}
	oldest := byKind[itemReaffirmation]
	sort.SliceStable(oldest, func(i, j int) bool { return recorded[oldest[i].Task.ID] < recorded[oldest[j].Task.ID] })
	out := []view.QueueItem{}
	for _, kind := range []string{itemReview, itemAuthorisation, itemReady, itemReaffirmation, itemWaiting} {
		out = append(out, byKind[kind]...)
	}
	return out
}

// itemDetail returns what an item adds at detail. The dependents stand in
// two lists: those that passing the item's gate meets, and the rest; the
// parent's dependents pass the same test. A task with no next junction, a
// parent or a leaf at its last gate, unblocks nothing.
func (b builder) itemDetail(id string, item view.QueueItem) *view.ItemDetail {
	f := b.f
	d := &view.ItemDetail{
		Sources:        []view.Supplier{},
		TaskReferences: []view.Reference{},
		Requires:       []view.Requirement{},
		Unblocks:       []view.Requirement{},
		AlsoRequiredBy: []view.Requirement{},
		ParentUnblocks: []view.ParentEdge{},
		Status:         b.status(f, f.Status(id)),
		StatusCommit:   b.statusCommit(id),
	}
	j := b.nextJunction(id)
	if j != nil {
		junction := b.junction(j, true)
		d.Junction, d.Sources = &junction, b.suppliers(j)
	}
	if task := f.Project().Tasks[id]; task != nil {
		d.TaskReferences = b.refs(task.References)
	}
	for _, c := range f.Requires(id) {
		d.Requires = append(d.Requires, b.requirement(c, false))
	}
	gate := f.Index(item.Gate)
	// meets reports whether passing the item's gate meets a dependent's
	// condition: it is not met, and its from gate lies at or before the gate.
	meets := func(c *derive.Condition) bool {
		from := f.Index(c.From)
		return j != nil && !c.Met && c.Why == derive.Determined && from >= 0 && from <= gate
	}
	for _, c := range f.Dependents(id) {
		if meets(c) {
			d.Unblocks = append(d.Unblocks, b.requirement(c, true))
		} else {
			d.AlsoRequiredBy = append(d.AlsoRequiredBy, b.requirement(c, true))
		}
	}
	if parent := b.parent(id); parent != "" {
		for _, c := range f.Dependents(parent) {
			if meets(c) {
				d.ParentUnblocks = append(d.ParentUnblocks, view.ParentEdge{Parent: b.ref(parent), Requires: b.requirement(c, true)})
			}
		}
	}
	if item.Kind == itemAuthorisation {
		a := b.authorisation(id)
		d.Authorisation = &a
	}
	return d
}

// briefSource names where a field of a resolved junction comes from: the
// task whose entry states it, the assignee, or the plain default.
func (b builder) briefSource(field derive.Field) view.BriefSource {
	switch field.By {
	case derive.ByTask:
		ref := b.ref(field.Task)
		return view.BriefSource{Kind: "task", ID: ref.ID, Title: ref.Title}
	case derive.ByAssignee:
		return view.BriefSource{Kind: "assignee"}
	}
	return view.BriefSource{Kind: "default"}
}

// briefEdge returns one requirement as a brief states it. A requirement
// names its originating task, with its title from the project that holds
// it; a dependent names the terminating task.
func (b builder) briefEdge(c *derive.Condition, dependent bool) view.BriefEdge {
	e := view.BriefEdge{TaskRef: refIn(c.Facts, c.Origin), From: c.From, To: c.To, Condition: c.Word()}
	if dependent {
		e.TaskRef = b.ref(c.Task)
	}
	if c.Entry != nil {
		e.Text = c.Entry.Text.V
	}
	if c.Link != nil {
		e.URL, e.Commit = c.Link.URL, c.Link.Commit
	}
	return e
}

// brief returns one item of work with every fact a brief states: the task,
// the gate, the junction with the source of each field, the requirements at
// both ends, the parent and the status with its file.
func (b builder) brief(item view.QueueItem) *view.Brief {
	f := b.f
	id, gate := item.Task.ID, item.Gate
	legend := b.legend()
	out := &view.Brief{
		Kind: item.Kind, Causes: item.Causes,
		Task:       view.BriefTask{TaskRef: item.Task, References: []view.Reference{}, Path: []view.TaskRef{}},
		Gate:       view.BriefGate{Gate: view.Gate{Key: gate}},
		Junction:   view.BriefJunction{References: []view.BriefReference{}},
		Requires:   []view.BriefEdge{},
		Dependents: []view.BriefEdge{},
	}
	if task := f.Project().Tasks[id]; task != nil {
		out.Task.Description = task.Description.V
		out.Task.References = b.refs(task.References)
	}
	authorities := f.Authorities(id)
	for i := len(authorities) - 1; i >= 0; i-- {
		out.Task.Path = append(out.Task.Path, b.ref(authorities[i].Task))
	}
	for _, g := range legend.Gates {
		if g.Key == gate {
			out.Gate.Gate = g
		}
	}
	if j := f.Junction(id, f.Next(id, gate)); j != nil {
		out.Gate.Follows = j.Kind.String()
	}
	if j := f.Junction(id, gate); j != nil {
		out.Junction.Contributor = view.BriefField{Value: j.Contributor.V, Source: b.briefSource(j.Contributor)}
		out.Junction.Model = view.BriefField{Value: j.Model.V, Source: b.briefSource(j.Model)}
		out.Junction.Reviewer = view.BriefField{Value: j.Reviewer.V, Source: b.briefSource(j.Reviewer)}
		from := b.ref(j.ReferencesFrom)
		for _, r := range j.References {
			out.Junction.References = append(out.Junction.References, view.BriefReference{
				URL: r.URL.V, Text: r.Text.V, Source: view.BriefSource{Kind: "task", ID: from.ID, Title: from.Title},
			})
		}
		if j.Contributor.By == derive.ByTask && j.Contributor.Task != id {
			supplier := &view.BriefSupplier{TaskRef: b.ref(j.Contributor.Task)}
			if task := f.Project().Tasks[j.Contributor.Task]; task != nil {
				supplier.Description = task.Description.V
			}
			out.Supplier = supplier
		}
	}
	for _, c := range f.Requires(id) {
		out.Requires = append(out.Requires, b.briefEdge(c, false))
	}
	for _, c := range f.Dependents(id) {
		out.Dependents = append(out.Dependents, b.briefEdge(c, true))
	}
	if parent := b.parent(id); parent != "" {
		out.Parent = &view.BriefParent{TaskRef: b.ref(parent), Dependents: []view.BriefEdge{}}
		for _, c := range f.Dependents(parent) {
			out.Parent.Dependents = append(out.Parent.Dependents, b.briefEdge(c, true))
		}
	}
	out.Status.Path = path.Join(f.Project().Where.Dir, planDir, "status", id+".yaml")
	if s := f.Status(id); s != nil {
		st := &out.Status
		st.Gate, st.State, st.Reason, st.Note, st.Date, st.Recorder = s.Gate, s.State, s.Reason, s.Note, s.Date, s.Recorder
		if s.Commit != nil {
			st.Commit = s.Commit.ID
		}
		for _, g := range legend.Gates {
			if g.Key == s.Gate {
				st.GateSymbol = g.Symbol
			}
		}
		for _, state := range legend.States {
			if state.Key == s.State {
				st.StateSymbol = state.Symbol
			}
		}
		for _, r := range legend.Reasons {
			if r.Key == s.Reason {
				st.ReasonSymbol = r.Symbol
			}
		}
	}
	return out
}
