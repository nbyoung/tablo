package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Repo is the path of a Git repository.
type Repo string

func (r Repo) git(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", string(r)}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(out), nil
}

// show returns a file at a revision, and false when the file is absent.
func (r Repo) show(rev, path string) (string, bool) {
	out, err := r.git("show", rev+":"+path)
	return out, err == nil
}

// load reads the .tableaux directory at a revision through git ls-tree and
// git show, never through the working tree.
func (r Repo) load(rev string) (*Project, error) {
	gates, ok := r.show(rev, ".tableaux/gates.yaml")
	if !ok {
		return nil, fmt.Errorf("no .tableaux/gates.yaml at %s", rev)
	}
	p := &Project{Tasks: map[string]*Task{}, Status: map[string]string{}, Gates: parseGates(gates), Trunk: "main"}
	if v, ok := r.show(rev, ".tableaux/version.yaml"); ok && scalar(v, "trunk") != "" {
		p.Trunk = scalar(v, "trunk")
	}
	for _, dir := range []string{"tasks", "status"} {
		names, err := r.git("ls-tree", "--name-only", rev, ".tableaux/"+dir+"/")
		if err != nil {
			continue
		}
		for _, path := range strings.Fields(names) {
			id := strings.TrimSuffix(path[strings.LastIndex(path, "/")+1:], ".yaml")
			text, _ := r.show(rev, path)
			if dir == "tasks" {
				p.Tasks[id] = parseTask(id, text)
			} else {
				p.Status[id] = scalar(text, "gate")
			}
		}
	}
	return p, nil
}

// Trailer is one "Key: value" line of a commit message.
type Trailer struct{ Key, Value string }

// Commit is a commit with the fields the audit reads.
type Commit struct {
	Hash, Author, Committer string
	Trailers                []Trailer
	Files                   []string
}

func (c Commit) touches(path string) bool {
	for _, f := range c.Files {
		if f == path {
			return true
		}
	}
	return false
}

func (c Commit) by(email string) bool { return c.Author == email || c.Committer == email }

// log lists the commits reachable from rev, newest first. With firstParent a
// merge lists the files it brings to the trunk.
func (r Repo) log(rev string, firstParent bool) ([]Commit, error) {
	args := []string{"log", "--name-only", "--format=%x1e%H%x1f%ae%x1f%ce%x1f%(trailers:only,unfold)%x1f"}
	if firstParent {
		args = append(args, "--first-parent", "-m")
	}
	out, err := r.git(append(args, rev, "--")...)
	if err != nil {
		return nil, err
	}
	var commits []Commit
	for _, chunk := range strings.Split(out, "\x1e")[1:] {
		f := strings.SplitN(chunk, "\x1f", 5)
		if len(f) < 5 {
			continue
		}
		c := Commit{Hash: f[0], Author: f[1], Committer: f[2]}
		for _, line := range strings.Split(f[3], "\n") {
			if k, v, ok := strings.Cut(line, ": "); ok {
				c.Trailers = append(c.Trailers, Trailer{k, strings.TrimSpace(v)})
			}
		}
		c.Files = strings.Fields(f[4])
		commits = append(commits, c)
	}
	return commits, nil
}

// Finding is one discrepancy between the files and the history.
type Finding struct {
	Kind    string // proposed, unreviewed, review-by-non-reviewer, model-mismatch, unknown-trailer
	Rule    string // the RULES.md id, empty for a derived fact
	Task    string
	Gate    string
	Commit  string
	Trailer string
	Action  string // what resolves the finding
}

// Audit reports the discrepancies at ref: proposed tasks, statuses past an
// unreviewed junction, reviews by non-reviewers, model mismatches and trailers
// that name nothing.
func Audit(repo Repo, ref string) ([]Finding, error) {
	tip, err := repo.load(ref)
	if err != nil {
		return nil, err
	}
	all, err := repo.log(ref, false)
	if err != nil {
		return nil, err
	}
	trunk, err := repo.log(ref, true)
	if err != nil {
		return nil, err
	}
	var out []Finding
	out = append(out, proposed(repo, tip, ref, all, trunk)...)
	out = append(out, unreviewed(tip, all)...)
	out = append(out, trailers(repo, tip, all)...)
	return out, nil
}

// proposed finds the tasks whose deciding commit is not by an authority. Off
// the trunk every task is proposed.
func proposed(repo Repo, tip *Project, ref string, all, trunk []Commit) []Finding {
	var out []Finding
	trees := map[string]*Project{}
	for _, id := range tip.ids() {
		file := ".tableaux/tasks/" + id + ".yaml"
		if ref != tip.Trunk {
			out = append(out, Finding{Kind: "proposed", Task: id, Gate: "defined", Commit: all[0].Hash,
				Action: "Merge the branch into " + tip.Trunk + ", then an authority commits `Authorised: " + id + "`"})
			continue
		}
		for _, c := range trunk {
			if !c.touches(file) && !hasTrailer(c, "Authorised", id) {
				continue
			}
			p := trees[c.Hash]
			if p == nil {
				p, _ = repo.load(c.Hash)
				trees[c.Hash] = p
			}
			if p == nil || p.Tasks[id] == nil {
				break
			}
			auth := p.Authorities(id)
			ok := false
			for _, a := range auth {
				ok = ok || c.by(a)
			}
			if !ok {
				out = append(out, Finding{Kind: "proposed", Task: id, Gate: "defined", Commit: c.Hash,
					Action: "An authority (" + auth[0] + ") commits `Authorised: " + id + "`"})
			}
			break
		}
	}
	return out
}

func hasTrailer(c Commit, key, id string) bool {
	for _, t := range c.Trailers {
		if t.Key == key && strings.Fields(t.Value + " ")[0] == id {
			return true
		}
	}
	return false
}

// unreviewed finds statuses that pass a reviewed junction (S11) with no
// Reviewed commit by its reviewer. The defined gate stands on authorisation.
func unreviewed(tip *Project, all []Commit) []Finding {
	accepted := map[string]bool{}
	for _, c := range all {
		for _, t := range c.Trailers {
			if t.Key == "Reviewed" {
				for _, who := range []string{c.Author, c.Committer} {
					accepted[t.Value+" "+who] = true
				}
			}
		}
	}
	var out []Finding
	for _, id := range tip.ids() {
		gate, ok := tip.Status[id]
		if !ok || tip.index(gate) < 0 {
			continue
		}
		var deciding string
		for _, c := range all {
			if c.touches(".tableaux/status/"+id+".yaml") || hasTrailer(c, "Reaffirmed", id) {
				deciding = c.Hash
				break
			}
		}
		for _, g := range tip.Gates[2 : tip.index(gate)+1] {
			j := tip.Junction(id, g)
			if !j.Applies || j.Recursive || j.Reviewer == "" || accepted[id+" "+g+" "+j.Reviewer] {
				continue
			}
			out = append(out, Finding{Kind: "unreviewed", Rule: "S11", Task: id, Gate: g, Commit: deciding,
				Action: j.Reviewer + " commits `Reviewed: " + id + " " + g + "`, or the status returns to the last reviewed gate"})
		}
	}
	return out
}

// trailers reads the history oldest first for H1, H2 and H3.
func trailers(repo Repo, tip *Project, all []Commit) []Finding {
	var out []Finding
	for i := len(all) - 1; i >= 0; i-- {
		c := all[i]
		for _, t := range c.Trailers {
			f := strings.Fields(t.Value)
			switch t.Key {
			case "Authorised", "Reaffirmed":
				if len(f) != 1 || tip.Tasks[f[0]] == nil {
					out = append(out, unknownTrailer(c, t))
				}
			case "Reviewed":
				if len(f) != 2 || tip.Tasks[f[0]] == nil || !tip.Applicable(f[0], f[1]) {
					out = append(out, unknownTrailer(c, t))
				} else if fnd, bad := nonReviewer(tip, c, f[0], f[1]); bad {
					out = append(out, fnd)
				}
			}
		}
		out = append(out, modelMismatch(repo, tip, c)...)
	}
	return out
}

func unknownTrailer(c Commit, t Trailer) Finding {
	return Finding{Kind: "unknown-trailer", Rule: "H1", Commit: c.Hash, Trailer: t.Key + ": " + t.Value,
		Action: "Name a task in the project and a gate that applies to it in a new trailer; the method ignores this one"}
}

func nonReviewer(tip *Project, c Commit, id, gate string) (Finding, bool) {
	r := tip.Junction(id, gate).Reviewer
	if gate == "defined" || r == "" || c.by(r) {
		return Finding{}, false
	}
	return Finding{Kind: "review-by-non-reviewer", Rule: "H2", Task: id, Gate: gate, Commit: c.Hash,
		Action: r + " commits `Reviewed: " + id + " " + gate + "`; " + c.Author + "'s commit has no effect"}, true
}

// modelMismatch attributes a commit's work to gates: a task-file edit is work
// at defined (D8), a status change is work at the status's gate, and a trailer
// names its own gate. It compares the Model trailer with each junction's model
// by prefix.
func modelMismatch(repo Repo, tip *Project, c Commit) []Finding {
	model := ""
	for _, t := range c.Trailers {
		if t.Key == "Model" {
			model = t.Value
		}
	}
	if model == "" {
		return nil
	}
	work := map[[2]string]bool{}
	var order [][2]string
	add := func(id, gate string) {
		k := [2]string{id, gate}
		if tip.Tasks[id] != nil && tip.index(gate) > 0 && !work[k] {
			work[k] = true
			order = append(order, k)
		}
	}
	statusGate := func(id string) string {
		text, _ := repo.show(c.Hash, ".tableaux/status/"+id+".yaml")
		return scalar(text, "gate")
	}
	for _, f := range c.Files {
		if id, ok := idOf(f, ".tableaux/tasks/"); ok {
			add(id, "defined")
		}
		if id, ok := idOf(f, ".tableaux/status/"); ok {
			add(id, statusGate(id))
		}
	}
	for _, t := range c.Trailers {
		f := strings.Fields(t.Value)
		switch {
		case t.Key == "Authorised" && len(f) == 1:
			add(f[0], "defined")
		case t.Key == "Reaffirmed" && len(f) == 1:
			add(f[0], statusGate(f[0]))
		case t.Key == "Reviewed" && len(f) == 2:
			add(f[0], f[1])
		}
	}
	var out []Finding
	for _, k := range order {
		if want := tip.Junction(k[0], k[1]).Model; want != "" && !strings.HasPrefix(model, want) {
			out = append(out, Finding{Kind: "model-mismatch", Rule: "H3", Task: k[0], Gate: k[1], Commit: c.Hash,
				Action: "Redo the work under " + want + ", or state " + model + " as the junction's model"})
		}
	}
	return out
}

func idOf(path, dir string) (string, bool) {
	if strings.HasPrefix(path, dir) && strings.HasSuffix(path, ".yaml") {
		return strings.TrimSuffix(strings.TrimPrefix(path, dir), ".yaml"), true
	}
	return "", false
}
