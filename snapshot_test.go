package tablo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablo/internal/validate"
)

var allViews = []View{Gates, Task, Authority, Assignment, Queue, Blockage, Tableau, Context, History, Audit}

// builtEnvelopes returns, as snapshot.go builds them over s, the envelope of
// validate and of every view at its default level and at each level, as the
// viewer sees them, with b2c9 as the task view's task, a queue's brief at
// the agent's junction and a history over a range.
func builtEnvelopes(t testing.TB, s *Snapshot, viewer string) map[string]*Envelope {
	t.Helper()
	ctx := context.Background()
	out := map[string]*Envelope{"validate": s.Validate()}
	view := func(name string, v View, p Params) {
		t.Helper()
		e, err := s.View(ctx, v, p)
		if err != nil {
			t.Fatalf("%s: View: %v", name, err)
		}
		out[name] = e
	}
	for _, v := range allViews {
		p := Params{Viewer: viewer, Now: "2026-09-10"}.Narrow(v)
		if v == Task {
			p.Task = "b2c9"
		}
		view(string(v), v, p)
		for _, l := range []Level{LevelGlance, LevelDetail, LevelProvenance} {
			p.Level = l
			view(string(v)+" "+string(l), v, p)
		}
	}
	view("queue brief", Queue, Params{Person: "bot@example.org", Brief: &Brief{Task: "b2c9", Gate: "design"}})
	first := s.facts.Log().Commits[len(s.facts.Log().Commits)-1].ID
	view("history range", History, Params{From: first, Level: LevelProvenance})
	return out
}

// TestViewsOverTheSmallProject loads the small project from its working tree
// and builds every view: each envelope names the view, a viewer and the
// level, and carries data.
func TestViewsOverTheSmallProject(t *testing.T) {
	s := small(t).load("")
	src := s.Source()
	if src.Ref != "main" || !src.Worktree || src.Commit == nil || len(*src.Commit) != 40 || src.Date == nil || *src.Date != "2026-09-03" ||
		src.Trunk.Name == nil || *src.Trunk.Name != "main" || src.Trunk.Resolved != "stated" || !src.OnTrunk {
		t.Errorf("the source reads %+v", src)
	}
	if p := s.Project(); p == nil || *p != (ProjectInfo{Tableaux: "0.3.1", Root: "e4a1", Title: "Base", Owner: "olive@example.org"}) {
		t.Errorf("the project reads %+v", p)
	}
	for name, e := range builtEnvelopes(t, s, "olive@example.org") {
		if e.Data == nil || e.Viewer == nil && name != "validate" {
			t.Errorf("%s: data %v, viewer %v", name, e.Data, e.Viewer)
		}
	}
	if !s.HasTask("b2c9") || s.HasTask("abcd") || s.HasTask("") {
		t.Errorf("HasTask does not agree with the files")
	}
	if got := strings.Join(s.GateKeys(), " "); got != "undefined defined design release" {
		t.Errorf("GateKeys = %s", got)
	}
}

// TestDefaults is T4 as far as the library resolves defaults: the person
// and the task per view, and the level from --brief, --level, --role and
// the viewer's roles. The viewer's own default, from the option, the
// variable and the configuration, is the command's (internal/cli).
func TestDefaults(t *testing.T) {
	s := small(t).load("")
	for email, want := range map[string]string{
		"olive@example.org": "owner authority assignee",
		"pat@example.org":   "assignee contributor reviewer",
		"bot@example.org":   "contributor agent",
		"ben@example.org":   "assignee contributor",
		"dan@example.org":   "observer",
		"":                  "observer",
	} {
		v := s.Viewer(email)
		var roles []string
		for _, r := range v.Roles {
			roles = append(roles, string(r))
		}
		if strings.Join(roles, " ") != want || (v.Email == nil) != (email == "") {
			t.Errorf("Viewer(%q) = %v, %v; want %s", email, v.Email, roles, want)
		}
	}
	tests := []struct {
		name string
		view View
		p    Params
		want string // the params as the envelope writes them
	}{
		{"gates opens at detail for every role", Gates, Params{}, `{"task":null,"role":null,"level":"detail"}`},
		{"task", Task, Params{Task: "b2c9", Viewer: "dan@example.org"}, `{"task":"b2c9","person":null,"role":null,"level":"detail"}`},
		{"authority for the owner", Authority, Params{Viewer: "olive@example.org"}, `{"task":"e4a1","person":null,"proposed":false,"role":null,"level":"detail"}`},
		{"authority for an agent", Authority, Params{Viewer: "bot@example.org"}, `{"task":"e4a1","person":null,"proposed":false,"role":null,"level":"glance"}`},
		{"assignment for an assignee", Assignment, Params{Viewer: "pat@example.org"}, `{"task":"e4a1","person":null,"role":null,"level":"detail"}`},
		{"queue for its viewer", Queue, Params{Viewer: "pat@example.org"}, `{"task":"e4a1","person":"pat@example.org","brief":null,"role":null,"level":"detail"}`},
		{"queue for an observer", Queue, Params{Viewer: "dan@example.org"}, `{"task":"e4a1","person":"dan@example.org","brief":null,"role":null,"level":"glance"}`},
		{"queue of another person", Queue, Params{Viewer: "olive@example.org", Person: "ben@example.org"}, `{"task":"e4a1","person":"ben@example.org","brief":null,"role":null,"level":"detail"}`},
		{"a brief is provenance", Queue, Params{Person: "bot@example.org", Brief: &Brief{Task: "b2c9", Gate: "design"}}, `{"task":"e4a1","person":"bot@example.org","brief":{"task":"b2c9","gate":"design"},"role":null,"level":"provenance"}`},
		{"blockage for a contributor", Blockage, Params{Viewer: "bot@example.org"}, `{"task":"e4a1","person":null,"role":null,"level":"detail"}`},
		{"tableau for the owner", Tableau, Params{Viewer: "olive@example.org"}, `{"person":null,"window":1,"columns":null,"historical":false,"role":null,"level":"detail"}`},
		{"tableau for another", Tableau, Params{Viewer: "pat@example.org"}, `{"person":null,"window":1,"columns":null,"historical":false,"role":null,"level":"glance"}`},
		{"tableau by columns", Tableau, Params{Columns: []string{"design"}}, `{"person":null,"window":null,"columns":["design"],"historical":false,"role":null,"level":"glance"}`},
		{"context of the viewer", Context, Params{Viewer: "pat@example.org"}, `{"task":null,"person":"pat@example.org","window":1,"columns":null,"historical":false,"role":null,"level":"detail"}`},
		{"context of a task", Context, Params{Viewer: "pat@example.org", Task: "b2c9"}, `{"task":"b2c9","person":null,"window":1,"columns":null,"historical":false,"role":null,"level":"detail"}`},
		{"history for the owner", History, Params{Viewer: "olive@example.org"}, `{"task":"e4a1","person":null,"role":null,"level":"provenance"}`},
		{"history for an observer", History, Params{}, `{"task":"e4a1","person":null,"role":null,"level":"detail"}`},
		{"audit for the owner", Audit, Params{Viewer: "olive@example.org", Now: "2026-09-10"}, `{"task":"e4a1","person":null,"stale":7,"now":"2026-09-10","minimum":"warning","role":null,"level":"provenance"}`},
		{"audit for an assignee", Audit, Params{Viewer: "ben@example.org", Now: "2026-09-10", Stale: 3, Minimum: "error"}, `{"task":"e4a1","person":null,"stale":3,"now":"2026-09-10","minimum":"error","role":null,"level":"glance"}`},
		{"the level beats the roles", History, Params{Viewer: "olive@example.org", Level: LevelGlance}, `{"task":"e4a1","person":null,"role":null,"level":"glance"}`},
		{"a role the viewer lacks", Audit, Params{Viewer: "dan@example.org", Role: RoleOwner, Now: "2026-09-10"}, `{"task":"e4a1","person":null,"stale":7,"now":"2026-09-10","minimum":"warning","role":"owner","level":"provenance"}`},
		{"the role beats the viewer's", Tableau, Params{Viewer: "olive@example.org", Role: RoleAgent}, `{"person":null,"window":1,"columns":null,"historical":false,"role":"agent","level":"glance"}`},
		{"the authority opens the audit at detail", Audit, Params{Role: RoleAuthority, Now: "2026-09-10"}, `{"task":"e4a1","person":null,"stale":7,"now":"2026-09-10","minimum":"warning","role":"authority","level":"detail"}`},
	}
	for _, tc := range tests {
		e, err := s.View(context.Background(), tc.view, tc.p)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		got, err := json.Marshal(e.Params)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: params %s\n want %s", tc.name, got, tc.want)
		}
		if tc.p.Viewer != "" && (e.Viewer.Email == nil || *e.Viewer.Email != tc.p.Viewer) {
			t.Errorf("%s: the envelope names the viewer %v", tc.name, e.Viewer.Email)
		}
	}
}

// TestViewRefuses covers the errors of View: the usage errors of Check and
// of the defaults, and a task, a gate or a ref the project or the
// repository lacks.
func TestViewRefuses(t *testing.T) {
	s := small(t).load("HEAD")
	var usage *UsageError
	var missing *NotFoundError
	for _, tc := range []struct {
		name string
		view View
		p    Params
		kind string // "usage", or the Kind of a *NotFoundError
	}{
		{"an option the view does not take", Gates, Params{Person: "a@b.c"}, "usage"},
		{"a queue with no person and no viewer", Queue, Params{}, "usage"},
		{"a context with no task, person or viewer", Context, Params{}, "usage"},
		{"an audit with no date", Audit, Params{}, "usage"},
		{"an unknown task", Task, Params{Task: "abcd"}, "task"},
		{"an unknown task by option", Authority, Params{Task: "abcd"}, "task"},
		{"an unknown column", Tableau, Params{Columns: []string{"mockup"}}, "gate"},
		{"an unknown brief task", Queue, Params{Person: "a@b.c", Brief: &Brief{Task: "abcd", Gate: "design"}}, "task"},
		{"an unknown brief gate", Queue, Params{Person: "a@b.c", Brief: &Brief{Task: "b2c9", Gate: "mockup"}}, "gate"},
		{"an unknown range start", History, Params{From: "nowhere"}, "ref"},
	} {
		_, err := s.View(context.Background(), tc.view, tc.p)
		switch {
		case tc.kind == "usage" && !errors.As(err, &usage):
			t.Errorf("%s: %v, want a *UsageError", tc.name, err)
		case tc.kind != "usage" && (!errors.As(err, &missing) || missing.Kind != tc.kind):
			t.Errorf("%s: %v, want a *NotFoundError of a %s", tc.name, err, tc.kind)
		}
	}
	r, err := Open(context.Background(), small(t).dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Load(context.Background(), "nowhere"); !errors.As(err, &missing) || missing.Kind != "ref" {
		t.Errorf("Load of an unknown ref: %v", err)
	}
	if _, err := r.Resolve(context.Background(), "nowhere"); !errors.As(err, &missing) {
		t.Errorf("Resolve of an unknown ref: %v", err)
	}
	if _, err := Open(context.Background(), t.TempDir(), Options{}); !errors.As(err, new(*ReadError)) {
		t.Errorf("Open outside a repository: %v", err)
	}
	tree, err := r.Load(context.Background(), "HEAD^{tree}")
	if err != nil {
		t.Fatal(err)
	}
	if src := tree.Source(); src.Commit != nil || src.Date != nil || src.Worktree {
		t.Errorf("a tree reads %+v", src)
	}
	if _, err := tree.View(context.Background(), Gates, Params{}); !errors.As(err, new(*ReadError)) {
		t.Errorf("a view of a tree: %v, want a *ReadError", err)
	}
	if e := tree.Validate(); e.Data.(validateData) != (validateData{Valid: true}) {
		t.Errorf("validate of a tree: %+v", e.Data)
	}
}

// TestDataIsNullWhenTheDerivationRefuses: a gates.yaml the Derivation cannot
// use leaves every view's data null beside its diagnostics, and exit 1.
func TestDataIsNullWhenTheDerivationRefuses(t *testing.T) {
	b := small(t)
	b.put(map[string]string{"gates.yaml": "gates: []\n"})
	s := b.load("")
	for _, v := range allViews {
		p := Params{Viewer: "olive@example.org", Now: "2026-09-10"}.Narrow(v)
		if v == Task {
			p.Task = "b2c9"
		}
		e, err := s.View(context.Background(), v, p)
		if err != nil {
			t.Fatalf("%s: %v", v, err)
		}
		if e.Data != nil || e.ExitCode() != 1 || !bytes.Contains(e.JSON(), []byte(`"data": null`)) {
			t.Errorf("%s over a refused project: data %v, exit %d", v, e.Data, e.ExitCode())
		}
	}
	if _, err := s.View(context.Background(), Audit, Params{}); !errors.As(err, new(*UsageError)) {
		t.Errorf("an audit with no date over a refused project: %v", err)
	}
	// The small project derives: its data is no null.
	if e, err := small(t).load("").View(context.Background(), Gates, Params{}); err != nil || e.Data == nil {
		t.Errorf("gates over the small project: %v, %v", e, err)
	}
}

// reviewed builds a project whose leaf b2c9 has a design junction that pat
// contributes and olive reviews, and returns it after the plan, which
// records the leaf at defined.
func reviewed(t testing.TB) *built {
	t.Helper()
	b := build(t)
	b.put(map[string]string{
		"version.yaml": baseVersion, "gates.yaml": baseGates, "tasks/e4a1.yaml": baseRoot,
		"tasks/b2c9.yaml": strings.Replace(baseLeaf, "{ contributor: bot@example.org, model: claude-fable }",
			"{ contributor: pat@example.org, reviewer: olive@example.org }", 1),
		"status/b2c9.yaml": "gate: defined\nstate: nominal\n",
	})
	b.commit("olive", "2026-09-01", "Plan the base")
	return b
}

func codes(s *Snapshot) string {
	var list []string
	for _, d := range s.Diagnostics() {
		list = append(list, d.Code)
	}
	return strings.Join(list, " ")
}

// TestReviewsReachTheValidator is the test decision 14 asks of
// History.Reviews: a status that passes a reviewed junction with its
// accepting commit raises no S11. Without the map's key the same history
// draws S11, which the adapter would then raise silently.
func TestReviewsReachTheValidator(t *testing.T) {
	b := reviewed(t)
	accepts := b.commit("olive", "2026-09-02", "Accept the design", "Reviewed: b2c9 design")
	b.put(map[string]string{"status/b2c9.yaml": "gate: design\nstate: nominal\n"})
	b.commit("pat", "2026-09-03", "Record the design")
	s := b.load("HEAD")
	if got := codes(s); strings.Contains(got, "S11") {
		t.Errorf("a status past a reviewed junction with its review raises %s", got)
	}
	facts := validateFacts(s.facts)
	key := validate.Junction{Task: "b2c9", Gate: "design"}
	if got, ok := facts.History.Reviews[key]; !ok || got != accepts {
		t.Errorf("Reviews holds %q, %v for b2c9 design; want %s", got, ok, accepts)
	}
	if _, ok := facts.History.Reviews[validate.Junction{Task: "b2c9", Gate: "release"}]; ok {
		t.Errorf("Reviews holds the release junction, which has no reviewer")
	}
	delete(facts.History.Reviews, key)
	if got := validate.Derived(s.project, facts); len(got) != 1 || got[0].Code != "S11" {
		t.Errorf("without the key the Validator raises %+v; want S11", got)
	}
	// The next junction's key is there before its review, empty.
	before := reviewed(t)
	s = before.load("HEAD")
	if got, ok := validateFacts(s.facts).History.Reviews[key]; !ok || got != "" {
		t.Errorf("before the review, Reviews holds %q, %v for the next junction", got, ok)
	}
}

// TestJunctionsAtTheCommit is the test decision 14 asks of
// Commit.Junctions: a junction's reviewer changed after its review raises no
// H2, since the files at the commit judge the commit, while a review from
// the wrong hand still raises it.
func TestJunctionsAtTheCommit(t *testing.T) {
	b := reviewed(t)
	wrong := b.commit("pat", "2026-09-02", "Review my own design", "Reviewed: b2c9 design")
	right := b.commit("olive", "2026-09-03", "Accept the design", "Reviewed: b2c9 design")
	b.put(map[string]string{"tasks/b2c9.yaml": strings.Replace(baseLeaf, "{ contributor: bot@example.org, model: claude-fable }",
		"{ contributor: pat@example.org, reviewer: dan@example.org }", 1)})
	b.commit("olive", "2026-09-04", "Hand the review to dan")
	s := b.load("HEAD")
	var h2 []string
	for _, d := range s.Diagnostics() {
		if d.Code == "H2" {
			h2 = append(h2, d.Commit)
		}
	}
	if len(h2) != 1 || h2[0] != wrong {
		t.Errorf("H2 names %v; want the wrong hand's commit %s alone", h2, wrong)
	}
	for _, c := range validateFacts(s.facts).History.Commits {
		if c.Hash == right {
			if r := c.Junctions[validate.Junction{Task: "b2c9", Gate: "design"}]; r.Reviewer != "olive@example.org" {
				t.Errorf("the accepting commit reads the reviewer %q", r.Reviewer)
			}
		}
	}
}

// TestModelsAtTheCommit: the junctions a commit is at reach H3 and H6, which
// read the model there.
func TestModelsAtTheCommit(t *testing.T) {
	b := small(t)
	b.put(map[string]string{"status/b2c9.yaml": "gate: defined\nstate: nominal\nnote: Further\n"})
	missing := b.commit("bot", "2026-09-04", "Go on with the design")
	b.put(map[string]string{"status/b2c9.yaml": "gate: defined\nstate: nominal\nnote: Further still\n"})
	outside := b.commit("bot", "2026-09-05", "Go on again", "Model: claude-sonnet-5")
	s := b.load("HEAD")
	got := map[string]string{}
	for _, d := range s.Diagnostics() {
		if d.Code == "H3" || d.Code == "H6" {
			got[d.Code] = d.Commit
		}
	}
	if got["H6"] != missing || got["H3"] != outside || len(got) != 2 {
		t.Errorf("H3 and H6 name %v; want H6 at %s and H3 at %s", got, missing, outside)
	}
}

// TestCheckouts covers the method that exports what a watcher polls: the
// checkout of a submodule whose HEAD gave the pin of a working-tree load,
// and nothing for a load at a ref.
func TestCheckouts(t *testing.T) {
	sub := build(t)
	sub.put(map[string]string{
		"version.yaml": baseVersion, "gates.yaml": baseGates, "tasks/f1a0.yaml": strings.Replace(baseRoot, "olive", "ben", 1),
	})
	sub.commit("ben", "2026-09-01", "Plan the firmware")
	b := small(t)
	b.git("olive", "2026-09-04", "-c", "protocol.file.allow=always", "submodule", "add", "-q", sub.dir, "firmware")
	b.put(map[string]string{"tasks/c3d7.yaml": baseSecond + "junctions:\n  design: { subproject: { url: firmware } }\n"})
	b.commit("olive", "2026-09-04", "Link the firmware")
	s := b.load("")
	want := []string{filepath.Join(b.dir, "firmware")}
	if got := s.Checkouts(); !reflect.DeepEqual(got, want) {
		t.Errorf("Checkouts of the working tree = %v, want %v", got, want)
	}
	if got := b.load("HEAD").Checkouts(); len(got) != 0 || got == nil {
		t.Errorf("Checkouts at HEAD = %#v, want an empty list", got)
	}
}

// TestRange covers a range on history: a start the pass holds, and one that
// no ancestor of the source or the trunk reaches, for which the snapshot
// reads the history once more.
func TestRange(t *testing.T) {
	b := small(t)
	first := b.git("olive", "2026-09-04", "rev-list", "--max-parents=0", "HEAD")
	b.git("olive", "2026-09-04", "checkout", "-q", "-b", "side", first)
	b.put(map[string]string{"tasks/c3d7.yaml": strings.Replace(baseSecond, "Second leaf", "Second leaf, renamed", 1)})
	side := b.commit("olive", "2026-09-04", "Rename the second leaf")
	b.git("olive", "2026-09-04", "checkout", "-q", "main")
	s := b.load("main")
	if s.facts.Log().Commit(side) != nil {
		t.Fatalf("the pass of main holds the side branch")
	}
	for _, from := range []string{first, "side"} {
		e, err := s.View(context.Background(), History, Params{From: from})
		if err != nil {
			t.Fatalf("history from %s: %v", from, err)
		}
		if e.Source.From == nil || e.Source.From.Ref != from || len(e.Source.From.Commit) != 40 {
			t.Errorf("history from %s: source.from %+v", from, e.Source.From)
		}
		if s.Source().From != nil {
			t.Errorf("the snapshot's source changes with a view")
		}
	}
	whole, _ := s.View(context.Background(), History, Params{})
	ranged, _ := s.View(context.Background(), History, Params{From: first})
	if bytes.Equal(whole.JSON(), ranged.JSON()) {
		t.Errorf("a range gives the whole history")
	}
	if len(s.ranges) != 1 {
		t.Errorf("the snapshot reads %d further passes; want one, for side", len(s.ranges))
	}
}

// TestSnapshotIsSafeToShare is T14: fifty goroutines build every view of one
// snapshot under -race, and every result equals the first.
func TestSnapshotIsSafeToShare(t *testing.T) {
	s := small(t).load("")
	results := make([]map[View][]byte, 50)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got := map[View][]byte{}
			for k := range allViews {
				v := allViews[(i+k)%len(allViews)]
				p := Params{Viewer: "pat@example.org", Now: "2026-09-10"}.Narrow(v)
				if v == Task {
					p.Task = "b2c9"
				}
				e, err := s.View(context.Background(), v, p)
				if err != nil {
					t.Errorf("%s: %v", v, err)
					return
				}
				got[v] = e.JSON()
			}
			results[i] = got
		}(i)
	}
	wg.Wait()
	for i, r := range results[1:] {
		for _, v := range allViews {
			if !bytes.Equal(r[v], results[0][v]) {
				t.Errorf("goroutine %d: %s differs from the first", i+1, v)
			}
		}
	}
}

// TestTheWeatherStation builds, with the corpus, the four views decision 16
// names through snapshot.go: the tableau equals tableau.json, and the
// tableau, the queue, the history and the audit validate against the schema.
func TestTheWeatherStation(t *testing.T) {
	s := loadAt(t, filepath.Join(corpus(t), "weather-station"), "main", Options{})
	sch := schema(t)
	for _, tc := range []struct {
		view View
		p    Params
	}{
		{Tableau, Params{Viewer: "ada@example.org"}},
		{Queue, Params{Viewer: "ada@example.org"}},
		{Queue, Params{Person: "dan@example.org", Level: LevelProvenance}},
		{History, Params{Viewer: "ada@example.org"}},
		{History, Params{Task: "c07d", Level: LevelProvenance}},
		{Audit, Params{Viewer: "ada@example.org", Now: "2026-09-28"}},
		{Audit, Params{Now: "2026-09-28", Minimum: "information", Level: LevelDetail}},
	} {
		e, err := s.View(context.Background(), tc.view, tc.p)
		if err != nil {
			t.Fatalf("%s: %v", tc.view, err)
		}
		if err := validates(t, sch, decodeMap(t, e.JSON())); err != nil {
			t.Errorf("%s %+v: %v", tc.view, tc.p, err)
		}
		if tc.view == Tableau && !bytes.Equal(e.JSON(), golden(t, "tableau.json")) {
			t.Errorf("the tableau differs from tableau.json:\n%s", e.JSON())
		}
	}
}

// TestCorpusConformsBeyondTheGoldens runs snapshot.go over each corpus entry
// at each of its refs and checks that the envelope of every view validates
// and that a view never fails on a project the files give.
func TestCorpusConformsBeyondTheGoldens(t *testing.T) {
	sch := schema(t)
	for _, e := range corpusEntries(t) {
		for _, ref := range e.refs {
			s := loadAt(t, e.dir, ref, Options{Trunk: e.trunk, Replace: e.replace})
			if err := validates(t, sch, decodeMap(t, s.Validate().JSON())); err != nil {
				t.Errorf("%s %s validate: %v", e.name, ref, err)
			}
			info := s.Project()
			for _, v := range allViews {
				p := Params{Viewer: "olive@example.org", Now: "2026-09-28"}.Narrow(v)
				if v == Task {
					if info == nil {
						continue
					}
					p.Task = info.Root
				}
				env, err := s.View(context.Background(), v, p)
				if err != nil {
					t.Errorf("%s %s %s: %v", e.name, ref, v, err)
					continue
				}
				if err := validates(t, sch, decodeMap(t, env.JSON())); err != nil {
					t.Errorf("%s %s %s: %v", e.name, ref, v, err)
				}
			}
		}
	}
}

// writeOut writes a test's output beside its failure, for a reader to diff.
func writeOut(t testing.TB, name string, data []byte) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(out, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestSourceDir: the source names the project's directory relative to the
// repository's root, "." at the root, whether -C names the project's
// directory or one below it, and the directory -C names where no project
// is (decision 11).
func TestSourceDir(t *testing.T) {
	b := small(t)
	b.put(map[string]string{"../lib/.tableaux/version.yaml": baseVersion, "../lib/.tableaux/gates.yaml": baseGates,
		"../lib/.tableaux/tasks/5a00.yaml": strings.Replace(baseRoot, "olive", "ben", 1)})
	if err := os.MkdirAll(filepath.Join(b.dir, "lib", "src", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(b.dir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	b.commit("ben", "2026-09-04", "Add the library")
	for _, tc := range []struct{ dir, want, root string }{
		{".", ".", "e4a1"},
		{filepath.Join(".tableaux", "tasks"), ".", "e4a1"},
		{"docs", ".", "e4a1"},
		{"lib", "lib", "5a00"},
		{filepath.Join("lib", "src", "deep"), "lib", "5a00"},
	} {
		for _, ref := range []string{"", "HEAD"} {
			s := loadAt(t, filepath.Join(b.dir, tc.dir), ref, Options{})
			if got := s.Source().Dir; got != tc.want || s.Project() == nil || s.Project().Root != tc.root {
				t.Errorf("-C %s at %q: dir %q, project %+v; want %s and %s", tc.dir, ref, got, s.Project(), tc.want, tc.root)
			}
			if err := validates(t, schema(t), decodeMap(t, s.Validate().JSON())); err != nil {
				t.Errorf("-C %s at %q: %v", tc.dir, ref, err)
			}
		}
	}
	none := build(t)
	none.put(map[string]string{"../sub/README.md": "# Nothing\n"})
	none.commit("olive", "2026-09-01", "Start")
	s := loadAt(t, filepath.Join(none.dir, "sub"), "HEAD", Options{})
	if got := s.Source().Dir; got != "sub" || s.Project() != nil {
		t.Errorf("-C sub with no project: dir %q, project %+v", got, s.Project())
	}
}
