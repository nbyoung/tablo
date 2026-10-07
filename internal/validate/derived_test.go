package validate

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/model"
)

// factsCommit is one commit of testdata/facts.yaml: one identity is its
// author and its committer, and its label stands for its hash.
type factsCommit struct {
	Hash, By, Tableaux string
	Trailers           []string
	Events             []Event
	At                 []Junction
}

// factsEntry is one entry of testdata/facts.yaml: the facts a test hands
// Derived for a corpus entry, and the diagnostics it then gives.
type factsEntry struct {
	Entry     string
	Junctions map[string]struct {
		Recursive                    bool
		Contributor, Model, Reviewer string
	}
	Requirements []Condition
	History      *struct {
		Trunk   string
		Commits []factsCommit
		Reviews map[string]string
	}
	Want []string
}

// junctionOf reads a junction as facts.yaml keys it: the task, a space, the gate.
func junctionOf(t *testing.T, key string) Junction {
	t.Helper()
	task, gate, ok := strings.Cut(key, " ")
	if !ok {
		t.Fatalf("facts.yaml: %q is no task and gate", key)
	}
	return Junction{Task: task, Gate: gate}
}

// defaults returns the plain default of every task at every gate that applies
// to it: the assignee contributes and nobody reviews.
func defaults(p *model.Project) map[Junction]Resolved {
	s := newShape(p)
	all := map[Junction]Resolved{}
	for _, id := range p.TaskIDs() {
		for _, gate := range s.applicable(id) {
			all[Junction{Task: id, Gate: gate}] = Resolved{Contributor: p.Tasks[id].Assignee.V}
		}
	}
	return all
}

// facts builds the facts of one entry of facts.yaml over the plain defaults.
func (e factsEntry) facts(t *testing.T, p *model.Project) *Facts {
	t.Helper()
	f := &Facts{Requirements: e.Requirements, Junctions: defaults(p)}
	for key, j := range e.Junctions {
		f.Junctions[junctionOf(t, key)] = Resolved{Recursive: j.Recursive, Contributor: j.Contributor, Model: j.Model, Reviewer: j.Reviewer}
	}
	if e.History == nil {
		return f
	}
	f.History = &History{Trunk: e.History.Trunk, Reviews: map[Junction]string{}}
	for key, commit := range e.History.Reviews {
		f.History.Reviews[junctionOf(t, key)] = commit
	}
	for _, c := range e.History.Commits {
		commit := Commit{Hash: c.Hash, Author: c.By, Committer: c.By, Tableaux: c.Tableaux, Events: c.Events, At: c.At}
		for _, line := range c.Trailers {
			key, value, _ := strings.Cut(line, ": ")
			commit.Trailers = append(commit.Trailers, Trailer{Key: key, Value: value})
		}
		f.History.Commits = append(f.History.Commits, commit)
	}
	return f
}

// factsEntries reads testdata/facts.yaml.
func factsEntries(t *testing.T) []factsEntry {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "facts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct{ Entries []factsEntry }
	if err := yaml.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	return file.Entries
}

// sorted returns a copy of lines in byte order.
func sorted(lines []string) []string {
	lines = append([]string{}, lines...)
	sort.Strings(lines)
	return lines
}

// derived returns where the diagnostics of Derived sit, and holds each to its
// rule: a rule of Derived, at its severity (T1).
func derived(t *testing.T, p *model.Project, f *Facts) []string {
	t.Helper()
	found := Derived(p, f)
	for _, d := range found {
		if rule, ok := RuleOf(d.Code); !ok || rule.Severity != d.Severity || (rule.Tier != TierFacts && rule.Tier != TierHistory) {
			t.Errorf("Derived raises %s, which is no rule of its own at its severity", where(d))
		}
		if d.Message == "" {
			t.Errorf("%s has no message", where(d))
		}
	}
	return places(found)
}

// TestDerivedOnFacts is T13, first part, and T14: Derived gives the lines
// facts.yaml states for each of its entries, the two with no history among
// them.
func TestDerivedOnFacts(t *testing.T) {
	entries := factsEntries(t)
	names := map[string]bool{}
	for _, e := range entries {
		names[e.Entry] = true
		p := entry(t, e.Entry)
		if got, want := sorted(derived(t, p, e.facts(t, p))), sorted(e.Want); !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n got  %s\n want %s", e.Entry, strings.Join(got, "\n      "), strings.Join(want, "\n      "))
		}
	}
	if len(names) != 9 {
		t.Errorf("facts.yaml holds %d corpus entries; want 9", len(names))
	}
}

// TestDerivedRules is T13, second part: the cases the corpus's nine entries
// do not hold. Each runs on a corpus entry with facts the test states.
func TestDerivedRules(t *testing.T) {
	history := func(commits ...Commit) *History { return &History{Trunk: "main", Commits: commits} }
	by := func(hash, email string, trailers ...string) Commit {
		c := Commit{Hash: hash, Author: email, Committer: email, Tableaux: "0.3.1"}
		for _, line := range trailers {
			key, value, _ := strings.Cut(line, ": ")
			c.Trailers = append(c.Trailers, Trailer{Key: key, Value: value})
		}
		return c
	}

	// H1: a gate that does not apply, a Reviewed with one word, and trailers
	// the method reads. a220 is exempt from release in junction-kinds.
	kinds := entry(t, "junction-kinds")
	f := &Facts{Junctions: defaults(kinds), History: history(
		by("T1", "olive@example.org", "Reviewed: a220 release", "Reviewed: 9f31", "Reviewed: a110 design extra", "Authorised: a110 a220",
			"Reaffirmed: a110", "Authorised: a110", "Reviewed: a110 nowhere", "Reviewed: a110 release", "Model: claude-fable-5-1", "Signed-off-by: a110"),
	)}
	want := []string{
		"warning: H1 commit=T1 trailer=Authorised: a110 a220",
		"warning: H1 commit=T1 trailer=Reviewed: 9f31",
		"warning: H1 commit=T1 trailer=Reviewed: a110 design extra",
		"warning: H1 commit=T1 trailer=Reviewed: a110 nowhere",
		"warning: H1 commit=T1 trailer=Reviewed: a220 release",
	}
	if got := sorted(derived(t, kinds, f)); !reflect.DeepEqual(got, want) {
		t.Errorf("H1:\n got  %s\n want %s", strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}

	// H2 is silent at a junction with no reviewer, and accepts the reviewer
	// as author or as committer; H3 reports a commit and a junction once.
	mismatch := entry(t, "model-mismatch")
	design := Junction{Task: "b2c9", Gate: "design"}
	f = &Facts{Junctions: defaults(mismatch), History: history()}
	f.Junctions[design] = Resolved{Contributor: "bot@example.org", Model: "claude-fable", Reviewer: "pat@example.org"}
	committed := by("C3", "bot@example.org", "Reviewed: b2c9 design")
	committed.Committer = "pat@example.org"
	two := by("C4", "bot@example.org", "Model: claude-fable-5-1", "Model: claude-sonnet-5", "Model: gpt")
	two.At = []Junction{design, {Task: "b2c9", Gate: "release"}}
	f.History.Commits = []Commit{two, committed, by("C2", "olive@example.org", "Reviewed: b2c9 release"), by("C1", "olive@example.org", "Reviewed: b2c9 design")}
	f.History.Reviews = map[Junction]string{design: "C3"}
	want = []string{
		"warning: H2 task=b2c9 gate=design commit=C1",
		"warning: H3 task=b2c9 gate=design commit=C4",
	}
	if got := sorted(derived(t, mismatch, f)); !reflect.DeepEqual(got, want) {
		t.Errorf("H2 and H3:\n got  %s\n want %s", strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}

	// H4 needs a junction that nothing accepts yet (decision 13 f): the facts
	// of handoff-inferred, with an accepting review, give nothing.
	inferred := entry(t, "handoff-inferred")
	f = &Facts{Junctions: defaults(inferred), History: history(Commit{Hash: "I2", Author: "pat@example.org", Events: []Event{{Task: "b2c9", Kind: "status", Gate: "defined"}}})}
	f.Junctions[design] = Resolved{Contributor: "pat@example.org", Reviewer: "olive@example.org"}
	if got, want := derived(t, inferred, f), []string{"status/b2c9.yaml: information: H4 task=b2c9 gate=design commit=I2"}; !reflect.DeepEqual(got, want) {
		t.Errorf("H4 with no accepting review: %v", got)
	}
	f.History.Reviews = map[Junction]string{design: "I1"}
	if got := derived(t, inferred, f); len(got) != 0 {
		t.Errorf("H4 after the reviewer accepts: %v", got)
	}

	// J13 from a literal Pin, on a junction and on a requirement.
	pinned := entry(t, "subproject-pin-off-trunk")
	f = &Facts{Junctions: defaults(pinned), History: history()}
	f.History.OffTrunk = []Pin{{Task: "b2c9", Gate: "design", Index: -1, Trunk: "main"}}
	if got, want := derived(t, pinned, f), []string{"tasks/b2c9.yaml:8:32: warning: J13 task=b2c9 gate=design"}; !reflect.DeepEqual(got, want) {
		t.Errorf("J13 on a junction: %v; want %v", got, want)
	}

	// R9 and R13 from a literal Condition, and J13 on the requirement's commit.
	behind := entry(t, "requires-commit-behind")
	unmet := Condition{Task: "b2c9", Index: 0, Origin: "5a00", From: "design", To: "release", Stands: "defined", Due: true, MetAtTip: true}
	f = &Facts{Junctions: defaults(behind), Requirements: []Condition{unmet}, History: history()}
	f.History.OffTrunk = []Pin{{Task: "b2c9", Index: 0, Trunk: "main"}}
	want = []string{
		"tasks/b2c9.yaml:8:5: warning: R9 task=b2c9",
		"tasks/b2c9.yaml:8:75: warning: J13 task=b2c9",
		"tasks/b2c9.yaml:8:75: warning: R13 task=b2c9",
	}
	if got := derived(t, behind, f); !reflect.DeepEqual(got, want) {
		t.Errorf("R9, R13 and J13 on a requirement:\n got  %v\n want %v", got, want)
	}
	// A requirement the tip does not meet either, and one that is met.
	unmet.MetAtTip = false
	met := unmet
	met.Met = true
	f.Requirements, f.History.OffTrunk = []Condition{unmet, met}, nil
	if got, want := derived(t, behind, f), []string{"tasks/b2c9.yaml:8:5: warning: R9 task=b2c9"}; !reflect.DeepEqual(got, want) {
		t.Errorf("R9 alone: %v", got)
	}
}

// TestWithoutHistory is T14: with no history ten rules skip and R9 stays, and
// Derived returns nil where it has nothing to read.
func TestWithoutHistory(t *testing.T) {
	for _, e := range factsEntries(t) {
		p := entry(t, e.Entry)
		f := e.facts(t, p)
		f.History = nil
		var want []string
		for _, line := range e.Want {
			if strings.Contains(line, ": R9 ") {
				want = append(want, line)
			}
		}
		if got := derived(t, p, f); !reflect.DeepEqual(sorted(got), sorted(want)) {
			t.Errorf("%s with no history: %v; want %v", e.Entry, got, want)
		}
		if got := Derived(p, nil); got != nil {
			t.Errorf("%s with nil facts: %v", e.Entry, places(got))
		}
	}
	// A trunk that no fact names warns of P5, so each case below would
	// report with a project Derived reads.
	f := &Facts{History: &History{}}
	if got := derived(t, entry(t, "unmet-requirement"), f); !reflect.DeepEqual(got, []string{"version.yaml: warning: P5"}) {
		t.Fatalf("an undetermined trunk: %v", got)
	}
	for _, name := range []string{"no-tableaux-directory", "gates-missing", "version-missing", "version-minor-ahead", "version-bad-pattern"} {
		if got := Derived(entry(t, name), f); got != nil {
			t.Errorf("%s: Derived reports %v", name, places(got))
		}
	}
	if Derived(nil, f) != nil {
		t.Error("Derived(nil) reports")
	}
}

// TestDerivedTrustsNoFact is T15: facts that name a task, an index or a gate
// the project lacks give no diagnostic and no panic.
func TestDerivedTrustsNoFact(t *testing.T) {
	p := entry(t, "unmet-requirement")
	ghost, nowhere := Junction{Task: "zzzz", Gate: "design"}, Junction{Task: "b2c9", Gate: "nowhere"}
	agent := Resolved{Contributor: "bot@example.org", Model: "claude-fable", Reviewer: "olive@example.org"}
	f := &Facts{
		Requirements: []Condition{
			{Task: "zzzz", Index: 0, Due: true},
			{Task: "c3d7", Index: -1, Due: true},
			{Task: "c3d7", Index: 7, Due: true, MetAtTip: true},
			{Task: "e4a1", Index: 0, Due: true},
		},
		Junctions: map[Junction]Resolved{ghost: agent, nowhere: agent, {Task: "", Gate: ""}: agent},
		History: &History{
			Trunk: "main",
			Commits: []Commit{
				{Hash: "X2", Author: "bot@example.org", At: []Junction{ghost, nowhere, {}}, Events: []Event{{Task: "zzzz", Kind: "status", Gate: "nowhere"}, {}}},
				{Hash: "X1", Author: "pat@example.org", Trailers: []Trailer{{Key: "Model", Value: "gpt"}, {Key: "", Value: ""}, {Key: "Other", Value: "zzzz"}}, At: []Junction{ghost, nowhere}},
			},
			OffTrunk: []Pin{
				{Task: "zzzz", Gate: "design", Index: -1},
				{Task: "c3d7", Gate: "nowhere", Index: -1},
				{Task: "c3d7", Gate: "design", Index: -1},
				{Task: "c3d7", Index: 0},
				{Task: "c3d7", Index: 9},
				{Task: "c3d7", Index: -1},
			},
			Reviews: map[Junction]string{ghost: "X1", nowhere: ""},
		},
	}
	if got := derived(t, p, f); len(got) != 0 {
		t.Errorf("facts that name nothing: %v", got)
	}
	// Facts with no junctions and a history with nothing in it.
	if got := derived(t, p, &Facts{History: &History{Trunk: "main"}}); len(got) != 0 {
		t.Errorf("empty facts: %v", got)
	}
}
