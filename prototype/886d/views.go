package main

import "sort"

// Options carry the abstract views' focusing parameters, and the open
// choices of VIEWS.md as switches so the output shows both readings.
type Options struct {
	Task, Person string
	Window       int    // Q2: columns either side of the next gates
	Cell         string // Q5: "state-at-gate" (default) or "state-at-next"
	Order        string // Q3: "kind" (default) or "dependents"
	Level        string // "glance" or "detail"
}

type Col struct {
	Gate   string `json:"gate"`
	Symbol string `json:"symbol"`
}

type Fold struct {
	Side  string         `json:"side"`
	From  string         `json:"from"`
	To    string         `json:"to"`
	Count int            `json:"count"`
	Gates map[string]int `json:"by_gate"`
}

type Cell struct {
	Gate    string `json:"gate"`
	Kind    string `json:"kind"` // status, marks, exempt, blank
	Symbols string `json:"symbols"`
	Acts    bool   `json:"person_acts,omitempty"`
}

type Row struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Depth      int     `json:"depth"`
	Parent     bool    `json:"parent"`
	Role       string  `json:"role,omitempty"`
	Hidden     int     `json:"collapsed_children,omitempty"`
	Authorised bool    `json:"authorised"`
	Status     *Status `json:"status"`
	NextGate   string  `json:"next_gate,omitempty"`
	Cells      []Cell  `json:"cells"`
}

type Tableau struct {
	View      string   `json:"view"`
	Ref       string   `json:"ref"`
	Task      string   `json:"task,omitempty"`
	Person    string   `json:"person,omitempty"`
	Level     string   `json:"level"`
	WindowN   int      `json:"window"`
	CellRule  string   `json:"cell_rule"`
	NextGates []string `json:"next_gates_in_view"`
	Columns   []Col    `json:"columns"`
	Folded    []Fold   `json:"folded"`
	Rows      []Row    `json:"rows"`
}

func (p *Project) mark(j Junction) string {
	switch j.Kind {
	case "not_applicable":
		return "—"
	case "recursive":
		return "🪆"
	}
	m := "🧑"
	if j.Model != "" {
		m = "🤖"
	}
	if j.Reviewer != "" && j.Reviewer != j.Contributor {
		m += "👀"
	}
	return m
}

func (p *Project) stateSymbol(s *Status) string {
	out := p.States[s.State].Symbol
	if s.Reason != "" {
		out += p.Reasons[s.Reason]
	}
	return out
}

// window returns the shown gate range: n columns either side of the next
// gates of the tasks in view, clipped to the gates.
func (p *Project) window(ids []string, n int) (lo, hi int, nexts []string) {
	lo, hi = len(p.Gates), -1
	seen := map[string]bool{}
	for _, id := range ids {
		g := p.NextGate(id, p.Status(id).Gate)
		if g == "" {
			continue
		}
		if !seen[g] {
			seen[g] = true
			nexts = append(nexts, g)
		}
		i := p.gidx[g]
		lo, hi = min(lo, i), max(hi, i)
	}
	if hi < 0 {
		return 0, len(p.Gates) - 1, nil
	}
	sort.Slice(nexts, func(a, b int) bool { return p.gidx[nexts[a]] < p.gidx[nexts[b]] })
	return max(0, lo-n), min(len(p.Gates)-1, hi+n), nexts
}

func (p *Project) row(id string, o Options, cols []Col) Row {
	t := p.Tasks[id]
	s := p.Status(id)
	auth, _ := p.Authorised(id)
	next := p.NextGate(id, s.Gate)
	r := Row{ID: id, Title: t.Title, Depth: t.Depth, Parent: !p.IsLeaf(id), Authorised: auth,
		Status: s, NextGate: next}
	at := s.Gate
	if o.Cell == "state-at-next" && next != "" {
		at = next
	}
	for _, c := range cols {
		j := p.Junction(id, c.Gate)
		cell := Cell{Gate: c.Gate, Kind: "marks", Symbols: p.mark(j)}
		switch {
		case c.Gate == at:
			cell.Kind, cell.Symbols = "status", p.stateSymbol(s)
		case c.Gate == "undefined":
			cell.Kind, cell.Symbols = "blank", ""
		case j.Kind == "not_applicable":
			cell.Kind = "exempt"
		}
		cell.Acts = o.Person != "" && j.Kind == "plain" && (j.Contributor == o.Person || j.Reviewer == o.Person)
		r.Cells = append(r.Cells, cell)
	}
	return r
}

// build lays the rows out over the window and counts the folded columns.
func (p *Project) build(view string, ids []string, roles map[string]string, o Options) *Tableau {
	lo, hi, nexts := p.window(ids, o.Window)
	tb := &Tableau{View: view, Ref: p.ref, Task: o.Task, Person: o.Person, Level: o.Level,
		WindowN: o.Window, CellRule: o.Cell, NextGates: nexts}
	for i := lo; i <= hi; i++ {
		tb.Columns = append(tb.Columns, Col{p.Gates[i].Key, p.Gates[i].Symbol})
	}
	// A folded column counts the leaf tasks in view whose current gate lies in it.
	count := map[string]int{}
	for _, id := range ids {
		if p.IsLeaf(id) {
			count[p.Status(id).Gate]++
		}
	}
	fold := func(side string, from, to int) {
		if from > to {
			return
		}
		f := Fold{Side: side, From: p.Gates[from].Key, To: p.Gates[to].Key, Gates: map[string]int{}}
		for i := from; i <= to; i++ {
			f.Gates[p.Gates[i].Key] = count[p.Gates[i].Key]
			f.Count += count[p.Gates[i].Key]
		}
		tb.Folded = append(tb.Folded, f)
	}
	fold("before", 0, lo-1)
	fold("after", hi+1, len(p.Gates)-1)
	in := map[string]bool{}
	for _, id := range ids {
		in[id] = true
	}
	for _, id := range ids {
		r := p.row(id, o, tb.Columns)
		r.Role = roles[id]
		for _, c := range p.Tasks[id].Children {
			if !in[c] {
				r.Hidden++
			}
		}
		tb.Rows = append(tb.Rows, r)
	}
	return tb
}

func (p *Project) subtree(id string) []string {
	out := []string{id}
	for _, c := range p.Tasks[id].Children {
		out = append(out, p.subtree(c)...)
	}
	return out
}

// Global is the global tableau: every task in display order. At glance the
// rows stop at depth one.
func (p *Project) Global(o Options) *Tableau {
	ids := p.Order
	if o.Level == "glance" {
		ids = nil
		for _, id := range p.Order {
			if p.Tasks[id].Depth <= 1 {
				ids = append(ids, id)
			}
		}
	}
	return p.build("global-tableau", ids, nil, o)
}

// Contextual is the contextual tableau, in the task form (the subtree under a
// task) or the person form (Q7: the person's leaf tasks, their spine and their
// siblings).
func (p *Project) Contextual(o Options) *Tableau {
	if o.Task != "" {
		ids := p.subtree(o.Task)
		if o.Level == "glance" {
			ids = append([]string{o.Task}, p.Tasks[o.Task].Children...)
		}
		return p.build("contextual-tableau", ids, nil, o)
	}
	roles := map[string]string{}
	for _, id := range p.Order {
		if !p.IsLeaf(id) {
			continue
		}
		nj := p.Junction(id, p.NextGate(id, p.Status(id).Gate))
		if p.Tasks[id].Assignee == o.Person || (nj.Kind == "plain" && nj.Contributor == o.Person) {
			roles[id] = "corner"
		}
	}
	for id := range roles {
		for a := p.Tasks[id].Parent; a != ""; a = p.Tasks[a].Parent {
			if roles[a] == "" {
				roles[a] = "spine"
			}
		}
	}
	for id := range roles {
		if roles[id] != "corner" {
			continue
		}
		if par := p.Tasks[id].Parent; par != "" {
			for _, s := range p.Tasks[par].Children {
				if roles[s] == "" {
					roles[s] = "sibling"
				}
			}
		}
	}
	var ids []string
	for _, id := range p.Order {
		if roles[id] != "" {
			ids = append(ids, id)
		}
	}
	return p.build("contextual-tableau", ids, roles, o)
}
