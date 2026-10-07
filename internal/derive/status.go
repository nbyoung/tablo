package derive

import (
	"github.com/nbyoung/tablo/internal/model"
)

// StatusKind says where a status comes from.
type StatusKind int

// The kinds of a status.
const (
	Recorded    StatusKind = iota // a leaf's file
	Absent                        // a leaf without a file: undefined at undefined
	RolledUp                      // a parent's, from its children
	Snapshotted                   // a leaf whose next junction is recursive
)

// Status is a task's status.
type Status struct {
	Task                      string
	Kind                      StatusKind
	Gate, State, Reason, Note string
	Why                       Why           // why the state is undetermined
	File                      *model.Status // nil for Absent and RolledUp
	From                      string        // RolledUp: the child the state comes from
	Considered                []string      // RolledUp: the children considered, in display order
	Snapshot                  *Snapshot     // Snapshotted
	Of                        *Status       // Snapshotted: the status of the task read there

	cyclic bool // the status reads one that reads it
}

// Derived reports whether a roll-up gives the state: a parent's status, or a
// snapshot of a parent.
func (s *Status) Derived() bool {
	return s != nil && (s.Kind == RolledUp || s.Kind == Snapshotted && s.Of.Derived())
}

// Status returns the status of a task: a leaf's from its file, undefined at
// undefined without one, or from the task its next junction reads when that
// junction is recursive; a parent's from its children. It is nil for a task
// the tree lacks.
func (f *Facts) Status(id string) *Status {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return f.status(id)
}

// Chain returns the statuses a status comes through, the task's first: each
// roll-up's child and each snapshot's task, to the leaf whose file states it.
func (f *Facts) Chain(id string) []*Status {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	var chain []*Status
	at, s := f, f.status(id)
	for s != nil {
		chain = append(chain, s)
		switch {
		case s.Kind == RolledUp && s.From != "":
			s = at.status(s.From)
		case s.Kind == Snapshotted && s.Of != nil:
			at, s = s.Snapshot.Facts, s.Of
		default:
			s = nil
		}
	}
	return chain
}

// status returns one status and keeps it. A status that is under way reads
// itself through a subproject: it ends the cycle.
func (f *Facts) status(id string) *Status {
	if s, ok := f.statuses[id]; ok {
		return s
	}
	task := f.task(id)
	if task == nil {
		return nil
	}
	if f.busy[id] {
		return f.cycle(id)
	}
	f.busy[id] = true
	var s *Status
	if len(f.children[id]) > 0 {
		s = f.rollUp(id)
	} else {
		s = f.leaf(id)
	}
	delete(f.busy, id)
	f.statuses[id] = s
	return s
}

// cycle returns the status of a task whose status reads itself: the gate
// stands and the state is empty.
func (f *Facts) cycle(id string) *Status {
	s := &Status{Task: id, Kind: RolledUp, Why: Cycle, cyclic: true}
	if file := f.p.Statuses[id]; file != nil && len(f.children[id]) == 0 {
		s.Kind, s.Gate, s.File = Snapshotted, file.Gate.V, file
		s.Snapshot = f.snapshotAfter(id, file.Gate.V)
	}
	return s
}

// snapshotAfter returns the snapshot of the task's next junction after a
// gate, or nil when that junction is not recursive.
func (f *Facts) snapshotAfter(id, gate string) *Snapshot {
	if j := f.junction(id, f.next(id, gate)); j != nil {
		return j.Snapshot
	}
	return nil
}

// leaf returns the status of a leaf. When the junction after the gate its
// file states is recursive, the file gives the gate alone and the task read
// there gives the rest.
func (f *Facts) leaf(id string) *Status {
	file := f.p.Statuses[id]
	if file == nil {
		return &Status{Task: id, Kind: Absent, Gate: "undefined", State: "undefined"}
	}
	snapshot := f.snapshotAfter(id, file.Gate.V)
	if snapshot == nil {
		s := &Status{Task: id, Kind: Recorded, Gate: file.Gate.V, State: file.State.V,
			Reason: file.Reason.V, Note: file.Note.V, File: file}
		if s.State == "" {
			s.Why = NoState
		}
		return s
	}
	s := &Status{Task: id, Kind: Snapshotted, Gate: file.Gate.V, File: file, Snapshot: snapshot, Why: snapshot.Why}
	if s.Why != Determined {
		return s
	}
	of := snapshot.Facts.status(snapshot.Target)
	if of.cyclic {
		s.Why, s.cyclic = Cycle, true
		return s
	}
	s.Of, s.State, s.Reason, s.Note, s.Why = of, of.State, of.Reason, of.Note, of.Why
	return s
}

// rollUp derives a parent's status from its children's, by the four steps
// of README.md#status. A child whose gate gates.yaml lacks stays out, and a
// state it lacks ranks 0.
func (f *Facts) rollUp(id string) *Status {
	s := &Status{Task: id, Kind: RolledUp}
	var all, severe []*Status
	for _, child := range f.children[id] {
		c := f.status(child)
		s.cyclic = s.cyclic || c.cyclic
		if _, named := f.index[c.Gate]; !named {
			continue
		}
		all = append(all, c)
		if f.severity[c.State] != 0 {
			severe = append(severe, c)
		}
	}
	considered := severe
	if len(considered) == 0 {
		considered = all
	}
	if len(considered) == 0 {
		s.Why = NoChildren
		return s
	}
	from := considered[0]
	for _, c := range considered {
		s.Considered = append(s.Considered, c.Task)
		gate, earliest := f.index[c.Gate], f.index[from.Gate]
		if gate < earliest || gate == earliest && f.severity[c.State] > f.severity[from.State] {
			from = c
		}
	}
	s.Gate, s.State, s.Reason, s.Note, s.Why, s.From = from.Gate, from.State, from.Reason, from.Note, from.Why, from.Task
	return s
}
