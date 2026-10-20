package views

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// The views of language 0.4.0 (design cdb0, T14).

// TestDelegatedSupplier: a junction the task-level field makes recursive
// states its subproject supplier at the task's key subproject, which points
// at the field in the task's file.
func TestDelegatedSupplier(t *testing.T) {
	dir := builtAt(t, "subproject-field")
	f := derived(t, load.Options{}, dir, "main", "")
	b := builder{f: f}
	file, err := os.ReadFile(filepath.Join(dir, ".tableaux", "tasks", "c100.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	delegated := 0
	for _, j := range f.Junctions("c100") {
		if !j.Delegated {
			continue
		}
		delegated++
		rows := b.suppliers(j)
		if len(rows) != 1 {
			t.Errorf("%s: %d suppliers; want the field alone", j.Gate, len(rows))
			continue
		}
		s := rows[0]
		if s.Field != "subproject" || s.Value != "sub" || s.By != "c100" || s.Key != "subproject" || s.File != ".tableaux/tasks/c100.yaml" {
			t.Errorf("%s: the supplier reads %+v", j.Gate, s)
		}
		if got := textAt(t, string(file), s.Line, s.Col); !strings.HasPrefix(got, "subproject:") {
			t.Errorf("%s: the supplier points at %q", j.Gate, got)
		}
	}
	if delegated != 8 {
		t.Errorf("c100 delegates %d gates; want 8", delegated)
	}
}

// TestNearestPerson: on review-nearest-person the default supplies b2c9's
// reviewer, the nearest person, whom the view states as e4a1's assignee, and
// a brief's source names that ancestor with the kind nearest, which the
// schema accepts and a word outside the set fails.
func TestNearestPerson(t *testing.T) {
	dir := builtAt(t, "review-nearest-person")
	f := derived(t, load.Options{}, dir, "main", "")
	b := builder{f: f}
	j := f.Junction("b2c9", "design")
	if out := b.junction(j, false); !out.ReviewerByDefault || out.Reviewer != "olive@example.org" {
		t.Errorf("b2c9's design: the reviewer %s by default %v", out.Reviewer, out.ReviewerByDefault)
	}
	var reviewer *view.Supplier
	for _, s := range b.suppliers(j) {
		if s.Field == "reviewer" {
			reviewer = &s
		}
	}
	root, err := os.ReadFile(filepath.Join(dir, ".tableaux", "tasks", "e4a1.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if reviewer == nil || reviewer.By != "e4a1" || reviewer.Key != "assignee" || reviewer.File != ".tableaux/tasks/e4a1.yaml" ||
		!strings.HasPrefix(textAt(t, string(root), reviewer.Line, reviewer.Col), "olive@example.org") {
		t.Errorf("b2c9's design: the reviewer's supplier is %+v", reviewer)
	}
	q := Query{Task: "e4a1", Person: "bot@example.org", Level: Provenance, Brief: &BriefOf{Task: "b2c9", Gate: "release"}}
	queue := Queue(f, q)
	if queue.Brief == nil {
		t.Fatal("bot's queue holds no brief of b2c9 at release")
	}
	if r := queue.Brief.Junction.Reviewer; r.Value != "olive@example.org" || r.Source.Kind != "nearest" || r.Source.ID != "e4a1" || r.Source.Title == "" {
		t.Errorf("the brief's reviewer is %+v", r)
	}
	s := envelopeSchema(t)
	data := encoded(t, queue)
	if err := s.Validate(envelope(t, "queue", q, data)); err != nil {
		t.Errorf("the brief does not validate: %v", err)
	}
	if err := s.Validate(envelope(t, "queue", q, bytes.Replace(data, []byte(`"kind": "nearest"`), []byte(`"kind": "nobody"`), 1))); err == nil {
		t.Error("the schema accepts the source kind nobody")
	}
}

// TestCoAuthor: on the crossing's X3 and X5, pat's commits that name bot in
// Co-Authored-By:, the history's lines at detail and their model checks, and
// the task view's model checks, carry the co-author; every other line
// carries "". The views validate against the envelope's schema.
func TestCoAuthor(t *testing.T) {
	repo, hashes, _ := scripted(t, "crossing")
	f := derived(t, load.Options{}, repo, hashes["X6"], "")
	named := map[string]bool{hashes["X3"]: true, hashes["X5"]: true}
	s := envelopeSchema(t)
	q := Query{Task: "e4a1", Level: Detail}
	h := History(f, q)
	met := 0
	for _, l := range h.Lines {
		commit := l.Events[0].Commit
		want := ""
		if named[commit] {
			want = "bot@example.org"
		}
		if l.CoAuthor != want {
			t.Errorf("the line of %s at %s carries the co-author %q; want %q", l.Task.ID, commit, l.CoAuthor, want)
		}
		if l.Check != nil && l.Check.CoAuthor != want {
			t.Errorf("the check of %s at %s carries the co-author %q; want %q", l.Task.ID, commit, l.Check.CoAuthor, want)
		}
		if named[commit] && l.Check != nil {
			met++
		}
	}
	if met != 2 {
		t.Errorf("%d checks of X3 and X5; want 2", met)
	}
	if err := s.Validate(envelope(t, "history", q, encoded(t, h))); err != nil {
		t.Errorf("the history does not validate: %v", err)
	}
	for _, id := range []string{"b2c9", "d3a6"} {
		q := Query{Task: id, Level: Provenance}
		task := Task(f, q)
		for _, m := range task.Models {
			if want := map[bool]string{true: "bot@example.org"}[named[m.Commit]]; m.CoAuthor != want {
				t.Errorf("%s: the model check of %s carries %q; want %q", id, m.Commit, m.CoAuthor, want)
			}
		}
		if err := s.Validate(envelope(t, "task", q, encoded(t, task))); err != nil {
			t.Errorf("%s: the task view does not validate: %v", id, err)
		}
	}
}

// TestThirdCommand: where the trunk is determined, the history's provenance
// holds the third command of README.md#history, the merges of the trunk's
// first-parent line that bring in a task file; with no trunk it holds two.
func TestThirdCommand(t *testing.T) {
	merge := derived(t, load.Options{}, builtAt(t, "authorised-by-merge"), "main", "")
	commands := History(merge, Query{Level: Provenance}).Commands
	if len(commands) != 3 || commands[2].Text != "git log --format='%H %as %ae %s' --first-parent --merges main -- .tableaux/tasks" {
		t.Errorf("authorised-by-merge: the commands are %+v", commands)
	}
	if c := History(merge, Query{Task: "c3d4", Level: Provenance}).Commands; len(c) != 3 ||
		c[2].Text != "git log --format='%H %as %ae %s' --first-parent --merges main -- .tableaux/tasks/c3d4.yaml" {
		t.Errorf("authorised-by-merge c3d4: the commands are %+v", c)
	}
	none := derived(t, load.Options{}, builtAt(t, "trunk-undetermined"), "main", "")
	if c := History(none, Query{Level: Provenance}).Commands; len(c) != 2 {
		t.Errorf("trunk-undetermined: the commands are %+v; want two", c)
	}
}
