package validate

import "github.com/nbyoung/tablo/internal/model"

// edge is one local requirement that passes R1 and R3 to R5: the terminating
// task requires the originating one.
type edge struct {
	from, to string
	pos      model.Pos // of the entry's id
}

// requirement applies the rules of one entry under requires and stops at the
// first that ends the sequence. codes holds the rules the schema raises under
// the entry. An entry that is no mapping, or that states both or neither of
// id and subproject, has its schema row and nothing more.
func (r *run) requirement(t *model.Task, q *model.Requirement, codes map[string]bool) {
	local, cross := q.ID.Node != nil, q.Subproject != nil
	if q.Node == nil || q.Node.Kind != model.Map || local == cross {
		return
	}
	// origin is the shape of the originating project, and task the
	// originating task there: "" for one that is unread.
	origin, task := r.shape, ""
	if local {
		if !q.ID.Node.Scalar() {
			return
		}
		if r.r1(t, q) || r.r3(t, q) || r.r4(t, q) || r.r5(t, q) {
			return
		}
		if r.project.Tasks[q.ID.V] != nil {
			task = q.ID.V
			r.edges = append(r.edges, edge{from: t.ID, to: task, pos: q.ID.Node.Pos})
		}
	} else {
		if codes["R11"] {
			return
		}
		id, ok := r.subproject(t, "", q.Subproject, codes)
		if !ok {
			return
		}
		origin, task = r.shapeOf(q.Subproject.Link.Project), id
		r.r12(t, q, task)
	}
	r.r6(t, q, origin, task)
	r.r10(t, q)
	r.r7(t, q)
}

// r1 reports a requirement on no task. It stays silent about an unread id.
func (r *run) r1(t *model.Task, q *model.Requirement) bool {
	id := q.ID.V
	if r.project.Tasks[id] != nil || r.shape.unread[id] {
		return false
	}
	r.add("R1", q.ID.Node.Pos, t.ID, "", "requires %s, which names no task", id)
	return true
}

// r2 reports each requirement whose edge lies on a cycle, over the local
// entries that pass R1 and R3 to R5.
func (r *run) r2() {
	next := map[string][]string{}
	for _, e := range r.edges {
		next[e.from] = append(next[e.from], e.to)
	}
	reaches := func(from, to string) bool {
		met := map[string]bool{from: true}
		for queue := []string{from}; len(queue) > 0; queue = queue[1:] {
			if queue[0] == to {
				return true
			}
			for _, id := range next[queue[0]] {
				if !met[id] {
					met[id] = true
					queue = append(queue, id)
				}
			}
		}
		return false
	}
	for _, e := range r.edges {
		if reaches(e.to, e.from) {
			r.add("R2", e.pos, e.from, "", "the requirement on %s closes a cycle", e.to)
		}
	}
}

// r3 reports a requirement on the task itself.
func (r *run) r3(t *model.Task, q *model.Requirement) bool {
	if q.ID.V != t.ID {
		return false
	}
	r.add("R3", q.ID.Node.Pos, t.ID, "", "%s requires itself", t.ID)
	return true
}

// r4 reports a requirement on an ancestor.
func (r *run) r4(t *model.Task, q *model.Requirement) bool {
	if above, _ := r.shape.ancestors(t.ID); indexOf(above, q.ID.V) < 0 {
		return false
	}
	r.add("R4", q.ID.Node.Pos, t.ID, "", "%s requires its ancestor %s", t.ID, q.ID.V)
	return true
}

// r5 reports a requirement on a descendant.
func (r *run) r5(t *model.Task, q *model.Requirement) bool {
	if above, _ := r.shape.ancestors(q.ID.V); indexOf(above, t.ID) < 0 {
		return false
	}
	r.add("R5", q.ID.Node.Pos, t.ID, "", "%s requires its descendant %s", t.ID, q.ID.V)
	return true
}

// r6 reports a from gate that names no gate of the originating project, or
// one that does not apply to the originating task. The schema reports a from
// that is no key.
func (r *run) r6(t *model.Task, q *model.Requirement, origin *shape, task string) {
	from := q.From
	if r.project.Gating == nil || origin.project.Gating == nil || !from.Node.Scalar() || !keyPattern.MatchString(from.V) {
		return
	}
	switch _, named := origin.gate[from.V]; {
	case !named:
		r.add("R6", from.Node.Pos, t.ID, from.V, "from %s names no gate in the originating project's gates.yaml", from.V)
	case task != "" && !origin.appliesTo(task, from.V):
		r.add("R6", from.Node.Pos, t.ID, from.V, "from %s does not apply to %s", from.V, task)
	}
}

// r7 reports a to gate that names no gate, or one that does not apply to the
// task. The schema reports a to that is no key, and R10 one that is undefined.
func (r *run) r7(t *model.Task, q *model.Requirement) {
	to := q.To
	if r.project.Gating == nil || !to.Node.Scalar() || !keyPattern.MatchString(to.V) || to.V == undefined {
		return
	}
	switch _, named := r.shape.gate[to.V]; {
	case !named:
		r.add("R7", to.Node.Pos, t.ID, to.V, "to %s names no gate in gates.yaml", to.V)
	case !r.shape.appliesTo(t.ID, to.V):
		r.add("R7", to.Node.Pos, t.ID, to.V, "to %s does not apply to %s", to.V, t.ID)
	}
}

// r10 reports a to gate of undefined.
func (r *run) r10(t *model.Task, q *model.Requirement) {
	if to := q.To; r.project.Gating != nil && to.Node.Scalar() && to.V == undefined {
		r.add("R10", to.Node.Pos, t.ID, to.V, "to is undefined: no work needs a result before definition")
	}
}

// r12 warns of a cross-project requirement on the task that a recursive
// junction of the same task already reads: the same link, as the Loader
// shares it, and the same task there.
func (r *run) r12(t *model.Task, q *model.Requirement, task string) {
	link := q.Subproject.Link
	for _, j := range t.Junctions {
		if j.Kind() != model.Recursive || j.Subproject == nil || j.Subproject.Link != link {
			continue
		}
		if read, ok := target(j.Subproject, r.shapeOf(link.Project)); ok && read == task {
			r.add("R12", q.Subproject.Node.Pos, t.ID, "", "the requirement on %s at %s names the task the %s junction reads", task, link.URL, j.Gate)
		}
	}
}
