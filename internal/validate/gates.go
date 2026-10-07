package validate

import "github.com/nbyoung/tablo/internal/model"

// gatesPath is the path of the gates file below .tableaux.
const gatesPath = "gates.yaml"

// gating applies the rules of gates.yaml: G1 when the file is absent, else
// its schema and the rules a schema cannot state.
func (r *run) gating() {
	if r.g1() {
		return
	}
	g := r.project.Gating
	if g == nil {
		return // the Loader reports a file that does not parse
	}
	r.leaves(checkFile(gatesFile, g.File, ""))
	r.g6()
	r.g9()
	r.g10()
	r.g12()
}

// g1 reports a project with no gates file, and ends the rules of the file.
func (r *run) g1() bool {
	if r.file(gatesPath) != nil {
		return false
	}
	r.add("G1", model.Pos{File: gatesPath}, "", "", "gates.yaml is missing")
	return true
}

// twice calls report for each scalar key of a list that an earlier item of
// the list states, with the later key.
func twice(keys []model.StrField, report func(key *model.Value)) {
	stated := map[string]bool{}
	for _, key := range keys {
		if !key.Node.Scalar() {
			continue
		}
		if stated[key.V] {
			report(key.Node)
		}
		stated[key.V] = true
	}
}

// g6 reports a gate key that an earlier gate states.
func (r *run) g6() {
	var keys []model.StrField
	for _, gate := range r.project.Gating.Gates {
		keys = append(keys, gate.Key)
	}
	twice(keys, func(key *model.Value) {
		r.add("G6", key.Pos, "", key.Text, "gate key %s is stated twice", key.Text)
	})
}

// g9 reports a state key that an earlier state states.
func (r *run) g9() {
	var keys []model.StrField
	for _, state := range r.project.Gating.States {
		keys = append(keys, state.Key)
	}
	twice(keys, func(key *model.Value) {
		r.add("G9", key.Pos, "", "", "state key %s is stated twice", key.Text)
	})
}

// g10 reports a reason key that an earlier reason states. The schema reports
// a malformed reason.
func (r *run) g10() {
	var keys []model.StrField
	for _, reason := range r.project.Gating.Reasons {
		keys = append(keys, reason.Key)
	}
	twice(keys, func(key *model.Value) {
		r.add("G10", key.Pos, "", "", "reason key %s is stated twice", key.Text)
	})
}

// g12 reports a states list that lacks undefined or complete, or whose first
// state of either key states a severity other than the integer 0. The
// schema's contains gives one leaf for every state, so the rule reads the
// states itself.
func (r *run) g12() {
	g := r.project.Gating
	if len(g.States) == 0 {
		return
	}
	for _, key := range []string{undefined, "complete"} {
		var first *model.State
		for _, state := range g.States {
			if state.Key.Node.Scalar() && state.Key.V == key {
				first = state
				break
			}
		}
		switch {
		case first == nil:
			r.add("G12", posOf(g.File, g.File.Root.Get("states")), "", "", "states lacks %s", key)
		case first.Severity.Node != nil && (!first.Severity.OK || first.Severity.V != 0):
			r.add("G12", first.Severity.Node.Pos, "", "", "the state %s has a severity other than 0", key)
		}
	}
}
