package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/internal/validate"
)

// T4: staleness (decision 5). A status is stale when a commit records it, it
// is not complete and its date is more than the age before Now. A snapshot
// and a leaf with no file never are.
func TestStaleness(t *testing.T) {
	f := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	cases := []struct {
		now   string
		stale int
		want  string
	}{
		{"2026-09-28", 5, "[3c5d]"}, // 3c5d dates from 2026-09-22: six days
		{"2026-09-28", 6, "[]"},
		{"2026-09-28", 0, "[]"},          // seven days
		{"2026-10-06", 0, "[3c5d 9f31]"}, // by gate: 3c5d at defined, and 9f31, which its reaffirmation dates 2026-09-28
		{"2026-10-05", 0, "[3c5d]"},      // seven days is not more than seven
		{"2026-09-26", 3, "[3c5d]"},      // the design's example: stale at three days on the 26th
		{"2026-09-25", 3, "[]"},          // and not on the 25th
		{"2027-09-28", 0, "[3c5d 9f31]"}, // c07d, a snapshot, and 7b2e, with no file, never are
		{"2026-09-01", 1, "[]"},          // a date before the status's
	}
	for _, c := range cases {
		rep := audited(t, Input{Facts: f, Now: c.now, Stale: c.stale})
		if got := fmt.Sprint(tasksOf(rep, Stale)); got != c.want {
			t.Errorf("at %s with the age %d the stale statuses are %s, want %s", c.now, c.stale, got, c.want)
		}
	}
	rep := audited(t, Input{Facts: f, Now: "2026-09-28", Stale: 3})
	found := of(rep, Stale)
	if len(found) != 1 {
		t.Fatalf("%d stale statuses at three days", len(found))
	}
	x := found[0]
	if x.Task != "3c5d" || x.Gate != "defined" || x.File != ".tableaux/status/3c5d.yaml" || x.Act != Reaffirm || x.Resolver != "dan@example.org" ||
		x.Severity != model.Warning || x.Kind != "A stale status" || x.Rule != "" || x.Diagnostic != nil ||
		x.Message != "The status dates from 2026-09-22, more than 3 days before 2026-09-28" {
		t.Errorf("the stale status reads %+v", x)
	}
	if s := f.Status("c07d"); s.Kind != derive.Snapshotted {
		t.Errorf("c07d is no snapshot: %+v", s)
	}
	if s := f.Status("7b2e"); s.Kind != derive.Absent {
		t.Errorf("7b2e has a status file: %+v", s)
	}
}

// T8: the working tree. An untracked task file is proposed with no commit;
// an edited status is no commit yet and is not stale; an edited authorised
// task gives no finding.
func TestWorkingTree(t *testing.T) {
	repo := clone(t, "weather-station")
	clean := derived(t, load.Options{}, repo, "", "")
	rep := audited(t, Input{Facts: clean, Now: "2026-10-06"})
	if got := fmt.Sprint(tasksOf(rep, Proposed), tasksOf(rep, Stale)); got != "[3c5d] [3c5d 9f31]" {
		t.Fatalf("the clean working tree gives %s", got)
	}
	write(t, repo, map[string]string{
		".tableaux/tasks/d200.yaml":  "title: New\ndescription: An untracked task.\nassignee: dan@example.org\nparent: { id: \"4e2b\", order: 3 }\n",
		".tableaux/status/9f31.yaml": "gate: function\nstate: nominal\n",
		".tableaux/tasks/7b2e.yaml":  "title: Gateway, edited\ndescription: An edit of an authorised task.\nassignee: ada@example.org\nparent: { id: \"a1c0\", order: 2 }\n",
	})
	f := derived(t, load.Options{}, repo, "", "")
	rep = audited(t, Input{Facts: f, Now: "2026-10-06"})
	if got := fmt.Sprint(tasksOf(rep, Proposed)); got != "[d200 3c5d]" {
		t.Errorf("the proposed tasks are %s, want the untracked d200 and 3c5d", got)
	}
	if got := fmt.Sprint(tasksOf(rep, Stale)); got != "[3c5d]" {
		t.Errorf("the stale statuses are %s; the edited status of 9f31 is no commit yet", got)
	}
	for _, x := range of(rep, Proposed) {
		if x.Task != "d200" {
			continue
		}
		if a := f.Authorisation("d200"); a.Why != derive.NoCommit || a.Commit != nil {
			t.Errorf("the untracked task reads %+v", a)
		}
		if x.Message != "The task stands proposed: no commit on the trunk holds its file" || x.Resolver != "ben@example.org" || x.Act != Authorise {
			t.Errorf("the untracked task gives %q for %q", x.Message, x.Resolver)
		}
	}
	if a := f.Authorisation("7b2e"); !a.Authorised || !a.Uncommitted {
		t.Errorf("the edited task reads %+v; want it authorised and uncommitted", a)
	}
	if s := f.Status("9f31"); !s.Uncommitted || s.Commit == nil {
		t.Errorf("the edited status reads %+v", s)
	}
}

// year writes a status of b2c9 and commits it as pat on a day of 2025.
func year(r *repository, day, gate, state string) string {
	return r.commit("pat", day, "Record "+gate, map[string]string{".tableaux/status/b2c9.yaml": "gate: " + gate + "\nstate: " + state + "\n"})
}

// T9: a complete status, a refusal and a project with no pass.
func TestCompleteRefusedAndNoPass(t *testing.T) {
	// A status at its last gate, complete, a year old: nothing waits.
	r := newRepository(t, "")
	year(r, "2025-09-02", "design", "nominal")
	f := r.facts()
	rep := audited(t, Input{Facts: f, Now: "2026-09-10"})
	if got := fmt.Sprint(tasksOf(rep, Stale), len(of(rep, SoleReview))); got != "[b2c9] 3" {
		t.Errorf("a year-old status at design gives %s; want it stale, with design, unit and release for nobody else to review", got)
	}
	year(r, "2025-09-03", "release", "complete")
	f = r.facts()
	if s := f.Status("b2c9"); s.State != "complete" || s.Gate != f.Last("b2c9") || s.Date != "2025-09-03" {
		t.Fatalf("the status reads %+v", s)
	}
	rep = audited(t, Input{Facts: f, Now: "2026-09-10"})
	if len(rep.Findings) != 0 {
		t.Errorf("a complete task a year old gives %d findings: %+v", len(rep.Findings), rep.Findings)
	}
	for _, gate := range f.Gates() {
		if gate != "undefined" && !historical(f, "b2c9", gate) {
			t.Errorf("the junction of a complete task at %s is not historical", gate)
		}
	}

	// No pass: rule findings and sole reviews alone.
	bare := mem(t, map[string]string{
		"tasks/e4a1.yaml":  "title: Root\nassignee: olive@example.org\n",
		"tasks/b2c9.yaml":  "title: Leaf\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 1 }\njunctions:\n  design: { reviewer: olive@example.org }\n",
		"status/b2c9.yaml": "gate: mockup\nstate: nominal\n",
	})
	diagnostics := []model.Diagnostic{{Code: "T12", Severity: model.Warning, Pos: model.Pos{File: "tasks/e4a1.yaml"}, Task: "e4a1", Message: "b2c9 and c3d7 share order 1 under e4a1"}}
	rep = audited(t, Input{Facts: bare, Diagnostics: diagnostics, Now: "2030-01-01"})
	keys := map[string]int{}
	for _, x := range rep.Findings {
		keys[x.Key]++
	}
	// mockup is the gate the status names, function and release follow, design has a reviewer.
	if fmt.Sprint(keys) != "map[T12:1 sole_review:3]" {
		t.Errorf("with no pass the findings are %v", keys)
	}
	if got := fmt.Sprint(gatesOf(rep, SoleReview)); got != "[mockup function release]" {
		t.Errorf("with no pass the sole reviews stand at %s", got)
	}

	// A refusal: no report, and no error.
	refused := derived(t, load.Options{}, builtAt(t, "gates-missing"), "main", "")
	if refused.Refused() == nil {
		t.Fatal("gates-missing is not refused")
	}
	if rep, err := Run(Input{Facts: refused, Diagnostics: validate.Files(refused.Project()), Now: "2026-09-01"}); rep != nil || err != nil {
		t.Errorf("a refusal gives %v, %v; want nil, nil", rep, err)
	}
}

// gatesOf returns the gates of the findings with a key, in their order.
func gatesOf(rep *Report, key string) []string {
	var gates []string
	for _, x := range of(rep, key) {
		gates = append(gates, x.Gate)
	}
	return gates
}

// sole returns the sole reviews of a report as "task gate" lines.
func sole(rep *Report) map[string]*Finding {
	at := map[string]*Finding{}
	for _, x := range of(rep, SoleReview) {
		at[x.Task+" "+x.Gate] = x
	}
	return at
}

// T12: the sole review, in both cases (decision 1): an agent whose reviewer
// is itself and a person's plain junction that states none, told apart by
// the junction and by no second kind; none at defined, where the
// authorisation reviews, at a historical junction, at a junction another
// person reviews, or on a parent.
func TestSoleReview(t *testing.T) {
	station := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	rep := audited(t, Input{Facts: station, Now: "2026-09-28"})
	at := sole(rep)
	if len(of(rep, SoleReview)) != 32 || len(at) != 32 {
		t.Errorf("weather-station at main gives %d sole reviews, want 32", len(of(rep, SoleReview)))
	}
	people := map[string]bool{}
	for key, x := range at {
		people[x.Resolver] = true
		j := station.Junction(x.Task, x.Gate)
		if x.Severity != model.Information || x.Demoted || x.Act != None || x.Rule != "" || x.Diagnostic != nil ||
			x.Kind != "A junction nobody else reviews" || x.Message != "Nobody but the contributor reviews the work at the gate" ||
			x.File != ".tableaux/tasks/"+x.Task+".yaml" {
			t.Errorf("the sole review of %s reads %+v", key, x)
		}
		if x.Gate == "defined" || !station.Leaf(x.Task) || j == nil || j.Kind != model.Plain || (j.Reviewer.V != "" && j.Reviewer.V != j.Contributor.V) {
			t.Errorf("%s is no junction nobody else reviews: %+v", key, j)
		}
		if historical(station, x.Task, x.Gate) {
			t.Errorf("%s is historical", key)
		}
	}
	if fmt.Sprint(people) != "map[ada@example.org:true ben@example.org:true]" {
		t.Errorf("the sole reviews of weather-station are for %v", people)
	}
	for _, key := range []string{
		"c07d mockup",                  // a historical junction: the status names design
		"c07d unit",                    // an agent's, which ben reviews as the assignee
		"c07d implementation",          // recursive
		"c07d reliability",             // exempt
		"9f31 mockup", "9f31 validate", // ben reviews
		"9f31 defined", "7b2e defined", // the authorisation reviews
		"4e2b design", "a1c0 design", // a parent's junctions are defaults
	} {
		if at[key] != nil {
			t.Errorf("weather-station lists a sole review at %s", key)
		}
	}
	for _, key := range []string{"9f31 function", "9f31 performance", "c07d design", "c07d integrate", "7b2e mockup", "3c5d mockup"} {
		if at[key] == nil {
			t.Errorf("weather-station lists no sole review at %s", key)
		}
	}
	if j := station.Junction("c07d", "unit"); j.Model.V == "" || j.Reviewer.V != "ben@example.org" || j.Reviewer.By != derive.ByAssignee {
		t.Errorf("c07d at unit reads %+v", j)
	}

	// On the branch: a person who names himself reviewer.
	branch := derived(t, load.Options{}, builtAt(t, "weather-station"), "sensor-board", "")
	rep = audited(t, Input{Facts: branch, Now: "2026-09-18"})
	at = sole(rep)
	if len(at) != 25 {
		t.Errorf("weather-station at sensor-board gives %d sole reviews, want 25", len(at))
	}
	if x, j := at["c07d mockup"], branch.Junction("c07d", "mockup"); x == nil || j.Reviewer.V != "ben@example.org" || j.Contributor.V != "ben@example.org" || j.Model.V != "" {
		t.Errorf("c07d at mockup on the branch reads %+v, %+v; want ben, who names himself reviewer", x, j)
	}

	// The three cases of one row: an agent whose reviewer is itself, and two
	// junctions that state none.
	kinds := derived(t, load.Options{}, builtAt(t, "junction-kinds"), "main", "")
	rep = audited(t, Input{Facts: kinds, Now: "2026-09-10"})
	at = sole(rep)
	if len(at) != 41 {
		t.Errorf("junction-kinds gives %d sole reviews, want 41", len(at))
	}
	agent, first, second := kinds.Junction("a300", "design"), kinds.Junction("a400", "design"), kinds.Junction("a600", "design")
	if at["a300 design"] == nil || agent.Model.V == "" || agent.Reviewer.V != agent.Contributor.V || agent.Reviewer.By != derive.ByAssignee {
		t.Errorf("a300 at design reads %+v, %+v; want an agent whose reviewer is itself", at["a300 design"], agent)
	}
	for id, j := range map[string]*derive.Junction{"a400": first, "a600": second} {
		if at[id+" design"] == nil || j.Model.V != "" || j.Reviewer.V != "" {
			t.Errorf("%s at design reads %+v, %+v; want a person's junction that states no reviewer", id, at[id+" design"], j)
		}
	}
	for _, x := range []*Finding{at["a300 design"], at["a400 design"], at["a600 design"]} {
		if x == nil || x.Resolver != "olive@example.org" || x.Message != at["a300 design"].Message {
			t.Errorf("the three do not read as one row: %+v", x)
		}
	}
	// a110 at function has a reviewer who is not its contributor, and at
	// design an agent the assignee reviews.
	for _, key := range []string{"a110 function", "a110 design", "a110 unit", "a500 unit", "a600 mockup", "a000 release", "a100 unit"} {
		if at[key] != nil {
			t.Errorf("junction-kinds lists a sole review at %s", key)
		}
	}

	// No gate key but the Derivation's fact excludes the junction the
	// authorisation reviews: the package's source names it nowhere.
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(file, "_test.go") && strings.Contains(string(data), `"defined"`) {
			t.Errorf("%s names the gate defined", file)
		}
	}
	for _, id := range station.Order() {
		for _, j := range station.Junctions(id) {
			if got, review := station.ByAuthorisation(id, j.Gate), station.Accepted(id, j.Gate); review != nil && got != review.ByAuthorisation {
				t.Errorf("ByAuthorisation(%s, %s) is %v and the review says %v", id, j.Gate, got, review.ByAuthorisation)
			}
		}
	}
}
