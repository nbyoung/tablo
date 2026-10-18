package views

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// itemsOf writes each item of a queue as its kind, its task and its gate,
// with its age where it has one and its causes where it waits.
func itemsOf(q *view.Queue) []string {
	out := []string{}
	for _, item := range q.Items {
		text := item.Kind + " " + item.Task.ID + " " + item.Gate
		if item.Age != 0 {
			text += fmt.Sprintf(" age %d", item.Age)
		}
		for _, w := range item.Causes {
			text += " " + w.Kind
			switch {
			case w.Requires != nil:
				text += ":" + w.Requires.Task.ID
			case w.Reason != "":
				text += ":" + w.Reason
			case w.By != "":
				text += ":" + w.By
			}
		}
		out = append(out, text)
	}
	return out
}

// T11: the queue's kinds and order.
func TestQueueKinds(t *testing.T) {
	f, _ := station(t, "main")
	for _, c := range []struct {
		name string
		q    Query
		want []string
	}{
		{"ada", Query{Task: "a1c0", Person: "ada@example.org"}, []string{
			"authorisation 3c5d defined", "ready 7b2e defined", "reaffirmation 9f31 function", "waiting 9f31 performance reason:blocked",
		}},
		{"dan", Query{Task: "a1c0", Person: "dan@example.org"}, []string{
			"reaffirmation 3c5d defined age 6", "waiting 3c5d mockup authorisation:ada@example.org",
		}},
		// c07d's next junction is recursive: no item for anyone but an authorisation (decision 8).
		{"ben", Query{Task: "a1c0", Person: "ben@example.org"}, []string{}},
		{"the agent", Query{Task: "a1c0", Person: "opus@example.org"}, []string{}},
		{"nobody", Query{Task: "a1c0"}, []string{}},
		{"ada under 4e2b", Query{Task: "4e2b", Person: "ada@example.org"}, []string{"reaffirmation 9f31 function", "waiting 9f31 performance reason:blocked"}},
		{"ada with no task", Query{Person: "ada@example.org"}, []string{
			"authorisation 3c5d defined", "ready 7b2e defined", "reaffirmation 9f31 function", "waiting 9f31 performance reason:blocked",
		}},
	} {
		for level := range levels {
			c.q.Level = level
			got := Queue(f, c.q)
			if !reflect.DeepEqual(itemsOf(got), c.want) {
				t.Errorf("%s at %s: the items are %q; want %q", c.name, levels[level], itemsOf(got), c.want)
			}
			if (got.QueueProvenance != nil) != (level == Provenance) || level == Provenance && got.Brief != nil {
				t.Errorf("%s at %s: the brief is %+v", c.name, levels[level], got.QueueProvenance)
			}
			for _, item := range got.Items {
				if (item.ItemDetail != nil) != (level >= Detail) || item.Causes == nil || item.Since == "" {
					t.Errorf("%s at %s: the item of %s is %+v", c.name, levels[level], item.Task.ID, item)
				}
			}
		}
	}

	// An agent's work carries the model its junction states.
	g := mem(t, map[string]string{
		"tasks/r000.yaml":  leafOf("r@x", "", 0, "{ design: { contributor: bot@x, model: m-1 } }"),
		"tasks/a000.yaml":  leafOf("a@x", "r000", 1, ""),
		"status/a000.yaml": "gate: function\nstate: nominal\n",
	})
	agent := Queue(g, Query{Task: "r000", Person: "bot@x"})
	if got := itemsOf(agent); !reflect.DeepEqual(got, []string{"ready a000 design"}) || agent.Items[0].Model != "m-1" || agent.Items[0].Since != "" {
		t.Errorf("the agent's queue is %q with %+v", got, agent.Items)
	}

	// Reaffirmations run oldest first, then in display order: ada records
	// 7b2e, which comes after 9f31, in a commit with an older author date.
	repo := clone(t, "weather-station")
	if err := os.WriteFile(filepath.Join(repo, ".tableaux", "status", "7b2e.yaml"), []byte("gate: defined\nstate: nominal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "--author=Ada <ada@example.org>", "-m", "Record the gateway")
	later := Queue(derived(t, load.Options{}, repo, "main", ""), Query{Task: "a1c0", Person: "ada@example.org"})
	want := []string{"authorisation 3c5d defined", "ready 7b2e mockup", "reaffirmation 7b2e defined", "reaffirmation 9f31 function", "waiting 9f31 performance reason:blocked"}
	if got := itemsOf(later); !reflect.DeepEqual(got, want) {
		t.Errorf("with an older status the items are %q; want %q", got, want)
	}

	// Every queue of the corpus lists its kinds in the queue's order.
	rank := map[string]int{"review": 0, "authorisation": 1, "ready": 2, "reaffirmation": 3, "waiting": 4}
	queues := 0
	for _, e := range expectations(t) {
		f := e.at(t, e.ref)
		for _, q := range queries(f, Glance)["queue"] {
			if q.Brief != nil {
				continue
			}
			queues++
			last := 0
			for _, item := range Queue(f, q).Items {
				if rank[item.Kind] < last || (item.Kind == "waiting") != (len(item.Causes) > 0) {
					t.Errorf("%s: %+v: %s of %s is out of order, or waits for nothing", e.name, q, item.Kind, item.Task.ID)
				}
				last = rank[item.Kind]
			}
		}
	}
	if queues < 40 {
		t.Errorf("the test reads %d queues", queues)
	}
}

// T12: the hand-off. A status that states the reason review gives the
// reviewer a review, holds the contributor's work, and is a cause; a
// hand-off the history implies and one the reviewer already accepts give
// none of the three.
func TestQueueHandoff(t *testing.T) {
	repo := clone(t, "handoff-inferred")
	file := filepath.Join(repo, ".tableaux", "status", "b2c9.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(data, []byte("reason: review\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	f := derived(t, load.Options{}, repo, "", "")
	if h := f.Handoff("b2c9"); h == nil || h.Kind != derive.Stated {
		t.Fatalf("the copy's hand-off is %+v", h)
	}
	olive := Queue(f, Query{Task: "e4a1", Person: "olive@example.org", Level: Detail})
	if got := itemsOf(olive); !reflect.DeepEqual(got, []string{"review b2c9 design"}) {
		t.Fatalf("the reviewer's items are %q", got)
	}
	if item := olive.Items[0]; item.Junction == nil || item.Junction.Reviewer != "olive@example.org" || !item.Junction.Acts || item.Status.Reason != "review" || !item.Status.Uncommitted {
		t.Errorf("the review item is %+v with %+v", item, item.ItemDetail)
	}
	pat := Queue(f, Query{Task: "e4a1", Person: "pat@example.org"})
	if got, want := itemsOf(pat), []string{"reaffirmation b2c9 defined", "waiting b2c9 design review:olive@example.org"}; !reflect.DeepEqual(got, want) {
		t.Errorf("the contributor's items are %q; want %q", got, want)
	}
	tree := Blockage(f, Query{Task: "e4a1", Level: Provenance})
	if got, want := causesOf(tree), []string{"review b2c9 design olive@example.org reviewer reviews holds 2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the causes are %q; want %q", got, want)
	}
	if c := tree.Causes[0]; len(c.Resolve) != 1 || c.Resolve[0].Text != "git commit --allow-empty --trailer 'Reviewed: b2c9 design'" || c.Junction == nil || c.Junction.Gate != "design" || c.Authorisation != nil || c.Linkage != nil {
		t.Errorf("the review cause resolves by %+v at %+v", c.Resolve, c.Junction)
	}

	// The stale hand-off's source is a day newer than the status: age 1.
	for name, reaffirmation := range map[string]string{"handoff-inferred": "reaffirmation b2c9 defined", "handoff-stale": "reaffirmation b2c9 defined age 1"} {
		f := entry(t, name)
		if got := itemsOf(Queue(f, Query{Task: "e4a1", Person: "olive@example.org"})); len(got) != 0 {
			t.Errorf("%s: the reviewer's items are %q", name, got)
		}
		if got, want := itemsOf(Queue(f, Query{Task: "e4a1", Person: "pat@example.org"})), []string{"ready b2c9 design", reaffirmation}; !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the contributor's items are %q; want %q", name, got, want)
		}
		if got := causesOf(Blockage(f, Query{Task: "e4a1"})); len(got) != 0 {
			t.Errorf("%s: the causes are %q", name, got)
		}
	}
}

// T13: off the trunk. Every task reads as proposed there, and the queue and
// the tree treat as proposed the tasks whose file differs from the trunk's
// tip alone (decision 9); with no history they treat none.
func TestQueueOffTrunk(t *testing.T) {
	f, _ := station(t, "sensor-board")
	if f.OnTrunk() || f.Authorisation("9f31").Authorised {
		t.Fatal("sensor-board reads as on the trunk")
	}
	ada := Queue(f, Query{Task: "a1c0", Person: "ada@example.org"})
	if got, want := itemsOf(ada), []string{"ready 9f31 defined", "ready 7b2e defined"}; !reflect.DeepEqual(got, want) {
		t.Errorf("at sensor-board ada's items are %q; want %q", got, want)
	}
	for _, c := range Blockage(f, Query{Task: "a1c0"}).Causes {
		if c.Kind == "authorisation" {
			t.Errorf("at sensor-board the tree holds the authorisation of %s", c.Task.ID)
		}
	}

	// A branch that changes a task file proposes that task.
	repo := clone(t, "weather-station")
	gitRun(t, repo, "checkout", "-q", "-b", "revision", "main")
	file := filepath.Join(repo, ".tableaux", "tasks", "7b2e.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(data), "title: Gateway", "title: Gateway, revised", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "commit", "-q", "-am", "Revise the gateway")
	branch := derived(t, load.Options{}, repo, "revision", "")
	if a := branch.Authorisation("7b2e"); branch.OnTrunk() || a.Why != derive.OffTrunk || !a.Differs || branch.Authorisation("9f31").Differs {
		t.Fatalf("on the branch 7b2e reads %+v", a)
	}
	want := []string{"authorisation 7b2e undefined", "reaffirmation 9f31 function", "waiting 9f31 performance reason:blocked", "waiting 7b2e defined authorisation:ada@example.org"}
	if got := itemsOf(Queue(branch, Query{Task: "a1c0", Person: "ada@example.org"})); !reflect.DeepEqual(got, want) {
		t.Errorf("on the branch ada's items are %q; want %q", got, want)
	}
	// 3c5d's file is the trunk's: on the branch it is no proposal.
	if got, want := itemsOf(Queue(branch, Query{Task: "a1c0", Person: "dan@example.org"})), []string{"ready 3c5d mockup", "reaffirmation 3c5d defined"}; !reflect.DeepEqual(got, want) {
		t.Errorf("on the branch dan's items are %q; want %q", got, want)
	}
	held := false
	for _, c := range Blockage(branch, Query{Task: "a1c0"}).Causes {
		if c.Kind == "authorisation" {
			held = held || c.Task.ID == "7b2e" && c.Resolver == "ada@example.org"
			if c.Task.ID != "7b2e" {
				t.Errorf("on the branch the tree holds the authorisation of %s", c.Task.ID)
			}
		}
	}
	if !held {
		t.Error("on the branch the tree holds no authorisation of 7b2e")
	}

	// Without history no task is proposed: the work is ready.
	station := builtAt(t, "weather-station")
	bare := derived(t, load.Options{}, station, gitRun(t, station, "rev-parse", "main^{tree}"), "")
	if got, want := itemsOf(Queue(bare, Query{Task: "a1c0", Person: "dan@example.org"})), []string{"ready 3c5d mockup"}; !reflect.DeepEqual(got, want) {
		t.Errorf("without history dan's items are %q; want %q", got, want)
	}
}

// T14: an item's detail.
func TestQueueItemDetail(t *testing.T) {
	requires := func(entries ...string) string {
		return "requires:\n  - " + strings.Join(entries, "\n  - ") + "\n"
	}
	f := mem(t, map[string]string{
		"tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
		"tasks/f000.yaml":  leafOf("p@x", "r000", 1, ""),
		"tasks/a000.yaml":  leafOf("a@x", "f000", 1, "{ mockup: { reviewer: r@x, references: [{ url: doc.md, text: The sketch }] } }") + "references:\n  - { url: plan.md, text: The plan }\n",
		"status/a000.yaml": "gate: defined\nstate: nominal\nnote: Under way\n",
		"tasks/b000.yaml":  leafOf("b@x", "r000", 2, "") + requires("{ id: a000, from: mockup, to: defined, text: The mockup }"),
		"tasks/c000.yaml":  leafOf("c@x", "r000", 3, "") + requires("{ id: a000, from: defined, to: defined, text: The definition }"),
		"tasks/d000.yaml":  leafOf("d@x", "r000", 4, "") + requires("{ id: f000, from: mockup, to: defined, text: The parent's mockup }"),
		"tasks/e000.yaml":  leafOf("e@x", "r000", 5, "") + requires("{ id: a000, from: design, to: defined, text: The design }", "{ id: f000, from: design, to: defined }"),
	})
	q := Queue(f, Query{Task: "r000", Person: "a@x", Level: Detail})
	if got := itemsOf(q); !reflect.DeepEqual(got, []string{"ready a000 mockup"}) {
		t.Fatalf("the items are %q", got)
	}
	item := q.Items[0]
	edges := func(list []view.Requirement) string {
		var out []string
		for _, r := range list {
			out = append(out, r.Task.ID+" "+r.From+" "+r.Condition)
		}
		return strings.Join(out, "; ")
	}
	if got := edges(item.Unblocks); got != "b000 mockup unmet" {
		t.Errorf("the item unblocks %q; want the dependent whose from the gate reaches", got)
	}
	if got := edges(item.AlsoRequiredBy); got != "c000 defined met; e000 design unmet" {
		t.Errorf("the item is also required by %q; want the met one and the one a later gate meets", got)
	}
	if len(item.ParentUnblocks) != 1 || item.ParentUnblocks[0].Parent.ID != "f000" || edges([]view.Requirement{item.ParentUnblocks[0].Requires}) != "d000 mockup unmet" {
		t.Errorf("the item unblocks for its parent %+v", item.ParentUnblocks)
	}
	if j := item.Junction; j == nil || j.Gate != "mockup" || j.Reviewer != "r@x" || !reflect.DeepEqual(j.From, []string{"a000"}) || len(item.Sources) != 2 || item.Sources[0].Field != "reviewer" || item.Sources[1].Field != "references" {
		t.Errorf("the junction is %+v from %+v", item.Junction, item.Sources)
	}
	if !reflect.DeepEqual(item.TaskReferences, []view.Reference{{Text: "The plan", URL: "plan.md"}}) || len(item.Requires) != 0 || item.Status.Note != "Under way" || item.StatusCommit != nil || item.Authorisation != nil {
		t.Errorf("the item's detail is %+v", item.ItemDetail)
	}
	// A dependent's own item lists what it requires.
	if b := Queue(f, Query{Task: "r000", Person: "b@x", Level: Detail}); len(b.Items) != 1 || b.Items[0].Kind != "waiting" || len(b.Items[0].Requires) != 1 || b.Items[0].Requires[0].Task.ID != "a000" || b.Items[0].Requires[0].Status == nil {
		t.Errorf("the dependent's queue is %+v", b.Items)
	}

	// Across projects: a requirement with its url and its commit.
	across := entry(t, "requires-across-projects")
	j := (builder{f: across}).nextJunction("b2c9")
	if j == nil {
		t.Fatal("requires-across-projects: b2c9 has no next junction")
	}
	items := Queue(across, Query{Task: "e4a1", Person: j.Contributor.V, Level: Detail}).Items
	var work *view.QueueItem
	for i := range items {
		if items[i].Task.ID == "b2c9" && items[i].Kind == "ready" {
			work = &items[i]
		}
	}
	if work == nil || len(work.Requires) != 1 {
		t.Fatalf("the queue of %s is %+v", j.Contributor.V, items)
	}
	if r := work.Requires[0]; r.Subproject != "https://example.org/umbrella.git" || len(r.Commit) != 40 || r.Task.ID != "9e10" || r.Condition != "met" {
		t.Errorf("b2c9 requires %+v", r)
	}

	// An authorisation item carries the authorisation, and no junction on a parent.
	station, hashes := station(t, "main")
	owed := Queue(station, Query{Task: "a1c0", Person: "ada@example.org", Level: Detail}).Items[0]
	if owed.Kind != "authorisation" || owed.Authorisation == nil || owed.Authorisation.Commit == nil || owed.Authorisation.Commit.Hash != hashes["W12"] || owed.Authorisation.Email != "dan@example.org" ||
		owed.StatusCommit == nil || owed.StatusCommit.Hash != hashes["W8"] {
		t.Errorf("the authorisation item is %+v with %+v", owed, owed.Authorisation)
	}
}

// T15: the brief.
func TestBrief(t *testing.T) {
	f, hashes := station(t, "main")
	brief := func(person, task, gate string, level Level) *view.Queue {
		return Queue(f, Query{Task: "a1c0", Person: person, Level: level, Brief: &BriefOf{Task: task, Gate: gate}})
	}
	ready := brief("ada@example.org", "7b2e", "defined", Provenance)
	if len(ready.Items) != 0 || ready.Items == nil || ready.Brief == nil {
		t.Fatalf("the brief of 7b2e is %+v with the items %v", ready.Brief, ready.Items)
	}
	b := ready.Brief
	if b.Kind != "ready" || len(b.Causes) != 0 || b.Task.TaskRef != (view.TaskRef{ID: "7b2e", Title: "Gateway"}) || b.Task.Description == "" ||
		!reflect.DeepEqual(b.Task.Path, []view.TaskRef{{ID: "a1c0", Title: "Weather station"}}) {
		t.Errorf("the brief is %s of %+v", b.Kind, b.Task)
	}
	if b.Gate.Key != "defined" || b.Gate.Symbol != "📝" || b.Gate.Criteria == "" || b.Gate.Follows != "plain" {
		t.Errorf("the gate is %+v", b.Gate)
	}
	if c := b.Junction.Contributor; c.Value != "ada@example.org" || c.Source != (view.BriefSource{Kind: "assignee"}) || b.Junction.Model.Value != "" || b.Junction.Model.Source.Kind != "default" ||
		b.Junction.Reviewer.Source.Kind != "default" || b.Junction.References == nil || b.Supplier != nil {
		t.Errorf("the junction is %+v with the supplier %+v", b.Junction, b.Supplier)
	}
	if s := b.Status; s.Path != ".tableaux/status/7b2e.yaml" || s.Gate != "undefined" || s.GateSymbol != "❔" || s.State != "undefined" || s.StateSymbol == "" || s.Reason != "" || s.ReasonSymbol != "" || s.Commit != hashes["W1"] {
		t.Errorf("the status is %+v", s)
	}
	if len(b.Requires) != 0 || len(b.Dependents) != 1 || b.Dependents[0].ID != "3c5d" || b.Dependents[0].From != "function" || b.Dependents[0].Condition != "pending" ||
		b.Parent == nil || b.Parent.ID != "a1c0" || b.Parent.Dependents == nil || len(b.Parent.Dependents) != 0 {
		t.Errorf("the edges are %+v, %+v and %+v", b.Requires, b.Dependents, b.Parent)
	}

	waiting := brief("ada@example.org", "9f31", "performance", Provenance).Brief
	if waiting == nil || waiting.Kind != "waiting" || len(waiting.Causes) != 1 || waiting.Causes[0].Reason != "blocked" {
		t.Fatalf("the brief of 9f31 is %+v", waiting)
	}
	if refs := waiting.Junction.References; len(refs) != 1 || refs[0].Text != "Power budget" || refs[0].Source != (view.BriefSource{Kind: "task", ID: "9f31", Title: "Sensor board"}) {
		t.Errorf("the junction's references are %+v", refs)
	}
	if s := waiting.Status; s.State != "stalled" || s.StateSymbol == "" || s.ReasonSymbol == "" || s.Recorder != "ada@example.org" || s.Date != "2026-09-28" || s.Commit != hashes["W13"] || s.Note == "" {
		t.Errorf("the status of 9f31 is %+v", s)
	}
	if d := waiting.Dependents; len(d) != 1 || d[0].TaskRef != (view.TaskRef{ID: "c07d", Title: "Node firmware"}) || d[0].To != "implementation" || d[0].Condition != "unmet" || d[0].Text != "Pin map and sensor bus" ||
		len(waiting.Task.Path) != 2 || waiting.Task.Path[0].ID != "a1c0" || waiting.Parent.ID != "4e2b" {
		t.Errorf("9f31 has the dependents %+v under %+v", d, waiting.Task.Path)
	}

	// No such work in the person's queue: null. Below provenance: no brief.
	for name, q := range map[string]*view.Queue{
		"a recursive junction":    brief("ben@example.org", "c07d", "implementation", Provenance),
		"another gate":            brief("ada@example.org", "7b2e", "mockup", Provenance),
		"another person":          brief("dan@example.org", "7b2e", "defined", Provenance),
		"an item of another kind": brief("ada@example.org", "3c5d", "defined", Provenance),
	} {
		if q.QueueProvenance == nil || q.Brief != nil || q.Items == nil || len(q.Items) != 0 {
			t.Errorf("%s: the queue is %+v with %+v", name, q.Items, q.QueueProvenance)
		}
	}
	if q := brief("ada@example.org", "7b2e", "defined", Detail); q.QueueProvenance != nil || len(q.Items) != 0 {
		t.Errorf("at detail the brief's queue is %+v", q)
	}
	// Without the parameter, provenance is detail with no brief.
	if q := Queue(f, Query{Task: "a1c0", Person: "ada@example.org", Level: Provenance}); q.QueueProvenance == nil || q.Brief != nil || len(q.Items) != 4 || q.Items[0].ItemDetail == nil {
		t.Errorf("the queue at provenance is %+v", q)
	}

	// The supplier: the task whose entry sends the work to the contributor.
	g := mem(t, map[string]string{
		"tasks/r000.yaml":  leafOf("r@x", "", 0, "{ design: { contributor: bot@x, model: m-1 } }") + "description: Sends the design to the agent\n",
		"tasks/a000.yaml":  leafOf("a@x", "r000", 1, "") + "requires:\n  - { id: b000, from: defined, to: release, text: The other }\n",
		"status/a000.yaml": "gate: function\nstate: nominal\n",
		"tasks/b000.yaml":  leafOf("b@x", "r000", 2, ""),
		"status/b000.yaml": "gate: design\nstate: nominal\n",
	})
	agent := Queue(g, Query{Task: "r000", Person: "bot@x", Level: Provenance, Brief: &BriefOf{Task: "a000", Gate: "design"}}).Brief
	if agent == nil || agent.Supplier == nil || agent.Supplier.ID != "r000" || agent.Supplier.Description != "Sends the design to the agent" {
		t.Fatalf("the agent's brief is %+v", agent)
	}
	source := view.BriefSource{Kind: "task", ID: "r000", Title: "A task"}
	if j := agent.Junction; j.Contributor != (view.BriefField{Value: "bot@x", Source: source}) || j.Model != (view.BriefField{Value: "m-1", Source: source}) ||
		j.Reviewer != (view.BriefField{Value: "a@x", Source: view.BriefSource{Kind: "assignee"}}) {
		t.Errorf("the agent's junction is %+v", j)
	}
	if agent.Gate.Follows != "plain" || agent.Status.Commit != "" || agent.Status.Path != ".tableaux/status/a000.yaml" ||
		len(agent.Requires) != 1 || agent.Requires[0].ID != "b000" || agent.Requires[0].Condition != "met" || agent.Requires[0].URL != "" {
		t.Errorf("the agent's brief follows %q with %+v and %+v", agent.Gate.Follows, agent.Status, agent.Requires)
	}
	// At the last gate nothing follows.
	last := mem(t, map[string]string{
		"tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
		"tasks/a000.yaml":  leafOf("a@x", "r000", 1, ""),
		"status/a000.yaml": "gate: design\nstate: nominal\n",
	})
	if b := Queue(last, Query{Task: "r000", Person: "a@x", Level: Provenance, Brief: &BriefOf{Task: "a000", Gate: "release"}}).Brief; b == nil || b.Gate.Follows != "" || b.Gate.Key != "release" {
		t.Errorf("the brief at the last gate is %+v", b)
	}
}

// The queue's causes at glance (decision 15): a reason carries the status's
// note, and an authorisation item the task's Authorisation.Why, "" where a
// commit decides but not by an authority; every other wait and item carries
// "".
func TestQueueCausesAtGlance(t *testing.T) {
	f, _ := station(t, "main")
	ada := Queue(f, Query{Task: "a1c0", Person: "ada@example.org"})
	reasons := 0
	for _, item := range ada.Items {
		for _, w := range item.Causes {
			if w.Kind == "reason" {
				reasons++
				if w.Note != "Barometer ICs on 14-week backorder" || w.Note != f.Status(item.Task.ID).Note {
					t.Errorf("the reason of %s carries the note %q", item.Task.ID, w.Note)
				}
			} else if w.Note != "" {
				t.Errorf("the %s wait of %s carries the note %q", w.Kind, item.Task.ID, w.Note)
			}
		}
	}
	if reasons != 1 {
		t.Errorf("ada's queue holds %d reasons; want 1", reasons)
	}
	brief := Queue(f, Query{Task: "a1c0", Person: "ada@example.org", Level: Provenance, Brief: &BriefOf{Task: "9f31", Gate: "performance"}}).Brief
	if brief == nil || len(brief.Causes) != 1 || brief.Causes[0].Note != "Barometer ICs on 14-week backorder" {
		t.Errorf("the brief of 9f31 waits for %+v", brief)
	}
	for _, w := range Queue(f, Query{Task: "a1c0", Person: "dan@example.org"}).Items[1].Causes {
		if w.Kind != "authorisation" || w.Note != "" {
			t.Errorf("dan's work waits for %+v", w)
		}
	}

	// whys returns each item of a queue as its kind, its task and its why.
	whys := func(q *view.Queue) []string {
		out := []string{}
		for _, item := range q.Items {
			out = append(out, item.Kind+" "+item.Task.ID+" "+item.Why)
		}
		return out
	}
	if a := f.Authorisation("3c5d"); a.Authorised || a.Commit == nil || a.Why != derive.Determined {
		t.Fatalf("on main 3c5d reads %+v", a)
	}
	if got, want := whys(ada), []string{"authorisation 3c5d ", "ready 7b2e ", "reaffirmation 9f31 ", "waiting 9f31 "}; !reflect.DeepEqual(got, want) {
		t.Errorf("on main ada's items are %q; want %q", got, want)
	}
	repo := clone(t, "weather-station")
	gitRun(t, repo, "checkout", "-q", "-b", "revision", "main")
	file := filepath.Join(repo, ".tableaux", "tasks", "7b2e.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(string(data), "title: Gateway", "title: Gateway, revised", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "commit", "-q", "-am", "Revise the gateway")
	branch := derived(t, load.Options{}, repo, "revision", "")
	for _, level := range []Level{Glance, Detail} {
		q := Queue(branch, Query{Task: "a1c0", Person: "ada@example.org", Level: level})
		if got, want := whys(q), []string{"authorisation 7b2e off_trunk", "reaffirmation 9f31 ", "waiting 9f31 ", "waiting 7b2e "}; !reflect.DeepEqual(got, want) {
			t.Errorf("on the branch at %s ada's items are %q; want %q", levels[level], got, want)
		}
		if level == Detail && (q.Items[0].Authorisation == nil || q.Items[0].Authorisation.Why != q.Items[0].Why) {
			t.Errorf("the detail's authorisation %+v disagrees with the glance's why %q", q.Items[0].Authorisation, q.Items[0].Why)
		}
	}
}

// Finished work has nothing to reaffirm (the owner's ruling of 2026-10-18):
// a leaf whose status is complete gives its recorder no reaffirmation, and a
// status in any other state still gives one. On the tooling plan at
// 50a373b, the tag corpus/tooling, the agents' twenty mockup leaves stand
// complete, and the agents' queue keeps eight reaffirmations of 28.
func TestQueueLeavesFinishedWork(t *testing.T) {
	repo := clone(t, "weather-station")
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repo, ".tableaux", "status", name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("7b2e.yaml", "gate: defined\nstate: nominal\n")
	gitRun(t, repo, "add", ".tableaux/status/7b2e.yaml")
	gitRun(t, repo, "commit", "-q", "-m", "Define the gateway")
	write("9f31.yaml", "gate: release\nstate: complete\n")
	gitRun(t, repo, "commit", "-q", "-am", "Release the sensor board")
	f := derived(t, load.Options{}, repo, "main", "")
	if s := f.Status("9f31"); s.Kind != derive.Recorded || s.Recorder != "olive@example.org" || s.State != "complete" || !f.Leaf("9f31") {
		t.Fatalf("9f31 reads %+v", s)
	}
	for _, level := range []Level{Glance, Detail, Provenance} {
		if got, want := itemsOf(Queue(f, Query{Task: "a1c0", Person: "olive@example.org", Level: level})), []string{"reaffirmation 7b2e defined"}; !reflect.DeepEqual(got, want) {
			t.Errorf("at %s olive's items are %q; want %q", levels[level], got, want)
		}
	}

	plan := tooling(t, "50a373b")
	kinds := map[string]int{}
	for _, item := range Queue(plan, Query{Person: "noreply@anthropic.com"}).Items {
		kinds[item.Kind]++
		if s := plan.Status(item.Task.ID); item.Kind == "reaffirmation" && s.State == "complete" {
			t.Errorf("the tooling plan gives a reaffirmation of %s, complete at %s", item.Task.ID, s.Gate)
		}
	}
	if want := map[string]int{"reaffirmation": 8, "ready": 8}; !reflect.DeepEqual(kinds, want) {
		t.Errorf("on the tooling plan the agents' queue holds %v; want %v", kinds, want)
	}
}
