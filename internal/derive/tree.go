package derive

import (
	"sort"

	"github.com/nbyoung/tablo/internal/model"
)

// grow builds the tree in display order: siblings by order, ascending, then
// by id, and those without an order after those with one.
func (f *Facts) grow() {
	for _, id := range f.p.TaskIDs() {
		task := f.p.Tasks[id]
		if task.Parent == nil {
			f.root = id
			continue
		}
		f.parent[id] = task.Parent.ID.V
		f.children[task.Parent.ID.V] = append(f.children[task.Parent.ID.V], id)
	}
	for _, siblings := range f.children {
		sort.SliceStable(siblings, func(i, j int) bool {
			a, b := f.p.Tasks[siblings[i]].Parent.Order, f.p.Tasks[siblings[j]].Parent.Order
			switch {
			case a.OK != b.OK:
				return a.OK
			case a.OK && a.V != b.V:
				return a.V < b.V
			}
			return siblings[i] < siblings[j]
		})
	}
	var walk func(id string)
	walk = func(id string) {
		f.place[id] = len(f.order)
		f.order = append(f.order, id)
		for _, child := range f.children[id] {
			walk(child)
		}
	}
	walk(f.root)
}

// task returns a task of the tree, or nil. After a refusal no task is one.
func (f *Facts) task(id string) *model.Task {
	if _, ok := f.place[id]; !ok {
		return nil
	}
	return f.p.Tasks[id]
}

// above returns the parent of a task of the tree, or nil for the root.
func (f *Facts) above(task *model.Task) *model.Task {
	if id, ok := f.parent[task.ID]; ok {
		return f.p.Tasks[id]
	}
	return nil
}

// Root returns the id of the root task.
func (f *Facts) Root() string {
	if f == nil {
		return ""
	}
	return f.root
}

// Owner returns the project's owner: the root task's assignee.
func (f *Facts) Owner() string {
	if f == nil || f.refused != nil {
		return ""
	}
	return f.p.Tasks[f.root].Assignee.V
}

// Order returns every task, depth first, siblings in display order.
func (f *Facts) Order() []string {
	if f == nil {
		return nil
	}
	return append([]string(nil), f.order...)
}

// Children returns the children of a task in display order.
func (f *Facts) Children(id string) []string {
	if f == nil {
		return nil
	}
	return append([]string(nil), f.children[id]...)
}

// Leaf reports whether the task is one of the tree and has no child.
func (f *Facts) Leaf(id string) bool {
	return f != nil && f.task(id) != nil && len(f.children[id]) == 0
}

// Authority is one authority of a task and the ancestor that makes it one.
type Authority struct{ Email, Task string }

// Authorities returns the authorities of a task, nearest first: the
// assignees of its ancestors. The root has none in the tree; its authority
// is the owner.
func (f *Facts) Authorities(id string) []Authority {
	if f == nil || f.task(id) == nil {
		return nil
	}
	var list []Authority
	for at, ok := f.parent[id]; ok; at, ok = f.parent[at] {
		list = append(list, Authority{Email: f.p.Tasks[at].Assignee.V, Task: at})
	}
	return list
}

// Gates returns the keys of gates.yaml, in order.
func (f *Facts) Gates() []string {
	if f == nil {
		return nil
	}
	return append([]string(nil), f.gates...)
}

// Index returns the place of a gate in gates.yaml, or -1 for a key it lacks.
func (f *Facts) Index(gate string) int {
	if f == nil {
		return -1
	}
	if i, ok := f.index[gate]; ok {
		return i
	}
	return -1
}

// Severity returns the severity of a state, or 0 for a key gates.yaml lacks.
func (f *Facts) Severity(state string) int {
	if f == nil {
		return 0
	}
	return f.severity[state]
}

// Role is a role of README.md#roles.
type Role string

// The roles, in README.md's order.
const (
	Owner       Role = "owner"
	AuthorityOf Role = "authority"
	Assignee    Role = "assignee"
	Contributor Role = "contributor"
	Agent       Role = "agent"
	Reviewer    Role = "reviewer"
	Observer    Role = "observer"
)

// Roles returns the roles the email holds anywhere in the project, in
// README.md's order, or Observer alone: owner; authority for the assignee
// of a task with children; assignee; contributor and reviewer at any
// junction of a leaf, since a parent's junctions are defaults; and agent
// beside contributor where the junction states a model.
func (f *Facts) Roles(email string) []Role {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	held := map[Role]bool{}
	if email != "" {
		held[Owner] = f.p.Tasks[f.root].Assignee.V == email
		for _, id := range f.order {
			if f.p.Tasks[id].Assignee.V == email {
				held[Assignee] = true
				held[AuthorityOf] = held[AuthorityOf] || len(f.children[id]) > 0
			}
			if len(f.children[id]) > 0 {
				continue
			}
			for _, j := range f.resolved(id) {
				if j.Kind != model.Plain {
					continue
				}
				if j.Contributor.V == email {
					held[Contributor] = true
					held[Agent] = held[Agent] || j.Model.V != ""
				}
				held[Reviewer] = held[Reviewer] || j.Reviewer.V == email
			}
		}
	}
	var roles []Role
	for _, role := range []Role{Owner, AuthorityOf, Assignee, Contributor, Agent, Reviewer} {
		if held[role] {
			roles = append(roles, role)
		}
	}
	if roles == nil {
		roles = []Role{Observer}
	}
	return roles
}
