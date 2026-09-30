package main

import (
	"fmt"
	"strings"
)

// Derived is everything derived for one view.
type Derived struct {
	View    *Repo
	Trunk   Trunk
	Auth    map[string]Auth
	Reviews map[string][]Review
	Status  map[string]string // leaf id -> gate of its status file
}

func derive(repo, ref, caller string) (*Derived, error) {
	hash, err := git(repo, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return nil, err
	}
	v := loadRepo(repo, hash)
	d := &Derived{View: v, Trunk: findTrunk(repo, hash, caller), Reviews: map[string][]Review{}, Status: map[string]string{}}
	d.Auth = authorise(v, d.Trunk)
	for _, id := range sortedIDs(v.Tasks) {
		if src, ok := show(repo, hash, ".tableaux/status/"+id+".yaml"); ok {
			d.Status[id] = str(asMap(parseYAML(src))["gate"])
			d.Reviews[id] = reviews(v, id, d.Status[id], d.Auth[id])
		}
	}
	return d, nil
}

func lbl(lab map[string]string, h string) string {
	if l, ok := lab[h]; ok {
		return l
	}
	if len(h) > 7 {
		return h[:7]
	}
	return h
}

func (j Junction) String() string {
	switch j.Kind {
	case "recursive":
		return fmt.Sprintf("recursive %v", j.Subproject)
	case "not_applicable":
		return "not_applicable from " + strings.Join(j.Source, ",")
	}
	s := "plain contributor=" + j.Contributor + "<" + j.Prov["contributor"] + ">"
	if j.Model != "" {
		s += " model=" + j.Model + "<" + j.Prov["model"] + ">"
	}
	if j.Reviewer != "" {
		s += " reviewer=" + j.Reviewer + "<" + j.Prov["reviewer"] + ">"
	}
	if j.Refs > 0 {
		s += fmt.Sprintf(" references=%d<%s>", j.Refs, j.Prov["references"])
	}
	if j.Undecided != "" {
		s += " undecided=" + j.Undecided
	}
	return s
}

func (d *Derived) print(w *strings.Builder, lab map[string]string) {
	fmt.Fprintf(w, "view %s  trunk %s (%s) %s\n", lbl(lab, d.View.Commit), d.Trunk.Ref, d.Trunk.How, lbl(lab, d.Trunk.Hash))
	if d.Trunk.How == "undetermined" {
		fmt.Fprintln(w, "warning P5: no trunk; every task reads as proposed")
	}
	for _, id := range sortedIDs(d.View.Tasks) {
		t := d.View.Tasks[id]
		a := d.Auth[id]
		fmt.Fprintf(w, "\ntask %s parent=%q assignee=%s\n  %s", id, t.Parent, t.Assignee, a.State)
		if a.Commit != "" {
			fmt.Fprintf(w, " by deciding commit %s (%s); authorities %v", lbl(lab, a.Commit), a.By, a.Authorities)
		}
		fmt.Fprintln(w)
		for _, g := range d.View.Gates {
			if j := d.View.resolve(id, g); j.departs(t) {
				fmt.Fprintf(w, "  %-14s %s  source=%v\n", g, j, j.Source)
			}
		}
		if g, ok := d.Status[id]; ok {
			fmt.Fprintf(w, "  status gate %s\n", g)
			for _, r := range d.Reviews[id] {
				fmt.Fprintf(w, "  review %s by %s: %s", r.Gate, r.Reviewer, orNone(lbl(lab, r.Commit), r.Commit))
				for _, ig := range r.Ignored {
					fmt.Fprintf(w, " (ignored %s)", lbl(lab, ig))
				}
				fmt.Fprintln(w)
			}
		}
	}
}

func orNone(l, h string) string {
	if h == "" {
		return "UNREVIEWED"
	}
	return l
}
