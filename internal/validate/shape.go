package validate

import (
	"regexp"
	"sort"
	"strings"

	"github.com/nbyoung/tablo/internal/model"
)

// idPattern and keyPattern are the patterns the schemas give an id and a key.
var (
	idPattern  = regexp.MustCompile(`^[0-9a-f]{4}$`)
	keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
)

// undefined is the key of the first gate and of its state.
const undefined = "undefined"

// resolved is the kind of a junction once the ancestors decide it.
type resolved int

// The kinds a junction resolves to.
const (
	plain resolved = iota
	recursive
	notApplicable
)

// shape is what the rules ask of a project beyond its files.
type shape struct {
	project  *model.Project
	gates    []string            // the scalar gate keys, in file order, the first statement of each
	gate     map[string]int      // a key to its index in gates
	states   map[string]bool     // the scalar state keys
	reasons  map[string]bool     // the scalar reason keys
	unread   map[string]bool     // ids whose task file is in p.Files and gave no task: L1
	children map[string][]string // by the parent id as written, each list in id order
	roots    []string            // tasks with a nil Parent, in id order
	applies  map[string][]string // what applicable has answered, by task
}

// newShape builds the shape of a project.
func newShape(p *model.Project) *shape {
	s := &shape{
		project:  p,
		gate:     map[string]int{},
		states:   map[string]bool{},
		reasons:  map[string]bool{},
		unread:   map[string]bool{},
		children: map[string][]string{},
		applies:  map[string][]string{},
	}
	if g := p.Gating; g != nil {
		for _, gate := range g.Gates {
			if _, stated := s.gate[gate.Key.V]; gate.Key.Node.Scalar() && !stated {
				s.gate[gate.Key.V] = len(s.gates)
				s.gates = append(s.gates, gate.Key.V)
			}
		}
		for _, state := range g.States {
			if state.Key.Node.Scalar() {
				s.states[state.Key.V] = true
			}
		}
		for _, reason := range g.Reasons {
			if reason.Key.Node.Scalar() {
				s.reasons[reason.Key.V] = true
			}
		}
	}
	for _, file := range p.Files {
		if id, ok := strings.CutPrefix(file.Path, "tasks/"); ok {
			id = strings.TrimSuffix(id, ".yaml")
			if p.Tasks[id] == nil {
				s.unread[id] = true
			}
		}
	}
	for _, id := range p.TaskIDs() {
		switch parent := p.Tasks[id].Parent; {
		case parent == nil:
			s.roots = append(s.roots, id)
		case parent.ID.Node.Scalar():
			s.children[parent.ID.V] = append(s.children[parent.ID.V], id)
		}
	}
	return s
}

// ancestors returns the tasks above id, nearest first, and whether the chain
// is whole: it ends at a task with no parent and meets no task that is
// missing, unread or already met.
func (s *shape) ancestors(id string) (chain []string, whole bool) {
	met := map[string]bool{id: true}
	task := s.project.Tasks[id]
	if task == nil {
		return nil, false
	}
	for task.Parent != nil {
		if !task.Parent.ID.Node.Scalar() {
			return chain, false
		}
		above := task.Parent.ID.V
		if task = s.project.Tasks[above]; task == nil || met[above] {
			return chain, false
		}
		met[above] = true
		chain = append(chain, above)
	}
	return chain, true
}

// entryAt returns the entry a task's own file states at a gate, or nil.
func (s *shape) entryAt(id, gate string) *model.Junction {
	task := s.project.Tasks[id]
	if task == nil {
		return nil
	}
	for _, j := range task.Junctions {
		if j.Gate == gate {
			return j
		}
	}
	return nil
}

// plainFields are the fields of a plain junction entry.
var plainFields = map[string]bool{"contributor": true, "model": true, "reviewer": true, "references": true}

// decides says what an entry decides about its gate, and whether it decides
// anything. An entry decides when it is a mapping of one kind and well formed
// as that kind: a plain entry with no field of another kind and none unknown;
// { applies: false } and no other field; a subproject and no other field. An
// entry of any other form, applies: true or a mixed one among them, decides
// nothing, so the entry a J rule rejects causes no second finding about the
// gate it meant to change.
func decides(j *model.Junction) (resolved, bool) {
	if j.Node == nil || j.Node.Kind != model.Map {
		return plain, false
	}
	switch j.Kind() {
	case model.Plain:
		for i := range j.Node.Fields {
			if !plainFields[j.Node.Fields[i].Key] {
				return plain, false
			}
		}
		return plain, true
	case model.NotApplicable:
		return notApplicable, len(j.Node.Fields) == 1 && j.Applies.OK && !j.Applies.V
	case model.Recursive:
		return recursive, len(j.Node.Fields) == 1
	}
	return plain, false
}

// kindAt resolves the kind of a task's junction at a gate: plain at
// undefined; else the first entry at that key that decides, from the task's
// own and then its ancestors', nearest first; else plain. A recursive entry
// decides on the task itself and never on an ancestor.
func (s *shape) kindAt(id, gate string) resolved {
	if gate == undefined {
		return plain
	}
	above, _ := s.ancestors(id)
	for i, at := range append([]string{id}, above...) {
		entry := s.entryAt(at, gate)
		if entry == nil {
			continue
		}
		switch kind, ok := decides(entry); {
		case !ok:
		case kind != recursive:
			return kind
		case i == 0:
			return recursive
		}
	}
	return plain
}

// applicable returns the gates that apply to a task, in order.
func (s *shape) applicable(id string) []string {
	if gates, ok := s.applies[id]; ok {
		return gates
	}
	gates := []string{}
	for _, gate := range s.gates {
		if s.kindAt(id, gate) != notApplicable {
			gates = append(gates, gate)
		}
	}
	s.applies[id] = gates
	return gates
}

// appliesTo reports whether a gate applies to a task.
func (s *shape) appliesTo(id, gate string) bool {
	return indexOf(s.applicable(id), gate) >= 0
}

// isParent reports whether a task names id as its parent.
func (s *shape) isParent(id string) bool {
	return len(s.children[id]) > 0
}

// indexOf returns the index of an item in a list, or -1.
func indexOf(list []string, item string) int {
	for i, x := range list {
		if x == item {
			return i
		}
	}
	return -1
}

// sortedKeys returns the keys of a set in byte order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
