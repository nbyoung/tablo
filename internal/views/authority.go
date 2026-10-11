package views

import (
	"path"
	"sort"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/view"
)

// Authority returns the authority delegation of the subtree under q.Task.
func Authority(f *derive.Facts, q Query) *view.Delegation {
	b := builder{f, q}
	top := b.top()
	shown := b.delegated(top)
	out := &view.Delegation{Legend: b.legend(), Rows: []view.DelegationRow{}}
	depth := map[string]int{top: 0}
	for _, id := range b.subtree(top) {
		if id != top {
			depth[id] = depth[b.parent(id)] + 1
		}
		if !shown[id] {
			continue
		}
		row := view.DelegationRow{TaskRef: b.ref(id), Depth: depth[id], Assignee: b.assignee(id), Parent: len(f.Children(id)) > 0, Proposed: true}
		if parent := b.parent(id); parent != "" {
			row.Delegated = row.Assignee != b.assignee(parent)
		}
		if a := f.Authorisation(id); a != nil {
			row.Proposed, row.DiffersFromTrunk, row.Uncommitted = !a.Authorised, a.Differs, a.Uncommitted
		}
		out.Rows = append(out.Rows, row)
	}
	if q.Level < Detail {
		return out
	}
	out.DelegationDetail = &view.DelegationDetail{Parents: []view.Chain{}}
	for _, row := range out.Rows {
		if row.Parent {
			out.Parents = append(out.Parents, b.delegation(row.ID))
		}
	}
	if q.Level < Provenance {
		return out
	}
	out.DelegationProvenance = b.deciding(out.Rows)
	return out
}

// top returns the task a tree view starts at: the query's, or the root.
func (b builder) top() string {
	if b.q.Task != "" {
		return b.q.Task
	}
	return b.f.Root()
}

// assignee returns the assignee of a task of the project in view.
func (b builder) assignee(id string) string {
	if task := b.f.Project().Tasks[id]; task != nil {
		return task.Assignee.V
	}
	return ""
}

// delegated returns the tasks of the subtree under top that the authority
// delegation shows. With a person, a task stays when the person is its
// assignee or one of its authorities; with the proposed filter, of those,
// the proposed stay. Each filter then keeps every ancestor of a task it
// keeps, within the subtree, so the rows stay a tree.
func (b builder) delegated(top string) map[string]bool {
	tasks := b.subtree(top)
	shown := map[string]bool{}
	for _, id := range tasks {
		shown[id] = true
	}
	// keep narrows the tasks shown to those a test passes, and their ancestors.
	keep := func(passes func(id string) bool) {
		kept := map[string]bool{}
		for _, id := range tasks {
			if !shown[id] || !passes(id) {
				continue
			}
			for at := id; ; at = b.parent(at) {
				kept[at] = true
				if at == top {
					break
				}
			}
		}
		shown = kept
	}
	if person := b.q.Person; person != "" {
		keep(func(id string) bool {
			if b.assignee(id) == person {
				return true
			}
			for _, a := range b.f.Authorities(id) {
				if a.Email == person {
					return true
				}
			}
			return false
		})
	}
	if b.q.Proposed {
		keep(func(id string) bool {
			a := b.f.Authorisation(id)
			return a == nil || !a.Authorised
		})
	}
	return shown
}

// delegation returns what one parent gives its subtree: the chain of
// authorities its children have, its children, and its defaults, which are
// its resolved junctions at the gates where its own file states an entry.
func (b builder) delegation(id string) view.Chain {
	f := b.f
	out := view.Chain{
		TaskRef: b.ref(id), Assignee: b.assignee(id),
		Authorities: []view.Authority{}, Children: append([]string{}, f.Children(id)...),
		Defaults: []view.Junction{}, Sources: []view.Supplier{},
	}
	if len(out.Children) > 0 {
		for _, a := range f.Authorities(out.Children[0]) {
			out.Authorities = append(out.Authorities, view.Authority{Email: a.Email, By: a.Task})
		}
	}
	stated := map[string]bool{}
	if task := f.Project().Tasks[id]; task != nil {
		for _, e := range task.Junctions {
			stated[e.Gate] = true
		}
	}
	for _, j := range f.Junctions(id) {
		if !stated[j.Gate] {
			continue
		}
		junction := b.junction(j, true)
		junction.Marks = defaultMarks(j)
		out.Defaults = append(out.Defaults, junction)
		out.Sources = append(out.Sources, b.suppliers(j)...)
	}
	return out
}

// defaultMarks returns the marks of a parent's junction as its subtree
// inherits it: the junction's marks, without the reviewer where no entry
// states one and the parent's own assignee takes the review, since a child
// takes its own assignee and inherits no review from its parent's (design
// 27a3, decision 12). It is never nil.
func defaultMarks(j *derive.Junction) []string {
	out := []string{}
	for _, m := range j.Marks() {
		if m == derive.MarkReviewer && j.Reviewer.By == derive.ByAssignee {
			continue
		}
		out = append(out, string(m))
	}
	return out
}

// deciding returns what the authority delegation adds at provenance: the
// deciding commits of the rows, the rows no commit decides, and the command
// that lists the trunk's commits to the task files.
func (b builder) deciding(rows []view.DelegationRow) *view.DelegationProvenance {
	f := b.f
	out := &view.DelegationProvenance{Deciding: []view.Deciding{}, Undecided: []view.Undecided{}, Commands: []view.Command{}}
	type group struct {
		view.Deciding
		seq, way int
	}
	var groups []*group
	for _, row := range rows {
		a := f.Authorisation(row.ID)
		if a == nil {
			continue
		}
		if a.Commit == nil {
			reason, found := why(a.Why), false
			for i := range out.Undecided {
				if out.Undecided[i].Why == reason {
					out.Undecided[i].Tasks, found = append(out.Undecided[i].Tasks, row.ID), true
				}
			}
			if !found {
				out.Undecided = append(out.Undecided, view.Undecided{Why: reason, Tasks: []string{row.ID}})
			}
			continue
		}
		word, by := way(a.Way), hand(a)
		var g *group
		for _, other := range groups {
			if other.Commit.Hash == a.Commit.ID && other.Way == word && other.By == by {
				g = other
			}
		}
		if g == nil {
			g = &group{Deciding: view.Deciding{Commit: *b.commit(a.Commit), Way: word, By: by, Email: a.By, Tasks: []string{}}, seq: a.Commit.Seq, way: int(a.Way)}
			if a.Way == derive.ByMerge && len(a.Commit.Parents) > 1 {
				g.Merged = a.Commit.Parents[1]
			}
			groups = append(groups, g)
		}
		g.Tasks = append(g.Tasks, row.ID)
	}
	// Newest first by the commit's place in the pass; then change, merge,
	// trailer; then the order the groups first occur.
	sort.SliceStable(groups, func(i, j int) bool {
		if groups[i].seq != groups[j].seq {
			return groups[i].seq < groups[j].seq
		}
		return groups[i].way < groups[j].way
	})
	for _, g := range groups {
		out.Deciding = append(out.Deciding, g.Deciding)
	}
	if tip := f.Trunk().Tip; tip != "" {
		out.Commands = append(out.Commands, view.Command{
			Text: "git log --first-parent --format='%H %as %ae %ce %P' " + tip + " -- " + path.Join(f.Project().Where.Dir, planDir, "tasks"),
		})
	}
	return out
}
