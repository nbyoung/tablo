package views

import (
	"path"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// The two files the gate definition reads, below .tableaux.
var gateFiles = []string{"gates.yaml", "version.yaml"}

// Gates returns the gate definition. The Derivation does not refuse f, and
// q.Task is "" or a task of the project.
func Gates(f *derive.Facts, q Query) *view.Gates {
	b := builder{f, q}
	out := &view.Gates{Legend: b.legend()}
	if q.Task != "" {
		ref := b.ref(q.Task)
		out.Task = &ref
	}
	if q.Level < Detail {
		return out
	}
	out.GatesDetail = &view.GatesDetail{ForTask: b.forTask()}
	if q.Level < Provenance {
		return out
	}
	out.GatesProvenance = &view.GatesProvenance{Files: []view.FileCommit{}, Commands: []view.Command{}}
	for _, below := range gateFiles {
		out.Files = append(out.Files, view.FileCommit{Path: path.Join(planDir, below), Commit: b.commit(f.LastChange(below))})
	}
	if log := f.Log(); log != nil {
		for _, below := range gateFiles {
			out.Commands = append(out.Commands, view.Command{Text: "git show " + log.Source + ":" + b.fromRoot(below)})
		}
		for _, below := range gateFiles {
			out.Commands = append(out.Commands, view.Command{
				Text: "git log -1 --format='%H %as %an <%ae> / %cn <%ce> %s' " + log.Source + " -- " + b.fromRoot(below),
			})
		}
	}
	return out
}

// forTask says how the junctions of the task parameter expand each gate,
// undefined first; without a task it is empty.
func (b builder) forTask() []view.GateFor {
	rows := []view.GateFor{}
	if b.q.Task == "" {
		return rows
	}
	for _, gate := range b.f.Gates() {
		row := view.GateFor{Gate: gate, Applies: true, References: []view.Reference{}}
		// undefined has no work of its own and no junction: it applies.
		if j := b.f.Junction(b.q.Task, gate); j != nil {
			row.Applies = j.Kind != model.NotApplicable
			row.References = b.refs(j.References)
			row.StatedBy = j.ReferencesFrom
		}
		rows = append(rows, row)
	}
	return rows
}

// fromRoot returns the path of a file below .tableaux, from the repository
// root, as a command line names it.
func (b builder) fromRoot(below string) string {
	return path.Join(b.f.Project().Where.Dir, planDir, below)
}
