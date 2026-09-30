package main

// Stand-ins for the loader and the derivation that the views need: read
// `.tableaux` at a ref through git, resolve junctions, derive status, roll-up,
// requirement conditions and authorisation. The real tasks own these.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Gate struct{ Key, Symbol, Name string }
type State struct {
	Key, Symbol string
	Severity    int
}
type Req struct{ ID, From, To, Text string }

type Task struct {
	ID, Title, Assignee, Parent string
	POrder                      int
	Requires                    []Req
	Junc                        map[string]map[string]any
	Children                    []string
	Depth                       int
	File                        map[string]any // status file; nil when absent
}

type commit struct {
	Hash, Date, Author, Committer string
	Reaff, Auth                   []string
	Files                         map[string]bool
}

type Status struct {
	Gate     string `json:"gate"`
	State    string `json:"state"`
	Reason   string `json:"reason,omitempty"`
	Note     string `json:"note,omitempty"`
	Date     string `json:"date,omitempty"`
	Recorder string `json:"recorder,omitempty"`
	Commit   string `json:"commit,omitempty"`
	From     string `json:"rolled_up_from,omitempty"`
	Derived  bool   `json:"derived,omitempty"`
	Snapshot bool   `json:"subproject_snapshot,omitempty"`
}

type Junction struct {
	Gate        string `json:"gate"`
	Kind        string `json:"kind"` // plain, recursive, not_applicable
	Contributor string `json:"contributor,omitempty"`
	Model       string `json:"model,omitempty"`
	Reviewer    string `json:"reviewer,omitempty"`
	Sub         string `json:"subproject,omitempty"`
	SubID       string `json:"subproject_task,omitempty"`
	Source      string `json:"source,omitempty"`
}

type Project struct {
	dir, ref, trunk string
	Gates           []Gate
	gidx            map[string]int
	States          map[string]State
	Reasons         map[string]string // key to symbol
	Tasks           map[string]*Task
	Root, Owner     string
	Order           []string
	labels          map[string]string
	all, first      []commit
	memo            map[string]*Status
}

func git(dir string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	return string(out), err
}

func readLabels(path string) map[string]string {
	m := map[string]string{}
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	for _, l := range strings.Split(string(b), "\n") {
		if f := strings.Fields(l); len(f) == 2 {
			m[f[1]] = f[0]
		}
	}
	return m
}

func (p *Project) label(h string) string {
	if l, ok := p.labels[h]; ok {
		return l
	}
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func Load(dir, ref string) (*Project, error) {
	p := &Project{dir: dir, ref: ref, gidx: map[string]int{}, States: map[string]State{},
		Reasons: map[string]string{}, Tasks: map[string]*Task{}, memo: map[string]*Status{},
		labels: readLabels(dir + ".labels.txt")}
	names, err := git(dir, "ls-tree", "-r", "--name-only", ref, "--", ".tableaux")
	if err != nil {
		return nil, fmt.Errorf("ls-tree %s: %w", ref, err)
	}
	files := map[string]any{}
	for _, n := range strings.Fields(names) {
		src, err := git(dir, "show", ref+":"+n)
		if err != nil {
			return nil, err
		}
		v, err := parseYAML(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", n, err)
		}
		files[n] = v
	}
	g := files[".tableaux/gates.yaml"]
	for i, x := range list(g, "gates") {
		gt := Gate{str(x, "key"), str(x, "symbol"), str(x, "name")}
		p.Gates = append(p.Gates, gt)
		p.gidx[gt.Key] = i
	}
	for _, x := range list(g, "states") {
		p.States[str(x, "key")] = State{str(x, "key"), str(x, "symbol"), num(x, "severity")}
	}
	for _, x := range list(g, "reasons") {
		p.Reasons[str(x, "key")] = str(x, "symbol")
	}
	p.trunk = str(files[".tableaux/version.yaml"], "trunk")
	if p.trunk == "" {
		p.trunk = "main"
	}
	for n, v := range files {
		if !strings.HasPrefix(n, ".tableaux/tasks/") {
			continue
		}
		id := strings.TrimSuffix(filepath.Base(n), ".yaml")
		t := &Task{ID: id, Title: str(v, "title"), Assignee: str(v, "assignee"),
			Parent: str(v, "parent", "id"), POrder: num(v, "parent", "order"),
			Junc: map[string]map[string]any{}}
		for _, r := range list(v, "requires") {
			t.Requires = append(t.Requires, Req{str(r, "id"), str(r, "from"), str(r, "to"), str(r, "text")})
		}
		for k, j := range amap(v, "junctions") {
			jm, _ := j.(map[string]any)
			t.Junc[k] = jm
		}
		if sv, ok := files[".tableaux/status/"+id+".yaml"]; ok {
			t.File, _ = sv.(map[string]any)
		}
		p.Tasks[id] = t
	}
	for _, t := range p.Tasks {
		if t.Parent == "" {
			p.Root, p.Owner = t.ID, t.Assignee
		} else if par := p.Tasks[t.Parent]; par != nil {
			par.Children = append(par.Children, t.ID)
		}
	}
	var walk func(id string, d int)
	walk = func(id string, d int) {
		t := p.Tasks[id]
		t.Depth = d
		p.Order = append(p.Order, id)
		sort.Slice(t.Children, func(a, b int) bool {
			x, y := p.Tasks[t.Children[a]], p.Tasks[t.Children[b]]
			if (x.POrder == 0) != (y.POrder == 0) {
				return x.POrder != 0
			}
			if x.POrder != y.POrder {
				return x.POrder < y.POrder
			}
			return x.ID < y.ID
		})
		for _, c := range t.Children {
			walk(c, d+1)
		}
	}
	if p.Root != "" {
		walk(p.Root, 0)
	}
	p.all = p.log(ref, false)
	p.first = p.log(p.trunk, true)
	return p, nil
}

func (p *Project) log(ref string, firstParent bool) []commit {
	args := []string{"log", "--name-only", "--format=%x1e%H%x1f%as%x1f%ae%x1f%ce%x1f" +
		"%(trailers:key=Reaffirmed,valueonly,separator=%x2C)%x1f%(trailers:key=Authorised,valueonly,separator=%x2C)%x1f"}
	if firstParent {
		args = append(args, "--first-parent")
	}
	out, err := git(p.dir, append(args, ref)...)
	if err != nil {
		return nil
	}
	var cs []commit
	for _, rec := range strings.Split(out, "\x1e")[1:] {
		f := strings.Split(rec, "\x1f")
		if len(f) < 7 {
			continue
		}
		c := commit{Hash: f[0], Date: f[1], Author: f[2], Committer: f[3], Files: map[string]bool{}}
		c.Reaff = splitList(f[4])
		c.Auth = splitList(f[5])
		for _, n := range strings.Split(f[6], "\n") {
			if n = strings.TrimSpace(n); n != "" {
				c.Files[n] = true
			}
		}
		cs = append(cs, c)
	}
	return cs
}

func splitList(s string) []string {
	var r []string
	for _, x := range strings.Split(s, ",") {
		if x = strings.TrimSpace(x); x != "" {
			r = append(r, x)
		}
	}
	return r
}

func has(l []string, s string) bool {
	for _, x := range l {
		if x == s {
			return true
		}
	}
	return false
}

// Authorities: the assignees of the ancestors, nearest first; the owner for the root.
func (p *Project) Authorities(id string) []string {
	var r []string
	for t := p.Tasks[id]; t != nil && t.Parent != ""; t = p.Tasks[t.Parent] {
		r = append(r, p.Tasks[t.Parent].Assignee)
	}
	return r
}

// Authorised follows the deciding-commit rule on the trunk's first-parent line.
// The authorities come from the tip, not from the tree at the deciding commit.
func (p *Project) Authorised(id string) (bool, string) {
	if p.ref != p.trunk {
		return false, ""
	}
	file := ".tableaux/tasks/" + id + ".yaml"
	auth := p.Authorities(id)
	if id == p.Root {
		auth = []string{p.Owner}
	}
	for _, c := range p.first {
		if c.Files[file] || has(c.Auth, id) {
			return has(auth, c.Author) || has(auth, c.Committer), p.label(c.Hash)
		}
	}
	return false, ""
}

func (p *Project) Junction(id, gate string) Junction {
	t := p.Tasks[id]
	j := Junction{Gate: gate, Kind: "plain", Contributor: t.Assignee}
	if gate == "undefined" {
		return j
	}
	var contrib, model, reviewer, src string
	for c := t; c != nil; c = p.Tasks[c.Parent] {
		e, ok := c.Junc[gate]
		if !ok {
			continue
		}
		if e["applies"] == false {
			if src == "" {
				return Junction{Gate: gate, Kind: "not_applicable", Source: c.ID}
			}
			break
		}
		if sp, ok := e["subproject"].(map[string]any); ok {
			if src == "" {
				return Junction{Gate: gate, Kind: "recursive", Sub: str(sp, "url"), SubID: str(sp, "id"), Source: c.ID}
			}
			break
		}
		if src == "" {
			src = c.ID
		}
		if v := str(e, "contributor"); v != "" && contrib == "" {
			contrib = v
		}
		if v := str(e, "model"); v != "" && model == "" {
			model = v
		}
		if v := str(e, "reviewer"); v != "" && reviewer == "" {
			reviewer = v
		}
	}
	if contrib != "" {
		j.Contributor = contrib
	}
	j.Model, j.Reviewer, j.Source = model, reviewer, src
	if model != "" && reviewer == "" {
		j.Reviewer = t.Assignee
	}
	return j
}

func (p *Project) Applicable(id, gate string) bool {
	return gate == "undefined" || p.Junction(id, gate).Kind != "not_applicable"
}

// NextGate is the first applicable gate after gate, or "" past the last.
func (p *Project) NextGate(id, gate string) string {
	for i := p.gidx[gate] + 1; i < len(p.Gates); i++ {
		if p.Applicable(id, p.Gates[i].Key) {
			return p.Gates[i].Key
		}
	}
	return ""
}

func (p *Project) firstGate(id string) string { return p.NextGate(id, "undefined") }

func (p *Project) lastGate(id string) string {
	for i := len(p.Gates) - 1; i > 0; i-- {
		if p.Applicable(id, p.Gates[i].Key) {
			return p.Gates[i].Key
		}
	}
	return ""
}

func (p *Project) IsLeaf(id string) bool { return len(p.Tasks[id].Children) == 0 }

func (p *Project) Status(id string) *Status {
	if s, ok := p.memo[id]; ok {
		return s
	}
	t := p.Tasks[id]
	var s *Status
	if p.IsLeaf(id) {
		s = p.leafStatus(t)
	} else {
		s = p.rollUp(t)
	}
	p.memo[id] = s
	return s
}

func (p *Project) leafStatus(t *Task) *Status {
	s := &Status{Gate: "undefined", State: "undefined"}
	file := ".tableaux/status/" + t.ID + ".yaml"
	if t.File == nil {
		file = ".tableaux/tasks/" + t.ID + ".yaml"
	} else {
		s.Gate, s.State = str(t.File, "gate"), str(t.File, "state")
		s.Reason, s.Note = str(t.File, "reason"), str(t.File, "note")
	}
	for _, c := range p.all {
		if c.Files[file] || (t.File != nil && has(c.Reaff, t.ID)) {
			s.Date, s.Recorder, s.Commit = c.Date, c.Author, p.label(c.Hash)
			break
		}
	}
	if next := p.NextGate(t.ID, s.Gate); next != "" {
		if j := p.Junction(t.ID, next); j.Kind == "recursive" {
			if snap := p.snapshot(j); snap != nil {
				snap.Gate, snap.Snapshot = s.Gate, true
				return snap
			}
		}
	}
	return s
}

// snapshot reads the subproject task's status at the commit the parent pins.
// The subproject sits beside the parent, at the path .gitmodules gives.
func (p *Project) snapshot(j Junction) *Status {
	tree, err := git(p.dir, "ls-tree", p.ref, j.Sub)
	if err != nil || !strings.HasPrefix(tree, "160000") {
		return nil
	}
	pin := strings.Fields(tree)[2]
	gm, _ := git(p.dir, "show", p.ref+":.gitmodules")
	rel := ""
	for _, l := range strings.Split(gm, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok && strings.TrimSpace(k) == "url" {
			rel = strings.TrimSpace(v)
		}
	}
	sub := filepath.Join(p.dir, rel)
	file := ".tableaux/status/" + j.SubID + ".yaml"
	src, err := git(sub, "show", pin+":"+file)
	if err != nil {
		return nil
	}
	v, err := parseYAML(src)
	if err != nil {
		return nil
	}
	s := &Status{State: str(v, "state"), Reason: str(v, "reason"), Note: str(v, "note")}
	out, _ := git(sub, "log", "-1", "--format=%H %as %ae", pin, "--", file)
	if f := strings.Fields(out); len(f) == 3 {
		s.Commit, s.Date, s.Recorder = readLabels(sub + ".labels.txt")[f[0]], f[1], f[2]
		if s.Commit == "" {
			s.Commit = f[0][:7]
		}
	}
	return s
}

// rollUp applies README.md#status: consider children with non-zero severity
// (all when none), take the earliest gate, the most severe child at it (the
// first in display order on a tie, F21) and the oldest date.
func (p *Project) rollUp(t *Task) *Status {
	var cons []string
	for _, c := range t.Children {
		if p.States[p.Status(c).State].Severity != 0 {
			cons = append(cons, c)
		}
	}
	if len(cons) == 0 {
		cons = t.Children
	}
	best, oldest := "", ""
	for _, c := range cons {
		cs := p.Status(c)
		if oldest == "" || cs.Date < oldest {
			oldest = cs.Date
		}
		if best == "" {
			best = c
			continue
		}
		bs := p.Status(best)
		gi, bi := p.gidx[cs.Gate], p.gidx[bs.Gate]
		if gi < bi || (gi == bi && p.States[cs.State].Severity > p.States[bs.State].Severity) {
			best = c
		}
	}
	b := p.Status(best)
	from := best
	if b.Derived {
		from = b.From
	}
	return &Status{Gate: b.Gate, State: b.State, Reason: b.Reason, Note: b.Note, Date: oldest,
		From: from, Derived: true}
}

type ReqCond struct {
	Task      string `json:"task"`
	Requires  string `json:"requires"`
	From      string `json:"from"`
	To        string `json:"to"`
	Text      string `json:"text,omitempty"`
	Met       bool   `json:"met"`
	Due       bool   `json:"due"`
	Condition string `json:"condition"`
}

func (p *Project) Reqs(id string) []ReqCond {
	t := p.Tasks[id]
	var out []ReqCond
	for _, r := range t.Requires {
		from, to := r.From, r.To
		if from == "" {
			from = p.lastGate(r.ID)
		}
		if to == "" {
			to = p.firstGate(id)
		}
		met := p.gidx[p.Status(r.ID).Gate] >= p.gidx[from]
		next := p.NextGate(id, p.Status(id).Gate)
		due := next != "" && p.gidx[next] >= p.gidx[to]
		c := "pending"
		switch {
		case met:
			c = "met"
		case due:
			c = "unmet"
		}
		out = append(out, ReqCond{id, r.ID, from, to, r.Text, met, due, c})
	}
	return out
}

// Dependents lists the conditions of every requirement on id, in display order.
func (p *Project) Dependents(id string) []ReqCond {
	var out []ReqCond
	for _, o := range p.Order {
		for _, rc := range p.Reqs(o) {
			if rc.Requires == id {
				out = append(out, rc)
			}
		}
	}
	return out
}
