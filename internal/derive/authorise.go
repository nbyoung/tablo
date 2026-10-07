package derive

import (
	"path"
	"strings"

	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
)

// passIndex is the pass as the facts of the history ask for it: the commits
// the source reaches, and which of them change a path or carry a trailer.
type passIndex struct {
	source   []*history.Commit            // the commits the source reaches, the newest first
	changed  map[string][]*history.Commit // by path: those that change it, the newest first
	trailers map[string][]*history.Commit // by key, NUL and value: those that carry it, the newest first
}

// indexed returns the index of the pass, built at its first call. Without
// history it is empty. A trailer's value indexes with single spaces.
func (f *Facts) indexed() *passIndex {
	if f.pass != nil {
		return f.pass
	}
	f.pass = &passIndex{changed: map[string][]*history.Commit{}, trailers: map[string][]*history.Commit{}}
	if f.log == nil {
		return f.pass
	}
	for _, c := range f.log.Commits {
		if !c.InSource {
			continue
		}
		f.pass.source = append(f.pass.source, c)
		for _, change := range c.Changes {
			if f.log.Changed(c, change.Path) {
				f.pass.changed[change.Path] = append(f.pass.changed[change.Path], c)
			}
		}
		for _, t := range c.Trailers {
			key := t.Key + "\x00" + strings.Join(strings.Fields(t.Value), " ")
			if list := f.pass.trailers[key]; len(list) == 0 || list[len(list)-1] != c {
				f.pass.trailers[key] = append(list, c)
			}
		}
	}
	return f.pass
}

// planPath returns the path of a task's file in a directory of .tableaux,
// relative to the repository root.
func (f *Facts) planPath(dir, id string) string {
	return path.Join(f.p.Where.Dir, ".tableaux", dir, id+".yaml")
}

// newest returns the newest commit the source reaches that changes a path
// or carries a trailer, given as its key, NUL and its value; "" asks for no
// trailer. It is nil when none does and without history.
func (f *Facts) newest(changed, trailer string) *history.Commit {
	pass := f.indexed()
	var found *history.Commit
	if list := pass.changed[changed]; len(list) > 0 {
		found = list[0]
	}
	if list := pass.trailers[trailer]; trailer != "" && len(list) > 0 && (found == nil || list[0].Seq < found.Seq) {
		found = list[0]
	}
	return found
}

// Way is how a deciding commit accepts.
type Way int

// The ways.
const (
	NoWay     Way = iota
	ByCommit      // it changes the task's file
	ByMerge       // it merges a change to the task's file
	ByTrailer     // it carries Authorised: and leaves the file alone
)

// String returns "commit", "merge" or "trailer", or "none".
func (w Way) String() string {
	switch w {
	case ByCommit:
		return "commit"
	case ByMerge:
		return "merge"
	case ByTrailer:
		return "trailer"
	}
	return "none"
}

// Authorisation is a task's authorisation.
type Authorisation struct {
	Task        string
	Authorised  bool
	Why         Why             // proposed with no deciding commit: NoHistory, NoTrunk, OffTrunk or NoCommit
	Commit      *history.Commit // the deciding commit
	Way         Way
	Judges      []string // the emails that may accept, as the files stand at the deciding commit, nearest first
	Author      bool     // the author is one of them
	Committer   bool     // the committer is one of them
	By          string   // the authority that accepts: the author's email before the committer's; else the author's
	Differs     bool     // the task's file in view is not the one at the trunk's tip
	Uncommitted bool     // the task's file in view is not the source commit's
}

// Authorisation returns the authorisation of a task of the tree, or nil.
// The source is on the trunk when its commit is on the first-parent line of
// the trunk's tip; off it, or with no trunk, every task is proposed and has
// no deciding commit.
func (f *Facts) Authorisation(id string) *Authorisation {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return f.authorisation(id)
}

// AuthorisationAt returns the authorisation as it stands at a commit of the
// trunk's first-parent line, read from the line up to that commit. The task
// need not be one of the tree in view.
func (f *Facts) AuthorisationAt(id, commit string) *Authorisation {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	if f.log == nil {
		return &Authorisation{Task: id, Why: NoHistory}
	}
	return f.decide(id, f.log.Commit(commit))
}

// authorisation returns the authorisation in view and keeps it.
func (f *Facts) authorisation(id string) *Authorisation {
	if a, ok := f.authorisations[id]; ok {
		return a
	}
	if f.task(id) == nil {
		return nil
	}
	a := &Authorisation{Task: id, Why: NoHistory}
	if f.log != nil {
		a = f.decide(id, f.log.Commit(f.log.Source))
		below := "tasks/" + id + ".yaml"
		a.Uncommitted = f.uncommitted(below)
		if tip := f.log.Project(f.log.Trunk.Tip); tip != nil {
			a.Differs = !same(file(f.p, below), file(tip, below))
		}
	}
	f.authorisations[id] = a
	return a
}

// decide reads the authorisation of a task from a commit back along the
// trunk's first-parent line. The deciding commit is the nearest that changes
// the task's file against its first parent or carries Authorised: for it.
// The task is authorised when the author's or the committer's email is a
// judge's.
func (f *Facts) decide(id string, from *history.Commit) *Authorisation {
	a := &Authorisation{Task: id}
	switch {
	case from == nil:
		a.Why = NoHistory
	case f.log.Trunk.Tip == "":
		a.Why = NoTrunk
	case from.Trunk < 0:
		a.Why = OffTrunk
	}
	if a.Why != Determined {
		return a
	}
	file := f.planPath("tasks", id)
	for c := from; c != nil; c = f.first(c) {
		changes := false
		for _, change := range c.Changes {
			changes = changes || change.Path == file
		}
		switch {
		case changes && c.Merge():
			a.Way = ByMerge
		case changes:
			a.Way = ByCommit
		case contains(c.Values("Authorised"), id):
			a.Way = ByTrailer
		default:
			continue
		}
		a.Commit, a.Judges = c, f.judges(id, c)
		a.Author, a.Committer = contains(a.Judges, c.Author.Email), contains(a.Judges, c.Committer.Email)
		a.Authorised = a.Author || a.Committer
		a.By = c.Author.Email
		if !a.Author && a.Committer {
			a.By = c.Committer.Email
		}
		return a
	}
	a.Why = NoCommit
	return a
}

// first returns the first parent of a commit in the pass, or nil.
func (f *Facts) first(c *history.Commit) *history.Commit {
	if len(c.Parents) == 0 {
		return nil
	}
	return f.log.Commit(c.Parents[0])
}

// judges returns the emails that may accept a task at its deciding commit:
// its authorities as the files stand at that commit, nearest first, and for
// the root the owner as the files stand at that commit's first parent, so
// that the owner who hands the root over is the one who accepts the
// hand-over (decision 3). The first commit of a root has no owner before it,
// and the root's own assignee judges.
func (f *Facts) judges(id string, c *history.Commit) []string {
	p := f.log.Project(c.ID)
	task := p.Tasks[id]
	if task == nil {
		return nil
	}
	var judges []string
	add := func(email string) {
		if email != "" && !contains(judges, email) {
			judges = append(judges, email)
		}
	}
	if task.Parent == nil {
		if before := f.first(c); before != nil {
			var roots []*model.Task
			for _, other := range f.log.Project(before.ID).Tasks {
				if other.Parent == nil {
					roots = append(roots, other)
				}
			}
			if len(roots) == 1 {
				add(roots[0].Assignee.V)
				return judges
			}
		}
		add(task.Assignee.V)
		return judges
	}
	seen := map[string]bool{id: true}
	for at := p.Tasks[task.Parent.ID.V]; at != nil && !seen[at.ID]; {
		seen[at.ID] = true
		add(at.Assignee.V)
		if at.Parent == nil {
			break
		}
		at = p.Tasks[at.Parent.ID.V]
	}
	return judges
}

// contains reports whether a list holds a string.
func contains(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}

// Pin is the commit a link reads and where it stands in that repository.
type Pin struct {
	Link     *model.Link
	Commit   string // the commit read; "" for a directory of the same repository
	Recorded string // a submodule: the gitlink of the source commit
	Moved    bool   // a submodule: the checkout's HEAD is not Recorded
	Tip      string // the tip of that project's trunk
	OnTrunk  Known  // Commit is on that trunk's first-parent line
	Behind   Known  // OnTrunk, and Commit is not Tip
}

// Pin returns the commit a link of this project reads and where it stands
// on the trunk of the project it leads to, or nil for no link. A directory
// of the same repository has no pin, so OnTrunk and Behind stay unknown.
func (f *Facts) Pin(l *model.Link) *Pin {
	if l == nil || !f.enter() {
		return nil
	}
	defer f.leave()
	pin := &Pin{Link: l, Commit: l.Commit}
	if l.Form == model.Submodule && f.log != nil {
		pin.Recorded = f.log.Object(f.log.Source, l.URL)
		pin.Moved = l.Checkout != "" && pin.Recorded != l.Commit
	}
	if l.Project == nil {
		return pin
	}
	sub := f.fam.of(l.Project)
	if sub.log == nil || sub.log.Trunk.Tip == "" {
		return pin
	}
	pin.Tip = sub.log.Trunk.Tip
	if c := sub.log.Commit(l.Commit); c != nil && l.Form != model.Directory {
		pin.OnTrunk = known(c.Trunk >= 0)
		if pin.OnTrunk == Yes {
			pin.Behind = known(l.Commit != pin.Tip)
		}
	}
	return pin
}
