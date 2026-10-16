package views

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nbyoung/tablo/internal/audit"
	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/internal/validate"
)

// validated returns what the Validator reports of a project without its
// history: validate.Files, and validate.Derived with the determined
// requirements alone, for R9. No adapter fills validate.Facts from the
// Derivation before the Plumbing command (4ed9), so the test fills the one
// part the weather station needs, as the Audit's own tests do.
func validated(f *derive.Facts) []model.Diagnostic {
	p := f.Project()
	out := validate.Files(p)
	facts := &validate.Facts{}
	for _, id := range f.Order() {
		for i, c := range f.Requires(id) {
			if c.Why != derive.Determined {
				continue
			}
			facts.Requirements = append(facts.Requirements, validate.Condition{Task: id, Index: i, Origin: c.Origin,
				From: c.From, To: c.To, Stands: c.Stands, Met: c.Met, Due: c.Due, MetAtTip: c.AtTrunk == derive.Yes})
		}
	}
	out = append(out, validate.Derived(p, facts)...)
	validate.Sort(out)
	return out
}

// report runs the Audit over the weather station at main, on the date of
// dada's golden data, with the default age.
func report(t testing.TB) (*derive.Facts, *audit.Report) {
	t.Helper()
	f, _ := station(t, "main")
	r, err := audit.Run(audit.Input{Facts: f, Diagnostics: validated(f), Now: "2026-09-28"})
	if err != nil || r == nil {
		t.Fatalf("Run: %v, %v", r, err)
	}
	return f, r
}

// T15: the audit's selection. The root at provenance is dada's golden data
// as a JSON value; a task, a person and the minimum select the findings, and
// the counts cover every finding kept whatever the minimum.
func TestAuditSelection(t *testing.T) {
	f, r := report(t)
	want, err := os.ReadFile(filepath.Join("..", "..", "view", "testdata", "audit.provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var wantValue, gotValue any
	if err := json.Unmarshal(want, &wantValue); err != nil {
		t.Fatal(err)
	}
	got := encoded(t, Audit(f, r, Query{Level: Provenance, Minimum: model.Warning}))
	if err := json.Unmarshal(got, &gotValue); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		out := filepath.Join(t.TempDir(), "audit.provenance.json")
		if err := os.WriteFile(out, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Errorf("the audit of the root differs from dada's golden data; the audit is in %s", out)
	}
	for _, c := range []struct {
		name    string
		q       Query
		counts  [3]int
		rows    int
		tasks   []string // the tasks of the rows, each row's first
		minimum string   // the mildest severity a row lists
	}{
		{"4e2b at warning", Query{Task: "4e2b", Minimum: model.Warning}, [3]int{0, 1, 12}, 1, []string{"c07d"}, "warning"},
		{"ada at information", Query{Person: "ada@example.org", Minimum: model.Information}, [3]int{0, 2, 20}, 12, nil, "information"},
		{"9f31 at information", Query{Task: "9f31", Minimum: model.Information}, [3]int{0, 0, 8}, 8, nil, "information"},
	} {
		a := Audit(f, r, c.q)
		if got := [3]int{a.Errors, a.Warnings, a.Information}; got != c.counts || len(a.Findings) != c.rows {
			t.Errorf("%s: the counts are %v over %d rows; want %v over %d", c.name, got, len(a.Findings), c.counts, c.rows)
		}
		for i, row := range a.Findings {
			if c.tasks != nil && (len(row.Tasks) == 0 || row.Tasks[0].ID != c.tasks[i]) {
				t.Errorf("%s: row %d names %v", c.name, i, row.Tasks)
			}
			if c.q.Person != "" && row.Resolver != c.q.Person {
				t.Errorf("%s: row %d is resolved by %s", c.name, i, row.Resolver)
			}
			if c.q.Task == "9f31" && (len(row.Tasks) != 1 || row.Tasks[0].ID != "9f31") {
				t.Errorf("%s: row %d names %v", c.name, i, row.Tasks)
			}
			if row.FindingDetail != nil || row.FindingProvenance != nil {
				t.Errorf("%s: row %d at glance carries detail or provenance", c.name, i)
			}
		}
		if a.AuditDetail != nil {
			t.Errorf("%s: the audit at glance lists the silent kinds", c.name)
		}
	}
	// The silent kinds arrive from detail on, and the same at provenance.
	detail := Audit(f, r, Query{Level: Detail, Minimum: model.Warning})
	provenance := Audit(f, r, Query{Level: Provenance, Minimum: model.Warning})
	if detail.AuditDetail == nil || len(detail.Silent) == 0 || !reflect.DeepEqual(detail.Silent, provenance.Silent) {
		t.Errorf("the silent kinds at detail are %+v", detail.AuditDetail)
	}
	for _, row := range detail.Findings {
		if row.FindingDetail == nil || row.FindingProvenance != nil {
			t.Errorf("a row at detail carries %+v and %+v", row.FindingDetail, row.FindingProvenance)
		}
	}
	// A person who resolves nothing sees no row and zero counts, and every
	// kind examined is silent: the seventeen of the golden data, R9, proposed
	// and sole_review.
	if a := Audit(f, r, Query{Person: "nobody@example.org", Level: Detail, Minimum: model.Information}); a.Errors+a.Warnings+a.Information != 0 || len(a.Findings) != 0 || a.Most != nil || len(a.Silent) != 20 {
		t.Errorf("a stranger's audit counts %d, %d and %d over %d rows, most %+v, %d silent", a.Errors, a.Warnings, a.Information, len(a.Findings), a.Most, len(a.Silent))
	}
}
