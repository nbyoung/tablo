package main

import (
	"path"
	"sort"
	"strings"
)

// Task is the part of a task file the derivations read.
type Task struct {
	ID, Assignee, Parent string
	Junctions            map[string]any
}

// Repo is a Tableaux project at one commit of a Git repository.
type Repo struct {
	Path   string
	Commit string
	Tasks  map[string]*Task
	Gates  []string
}

func loadRepo(repo, commit string) *Repo {
	r := &Repo{Path: repo, Commit: commit, Tasks: map[string]*Task{}}
	for _, f := range listFiles(repo, commit, ".tableaux/tasks") {
		src, _ := show(repo, commit, f)
		m := asMap(parseYAML(src))
		id := strings.TrimSuffix(path.Base(f), ".yaml")
		r.Tasks[id] = &Task{ID: id, Assignee: str(m["assignee"]), Parent: str(asMap(m["parent"])["id"]), Junctions: asMap(m["junctions"])}
	}
	if src, ok := show(repo, commit, ".tableaux/gates.yaml"); ok {
		for _, g := range asList(asMap(parseYAML(src))["gates"]) {
			r.Gates = append(r.Gates, str(asMap(g)["key"]))
		}
	}
	return r
}

func (r *Repo) gateIndex(g string) int {
	for i, k := range r.Gates {
		if k == g {
			return i
		}
	}
	return -1
}

// Junction is a resolved junction with the task that supplies each field.
type Junction struct {
	Kind        string // plain, recursive or not_applicable
	Contributor string
	Model       string
	Reviewer    string
	Refs        int
	Subproject  map[string]any
	Prov        map[string]string // field -> supplying task, or "default"
	Source      []string          // supplying tasks, nearest first
	Undecided   string            // F7 when an exemption sits between the entry and an ancestor's
}

// resolve reads a task's junction at a gate: its own entry, then its
// ancestors' nearest first, then the plain default; a plain entry inherits
// field by field and a not-applicable entry stops the walk.
func (r *Repo) resolve(id, gate string) Junction {
	t := r.Tasks[id]
	j := Junction{Kind: "plain", Prov: map[string]string{}}
	seen := map[string]bool{}
	stopped := false
	for cur := t; cur != nil && !seen[cur.ID]; cur = r.Tasks[cur.Parent] {
		seen[cur.ID] = true
		e := asMap(asMap(cur.Junctions)[gate])
		if e == nil {
			continue
		}
		if sp := asMap(e["subproject"]); sp != nil {
			if cur == t {
				return Junction{Kind: "recursive", Subproject: sp, Source: []string{id}, Prov: map[string]string{"subproject": id}}
			}
			continue // a recursive junction names one task's work and does not inherit
		}
		if str(e["applies"]) == "false" {
			if !stopped && len(j.Source) == 0 {
				return Junction{Kind: "not_applicable", Source: []string{cur.ID}, Prov: map[string]string{}}
			}
			stopped = true
			continue
		}
		if stopped {
			if j.Contributor == "" || j.Reviewer == "" || j.Refs == 0 {
				if e["contributor"] != nil || e["reviewer"] != nil || e["references"] != nil {
					j.Undecided = "F7"
				}
			}
			continue
		}
		used := false
		if (e["contributor"] != nil || e["model"] != nil) && j.Contributor == "" && j.Model == "" {
			j.Contributor, j.Model = str(e["contributor"]), str(e["model"])
			j.Prov["contributor"], used = cur.ID, true
			if j.Model != "" {
				j.Prov["model"] = cur.ID
			}
		}
		if e["reviewer"] != nil && j.Reviewer == "" {
			j.Reviewer, j.Prov["reviewer"], used = str(e["reviewer"]), cur.ID, true
		}
		if e["references"] != nil && j.Refs == 0 {
			j.Refs, j.Prov["references"], used = len(asList(e["references"])), cur.ID, true
		}
		if used {
			j.Source = append(j.Source, cur.ID)
		}
	}
	if j.Contributor == "" {
		j.Contributor, j.Prov["contributor"] = t.Assignee, "default"
	}
	if j.Model != "" && j.Reviewer == "" { // an agent with no stated reviewer takes the assignee
		j.Reviewer, j.Prov["reviewer"] = t.Assignee, "default (agent)"
	}
	return j
}

// departs reports whether a junction differs from the plain default.
func (j Junction) departs(t *Task) bool {
	return j.Kind != "plain" || j.Model != "" || j.Reviewer != "" || j.Refs > 0 || j.Undecided != "" || j.Contributor != t.Assignee
}

// Trunk is how the trunk resolved.
type Trunk struct {
	How  string // stated, inferred, caller or undetermined
	Ref  string
	Hash string
}

// findTrunk applies the README's order: version.yaml, origin/HEAD, the caller.
func findTrunk(repo, view, caller string) Trunk {
	resolve := func(name string) string {
		for _, c := range []string{"refs/heads/" + name, "refs/remotes/origin/" + name} {
			if h, err := git(repo, "rev-parse", "--verify", "-q", c+"^{commit}"); err == nil {
				return h
			}
		}
		return ""
	}
	if src, ok := show(repo, view, ".tableaux/version.yaml"); ok {
		if name := str(asMap(parseYAML(src))["trunk"]); name != "" {
			return Trunk{"stated", name, resolve(name)}
		}
	}
	if sym, err := git(repo, "symbolic-ref", "-q", "refs/remotes/origin/HEAD"); err == nil {
		if h, err := git(repo, "rev-parse", sym+"^{commit}"); err == nil {
			return Trunk{"inferred", strings.TrimPrefix(sym, "refs/remotes/"), h}
		}
	}
	if caller != "" {
		return Trunk{"caller", caller, resolve(caller)}
	}
	return Trunk{How: "undetermined"}
}

// Auth is a task's authorisation.
type Auth struct {
	State       string // authorised or proposed
	Commit, By  string // deciding commit and its authority-or-proposer
	Authorities []string
}

// authorise derives every task's authorisation for the view. Off the trunk,
// and with no trunk, every task is proposed.
func authorise(view *Repo, tr Trunk) map[string]Auth {
	out := map[string]Auth{}
	onTrunk := tr.Hash != "" && tr.Hash == view.Commit
	for id := range view.Tasks {
		if !onTrunk {
			out[id] = Auth{State: "proposed"}
			continue
		}
		out[id] = decide(view.Path, tr.Hash, id)
	}
	return out
}

// decide finds the newest first-parent commit that changed the task's file or
// carries an Authorised trailer for it, and judges its author and committer
// against the authorities read at that commit.
func decide(repo, trunk, id string) Auth {
	order, _ := git(repo, "rev-list", "--first-parent", trunk)
	idx := map[string]int{}
	for i, h := range strings.Fields(order) {
		idx[h] = i
	}
	best := ""
	for _, args := range [][]string{
		{"log", "--first-parent", "-1", "--format=%H", trunk, "--", ".tableaux/tasks/" + id + ".yaml"},
		{"log", "--first-parent", "-1", "--format=%H", "-E", "--grep=^Authorised: " + id + "$", trunk},
	} {
		h, err := git(repo, args...)
		if err == nil && h != "" && (best == "" || idx[h] < idx[best]) {
			best = h
		}
	}
	if best == "" {
		return Auth{State: "proposed"}
	}
	at := loadRepo(repo, best)
	var authorities []string
	t := at.Tasks[id]
	for p := t; p != nil && p.Parent != ""; {
		p = at.Tasks[p.Parent]
		if p != nil {
			authorities = append(authorities, p.Assignee)
		}
	}
	if t != nil && t.Parent == "" { // the owner alone authorises the root, as the tree stood before the change
		owner := t.Assignee
		if prev, err := git(repo, "rev-parse", "-q", "--verify", best+"^1"); err == nil {
			if pt := loadRepo(repo, prev).Tasks[id]; pt != nil {
				owner = pt.Assignee
			}
		}
		authorities = []string{owner}
	}
	who, _ := git(repo, "show", "-s", "--format=%ae %ce", best)
	f := strings.Fields(who)
	a := Auth{State: "proposed", Commit: best, By: f[0], Authorities: authorities}
	for _, e := range f {
		for _, au := range authorities {
			if e == au {
				a.State, a.By = "authorised", e
			}
		}
	}
	if t != nil && t.Parent == "" {
		a.Authorities = nil
	}
	return a
}

// Review is a reviewed junction the status passes and the commit that accepts it.
type Review struct {
	Gate, Reviewer, Commit string
	Ignored                []string // Reviewed: commits from someone else
}

// reviews lists, for a task at a status gate, each passed junction with a
// reviewer and the commit in the view's history that the reviewer made.
func reviews(view *Repo, id, statusGate string, auth Auth) []Review {
	var out []Review
	upto := view.gateIndex(statusGate)
	for i, g := range view.Gates {
		if i > upto || g == "undefined" {
			continue
		}
		j := view.resolve(id, g)
		if j.Kind != "plain" || j.Reviewer == "" {
			continue
		}
		if g == "defined" && auth.State == "authorised" {
			continue // authorisation stands as the review of defined
		}
		rv := Review{Gate: g, Reviewer: j.Reviewer}
		logs, _ := git(view.Path, "log", "--format=%H %ae %ce", "-E", "--grep=^Reviewed: "+id+" "+g+"$", view.Commit)
		lines := strings.Split(logs, "\n")
		for k := len(lines) - 1; k >= 0; k-- { // oldest first: the first acceptance completes the gate
			f := strings.Fields(lines[k])
			switch {
			case len(f) != 3:
			case f[1] == j.Reviewer || f[2] == j.Reviewer:
				if rv.Commit == "" {
					rv.Commit = f[0]
				}
			default:
				rv.Ignored = append(rv.Ignored, f[0])
			}
		}
		out = append(out, rv)
	}
	return out
}

func sortedIDs(m map[string]*Task) []string {
	ids := make([]string, 0, len(m))
	for id := range m {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
