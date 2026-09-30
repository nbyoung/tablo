package main

import "sort"

type Held struct {
	Task  string `json:"task"`
	Title string `json:"title"`
	Gate  string `json:"gate"`
	Holds []Held `json:"holds,omitempty"`
}

type Cause struct {
	Kind     string `json:"kind"`
	Cause    string `json:"cause"`
	Task     string `json:"task"`
	Resolver string `json:"resolver"`
	Action   string `json:"action"`
	Count    int    `json:"holds"`
	Held     []Held `json:"tree"`
}

type Blockage struct {
	View     string    `json:"view"`
	Ref      string    `json:"ref"`
	Person   string    `json:"person,omitempty"`
	Causes   []Cause   `json:"causes"`
	NotDue   []ReqCond `json:"not_yet_due"`
	NotShown []string  `json:"not_derived"`
}

// resolver is the contributor at the task's next junction, or the assignee.
func (p *Project) resolver(id string) string {
	if g := p.NextGate(id, p.Status(id).Gate); g != "" {
		if j := p.Junction(id, g); j.Kind == "plain" {
			return j.Contributor
		}
	}
	return p.Tasks[id].Assignee
}

// held lists what a held task holds in turn: the tasks whose requirement on it
// is unmet, transitively. A task on the path stops the recursion.
func (p *Project) held(id string, path map[string]bool) []Held {
	var out []Held
	path[id] = true
	defer delete(path, id)
	for _, d := range p.Dependents(id) {
		if d.Condition != "unmet" || path[d.Task] {
			continue
		}
		out = append(out, Held{d.Task, p.Tasks[d.Task].Title, d.To, p.held(d.Task, path)})
	}
	return out
}

func countHeld(h []Held) int {
	n := len(h)
	for _, x := range h {
		n += countHeld(x.Holds)
	}
	return n
}

// Blockage derives the work-blockage tree from three of the five causes:
// an unmet requirement, a stalled/at-risk/blocked/overloaded status, and an
// authorisation outstanding. A review outstanding needs a status with the
// reason review and a pin that has not advanced needs a moving subproject;
// the weather station has neither.
func (p *Project) Blockage(o Options) *Blockage {
	b := &Blockage{View: "work-blockage-tree", Ref: p.ref, Person: o.Person, Causes: []Cause{}, NotDue: []ReqCond{},
		NotShown: []string{"review outstanding", "subproject pin not advanced"}}
	add := func(c Cause) {
		c.Count = countHeld(c.Held)
		if o.Person == "" || c.Resolver == o.Person {
			b.Causes = append(b.Causes, c)
		}
	}
	for _, id := range p.Order {
		t := p.Tasks[id]
		for _, r := range p.Reqs(id) {
			switch r.Condition {
			case "unmet":
				add(Cause{Kind: "unmet-requirement", Task: r.Requires,
					Cause:    r.Requires + " " + p.Tasks[r.Requires].Title + " has not passed " + r.From,
					Resolver: p.resolver(r.Requires),
					Action:   "record " + r.Requires + " through " + r.From,
					Held:     []Held{{id, t.Title, r.To, p.held(id, map[string]bool{r.Requires: true})}}})
			case "pending":
				b.NotDue = append(b.NotDue, r)
			}
		}
		if s := p.Status(id); p.IsLeaf(id) && (s.Reason == "blocked" || s.Reason == "overloaded" ||
			s.State == "at_risk" || s.State == "stalled") {
			add(Cause{Kind: "status", Task: id,
				Cause:    id + " " + t.Title + " is " + s.State + " at " + s.Gate + ": " + s.Note,
				Resolver: p.resolver(id),
				Action:   "clear the " + s.Reason + " reason, or record a new status",
				Held: append([]Held{{id, t.Title, p.NextGate(id, s.Gate), nil}},
					p.held(id, map[string]bool{})...)})
		}
		if ok, _ := p.Authorised(id); !ok {
			auth := p.Authorities(id)
			add(Cause{Kind: "authorisation", Task: id,
				Cause: id + " " + t.Title + " is proposed",
				Resolver: func() string {
					if len(auth) > 0 {
						return auth[0]
					}
					return p.Owner
				}(),
				Action: "commit a trailer: Authorised: " + id,
				Held: append([]Held{{id, t.Title, p.NextGate(id, p.Status(id).Gate), nil}},
					p.held(id, map[string]bool{})...)})
		}
	}
	sort.SliceStable(b.Causes, func(i, j int) bool { return b.Causes[i].Count > b.Causes[j].Count })
	return b
}

type Item struct {
	Kind        string `json:"kind"`
	Task        string `json:"task"`
	Title       string `json:"title"`
	Gate        string `json:"gate,omitempty"`
	Model       string `json:"model,omitempty"`
	Reviewer    string `json:"reviewer,omitempty"`
	Cause       string `json:"cause,omitempty"`
	Date        string `json:"status_date,omitempty"`
	Dependents  int    `json:"dependents"`
	Requirement string `json:"requirement,omitempty"`
}

type Queue struct {
	View   string `json:"view"`
	Ref    string `json:"ref"`
	Person string `json:"person"`
	Order  string `json:"order"`
	Items  []Item `json:"items"`
}

// Queue lists what a person does next, in the five kinds of VIEWS.md.
func (p *Project) Queue(o Options) *Queue {
	q := &Queue{View: "contributor-work-queue", Ref: p.ref, Person: o.Person, Order: o.Order, Items: []Item{}}
	var reviews, auths, ready, reaff, waiting []Item
	for _, id := range p.Order {
		t := p.Tasks[id]
		if ok, _ := p.Authorised(id); !ok && has(p.Authorities(id), o.Person) {
			auths = append(auths, Item{Kind: "authorisation owed", Task: id, Title: t.Title,
				Cause: "proposed; the deciding commit is not by an authority"})
		}
		if !p.IsLeaf(id) {
			continue
		}
		s := p.Status(id)
		next := p.NextGate(id, s.Gate)
		if next != "" {
			j := p.Junction(id, next)
			it := Item{Task: id, Title: t.Title, Gate: next, Model: j.Model, Reviewer: j.Reviewer,
				Date: s.Date, Dependents: len(p.Dependents(id))}
			if s.Reason == "review" && j.Reviewer == o.Person {
				it.Kind = "review owed"
				reviews = append(reviews, it)
			}
			if j.Kind == "plain" && j.Contributor == o.Person {
				var wait string
				for _, r := range p.Reqs(id) {
					if r.Condition == "unmet" {
						wait = "waits for " + r.Requires + " at " + r.From
						it.Requirement = r.Requires + " " + r.From + " to " + r.To
					}
				}
				switch {
				case wait != "":
				case s.Reason == "blocked" || s.Reason == "overloaded":
					wait = s.Reason + ": " + s.Note
				case s.Reason == "review":
					wait = "waits for review by " + j.Reviewer
				}
				if wait == "" {
					it.Kind = "work ready"
					ready = append(ready, it)
				} else {
					it.Kind, it.Cause = "work waiting", wait
					waiting = append(waiting, it)
				}
			}
		}
		if s.Recorder == o.Person && !s.Snapshot && t.File != nil {
			reaff = append(reaff, Item{Kind: "reaffirmation", Task: id, Title: t.Title, Gate: s.Gate, Date: s.Date})
		}
	}
	sort.SliceStable(reaff, func(i, j int) bool { return reaff[i].Date < reaff[j].Date })
	if o.Order == "dependents" {
		sort.SliceStable(ready, func(i, j int) bool { return ready[i].Dependents > ready[j].Dependents })
	}
	for _, l := range [][]Item{reviews, auths, ready, reaff, waiting} {
		q.Items = append(q.Items, l...)
	}
	return q
}
