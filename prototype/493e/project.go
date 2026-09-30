package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

// Stand-in for the loader (task 27a3): reads .tableaux at a commit through git.

func git(repo string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

type Gate struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name"`
	Criteria string `json:"criteria"`
}

type State struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Severity int    `json:"severity"`
	Synopsis string `json:"synopsis"`
}

type Reason struct {
	Key      string `json:"key"`
	Symbol   string `json:"symbol"`
	Synopsis string `json:"synopsis"`
}

type Requirement struct {
	ID, From, To, Text string
}

type Task struct {
	ID, Title, Description, Assignee string
	References                       []any
	Requires                         []Requirement
	Junctions                        map[string]any
	Parent                           string
	Order                            int
	HasOrder                         bool
	Children                         []string
	Blob                             string
}

type Project struct {
	Tableaux, Trunk string
	Gates           []Gate
	States          []State
	Reasons         []Reason
	Tasks           map[string]*Task
	Root            string
	statuses        map[string]map[string]any
}

func (p *Project) gateIndex(key string) int {
	for i, g := range p.Gates {
		if g.Key == key {
			return i
		}
	}
	return -1
}

func (p *Project) severity(state string) int {
	for _, s := range p.States {
		if s.Key == state {
			return s.Severity
		}
	}
	return 0
}

// load reads the project at a commit.
func load(repo, commit string) (*Project, error) {
	names, err := git(repo, "ls-tree", "-r", "--full-tree", commit, ".tableaux")
	if err != nil {
		return nil, err
	}
	p := &Project{Tasks: map[string]*Task{}, statuses: map[string]map[string]any{}}
	read := func(path string) (map[string]any, error) {
		src, err := git(repo, "show", commit+":"+path)
		if err != nil {
			return nil, err
		}
		m, err := parseYAML(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return m, nil
	}
	for _, line := range strings.Split(strings.TrimSpace(names), "\n") {
		meta, path, _ := strings.Cut(line, "\t")
		blob := strings.Fields(meta)[2]
		dir, file := ".tableaux/", strings.TrimPrefix(path, ".tableaux/")
		m := map[string]any(nil)
		switch {
		case file == "gates.yaml" || file == "version.yaml" ||
			strings.HasPrefix(file, "tasks/") || strings.HasPrefix(file, "status/"):
			if m, err = read(dir + file); err != nil {
				return nil, err
			}
		default:
			continue
		}
		id := strings.TrimSuffix(file[strings.LastIndex(file, "/")+1:], ".yaml")
		switch {
		case file == "version.yaml":
			p.Tableaux, p.Trunk = str(m, "tableaux"), str(m, "trunk")
		case file == "gates.yaml":
			for _, g := range lst(m["gates"]) {
				g := mp(g)
				p.Gates = append(p.Gates, Gate{str(g, "key"), str(g, "symbol"), str(g, "name"), str(g, "criteria")})
			}
			for _, s := range lst(m["states"]) {
				s := mp(s)
				sev, _ := s["severity"].(int)
				p.States = append(p.States, State{str(s, "key"), str(s, "symbol"), sev, str(s, "synopsis")})
			}
			for _, r := range lst(m["reasons"]) {
				r := mp(r)
				p.Reasons = append(p.Reasons, Reason{str(r, "key"), str(r, "symbol"), str(r, "synopsis")})
			}
		case strings.HasPrefix(file, "status/"):
			p.statuses[id] = m
		default:
			t := &Task{ID: id, Title: str(m, "title"), Description: str(m, "description"),
				Assignee: str(m, "assignee"), References: lst(m["references"]),
				Junctions: mp(m["junctions"]), Blob: blob}
			for _, r := range lst(m["requires"]) {
				r := mp(r)
				t.Requires = append(t.Requires, Requirement{str(r, "id"), str(r, "from"), str(r, "to"), str(r, "text")})
			}
			if par := mp(m["parent"]); par != nil {
				t.Parent = str(par, "id")
				t.Order, t.HasOrder = par["order"].(int)
			} else {
				p.Root = id
			}
			p.Tasks[id] = t
		}
	}
	for _, t := range p.Tasks {
		if t.Parent != "" {
			par := p.Tasks[t.Parent]
			if par == nil {
				return nil, fmt.Errorf("task %s: parent %s missing", t.ID, t.Parent)
			}
			par.Children = append(par.Children, t.ID)
		}
	}
	for _, t := range p.Tasks {
		sort.Slice(t.Children, func(i, j int) bool { return p.before(t.Children[i], t.Children[j]) })
	}
	return p, nil
}

// before orders siblings: order ascending with unordered last, then id.
func (p *Project) before(a, b string) bool {
	x, y := p.Tasks[a], p.Tasks[b]
	if x.HasOrder != y.HasOrder {
		return x.HasOrder
	}
	if x.Order != y.Order {
		return x.Order < y.Order
	}
	return a < b
}

// walk lists the subtree under id depth first, with depths.
func (p *Project) walk(id string) (ids []string, depth map[string]int) {
	depth = map[string]int{}
	var rec func(id string, d int)
	rec = func(id string, d int) {
		ids = append(ids, id)
		depth[id] = d
		for _, c := range p.Tasks[id].Children {
			rec(c, d+1)
		}
	}
	rec(id, 0)
	return ids, depth
}

// ancestors lists the ancestors of id, nearest first.
func (p *Project) ancestors(id string) []string {
	var out []string
	for t := p.Tasks[id]; t.Parent != ""; t = p.Tasks[t.Parent] {
		out = append(out, t.Parent)
	}
	return out
}

// authorities lists the assignees of the ancestors, nearest first, without repeats.
func (p *Project) authorities(id string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range p.ancestors(id) {
		if e := p.Tasks[a].Assignee; !seen[e] {
			seen[e] = true
			out = append(out, e)
		}
	}
	return out
}

// Junction is a task's junction at a gate, resolved.
type Junction struct {
	Gate        string            `json:"gate"`
	Kind        string            `json:"kind"`
	Contributor string            `json:"contributor,omitempty"`
	Model       string            `json:"model,omitempty"`
	Reviewer    string            `json:"reviewer,omitempty"`
	References  []any             `json:"references,omitempty"`
	Subproject  map[string]any    `json:"subproject,omitempty"`
	Source      string            `json:"source,omitempty"` // nearest task whose entry contributes; empty for the plain default
	Sources     map[string]string `json:"sources"`          // per field: a task id, "assignee" or "default"
}

// junction resolves the task's entry at a gate from its own file, its
// ancestors' files nearest first, then the plain default.
func (p *Project) junction(id, gate string) Junction {
	t := p.Tasks[id]
	j := Junction{Gate: gate, Kind: "plain", Contributor: t.Assignee, Sources: map[string]string{"contributor": "default"}}
	if gate == "undefined" {
		return j
	}
	var gotC, gotR, gotRef bool
	for i, cid := range append([]string{id}, p.ancestors(id)...) {
		e := mp(p.Tasks[cid].Junctions[gate])
		if e == nil {
			continue
		}
		if e["applies"] == false {
			if j.Source == "" {
				return Junction{Gate: gate, Kind: "not_applicable", Source: cid, Sources: map[string]string{"applies": cid}}
			}
			break
		}
		if sp := mp(e["subproject"]); sp != nil && i == 0 {
			return Junction{Gate: gate, Kind: "recursive", Subproject: sp, Source: cid, Sources: map[string]string{"subproject": cid}}
		}
		if j.Source == "" {
			j.Source = cid
		}
		if !gotC && (e["contributor"] != nil || e["model"] != nil) {
			gotC = true
			j.Contributor, j.Model = str(e, "contributor"), str(e, "model")
			j.Sources["contributor"] = cid
		}
		if !gotR && e["reviewer"] != nil {
			gotR = true
			j.Reviewer, j.Sources["reviewer"] = str(e, "reviewer"), cid
		}
		if !gotRef && e["references"] != nil {
			gotRef = true
			j.References, j.Sources["references"] = lst(e["references"]), cid
		}
	}
	if j.Model != "" && j.Reviewer == "" {
		j.Reviewer, j.Sources["reviewer"] = t.Assignee, "assignee"
	}
	return j
}

func (p *Project) applicable(id string) []string {
	var out []string
	for _, g := range p.Gates {
		if p.junction(id, g.Key).Kind != "not_applicable" {
			out = append(out, g.Key)
		}
	}
	return out
}

// nextGate is the first applicable gate after the given one, or "".
func (p *Project) nextGate(id, after string) string {
	for _, g := range p.applicable(id) {
		if p.gateIndex(g) > p.gateIndex(after) {
			return g
		}
	}
	return ""
}
