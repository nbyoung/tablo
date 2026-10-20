package validate

import "github.com/nbyoung/tablo/internal/model"

// a1 judges agents.yaml, when the project holds one, by its schema: every
// leaf is A1, at the leaf's node or else the file.
func (r *run) a1() {
	if a := r.project.Agents; a != nil {
		r.leaves(checkFile(agentsFile, a.File, ""))
	}
}

// a2 reports a plain entry that states a model beside a contributor the
// agents file does not list, where the project keeps the file. The entry
// has no schema leaf, so its contributor and its model are scalars or
// absent.
func (r *run) a2(t *model.Task, j *model.Junction) {
	a := r.project.Agents
	if a == nil || !j.Model.Node.Scalar() || !j.Contributor.Node.Scalar() || a.Lists(j.Contributor.V) {
		return
	}
	r.add("A2", j.Contributor.Node.Pos, t.ID, j.Gate, "the %s junction states the model %s beside %s, whom agents.yaml does not list",
		j.Gate, j.Model.V, j.Contributor.V)
}
