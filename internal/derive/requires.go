package derive

import (
	"github.com/nbyoung/tablo/internal/model"
)

// Condition is one requires entry with its gates resolved and its condition.
type Condition struct {
	Task     string // the terminating task
	Entry    *model.Requirement
	Link     *model.Link // nil for an entry by id
	Origin   string      // the originating task
	Facts    *Facts      // the facts of the originating project; nil when the link has none
	From, To string
	Stands   string // the gate the originating task stands at
	Met, Due bool
	Why      Why // NoLink, NoTask or NoGate: Met and Due are then false
}

// Word returns "met", "unmet" or "pending", as the corpus writes the
// condition, or "" when it is undetermined.
func (c *Condition) Word() string {
	switch {
	case c.Why != Determined:
		return ""
	case c.Met:
		return "met"
	case c.Due:
		return "unmet"
	}
	return "pending"
}

// Requires returns the conditions of a task's requires entries, in file order.
func (f *Facts) Requires(id string) []*Condition {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return append([]*Condition(nil), f.requires(id)...)
}

// Dependents returns the conditions of the entries of this project that
// name the task as their originating task, by terminating task in display
// order, then in file order.
func (f *Facts) Dependents(id string) []*Condition {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	var list []*Condition
	for _, other := range f.order {
		for _, c := range f.requires(other) {
			if c.Link == nil && c.Entry.Subproject == nil && c.Origin == id {
				list = append(list, c)
			}
		}
	}
	return list
}

// requires resolves each entry of a task. from defaults to the originating
// task's last applicable gate in its own project, and to to this task's
// first applicable gate after undefined. A task stands at its status's gate:
// a leaf's file, a parent's roll-up. The requirement is met when the
// originating task stands at or past from, and due when no applicable gate
// of this task lies between the gate it stands at and to.
func (f *Facts) requires(id string) []*Condition {
	if list, ok := f.conditions[id]; ok {
		return list
	}
	task := f.task(id)
	if task == nil {
		return nil
	}
	list := make([]*Condition, 0, len(task.Requires))
	for _, e := range task.Requires {
		c := &Condition{Task: id, Entry: e, Origin: e.ID.V, Facts: f, From: e.From.V, To: e.To.V}
		if e.Subproject != nil {
			c.Origin = e.Subproject.ID.V
			c.Link, c.Facts = f.reach(e.Subproject)
		}
		list = append(list, c)
		origin := c.Facts
		if origin == nil {
			c.Why = NoLink
			continue
		}
		if origin.task(c.Origin) == nil {
			c.Why = NoTask
			continue
		}
		if c.From == "" {
			c.From = origin.last(c.Origin)
		}
		if c.To == "" {
			c.To = f.next(id, "undefined")
		}
		from, fromNamed := origin.index[c.From]
		to, toNamed := f.index[c.To]
		if !fromNamed || !toNamed {
			c.Why = NoGate
			continue
		}
		if there := origin.status(c.Origin); there != nil {
			c.Stands = there.Gate
		}
		if stands, named := origin.index[c.Stands]; named {
			c.Met = stands >= from
		}
		if here := f.status(id); here != nil {
			next, named := f.index[f.next(id, here.Gate)]
			c.Due = !named || next >= to
		}
	}
	f.conditions[id] = list
	return list
}
