package views

import (
	"path"
	"strings"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// place is one plain junction of a leaf in view, as an email holds it.
type place struct {
	j   *derive.Junction
	row view.JunctionRow
}

// positions holds what one email holds among the tasks in view.
type positions struct {
	email       string
	assigned    []string // the tasks in view it is the assignee of, in display order
	contributes []place  // gate order, then display order
	reviews     []place
}

// Assignment returns the task assignment of the subtree under q.Task.
func Assignment(f *derive.Facts, q Query) *view.Assignment {
	b := builder{f, q}
	tasks := b.subtree(b.top())
	out := &view.Assignment{Legend: b.legend(), People: []view.PersonRow{}, Recursive: []string{}}
	for _, id := range tasks {
		if j := b.nextJunction(id); j != nil && j.Kind == model.Recursive {
			out.Recursive = append(out.Recursive, id)
		}
	}
	people := b.people(tasks)
	for _, p := range people {
		row := view.PersonRow{Email: p.email, Owner: p.email == f.Owner(), Assigned: len(p.assigned), Models: []string{}}
		for _, c := range p.contributes {
			if c.row.Stands == standsNext {
				row.ContributesNext++
			}
			if m := c.row.Model; m != "" && !has(row.Models, m) {
				row.Models = append(row.Models, m)
			}
		}
		for _, r := range p.reviews {
			if r.row.Stands == standsNext {
				row.ReviewsNext++
			}
		}
		out.People = append(out.People, row)
	}
	if q.Level < Detail {
		return out
	}
	out.AssignmentDetail = &view.AssignmentDetail{Sections: []view.PersonSection{}}
	inView := map[string]bool{}
	for _, id := range tasks {
		inView[id] = true
	}
	for _, p := range people {
		out.Sections = append(out.Sections, b.section(p, inView))
	}
	if q.Level < Provenance {
		return out
	}
	out.AssignmentProvenance = &view.AssignmentProvenance{Commands: b.assignmentCommands(out.Sections)}
	return out
}

// assignmentCommands returns the commands of the assignment's provenance,
// over the source commit: one git show per task file a position's source
// names, once, in the order first named; one git grep of the assignees of
// the tasks directory; and, where a section holds a recursive row, one git
// ls-tree over each submodule path once. It is [] without a history.
func (b builder) assignmentCommands(sections []view.PersonSection) []view.Command {
	out := []view.Command{}
	log := b.f.Log()
	if log == nil {
		return out
	}
	shown := map[string]bool{}
	for _, s := range sections {
		if s.SectionProvenance == nil {
			continue
		}
		for _, p := range s.Positions {
			for _, from := range []view.Stated{p.From, p.ReviewerFrom} {
				if from.File == "" || shown[from.By] {
					continue
				}
				shown[from.By] = true
				out = append(out, view.Command{Text: "git show " + log.Source + ":" + b.fromRoot(path.Join("tasks", from.By+".yaml"))})
			}
		}
	}
	out = append(out, view.Command{Text: "git grep -n '^assignee:' " + log.Source + " -- " + b.fromRoot("tasks")})
	var paths []string
	named := map[string]bool{}
	for _, s := range sections {
		for _, r := range s.Recursive {
			if url := r.Subproject.URL; r.Subproject.Form == model.Submodule.String() && !named[url] {
				named[url] = true
				paths = append(paths, url)
			}
		}
	}
	if len(paths) > 0 {
		out = append(out, view.Command{Text: "git ls-tree " + log.Source + " " + strings.Join(paths, " ")})
	}
	return out
}

// has reports whether a list holds a string.
func has(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// nextJunction returns the junction of a leaf at its next gate, the next
// applicable gate after the gate its status names, or nil for a parent and
// at the last gate.
func (b builder) nextJunction(id string) *derive.Junction {
	if !b.f.Leaf(id) {
		return nil
	}
	s := b.f.Status(id)
	if s == nil {
		return nil
	}
	return b.f.Junction(id, b.f.Next(id, s.Gate))
}

// people returns what each email in view holds, in the order of the emails:
// the assignees of the tasks in view, in display order; then every other
// contributor and reviewer of a plain junction of a leaf in view, by task in
// display order, then gate, the contributor before the reviewer. With a
// person it returns that email alone, and nothing when it holds no position.
func (b builder) people(tasks []string) []*positions {
	f := b.f
	var list []*positions
	byEmail := map[string]*positions{}
	of := func(email string) *positions {
		if email == "" {
			return nil
		}
		p := byEmail[email]
		if p == nil {
			p = &positions{email: email}
			byEmail[email] = p
			list = append(list, p)
		}
		return p
	}
	for _, id := range tasks {
		if p := of(b.assignee(id)); p != nil {
			p.assigned = append(p.assigned, id)
		}
	}
	// A parent's junctions are defaults: the positions count on leaves alone.
	var leaves []string
	for _, id := range tasks {
		if !f.Leaf(id) {
			continue
		}
		leaves = append(leaves, id)
		for _, j := range f.Junctions(id) {
			if j.Kind == model.Plain {
				of(j.Contributor.V)
				of(j.Reviewer.V)
			}
		}
	}
	for _, gate := range f.Gates() {
		for _, id := range leaves {
			j := f.Junction(id, gate)
			if j == nil || j.Kind != model.Plain {
				continue
			}
			at := place{j: j, row: view.JunctionRow{Task: b.ref(id), Junction: b.junction(j, false)}}
			if p := of(j.Contributor.V); p != nil {
				p.contributes = append(p.contributes, at)
			}
			if p := of(j.Reviewer.V); p != nil {
				p.reviews = append(p.reviews, at)
			}
		}
	}
	if b.q.Person == "" {
		return list
	}
	if p := byEmail[b.q.Person]; p != nil {
		return []*positions{p}
	}
	return nil
}

// section returns what one email carries at detail and, at provenance, its
// positions. inView holds the tasks in view.
func (b builder) section(p *positions, inView map[string]bool) view.PersonSection {
	f := b.f
	s := view.PersonSection{
		Email:    p.email,
		Assigned: []view.AssignedRow{}, Recursive: []view.RecursiveRow{},
		Contributes: []view.JunctionRow{}, Reviews: []view.JunctionRow{},
		Models: []view.ModelRow{}, Counts: []view.GateCount{}, Authority: []view.Subtree{},
	}
	counts := map[string]*view.GateCount{}
	for _, gate := range f.Gates() {
		s.Counts = append(s.Counts, view.GateCount{Gate: gate})
	}
	for i := range s.Counts {
		counts[s.Counts[i].Gate] = &s.Counts[i]
	}
	var recursive []*derive.Junction
	for _, id := range p.assigned {
		status := f.Status(id)
		row := view.AssignedRow{TaskRef: b.ref(id), Under: b.parent(id), Status: b.status(f, status)}
		if status != nil && counts[status.Gate] != nil {
			counts[status.Gate].Standing++
		}
		if j := b.nextJunction(id); j != nil {
			next := b.junction(j, false)
			row.Next, row.NextGate = &next, j.Gate
			if j.Kind == model.Recursive {
				recursive = append(recursive, j)
				s.Recursive = append(s.Recursive, b.recursive(j))
			}
		}
		s.Assigned = append(s.Assigned, row)
		if len(f.Children(id)) > 0 && !b.under(id, p.email, inView) {
			s.Authority = append(s.Authority, view.Subtree{Task: b.ref(id), Descendants: len(b.subtree(id)) - 1})
		}
	}
	for _, c := range p.contributes {
		s.Contributes = append(s.Contributes, c.row)
		count, next := counts[c.row.Gate], c.row.Stands == standsNext
		count.Contributes++
		if next {
			count.ContributesNext++
		}
		if c.row.Model == "" {
			continue
		}
		var row *view.ModelRow
		for i := range s.Models {
			if m := &s.Models[i]; m.Gate == c.row.Gate && m.Model == c.row.Model && m.StatedBy == c.j.Model.Task {
				row = m
			}
		}
		if row == nil {
			s.Models = append(s.Models, view.ModelRow{Gate: c.row.Gate, Model: c.row.Model, StatedBy: c.j.Model.Task})
			row = &s.Models[len(s.Models)-1]
		}
		row.Junctions++
		if next {
			row.Next++
		}
	}
	for _, r := range p.reviews {
		s.Reviews = append(s.Reviews, r.row)
		counts[r.row.Gate].Reviews++
		if r.row.Stands == standsNext {
			counts[r.row.Gate].ReviewsNext++
		}
	}
	if b.q.Level >= Provenance {
		s.SectionProvenance = &view.SectionProvenance{Positions: b.positions(p, s.Authority, recursive)}
	}
	return s
}

// under reports whether an ancestor of a task, among the tasks in view, is
// assigned to an email.
func (b builder) under(id, email string, inView map[string]bool) bool {
	for _, a := range b.f.Authorities(id) {
		if inView[a.Task] && a.Email == email {
			return true
		}
	}
	return false
}

// recursive returns the row of an assigned leaf whose next junction a
// subproject carries, with the status of the task read there.
func (b builder) recursive(j *derive.Junction) view.RecursiveRow {
	row := view.RecursiveRow{Task: b.ref(j.Task), Gate: j.Gate}
	n := j.Snapshot
	if ref := b.subRef(n); ref != nil {
		row.Subproject = *ref
	}
	if n != nil && n.Facts != nil {
		if s := n.Facts.Status(n.Target); s != nil {
			status := b.status(n.Facts, s)
			row.Status = &status
		}
	}
	return row
}

// positions groups what an email holds by the source that states it: its
// assignments; its authority; its contributions, by gate, the contributor's
// source, the reviewer and the reviewer's source; its reviews, by gate, the
// reviewer's source and the contributor's; and its recursive junctions.
// Within a kind the order is gate order, then first occurrence.
func (b builder) positions(p *positions, authority []view.Subtree, recursive []*derive.Junction) []view.Position {
	out := []view.Position{}
	if len(p.assigned) > 0 {
		out = append(out, view.Position{
			Kind: "assignee", Tasks: append([]string{}, p.assigned...),
			From: view.Stated{By: "own", Key: "assignee"},
		})
	}
	for _, a := range authority {
		from := view.Stated{By: a.Task.ID, Key: "assignee", File: taskFile(a.Task.ID)}
		if task := b.f.Project().Tasks[a.Task.ID]; task != nil && task.Assignee.Node != nil {
			from.Line, from.Col = task.Assignee.Node.Pos.Line, task.Assignee.Node.Pos.Col
		}
		out = append(out, view.Position{Kind: "authority", Tasks: []string{a.Task.ID}, From: from})
	}
	// group adds a junction's task to the position of its kind that shares
	// its sources, or starts one. The places come in gate order.
	group := func(kind string, at place, reviewer string) {
		from := statedField(at.j.Gate, "contributor", at.j.Contributor)
		reviewerFrom := statedField(at.j.Gate, "reviewer", at.j.Reviewer)
		for i := range out {
			if g := &out[i]; g.Kind == kind && g.Gate == at.j.Gate && g.From == from && g.Reviewer == reviewer && g.ReviewerFrom == reviewerFrom {
				g.Tasks = append(g.Tasks, at.j.Task)
				return
			}
		}
		out = append(out, view.Position{Kind: kind, Gate: at.j.Gate, Tasks: []string{at.j.Task}, Reviewer: reviewer, From: from, ReviewerFrom: reviewerFrom})
	}
	for _, c := range p.contributes {
		group("contributes", c, c.j.Reviewer.V)
	}
	for _, r := range p.reviews {
		group("reviews", r, "")
	}
	for _, j := range recursive {
		row := view.Position{Kind: "recursive", Gate: j.Gate, Tasks: []string{j.Task}, From: b.statedEntry(j.Entry, j.Gate, "subproject")}
		if ref := b.subRef(j.Snapshot); ref != nil {
			row.URL, row.Pin = ref.URL, ref.Commit
		}
		out = append(out, row)
	}
	return out
}
