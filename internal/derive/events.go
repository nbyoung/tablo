package derive

import (
	"path"
	"sort"
	"strings"

	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
)

// EventKind is an event of SYNTAX.md#history.
type EventKind string

// The kinds, in the order one commit's events of one task take.
const (
	TaskEvent  EventKind = "task"
	Authorised EventKind = "authorised"
	StatusSet  EventKind = "status"
	Reaffirmed EventKind = "reaffirmed"
	Reviewed   EventKind = "reviewed"
	Pinned     EventKind = "pin"
)

// Effect is what a Reviewed: trailer does.
type Effect int

// The effects.
const (
	NoEffect   Effect = iota // not a reviewed event
	Accepts                  // the reviewer's first: it completes the junction
	Repeats                  // the reviewer's, after one that accepts
	WrongHand                // neither author nor committer is the junction's reviewer
	NoReviewer               // the junction has no reviewer
	AtDefined                // the authorisation stands as the review of defined
)

// String returns the effect's name.
func (e Effect) String() string {
	names := [...]string{"none", "accepts", "repeats", "wrong hand", "no reviewer", "at defined"}
	if e < 0 || int(e) >= len(names) {
		return "effect?"
	}
	return names[e]
}

// Verdict is how a commit's Model: trailer reads against its junction.
type Verdict int

// The verdicts.
const (
	NoModel  Verdict = iota // the junction states no model and the commit carries no trailer, or another hand commits
	Unstated                // a trailer where the junction states no model
	Match                   // the trailer is the stated model or starts with it
	Mismatch                // the trailer names a model outside the one stated
	Missing                 // the contributor's commit carries no trailer
	Exempt                  // as Missing, where version.yaml at the commit states a language before 0.2.1
)

// String returns the verdict's name.
func (v Verdict) String() string {
	names := [...]string{"no model", "unstated", "match", "mismatch", "missing", "exempt"}
	if v < 0 || int(v) >= len(names) {
		return "verdict?"
	}
	return names[v]
}

// Reading is the model check of one commit at one junction.
type Reading struct {
	Gate        string // the junction the commit is at
	Contributor string // its contributor, as the files stand at the commit
	Stated      string // the model it states there; "" for none
	Trailer     string // the commit's first Model: trailer; "" for none
	Verdict     Verdict
}

// Event is one event of one task.
type Event struct {
	Commit                    *history.Commit
	Task                      string // the task of this project the event concerns
	Kind                      EventKind
	Gate, State, Reason, Note string      // status: as the commit records them; reviewed: the gate
	URL, Old, New             string      // pin
	Link                      *model.Link // pin, and an event taken from a subproject
	Sub                       string      // an event taken from a subproject: the task there; else ""
	Reading                   *Reading    // status and reaffirmed
	Effect                    Effect      // reviewed
}

// Unread is a task trailer the method cannot read (H1).
type Unread struct {
	Commit *history.Commit
	Line   string // the trailer as written
	NoTask bool   // it names no task of the project
	NoGate bool   // Reviewed: it names no gate that applies to the task
}

// Events returns the events of a task: its own and, for each snapshot, those
// of the task read there and of its descendants up to the pin. The order is
// author time; then a task's own events before a subproject's; then oldest
// first in the pass; then the kinds as listed.
func (f *Facts) Events(id string) []*Event {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return append([]*Event(nil), f.eventsOf(id)...)
}

// History returns the events of every task of the project, each task's as
// Events gives them, in one order: author time; then a task's own events
// before a subproject's; then oldest first in the pass; then tasks in
// display order; then the kinds as listed.
func (f *Facts) History() []*Event {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	var all []*Event
	for _, id := range f.order {
		all = append(all, f.ownEvents()[id]...)
	}
	for _, id := range f.order {
		all = append(all, f.taken(id)...)
	}
	arrange(all)
	return all
}

// Unread returns the task trailers the source's history carries that name
// no task of the project or, for Reviewed:, no gate that applies to the
// task, the oldest commit first. They make no event.
func (f *Facts) Unread() []*Unread {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	f.ownEvents()
	return append([]*Unread(nil), f.unread...)
}

// arrange puts events in the order of a history. The sort is stable, so
// events of one commit keep the order they come in.
func arrange(events []*Event) {
	sort.SliceStable(events, func(i, j int) bool {
		a, b := events[i], events[j]
		switch {
		case a.Commit.Time != b.Commit.Time:
			return a.Commit.Time < b.Commit.Time
		case (a.Sub == "") != (b.Sub == ""):
			return a.Sub == ""
		}
		return a.Commit.Seq > b.Commit.Seq
	})
}

// eventsOf returns a task's events with its subprojects' and keeps them.
func (f *Facts) eventsOf(id string) []*Event {
	if list, ok := f.events[id]; ok {
		return list
	}
	if f.eventsBusy[id] {
		return nil // two projects that read each other
	}
	f.eventsBusy[id] = true
	list := append(append([]*Event(nil), f.ownEvents()[id]...), f.taken(id)...)
	arrange(list)
	delete(f.eventsBusy, id)
	f.events[id] = list
	return list
}

// taken returns the events a task takes from its subprojects: for each
// snapshot, those of the task read there and of its descendants, each with
// Sub set. The subproject's facts end at the pin, so its events do too.
func (f *Facts) taken(id string) []*Event {
	if f.task(id) == nil {
		return nil
	}
	f.resolved(id)
	var list []*Event
	for _, s := range f.snapshots[id] {
		if s.Why != Determined {
			continue
		}
		sub := s.Facts
		from := sub.place[s.Target]
		for i := from; i < len(sub.order) && (i == from || sub.under(sub.order[i], s.Target)); i++ {
			for _, e := range sub.eventsOf(sub.order[i]) {
				copied := *e
				copied.Task, copied.Sub, copied.Link = id, e.Task, s.Link
				list = append(list, &copied)
			}
		}
	}
	return list
}

// under reports whether a task is a descendant of another.
func (f *Facts) under(id, ancestor string) bool {
	for at, ok := f.parent[id]; ok; at, ok = f.parent[at] {
		if at == ancestor {
			return true
		}
	}
	return false
}

// ownEvents returns the events of this project's own history by task, each
// task's the oldest commit first and one commit's in the order of the kinds,
// and fills the unread trailers. One walk of the commits the source reaches
// makes them all.
func (f *Facts) ownEvents() map[string][]*Event {
	if f.own != nil {
		return f.own
	}
	f.own = map[string][]*Event{}
	pass := f.indexed()
	plan := path.Join(f.p.Where.Dir, ".tableaux") + "/"
	for i := len(pass.source) - 1; i >= 0; i-- {
		c := pass.source[i]
		// What the commit does, by task and kind.
		at := map[string]map[EventKind][]*Event{}
		add := func(e *Event) {
			e.Commit = c
			if at[e.Task] == nil {
				at[e.Task] = map[EventKind][]*Event{}
			}
			at[e.Task][e.Kind] = append(at[e.Task][e.Kind], e)
		}
		for _, change := range c.Changes {
			if !f.log.Changed(c, change.Path) {
				continue
			}
			below, inPlan := strings.CutPrefix(change.Path, plan)
			dir, name := path.Split(below)
			id, isFile := strings.CutSuffix(name, ".yaml")
			switch {
			case change.Gitlink:
				for _, id := range f.naming(c, change.Path) {
					add(&Event{Task: id, Kind: Pinned, URL: change.Path, Old: f.before(c, change.Path), New: change.New, Link: f.link(change.Path, "")})
				}
			case inPlan && isFile && dir == "tasks/":
				add(&Event{Task: id, Kind: TaskEvent})
				for _, e := range f.moved(c, id) {
					add(e)
				}
			case inPlan && isFile && dir == "status/":
				add(f.statusEvent(c, id))
			}
		}
		for _, t := range c.Trailers {
			words := strings.Fields(t.Value)
			switch t.Key {
			case "Authorised", "Reaffirmed":
				switch {
				case len(words) != 1 || f.task(words[0]) == nil:
					f.unread = append(f.unread, &Unread{Commit: c, Line: t.Key + ": " + t.Value, NoTask: true})
				case t.Key == "Authorised" && len(at[words[0]][Authorised]) == 0:
					add(&Event{Task: words[0], Kind: Authorised})
				case t.Key == "Reaffirmed" && len(at[words[0]][Reaffirmed]) == 0:
					add(&Event{Task: words[0], Kind: Reaffirmed, Reading: f.readingAt(c, words[0], "", false)})
				}
			case "Reviewed":
				id, gate := "", ""
				if len(words) == 2 {
					id, gate = words[0], words[1]
				} else if len(words) > 0 {
					id = words[0]
				}
				u := &Unread{Commit: c, Line: t.Key + ": " + t.Value, NoTask: f.task(id) == nil}
				if at, named := f.index[gate]; !named || at == 0 {
					u.NoGate = true // no gate after undefined
				} else if j := f.junction(id, gate); j != nil {
					u.NoGate = j.Kind == model.NotApplicable
				}
				if u.NoTask || u.NoGate {
					f.unread = append(f.unread, u)
					continue
				}
				repeated := false
				for _, e := range at[id][Reviewed] {
					repeated = repeated || e.Gate == gate
				}
				if !repeated {
					e := &Event{Task: id, Kind: Reviewed, Gate: gate}
					for _, r := range f.reviews(id, gate) {
						if r.commit == c {
							e.Effect = r.effect
						}
					}
					add(e)
				}
			}
		}
		for id, kinds := range at {
			for _, kind := range []EventKind{TaskEvent, Authorised, StatusSet, Reaffirmed, Reviewed, Pinned} {
				f.own[id] = append(f.own[id], kinds[kind]...)
			}
		}
	}
	return f.own
}

// before returns the object a path holds at a commit's first parent, or "".
func (f *Facts) before(c *history.Commit, p string) string {
	if parent := f.first(c); parent != nil {
		return f.log.Object(parent.ID, p)
	}
	return ""
}

// link returns the link of the project in view with a url, at a commit when
// one is given, or nil.
func (f *Facts) link(url, commit string) *model.Link {
	var found *model.Link
	for _, l := range f.p.Links {
		if l.URL == url && (found == nil || l.Commit == commit && found.Commit != commit) {
			found = l
		}
	}
	return found
}

// fields returns the subproject fields of a task, a requirement's before a
// junction's, each in file order.
func fields(task *model.Task) []*model.Subproject {
	var all []*model.Subproject
	for _, r := range task.Requires {
		if r.Subproject != nil {
			all = append(all, r.Subproject)
		}
	}
	for _, j := range task.Junctions {
		if j.Subproject != nil {
			all = append(all, j.Subproject)
		}
	}
	return all
}

// naming returns the tasks whose files, as they stand at a commit, name a
// submodule path in a junction or a requirement, by id.
func (f *Facts) naming(c *history.Commit, submodule string) []string {
	p := f.log.Project(c.ID)
	var ids []string
	for _, id := range p.TaskIDs() {
		for _, field := range fields(p.Tasks[id]) {
			if url := field.URL.V; url != "" && !absolute.MatchString(url) && path.Clean(url) == submodule {
				ids = append(ids, id)
				break
			}
		}
	}
	return ids
}

// commits returns the commit a task's file states beside each absolute URL,
// and the URLs in file order.
func commits(task *model.Task) (urls []string, by map[string]string) {
	by = map[string]string{}
	if task == nil {
		return nil, by
	}
	for _, field := range fields(task) {
		url := field.URL.V
		if _, seen := by[url]; seen || !absolute.MatchString(url) {
			continue
		}
		urls = append(urls, url)
		by[url] = field.Commit.V
	}
	return urls, by
}

// moved returns the pin events of a commit that changes a task's file: one
// for each absolute URL whose commit field the change sets or moves. old is
// empty when the linkage first appears.
func (f *Facts) moved(c *history.Commit, id string) []*Event {
	urls, now := commits(f.log.Project(c.ID).Tasks[id])
	var was map[string]string
	if parent := f.first(c); parent != nil {
		_, was = commits(f.log.Project(parent.ID).Tasks[id])
	}
	var list []*Event
	for _, url := range urls {
		if now[url] != "" && now[url] != was[url] {
			list = append(list, &Event{Task: id, Kind: Pinned, URL: url, Old: was[url], New: now[url], Link: f.link(url, now[url])})
		}
	}
	return list
}

// statusEvent returns the event of a commit that changes a status file,
// with the gate, state, reason and note the commit records.
func (f *Facts) statusEvent(c *history.Commit, id string) *Event {
	e := &Event{Task: id, Kind: StatusSet}
	if s := f.log.Project(c.ID).Statuses[id]; s != nil {
		e.Gate, e.State, e.Reason, e.Note = s.Gate.V, s.State.V, s.Reason.V, s.Note.V
	}
	was := ""
	if parent := f.first(c); parent != nil {
		if s := f.log.Project(parent.ID).Statuses[id]; s != nil {
			was = s.Gate.V
		}
	}
	e.Reading = f.readingAt(c, id, e.Gate, e.Gate != was)
	return e
}

// readingAt gives the junction a status or reaffirmed commit is at and its
// model check. A commit that changes the status's gate is at the junction of
// the gate it records; any other is at the task's next applicable junction
// as the files stand at that commit. gate is the gate the status states at
// the commit; "" reads it from the files there.
func (f *Facts) readingAt(c *history.Commit, id, gate string, records bool) *Reading {
	then := f.past(c.ID)
	if !records {
		if gate == "" {
			gate = "undefined"
			if s := then.p.Statuses[id]; s != nil {
				gate = s.Gate.V
			}
		}
		gate = then.next(id, gate)
	}
	r := &Reading{Gate: gate}
	if values := c.Values("Model"); len(values) > 0 {
		r.Trailer = values[0]
	}
	if j := then.junction(id, gate); j != nil && j.Kind == model.Plain {
		r.Contributor, r.Stated = j.Contributor.V, j.Model.V
	}
	switch {
	case r.Stated == "" && r.Trailer != "":
		r.Verdict = Unstated
	case r.Stated == "":
		r.Verdict = NoModel
	case r.Trailer != "" && strings.HasPrefix(r.Trailer, r.Stated):
		r.Verdict = Match
	case r.Trailer != "":
		r.Verdict = Mismatch
	case c.Author.Email != r.Contributor:
		r.Verdict = NoModel
	case before021(then.p.Version):
		r.Verdict = Exempt
	default:
		r.Verdict = Missing
	}
	return r
}

// before021 reports whether a version.yaml states a language before 0.2.1,
// which introduced the Model: trailer.
func before021(v *model.Version) bool {
	if v == nil || !v.WellFormed {
		return false
	}
	switch {
	case v.Major != 0:
		return false
	case v.Minor != 2:
		return v.Minor < 2
	}
	return v.Patch < 1
}
