package validate

import (
	"math/rand"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// faulty returns a project with a fault of most kinds: schema leaves, tree
// and requirement rules, junction and status rules, and warnings.
func faulty(t *testing.T) *model.Project {
	t.Helper()
	const child = "title: Child\ndescription: A child.\nassignee: pat@example.org\n"
	return project(t, with(base(),
		"gates.yaml", baseGates+"zeta: 1\nalpha: 2\n",
		"tasks/b2c9.yaml", "requires: [{ id: \"c3d7\" }, { text: Nothing }, { id: 0c99, from: Late }]\njunctions: { design: { model: claude-fable, colour: red }, nonesuch: {} }\n"+baseLeaf,
		"tasks/c3d7.yaml", child+"requires: [{ id: \"b2c9\", to: undefined }]\nparent: { id: \"e4a1\", order: 1 }\n",
		"tasks/d4e8.yaml", child+"parent: { id: \"e4a1\", order: 2 }\n",
		"tasks/f6a0.yaml", child+"parent: { id: e4a1, order: 2 }\n",
		"tasks/leaf-two.yaml", child+"parent: { id: \"0c99\" }\n",
		"status/b2c9.yaml", "gate: release\nstate: nominal\nreason: nonesuch\n",
		"status/e4a1.yaml", "gate: Design\n",
		"status/0c99.yaml", "gate: defined\n",
	))
}

// TestSort is T16, first part: Sort gives the envelope's order whatever the
// order it takes: by file, a diagnostic with none first, then line, column,
// code, task, gate, commit and message.
func TestSort(t *testing.T) {
	at := func(file string, line, col int) model.Pos { return model.Pos{File: file, Line: line, Col: col} }
	want := []model.Diagnostic{
		{Code: "H1", Commit: "a", Trailer: "Authorised: zzzz", Message: "m"},
		{Code: "H1", Commit: "a", Trailer: "Reviewed: zzzz design", Message: "m"},
		{Code: "H1", Commit: "b", Message: "a"},
		{Code: "H2", Task: "b2c9", Gate: "design", Commit: "a"},
		{Code: "H2", Task: "b2c9", Gate: "release", Commit: "a"},
		{Code: "H2", Task: "c3d7", Gate: "design", Commit: "a"},
		{Code: "T8"},
		{Code: "G1", Pos: at("gates.yaml", 0, 0)},
		{Code: "G11", Pos: at("gates.yaml", 1, 1), Message: "alpha"},
		{Code: "G11", Pos: at("gates.yaml", 1, 1), Message: "zeta"},
		{Code: "G5", Pos: at("gates.yaml", 1, 1)},
		{Code: "L1", Pos: at("status/b2c9.yaml", 3, 0)},
		{Code: "S5", Pos: at("status/b2c9.yaml", 3, 1)},
		{Code: "S3", Pos: at("status/b2c9.yaml", 10, 1)},
		{Code: "J12", Pos: at("tasks/b2c9.yaml", 0, 0)},
		{Code: "R1", Pos: at("tasks/b2c9.yaml", 8, 11)},
		{Code: "T6", Pos: at("tasks/b2c9.yaml", 8, 11)},
		{Code: "T6", Pos: at("tasks/b2c9.yaml", 8, 12)},
		{Code: "P3", Pos: at("version.yaml", 1, 1)},
	}
	random := rand.New(rand.NewSource(1))
	for i := 0; i < 50; i++ {
		got := append([]model.Diagnostic{}, want...)
		random.Shuffle(len(got), func(i, j int) { got[i], got[j] = got[j], got[i] })
		Sort(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Sort gives\n%s\nwant\n%s", strings.Join(places(got), "\n"), strings.Join(places(want), "\n"))
		}
	}
	Sort(nil)
}

// TestOnce is T16, second part: the two leaves of one oneOf give one R8, two
// orders that children share under one parent give two T12, and Files returns
// its diagnostics in the order of Sort, each one once.
func TestOnce(t *testing.T) {
	const child = "title: Child\ndescription: A child.\nassignee: pat@example.org\n"
	files := with(base(), "tasks/b2c9.yaml", "requires: [{ text: Nothing }]\n"+baseLeaf)
	for id, order := range map[string]string{"c3d7": "1", "d4e8": "2", "f6a0": "2", "0b1c": "3", "a7e9": "2"} {
		files["tasks/"+id+".yaml"] = child + "parent: { id: \"e4a1\", order: " + order + " }\n"
	}
	found := Files(project(t, files))
	got := make([]string, len(found))
	for i, d := range found {
		got[i] = where(d) + ": " + d.Message
	}
	want := []string{
		"tasks/b2c9.yaml:1:12: error: R8 task=b2c9: requires.0 states neither id nor subproject",
		"tasks/e4a1.yaml: warning: T12 task=e4a1: a7e9 and d4e8 and f6a0 share order 2 under e4a1",
		"tasks/e4a1.yaml: warning: T12 task=e4a1: b2c9 and c3d7 share order 1 under e4a1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	all := Files(faulty(t))
	again := append([]model.Diagnostic{}, all...)
	Sort(again)
	if !reflect.DeepEqual(all, again) {
		t.Errorf("Files returns its diagnostics out of the order of Sort:\n%s", strings.Join(places(all), "\n"))
	}
	for i := 1; i < len(all); i++ {
		if all[i] == all[i-1] {
			t.Errorf("Files returns %s twice", where(all[i]))
		}
	}
	if len(all) < 15 {
		t.Errorf("the faulty project gives %d diagnostics:\n%s", len(all), strings.Join(places(all), "\n"))
	}
}

// TestPureAndShared is T17: two runs give equal slices, eight goroutines
// share one project, and the package depends on neither the Loader nor Git
// nor a process.
func TestPureAndShared(t *testing.T) {
	p := faulty(t)
	f := &Facts{
		Junctions:    defaults(p),
		Requirements: []Condition{{Task: "c3d7", Index: 0, Origin: "b2c9", From: "release", To: "defined", Stands: "undefined", Due: true}},
		History: &History{Commits: []Commit{{
			Hash: "C1", Author: "pat@example.org",
			Trailers: []Trailer{{Key: "Reviewed", Value: "b2c9 nowhere"}, {Key: "Authorised", Value: "zzzz"}},
			Events:   []Event{{Task: "d4e8", Kind: "task"}},
		}}},
	}
	f.Junctions[Junction{Task: "d4e8", Gate: "defined"}] = Resolved{Contributor: "pat@example.org", Reviewer: "olive@example.org"}
	files, facts := Files(p), Derived(p, f)
	if len(files) == 0 || codes(facts) != "H1 H1 R9 H4 P5" {
		t.Fatalf("the project gives %d diagnostics from its files and %s from its facts", len(files), codes(facts))
	}
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			for j := 0; j < 10; j++ {
				if got := Files(p); !reflect.DeepEqual(got, files) {
					t.Errorf("Files differs between runs: %v", places(got))
				}
				if got := Derived(p, f); !reflect.DeepEqual(got, facts) {
					t.Errorf("Derived differs between runs: %v", places(got))
				}
			}
		}()
	}
	group.Wait()

	exe, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the host has no go command to list the dependencies")
	}
	out, err := exec.Command(exe, "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if dep == "os/exec" || strings.HasSuffix(dep, "/internal/git") || strings.HasSuffix(dep, "/internal/load") {
			t.Errorf("the package depends on %s", dep)
		}
	}
}

// TestOwnPlan is T18: tablo's own plan, in the working tree, is clean.
func TestOwnPlan(t *testing.T) {
	p := worktree(t, filepath.Join("..", ".."))
	if !p.Exists || len(p.Tasks) < 12 {
		t.Fatalf("the working tree holds no plan of tablo: %d tasks", len(p.Tasks))
	}
	if got := Files(p); len(got) != 0 {
		t.Errorf("tablo's own plan:\n%s", strings.Join(places(got), "\n"))
	}
}
