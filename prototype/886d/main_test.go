package main

import (
	"encoding/json"
	"os"
	"testing"
)

const ada, ben, dan = "ada@example.org", "ben@example.org", "dan@example.org"

func load(t *testing.T) *Project {
	t.Helper()
	if _, err := os.Stat(corpus); err != nil {
		t.Skip("corpus not built:", err)
	}
	p, err := Load(corpus, "main")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestYAMLSubset(t *testing.T) {
	v, err := parseYAML("a: 1 # c\nb:\n  - { k: \"x, y\", n: [1, two] }\nd: >\n  folded\n  text\nm:\n  g: { applies: false }\n")
	if err != nil {
		t.Fatal(err)
	}
	if num(v, "a") != 1 || str(list(v, "b")[0], "k") != "x, y" || str(v, "d") != "folded text" ||
		amap(amap(v, "m"), "g")["applies"] != false {
		t.Fatalf("unexpected parse: %v", v)
	}
}

func TestRollUp(t *testing.T) {
	p := load(t)
	// README.md#status: the earliest gate wins, so 4e2b takes 9f31 at function
	// and not c07d at design. The corpus's expected.yaml says c07d; see the README.
	n := p.Status("4e2b")
	if n.Gate != "function" || n.State != "stalled" || n.Reason != "blocked" || n.From != "9f31" || n.Date != "2026-09-17" {
		t.Errorf("4e2b: %+v", n)
	}
	r := p.Status("a1c0")
	if r.Gate != "defined" || r.State != "nominal" || r.From != "3c5d" || r.Date != "2026-09-17" {
		t.Errorf("root: %+v", r)
	}
	c := p.Status("c07d")
	if c.Gate != "design" || c.State != "nominal" || c.Note != "Sleep scheduler in progress" ||
		c.Date != "2026-09-17" || c.Recorder != ben || c.Commit != "F2" || !c.Snapshot {
		t.Errorf("c07d snapshot: %+v", c)
	}
	f := p.Status("9f31")
	if f.Date != "2026-09-28" || f.Commit != "W13" || f.Recorder != ada {
		t.Errorf("9f31 reaffirmed: %+v", f)
	}
	if u := p.Status("7b2e"); u.Gate != "undefined" || u.Date != "2026-09-15" || u.Commit != "W1" {
		t.Errorf("7b2e: %+v", u)
	}
}

func TestConditionsAndAuthorisation(t *testing.T) {
	p := load(t)
	if c := p.Reqs("c07d")[0]; c.Condition != "unmet" || !c.Due || c.Met {
		t.Errorf("c07d: %+v", c)
	}
	if c := p.Reqs("3c5d")[0]; c.Condition != "pending" || c.Due {
		t.Errorf("3c5d: %+v", c)
	}
	for id, want := range map[string]bool{"a1c0": true, "4e2b": true, "9f31": true, "c07d": true, "7b2e": true, "3c5d": false} {
		if got, _ := p.Authorised(id); got != want {
			t.Errorf("authorised %s = %v", id, got)
		}
	}
}

func TestWindowAndFold(t *testing.T) {
	p := load(t)
	g := p.Global(Options{Window: 1, Cell: "state-at-gate", Level: "detail"})
	if len(g.Columns) != 9 || g.Columns[8].Gate != "unit" || len(g.Folded) != 1 || g.Folded[0].Side != "after" || g.Folded[0].Count != 0 {
		t.Errorf("window 1: %+v %+v", g.Columns, g.Folded)
	}
	g = p.Global(Options{Window: 0, Cell: "state-at-gate", Level: "detail"})
	if g.Columns[0].Gate != "defined" || len(g.Folded) != 2 || g.Folded[0].Count != 1 || g.Folded[0].Gates["undefined"] != 1 {
		t.Errorf("window 0: %+v %+v", g.Columns, g.Folded)
	}
	// The state symbol sits in the column of the gate the status names.
	for _, r := range g.Rows {
		if r.ID == "4e2b" {
			for _, c := range r.Cells {
				if (c.Gate == "function") != (c.Kind == "status") {
					t.Errorf("4e2b cell %+v", c)
				}
			}
		}
	}
	glance := p.Global(Options{Window: 1, Level: "glance"})
	if len(glance.Rows) != 4 || glance.Rows[1].Hidden != 2 {
		t.Errorf("glance rows: %d", len(glance.Rows))
	}
}

func TestContextualPerson(t *testing.T) {
	p := load(t)
	tb := p.Contextual(Options{Person: ben, Window: 1})
	got := map[string]string{}
	for _, r := range tb.Rows {
		got[r.ID] = r.Role
	}
	want := map[string]string{"a1c0": "spine", "4e2b": "spine", "c07d": "corner", "9f31": "sibling"}
	if len(got) != len(want) {
		t.Fatalf("rows %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q", k, got[k], v)
		}
	}
}

func TestBlockage(t *testing.T) {
	p := load(t)
	b := p.Blockage(Options{})
	if len(b.Causes) != 3 || b.Causes[0].Kind != "status" || b.Causes[0].Count != 2 {
		t.Fatalf("causes: %+v", b.Causes)
	}
	kinds := map[string]int{}
	for _, c := range b.Causes {
		kinds[c.Kind] = c.Count
	}
	if kinds["unmet-requirement"] != 1 || kinds["authorisation"] != 1 || len(b.NotDue) != 1 {
		t.Errorf("kinds: %v", kinds)
	}
	if _, err := json.Marshal(b); err != nil {
		t.Fatal(err)
	}
}

func TestQueue(t *testing.T) {
	p := load(t)
	var kinds []string
	for _, i := range p.Queue(Options{Person: ada, Order: "kind"}).Items {
		kinds = append(kinds, i.Kind+" "+i.Task)
	}
	want := []string{"authorisation owed 3c5d", "work ready 7b2e", "reaffirmation 9f31", "work waiting 9f31"}
	if len(kinds) != len(want) {
		t.Fatalf("ada: %v", kinds)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("ada[%d] = %q, want %q", i, kinds[i], want[i])
		}
	}
	if n := len(p.Queue(Options{Person: ben}).Items); n != 0 {
		t.Errorf("ben: %d items", n)
	}
}

func TestOffTrunkAllProposed(t *testing.T) {
	load(t)
	b, err := Load(corpus, "sensor-board")
	if err != nil {
		t.Skip("branch absent:", err)
	}
	for id := range b.Tasks {
		if ok, _ := b.Authorised(id); ok {
			t.Errorf("%s authorised off the trunk", id)
		}
	}
}
