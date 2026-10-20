package audit

import (
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// T15 of design cdb0: the rows of language 0.4.0. R14 asks the assignee to
// state the commit and J19 to raise the subproject's language. H7 and H8,
// which nothing resolves, settle as information: H7 once its junction is
// historical, H8, which names a task and no gate, once its task is complete.
// A clean project lists the four new rules a view names among the silent.
func TestLanguageRows(t *testing.T) {
	r := newRepository(t, "")
	early := r.commit("pat", "2026-09-02", "Define the leaf", map[string]string{".tableaux/status/b2c9.yaml": "gate: defined\nstate: nominal\n"})
	before := r.facts()
	file := model.Pos{File: "tasks/b2c9.yaml", Line: 1, Col: 1}
	for _, c := range []struct {
		code string
		act  Act
	}{{"R14", StateCommit}, {"J19", RaiseLanguage}} {
		if x := resolved(t, before, model.Diagnostic{Code: c.code, Pos: file, Task: "b2c9"}); x.Act != c.act || x.Resolver != "pat@example.org" {
			t.Errorf("%s: %s by %q; want %s by the assignee", c.code, x.Act, x.Resolver, c.act)
		}
	}
	h7 := model.Diagnostic{Code: "H7", Severity: model.Warning, Task: "b2c9", Gate: "design", Commit: early}
	h8 := model.Diagnostic{Code: "H8", Severity: model.Warning, Task: "b2c9", Commit: early, Trailer: "Reaffirmed: b2c9"}
	severities := func(rep *Report) map[string]string {
		out := map[string]string{}
		for _, x := range rep.Findings {
			out[x.Key] = x.Severity.String()
			if x.Act != None {
				t.Errorf("%s asks %s", x.Key, x.Act)
			}
		}
		return out
	}
	got := severities(audited(t, Input{Facts: before, Diagnostics: []model.Diagnostic{h7, h8}, Now: "2026-09-03"}))
	if got["H7"] != "warning" || got["H8"] != "warning" {
		t.Errorf("before the junction passes: %v", got)
	}
	r.commit("pat", "2026-09-04", "Pass the design", map[string]string{".tableaux/status/b2c9.yaml": "gate: unit\nstate: nominal\n"})
	got = severities(audited(t, Input{Facts: r.facts(), Diagnostics: []model.Diagnostic{h7, h8}, Now: "2026-09-05"}))
	if got["H7"] != "information" || got["H8"] != "warning" {
		t.Errorf("past design, short of complete: %v", got)
	}
	r.commit("pat", "2026-09-06", "Complete the leaf", map[string]string{".tableaux/status/b2c9.yaml": "gate: release\nstate: complete\n"})
	got = severities(audited(t, Input{Facts: r.facts(), Diagnostics: []model.Diagnostic{h7, h8}, Now: "2026-09-07"}))
	if got["H7"] != "information" || got["H8"] != "information" {
		t.Errorf("once complete: %v", got)
	}

	quiet := map[string]bool{}
	for _, s := range audited(t, Input{Facts: before, Now: "2026-09-03"}).Silent() {
		quiet[s.Key] = true
	}
	for _, id := range []string{"R14", "J19", "H7", "H8"} {
		if !quiet[id] {
			t.Errorf("a clean project does not list %s among the silent: %v", id, quiet)
		}
	}
}
