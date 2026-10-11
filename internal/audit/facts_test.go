package audit

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/internal/validate"
)

// ids returns the commits of a finding, in their order.
func ids(x *Finding) []string {
	var list []string
	for _, c := range x.Commits {
		list = append(list, c.ID)
	}
	return list
}

// texts returns the commands of a finding, one per line, each with its comment.
func texts(x *Finding) string {
	var lines []string
	for _, c := range x.Commands {
		lines = append(lines, c.Text+"  # "+c.Comment)
	}
	return strings.Join(lines, "\n")
}

// T7: the acts that take a condition, on a history the corpus has no entry
// for. An H2 asks a review of the reviewer until a review accepts the
// junction, and nothing since.
func TestReviewFromTheWrongHand(t *testing.T) {
	r := newRepository(t, "{ design: { reviewer: olive@example.org } }")
	r.commit("pat", "2026-09-02", "Record the design", map[string]string{".tableaux/status/b2c9.yaml": "gate: defined\nstate: nominal\nreason: review\n"})
	wrong := r.commit("ben", "2026-09-03", "Accept the design", nil, "Reviewed: b2c9 design")
	h2 := model.Diagnostic{Code: "H2", Severity: model.Warning, Task: "b2c9", Gate: "design", Commit: wrong,
		Message: "ben@example.org is not the reviewer of the design junction; olive@example.org is"}

	f := r.facts()
	x := resolved(t, f, h2)
	if x.Act != Review || x.Resolver != "olive@example.org" || fmt.Sprint(ids(x)) != fmt.Sprint([]string{wrong}) {
		t.Errorf("before the reviewer's commit: %s by %q with %v", x.Act, x.Resolver, ids(x))
	}
	if want := "git show --no-patch --format='%as %H %ae %ce%n%(trailers)' " + wrong + "  # shows the finding\n" +
		"git commit --allow-empty --trailer 'Reviewed: b2c9 design'  # resolves it"; texts(x) != want {
		t.Errorf("before the reviewer's commit the commands are\n%s", texts(x))
	}
	if x.Facts.Junction != f.Junction("b2c9", "design") || x.Facts.Status != nil {
		t.Errorf("before the reviewer's commit the facts are %+v", x.Facts)
	}

	accepts := r.commit("olive", "2026-09-04", "Accept the design", nil, "Reviewed: b2c9 design")
	f = r.facts()
	if review := f.Accepted("b2c9", "design"); review == nil || review.Commit == nil || review.Commit.ID != accepts {
		t.Fatalf("the review reads %+v", review)
	}
	x = resolved(t, f, h2)
	if x.Act != None || x.Resolver != "ben@example.org" || fmt.Sprint(ids(x)) != fmt.Sprint([]string{wrong, accepts}) {
		t.Errorf("after the reviewer's commit: %s by %q with %v; want none by the author with both commits, the oldest first", x.Act, x.Resolver, ids(x))
	}
	if want := "git show --no-patch --format='%as %H %ae %ce%n%(trailers)' " + accepts + "  # shows the finding"; texts(x) != want {
		t.Errorf("after the reviewer's commit the commands are\n%s", texts(x))
	}

	// The status still states review: H5 asks the contributor to clear it,
	// and names the status's commit and the commit that accepts.
	h5 := model.Diagnostic{Code: "H5", Severity: model.Warning, Pos: model.Pos{File: "status/b2c9.yaml", Line: 3, Col: 9}, Task: "b2c9", Gate: "design", Commit: accepts}
	x = resolved(t, f, h5)
	hand := f.Handoff("b2c9")
	if hand == nil || hand.Kind != derive.Stale || hand.Commit.ID != accepts {
		t.Fatalf("the hand-off reads %+v", hand)
	}
	if x.Act != ClearHandoff || x.Resolver != "pat@example.org" || x.Facts.Status != f.Status("b2c9") || x.Facts.Junction != f.Junction("b2c9", "design") ||
		len(x.Commits) != 2 || x.Commits[0] != f.Status("b2c9").Commit || x.Commits[1] != hand.Commit {
		t.Errorf("the stale hand-off: %s by %q with %v and %+v", x.Act, x.Resolver, ids(x), x.Facts)
	}
}

// T7: an H4 where the agent is its own reviewer asks the contributor to
// record the hand-off, and the junction it carries says who reviews.
func TestHandoffToItself(t *testing.T) {
	r := newRepository(t, "{ design: { contributor: pat@example.org, model: claude-fable } }")
	newest := r.commit("pat", "2026-09-02", "Record the definition", map[string]string{".tableaux/status/b2c9.yaml": "gate: defined\nstate: nominal\n"}, "Model: claude-fable-5")
	f := r.facts()
	hand := f.Handoff("b2c9")
	if hand == nil || hand.Kind != derive.Implied || !hand.Self || hand.Commit.ID != newest {
		t.Fatalf("the hand-off reads %+v", hand)
	}
	x := resolved(t, f, model.Diagnostic{Code: "H4", Severity: model.Information, Pos: model.Pos{File: "status/b2c9.yaml"}, Task: "b2c9", Gate: "design", Commit: newest})
	j := x.Facts.Junction
	if x.Act != RecordHandoff || x.Resolver != "pat@example.org" || x.Demoted || x.Severity != model.Information {
		t.Errorf("the hand-off: %s by %q, %v", x.Act, x.Resolver, x.Severity)
	}
	if j == nil || j != f.Junction("b2c9", "design") || j.Reviewer.V != j.Contributor.V || j.Model.V == "" {
		t.Errorf("the junction reads %+v; want one whose reviewer is its contributor, an agent", j)
	}
	if x.Facts.Status != f.Status("b2c9") || fmt.Sprint(ids(x)) != fmt.Sprint([]string{newest}) || len(x.Commands) != 1 {
		t.Errorf("the hand-off carries %+v, %v and %d commands", x.Facts, ids(x), len(x.Commands))
	}
	// The same junction is a sole review: the kind and the hand-off read one fact.
	rep := audited(t, Input{Facts: f, Now: "2026-09-02"})
	if at := sole(rep); at["b2c9 design"] == nil || at["b2c9 design"].Facts.Junction != j {
		t.Errorf("the junction is no sole review: %v", gatesOf(rep, SoleReview))
	}

	// A leaf with no status file hands off too: the diagnostic stands in the
	// task file, and the finding carries the status the leaf reads.
	bare := newRepository(t, "{ defined: { reviewer: olive@example.org } }")
	g := bare.facts()
	first := g.Status("b2c9").Commit
	x = resolved(t, g, model.Diagnostic{Code: "H4", Severity: model.Information, Pos: model.Pos{File: "tasks/b2c9.yaml"}, Task: "b2c9", Gate: "defined", Commit: first.ID})
	if x.Facts.Status != g.Status("b2c9") || x.Facts.Status.Kind != derive.Absent || len(x.Commits) != 1 || x.Commits[0] != first {
		t.Errorf("a leaf with no status file carries %+v and %v", x.Facts.Status, ids(x))
	}
}

// T7: a subproject the host has no clone of is nobody's to mend in the
// files, and one whose path leads nowhere is its keeper's.
func TestSubprojectTheToolCannotRead(t *testing.T) {
	// submodule-subproject, cloned without its submodule.
	to := filepath.Join(t.TempDir(), "clone")
	gitAs(t, t.TempDir(), "olive", "2026-09-01", "clone", "-q", builtAt(t, "submodule-subproject"), to)
	f := derived(t, load.Options{}, to, "main", "")
	found := 0
	rep := audited(t, Input{Facts: f, Diagnostics: validate.Files(f.Project()), Now: now(f)})
	for _, x := range of(rep, "J8") {
		found++
		link := x.Facts.Junction.Snapshot.Link
		if link.Problem != model.NoClone || link.Form != model.Submodule {
			t.Fatalf("the link reads %+v", link)
		}
		if x.Act != Checkout || x.Resolver != "" || x.Severity != model.Error || x.Facts.Snapshot != x.Facts.Junction.Snapshot {
			t.Errorf("a submodule with no clone: %s by %q, %+v", x.Act, x.Resolver, x.Facts)
		}
		if want := "git log -1 --format='%as %H %ae' -- " + x.File + "  # shows the finding\n" +
			"git submodule update --init " + link.URL + "  # resolves it"; texts(x) != want {
			t.Errorf("a submodule with no clone gives the commands\n%s", texts(x))
		}
	}
	if found == 0 {
		t.Errorf("the clone without its submodule raises no J8: %+v", rep.Findings)
	}

	missing := derived(t, load.Options{}, builtAt(t, "subproject-path-missing"), "main", "")
	rep = audited(t, Input{Facts: missing, Diagnostics: validate.Files(missing.Project()), Now: now(missing)})
	list := of(rep, "J8")
	if len(list) != 1 {
		t.Fatalf("subproject-path-missing raises %d J8", len(list))
	}
	x := list[0]
	if x.Act != Revise || x.Resolver != "olive@example.org" || x.Facts.Snapshot == nil || x.Facts.Snapshot.Link.Problem != model.Missing || len(x.Commands) != 1 {
		t.Errorf("a path that leads nowhere: %s by %q, %+v", x.Act, x.Resolver, x.Facts)
	}
}

// The matrix of design/dada/words.md: the facts a finding of each key
// carries. "+" marks a fact it always carries where its task and its gate
// are the project's, "?" one it carries by a condition, and a fact the row
// does not name it never carries.
var matrix = map[string]string{
	Proposed:   "+authorisation",
	Stale:      "+status",
	SoleReview: "+junction",
	"S11":      "+junction +status",
	"R9":       "?requirement",
	"R12":      "?requirement",
	"R13":      "?requirement",
	"J8":       "+junction ?linkage ?status",
	"J9":       "+junction ?linkage ?status",
	"J13":      "+junction ?linkage ?status",
	"J14":      "+junction ?linkage ?status",
	"J15":      "+junction ?linkage ?status",
	"J16":      "+junction ?linkage ?status",
	"J17":      "+junction ?linkage ?status",
	"H1":       "?junction ?status",
	"H2":       "+junction ?status",
	"H3":       "+junction ?status ?model",
	"H6":       "+junction ?status ?model",
	"H4":       "+junction +status",
	"H5":       "+junction +status",
	"P5":       "",
}

// holds checks a finding's facts against the matrix: each fact it carries is
// the very pointer the method of derive.Facts returns, each fact its row
// marks "+" is there, and each fact its row does not name is nil. It
// returns the names of the facts carried.
func holds(t *testing.T, where string, f *derive.Facts, x *Finding) string {
	t.Helper()
	row, known := matrix[x.Key]
	if !known {
		row = "?junction ?status" // every other rule
	}
	var reading *derive.Reading
	if x.Diagnostic != nil {
		for _, e := range f.Events(x.Task) {
			if e.Sub == "" && e.Reading != nil && e.Commit.ID == x.Diagnostic.Commit && e.Reading.Gate == x.Gate {
				reading = e.Reading
				break
			}
		}
	}
	var requirement *derive.Condition
	for _, c := range f.Requires(x.Task) {
		if c == x.Facts.Requirement {
			requirement = c
		}
	}
	var snapshot *derive.Snapshot
	if j := f.Junction(x.Task, x.Gate); j != nil {
		snapshot = j.Snapshot
	}
	var carried []string
	for _, fact := range []struct {
		name      string
		got, want any
		set       bool
	}{
		{"junction", x.Facts.Junction, f.Junction(x.Task, x.Gate), x.Facts.Junction != nil},
		{"status", x.Facts.Status, f.Status(x.Task), x.Facts.Status != nil},
		{"requirement", x.Facts.Requirement, requirement, x.Facts.Requirement != nil},
		{"authorisation", x.Facts.Authorisation, f.Authorisation(x.Task), x.Facts.Authorisation != nil},
		{"linkage", x.Facts.Snapshot, snapshot, x.Facts.Snapshot != nil},
		{"model", x.Facts.Reading, reading, x.Facts.Reading != nil},
	} {
		always, may := strings.Contains(row, "+"+fact.name), strings.Contains(row, "?"+fact.name)
		switch {
		case fact.set && !always && !may:
			t.Errorf("%s: a finding of %s carries the fact %s", where, x.Key, fact.name)
		case fact.set && fact.got != fact.want:
			t.Errorf("%s: the fact %s of %s is not the Derivation's own value", where, fact.name, x.Key)
		case !fact.set && always && f.Junction(x.Task, x.Gate) != nil:
			t.Errorf("%s: a finding of %s carries no %s", where, x.Key, fact.name)
		}
		if fact.set {
			carried = append(carried, fact.name)
		}
	}
	return strings.Join(carried, " ")
}

// T14: the typed facts (decision 8). On every entry of the corpus the
// Derivation reads, at every ref it states, each finding holds the facts its
// row of the matrix marks, each the very pointer the method of derive.Facts
// returns, and no other. The commits are the pass's own.
func TestTypedFacts(t *testing.T) {
	seen := map[string]map[string]int{}
	for _, e := range entries(t) {
		for i, ref := range e.refs {
			f := e.at(t, ref)
			if f.Refused() != nil {
				continue
			}
			var want map[string]any
			if i == 0 {
				want = e.want
			}
			list := diagnostics(f, want, e.labels)
			for _, stale := range []int{0, 3} {
				rep := audited(t, Input{Facts: f, Diagnostics: list, Now: now(f), Stale: stale})
				for i := range rep.Findings {
					x := &rep.Findings[i]
					carried := holds(t, e.name+" at "+ref, f, x)
					if seen[x.Key] == nil {
						seen[x.Key] = map[string]int{}
					}
					seen[x.Key][carried]++
					for j, c := range x.Commits {
						if c == nil || f.Log().Commit(c.ID) != c || (j > 0 && x.Commits[j-1].Seq <= c.Seq) {
							t.Errorf("%s at %s: the commits of %s are not the pass's, each once, the oldest first", e.name, ref, x.Key)
						}
					}
				}
			}
		}
	}
	t.Logf("the facts by key: %v", seen)
	// The rows the corpus raises, with the facts each carries there.
	for key, want := range map[string]string{
		Proposed: "authorisation", Stale: "status", SoleReview: "junction",
		"S11": "junction status", "R9": "requirement", "R12": "requirement", "R13": "",
		"J8": "junction linkage", "J9": "junction linkage", "J13": "junction linkage", "J17": "junction linkage",
		"H1": "", "H2": "junction", "H3": "junction model", "H6": "junction model",
		"H4": "junction status", "H5": "junction status", "P5": "",
		"L2": "status", "G5": "", "T12": "", "S2": "",
	} {
		if seen[key][want] == 0 {
			t.Errorf("no finding of %s in the corpus carries exactly %q: %v", key, want, seen[key])
		}
	}

	// R13 at the position of its entry: the literal diagnostic of the corpus
	// test has none, so it carries no requirement there.
	behind := derived(t, load.Options{Replace: map[string]string{"https://example.org/lib.git": builtAt(t, "requires-commit-behind.lib")}},
		builtAt(t, "requires-commit-behind"), "main", "")
	c := behind.Requires("b2c9")[0]
	if c.Entry.Subproject == nil || c.Link == nil {
		t.Fatalf("the requirement of requires-commit-behind reads %+v", c)
	}
	at := c.Entry.Subproject.Node.Pos
	if node := c.Entry.Subproject.Commit.Node; node != nil {
		at = node.Pos
	}
	x := resolved(t, behind, model.Diagnostic{Code: "R13", Severity: model.Warning, Pos: at, Task: "b2c9"})
	if x.Facts.Requirement != c || x.Act != MovePin || x.Resolver != "pat@example.org" || x.Facts.Snapshot != nil || x.Facts.Junction != nil {
		t.Errorf("R13 at its entry carries %+v for %q", x.Facts, x.Resolver)
	}

	// The commits by kind, on weather-station.
	station := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	rep := audited(t, Input{Facts: station, Now: "2026-09-28", Stale: 3})
	if x := of(rep, Proposed)[0]; len(x.Commits) != 1 || x.Commits[0] != station.Authorisation("3c5d").Commit {
		t.Errorf("the proposed task names %v", ids(x))
	}
	if x := of(rep, Stale)[0]; len(x.Commits) != 1 || x.Commits[0] != station.Status("3c5d").Commit {
		t.Errorf("the stale status names %v", ids(x))
	}
	for _, x := range of(rep, SoleReview) {
		if len(x.Commits) != 0 || len(x.Commands) != 1 || !strings.HasPrefix(x.Commands[0].Text, "git log -1 ") {
			t.Errorf("the sole review of %s at %s names %v and runs %q", x.Task, x.Gate, ids(x), texts(x))
		}
	}
}

// T14: the types hold no action and no worded fact: every fact of a finding
// is a pointer to a value of the Derivation, and a commit one of the pass.
func TestFindingHoldsNoWords(t *testing.T) {
	finding := reflect.TypeOf(Finding{})
	for i := 0; i < finding.NumField(); i++ {
		if name := finding.Field(i).Name; strings.HasPrefix(name, "Action") {
			t.Errorf("Finding holds %s", name)
		}
	}
	facts := reflect.TypeOf(Facts{})
	for i := 0; i < facts.NumField(); i++ {
		field := facts.Field(i)
		if field.Type.Kind() != reflect.Pointer || field.Type.Elem().PkgPath() != reflect.TypeOf(derive.Facts{}).PkgPath() {
			t.Errorf("the fact %s is no value of the Derivation: %s", field.Name, field.Type)
		}
	}
	if field, _ := finding.FieldByName("Commits"); field.Type != reflect.TypeOf([]*history.Commit(nil)) {
		t.Errorf("the commits are %s", field.Type)
	}
	if field, _ := finding.FieldByName("Act"); field.Type != reflect.TypeOf(None) {
		t.Errorf("the act is %s", field.Type)
	}
}
