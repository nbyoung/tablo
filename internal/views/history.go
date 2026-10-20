package views

import (
	"strings"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// The words of a line's effect: what the line does to its task's acceptance
// or review; "" for nothing.
const (
	effectAuthorisedCommit  = "authorised_commit"
	effectAuthorisedMerge   = "authorised_merge"
	effectAuthorisedTrailer = "authorised_trailer"
	effectProposed          = "proposed"
	effectHandoff           = "handoff"
	effectAccepts           = "accepts"
	effectStands            = "stands"
	effectNone              = "none"
)

// eventKinds are the six kinds of SYNTAX.md#history, in its order.
var eventKinds = []derive.EventKind{derive.TaskEvent, derive.Authorised, derive.StatusSet, derive.Reaffirmed, derive.Reviewed, derive.Pinned}

// History returns the history of the subtree under q.Task, the root when it
// is "": each event of f.History() whose task lies there, whose author is
// q.Person when it names one, and that the range from q.From holds when it
// names one, joined into lines. An event taken from a subproject joins from
// Detail on. It fails in no case.
func History(f *derive.Facts, q Query) *view.History {
	b := builder{f, q}
	subject := q.Task
	if subject == "" {
		subject = f.Root()
	}
	in := map[string]bool{}
	for _, id := range b.subtree(subject) {
		in[id] = true
	}
	out := &view.History{Legend: b.legend(), Subject: b.ref(subject), Lines: []view.Line{}}
	var group []*derive.Event
	var at lineKey
	for _, e := range f.History() {
		switch {
		case !in[e.Task]:
			continue
		case e.Sub != "" && q.Level < Detail:
			continue
		case q.Person != "" && e.Commit.Author.Email != q.Person:
			continue
		case !b.inRange(e):
			continue
		}
		key := lineKey{commit: e.Commit.ID, task: e.Task, sub: e.Sub}
		if e.Sub != "" && e.Link != nil {
			key.url = e.Link.URL
		}
		if len(group) > 0 && key != at {
			out.Lines = append(out.Lines, b.line(subject, group))
			group = nil
		}
		at = key
		group = append(group, e)
	}
	if len(group) > 0 {
		out.Lines = append(out.Lines, b.line(subject, group))
	}
	b.count(out)
	if q.Level >= Provenance {
		out.HistoryProvenance = &view.HistoryProvenance{Commands: b.historyCommands(subject)}
	}
	return out
}

// lineKey is what the events of one line share: the commit, the task, and
// for a taken event the subproject task and the link's url.
type lineKey struct{ commit, task, sub, url string }

// inRange reports whether the range from q.From holds an event: always
// without a start; for an event of the project, when f.Log().Reaches(From,
// commit) is false; for a taken event, when the commit the link reads at the
// start does not reach it in the subproject's pass, or when no link reads a
// commit at the start.
func (b builder) inRange(e *derive.Event) bool {
	from := b.q.From
	if from == "" {
		return true
	}
	if e.Sub == "" {
		log := b.f.Log()
		return log == nil || !log.Reaches(from, e.Commit.ID)
	}
	pinned := b.pinnedAt(from, e.Task, e.Link)
	if pinned == "" {
		return true
	}
	log := b.f.Sub(e.Link).Log()
	return log == nil || !log.Reaches(pinned, e.Commit.ID)
}

// pinnedAt returns the commit a task's link to a subproject reads as the
// files stand at a commit of the project: the Where.Commit of the facts of
// the snapshot of f.At(commit).Snapshots(task) with the link's url; "" for
// none.
func (b builder) pinnedAt(commit, task string, link *model.Link) string {
	if link == nil {
		return ""
	}
	for _, n := range b.f.At(commit).Snapshots(task) {
		if n.Link != nil && n.Link.URL == link.URL && n.Facts != nil && n.Facts.Project() != nil {
			return n.Facts.Project().Where.Commit
		}
	}
	return ""
}

// line returns one line from its events, with the subject's id.
func (b builder) line(subject string, events []*derive.Event) view.Line {
	first := events[0]
	c := first.Commit
	l := view.Line{Task: b.ref(first.Task), Events: make([]view.Event, 0, len(events))}
	for _, e := range events {
		l.Events = append(l.Events, event(e))
	}
	// The project the line comes from, and its task there.
	from, task := b.f, first.Task
	if first.Sub != "" {
		from, task = b.f.Sub(first.Link), first.Sub
	} else if log := b.f.Log(); !b.f.OnTrunk() && log != nil && log.Trunk.Tip != "" {
		l.Proposal = !log.Reaches(log.Trunk.Tip, c.ID)
	}
	if b.q.Level < Detail {
		return l
	}
	d := &view.LineDetail{Effect: effect(from, task, events)}
	if values := c.Values("Model"); len(values) > 0 {
		d.Model = values[0]
	}
	if c.Committer.Email != c.Author.Email {
		d.Committer = c.Committer.Email
	}
	d.CoAuthor = from.CoAuthor(c)
	d.After = b.statusAt(from, c.ID, task)
	for _, parent := range c.Parents {
		if from.At(parent) != nil && sameStatus(b.statusAt(from, parent, task), d.After) {
			d.Unchanged = true
			break
		}
	}
	if first.Sub == "" && first.Task != subject {
		d.SubjectAfter = b.statusAt(b.f, c.ID, subject)
	}
	for _, e := range events {
		if (e.Kind == derive.StatusSet || e.Kind == derive.Reaffirmed) && e.Reading != nil && e.Reading.Verdict != derive.NoModel {
			d.Check = &view.ModelCheck{Commit: c.ID, Gate: e.Reading.Gate, Stated: e.Reading.Stated, Trailer: e.Reading.Trailer,
				Reading: reading(e.Reading.Verdict), CoAuthor: e.Reading.CoAuthor}
			break
		}
	}
	l.LineDetail = d
	if b.q.Level >= Provenance {
		l.LineProvenance = &view.LineProvenance{Commit: b.commit(c)}
	}
	return l
}

// statusAt returns a task's status as the files of the project f holds stand
// at a commit of its pass, or nil where those files lack the task. The files
// at a commit state no date and no recorder.
func (b builder) statusAt(f *derive.Facts, commit, task string) *view.Status {
	past := f.At(commit)
	s := past.Status(task)
	if s == nil {
		return nil
	}
	out := b.status(past, s)
	return &out
}

// sameStatus reports whether two statuses agree in kind, gate, state, reason
// and note; two nils agree.
func sameStatus(x, y *view.Status) bool {
	if x == nil || y == nil {
		return x == y
	}
	return x.Kind == y.Kind && x.Gate == y.Gate && x.State == y.State && x.Reason == y.Reason && x.Note == y.Note
}

// effect returns a line's effect by the table of model.md, reading the
// facts of the project the line comes from.
func effect(f *derive.Facts, task string, events []*derive.Event) string {
	has := map[derive.EventKind]bool{}
	for _, e := range events {
		has[e.Kind] = true
	}
	commit := events[0].Commit.ID
	if has[derive.TaskEvent] || has[derive.Authorised] {
		a := f.AuthorisationAt(task, commit)
		if a != nil && a.Authorised && a.Commit != nil && a.Commit.ID == commit {
			switch a.Way {
			case derive.ByCommit:
				return effectAuthorisedCommit
			case derive.ByMerge:
				return effectAuthorisedMerge
			case derive.ByTrailer:
				return effectAuthorisedTrailer
			}
		}
		undetermined := a != nil && !a.Authorised && (a.Why == derive.NoHistory || a.Why == derive.NoTrunk)
		switch {
		case undetermined:
		case has[derive.TaskEvent] && (a == nil || !a.Authorised):
			return effectProposed
		case !has[derive.TaskEvent]:
			return effectNone
		}
	}
	for _, e := range events {
		if e.Kind == derive.StatusSet && e.Reason == "review" {
			return effectHandoff
		}
	}
	for _, e := range events {
		if e.Kind != derive.Reviewed {
			continue
		}
		switch e.Effect {
		case derive.Accepts:
			return effectAccepts
		case derive.AtDefined:
			return effectStands
		}
		return effectNone
	}
	return ""
}

// count fills Events, Commits, Kinds and Days from the project's own lines.
func (b builder) count(h *view.History) {
	kinds := map[derive.EventKind]int{}
	commits := map[string]bool{}
	type day struct {
		events  int
		commits map[string]bool
		kinds   map[derive.EventKind]int
	}
	days := map[string]*day{}
	var dates []string
	for _, l := range h.Lines {
		if l.Events[0].Subproject != nil {
			continue
		}
		date, commit := l.Events[0].Date, l.Events[0].Commit
		d := days[date]
		if d == nil {
			d = &day{commits: map[string]bool{}, kinds: map[derive.EventKind]int{}}
			days[date] = d
			dates = append(dates, date)
		}
		for _, e := range l.Events {
			h.Events++
			d.events++
			kinds[derive.EventKind(e.Kind)]++
			d.kinds[derive.EventKind(e.Kind)]++
		}
		commits[commit] = true
		d.commits[commit] = true
	}
	h.Commits = len(commits)
	h.Kinds = make([]view.EventCount, 0, len(eventKinds))
	for _, k := range eventKinds {
		h.Kinds = append(h.Kinds, view.EventCount{Kind: string(k), Count: kinds[k]})
	}
	h.Days = make([]view.Day, 0, len(dates))
	for _, date := range dates {
		d := days[date]
		out := view.Day{Date: date, Events: d.events, Commits: len(d.commits), Kinds: []view.EventCount{}}
		for _, k := range eventKinds {
			if n := d.kinds[k]; n > 0 {
				out.Kinds = append(out.Kinds, view.EventCount{Kind: string(k), Count: n})
			}
		}
		h.Days = append(h.Days, out)
	}
}

// historyCommands returns the two commands of the history's provenance.
func (b builder) historyCommands(top string) []view.Command {
	log := b.f.Log()
	if log == nil {
		return []view.Command{}
	}
	span := log.Source
	if b.q.From != "" {
		span = b.q.From + ".." + span
	}
	ids := b.subtree(top)
	var paths []string
	pattern := strings.Join(ids, "|")
	if top == b.f.Root() {
		paths = append(paths, b.fromRoot("tasks"), b.fromRoot("status"))
		pattern = "[0-9a-f]{4}"
	} else {
		for _, id := range ids {
			paths = append(paths, b.fromRoot("tasks/"+id+".yaml"), b.fromRoot("status/"+id+".yaml"))
		}
	}
	named := map[string]bool{}
	for _, id := range ids {
		for _, n := range b.f.Snapshots(id) {
			if n.Link != nil && n.Link.Form == model.Submodule && !named[n.Link.URL] {
				named[n.Link.URL] = true
				paths = append(paths, n.Link.URL)
			}
		}
	}
	format := "git log --format='%H %as %ae %s' "
	return []view.Command{
		{Text: format + span + " -- " + strings.Join(paths, " ")},
		{Text: format + "-E --grep='^(Authorised|Reaffirmed): (" + pattern + ")$' --grep='^Reviewed: (" + pattern + ") ' " + span},
	}
}
