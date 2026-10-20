package views

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
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
