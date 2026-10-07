// Package derive gives every fact a view shows that no file states: each
// junction resolved through the ancestors with the supplier of every field,
// each requirement's condition, each task's authorisation, each status's
// date and recorder, each review, each parent's roll-up, each task's events,
// and each recursive junction's status from the subproject at its pin. It is
// a pure function of the model the Loader returns and of the passes
// internal/history makes: it runs no process and reads no file, clock or
// environment, so the same model and the same passes give equal facts on
// every host. It raises no diagnostic; the Validator explains what the facts
// leave undetermined.
package derive

import (
	"sync"

	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
)

// New derives the facts of p, the project a Load returns, and of every
// project its links reach. logs is nil for a load with no history to read.
func New(p *model.Project, logs *history.Set) *Facts {
	return (&family{logs: logs, facts: map[*model.Project]*Facts{}}).of(p)
}

// family holds the facts of every project of one load. One lock guards them
// all, since a fact of one project reads the facts of another.
type family struct {
	mu    sync.Mutex
	logs  *history.Set
	facts map[*model.Project]*Facts
}

// of returns the facts of one project of the load. The caller holds the
// lock, or is New.
func (fam *family) of(p *model.Project) *Facts {
	if f, ok := fam.facts[p]; ok {
		return f
	}
	f := build(fam, p, fam.logs.Of(p))
	fam.facts[p] = f
	return f
}

// Facts are the derived facts of one project at one source. A Facts is
// immutable and safe for concurrent use; it computes each fact at its first
// call and keeps it. After a refusal every method below Refused returns the
// zero value of its result.
type Facts struct {
	fam     *family
	p       *model.Project
	log     *history.Log
	refused *Refusal

	// The tree and the gating, built once: empty after a refusal.
	root     string
	order    []string
	place    map[string]int // a task's index in order
	parent   map[string]string
	children map[string][]string
	gates    []string
	index    map[string]int
	severity map[string]int

	// The facts, each kept from its first call.
	junctions  map[string][]*Junction // by task: one per gate after undefined
	snapshots  map[string][]*Snapshot
	statuses   map[string]*Status
	busy       map[string]bool // the statuses under way, to end a cycle
	conditions map[string][]*Condition
}

// build makes the facts of one project: the refusal, or the tree and the gating.
func build(fam *family, p *model.Project, log *history.Log) *Facts {
	f := &Facts{
		fam: fam, p: p, log: log,
		place: map[string]int{}, parent: map[string]string{}, children: map[string][]string{},
		index: map[string]int{}, severity: map[string]int{},
		junctions: map[string][]*Junction{}, snapshots: map[string][]*Snapshot{},
		statuses: map[string]*Status{}, busy: map[string]bool{},
		conditions: map[string][]*Condition{},
	}
	if f.refused = refusal(p); f.refused != nil {
		return f
	}
	for i, gate := range p.Gating.Gates {
		f.gates = append(f.gates, gate.Key.V)
		f.index[gate.Key.V] = i
	}
	for _, state := range p.Gating.States {
		if _, stated := f.severity[state.Key.V]; !stated {
			f.severity[state.Key.V] = state.Severity.V
		}
	}
	f.grow()
	return f
}

// enter takes the family's lock for one exported method. It reports false,
// and takes no lock, for a nil Facts and after a refusal.
func (f *Facts) enter() bool {
	if f == nil || f.refused != nil {
		return false
	}
	f.fam.mu.Lock()
	return true
}

// leave releases the family's lock.
func (f *Facts) leave() { f.fam.mu.Unlock() }

// Project returns the project the facts derive from.
func (f *Facts) Project() *model.Project {
	if f == nil {
		return nil
	}
	return f.p
}

// Log returns the pass the facts read, or nil without history.
func (f *Facts) Log() *history.Log {
	if f == nil {
		return nil
	}
	return f.log
}

// Trunk returns the project's trunk: its name, what names it, the ref and
// the tip. It is the zero Trunk without history.
func (f *Facts) Trunk() history.Trunk {
	if f.Log() == nil {
		return history.Trunk{}
	}
	return f.log.Trunk
}

// OnTrunk reports whether the source is on the trunk's first-parent line.
func (f *Facts) OnTrunk() bool {
	if f.Log() == nil {
		return false
	}
	source := f.log.Commit(f.log.Source)
	return source != nil && source.Trunk >= 0
}

// Refused returns why the files give no structure to derive from, or nil.
func (f *Facts) Refused() *Refusal {
	if f == nil {
		return nil
	}
	return f.refused
}

// Sub returns the facts of the project a link leads to, or nil when the link
// has none.
func (f *Facts) Sub(l *model.Link) *Facts {
	if l == nil || l.Project == nil || !f.enter() {
		return nil
	}
	defer f.leave()
	return f.fam.of(l.Project)
}

// reach returns the link a subproject field names and the facts of the
// project it leads to; either is nil when the field has none.
func (f *Facts) reach(field *model.Subproject) (*model.Link, *Facts) {
	if field == nil || field.Link == nil {
		return nil, nil
	}
	if field.Link.Project == nil {
		return field.Link, nil
	}
	return field.Link, f.fam.of(field.Link.Project)
}

// Refusal says why the files give no structure to derive from.
type Refusal struct {
	Part  string   // "project", "version", "gates", "states" or "tree"
	Rules []string // the rules of RULES.md that report it
}

// refusal returns the first part of a project the Derivation cannot read, in
// the order project, version, gates, states, tree, or nil. Every other fault
// leaves the facts standing.
func refusal(p *model.Project) *Refusal {
	switch {
	case p == nil || !p.Exists:
		return &Refusal{Part: "project", Rules: []string{"P1"}}
	case p.Version == nil || !p.Version.Accepted:
		return &Refusal{Part: "version", Rules: []string{"P2", "P3", "P4"}}
	case !gatesRead(p.Gating):
		return &Refusal{Part: "gates", Rules: []string{"G1", "G2", "G3", "G4", "G6"}}
	case !statesRead(p.Gating):
		return &Refusal{Part: "states", Rules: []string{"G7", "G8", "G9", "G12"}}
	case !treeRead(p):
		return &Refusal{Part: "tree", Rules: []string{"T8", "T9", "T10"}}
	}
	return nil
}

// gatesRead reports whether the gates start at undefined with one more at
// least, each with a key that is a scalar, not empty and stated once.
func gatesRead(g *model.Gating) bool {
	if g == nil || len(g.Gates) < 2 || g.Gates[0].Key.V != "undefined" {
		return false
	}
	seen := map[string]bool{}
	for _, gate := range g.Gates {
		if !gate.Key.Node.Scalar() || gate.Key.V == "" || seen[gate.Key.V] {
			return false
		}
		seen[gate.Key.V] = true
	}
	return true
}

// statesRead reports whether the states hold undefined and complete at
// severity 0, state no key twice, and give each an integer severity from 0.
func statesRead(g *model.Gating) bool {
	seen := map[string]bool{}
	for _, state := range g.States {
		if seen[state.Key.V] || !state.Severity.OK || state.Severity.V < 0 {
			return false
		}
		seen[state.Key.V] = true
		if (state.Key.V == "undefined" || state.Key.V == "complete") && state.Severity.V != 0 {
			return false
		}
	}
	return seen["undefined"] && seen["complete"]
}

// treeRead reports whether the tasks form one tree: one root, every parent
// there, and no cycle.
func treeRead(p *model.Project) bool {
	roots := 0
	for _, task := range p.Tasks {
		if task.Parent == nil {
			roots++
			continue
		}
		if !task.Parent.ID.Node.Scalar() || p.Tasks[task.Parent.ID.V] == nil {
			return false
		}
	}
	if roots != 1 {
		return false
	}
	for _, task := range p.Tasks {
		seen := map[string]bool{}
		for cur := task; cur.Parent != nil; cur = p.Tasks[cur.Parent.ID.V] {
			if seen[cur.ID] {
				return false
			}
			seen[cur.ID] = true
		}
	}
	return true
}

// Why says why a fact is undetermined.
type Why int

// The reasons. Determined is none.
const (
	Determined Why = iota
	NoHistory      // no commit to read, or the pass does not hold the commit
	NoTrunk        // the trunk is undetermined
	OffTrunk       // the source is off the trunk's first-parent line
	NoCommit       // no commit of the history decides
	NoLink         // the link has no project
	NoTask         // the project holds no such task
	NoGate         // the key names no gate of gates.yaml, or none applies
	NoState        // the file states no state where one is due
	NoChildren     // no child of the parent has a status to roll up
	Cycle          // the statuses of two projects read each other
)

// String returns the reason's name.
func (w Why) String() string {
	names := [...]string{"determined", "no history", "no trunk", "off trunk", "no commit", "no link",
		"no task", "no gate", "no state", "no children", "cycle"}
	if w < 0 || int(w) >= len(names) {
		return "why?"
	}
	return names[w]
}

// Known is a fact that may be undetermined.
type Known int

// The values of a Known.
const (
	Unknown Known = iota
	No
	Yes
)

// String returns "unknown", "no" or "yes".
func (k Known) String() string {
	switch k {
	case No:
		return "no"
	case Yes:
		return "yes"
	}
	return "unknown"
}
