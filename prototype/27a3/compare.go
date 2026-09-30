package main

import (
	"fmt"
	"reflect"
	"strings"
)

// compare checks every fact expected.yaml states about authorisation,
// authorities, resolved junctions and reviews against the derivation, and
// returns a report and the number of disagreements.
func compare(repo string, d *Derived, lab map[string]string, exp map[string]any) (string, int) {
	var b strings.Builder
	bad, n := 0, 0
	check := func(what string, got, want any) {
		n++
		if reflect.DeepEqual(got, want) {
			return
		}
		bad++
		fmt.Fprintf(&b, "DIFF %s: got %v want %v\n", what, got, want)
	}
	for id, tv := range asMap(exp["tasks"]) {
		te := asMap(tv)
		a := d.Auth[id]
		if ae := asMap(te["authorisation"]); ae != nil {
			check(id+" authorisation.state", a.State, str(ae["state"]))
			if ae["by"] != nil {
				check(id+" authorisation.by", a.By, str(ae["by"]))
			}
			if ae["commit"] != nil {
				check(id+" authorisation.commit", lbl(lab, a.Commit), str(ae["commit"]))
			}
		}
		if te["authorities"] != nil && a.State != "" && d.Trunk.Hash == d.View.Commit {
			check(id+" authorities", strs(a.Authorities), strs(asList(te["authorities"])))
		}
		t := d.View.Tasks[id]
		if je := asMap(te["junctions"]); je != nil && t != nil {
			for g, gv := range je {
				ge := asMap(gv)
				j := d.View.resolve(id, g)
				if g == "default" {
					check(id+" default contributor", t.Assignee, str(ge["contributor"]))
					continue
				}
				check(id+" "+g+" kind", j.Kind, mapKind(str(ge["kind"])))
				for _, f := range []string{"contributor", "model", "reviewer"} {
					if ge[f] != nil {
						check(id+" "+g+" "+f, map[string]string{"contributor": j.Contributor, "model": j.Model, "reviewer": j.Reviewer}[f], str(ge[f]))
					}
				}
				check(id+" "+g+" source", strs(j.Source), strs(oneOrList(ge["source"])))
				check(id+" "+g+" undecided", j.Undecided, str(ge["undecided"]))
			}
		}
		if ap := asList(te["applicable"]); ap != nil && t != nil {
			var got []string
			for _, g := range d.View.Gates {
				if d.View.resolve(id, g).Kind != "not_applicable" {
					got = append(got, g)
				}
			}
			check(id+" applicable", got, strs(ap))
		}
		if rv := asList(te["reviews"]); rv != nil {
			var got, want []string
			for _, r := range d.Reviews[id] {
				got = append(got, r.Gate+" "+r.Reviewer+" "+lbl(lab, r.Commit))
			}
			for _, r := range rv {
				re := asMap(r)
				want = append(want, str(re["gate"])+" "+str(re["reviewer"])+" "+str(re["commit"]))
			}
			check(id+" reviews", got, want)
		}
	}
	// The same project seen from another branch: off the trunk every task is proposed.
	for br, rv := range asMap(exp["refs"]) {
		od, err := derive(repo, br, "")
		if err != nil {
			check("refs."+br, err.Error(), "a branch")
			continue
		}
		for id, tv := range asMap(asMap(rv)["tasks"]) {
			if od.View.Tasks[id] == nil { // the branch does not hold the task, so it has no authorisation there
				fmt.Fprintf(&b, "NOTE refs.%s %s: expected.yaml states a task the branch lacks\n", br, id)
				continue
			}
			want := str(asMap(asMap(tv)["authorisation"])["state"])
			check("refs."+br+" "+id+" authorisation.state", od.Auth[id].State, want)
		}
	}
	fmt.Fprintf(&b, "%d checks, %d disagreements\n", n, bad)
	return b.String(), bad
}

func mapKind(k string) string {
	if k == "not_applicable" || k == "recursive" {
		return k
	}
	return "plain"
}

func oneOrList(v any) []any {
	if l, ok := v.([]any); ok {
		return l
	}
	if v == nil {
		return nil
	}
	return []any{v}
}

func strs[T any](in []T) []string {
	out := []string{}
	for _, v := range in {
		out = append(out, fmt.Sprint(v))
	}
	return out
}
