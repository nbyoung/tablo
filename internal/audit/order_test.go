package audit

import (
	"fmt"
	"math/rand"
	"reflect"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// T6: the order. A shuffled slice of literal findings sorts to the order of
// the design, a demoted finding among the information, and the sort keeps
// the order given among findings that agree in every key.
func TestOrder(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/e4a1.yaml": "title: Root\nassignee: olive@example.org\n",
		"tasks/b2c9.yaml": "title: First\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 1 }\n",
		"tasks/a100.yaml": "title: Second\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 2 }\n",
	})
	old, young := &history.Commit{ID: "old", Seq: 9}, &history.Commit{ID: "young", Seq: 2}
	at := func(file string, line, col int, trailer string) *model.Diagnostic {
		return &model.Diagnostic{Pos: model.Pos{File: file, Line: line, Col: col}, Trailer: trailer}
	}
	// The findings in the order of the design; Sentence numbers them.
	want := []Finding{
		// Errors first; among them the kinds in the order of validate.Rules.
		{Key: "P3", Severity: model.Error},
		{Key: "T2", Severity: model.Error, Task: "a100"},
		{Key: "S11", Severity: model.Error, Task: "b2c9", Gate: "design"},
		// A code the table lacks sorts after the rules, by bytes.
		{Key: "X1", Severity: model.Error},
		{Key: "X2", Severity: model.Error},
		// Warnings. The gate: none, then gates.yaml's order, then any other key by bytes.
		{Key: "H3", Severity: model.Warning, Task: "b2c9"},
		{Key: "H3", Severity: model.Warning, Task: "b2c9", Gate: "defined"},
		{Key: "H3", Severity: model.Warning, Task: "b2c9", Gate: "release"},
		{Key: "H3", Severity: model.Warning, Task: "b2c9", Gate: "Design"},
		{Key: "H3", Severity: model.Warning, Task: "b2c9", Gate: "nowhere"},
		// The task: none, then display order, then any other id by bytes.
		{Key: "H6", Severity: model.Warning, Gate: "design"},
		{Key: "H6", Severity: model.Warning, Task: "e4a1", Gate: "design"},
		{Key: "H6", Severity: model.Warning, Task: "b2c9", Gate: "design"},
		{Key: "H6", Severity: model.Warning, Task: "a100", Gate: "design"},
		{Key: "H6", Severity: model.Warning, Task: "leaf-two", Gate: "design"},
		{Key: "H6", Severity: model.Warning, Task: "zzzz", Gate: "design"},
		// The kinds no rule states follow every rule.
		{Key: Proposed, Severity: model.Warning, Task: "a100"},
		{Key: Stale, Severity: model.Warning, Task: "b2c9", Gate: "defined"},
		// Information. The oldest commit: none, then the oldest first.
		{Key: "H1", Severity: model.Information, Demoted: true},
		{Key: "H1", Severity: model.Information, Demoted: true, Commits: []*history.Commit{old, young}},
		{Key: "H1", Severity: model.Information, Demoted: true, Commits: []*history.Commit{young}},
		// The diagnostic's file, line and column; the message; the trailer.
		{Key: "H4", Severity: model.Information, Task: "b2c9", Gate: "design", Diagnostic: at("status/a100.yaml", 9, 9, "")},
		{Key: "H4", Severity: model.Information, Task: "b2c9", Gate: "design", Diagnostic: at("status/b2c9.yaml", 1, 7, "")},
		{Key: "H4", Severity: model.Information, Task: "b2c9", Gate: "design", Diagnostic: at("status/b2c9.yaml", 2, 1, "")},
		{Key: "H4", Severity: model.Information, Task: "b2c9", Gate: "design", Diagnostic: at("status/b2c9.yaml", 2, 3, ""), Message: "a"},
		{Key: "H4", Severity: model.Information, Task: "b2c9", Gate: "design", Diagnostic: at("status/b2c9.yaml", 2, 3, ""), Message: "b"},
		{Key: "H4", Severity: model.Information, Task: "b2c9", Gate: "design", Diagnostic: at("status/b2c9.yaml", 2, 3, "Reviewed: a"), Message: "b"},
		{Key: "H4", Severity: model.Information, Task: "b2c9", Gate: "design", Diagnostic: at("status/b2c9.yaml", 2, 3, "Reviewed: b"), Message: "b"},
		{Key: SoleReview, Severity: model.Information, Task: "b2c9", Gate: "mockup"},
		{Key: SoleReview, Severity: model.Information, Task: "a100", Gate: "mockup"},
		{Key: SoleReview, Severity: model.Information, Task: "b2c9", Gate: "design"},
	}
	for i := range want {
		want[i].Sentence = fmt.Sprint(i)
	}
	numbers := func(list []Finding) string {
		var out []string
		for _, x := range list {
			out = append(out, x.Sentence)
		}
		return fmt.Sprint(out)
	}
	for seed := int64(1); seed <= 20; seed++ {
		got := append([]Finding(nil), want...)
		rand.New(rand.NewSource(seed)).Shuffle(len(got), func(i, j int) { got[i], got[j] = got[j], got[i] })
		arrange(f, got)
		if numbers(got) != numbers(want) {
			t.Fatalf("the shuffle of seed %d sorts to %s", seed, numbers(got))
		}
	}
	// The sort is stable: findings that agree in every key keep their order.
	same := []Finding{{Key: "H6", Severity: model.Warning, Sentence: "first"}, {Key: "P3", Severity: model.Error}, {Key: "H6", Severity: model.Warning, Sentence: "second"}}
	arrange(f, same)
	if same[0].Key != "P3" || same[1].Sentence != "first" || same[2].Sentence != "second" {
		t.Errorf("the sort is not stable: %+v", same)
	}
}

// T6: two runs give deep-equal reports, in the order of the design.
func TestTwoRunsAgree(t *testing.T) {
	for _, name := range []string{"weather-station", "junction-kinds", "unknown-trailer"} {
		var e *entry
		for _, each := range entries(t) {
			if each.name == name {
				e = each
			}
		}
		if e == nil {
			t.Fatalf("the corpus builds no %s", name)
		}
		f := e.at(t, e.refs[0])
		in := Input{Facts: f, Diagnostics: diagnostics(f, e.want, e.labels), Now: "2026-10-06", Stale: 3}
		first, second := audited(t, in), audited(t, in)
		if !reflect.DeepEqual(first, second) {
			t.Errorf("%s: two runs differ", name)
		}
		// A second derivation of the same source gives the same findings, the facts aside.
		again := e.at(t, e.refs[0])
		third := audited(t, Input{Facts: again, Diagnostics: diagnostics(again, e.want, e.labels), Now: "2026-10-06", Stale: 3})
		if lines(first) != lines(third) {
			t.Errorf("%s: two derivations differ:\n%s\n%s", name, lines(first), lines(third))
		}
		sorted := append([]Finding(nil), first.Findings...)
		arrange(f, sorted)
		if !reflect.DeepEqual(sorted, first.Findings) {
			t.Errorf("%s: the report is not in the order of the design", name)
		}
		for i := 1; i < len(first.Findings); i++ {
			if first.Findings[i-1].Severity > first.Findings[i].Severity {
				t.Errorf("%s: the finding %d is graver than the one before it", name, i)
			}
		}
	}
}

// lines writes the findings of a report, one per line, without their facts.
func lines(rep *Report) string {
	out := ""
	for i := range rep.Findings {
		x := &rep.Findings[i]
		out += fmt.Sprintln(x.Key, x.Severity, x.Demoted, x.Task, x.Gate, x.File, x.Act, x.Resolver, x.Message, ids(x), texts(x))
	}
	return out
}

// bySeverity counts the findings of a report by listed severity.
func bySeverity(rep *Report) [3]int {
	var n [3]int
	for _, x := range rep.Findings {
		n[x.Severity]++
	}
	return n
}

// T13: the demotion (decision 3). A finding nothing resolves lists as
// information once the task passes its junction, while its diagnostic keeps
// the rule's own severity.
func TestDemotionByJunction(t *testing.T) {
	r := newRepository(t, "{ design: { contributor: pat@example.org, model: claude-fable, reviewer: olive@example.org } }")
	r.commit("pat", "2026-09-02", "Record the definition", map[string]string{".tableaux/status/b2c9.yaml": "gate: defined\nstate: nominal\n"}, "Model: claude-fable-5")
	wrong := r.commit("ben", "2026-09-03", "Accept the design", nil, "Reviewed: b2c9 design")
	accepts := r.commit("olive", "2026-09-04", "Accept the design", nil, "Reviewed: b2c9 design")
	outside := r.commit("pat", "2026-09-05", "Record the design", map[string]string{".tableaux/status/b2c9.yaml": "gate: design\nstate: nominal\n"}, "Model: claude-sonnet-5")
	list := []model.Diagnostic{
		{Code: "H3", Severity: model.Warning, Task: "b2c9", Gate: "design", Commit: outside, Message: "Model claude-sonnet-5 is outside the stated model claude-fable"},
		{Code: "H2", Severity: model.Warning, Task: "b2c9", Gate: "design", Commit: wrong, Message: "ben is not the reviewer of the design junction; olive is"},
		{Code: "H1", Severity: model.Warning, Task: "b2c9", Commit: wrong, Trailer: "Reviewed: b2c9 nowhere", Message: "nowhere names no gate"},
		{Code: "H1", Severity: model.Warning, Task: "b2c9", Gate: "nowhere", Commit: wrong, Trailer: "Reviewed: b2c9 nowhere", Message: "nowhere names no gate, as a literal states it"},
		{Code: "H4", Severity: model.Information, Pos: model.Pos{File: "status/b2c9.yaml"}, Task: "b2c9", Gate: "design", Commit: outside, Message: "the newest event is the contributor's"},
		{Code: "H2", Severity: model.Warning, Task: "b2c9", Gate: "unit", Commit: wrong, Message: "a review no review accepts since"},
	}
	run := func() (*derive.Facts, *Report) {
		f := r.facts()
		return f, audited(t, Input{Facts: f, Diagnostics: append([]model.Diagnostic(nil), list...), Now: "2026-09-06"})
	}
	only := func(rep *Report, key, gate string) *Finding {
		t.Helper()
		var found []*Finding
		for _, x := range of(rep, key) {
			if x.Gate == gate {
				found = append(found, x)
			}
		}
		if len(found) != 1 {
			t.Fatalf("%d findings of %s at %q", len(found), key, gate)
		}
		return found[0]
	}

	// The status names design: the junction is not historical.
	f, rep := run()
	if review := f.Accepted("b2c9", "design"); review == nil || review.Commit == nil || review.Commit.ID != accepts {
		t.Fatalf("the review reads %+v", review)
	}
	h3 := only(rep, "H3", "design")
	if h3.Severity != model.Warning || h3.Demoted || h3.Act != None || h3.Resolver != "pat@example.org" || h3.Facts.Status != nil || h3.Facts.Reading == nil {
		t.Errorf("while the status names design the H3 reads %+v", h3)
	}
	if h3.Facts.Reading.Verdict != derive.Mismatch || h3.Facts.Reading.Gate != "design" {
		t.Errorf("the model check reads %+v", h3.Facts.Reading)
	}
	if h2 := only(rep, "H2", "design"); h2.Act != None || h2.Demoted || h2.Severity != model.Warning {
		t.Errorf("while the status names design the accepted H2 reads %+v", h2)
	}
	if got := bySeverity(rep); got != [3]int{0, 5, 1 + len(of(rep, SoleReview))} {
		t.Errorf("while the status names design the counts are %v", got)
	}

	// The status names the next gate: design is historical.
	r.commit("pat", "2026-09-06", "Record the unit tests", map[string]string{".tableaux/status/b2c9.yaml": "gate: unit\nstate: nominal\n"})
	f, rep = run()
	h3 = only(rep, "H3", "design")
	if h3.Severity != model.Information || !h3.Demoted || h3.Facts.Status != f.Status("b2c9") || h3.Facts.Status.Gate != "unit" {
		t.Errorf("once the status names unit the H3 reads %+v", h3)
	}
	if h3.Diagnostic.Severity != model.Warning || h3.Act != None || len(h3.Commits) != 1 || h3.Commits[0].ID != outside {
		t.Errorf("the demoted H3 carries %+v, %v", h3.Diagnostic, ids(h3))
	}
	if h2 := only(rep, "H2", "design"); h2.Severity != model.Information || !h2.Demoted || h2.Act != None || h2.Diagnostic.Severity != model.Warning || h2.Facts.Status != f.Status("b2c9") {
		t.Errorf("once the status names unit the accepted H2 reads %+v", h2)
	}
	// A review no review accepts asks an act: it never demotes. Nor does the
	// junction at the gate the status names make one historical.
	if h2 := only(rep, "H2", "unit"); h2.Act != Review || h2.Demoted || h2.Severity != model.Warning {
		t.Errorf("the H2 no review accepts reads %+v", h2)
	}
	// An H1 that names a task and no gate of the project waits for the task to complete.
	for _, gate := range []string{"", "nowhere"} {
		if h1 := only(rep, "H1", gate); h1.Demoted || h1.Severity != model.Warning || h1.Facts.Status != nil {
			t.Errorf("before the task completes the H1 at %q reads %+v", gate, h1)
		}
	}
	if h4 := only(rep, "H4", "design"); h4.Demoted || h4.Severity != model.Information || h4.Act != RecordHandoff {
		t.Errorf("the H4 reads %+v", h4)
	}
	if got := bySeverity(rep); got != [3]int{0, 3, 3 + len(of(rep, SoleReview))} {
		t.Errorf("once the status names unit the counts are %v", got)
	}
	// The demoted findings sort among the information, after every warning.
	for i, x := range rep.Findings {
		if x.Demoted && (i < 3 || x.Severity != model.Information) {
			t.Errorf("the demoted %s stands at %d", x.Key, i)
		}
	}

	// A complete status: every junction is historical, and the H1 demotes.
	r.commit("pat", "2026-09-06", "Release", map[string]string{".tableaux/status/b2c9.yaml": "gate: release\nstate: complete\n"})
	f, rep = run()
	for _, x := range []*Finding{only(rep, "H3", "design"), only(rep, "H2", "design"), only(rep, "H1", ""), only(rep, "H1", "nowhere")} {
		if x.Severity != model.Information || !x.Demoted || x.Diagnostic.Severity != model.Warning || x.Facts.Status != f.Status("b2c9") {
			t.Errorf("at a complete status the %s reads %+v", x.Key, x)
		}
	}
	if h2 := only(rep, "H2", "unit"); h2.Act != Review || h2.Demoted {
		t.Errorf("at a complete status the H2 no review accepts reads %+v", h2)
	}
	if h4 := only(rep, "H4", "design"); h4.Demoted || h4.Severity != model.Information {
		t.Errorf("at a complete status the H4 reads %+v", h4)
	}
	if got := bySeverity(rep); got != [3]int{0, 1, 5} || len(of(rep, SoleReview)) != 0 {
		t.Errorf("at a complete status the counts are %v with %d sole reviews", got, len(of(rep, SoleReview)))
	}
	for _, d := range list {
		if d.Code != "H4" && d.Severity != model.Warning {
			t.Errorf("the run changes the diagnostic %+v", d)
		}
	}
}

// T13: a finding that names no task of the project demotes by the age of
// its commit (decisions 3 and 9): unknown-trailer's H1 findings stand at
// commit U2 of 2026-09-02.
func TestDemotionByAge(t *testing.T) {
	var e *entry
	for _, each := range entries(t) {
		if each.name == "unknown-trailer" {
			e = each
		}
	}
	if e == nil {
		t.Fatal("the corpus builds no unknown-trailer")
	}
	f := derived(t, load.Options{}, e.dir, "main", "")
	list := diagnostics(f, e.want, e.labels)
	u2 := f.Log().Commit(e.labels["U2"])
	if u2 == nil || u2.Date() != "2026-09-02" {
		t.Fatalf("U2 reads %+v", u2)
	}
	for _, c := range []struct {
		now     string
		stale   int
		demoted bool
	}{
		{"2026-09-03", 0, false},
		{"2026-09-09", 7, false}, // seven days: within the threshold
		{"2026-09-10", 7, true},  // eight: beyond it
		{"2026-09-10", 8, false},
		{"2026-09-06", 3, true},
		{"2026-09-01", 1, false}, // a date before the commit's
	} {
		rep := audited(t, Input{Facts: f, Diagnostics: append([]model.Diagnostic(nil), list...), Now: c.now, Stale: c.stale})
		found := of(rep, "H1")
		if len(found) != 2 {
			t.Fatalf("unknown-trailer gives %d H1 findings", len(found))
		}
		for _, x := range found {
			want := model.Warning
			if c.demoted {
				want = model.Information
			}
			if x.Task != "" || x.Demoted != c.demoted || x.Severity != want || x.Diagnostic.Severity != model.Warning || x.Facts.Status != nil ||
				x.Act != None || len(x.Commits) != 1 || x.Commits[0] != u2 {
				t.Errorf("at %s with the age %d the H1 reads %+v", c.now, c.stale, x)
			}
		}
	}
	// With no such commit in the pass the finding has no date to age by.
	lost := []model.Diagnostic{
		{Code: "H1", Severity: model.Warning, Commit: "0000000000000000000000000000000000000000", Trailer: "Authorised: zzzz", Message: "zzzz names no task"},
		{Code: "H1", Severity: model.Warning, Trailer: "Authorised: zzzz", Message: "zzzz names no task"},
		{Code: "H3", Severity: model.Warning, Task: "zzzz", Gate: "design", Commit: e.labels["U2"], Message: "a task the project lacks ages by its commit"},
	}
	rep := audited(t, Input{Facts: f, Diagnostics: lost, Now: "2030-01-01"})
	for _, x := range of(rep, "H1") {
		if x.Demoted || x.Severity != model.Warning || x.Resolver != f.Owner() {
			t.Errorf("an H1 with no commit in the pass reads %+v", x)
		}
	}
	if x := of(rep, "H3")[0]; !x.Demoted || x.Severity != model.Information || x.Facts.Status != nil || x.Facts.Junction != nil {
		t.Errorf("an H3 of no task of the project reads %+v", x)
	}
}
