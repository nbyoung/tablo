package validate

import "github.com/nbyoung/tablo/internal/model"

// junction applies the rules of one entry under junctions. J2, J11 and J1
// each end the entry. An entry that is no mapping, or that mixes the fields
// of two kinds, is J4; any other validates alone against the schema of its
// kind. A recursive entry takes J3 and, with no J4 and no J7, the rules of
// its subproject.
func (r *run) junction(t *model.Task, j *model.Junction) {
	if r.j2(t, j) || r.j11(t, j) || r.j1(t, j) || r.j4(t, j) {
		return
	}
	kind := j.Kind()
	codes := r.leaves(checkEntry(t.File, t.ID, j, kind))
	if kind != model.Recursive {
		return
	}
	r.j3(t, j)
	if !codes["J4"] && !codes["J7"] {
		r.subproject(t, j.Gate, j.Subproject, codes)
	}
}

// subproject applies the rules of a subproject field, on a junction or, with
// no gate, on a requirement: J14, J15, J8, J16, J17 and J9, in that order. It
// returns the task the field reads in the linked project, and false where a
// rule ends the sequence.
func (r *run) subproject(t *model.Task, gate string, sub *model.Subproject, codes map[string]bool) (string, bool) {
	// J14 is the schema's. A url that is no scalar has no link, and the
	// schema reports it.
	if codes["J14"] || sub == nil || sub.Link == nil {
		return "", false
	}
	if r.j15(t, gate, sub) || r.j8(t, gate, sub) {
		return "", false
	}
	r.j16(t, gate, sub)
	r.j17(t, gate, sub)
	if r.j9(t, gate, sub) {
		return "", false
	}
	return target(sub, r.shapeOf(sub.Link.Project))
}

// target returns the task a subproject field reads in the linked project: its
// id, or that project's one root. ok is false when the project holds no such
// task.
func target(sub *model.Subproject, linked *shape) (id string, ok bool) {
	if sub.ID.Node != nil {
		return sub.ID.V, sub.ID.Node.Scalar() && linked.project.Tasks[sub.ID.V] != nil
	}
	if len(linked.roots) == 1 && len(linked.unread) == 0 {
		return linked.roots[0], true
	}
	return "", false
}

// isExemption reports whether an entry is { applies: false } and no more.
func isExemption(j *model.Junction) bool {
	kind, ok := decides(j)
	return ok && kind == notApplicable
}

// j1 reports a junction key that is no gate key, or that names no gate.
func (r *run) j1(t *model.Task, j *model.Junction) bool {
	switch _, named := r.shape.gate[j.Gate]; {
	case !keyPattern.MatchString(j.Gate):
		r.add("J1", j.KeyPos, t.ID, j.Gate, "junction key %s is no gate key", j.Gate)
	case r.project.Gating != nil && !named:
		r.add("J1", j.KeyPos, t.ID, j.Gate, "junction key %s names no gate in gates.yaml", j.Gate)
	default:
		return false
	}
	return true
}

// j2 reports a not-applicable entry at undefined.
func (r *run) j2(t *model.Task, j *model.Junction) bool {
	if j.Gate != undefined || !isExemption(j) {
		return false
	}
	r.add("J2", j.KeyPos, t.ID, j.Gate, "a not-applicable entry at undefined: the undefined gate always applies")
	return true
}

// j3 reports a recursive junction on a task that another names as its parent.
func (r *run) j3(t *model.Task, j *model.Junction) {
	if r.shape.isParent(t.ID) {
		r.add("J3", j.KeyPos, t.ID, j.Gate, "a parent states a recursive junction")
	}
}

// j4 reports an entry that is no mapping, or that mixes the fields of more
// than one kind. The schema reports an unknown field beside subproject.
func (r *run) j4(t *model.Task, j *model.Junction) bool {
	switch {
	case j.Node == nil || j.Node.Kind != model.Map:
		r.add("J4", posOf(t.File, j.Node), t.ID, j.Gate, "the entry is no mapping, so it is none of the three kinds")
	case j.Kind() == model.Mixed:
		r.add("J4", j.KeyPos, t.ID, j.Gate, "the entry mixes the fields of more than one kind")
	default:
		return false
	}
	return true
}

// j8 reports a subproject the tool cannot read: a link that does not resolve,
// or a project at a version the module does not accept.
func (r *run) j8(t *model.Task, gate string, sub *model.Subproject) bool {
	link := sub.Link
	if link.Problem != model.Resolved || link.Project == nil {
		r.add("J8", posOf(t.File, sub.URL.Node, sub.Node), t.ID, gate,
			"url %s resolves to no Tableaux project the tool can read: %s", sub.URL.V, link.Problem)
		return true
	}
	if v := link.Project.Version; v != nil && v.WellFormed && !v.Accepted {
		r.add("J8", posOf(t.File, sub.URL.Node, sub.Node), t.ID, gate,
			"url %s resolves to a project at tableaux %s, which the tool does not read", sub.URL.V, v.Tableaux.V)
		return true
	}
	return false
}

// j9 reports a subproject whose task, the id or that project's one root, is
// no task there. It stays silent about an id that is unread there.
func (r *run) j9(t *model.Task, gate string, sub *model.Subproject) bool {
	linked := r.shapeOf(sub.Link.Project)
	if _, ok := target(sub, linked); ok {
		return false
	}
	switch {
	case sub.ID.Node == nil:
		if len(linked.unread) == 0 {
			r.add("J9", posOf(t.File, sub.Node), t.ID, gate, "the project at %s has no one root task", sub.Link.URL)
		}
	case sub.ID.Node.Scalar() && !linked.unread[sub.ID.V]:
		r.add("J9", sub.ID.Node.Pos, t.ID, gate, "id %s names no task in the project at %s", sub.ID.V, sub.Link.URL)
	}
	return true
}

// j11 reports any other entry at undefined.
func (r *run) j11(t *model.Task, j *model.Junction) bool {
	if j.Gate != undefined {
		return false
	}
	r.add("J11", j.KeyPos, t.ID, j.Gate, "an entry at undefined: the undefined gate has no work of its own")
	return true
}

// j12 reports a task to which no gate but undefined applies.
func (r *run) j12(t *model.Task) {
	if r.project.Gating == nil || len(r.shape.gates) == 0 {
		return
	}
	for _, gate := range r.shape.applicable(t.ID) {
		if gate != undefined {
			return
		}
	}
	r.add("J12", model.Pos{File: t.File.Path}, t.ID, "", "no gate after undefined applies to %s", t.ID)
}

// j15 reports an absolute URL that states no commit.
func (r *run) j15(t *model.Task, gate string, sub *model.Subproject) bool {
	if sub.Link.Form != model.URL || sub.Commit.Node != nil {
		return false
	}
	r.add("J15", posOf(t.File, sub.Node), t.ID, gate, "the absolute url %s states no commit", sub.URL.V)
	return true
}

// j16 reports a commit on a directory of the same repository.
func (r *run) j16(t *model.Task, gate string, sub *model.Subproject) {
	if sub.Link.Form == model.Directory && sub.Commit.Node != nil {
		r.add("J16", sub.Commit.Node.Pos, t.ID, gate, "url %s is a directory of this repository, which nothing pins", sub.URL.V)
	}
}

// j17 reports a commit on a submodule path that differs from the pin.
func (r *run) j17(t *model.Task, gate string, sub *model.Subproject) {
	if sub.Link.Form == model.Submodule && sub.Commit.Node.Scalar() && sub.Commit.V != sub.Link.Commit {
		r.add("J17", sub.Commit.Node.Pos, t.ID, gate,
			"commit %s is not the commit the submodule %s pins, %s", sub.Commit.V, sub.URL.V, sub.Link.Commit)
	}
}
