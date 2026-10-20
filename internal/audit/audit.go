// Package audit turns the Validator's diagnostics and three facts of the
// Derivation into findings: each names its task, its gate, its commits, the
// act that resolves it and the person who takes it, and holds the facts of
// the Derivation it rests on. It is a pure function of its input: it runs no
// process and reads no file, clock or environment.
package audit

import (
	"cmp"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/internal/validate"
)

// Act is the kind of action that resolves a finding.
type Act string

// The acts: the ten of design dada, then the two that VIEWS.md#audit adds at
// 0.4.0, for R14 and J19.
const (
	Authorise     Act = "authorise"      // an authority accepts the task
	Review        Act = "review"         // the reviewer accepts the junction
	Reaffirm      Act = "reaffirm"       // the recorder confirms the status
	RecordHandoff Act = "record_handoff" // the contributor states the reason review
	ClearHandoff  Act = "clear_handoff"  // the contributor records the status at the gate
	Revise        Act = "revise"         // someone revises the file
	MovePin       Act = "move_pin"       // someone moves the pin or the commit field
	Checkout      Act = "checkout"       // whoever holds the clone gives the tool the subproject
	Await         Act = "await"          // the originating task passes the gate a requirement names
	StateCommit   Act = "state_commit"   // someone states the commit an unpinned requirement lacks
	RaiseLanguage Act = "raise_language" // the subproject raises its version.yaml to the project's language
	None          Act = "none"           // nothing resolves it
)

// The keys of the three kinds of finding that no rule states.
const (
	Proposed   = "proposed"
	Stale      = "stale"
	SoleReview = "sole_review"
)

// DefaultStale is the age in days beyond which a status is stale.
const DefaultStale = 7

// Source says where a sentence stands: "README.md, Status" and its address.
type Source struct{ Text, URL string }

// Input is what one audit reads.
type Input struct {
	Facts       *derive.Facts      // the facts of the project in view
	Diagnostics []model.Diagnostic // the Loader's and the Validator's, as snapshot.go sorts them
	Now         string             // the date staleness counts from, YYYY-MM-DD; required
	Stale       int                // the age in days; 0 is DefaultStale
}

// A DateError reports an Input whose Now is empty or no date of the form
// YYYY-MM-DD. snapshot.go returns it as a *tablo.UsageError.
type DateError struct{ Now string }

// Error says what the date lacks.
func (e *DateError) Error() string {
	if e.Now == "" {
		return "the audit needs the date staleness counts from, as YYYY-MM-DD"
	}
	return "the date " + e.Now + " is not of the form YYYY-MM-DD"
}

// Report is the audit of one project at one source. It is immutable.
type Report struct {
	Now      string    // the date staleness counts from, as given
	Stale    int       // the age in days, as resolved
	Findings []Finding // one per diagnostic, proposed task, stale status and sole review, in the order of the design

	history bool // the run has a pass to read: it examines the rules of the history
	trunk   bool // the run has a trunk: it examines the proposed tasks
}

// Finding is one disagreement between the files and the history, of one task
// or of none.
type Finding struct {
	Key      string         // the rule id, or Proposed, Stale or SoleReview
	Rule     string         // the rule id of RULES.md; "" for the three kinds
	Severity model.Severity // as the audit lists it: the rule's, or Information after the demotion
	Demoted  bool           // Severity is Information and the rule's is graver
	Kind     string         // the rule's title, or the kind's
	Task     string         // "" when the finding names none
	Gate     string         // "" when the finding names none
	File     string         // relative to the project directory, .tableaux/…; "" for none
	Message  string         // a rule finding: the diagnostic's, byte for byte
	Act      Act
	Resolver string // an email; "" when the files name nobody
	Sentence string // the sentence of the method the kind rests on
	Source   Source // where the sentence stands
	Facts    Facts
	Commits  []*history.Commit // the commits involved, each once, the oldest first
	Commands []Command         // what shows the finding, then what resolves it

	Diagnostic *model.Diagnostic // the diagnostic a rule finding carries, its severity the rule's own; nil for the three kinds
}

// Facts are the typed facts of one finding: values of the Derivation, never
// copies and never words. A fact the finding does not carry is nil.
type Facts struct {
	Junction      *derive.Junction      // Facts.Junction(task, gate)
	Status        *derive.Status        // Facts.Status(task)
	Requirement   *derive.Condition     // the entry of Facts.Requires(task) the diagnostic stands at
	Authorisation *derive.Authorisation // Facts.Authorisation(task)
	Snapshot      *derive.Snapshot      // Junction.Snapshot: the link, whose pin is Facts.Pin(Snapshot.Link)
	Reading       *derive.Reading       // the model check of the finding's commit at its gate
}

// Command is a line a reader runs, and what it does: "shows the finding" or
// "resolves it".
type Command struct{ Text, Comment string }

// dateForm is the form of a date: of Now, and of a status's.
const dateForm = "2006-01-02"

// planDir is the directory a finding's file stands in, with its slash.
const planDir = ".tableaux/"

// run is one audit under way.
type run struct {
	f        *derive.Facts
	now      time.Time      // Input.Now, at midnight UTC
	stale    int            // the age in days, resolved
	place    map[string]int // a task's place in display order
	findings []Finding
}

// Run audits a project. It returns a *DateError for a Now that is no date,
// before it reads anything else; nil and no error when in.Facts is nil or
// refuses; and never fails or panics otherwise. It changes nothing of its
// input, and the findings point into in.Diagnostics and into the facts.
func Run(in Input) (*Report, error) {
	now, err := time.Parse(dateForm, in.Now)
	if err != nil {
		return nil, &DateError{Now: in.Now}
	}
	if in.Facts == nil || in.Facts.Refused() != nil {
		return nil, nil
	}
	r := &run{f: in.Facts, now: now, stale: in.Stale, place: map[string]int{}}
	if r.stale == 0 {
		r.stale = DefaultStale
	}
	for i, id := range r.f.Order() {
		r.place[id] = i
	}
	for i := range in.Diagnostics {
		r.wrap(&in.Diagnostics[i])
	}
	r.proposed()
	r.stales()
	r.soleReviews()
	for i := range r.findings {
		r.attach(&r.findings[i])
	}
	for i := range r.findings {
		r.demote(&r.findings[i])
	}
	arrange(r.f, r.findings)
	return &Report{
		Now: in.Now, Stale: r.stale, Findings: r.findings,
		history: r.f.Log() != nil, trunk: r.f.Trunk().Tip != "",
	}, nil
}

// wrap makes the finding of one diagnostic, by the rule's row of the table.
// A code the table lacks resolves as most rules do: its keeper revises.
func (r *run) wrap(d *model.Diagnostic) {
	x := Finding{
		Key: d.Code, Rule: d.Code, Severity: d.Severity, Task: d.Task, Gate: d.Gate,
		Message: d.Message, Act: Revise, Diagnostic: d,
	}
	if d.Pos.File != "" {
		x.File = planDir + d.Pos.File
	}
	if known, ok := validate.RuleOf(d.Code); ok {
		x.Kind = known.Title
	}
	who := keeper
	if row, ok := table[d.Code]; ok {
		x.Act, who, x.Sentence, x.Source = row.act, row.who, row.sentence, row.source
	}
	switch d.Code {
	case "J8":
		// Nothing in the files mends a clone the host lacks.
		if link := r.link(d); link != nil && (link.Problem == model.NoClone || link.Problem == model.CommitAbsent) {
			x.Act, who = Checkout, nobody
		}
	case "H2":
		// A review accepts the junction since: the commit stands, and nothing resolves it.
		if review := r.f.Accepted(d.Task, d.Gate); review != nil && review.Commit != nil {
			x.Act, who = None, author
		}
	}
	x.Resolver = r.resolve(who, d)
	r.findings = append(r.findings, x)
}

// complete is the one state the audit names: a complete task waits for
// nothing, and each of its junctions is historical.
const complete = "complete"

// raise makes a finding of one of the three kinds no rule states.
func (r *run) raise(key string, severity model.Severity, id, gate, file, message, resolver string) {
	kind := kinds[key]
	r.findings = append(r.findings, Finding{
		Key: key, Severity: severity, Kind: kind.kind, Task: id, Gate: gate, File: planDir + file + "/" + id + ".yaml",
		Message: message, Act: kind.act, Resolver: resolver, Sentence: kind.sentence, Source: kind.source,
	})
}

// proposed raises a warning for each proposed task, in display order. On
// the trunk it is each task no authority accepts. Off the trunk it is each
// task whose file differs from the one at the trunk's tip, so a branch
// reports the tasks it adds or changes and not the plan it inherits. With
// no history and with no trunk it raises none: rule P5 already says that
// every task reads as proposed.
func (r *run) proposed() {
	if r.f.Log() == nil || r.f.Trunk().Tip == "" {
		return
	}
	for _, id := range r.f.Order() {
		a := r.f.Authorisation(id)
		if a == nil || a.Authorised {
			continue
		}
		message := "The task stands proposed: "
		switch {
		case a.Why == derive.Determined && a.Commit != nil:
			message += a.Commit.Author.Email + ", the author of its deciding commit, is none of its authorities"
		case a.Why == derive.NoCommit:
			message += "no commit on the trunk holds its file"
		case a.Why == derive.OffTrunk && a.Differs:
			message += "the source changes its file and is off the trunk `" + r.f.Trunk().Name + "`"
		default:
			continue
		}
		// A new Authorised: commit reads the tree as it then stands; for the
		// root the judge of the deciding commit is the outgoing owner.
		resolver := r.f.Owner()
		if list := r.f.Authorities(id); len(list) > 0 {
			resolver = list[0].Email
		} else if len(a.Judges) > 0 {
			resolver = a.Judges[0]
		}
		r.raise(Proposed, model.Warning, id, "", "tasks", message, resolver)
	}
}

// days returns the days from a date to Now. Both are calendar dates, so the
// count takes no zone. ok is false for a text that is no date.
func (r *run) days(date string) (days int, ok bool) {
	then, err := time.Parse(dateForm, date)
	if err != nil {
		return 0, false
	}
	return int(r.now.Sub(then).Hours() / 24), true
}

// stales raises a warning for each stale status, in display order: a leaf's
// recorded status that a commit records, that is not complete and whose
// date is more than the age before Now. A leaf with no file has recorded
// nothing to confirm, a snapshot's date is the subproject's, and an edit of
// the working tree is no commit yet.
func (r *run) stales() {
	for _, id := range r.f.Order() {
		s := r.f.Status(id)
		if !r.f.Leaf(id) || s == nil || s.Kind != derive.Recorded || s.Commit == nil || s.Uncommitted || s.State == complete {
			continue
		}
		if days, ok := r.days(s.Date); !ok || days <= r.stale {
			continue
		}
		message := "The status dates from " + s.Date + ", more than " + strconv.Itoa(r.stale) + " days before " + r.now.Format(dateForm)
		r.raise(Stale, model.Warning, id, s.Gate, "status", message, r.recorder(id))
	}
}

// soleReviews raises, as information, each junction nobody but its
// contributor reviews, by leaf in display order and then by gate: a plain
// junction that is not historical, whose reviewer is its contributor or
// which states none, and whose review the authorisation does not stand for.
// A parent gives none, since its junctions are defaults.
func (r *run) soleReviews() {
	for _, id := range r.f.Order() {
		if !r.f.Leaf(id) {
			continue
		}
		for _, j := range r.f.Junctions(id) {
			if j.Kind != model.Plain || (j.Reviewer.V != "" && j.Reviewer.V != j.Contributor.V) {
				continue
			}
			if historical(r.f, id, j.Gate) || r.f.ByAuthorisation(id, j.Gate) {
				continue
			}
			r.raise(SoleReview, model.Information, id, j.Gate, "tasks", "Nobody but the contributor reviews the work at the gate", r.authority(id))
		}
	}
}

// historical reports whether the junction at the gate is one of the task's
// historical junctions, or the task is complete: the one predicate of the
// demotion and of a sole review. A parent reads by its roll-up and a
// snapshot by the task it reads. The junction at the gate the status names
// is not historical.
func historical(f *derive.Facts, task, gate string) bool {
	s := f.Status(task)
	if s == nil {
		return false
	}
	if s.State == complete {
		return true
	}
	at := f.Index(gate)
	return at >= 0 && at < f.Index(s.Gate)
}

// demote lists as information a finding nothing resolves once it is past:
// the task has passed its junction; or it names a task and no gate of the
// project, and the task is complete; or it names no task of the project, and
// its commit is older than the age a stale status takes. Such a finding
// with no commit in the pass has no date to age by and keeps the rule's
// severity. The diagnostic keeps the rule's own severity, so the envelope's
// diagnostics and the exit code do not change. The audit guesses no
// junction.
func (r *run) demote(x *Finding) {
	if x.Act != None || x.Severity >= model.Information {
		return
	}
	switch {
	case r.task(x.Task) == nil:
		if x.Diagnostic == nil {
			return
		}
		c := r.commit(x.Diagnostic.Commit)
		if c == nil {
			return
		}
		if days, ok := r.days(c.Date()); !ok || days <= r.stale {
			return
		}
	case r.f.Index(x.Gate) >= 0:
		if !historical(r.f, x.Task, x.Gate) {
			return
		}
		x.Facts.Status = r.f.Status(x.Task) // the status says so
	default:
		s := r.f.Status(x.Task)
		if s == nil || s.State != complete {
			return
		}
		x.Facts.Status = s
	}
	x.Severity, x.Demoted = model.Information, true
}

// rank places a key among the kinds: the rules in the order of
// validate.Rules, then the three kinds no rule states.
var rank = func() map[string]int {
	at := map[string]int{}
	for i, known := range validate.Rules {
		at[known.ID] = i
	}
	for i, key := range []string{Proposed, Stale, SoleReview} {
		at[key] = len(validate.Rules) + 1 + i
	}
	return at
}()

// arrange sorts findings into the order of the design: the listed severity,
// errors first; the kind; the gate, none first, then the order of
// gates.yaml, then any other key by bytes; the task, none first, then
// display order, then any other id by bytes; the oldest commit, none first,
// then the oldest first; the diagnostic's file, line and column; the
// message; the trailer. The sort is stable over the order given. A code the
// table lacks sorts after the rules, by bytes.
func arrange(f *derive.Facts, findings []Finding) {
	place := map[string]int{}
	for i, id := range f.Order() {
		place[id] = i
	}
	kind := func(x *Finding) int {
		if at, ok := rank[x.Key]; ok {
			return at
		}
		return len(validate.Rules)
	}
	// among places a key: none, then one of a list by its index, then any other.
	among := func(key string, index int, listed bool) (class, at int) {
		switch {
		case key == "":
			return 0, 0
		case listed:
			return 1, index
		}
		return 2, 0
	}
	gate := func(x *Finding) (int, int) {
		at := f.Index(x.Gate)
		return among(x.Gate, at, at >= 0)
	}
	task := func(x *Finding) (int, int) {
		at, listed := place[x.Task]
		return among(x.Task, at, listed)
	}
	// oldest places a finding by its oldest commit: none first, then the
	// oldest first. A pass counts its commits from the newest.
	oldest := func(x *Finding) (int, int) {
		if len(x.Commits) == 0 {
			return 0, 0
		}
		return 1, -x.Commits[0].Seq
	}
	diagnostic := func(x *Finding) model.Diagnostic {
		if x.Diagnostic == nil {
			return model.Diagnostic{}
		}
		return *x.Diagnostic
	}
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := &findings[i], &findings[j]
		ga, gi := gate(a)
		gb, gj := gate(b)
		ta, ti := task(a)
		tb, tj := task(b)
		ca, ci := oldest(a)
		cb, cj := oldest(b)
		da, db := diagnostic(a), diagnostic(b)
		return cmp.Or(
			cmp.Compare(a.Severity, b.Severity),
			cmp.Compare(kind(a), kind(b)), cmp.Compare(a.Key, b.Key),
			cmp.Compare(ga, gb), cmp.Compare(gi, gj), cmp.Compare(a.Gate, b.Gate),
			cmp.Compare(ta, tb), cmp.Compare(ti, tj), cmp.Compare(a.Task, b.Task),
			cmp.Compare(ca, cb), cmp.Compare(ci, cj),
			cmp.Compare(da.Pos.File, db.Pos.File), cmp.Compare(da.Pos.Line, db.Pos.Line), cmp.Compare(da.Pos.Col, db.Pos.Col),
			cmp.Compare(a.Message, b.Message),
			cmp.Compare(da.Trailer, db.Trailer),
		) < 0
	})
}

// task returns a task of the tree, or nil.
func (r *run) task(id string) *model.Task {
	if _, ok := r.place[id]; !ok {
		return nil
	}
	return r.f.Project().Tasks[id]
}

// resolve returns the email of the person a role names for a diagnostic, or
// "" where the files name nobody. Each role falls back as the design states,
// so a diagnostic whose task, gate or commit the project lacks still names
// someone.
func (r *run) resolve(who role, d *model.Diagnostic) string {
	switch who {
	case authority:
		return r.authority(d.Task)
	case assignee:
		return r.assignee(d.Task)
	case keeper:
		return r.keeper(d.Pos.File)
	case recorder:
		return r.recorder(d.Task)
	case contributor:
		return r.contributor(d.Task, d.Gate)
	case reviewer:
		return r.reviewer(d.Task, d.Gate)
	case author:
		return r.author(d.Commit)
	case origin:
		return r.origin(d)
	case nobody:
		return ""
	}
	return r.f.Owner()
}

// authority returns the task's nearest authority, and the owner for the
// root and for an id that names no task.
func (r *run) authority(id string) string {
	if list := r.f.Authorities(id); len(list) > 0 {
		return list[0].Email
	}
	return r.f.Owner()
}

// assignee returns the task's assignee as its file states it, and the owner
// for an id that names no task and for a task with none.
func (r *run) assignee(id string) string {
	if task := r.task(id); task != nil && task.Assignee.V != "" {
		return task.Assignee.V
	}
	return r.f.Owner()
}

// keeper returns who keeps a file below .tableaux: the authority of the task
// a task file states, since an authority's commit is proposal and acceptance
// at once; the recorder of the task a status file states; and the owner for
// any other file and for none.
func (r *run) keeper(file string) string {
	dir, name := path.Split(file)
	if id, isYAML := strings.CutSuffix(name, ".yaml"); isYAML && r.task(id) != nil {
		switch dir {
		case "tasks/":
			return r.authority(id)
		case "status/":
			return r.recorder(id)
		}
	}
	return r.f.Owner()
}

// recorder returns who records the task's status: the recorder of the
// status, and the contributor of the task's next junction when it has none.
func (r *run) recorder(id string) string {
	s := r.f.Status(id)
	if s == nil {
		return r.assignee(id)
	}
	if s.Recorder != "" {
		return s.Recorder
	}
	return r.contributor(id, r.f.Next(id, s.Gate))
}

// contributor returns the contributor of a plain junction, else the assignee.
func (r *run) contributor(id, gate string) string {
	if j := r.f.Junction(id, gate); j != nil && j.Kind == model.Plain && j.Contributor.V != "" {
		return j.Contributor.V
	}
	return r.assignee(id)
}

// reviewer returns the reviewer of a junction, else the assignee.
func (r *run) reviewer(id, gate string) string {
	if j := r.f.Junction(id, gate); j != nil && j.Kind == model.Plain && j.Reviewer.V != "" {
		return j.Reviewer.V
	}
	return r.assignee(id)
}

// commit returns a commit of the pass, or nil.
func (r *run) commit(id string) *history.Commit {
	if log := r.f.Log(); log != nil && id != "" {
		return log.Commit(id)
	}
	return nil
}

// author returns the author's email of a commit of the pass, and the owner
// when the pass holds no such commit.
func (r *run) author(id string) string {
	if c := r.commit(id); c != nil {
		return c.Author.Email
	}
	return r.f.Owner()
}

// origin returns whoever works on the originating task of the requirement a
// diagnostic stands at, in that task's own project: the first status of its
// chain whose task is a leaf there gives the contributor of that leaf's next
// junction when it is plain, else the leaf's assignee. With no condition or
// no such leaf it is the assignee of the diagnostic's task.
func (r *run) origin(d *model.Diagnostic) string {
	if c := r.condition(d); c != nil && c.Facts != nil {
		there := c.Facts
		for _, s := range there.Chain(c.Origin) {
			if !there.Leaf(s.Task) {
				continue
			}
			if j := there.Junction(s.Task, there.Next(s.Task, s.Gate)); j != nil && j.Kind == model.Plain && j.Contributor.V != "" {
				return j.Contributor.V
			}
			return there.Project().Tasks[s.Task].Assignee.V
		}
	}
	return r.assignee(d.Task)
}

// condition returns the requirement a diagnostic stands at: the entry of its
// task whose node, or whose subproject, url or commit node, has the
// diagnostic's position, since a diagnostic carries no index. It is nil
// when no entry stands there.
func (r *run) condition(d *model.Diagnostic) *derive.Condition {
	if d.Pos.File == "" || d.Pos.Line == 0 {
		return nil
	}
	for _, c := range r.f.Requires(d.Task) {
		nodes := []*model.Value{c.Entry.Node}
		if sub := c.Entry.Subproject; sub != nil {
			nodes = append(nodes, sub.Node, sub.URL.Node, sub.Commit.Node)
		}
		for _, node := range nodes {
			if node != nil && node.Pos == d.Pos {
				return c
			}
		}
	}
	return nil
}

// link returns the link a diagnostic concerns: that of the snapshot its
// junction reads when it names a gate, and that of the requirement it stands
// at when it names none. It is nil when neither is found.
func (r *run) link(d *model.Diagnostic) *model.Link {
	if d.Gate != "" {
		if j := r.f.Junction(d.Task, d.Gate); j != nil && j.Snapshot != nil {
			return j.Snapshot.Link
		}
		return nil
	}
	if c := r.condition(d); c != nil {
		return c.Link
	}
	return nil
}
