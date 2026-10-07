package derive

import (
	"github.com/nbyoung/tablo/internal/model"
)

// By says what supplies a field of a resolved junction.
type By int

// The suppliers. e4c7 A3 names them default, task and assignee.
const (
	ByDefault  By = iota // no entry states the field, and the plain default gives none
	ByTask               // the entry of Field.Task states it
	ByAssignee           // no entry states it, and the task's assignee takes it
)

// Field is one field of a resolved plain junction.
type Field struct {
	V    string
	By   By
	Task string       // the task whose entry states it; "" unless By is ByTask
	Node *model.Value // the value in that entry; nil unless By is ByTask
}

// Junction is a task's junction at one gate after undefined, resolved.
type Junction struct {
	Task, Gate                   string
	Kind                         model.JunctionKind // Plain, Recursive or NotApplicable
	Contributor, Model, Reviewer Field              // of a plain junction
	References                   []*model.Reference // of a plain junction
	ReferencesFrom               string             // the task whose entry states them; "" for none
	Entry                        string             // the task whose entry makes it recursive or exempt
	Snapshot                     *Snapshot          // of a recursive junction

	sources []string
}

// Sources returns the tasks whose entries supply the junction, nearest
// first: each task whose plain entry states a field the junction takes, the
// task whose entry makes it recursive or exempt, and the task whose plain
// entry lifts an exemption though it states no field.
func (j *Junction) Sources() []string {
	return append([]string(nil), j.sources...)
}

// Mark is a junction mark of the method, by the key tableaud 438a A8 gives it.
type Mark string

// The marks.
const (
	MarkPerson     Mark = "person"     // 🧑
	MarkAgent      Mark = "agent"      // 🤖
	MarkReviewer   Mark = "reviewer"   // 👀
	MarkSubproject Mark = "subproject" // 🪆
	MarkExempt     Mark = "exempt"     // —
)

// Symbol returns the mark as README.md#junctions draws it.
func (m Mark) Symbol() string {
	switch m {
	case MarkPerson:
		return "🧑"
	case MarkAgent:
		return "🤖"
	case MarkReviewer:
		return "👀"
	case MarkSubproject:
		return "🪆"
	case MarkExempt:
		return "—"
	}
	return ""
}

// Marks returns the marks of the junction, the contributor's first: person
// or agent, then reviewer only where the reviewer differs from the
// contributor; or subproject; or exempt.
func (j *Junction) Marks() []Mark {
	switch j.Kind {
	case model.Recursive:
		return []Mark{MarkSubproject}
	case model.NotApplicable:
		return []Mark{MarkExempt}
	}
	marks := []Mark{MarkPerson}
	if j.Model.V != "" {
		marks[0] = MarkAgent
	}
	if j.Reviewer.V != "" && j.Reviewer.V != j.Contributor.V {
		marks = append(marks, MarkReviewer)
	}
	return marks
}

// Junction returns the task's junction at a gate, resolved, or nil at
// undefined, at a key gates.yaml lacks and for a task the tree lacks.
func (f *Facts) Junction(id, gate string) *Junction {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return f.junction(id, gate)
}

// Junctions returns the task's junction at every gate after undefined, in order.
func (f *Facts) Junctions(id string) []*Junction {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return append([]*Junction(nil), f.resolved(id)...)
}

// Applicable returns the gates that apply to the task, undefined first.
func (f *Facts) Applicable(id string) []string {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return f.applicable(id)
}

// First returns the task's first applicable gate after undefined, or "".
func (f *Facts) First(id string) string {
	if !f.enter() {
		return ""
	}
	defer f.leave()
	return f.next(id, "undefined")
}

// Last returns the task's last applicable gate, or "".
func (f *Facts) Last(id string) string {
	if !f.enter() {
		return ""
	}
	defer f.leave()
	return f.last(id)
}

// Next returns the next applicable gate after gate, or "" at the last and
// for a key gates.yaml lacks.
func (f *Facts) Next(id, gate string) string {
	if !f.enter() {
		return ""
	}
	defer f.leave()
	return f.next(id, gate)
}

// junction returns one resolved junction, or nil.
func (f *Facts) junction(id, gate string) *Junction {
	list := f.resolved(id)
	if i, ok := f.index[gate]; ok && i > 0 && list != nil {
		return list[i-1]
	}
	return nil
}

// applicable lists the gates no junction exempts the task from.
func (f *Facts) applicable(id string) []string {
	list := f.resolved(id)
	if list == nil {
		return nil
	}
	gates := []string{f.gates[0]}
	for _, j := range list {
		if j.Kind != model.NotApplicable {
			gates = append(gates, j.Gate)
		}
	}
	return gates
}

// next returns the first applicable gate after gate, or "".
func (f *Facts) next(id, gate string) string {
	list := f.resolved(id)
	i, ok := f.index[gate]
	if !ok || list == nil {
		return ""
	}
	for _, j := range list[i:] {
		if j.Kind != model.NotApplicable {
			return j.Gate
		}
	}
	return ""
}

// last returns the last applicable gate: undefined when every other is exempt.
func (f *Facts) last(id string) string {
	gates := f.applicable(id)
	if len(gates) == 0 {
		return ""
	}
	return gates[len(gates)-1]
}

// resolved returns the task's junctions at every gate after undefined, with
// the snapshot of each recursive one, or nil for a task the tree lacks.
func (f *Facts) resolved(id string) []*Junction {
	if list, ok := f.junctions[id]; ok {
		return list
	}
	task := f.task(id)
	if task == nil {
		return nil
	}
	list := make([]*Junction, 0, len(f.gates)-1)
	var snapshots []*Snapshot
	for _, gate := range f.gates[1:] {
		j := f.resolve(task, gate)
		if j.Kind == model.Recursive {
			field := entry(task, gate).Subproject
			link, sub := f.reach(field)
			target := field.ID.V
			if target == "" {
				target = sub.Root()
			}
			for _, s := range snapshots {
				if s.Entry.URL.V == field.URL.V && s.Link == link && s.Target == target {
					j.Snapshot = s
				}
			}
			if j.Snapshot == nil {
				j.Snapshot = &Snapshot{Task: id, Entry: field, Link: link, Target: target, Facts: sub}
				switch {
				case sub == nil:
					j.Snapshot.Why = NoLink
				case sub.task(target) == nil:
					j.Snapshot.Why = NoTask
				}
				snapshots = append(snapshots, j.Snapshot)
			}
			j.Snapshot.Gates = append(j.Snapshot.Gates, gate)
		}
		list = append(list, j)
	}
	f.junctions[id], f.snapshots[id] = list, snapshots
	return list
}

// entry returns the entry a task's file states at a gate, or nil.
func entry(task *model.Task, gate string) *model.Junction {
	for _, e := range task.Junctions {
		if e.Gate == gate {
			return e
		}
	}
	return nil
}

// stated reports whether a field of an entry states a value.
func stated(field model.StrField) bool {
	return field.Node.Scalar() && field.V != ""
}

// resolve walks the task's own entry at a gate and then its ancestors',
// nearest first. The task's own recursive entry resolves at once. A plain
// entry supplies, field by field, what no nearer entry supplies: the
// reviewer; the references, whole; and the contributor with the model as one
// pair. A not-applicable entry ends the walk: with no nearer plain entry the
// junction is exempt, and with one it is plain and holds only what the walk
// gathered below the exemption. A mixed entry, one that states applies:
// true, and a recursive entry of an ancestor supply nothing. Then the
// defaults: the assignee contributes, and reviews an agent that has no
// reviewer.
func (f *Facts) resolve(task *model.Task, gate string) *Junction {
	j := &Junction{Task: task.ID, Gate: gate, Kind: model.Plain}
	nearest := "" // the nearest task with a plain entry
	pair := false // an entry states the contributor or the model
walk:
	for cur := task; cur != nil; cur = f.above(cur) {
		e := entry(cur, gate)
		if e == nil {
			continue
		}
		switch e.Kind() {
		case model.Mixed:
			continue
		case model.Recursive:
			if cur == task {
				return &Junction{Task: task.ID, Gate: gate, Kind: model.Recursive, Entry: task.ID, sources: []string{task.ID}}
			}
			continue
		case model.NotApplicable:
			if !e.Applies.OK || e.Applies.V {
				continue
			}
			if nearest == "" {
				return &Junction{Task: task.ID, Gate: gate, Kind: model.NotApplicable, Entry: cur.ID, sources: []string{cur.ID}}
			}
			if len(j.sources) == 0 {
				j.sources = []string{nearest}
			}
			break walk
		}
		if nearest == "" {
			nearest = cur.ID
		}
		supplies := false
		if !pair && (stated(e.Contributor) || stated(e.Model)) {
			pair, supplies = true, true
			if stated(e.Contributor) {
				j.Contributor = Field{V: e.Contributor.V, By: ByTask, Task: cur.ID, Node: e.Contributor.Node}
			}
			if stated(e.Model) {
				j.Model = Field{V: e.Model.V, By: ByTask, Task: cur.ID, Node: e.Model.Node}
			}
		}
		if j.Reviewer.By != ByTask && stated(e.Reviewer) {
			j.Reviewer = Field{V: e.Reviewer.V, By: ByTask, Task: cur.ID, Node: e.Reviewer.Node}
			supplies = true
		}
		if j.ReferencesFrom == "" && len(e.References) > 0 {
			j.References, j.ReferencesFrom = e.References, cur.ID
			supplies = true
		}
		if supplies {
			j.sources = append(j.sources, cur.ID)
		}
	}
	if j.Contributor.By != ByTask {
		j.Contributor = Field{V: task.Assignee.V, By: ByAssignee}
	}
	if j.Reviewer.By != ByTask && j.Model.V != "" {
		j.Reviewer = Field{V: task.Assignee.V, By: ByAssignee}
	}
	return j
}

// Snapshot is the task a recursive junction reads, in the project its url fixes.
type Snapshot struct {
	Task   string            // the task that holds the junction
	Gates  []string          // the gates whose junctions name this target, in order
	Entry  *model.Subproject // the field of the first of them
	Link   *model.Link
	Target string // the task read there: the id stated, or that project's root
	Facts  *Facts // the facts of that project; nil when the link has none
	Why    Why    // NoLink or NoTask
}

// Snapshots returns one snapshot per distinct link and target, in gate order.
func (f *Facts) Snapshots(id string) []*Snapshot {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	f.resolved(id)
	return append([]*Snapshot(nil), f.snapshots[id]...)
}
