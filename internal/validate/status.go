package validate

import "github.com/nbyoung/tablo/internal/model"

// complete is the key of the state of a task at its last applicable gate.
const complete = "complete"

// status applies the rules of one status file and stops at the first that
// ends the sequence.
func (r *run) status(st *model.Status) {
	if r.s1(st) {
		return
	}
	r.s2(st)
	codes := r.leaves(checkFile(statusFile, st.File, st.ID))
	// S3 and S4 are the schema's. The rules below name a gate, so they need
	// one in the file and a gates file to find it in.
	if !st.Gate.Node.Scalar() || r.project.Gating == nil {
		return
	}
	if r.s5(st, codes) {
		return
	}
	r.s6(st)
	if r.s7(st) {
		return
	}
	r.s8(st)
	r.s12(st)
	r.s9(st)
	r.s10(st)
}

// last returns the last gate that applies to a task, or "".
func (r *run) last(id string) string {
	if gates := r.shape.applicable(id); len(gates) > 0 {
		return gates[len(gates)-1]
	}
	return ""
}

// next returns the gate that applies to a task after the one given, and
// false when the gate is undefined, the last or none that applies: S9 and S10
// start at the gate after undefined, where S4 already fixes the state.
func (r *run) next(id, gate string) (string, bool) {
	gates := r.shape.applicable(id)
	i := indexOf(gates, gate)
	if gate == undefined || i < 0 || i+1 >= len(gates) {
		return "", false
	}
	return gates[i+1], true
}

// s1 reports a status whose file name is the id of no task. A status of an
// unread task ends here too, in silence.
func (r *run) s1(st *model.Status) bool {
	if r.project.Tasks[st.ID] != nil {
		return false
	}
	if !r.shape.unread[st.ID] {
		r.add("S1", model.Pos{File: st.File.Path}, st.ID, "", "the file name %s is the id of no task", st.ID)
	}
	return true
}

// s2 reports a status on a task that another names as its parent.
func (r *run) s2(st *model.Status) {
	if r.shape.isParent(st.ID) {
		r.add("S2", model.Pos{File: st.File.Path}, st.ID, "", "%s is a parent: its status derives from its children", st.ID)
	}
}

// s5 reports a gate that names no gate. The schema reports one that is no
// key, and either ends the sequence.
func (r *run) s5(st *model.Status, codes map[string]bool) bool {
	if codes["S5"] {
		return true
	}
	if _, named := r.shape.gate[st.Gate.V]; named {
		return false
	}
	r.add("S5", st.Gate.Node.Pos, st.ID, st.Gate.V, "gate %s names no gate in gates.yaml", st.Gate.V)
	return true
}

// s6 reports a state that names no state and a reason that names no reason.
// The schema reports one that is no key.
func (r *run) s6(st *model.Status) {
	if state := st.State; state.Node.Scalar() && keyPattern.MatchString(state.V) && !r.shape.states[state.V] {
		r.add("S6", state.Node.Pos, st.ID, "", "state %s names no state in gates.yaml", state.V)
	}
	if reason := st.Reason; reason.Node.Scalar() && keyPattern.MatchString(reason.V) && !r.shape.reasons[reason.V] {
		r.add("S6", reason.Node.Pos, st.ID, "", "reason %s names no reason in gates.yaml", reason.V)
	}
}

// s7 reports a status at a gate that does not apply to its task.
func (r *run) s7(st *model.Status) bool {
	if r.shape.appliesTo(st.ID, st.Gate.V) {
		return false
	}
	r.add("S7", st.Gate.Node.Pos, st.ID, st.Gate.V, "gate %s does not apply to %s", st.Gate.V, st.ID)
	return true
}

// s8 reports a status at its last applicable gate without the state complete.
func (r *run) s8(st *model.Status) {
	if st.Gate.V != r.last(st.ID) || (st.State.Node.Scalar() && st.State.V == complete) {
		return
	}
	r.add("S8", posOf(st.File, st.State.Node, st.Gate.Node), st.ID, st.Gate.V,
		"%s stands at its last applicable gate, %s, without the state complete", st.ID, st.Gate.V)
}

// s9 reports a status that states more than its gate before a recursive
// junction, at the first of state, reason and note in the file.
func (r *run) s9(st *model.Status) {
	next, ok := r.next(st.ID, st.Gate.V)
	if !ok || r.shape.kindAt(st.ID, next) != recursive {
		return
	}
	root := st.File.Root
	for i := range root.Fields {
		switch root.Fields[i].Key {
		case "state", "reason", "note":
			r.add("S9", root.Fields[i].KeyPos, st.ID, st.Gate.V,
				"the next junction, %s, is recursive, so the file holds only the gate", next)
			return
		}
	}
}

// s10 reports a status that states no state before a plain junction.
func (r *run) s10(st *model.Status) {
	next, ok := r.next(st.ID, st.Gate.V)
	if !ok || r.shape.kindAt(st.ID, next) != plain || st.State.Node != nil {
		return
	}
	r.add("S10", st.Gate.Node.Pos, st.ID, st.Gate.V, "the next junction, %s, is plain and the file states no state", next)
}

// s12 reports the state complete before the last applicable gate.
func (r *run) s12(st *model.Status) {
	last := r.last(st.ID)
	if !st.State.Node.Scalar() || st.State.V != complete || st.Gate.V == last {
		return
	}
	r.add("S12", st.State.Node.Pos, st.ID, st.Gate.V,
		"the state complete at %s, which is not the last applicable gate, %s", st.Gate.V, last)
}
