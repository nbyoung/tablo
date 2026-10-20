package derive

import (
	"fmt"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// The facts of language 0.4.0 (design cdb0).

// TestTaskLevelField is T11: the task-level field makes c100 recursive at
// every gate after defined where its file states no entry, Delegated and
// sourced from c100, lifting the root's exemption at performance; its own
// entries replace it at reliability and release. One snapshot reads the
// subproject, and each move of the field's path is a pin event. On a parent
// the field makes the parent's junctions recursive and the child's plain.
func TestTaskLevelField(t *testing.T) {
	f, hashes := view(t, "subproject-field")
	var kinds []string
	for _, j := range f.Junctions("c100") {
		word := j.Kind.String()
		if j.Delegated {
			word += " delegated"
			if j.Entry != "c100" || fmt.Sprint(j.Sources()) != "[c100]" || j.Snapshot == nil {
				t.Errorf("c100 %s: entry %s, sources %v, snapshot %v", j.Gate, j.Entry, j.Sources(), j.Snapshot)
			}
		}
		kinds = append(kinds, j.Gate+" "+word)
	}
	want := "defined plain, mockup recursive delegated, function recursive delegated, performance recursive delegated, " +
		"reliability not_applicable, design recursive delegated, implementation recursive delegated, unit recursive delegated, " +
		"integrate recursive delegated, validate recursive delegated, release plain"
	if got := strings.Join(kinds, ", "); got != want {
		t.Errorf("c100's junctions:\n got  %s\n want %s", got, want)
	}
	snapshots := f.Snapshots("c100")
	if len(snapshots) != 1 || snapshots[0].Entry != f.Project().Tasks["c100"].Subproject || snapshots[0].Target != "5a00" || len(snapshots[0].Gates) != 8 {
		t.Errorf("c100's snapshots: %+v", snapshots)
	}
	var pins []string
	for _, e := range f.Events("c100") {
		if e.Kind == Pinned {
			pins = append(pins, e.Commit.ID+" "+e.URL)
		}
	}
	if got, want := strings.Join(pins, ", "), hashes["U2"]+" sub, "+hashes["U3"]+" sub, "+hashes["U4"]+" sub"; got != want {
		t.Errorf("c100's pin events: %s; want U2, U3 and U4 at sub", got)
	}

	parent, _ := view(t, "subproject-field-on-parent")
	for _, j := range parent.Junctions("e4a1") {
		if j.Gate != "defined" && (j.Kind != model.Recursive || !j.Delegated) {
			t.Errorf("the parent's %s junction is %s, delegated %v; want recursive by the field", j.Gate, j.Kind, j.Delegated)
		}
	}
	for _, j := range parent.Junctions("b2c9") {
		if j.Kind != model.Plain {
			t.Errorf("the child's %s junction is %s; want plain, since the field names one task's work", j.Gate, j.Kind)
		}
	}
}
