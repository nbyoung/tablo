package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const corpus = "/home/nbyoung/Projects/Tableaux/tableaux/corpus/build/weather-station"

func TestParseYAML(t *testing.T) {
	m, err := parseYAML("# c\ntitle: A, b # note\ndescription: >\n  one\n  two\nparent: { id: \"x\", order: 2 }\nrequires:\n  - { id: \"y\", text: \"p, q\" }\njunctions:\n  m: { applies: false }\n")
	if err != nil {
		t.Fatal(err)
	}
	if str(m, "title") != "A, b" || str(m, "description") != "one two" || mp(m["parent"])["order"] != 2 ||
		str(mp(lst(m["requires"])[0]), "text") != "p, q" || mp(mp(m["junctions"])["m"])["applies"] != false {
		t.Fatalf("parsed %v", m)
	}
}

func repoAt(t *testing.T, ref string) *Repo {
	t.Helper()
	if _, err := os.Stat(corpus); err != nil {
		t.Skip("the corpus is absent")
	}
	labels, _ := os.ReadFile(filepath.Join(filepath.Dir(corpus), "weather-station.labels.txt"))
	for _, l := range strings.Split(string(labels), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[0] == ref {
			ref = f[1]
		}
	}
	r, err := newRepo(corpus, ref)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func view(t *testing.T, r *Repo, name string, pr Params) M {
	t.Helper()
	if pr.Level == "" {
		pr.Level = "provenance"
	}
	if pr.Window == 0 {
		pr.Window = -1
	}
	out, err := render(r, name, pr)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(out) // the view is JSON data
	if err != nil {
		t.Fatal(err)
	}
	var round M
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	return round
}

func TestAuthorisationAgainstExpected(t *testing.T) {
	r := repoAt(t, "main")
	want := map[string]struct{ state, commit, by string }{
		"a1c0": {"authorised", "W1", "ada@example.org"}, "4e2b": {"authorised", "W1", "ada@example.org"},
		"9f31": {"authorised", "W4", "ben@example.org"}, "c07d": {"authorised", "W2", "ben@example.org"},
		"7b2e": {"authorised", "W1", "ada@example.org"}, "3c5d": {"proposed", "W12", "dan@example.org"},
	}
	for id, w := range want {
		a := r.authorisation(id)
		if a.State != w.state || a.Commit.Hash != labelHash(t, w.commit) || (a.By == "author" && a.Commit.Author != w.by) {
			t.Errorf("%s: got %s %s %s, want %v", id, a.State, a.Commit.Hash[:7], a.By, w)
		}
	}
	if a := r.authorisation("9f31"); a.Way != "merge" {
		t.Errorf("9f31 way %q", a.Way)
	}
	if a := r.authorisation("3c5d"); a.Way != "commit" {
		t.Errorf("3c5d way %q", a.Way)
	}
}

func labelHash(t *testing.T, label string) string {
	t.Helper()
	labels, _ := os.ReadFile(filepath.Join(filepath.Dir(corpus), "weather-station.labels.txt"))
	for _, l := range strings.Split(string(labels), "\n") {
		if f := strings.Fields(l); len(f) == 2 && f[0] == label {
			return f[1]
		}
	}
	t.Fatalf("no label %s", label)
	return ""
}

func TestTaskViewAgainstExpected(t *testing.T) {
	r := repoAt(t, "main")
	v := view(t, r, "task", Params{Task: "9f31"})
	st := mp(mp(v["glance"])["status"])
	if st["gate"] != "function" || st["state"] != "stalled" || st["reason"] != "blocked" ||
		st["date"] != "2026-09-28" || st["recorder"] != "ada@example.org" || st["commit"] != labelHash(t, "W13") {
		t.Errorf("9f31 status %v", st)
	}
	src := map[string]M{}
	for _, j := range lst(v["detail"].(M)["junctions"]) {
		src[str(mp(j), "gate")] = mp(j)
	}
	if src["mockup"]["reviewer"] != "ben@example.org" || src["mockup"]["contributor"] != "ada@example.org" {
		t.Errorf("mockup %v", src["mockup"])
	}
	// events match expected.yaml: task, authorised, status, reviewed, status, reaffirmed
	var got []string
	for _, e := range r.events("9f31") {
		got = append(got, e.Event+" "+e.Commit)
	}
	var want []string
	for _, e := range []struct{ ev, l string }{{"task", "W3"}, {"authorised", "W4"}, {"status", "W7"}, {"reviewed", "W9"}, {"status", "W10"}, {"reaffirmed", "W13"}} {
		want = append(want, e.ev+" "+labelHash(t, e.l))
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("events\n got %v\nwant %v", got, want)
	}
	if rv := r.reviews("9f31"); len(rv) != 1 || rv[0].Gate != "mockup" || rv[0].Commit != labelHash(t, "W9") {
		t.Errorf("reviews %v", rv)
	}
}

func TestRequirementsAndRecursiveSnapshot(t *testing.T) {
	r := repoAt(t, "main")
	q := r.requirements("c07d")
	if len(q) != 1 || q[0].Met || !q[0].Due || q[0].Condition != "unmet" {
		t.Errorf("c07d requirement %+v", q)
	}
	if q := r.requirements("3c5d"); q[0].Met || q[0].Due || q[0].Condition != "pending" {
		t.Errorf("3c5d requirement %+v", q)
	}
	if d := r.dependents("9f31"); len(d) != 1 || d[0].ID != "c07d" {
		t.Errorf("9f31 dependents %+v", d)
	}
	s := r.status("c07d")
	if s.Gate != "design" || s.State != "nominal" || s.Note != "Sleep scheduler in progress" ||
		s.Date != "2026-09-17" || s.Recorder != "ben@example.org" || s.Commit != "3dc465e9ecb1a462cebe6fa8cd5384742792d388" ||
		s.Snapshot["task"] != "f1a0" {
		t.Errorf("c07d snapshot %+v", s)
	}
	if u := r.status("7b2e"); u.Gate != "undefined" || u.Date != "2026-09-15" {
		t.Errorf("7b2e %+v", u)
	}
	if a := r.status("a1c0"); a.Gate != "defined" || a.State != "nominal" || a.From != "3c5d" || !a.Derived {
		t.Errorf("a1c0 roll-up %+v", a)
	}
	// README.md's rule takes the earliest gate, function (9f31), where expected.yaml
	// says design (c07d); the prototype follows the rule and README.md records the conflict.
	if a := r.status("4e2b"); a.Gate != "function" || a.State != "stalled" || a.From != "9f31" {
		t.Errorf("4e2b roll-up %+v", a)
	}
}

func TestJunctionResolution(t *testing.T) {
	r := repoAt(t, "main")
	p := r.proj
	if j := p.junction("c07d", "unit"); j.Model != "claude-opus-5-5" || j.Reviewer != "ben@example.org" || j.Sources["reviewer"] != "assignee" {
		t.Errorf("unit %+v", j)
	}
	if j := p.junction("9f31", "mockup"); j.Source != "4e2b" || j.Sources["reviewer"] != "4e2b" || j.Sources["contributor"] != "default" {
		t.Errorf("mockup %+v", j)
	}
	if j := p.junction("c07d", "reliability"); j.Kind != "not_applicable" {
		t.Errorf("reliability %+v", j)
	}
	if j := p.junction("c07d", "implementation"); j.Kind != "recursive" {
		t.Errorf("implementation %+v", j)
	}
	want := "undefined,defined,mockup,function,performance,design,implementation,unit,integrate,validate,release"
	if got := strings.Join(p.applicable("c07d"), ","); got != want {
		t.Errorf("applicable %s", got)
	}
}

func TestGateDefinitionView(t *testing.T) {
	r := repoAt(t, "main")
	v := view(t, r, "gate", Params{})
	if n := len(lst(mp(v["glance"])["gates"])); n != 12 {
		t.Errorf("%d gates", n)
	}
	if n := len(lst(mp(v["glance"])["marks"])); n != 5 {
		t.Errorf("%d marks", n)
	}
	v = view(t, r, "gate", Params{Task: "9f31", Window: 1})
	g := lst(mp(v["glance"])["gates"])
	if len(g) != 3 || str(mp(g[0]), "key") != "function" || mp(v["glance"])["folded"] != float64(9) {
		t.Errorf("window %v", g)
	}
	v = view(t, r, "gate", Params{Task: "9f31", Columns: []string{"performance"}})
	d := lst(mp(v["detail"])["gates"])
	if len(d) != 1 || len(lst(mp(d[0])["references"])) != 1 || mp(d[0])["references_from"] != "9f31" {
		t.Errorf("references %v", d)
	}
}

func TestAuthorityView(t *testing.T) {
	r := repoAt(t, "main")
	rows := lst(mp(view(t, r, "authority", Params{}))["glance"].(M)["rows"])
	var got []string
	for _, x := range rows {
		x := mp(x)
		got = append(got, str(x, "id")+"/"+str(x, "authorisation")+"/"+map[bool]string{true: "delegated", false: "-"}[x["delegated"] == true])
	}
	want := "a1c0/authorised/-,4e2b/authorised/delegated,9f31/authorised/delegated,c07d/authorised/-,7b2e/authorised/-,3c5d/proposed/delegated"
	if strings.Join(got, ",") != want {
		t.Errorf("rows\n got %v\nwant %s", got, want)
	}
	pv := lst(mp(view(t, r, "authority", Params{Proposed: true}))["glance"].(M)["rows"])
	if len(pv) != 1 || mp(pv[0])["id"] != "3c5d" {
		t.Errorf("proposed %v", pv)
	}
	pv = lst(mp(view(t, r, "authority", Params{Person: "ben@example.org"}))["glance"].(M)["rows"])
	if len(pv) != 4 { // the spine a1c0 and 4e2b, and the subtree of 4e2b
		t.Errorf("ben rows %d", len(pv))
	}
	// off the trunk, every task reads as proposed
	b := repoAt(t, "W3")
	if b.onTrunk {
		t.Fatal("W3 is on the trunk's first-parent line")
	}
	for _, x := range lst(mp(view(t, b, "authority", Params{}))["glance"].(M)["rows"]) {
		if mp(x)["authorisation"] != "proposed" {
			t.Errorf("off trunk: %v", x)
		}
	}
	if n := len(lst(mp(view(t, b, "authority", Params{Level: "glance"}))["glance"].(M)["rows"])); n != 5 {
		t.Errorf("W3 has %d tasks", n)
	}
}

func TestAssignmentView(t *testing.T) {
	r := repoAt(t, "main")
	v := view(t, r, "assignment", Params{Window: -1, Columns: split("")})
	rows := map[string]M{}
	for _, x := range lst(mp(v["glance"])["people"]) {
		rows[str(mp(x), "email")] = mp(x)
	}
	if rows["ada@example.org"]["assigned"] != float64(3) || rows["ben@example.org"]["assigned"] != float64(2) ||
		rows["dan@example.org"]["assigned"] != float64(1) {
		t.Errorf("assigned %v", rows)
	}
	if m := lst(rows["opus@example.org"]["models"]); len(m) != 1 || m[0] != "claude-opus-5-5" {
		t.Errorf("opus %v", rows["opus@example.org"])
	}
	// c07d's recursive implementation and exempt reliability are no one's junction
	all := view(t, r, "assignment", Params{Person: "ben@example.org", Columns: []string{"implementation", "reliability", "mockup"}})
	det := mp(lst(mp(all["detail"])["people"])[0])
	for _, c := range lst(det["contributes"]) {
		if g := mp(c)["gate"]; g == "implementation" || g == "reliability" {
			t.Errorf("junction %v", c)
		}
	}
	if a := lst(det["authority_over"]); len(a) != 1 || mp(a[0])["id"] != "4e2b" || mp(a[0])["descendants"] != float64(2) {
		t.Errorf("authority_over %v", det["authority_over"])
	}
}

func TestLevelsNest(t *testing.T) {
	r := repoAt(t, "main")
	if v := view(t, r, "task", Params{Task: "9f31", Level: "glance"}); v["detail"] != nil || v["provenance"] != nil {
		t.Error("glance carries a deeper level")
	}
	if v := view(t, r, "task", Params{Task: "9f31", Level: "detail"}); v["detail"] == nil || v["provenance"] != nil {
		t.Error("detail level")
	}
}
