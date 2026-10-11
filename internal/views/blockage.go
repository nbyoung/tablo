package views

import (
	"sort"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// The kinds of a cause, in VIEWS.md's order.
const (
	causeRequirement   = "requirement"
	causeStatus        = "status"
	causeReview        = "review"
	causeAuthorisation = "authorisation"
	causeSnapshot      = "snapshot"
)

// causeKinds lists the kinds of a cause in VIEWS.md's order.
var causeKinds = []string{causeRequirement, causeStatus, causeReview, causeAuthorisation, causeSnapshot}

// The acts that resolve a cause.
const (
	actContributes = "contributes"
	actRecords     = "records"
	actReviews     = "reviews"
	actAuthorises  = "authorises"
	actAdvances    = "advances"
)

// The ways a cause holds a task.
const (
	viaSelf        = "self"
	viaParent      = "parent"
	viaRequirement = "requirement"
)

// What a snapshot cause advances.
const (
	advancePin  = "pin"
	advanceGate = "gate"
)

// cause is one root of the tree as the view builds it: the cause as the
// glance shows it, the tasks it holds, and what its provenance reads.
type cause struct {
	view.Cause
	held  []view.Held     // the tasks it holds directly, each with what it holds in turn
	tasks map[string]bool // the distinct tasks it holds, at every depth
	place int             // the place of its task in display order; a subproject's task comes last
	far   bool            // its task is of another project

	facts    *derive.Facts    // the facts of the project its task is of
	origin   string           // its task, in that project
	junction *derive.Junction // its task's next junction, or nil
	link     *model.Link      // a snapshot: the link
}

// Blockage returns the work-blockage tree, selected by q.Task and q.Person.
func Blockage(f *derive.Facts, q Query) *view.Blockage {
	b := builder{f, q}
	out := &view.Blockage{Legend: b.legend(), Causes: []view.Cause{}, Kinds: []view.KindCount{}}
	causes := b.selected(b.causes())
	// The largest first; then by task in display order, a subproject's task
	// last, by url, then id; then by kind; then a pin before a gate.
	rank := map[string]int{}
	for i, kind := range causeKinds {
		rank[kind] = i
	}
	sort.SliceStable(causes, func(i, j int) bool {
		x, y := causes[i], causes[j]
		switch {
		case x.Holds != y.Holds:
			return x.Holds > y.Holds
		case x.place != y.place:
			return x.place < y.place
		case x.far && x.URL != y.URL:
			return x.URL < y.URL
		case x.far && x.Task.ID != y.Task.ID:
			return x.Task.ID < y.Task.ID
		case x.Kind != y.Kind:
			return rank[x.Kind] < rank[y.Kind]
		case x.Advance != y.Advance:
			return x.Advance == advancePin
		}
		return x.URL < y.URL
	})
	counts := map[string]int{}
	for i, c := range causes {
		counts[c.Kind]++
		if q.Level >= Detail {
			also(c.held, i, causes)
			c.CauseDetail = &view.CauseDetail{Held: c.held}
		}
		if q.Level >= Provenance {
			c.CauseProvenance = b.causeProvenance(c)
		}
		out.Causes = append(out.Causes, c.Cause)
	}
	for _, kind := range causeKinds {
		out.Kinds = append(out.Kinds, view.KindCount{Kind: kind, Count: counts[kind]})
	}
	// The requirements not yet due, of the tasks in the subtree, and of the
	// person's own tasks among them with a person.
	tasks := b.subtree(b.top())
	if q.Person != "" {
		own := map[string]bool{}
		for _, id := range b.own(q.Person) {
			own[id] = true
		}
		var kept []string
		for _, id := range tasks {
			if own[id] {
				kept = append(kept, id)
			}
		}
		tasks = kept
	}
	waiting := []view.Waiting{}
	for _, id := range tasks {
		row := view.Waiting{Task: b.ref(id), Status: b.status(f, f.Status(id)), Next: b.next(id), Requires: []view.Requirement{}}
		for _, c := range f.Requires(id) {
			if c.Due || c.Why != derive.Determined {
				continue
			}
			out.NotDueCount++
			if !c.Met {
				out.NotDueUnmet++
			}
			row.Requires = append(row.Requires, b.requirement(c, false))
		}
		if len(row.Requires) > 0 {
			waiting = append(waiting, row)
		}
	}
	if q.Level >= Detail {
		out.BlockageDetail = &view.BlockageDetail{NotDue: waiting}
	}
	if q.Level >= Provenance {
		out.BlockageProvenance = &view.BlockageProvenance{Commands: []view.Command{}}
		if log := f.Log(); log != nil {
			out.Commands = append(out.Commands, view.Command{
				Text: "git log --format='%H %as %ae %ce %s' " + log.Source + " -- " + b.fromRoot("status") + " " + b.fromRoot("tasks"),
			})
		}
	}
	return out
}

// unmet reports whether a condition is due and not met.
func unmet(c *derive.Condition) bool {
	return c.Due && !c.Met && c.Why == derive.Determined
}

// across reports whether a condition is of a cross-project entry.
func across(c *derive.Condition) bool {
	return c.Link != nil || c.Entry != nil && c.Entry.Subproject != nil
}

// mildest returns the severity of the project's mildest working state: the
// lowest severity above 0 among the states of gates.yaml, or 0 for none.
func (b builder) mildest() int {
	low := 0
	for _, s := range b.f.Project().Gating.States {
		if v := b.f.Severity(s.Key.V); v > 0 && (low == 0 || v < low) {
			low = v
		}
	}
	return low
}

// troubled reports whether a status is a cause by structure: it states a
// reason other than the reserved one, or its state is more severe than the
// project's mildest working state (decision 12).
func (b builder) troubled(s *derive.Status) bool {
	if s.Reason != "" && s.Reason != reserved {
		return true
	}
	mild := b.mildest()
	return mild > 0 && b.f.Severity(s.State) > mild
}

// holds returns what a task holds, in order: its parent, when the task is
// one of the children the parent's roll-up considers and stands at the gate
// the parent stands at; then each task whose due requirement on it is not
// met, in the order of the dependents. A task on the path from the cause is
// not held again, so a cycle ends. It is never nil.
func (b builder) holds(id string, path map[string]bool) []view.Held {
	f := b.f
	out := []view.Held{}
	if parent := b.parent(id); parent != "" && !path[parent] {
		above, here := f.Status(parent), f.Status(id)
		if above != nil && here != nil && above.Gate == here.Gate && has(above.Considered, id) {
			out = append(out, b.hold(parent, b.next(parent), viaParent, id, nil, path))
		}
	}
	for _, c := range f.Dependents(id) {
		if unmet(c) && !path[c.Task] {
			out = append(out, b.hold(c.Task, c.To, viaRequirement, "", c, path))
		}
	}
	return out
}

// hold returns a task a cause holds at a gate, with what it holds in turn.
func (b builder) hold(id, gate, via, child string, c *derive.Condition, path map[string]bool) view.Held {
	f := b.f
	h := view.Held{Task: b.ref(id), Gate: gate, Parent: len(f.Children(id)) > 0, Via: via, Child: child, Also: []int{}}
	if c != nil {
		r := b.requirement(c, false)
		h.Requires = &r
	}
	if f.Leaf(id) {
		if j := f.Junction(id, gate); j != nil && j.Kind == model.Plain {
			h.Contributor = j.Contributor.V
		}
	}
	path[id] = true
	h.Held = b.holds(id, path)
	delete(path, id)
	return h
}

// beneath returns the leaves beneath a task that keep it at its gate: the
// task itself when it is a leaf, else the leaves beneath each child that
// its roll-up considers and that stands at its gate.
func (b builder) beneath(id string) []string {
	f := b.f
	if f.Leaf(id) {
		return []string{id}
	}
	var list []string
	s := f.Status(id)
	if s == nil {
		return list
	}
	for _, child := range s.Considered {
		if c := f.Status(child); c != nil && c.Gate == s.Gate {
			list = append(list, b.beneath(child)...)
		}
	}
	return list
}

// newCause returns a cause at a task of the project in view, with its
// status and its next junction.
func (b builder) newCause(kind, id, act string) *cause {
	f := b.f
	c := &cause{
		Cause: view.Cause{Kind: kind, Task: b.ref(id), Status: b.status(f, f.Status(id)), Act: act},
		facts: f, origin: id, junction: b.nextJunction(id),
	}
	for i, other := range f.Order() {
		if other == id {
			c.place = i
		}
	}
	return c
}

// mark returns the first mark of a junction, or "" for none.
func mark(j *derive.Junction) string {
	if j != nil {
		if marks := j.Marks(); len(marks) > 0 {
			return string(marks[0])
		}
	}
	return ""
}

// contributor returns the contributor of a plain junction, or "".
func contributor(j *derive.Junction) string {
	if j != nil && j.Kind == model.Plain {
		return j.Contributor.V
	}
	return ""
}

// causes returns every cause of the project, each with the tasks it holds,
// in no order a reader sees. The first four kinds hold the task itself and
// through it what it holds: a status that is a cause by structure, a
// hand-off the status states, a task that is proposed, and a snapshot. A
// requirement roots at each leaf beneath its originating task that nothing
// else holds (decision 10), or, across projects, at the originating task.
func (b builder) causes() []*cause {
	f := b.f
	var list []*cause
	self := map[string]bool{} // the leaves one of the first four kinds holds
	own := func(c *cause, id string) {
		c.held = []view.Held{b.hold(id, b.next(id), viaSelf, "", nil, map[string]bool{})}
		self[id] = true
		list = append(list, c)
	}
	for _, id := range f.Order() {
		s := f.Status(id)
		if s == nil {
			continue
		}
		if f.Leaf(id) && b.troubled(s) {
			c := b.newCause(causeStatus, id, actRecords)
			c.Gate, c.Mark = s.Gate, mark(c.junction)
			if c.Resolver = contributor(c.junction); c.Resolver == "" {
				c.Resolver = s.Recorder
			}
			own(c, id)
		}
		if h := f.Handoff(id); h != nil && h.Kind == derive.Stated {
			c := b.newCause(causeReview, id, actReviews)
			c.Gate, c.Resolver, c.Mark = h.Gate, h.Reviewer, string(derive.MarkReviewer)
			own(c, id)
		}
		if b.proposed(id) {
			c := b.newCause(causeAuthorisation, id, actAuthorises)
			c.Gate, c.Resolver = s.Gate, b.authority(id)
			own(c, id)
		}
	}
	list = append(list, b.snapshots(self)...)
	list = append(list, b.requirements(self)...)
	for _, c := range list {
		c.tasks = map[string]bool{}
		gather(c.held, c.tasks)
		c.Holds = len(c.tasks)
	}
	return list
}

// gather adds the tasks of a list of held tasks, at every depth, to a set.
func gather(list []view.Held, into map[string]bool) {
	for _, h := range list {
		into[h.Task.ID] = true
		gather(h.Held, into)
	}
}

// snapshots returns the snapshot causes, and adds the leaves they hold to
// self. A snapshot is a cause in two ways (decision 11). The pin advances
// when it is behind its trunk and the task a leaf reads stands at another
// gate or state at the tip, or when a due requirement that is not met is
// met at the tip: one cause per link, which holds each such leaf and each
// task with such a requirement, and which the assignee of the first of them
// resolves. The gate advances when the task read has passed the next gate
// of the leaf that reads it: one cause per leaf, which holds what the leaf
// holds, and which the leaf's assignee resolves.
func (b builder) snapshots(self map[string]bool) []*cause {
	f := b.f
	var list []*cause
	byLink := map[*model.Link]*cause{}
	// pinned returns the cause of a link whose pin advances, made at the
	// first task it holds.
	pinned := func(link *model.Link, id string) *cause {
		if c := byLink[link]; c != nil {
			return c
		}
		c := b.snapshotCause(link, id, advancePin)
		byLink[link] = c
		list = append(list, c)
		return c
	}
	for _, id := range f.Order() {
		s := f.Status(id)
		if s != nil && s.Kind == derive.Snapshotted && s.Snapshot != nil && s.Snapshot.Link != nil && s.Of != nil {
			link := s.Snapshot.Link
			if pin := f.Pin(link); pin != nil && pin.Behind == derive.Yes {
				if tip := f.AtTip(s.Snapshot); tip != nil && (tip.Gate != s.Of.Gate || tip.State != s.Of.State) {
					c := pinned(link, id)
					c.held = append(c.held, b.hold(id, b.next(id), viaSelf, "", nil, map[string]bool{}))
					self[id] = true
				}
			}
			if next := f.Index(b.next(id)); next >= 0 && f.Index(s.Of.Gate) >= next {
				c := b.snapshotCause(link, id, advanceGate)
				c.held = b.holds(id, map[string]bool{id: true})
				self[id] = true
				list = append(list, c)
			}
		}
		for _, c := range f.Requires(id) {
			if c.Link != nil && unmet(c) && c.AtTrunk == derive.Yes {
				at := pinned(c.Link, id)
				at.held = append(at.held, b.hold(id, c.To, viaRequirement, "", c, map[string]bool{}))
			}
		}
	}
	return list
}

// snapshotCause returns a snapshot cause of a link at a task of the project in
// view, which its assignee resolves.
func (b builder) snapshotCause(link *model.Link, id, advance string) *cause {
	c := b.newCause(causeSnapshot, id, actAdvances)
	c.URL, c.Gate, c.Resolver, c.Mark, c.Advance = link.URL, c.Status.Gate, b.assignee(id), string(derive.MarkSubproject), advance
	c.link, c.held = link, []view.Held{}
	if pin := b.f.Pin(link); pin != nil {
		c.Pin, c.Tip = pin.Commit, pin.Tip
	}
	return c
}

// requirements returns the requirement causes. A due local requirement
// that is not met roots at each leaf beneath its originating task that has
// none of the first four causes and no such requirement of its own, at that
// leaf's next gate, where its contributor acts; the cause holds what the
// leaf holds, and not the leaf. Any other leaf carries the cause that holds
// it to the dependent. A cross-project requirement that the tip of the
// other trunk does not meet either roots at its originating task, one cause
// per link and task, read from the facts of that project.
func (b builder) requirements(self map[string]bool) []*cause {
	f := b.f
	var list []*cause
	rooted := map[string]*cause{}
	for _, id := range f.Order() {
		for _, c := range f.Requires(id) {
			if !unmet(c) {
				continue
			}
			if !across(c) {
				for _, leaf := range b.beneath(c.Origin) {
					if rooted[leaf] != nil || self[leaf] || b.waitsOnAnother(leaf) {
						continue
					}
					at := b.newCause(causeRequirement, leaf, actContributes)
					at.Gate, at.Resolver, at.Mark = b.next(leaf), contributor(at.junction), mark(at.junction)
					at.held = b.holds(leaf, map[string]bool{leaf: true})
					rooted[leaf] = at
					list = append(list, at)
				}
				continue
			}
			if c.Link == nil || c.Facts == nil || c.AtTrunk == derive.Yes {
				continue
			}
			key := c.Link.URL + "\x00" + c.Link.Commit + "\x00" + c.Origin
			at := rooted[key]
			if at == nil {
				at = b.foreign(c)
				rooted[key] = at
				list = append(list, at)
			}
			at.held = append(at.held, b.hold(id, c.To, viaRequirement, "", c, map[string]bool{}))
		}
	}
	return list
}

// waitsOnAnother reports whether a task has a due requirement of its own
// that is not met.
func (b builder) waitsOnAnother(id string) bool {
	for _, c := range b.f.Requires(id) {
		if unmet(c) {
			return true
		}
	}
	return false
}

// foreign returns the requirement cause at the originating task of a
// cross-project condition: its title, its status and its next junction read
// the facts of the project that holds it. Its contributor acts where the
// task is a leaf there with a plain next junction.
func (b builder) foreign(c *derive.Condition) *cause {
	there := builder{c.Facts, b.q}
	at := &cause{
		Cause: view.Cause{
			Kind: causeRequirement, Task: there.ref(c.Origin), URL: c.Link.URL,
			Status: there.status(c.Facts, c.Facts.Status(c.Origin)), Gate: there.next(c.Origin), Act: actContributes,
		},
		held: []view.Held{}, place: len(b.f.Order()), far: true,
		facts: c.Facts, origin: c.Origin, junction: there.nextJunction(c.Origin), link: c.Link,
	}
	at.Resolver, at.Mark = contributor(at.junction), mark(at.junction)
	return at
}

// selected returns the causes the query keeps. With a task other than the
// root, a cause stays when its task, if of this project, or a task it holds
// lies in the subtree. With a person, it stays when the person resolves it,
// or when its task or a task it holds is one of the person's own.
func (b builder) selected(causes []*cause) []*cause {
	f := b.f
	// touches reports whether a cause stands at, or holds, a task of a set.
	touches := func(c *cause, set map[string]bool) bool {
		if !c.far && set[c.origin] {
			return true
		}
		for id := range c.tasks {
			if set[id] {
				return true
			}
		}
		return false
	}
	if top := b.top(); top != f.Root() {
		within := map[string]bool{}
		for _, id := range b.subtree(top) {
			within[id] = true
		}
		var kept []*cause
		for _, c := range causes {
			if touches(c, within) {
				kept = append(kept, c)
			}
		}
		causes = kept
	}
	if person := b.q.Person; person != "" {
		own := map[string]bool{}
		for _, id := range b.own(person) {
			own[id] = true
		}
		var kept []*cause
		for _, c := range causes {
			if c.Resolver == person || touches(c, own) {
				kept = append(kept, c)
			}
		}
		causes = kept
	}
	return causes
}

// also writes, on each held task of the cause at index at, the other causes
// in view that hold the task, counted from 1.
func also(list []view.Held, at int, causes []*cause) {
	for i := range list {
		h := &list[i]
		for n, other := range causes {
			if n != at && other.tasks[h.Task.ID] {
				h.Also = append(h.Also, n+1)
			}
		}
		also(h.Held, at, causes)
	}
}

// causeProvenance returns what a cause adds at provenance: the deciding
// commit of its status, its task's next junction, the authorisation of a
// proposed task, the linkage of a snapshot, and the commit that resolves a
// review or an authorisation.
func (b builder) causeProvenance(c *cause) *view.CauseProvenance {
	there := builder{c.facts, b.q}
	p := &view.CauseProvenance{StatusCommit: there.statusCommit(c.origin), Resolve: []view.Command{}}
	if c.junction != nil {
		junction := there.junction(c.junction, true)
		p.Junction = &junction
	}
	switch c.Kind {
	case causeReview:
		p.Resolve = append(p.Resolve, view.Command{Text: "git commit --allow-empty --trailer 'Reviewed: " + c.origin + " " + c.Gate + "'"})
	case causeAuthorisation:
		a := b.authorisation(c.origin)
		p.Authorisation = &a
		p.Resolve = append(p.Resolve, view.Command{Text: "git commit --allow-empty --trailer 'Authorised: " + c.origin + "'"})
	case causeSnapshot:
		p.Linkage = b.linkage(c)
	}
	return p
}

// linkage returns the linkage of a snapshot cause: that of the first leaf
// it holds whose recursive junction reads the link, or, for a link that a
// requirement alone reads, the link with its pin.
func (b builder) linkage(c *cause) *view.Linkage {
	f := b.f
	ids := []string{c.origin}
	for _, h := range c.held {
		ids = append(ids, h.Task.ID)
	}
	for _, id := range ids {
		for i, n := range f.Snapshots(id) {
			if n.Link == c.link {
				return &b.linkages(id)[i]
			}
		}
	}
	l := &view.Linkage{
		Gates:   []string{},
		SubRef:  view.SubRef{URL: c.link.URL, Form: c.link.Form.String(), Commit: c.link.Commit},
		OnTrunk: derive.Unknown.String(), Behind: derive.Unknown.String(),
	}
	if pin := f.Pin(c.link); pin != nil {
		l.Recorded, l.Moved, l.Tip = pin.Recorded, pin.Moved, pin.Tip
		l.OnTrunk, l.Behind = pin.OnTrunk.String(), pin.Behind.String()
		if sub := f.Sub(c.link); sub != nil {
			l.Trunk = sub.Trunk().Name
			if log := sub.Log(); log != nil {
				l.Pinned = b.commit(log.Commit(pin.Commit))
			}
		}
	}
	return l
}
