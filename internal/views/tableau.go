package views

import (
	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// The words of a cell's kind.
const (
	cellEmpty  = "empty"
	cellStatus = "status"
	cellMarks  = "marks"
	cellExempt = "exempt"
)

// The labels of a row of the contextual tableau; a row of the focus has none.
const (
	labelSpine   = "spine"
	labelSibling = "sibling"
)

// Tableau returns the global tableau. It reads no q.Task.
func Tableau(f *derive.Facts, q Query) *view.Tableau {
	b := builder{f, q}
	all := f.Order()
	rows := all
	if q.Level < Detail {
		rows = append([]string{f.Root()}, f.Children(f.Root())...)
	}
	return b.grid(nil, all, all, rows, nil)
}

// Context returns the contextual tableau: of the subtree under q.Task, or,
// with q.Task empty, of q.Person. The focus is the subtree, or the person's
// own tasks; the detail level adds the spine and the siblings, which count
// in the columns and do not widen the window (decision 2).
func Context(f *derive.Facts, q Query) *view.Tableau {
	b := builder{f, q}
	var subject *view.TaskRef
	var focus, glance, anchors []string
	if q.Task != "" {
		ref := b.ref(q.Task)
		subject = &ref
		focus = b.subtree(q.Task)
		glance = append([]string{q.Task}, f.Children(q.Task)...)
		anchors = []string{q.Task}
	} else {
		focus = b.own(q.Person)
		glance, anchors = focus, focus
	}
	// A task keeps the first label that applies: of the focus, else spine,
	// else sibling.
	labels := map[string]string{}
	for _, id := range focus {
		labels[id] = ""
	}
	for _, id := range anchors {
		for _, a := range f.Authorities(id) {
			if _, labelled := labels[a.Task]; !labelled {
				labels[a.Task] = labelSpine
			}
		}
	}
	for _, id := range anchors {
		for _, child := range f.Children(b.parent(id)) {
			if _, labelled := labels[child]; !labelled {
				labels[child] = labelSibling
			}
		}
	}
	var detail []string
	for _, id := range f.Order() {
		if _, labelled := labels[id]; labelled {
			detail = append(detail, id)
		}
	}
	rows := detail
	if q.Level < Detail {
		rows = glance
	}
	return b.grid(subject, focus, detail, rows, labels)
}

// next returns a task's next gate: the next applicable gate after the gate
// its status names, a parent's by its roll-up, or "" at the last gate.
func (b builder) next(id string) string {
	s := b.f.Status(id)
	if s == nil {
		return ""
	}
	return b.f.Next(id, s.Gate)
}

// own returns a person's own tasks, in display order: each task the person
// is assigned, parents among them, and each leaf whose next junction is
// plain with the person as its contributor (decision 3).
func (b builder) own(person string) []string {
	var list []string
	if person == "" {
		return list
	}
	for _, id := range b.f.Order() {
		if b.assignee(id) == person {
			list = append(list, id)
			continue
		}
		if j := b.nextJunction(id); j != nil && j.Kind == model.Plain && j.Contributor.V == person {
			list = append(list, id)
		}
	}
	return list
}

// columns returns the columns of a tableau: every gate once, in order,
// alone or in a folded run. The window runs q.Window gates either side of
// the next gates of the focus, and is every gate when the focus has no next
// gate; q.Columns names the gates that show in its place. The counts are
// those of the tasks counted, each at the gate its status names.
func (b builder) columns(focus, counted []string) []view.Column {
	f := b.f
	gates := f.Gates()
	shown := make([]bool, len(gates))
	if len(b.q.Columns) > 0 {
		for _, gate := range b.q.Columns {
			if i := f.Index(gate); i >= 0 {
				shown[i] = true
			}
		}
	} else {
		lo, hi := -1, -1
		for _, id := range focus {
			i := f.Index(b.next(id))
			if i < 0 {
				continue
			}
			if lo < 0 || i < lo {
				lo = i
			}
			if i > hi {
				hi = i
			}
		}
		every, n := lo < 0, max(b.q.Window, 0)
		for i := range shown {
			shown[i] = every || i >= lo-n && i <= hi+n
		}
	}
	count := make([]int, len(gates))
	for _, id := range counted {
		if s := f.Status(id); s != nil {
			if i := f.Index(s.Gate); i >= 0 {
				count[i]++
			}
		}
	}
	out := []view.Column{}
	for i, gate := range gates {
		if last := len(out) - 1; !shown[i] && last >= 0 && out[last].Folded {
			c := &out[last]
			c.Gates, c.ByGate, c.Count = append(c.Gates, gate), append(c.ByGate, count[i]), c.Count+count[i]
			continue
		}
		out = append(out, view.Column{Gates: []string{gate}, Folded: !shown[i], Count: count[i], ByGate: []int{count[i]}})
	}
	return out
}

// cells returns the cells of a task, one per gate. The cell at the gate the
// status names shows the status. A cell before it is empty unless the view
// shows historical junctions, and undefined has no junction to show. Any
// other cell reads exempt where the gate does not apply, and else carries
// the marks of the junction: a leaf's own, and a parent's as its subtree
// inherits them (decision 4). The person in view acts on a leaf's plain
// junction after the status where the person contributes or reviews.
func (b builder) cells(id string) []view.Cell {
	f := b.f
	s := f.Status(id)
	at := -1
	if s != nil {
		at = f.Index(s.Gate)
	}
	leaf := f.Leaf(id)
	cells := []view.Cell{}
	for i, gate := range f.Gates() {
		c := view.Cell{Kind: cellEmpty, Historical: i < at, Marks: []string{}}
		j := f.Junction(id, gate)
		switch {
		case i == at:
			c.Kind, c.State, c.Reason = cellStatus, s.State, s.Reason
		case j == nil || i < at && !b.q.Historical:
		case j.Kind == model.NotApplicable:
			c.Kind = cellExempt
		default:
			c.Kind = cellMarks
			marks := j.Marks()
			if !leaf {
				marks = j.DefaultMarks()
			}
			for _, m := range marks {
				c.Marks = append(c.Marks, string(m))
			}
			if person := b.q.Person; person != "" && leaf && i > at && j.Kind == model.Plain {
				c.Acts = j.Contributor.V == person || j.Reviewer.V == person
			}
		}
		cells = append(cells, c)
	}
	return cells
}

// leaf names the task at the end of a status's chain, with the subproject
// it is of, or returns nil for a chain of one.
func (b builder) leaf(id string) *view.Leaf {
	steps := b.chain(id)
	if len(steps) < 2 {
		return nil
	}
	last := steps[len(steps)-1]
	return &view.Leaf{Task: last.Task, URL: last.URL}
}

// rowProvenance returns what a row adds at provenance: the fields of the
// same names of the task definition, for the row's task.
func (b builder) rowProvenance(id string) *view.RowProvenance {
	f := b.f
	p := &view.RowProvenance{
		StatusCommit: b.statusCommit(id),
		Chain:        b.chain(id),
		Junctions:    []view.Junction{},
		Sources:      []view.Supplier{},
		Linkages:     b.linkages(id),
	}
	if s := f.Status(id); s != nil {
		p.OwnCommit = b.commit(s.Own)
	}
	for _, j := range f.Junctions(id) {
		p.Junctions = append(p.Junctions, b.junction(j, true))
		p.Sources = append(p.Sources, b.suppliers(j)...)
	}
	return p
}

// grid returns a tableau. The focus fixes the window and the tasks counted
// fix the counts, at every level; rows are the rows of the level, in
// display order, and labels gives each its label.
func (b builder) grid(subject *view.TaskRef, focus, counted, rows []string, labels map[string]string) *view.Tableau {
	f := b.f
	out := &view.Tableau{Legend: b.legend(), Subject: subject, Columns: b.columns(focus, counted), Rows: []view.GridRow{}}
	shown := map[string]bool{}
	for _, id := range rows {
		shown[id] = true
	}
	// nearest returns the ancestors of a task that have a row, nearest first.
	nearest := func(id string) []string {
		var list []string
		for _, a := range f.Authorities(id) {
			if shown[a.Task] {
				list = append(list, a.Task)
			}
		}
		return list
	}
	hidden := map[string]int{}
	for _, id := range f.Order() {
		if above := nearest(id); !shown[id] && len(above) > 0 {
			hidden[above[0]]++
		}
	}
	for _, id := range rows {
		row := view.GridRow{
			TaskRef: b.ref(id), Under: b.parent(id), Depth: len(nearest(id)),
			Parent: len(f.Children(id)) > 0, Hidden: hidden[id], Label: labels[id],
			Status: b.status(f, f.Status(id)), Leaf: b.leaf(id), Cells: b.cells(id),
		}
		if b.q.Level >= Provenance {
			row.RowProvenance = b.rowProvenance(id)
		}
		out.Rows = append(out.Rows, row)
	}
	if b.q.Level >= Provenance {
		out.TableauProvenance = &view.TableauProvenance{Commands: []view.Command{}}
		if log := f.Log(); log != nil {
			out.Commands = append(out.Commands, view.Command{
				Text: "git log --format='%H %as %ae %ce %s' " + log.Source + " -- " + b.fromRoot("status"),
			})
		}
	}
	return out
}
