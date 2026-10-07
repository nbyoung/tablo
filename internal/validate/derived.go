package validate

import (
	"strings"

	"github.com/nbyoung/tablo/internal/model"
)

// Facts are the derived facts that the rules beyond the files read.
type Facts struct {
	Requirements []Condition           // one for each requires entry the Derivation resolves
	Junctions    map[Junction]Resolved // each task's junction at each gate that applies to it
	History      *History              // nil when the source has no commit
}

// Junction names the meeting of a task and a gate.
type Junction struct{ Task, Gate string }

// Resolved is a junction after inheritance and the defaults.
type Resolved struct {
	Recursive                    bool
	Contributor, Model, Reviewer string // a plain junction; Reviewer is "" where it has none
}

// Condition is the derived condition of one requires entry.
type Condition struct {
	Task     string // the terminating task
	Index    int    // the entry's index in Task.Requires
	Origin   string // the originating task's id
	From, To string // the two gates, resolved
	Stands   string // the gate the originating task stands at
	Met, Due bool
	MetAtTip bool // a cross-project entry that the tip of the originating project's trunk meets
}

// History is what one pass over the history of the source gives.
type History struct {
	Trunk    string   // the trunk's name; "" when it is undetermined
	Commits  []Commit // every commit with a method trailer, an event or a junction, newest first
	OffTrunk []Pin    // each linkage whose commit its subproject's trunk does not contain

	// Reviews holds the commit, in full, that accepts each junction with a
	// reviewer: each one a leaf's status passes, and each leaf's next one. A
	// junction with no entry, or with "", has no accepting commit yet. The
	// Derivation judges the commit by the files as they stand at it, and S11,
	// H4 and H5 read the answer here and search no trailer.
	Reviews map[Junction]string
}

// Commit is one commit of the history in view.
type Commit struct {
	Hash              string     // in full
	Author, Committer string     // the two emails
	Tableaux          string     // the language version.yaml states at the commit; "" when none
	Trailers          []Trailer  // Authorised, Reviewed, Reaffirmed and Model, in message order
	Events            []Event    // the events of the history view that the commit gives
	At                []Junction // the junctions the commit is at, as README.md "Junctions" defines it

	// Junctions holds, as the files stand at the commit, each junction of At
	// and each junction that a Reviewed: trailer of the commit names. The
	// files at a commit judge the commit, so H2, H3 and H6 read the reviewer,
	// the contributor and the model here and never from Facts.Junctions. Of
	// a junction with no entry the three rules say nothing.
	Junctions map[Junction]Resolved
}

// Trailer is one trailer of a commit: "Reviewed" and "9f31 design".
type Trailer struct{ Key, Value string }

// Event is one event of the history view; Kind is of the history schema's
// event enum.
type Event struct{ Task, Kind, Gate string }

// Pin is a linkage whose commit is off its subproject's trunk.
type Pin struct {
	Task  string
	Gate  string // the junction's gate; "" on a requirement
	Index int    // the requirement's index; -1 on a junction
	Trunk string // the subproject's trunk
}

// The keys of the method's four trailers, and the kind of the event a
// Reviewed: trailer gives.
const (
	keyAuthorised = "Authorised"
	keyReaffirmed = "Reaffirmed"
	keyReviewed   = "Reviewed"
	keyModel      = "Model"
	reviewedEvent = "reviewed"
)

// Derived applies the rules that read derived facts: R9 from f.Requirements,
// and with f.History also R13, P5, S11, J13 and H1 to H6. It returns nil when
// f is nil, when p has no .tableaux or no gates.yaml, and when the module does
// not accept p's version. With f.History nil it applies R9 alone.
func Derived(p *model.Project, f *Facts) []model.Diagnostic {
	if p == nil || f == nil || !p.Exists || p.Gating == nil || p.Version == nil || !p.Version.Accepted {
		return nil
	}
	r := &run{project: p, shape: newShape(p), linked: map[*model.Project]*shape{}, facts: f}
	r.r9()
	if f.History != nil {
		r.p5()
		r.j13()
		r.s11()
		wrong := r.trailers()
		r.models()
		r.h4(wrong)
		r.h5()
	}
	Sort(r.found)
	return unique(r.found)
}

// commit records a diagnostic that names a commit and, for H1, a trailer.
func (r *run) commit(code string, pos model.Pos, j Junction, hash, trailer, format string, args ...any) {
	r.add(code, pos, j.Task, j.Gate, format, args...)
	last := &r.found[len(r.found)-1]
	last.Commit, last.Trailer = hash, trailer
}

// reviewed returns a junction as the facts resolve it in view, when it is
// plain with a reviewer, and the task and the gate are the project's.
func (r *run) reviewed(j Junction) (Resolved, bool) {
	junction, ok := r.facts.Junctions[j]
	_, gate := r.shape.gate[j.Gate]
	return junction, ok && gate && r.project.Tasks[j.Task] != nil && !junction.Recursive && junction.Reviewer != ""
}

// stood returns a junction as the files resolve it at a commit, when it is
// plain there, and the task and the gate are the project's.
func (r *run) stood(c Commit, j Junction) (Resolved, bool) {
	junction, ok := c.Junctions[j]
	_, gate := r.shape.gate[j.Gate]
	return junction, ok && gate && r.project.Tasks[j.Task] != nil && !junction.Recursive
}

// standing is a leaf that S11, H4 and H5 read: its status, when it has a
// file, the gate it stands at and the gates that apply to it.
type standing struct {
	id     string
	status *model.Status // nil for a leaf with no status file
	gate   string        // undefined for a leaf with no status file
	gates  []string
}

// standings returns each leaf whose parent chain is whole and whose status gate,
// when it has a file, is a scalar that applies to it, in id order.
func (r *run) standings() []standing {
	var all []standing
	for _, id := range r.project.TaskIDs() {
		if r.shape.isParent(id) {
			continue
		}
		if _, whole := r.shape.ancestors(id); !whole {
			continue
		}
		at := standing{id: id, status: r.project.Statuses[id], gate: undefined, gates: r.shape.applicable(id)}
		if at.status != nil {
			if !at.status.Gate.Node.Scalar() {
				continue
			}
			at.gate = at.status.Gate.V
		}
		if indexOf(at.gates, at.gate) < 0 {
			continue
		}
		all = append(all, at)
	}
	return all
}

// next returns the junction of a leaf at the gate after the one it stands at.
func (at standing) next() (Junction, bool) {
	i := indexOf(at.gates, at.gate)
	if i+1 >= len(at.gates) {
		return Junction{}, false
	}
	return Junction{Task: at.id, Gate: at.gates[i+1]}, true
}

// handsOff reports whether the leaf's status states the reason review.
func (at standing) handsOff() bool {
	return at.status != nil && at.status.Reason.Node.Scalar() && at.status.Reason.V == "review"
}

// p5 warns of an undetermined trunk.
func (r *run) p5() {
	if r.facts.History.Trunk == "" {
		r.add("P5", model.Pos{File: "version.yaml"}, "", "",
			"the trunk is undetermined: version.yaml names none, refs/remotes/origin/HEAD is absent and the caller names no branch")
	}
}

// entry returns the requires entry a condition names, or nil.
func (r *run) entry(c Condition) *model.Requirement {
	task := r.project.Tasks[c.Task]
	if task == nil || c.Index < 0 || c.Index >= len(task.Requires) {
		return nil
	}
	return task.Requires[c.Index]
}

// r9 warns of a requirement that is due and unmet, and r13, with a history,
// of one that the tip of the originating project's trunk meets.
func (r *run) r9() {
	for _, c := range r.facts.Requirements {
		q := r.entry(c)
		if q == nil || q.Node == nil || !c.Due || c.Met {
			continue
		}
		r.add("R9", q.Node.Pos, c.Task, "", "the requirement on %s from %s is due at %s and unmet: %s stands at %s",
			c.Origin, c.From, c.To, c.Origin, c.Stands)
		if r.facts.History != nil {
			r.r13(c, q)
		}
	}
}

// r13 warns of a cross-project requirement that is unmet at its commit and
// met at the tip of the originating project's trunk.
func (r *run) r13(c Condition, q *model.Requirement) {
	if sub := q.Subproject; sub != nil && sub.Node != nil && c.MetAtTip {
		r.add("R13", posOf(r.project.Tasks[c.Task].File, sub.Commit.Node, sub.Node), c.Task, "",
			"the requirement on %s is unmet at its commit and met at the tip of the originating project's trunk", c.Origin)
	}
}

// j13 warns of each linkage whose commit is off its subproject's trunk.
func (r *run) j13() {
	for _, pin := range r.facts.History.OffTrunk {
		task := r.project.Tasks[pin.Task]
		if task == nil {
			continue
		}
		var sub *model.Subproject
		if pin.Gate != "" {
			if j := r.shape.entryAt(pin.Task, pin.Gate); j != nil {
				sub = j.Subproject
			}
		} else if pin.Index >= 0 && pin.Index < len(task.Requires) {
			sub = task.Requires[pin.Index].Subproject
		}
		if sub == nil || sub.Link == nil {
			continue
		}
		r.add("J13", posOf(task.File, sub.Commit.Node, sub.URL.Node, sub.Node), pin.Task, pin.Gate,
			"the commit %s of %s is not on its trunk, %s", sub.Link.Commit, sub.Link.URL, pin.Trunk)
	}
}

// s11 reports a status that passes a reviewed junction other than defined
// which no commit accepts. The accepting review is a fact: the rule reads
// History.Reviews and searches no trailer.
func (r *run) s11() {
	for _, at := range r.standings() {
		if at.status == nil {
			continue
		}
		for _, gate := range at.gates[:indexOf(at.gates, at.gate)+1] {
			j := Junction{Task: at.id, Gate: gate}
			junction, ok := r.reviewed(j)
			if gate == undefined || gate == "defined" || !ok || r.facts.History.Reviews[j] != "" {
				continue
			}
			r.add("S11", at.status.Gate.Node.Pos, at.id, gate,
				"the status passes %s, whose reviewer is %s, and no commit by the reviewer carries Reviewed: %s %s",
				gate, junction.Reviewer, at.id, gate)
		}
	}
}

// review names a Reviewed: trailer: its commit and the junction it names.
type review struct {
	hash string
	at   Junction
}

// trailers applies H1 to every task trailer of the history and H2 to every
// Reviewed: trailer that passes it. It returns the reviews H2 reports, which
// H4 sets aside.
func (r *run) trailers() map[review]bool {
	wrong := map[review]bool{}
	for _, c := range r.facts.History.Commits {
		for _, trailer := range c.Trailers {
			if j, ok := r.h1(c, trailer); ok && r.h2(c, j) {
				wrong[review{c.Hash, j}] = true
			}
		}
	}
	return wrong
}

// h1 warns of a task trailer that names nothing: a value that is not one
// word, or two for Reviewed; a task the project lacks; a gate it lacks or one
// that does not apply to the task. It stays silent about an unread task. It
// returns the junction a Reviewed: trailer names when the trailer passes.
func (r *run) h1(c Commit, trailer Trailer) (Junction, bool) {
	want := 1
	switch trailer.Key {
	case keyReviewed:
		want = 2
	case keyAuthorised, keyReaffirmed:
	default:
		return Junction{}, false
	}
	warn := func(format string, args ...any) {
		r.commit("H1", model.Pos{}, Junction{}, c.Hash, trailer.Key+": "+trailer.Value, format, args...)
	}
	words := strings.Fields(trailer.Value)
	if len(words) != want {
		warn("the trailer is malformed")
		return Junction{}, false
	}
	id := words[0]
	task := r.project.Tasks[id] != nil
	known := task || r.shape.unread[id]
	if want == 1 {
		if !known {
			warn("%s names no task", id)
		}
		return Junction{}, false
	}
	gate := words[1]
	_, named := r.shape.gate[gate]
	switch {
	case !known && !named:
		warn("%s names no task and %s names no gate", id, gate)
	case !known:
		warn("%s names no task", id)
	case !named:
		warn("%s names no gate", gate)
	case task && !r.shape.appliesTo(id, gate):
		warn("%s does not apply to %s", gate, id)
	case task:
		return Junction{Task: id, Gate: gate}, true
	}
	return Junction{}, false
}

// h2 warns of a review from a hand that is not the reviewer's: a Reviewed:
// trailer at a junction with a reviewer, on a commit that the reviewer
// neither authors nor commits. The reviewer is the junction's as the files
// stand at the commit. It stays silent at a junction with no reviewer.
func (r *run) h2(c Commit, j Junction) bool {
	junction, ok := r.stood(c, j)
	if !ok || junction.Reviewer == "" || c.Author == junction.Reviewer || c.Committer == junction.Reviewer {
		return false
	}
	r.commit("H2", model.Pos{}, j, c.Hash, "", "%s is not the reviewer of the %s junction; %s is", c.Author, j.Gate, junction.Reviewer)
	return true
}

// models applies H3 and H6 to every commit at a plain junction that states a
// model as the files stand at the commit.
func (r *run) models() {
	for _, c := range r.facts.History.Commits {
		var models []string
		for _, trailer := range c.Trailers {
			if trailer.Key == keyModel {
				models = append(models, strings.TrimSpace(trailer.Value))
			}
		}
		for _, j := range c.At {
			junction, ok := r.stood(c, j)
			if !ok || junction.Model == "" {
				continue
			}
			r.h3(c, j, junction, models)
			r.h6(c, j, junction, models)
		}
	}
}

// h3 warns of a Model: trailer whose value does not start with the model the
// junction states, once for the commit and the junction, with the first such
// value.
func (r *run) h3(c Commit, j Junction, junction Resolved, models []string) {
	for _, ran := range models {
		if !strings.HasPrefix(ran, junction.Model) {
			r.commit("H3", model.Pos{}, j, c.Hash, "", "Model %s is outside the stated model %s", ran, junction.Model)
			return
		}
	}
}

// h6 warns of a commit by the junction's contributor with no Model: trailer,
// unless version.yaml at the commit states a language before 0.2.1.
func (r *run) h6(c Commit, j Junction, junction Resolved, models []string) {
	if len(models) > 0 || c.Author != junction.Contributor {
		return
	}
	if major, minor, patch, ok := model.ParseVersion(c.Tableaux); ok && (major == 0 && (minor < 2 || (minor == 2 && patch < 1))) {
		return // the language has no Model trailer before 0.2.1
	}
	r.commit("H6", model.Pos{}, j, c.Hash, "",
		"the contributor's commit at the %s junction carries no Model: trailer; the junction states %s", j.Gate, junction.Model)
}

// h4 reports, as information, a hand-off the history implies: a leaf whose
// next junction has a reviewer and no accepting commit yet, whose status
// states no review, and whose newest event is the contributor's. A review
// that H2 reports is no event here.
func (r *run) h4(wrong map[review]bool) {
	for _, at := range r.standings() {
		j, ok := at.next()
		if !ok || at.handsOff() {
			continue
		}
		junction, ok := r.reviewed(j)
		if !ok || r.facts.History.Reviews[j] != "" {
			continue
		}
		for _, c := range r.facts.History.Commits {
			if !r.concerns(c, at.id, wrong) {
				continue
			}
			if c.Author == junction.Contributor {
				file := r.project.Tasks[at.id].File
				if at.status != nil {
					file = at.status.File
				}
				r.commit("H4", model.Pos{File: file.Path}, j, c.Hash, "",
					"the newest event is the contributor's and the status states no review; %s may have %s to review",
					junction.Reviewer, j.Gate)
			}
			break
		}
	}
}

// concerns reports whether a commit holds an event of a task, a review that
// H2 reports aside.
func (r *run) concerns(c Commit, id string, wrong map[review]bool) bool {
	for _, event := range c.Events {
		if event.Task != id {
			continue
		}
		if event.Kind == reviewedEvent && wrong[review{c.Hash, Junction{Task: id, Gate: event.Gate}}] {
			continue
		}
		return true
	}
	return false
}

// h5 warns of a stale hand-off: a status that still states the reason review
// after the commit that accepts its next junction.
func (r *run) h5() {
	for _, at := range r.standings() {
		j, ok := at.next()
		if !ok || !at.handsOff() {
			continue
		}
		junction, ok := r.reviewed(j)
		accepts := r.facts.History.Reviews[j]
		if !ok || accepts == "" {
			continue
		}
		r.commit("H5", at.status.Reason.Node.Pos, j, accepts, "",
			"the status still states review after %s accepted %s", junction.Reviewer, j.Gate)
	}
}
