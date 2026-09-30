package main

import (
	"fmt"
	"sort"
	"strings"
)

// Params are the focusing parameters every view takes (VIEWS.md, Parameters).
type Params struct {
	Task     string   `json:"task,omitempty"`
	Person   string   `json:"person,omitempty"`
	Level    string   `json:"level"`              // glance, detail or provenance; the view includes every level up to it
	Columns  []string `json:"columns,omitempty"`  // explicit gate columns
	Window   int      `json:"window"`             // columns either side of the next gates; -1 leaves the view's default
	Proposed bool     `json:"proposed,omitempty"` // authority view: proposed tasks only
}

type M = map[string]any

// marks are the junction marks; they belong to the method, not to a project.
var marks = []M{
	{"symbol": "🤖", "meaning": "an agent contributes"},
	{"symbol": "👀", "meaning": "a reviewer accepts"},
	{"symbol": "🪆", "meaning": "a subproject does the work"},
	{"symbol": "—", "meaning": "the gate does not apply"},
	{"symbol": "🧑", "meaning": "a person contributes"},
}

func render(r *Repo, view string, pr Params) (M, error) {
	if pr.Task != "" && r.proj.Tasks[pr.Task] == nil {
		return nil, fmt.Errorf("no task %q at %s", pr.Task, r.ref[:7])
	}
	var g, d, pv M
	var err error
	switch view {
	case "gate":
		g, d, pv = r.gateView(pr)
	case "task":
		if pr.Task == "" {
			return nil, fmt.Errorf("the task view needs --task")
		}
		g, d, pv = r.taskView(pr)
	case "authority":
		g, d, pv = r.authorityView(pr)
	case "assignment":
		g, d, pv = r.assignmentView(pr)
	default:
		err = fmt.Errorf("unknown view %q (gate, task, authority, assignment)", view)
	}
	if err != nil {
		return nil, err
	}
	out := M{"view": view, "ref": r.ref, "on_trunk": r.onTrunk, "params": pr, "glance": g}
	switch pr.Level {
	case "glance":
	case "detail":
		out["detail"] = d
	default:
		out["detail"], out["provenance"] = d, pv
	}
	return out, nil
}

// columns picks the gate columns: explicit ones, else a window of n either
// side of the next gates of the leaves in view, else nil for every gate.
func (r *Repo) columns(pr Params, ids []string, defaultN int) []string {
	if len(pr.Columns) > 0 {
		return pr.Columns
	}
	n := pr.Window
	if n < 0 {
		n = defaultN
	}
	if n < 0 {
		return nil
	}
	lo, hi := len(r.proj.Gates), -1
	for _, id := range ids {
		if len(r.proj.Tasks[id].Children) > 0 {
			continue
		}
		if nx := r.proj.nextGate(id, r.status(id).Gate); nx != "" {
			i := r.proj.gateIndex(nx)
			lo, hi = min(lo, i), max(hi, i)
		}
	}
	if hi < 0 {
		return nil
	}
	var out []string
	for i, g := range r.proj.Gates {
		if i >= lo-n && i <= hi+n {
			out = append(out, g.Key)
		}
	}
	return out
}

func (r *Repo) fileCommits(paths ...string) M {
	out := M{}
	for _, p := range paths {
		if l := r.pathLog(p, false); len(l) > 0 {
			out[p] = r.commits[l[0]]
		}
	}
	return out
}

// gateView answers: what do the columns and symbols mean?
func (r *Repo) gateView(pr Params) (g, d, pv M) {
	p := r.proj
	cols := r.columns(pr, nil, -1)
	if pr.Task != "" {
		cols = r.columns(pr, []string{pr.Task}, -1)
	}
	var gl, dt []M
	for _, gt := range p.Gates {
		if cols != nil && !has(cols, gt.Key) {
			continue
		}
		gl = append(gl, M{"key": gt.Key, "symbol": gt.Symbol, "name": gt.Name})
		e := M{"key": gt.Key, "criteria": gt.Criteria}
		if pr.Task != "" {
			j := p.junction(pr.Task, gt.Key)
			e["applies"] = j.Kind != "not_applicable"
			e["references"] = j.References
			e["references_from"] = j.Sources["references"]
		}
		dt = append(dt, e)
	}
	folded := len(p.Gates) - len(gl)
	st, rs := []M{}, []M{}
	ds, dr := []M{}, []M{}
	for _, s := range p.States {
		st = append(st, M{"key": s.Key, "symbol": s.Symbol})
		ds = append(ds, M{"key": s.Key, "severity": s.Severity, "synopsis": s.Synopsis})
	}
	for _, x := range p.Reasons {
		rs = append(rs, M{"key": x.Key, "symbol": x.Symbol})
		dr = append(dr, M{"key": x.Key, "synopsis": x.Synopsis})
	}
	return M{"gates": gl, "folded": folded, "states": st, "reasons": rs, "marks": marks},
		M{"gates": dt, "states": ds, "reasons": dr},
		M{"tableaux": p.Tableaux, "trunk": p.Trunk, "last_changed": r.fileCommits(".tableaux/gates.yaml", ".tableaux/version.yaml")}
}

func markOf(j Junction) []string {
	switch j.Kind {
	case "not_applicable":
		return []string{"—"}
	case "recursive":
		return []string{"🪆"}
	}
	var m []string
	if j.Model != "" {
		m = append(m, "🤖")
	} else if j.Contributor != "" {
		m = append(m, "🧑")
	}
	if j.Reviewer != "" {
		m = append(m, "👀")
	}
	return m
}

// taskView answers: what is this task and where does it stand?
func (r *Repo) taskView(pr Params) (g, d, pv M) {
	p := r.proj
	t := p.Tasks[pr.Task]
	st := r.status(t.ID)
	auth := r.authorisation(t.ID)
	var parent M
	if t.Parent != "" {
		pt := p.Tasks[t.Parent]
		parent = M{"id": pt.ID, "title": pt.Title, "order": t.Order}
	}
	next := p.nextGate(t.ID, st.Gate)
	cols := r.columns(pr, nil, -1)
	if len(pr.Columns) == 0 && pr.Window >= 0 {
		cols = r.columns(pr, []string{t.ID}, -1)
	}
	var js, src []M
	for _, gt := range p.Gates {
		if cols != nil && !has(cols, gt.Key) {
			continue
		}
		j := p.junction(t.ID, gt.Key)
		js = append(js, M{"gate": gt.Key, "symbol": gt.Symbol, "kind": j.Kind, "marks": markOf(j), "contributor": j.Contributor,
			"model": j.Model, "reviewer": j.Reviewer, "references": j.References, "subproject": j.Subproject})
		src = append(src, M{"gate": gt.Key, "source": j.Source, "sources": j.Sources})
	}
	ev := r.events(t.ID)
	if len(ev) > 5 {
		ev = ev[len(ev)-5:]
	}
	as := M{"state": auth.State}
	if auth.State == "authorised" {
		as["way"] = auth.Way
	}
	g = M{"id": t.ID, "title": t.Title, "assignee": t.Assignee, "parent": parent, "next_gate": next, "status": st}
	d = M{"description": t.Description, "references": t.References, "requires": r.requirements(t.ID),
		"dependents": r.dependents(t.ID), "junctions": js, "authorisation": as, "children": t.Children,
		"authorities": p.authorities(t.ID)}
	pv = M{"authorisation": auth, "status_commit": r.commits[st.Commit], "junction_sources": src,
		"reviews": r.reviews(t.ID), "events": ev,
		"command": "git log --format='%h %as %ae %s' -- .tableaux/tasks/" + t.ID + ".yaml .tableaux/status/" + t.ID + ".yaml"}
	return g, d, pv
}

// authorityView answers: who may accept what?
func (r *Repo) authorityView(pr Params) (g, d, pv M) {
	p := r.proj
	root := pr.Task
	if root == "" {
		root = p.Root
	}
	ids, depth := p.walk(root)
	if pr.Person != "" {
		keep := map[string]bool{}
		for _, id := range ids {
			if has(p.authorities(id), pr.Person) || (id == p.Root && p.Tasks[id].Assignee == pr.Person) {
				keep[id] = true
			}
			if p.Tasks[id].Assignee == pr.Person {
				for _, a := range p.ancestors(id) {
					keep[a] = true
				}
			}
		}
		var f []string
		for _, id := range ids {
			if keep[id] {
				f = append(f, id)
			}
		}
		ids = f
	}
	var gr, dr, pr2 []M
	for _, id := range ids {
		t := p.Tasks[id]
		a := r.authorisation(id)
		if pr.Proposed && a.State != "proposed" {
			continue
		}
		delegated := t.Parent != "" && p.Tasks[t.Parent].Assignee != t.Assignee
		gr = append(gr, M{"id": id, "title": t.Title, "depth": depth[id], "assignee": t.Assignee, "delegated": delegated,
			"authorisation": a.State, "differs_from_trunk": a.Differ, "children": len(t.Children)})
		row := M{"id": id, "authorities": append([]string{}, p.authorities(id)...)}
		if len(t.Children) > 0 && len(t.Junctions) > 0 {
			row["junction_defaults"] = t.Junctions
		}
		dr = append(dr, row)
		pr2 = append(pr2, M{"id": id, "commit": a.Commit, "by": a.By, "way": a.Way})
	}
	return M{"rows": gr}, M{"rows": dr}, M{"rows": pr2}
}

// assignmentView answers: what does each person carry?
func (r *Repo) assignmentView(pr Params) (g, d, pv M) {
	p := r.proj
	root := pr.Task
	if root == "" {
		root = p.Root
	}
	ids, _ := p.walk(root)
	cols := r.columns(pr, ids, 0)
	type pos struct {
		id, gate string
		j        Junction
		next     bool
	}
	type card struct {
		assigned        []M
		contrib, review []pos
		byGate          map[string]int
		models          map[string]bool
		authority       []M
		assignedIDs     map[string]bool
	}
	cards := map[string]*card{}
	get := func(e string) *card {
		if e == "" {
			return nil
		}
		if cards[e] == nil {
			cards[e] = &card{byGate: map[string]int{}, models: map[string]bool{}, assignedIDs: map[string]bool{}}
		}
		return cards[e]
	}
	for _, id := range ids {
		t := p.Tasks[id]
		st := r.status(id)
		c := get(t.Assignee)
		c.assigned = append(c.assigned, M{"id": id, "title": t.Title, "status": M{"gate": st.Gate, "state": st.State, "reason": st.Reason}})
		c.assignedIDs[id] = true
		c.byGate[st.Gate]++
		if len(t.Children) > 0 {
			continue
		}
		next := p.nextGate(id, st.Gate)
		for _, gt := range p.Gates {
			j := p.junction(id, gt.Key)
			if j.Kind != "plain" || gt.Key == "undefined" {
				continue
			}
			if cc := get(j.Contributor); cc != nil && j.Model != "" {
				cc.models[j.Model] = true // a person's models span every gate, whatever the window
			}
			if cols != nil && !has(cols, gt.Key) {
				continue
			}
			ps := pos{id, gt.Key, j, gt.Key == next}
			if cc := get(j.Contributor); cc != nil {
				cc.contrib = append(cc.contrib, ps)
			}
			if rc := get(j.Reviewer); rc != nil {
				rc.review = append(rc.review, ps)
			}
		}
	}
	for _, c := range cards {
		for id := range c.assignedIDs {
			if len(p.Tasks[id].Children) == 0 {
				continue
			}
			top := true
			for _, a := range p.ancestors(id) {
				top = top && !c.assignedIDs[a]
			}
			if sub, _ := p.walk(id); top {
				c.authority = append(c.authority, M{"id": id, "title": p.Tasks[id].Title, "descendants": len(sub) - 1})
			}
		}
		sort.Slice(c.authority, func(i, j int) bool { return c.authority[i]["id"].(string) < c.authority[j]["id"].(string) })
	}
	var emails []string
	for e := range cards {
		if pr.Person == "" || e == pr.Person {
			emails = append(emails, e)
		}
	}
	sort.Strings(emails)
	nextOf := func(ps []pos) (n int) {
		for _, x := range ps {
			if x.next {
				n++
			}
		}
		return n
	}
	entry := func(x pos, sources bool) M {
		e := M{"task": x.id, "gate": x.gate, "next": x.next}
		if sources {
			e["sources"], e["source"] = x.j.Sources, x.j.Source
			return e
		}
		e["model"], e["contributor"], e["reviewer"] = x.j.Model, x.j.Contributor, x.j.Reviewer
		return e
	}
	gr, dr, pr2 := []M{}, []M{}, []M{}
	for _, e := range emails {
		c := cards[e]
		models := []string{}
		for m := range c.models {
			models = append(models, m)
		}
		sort.Strings(models)
		gr = append(gr, M{"email": e, "assigned": len(c.assigned), "contributes_next": nextOf(c.contrib),
			"reviews_next": nextOf(c.review), "models": models})
		var cl, rl, cp, rp []M
		for _, x := range c.contrib {
			cl, cp = append(cl, entry(x, false)), append(cp, entry(x, true))
		}
		for _, x := range c.review {
			rl, rp = append(rl, entry(x, false)), append(rp, entry(x, true))
		}
		dr = append(dr, M{"email": e, "assigned": c.assigned, "contributes": cl, "reviews": rl,
			"authority_over": c.authority, "assigned_by_gate": c.byGate})
		pr2 = append(pr2, M{"email": e, "contributes": cp, "reviews": rp})
	}
	return M{"people": gr, "columns": cols}, M{"people": dr}, M{"people": pr2}
}

func split(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}
