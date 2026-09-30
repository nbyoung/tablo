package main

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Stand-in for derivation (task 27a3): authorisation, status, requirements,
// reviews and events, read from the Git history.

type Commit struct {
	Hash      string              `json:"hash"`
	Parents   []string            `json:"-"`
	Author    string              `json:"author"`
	Committer string              `json:"committer"`
	Date      string              `json:"date"`
	Time      int                 `json:"-"`
	Subject   string              `json:"subject"`
	Trailers  map[string][]string `json:"-"`
}

type Repo struct {
	path, ref string
	proj      *Project
	commits   map[string]*Commit
	order     []string // newest first
	trunkFP   map[string]bool
	onTrunk   bool
	trunkTip  string
	projAt    map[string]*Project
}

func newRepo(path, ref string) (*Repo, error) {
	h, err := git(path, "rev-parse", ref+"^{commit}")
	if err != nil {
		return nil, err
	}
	r := &Repo{path: path, ref: strings.TrimSpace(h), commits: map[string]*Commit{}, projAt: map[string]*Project{}, trunkFP: map[string]bool{}}
	if r.proj, err = load(path, r.ref); err != nil {
		return nil, err
	}
	r.projAt[r.ref] = r.proj
	const sep, end = "\x1f", "\x1e"
	out, err := git(path, "log", "--format=%H"+sep+"%P"+sep+"%ae"+sep+"%ce"+sep+"%as"+sep+"%at"+sep+"%s"+sep+"%(trailers:only,unfold)"+end, r.ref)
	if err != nil {
		return nil, err
	}
	for _, rec := range strings.Split(out, end) {
		f := strings.Split(strings.TrimLeft(rec, "\n"), sep)
		if len(f) < 8 {
			continue
		}
		c := &Commit{Hash: f[0], Parents: strings.Fields(f[1]), Author: f[2], Committer: f[3], Date: f[4], Subject: f[6], Trailers: map[string][]string{}}
		c.Time, _ = strconv.Atoi(f[5])
		for _, l := range strings.Split(f[7], "\n") {
			if k, v, ok := strings.Cut(l, ": "); ok {
				c.Trailers[k] = append(c.Trailers[k], strings.TrimSpace(v))
			}
		}
		r.commits[c.Hash] = c
		r.order = append(r.order, c.Hash)
	}
	if tip, err := git(path, "rev-parse", "--verify", "-q", r.proj.Trunk+"^{commit}"); err == nil {
		r.trunkTip = strings.TrimSpace(tip)
		fp, _ := git(path, "rev-list", "--first-parent", r.trunkTip)
		for _, h := range strings.Fields(fp) {
			r.trunkFP[h] = true
		}
	}
	r.onTrunk = r.trunkFP[r.ref]
	return r, nil
}

func (r *Repo) projectAt(hash string) *Project {
	if p, ok := r.projAt[hash]; ok {
		return p
	}
	p, err := load(r.path, hash)
	if err != nil {
		p = nil
	}
	r.projAt[hash] = p
	return p
}

// pathLog lists the commits that changed a path, newest first.
func (r *Repo) pathLog(path string, firstParent bool) []string {
	args := []string{"log", "--format=%H"}
	if firstParent {
		args = append(args, "--first-parent")
	}
	out, _ := git(r.path, append(args, r.ref, "--", path)...)
	return strings.Fields(out)
}

func taskPath(id string) string   { return ".tableaux/tasks/" + id + ".yaml" }
func statusPath(id string) string { return ".tableaux/status/" + id + ".yaml" }

// firstIn returns the first commit of the log order that is in the set.
func (r *Repo) firstIn(order []string, in func(*Commit) bool) *Commit {
	for _, h := range order {
		if c := r.commits[h]; in(c) {
			return c
		}
	}
	return nil
}

func has(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// Auth is the authorisation of a task.
type Auth struct {
	State  string  `json:"state"`
	Commit *Commit `json:"commit,omitempty"`
	By     string  `json:"by,omitempty"`  // "author" or "committer", whichever is an authority
	Way    string  `json:"way,omitempty"` // "commit", "merge" or "trailer"
	Differ bool    `json:"differs_from_trunk,omitempty"`
}

// authorisation reads the trunk's first-parent history; off the trunk every
// task reads as proposed and the result marks the tasks whose file differs.
func (r *Repo) authorisation(id string) Auth {
	if !r.onTrunk {
		a := Auth{State: "proposed"}
		if r.trunkTip != "" {
			blob, _ := git(r.path, "rev-parse", "-q", "--verify", r.trunkTip+":"+taskPath(id))
			a.Differ = strings.TrimSpace(blob) != r.proj.Tasks[id].Blob
		}
		return a
	}
	changed := r.pathLog(taskPath(id), true)
	var fp []string
	// the first-parent line of ref: follow first parents from ref
	for h := r.ref; h != ""; {
		fp = append(fp, h)
		if ps := r.commits[h].Parents; len(ps) > 0 {
			h = ps[0]
		} else {
			h = ""
		}
	}
	d := r.firstIn(fp, func(c *Commit) bool { return has(changed, c.Hash) || has(c.Trailers["Authorised"], id) })
	if d == nil {
		return Auth{State: "proposed"}
	}
	p := r.projectAt(d.Hash)
	if p == nil || p.Tasks[id] == nil {
		return Auth{State: "proposed", Commit: d}
	}
	auth := p.authorities(id)
	if id == p.Root {
		auth = []string{p.Tasks[id].Assignee}
	}
	a := Auth{State: "proposed", Commit: d, Way: "commit"}
	if len(d.Parents) > 1 {
		a.Way = "merge"
	}
	if has(d.Trailers["Authorised"], id) && len(d.Parents) < 2 {
		a.Way = "trailer"
	}
	switch {
	case has(auth, d.Author):
		a.State, a.By = "authorised", "author"
	case has(auth, d.Committer):
		a.State, a.By = "authorised", "committer"
	}
	return a
}

// Status is a leaf's status, or a parent's roll-up.
type Status struct {
	Gate     string         `json:"gate"`
	State    string         `json:"state"`
	Reason   string         `json:"reason,omitempty"`
	Note     string         `json:"note,omitempty"`
	Date     string         `json:"date,omitempty"`
	Recorder string         `json:"recorder,omitempty"`
	Commit   string         `json:"commit,omitempty"`
	Derived  bool           `json:"derived,omitempty"`
	From     string         `json:"from,omitempty"` // the child a roll-up comes from
	Snapshot map[string]any `json:"snapshot,omitempty"`
}

func (r *Repo) status(id string) Status {
	t := r.proj.Tasks[id]
	if len(t.Children) > 0 {
		return r.rollup(t)
	}
	f := r.proj.statuses[id]
	var s Status
	if f == nil {
		s = Status{Gate: "undefined", State: "undefined"}
		if c := r.firstIn(r.pathLog(taskPath(id), false), func(*Commit) bool { return true }); c != nil {
			s.Date, s.Recorder, s.Commit = c.Date, c.Author, c.Hash
		}
		return s
	}
	s = Status{Gate: str(f, "gate"), State: str(f, "state"), Reason: str(f, "reason"), Note: str(f, "note")}
	in := r.pathLog(statusPath(id), false)
	if c := r.firstIn(r.order, func(c *Commit) bool { return has(in, c.Hash) || has(c.Trailers["Reaffirmed"], id) }); c != nil {
		s.Date, s.Recorder, s.Commit = c.Date, c.Author, c.Hash
	}
	if next := r.proj.nextGate(id, s.Gate); next != "" {
		if j := r.proj.junction(id, next); j.Kind == "recursive" {
			r.overlay(&s, j)
		}
	}
	return s
}

// overlay replaces a status by the subproject's task, read at the pinned commit.
func (r *Repo) overlay(s *Status, j Junction) {
	url := str(j.Subproject, "url")
	tree, err := git(r.path, "ls-tree", r.ref, url)
	f := strings.Fields(tree)
	if err != nil || len(f) < 3 || f[0] != "160000" {
		s.Snapshot = map[string]any{"url": url, "error": "no submodule pin"}
		return
	}
	pin := f[2]
	sub := filepath.Join(r.path, url)
	if cfg, err := git(r.path, "config", "submodule."+url+".url"); err == nil {
		sub = strings.TrimSpace(cfg)
	}
	sr, err := newRepo(sub, pin)
	if err != nil {
		s.Snapshot = map[string]any{"url": url, "pin": pin, "error": err.Error()}
		return
	}
	sid := str(j.Subproject, "id")
	if sid == "" {
		sid = sr.proj.Root
	}
	st := sr.status(sid)
	s.State, s.Reason, s.Note, s.Date, s.Recorder, s.Commit = st.State, st.Reason, st.Note, st.Date, st.Recorder, st.Commit
	s.Snapshot = map[string]any{"url": url, "pin": pin, "task": sid, "gate": st.Gate}
}

func (r *Repo) rollup(t *Task) Status {
	type kid struct {
		id string
		s  Status
	}
	var all, use []kid
	for _, c := range t.Children {
		all = append(all, kid{c, r.status(c)})
	}
	for _, k := range all {
		if r.proj.severity(k.s.State) > 0 {
			use = append(use, k)
		}
	}
	if len(use) == 0 {
		use = all
	}
	best := use[0]
	date := ""
	for _, k := range use {
		if gi, bi := r.proj.gateIndex(k.s.Gate), r.proj.gateIndex(best.s.Gate); gi < bi ||
			(gi == bi && r.proj.severity(k.s.State) > r.proj.severity(best.s.State)) {
			best = k
		}
		if date == "" || k.s.Date < date {
			date = k.s.Date
		}
	}
	// the most severe child at the earliest gate; ties keep the first in display order
	return Status{Gate: best.s.Gate, State: best.s.State, Reason: best.s.Reason, Note: best.s.Note, Date: date, Derived: true, From: best.id}
}

// Cond is a requirement with its condition.
type Cond struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text,omitempty"`
	Met       bool   `json:"met"`
	Due       bool   `json:"due"`
	Condition string `json:"condition"` // met, unmet (due, not met) or pending
}

func (r *Repo) requirements(id string) []Cond {
	p := r.proj
	next := p.nextGate(id, r.status(id).Gate)
	var out []Cond
	for _, q := range p.Tasks[id].Requires {
		c := Cond{ID: q.ID, From: q.From, To: q.To, Text: q.Text}
		if o := p.Tasks[q.ID]; o != nil {
			c.Title = o.Title
			c.Met = p.gateIndex(r.status(q.ID).Gate) >= p.gateIndex(q.From)
		}
		c.Due = next != "" && p.gateIndex(next) >= p.gateIndex(q.To)
		c.Condition = map[bool]string{true: "met", false: "pending"}[c.Met]
		if !c.Met && c.Due {
			c.Condition = "unmet"
		}
		out = append(out, c)
	}
	return out
}

// dependents lists the requirements that other tasks state on id.
func (r *Repo) dependents(id string) []Cond {
	var out []Cond
	ids, _ := r.proj.walk(r.proj.Root)
	for _, o := range ids {
		for _, c := range r.requirements(o) {
			if c.ID == id {
				c.ID, c.Title = o, r.proj.Tasks[o].Title
				out = append(out, c)
			}
		}
	}
	return out
}

// Review is the commit that accepts a reviewed junction.
type Review struct {
	Gate     string `json:"gate"`
	Reviewer string `json:"reviewer"`
	Commit   string `json:"commit"`
	Date     string `json:"date"`
}

func (r *Repo) reviews(id string) []Review {
	p := r.proj
	var out []Review
	gi := p.gateIndex(r.status(id).Gate)
	for i, g := range p.Gates {
		j := p.junction(id, g.Key)
		if i > gi || g.Key == "defined" || g.Key == "undefined" || j.Kind != "plain" || j.Reviewer == "" {
			continue
		}
		if c := r.firstIn(r.order, func(c *Commit) bool {
			return has(c.Trailers["Reviewed"], id+" "+g.Key) && (c.Author == j.Reviewer || c.Committer == j.Reviewer)
		}); c != nil {
			out = append(out, Review{g.Key, j.Reviewer, c.Hash, c.Date})
		}
	}
	return out
}

// Event is one entry of a task's history.
type Event struct {
	Date      string `json:"date"`
	Commit    string `json:"commit"`
	By        string `json:"by"`
	Committer string `json:"committer,omitempty"`
	Task      string `json:"task"`
	Event     string `json:"event"`
	Gate      string `json:"gate,omitempty"`
	State     string `json:"state,omitempty"`
	Reason    string `json:"reason,omitempty"`
	Note      string `json:"note,omitempty"`
}

func (r *Repo) events(id string) []Event {
	files, merges := r.pathLog(taskPath(id), false), r.pathLog(taskPath(id), true)
	stat := r.pathLog(statusPath(id), false)
	var out []Event
	for i := len(r.order) - 1; i >= 0; i-- {
		c := r.commits[r.order[i]]
		add := func(e Event) {
			e.Date, e.Commit, e.By, e.Task = c.Date, c.Hash, c.Author, id
			if c.Committer != c.Author {
				e.Committer = c.Committer
			}
			out = append(out, e)
		}
		if has(files, c.Hash) && len(c.Parents) < 2 {
			add(Event{Event: "task"})
		}
		if has(c.Trailers["Authorised"], id) || (len(c.Parents) > 1 && has(merges, c.Hash)) {
			add(Event{Event: "authorised"})
		}
		if has(stat, c.Hash) && len(c.Parents) < 2 {
			src, _ := git(r.path, "show", c.Hash+":"+statusPath(id))
			f, _ := parseYAML(src)
			add(Event{Event: "status", Gate: str(f, "gate"), State: str(f, "state"), Reason: str(f, "reason"), Note: str(f, "note")})
		}
		if has(c.Trailers["Reaffirmed"], id) {
			add(Event{Event: "reaffirmed"})
		}
		for _, v := range c.Trailers["Reviewed"] {
			if tid, g, _ := strings.Cut(v, " "); tid == id {
				add(Event{Event: "reviewed", Gate: g})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return r.commits[out[i].Commit].Time < r.commits[out[j].Commit].Time })
	return out
}
