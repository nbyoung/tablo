package views

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// T5: the gate definition for a task.
func TestGatesForTask(t *testing.T) {
	f, _ := station(t, "main")
	board := Gates(f, Query{Task: "9f31", Level: Detail})
	if board.Task == nil || *board.Task != (view.TaskRef{ID: "9f31", Title: "Sensor board"}) || board.GatesProvenance != nil {
		t.Fatalf("the gate definition of 9f31 names %+v, with the provenance %v", board.Task, board.GatesProvenance)
	}
	if len(board.ForTask) != len(f.Gates()) {
		t.Fatalf("for_task holds %d gates of %d", len(board.ForTask), len(f.Gates()))
	}
	for i, row := range board.ForTask {
		want := view.GateFor{Gate: f.Gates()[i], Applies: true, References: []view.Reference{}}
		if row.Gate == "performance" {
			want.References = []view.Reference{{Text: "Power budget", URL: "docs/sensor-board.md#power-budget"}}
			want.StatedBy = "9f31"
		}
		if !reflect.DeepEqual(row, want) {
			t.Errorf("9f31 at %s is %+v; want %+v", f.Gates()[i], row, want)
		}
	}
	firmware := Gates(f, Query{Task: "c07d", Level: Detail})
	for _, row := range firmware.ForTask {
		if want := row.Gate != "reliability"; row.Applies != want {
			t.Errorf("c07d: %s applies %v; want %v", row.Gate, row.Applies, want)
		}
	}
	if first := firmware.ForTask[0]; first.Gate != "undefined" || !first.Applies {
		t.Errorf("the first gate of c07d is %+v; want undefined, which applies", first)
	}
	none := Gates(f, Query{Level: Detail})
	if none.Task != nil || none.ForTask == nil || len(none.ForTask) != 0 {
		t.Errorf("without a task the view names %v with for_task %#v; want null and []", none.Task, none.ForTask)
	}
	glance := Gates(f, Query{Task: "9f31"})
	if glance.GatesDetail != nil || glance.GatesProvenance != nil || glance.Task == nil || len(glance.Legend.Gates) != 12 {
		t.Errorf("the glance holds the detail %v, the provenance %v, the task %v and %d gates", glance.GatesDetail, glance.GatesProvenance, glance.Task, len(glance.Legend.Gates))
	}
}

// T6: the gate definition's provenance: the two files with the newest
// commit that changes each, and the four commands.
func TestGatesProvenance(t *testing.T) {
	f, hashes := station(t, "main")
	g := Gates(f, Query{Level: Provenance})
	if g.GatesDetail == nil || len(g.Files) != 2 || g.Files[0].Path != ".tableaux/gates.yaml" || g.Files[1].Path != ".tableaux/version.yaml" {
		t.Fatalf("the files are %+v", g.Files)
	}
	for _, file := range g.Files {
		if file.Commit == nil || file.Commit.Hash != hashes["W1"] || file.Commit.Subject != "Plan the weather station" {
			t.Errorf("%s last changes at %+v; want W1", file.Path, file.Commit)
		}
	}
	tip := hashes["W13"]
	want := []view.Command{
		{Text: "git show " + tip + ":.tableaux/gates.yaml"},
		{Text: "git show " + tip + ":.tableaux/version.yaml"},
		{Text: "git log -1 --format='%H %as %an <%ae> / %cn <%ce> %s' " + tip + " -- .tableaux/gates.yaml"},
		{Text: "git log -1 --format='%H %as %an <%ae> / %cn <%ce> %s' " + tip + " -- .tableaux/version.yaml"},
	}
	if !reflect.DeepEqual(g.Commands, want) {
		t.Errorf("the commands are %+v; want %+v", g.Commands, want)
	}
	// The commands reproduce the facts: each runs, and the log names W1.
	repo := builtAt(t, "weather-station")
	if got := gitRun(t, repo, "show", tip+":.tableaux/version.yaml"); !strings.Contains(got, "tableaux: 0.3.1") {
		t.Errorf("git show gives %q", got)
	}
	if got := gitRun(t, repo, "log", "-1", "--format=%H", tip, "--", ".tableaux/gates.yaml"); got != hashes["W1"] {
		t.Errorf("git log names %s for gates.yaml; want W1", got)
	}

	// A later commit to gates.yaml moves that file's commit alone.
	copied := clone(t, "weather-station")
	gates := filepath.Join(copied, ".tableaux", "gates.yaml")
	data, err := os.ReadFile(gates)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gates, append(data, []byte("# a later comment\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, copied, "commit", "-q", "-m", "Comment the gates", "--", ".tableaux/gates.yaml")
	head := gitRun(t, copied, "rev-parse", "HEAD")
	later := Gates(derived(t, load.Options{}, copied, "main", ""), Query{Level: Provenance})
	if c := later.Files[0].Commit; c == nil || c.Hash != head || !reflect.DeepEqual(c.Files, []view.File{{Path: ".tableaux/gates.yaml", Change: "changed"}}) {
		t.Errorf("after a commit gates.yaml last changes at %+v; want %s", c, head)
	}
	if c := later.Files[1].Commit; c == nil || c.Hash != hashes["W1"] {
		t.Errorf("after a commit to gates.yaml, version.yaml last changes at %+v; want W1", c)
	}
	if later.Commands[0].Text != "git show "+head+":.tableaux/gates.yaml" {
		t.Errorf("the first command is %q", later.Commands[0].Text)
	}

	// A project in a directory of its repository: the path in the data is
	// relative to the project, the path of a command to the repository.
	lib := Gates(derived(t, load.Options{}, filepath.Join(builtAt(t, "subproject-same-repository"), "lib"), "main", ""), Query{Level: Provenance})
	if lib.Files[0].Path != ".tableaux/gates.yaml" || len(lib.Commands) != 4 || !strings.HasSuffix(lib.Commands[0].Text, ":lib/.tableaux/gates.yaml") ||
		!strings.HasSuffix(lib.Commands[3].Text, " -- lib/.tableaux/version.yaml") {
		t.Errorf("the project in lib names %q and runs %+v", lib.Files[0].Path, lib.Commands)
	}
}
