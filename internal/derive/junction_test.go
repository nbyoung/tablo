package derive

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// symbols joins the marks of a junction as a view draws them.
func symbols(j *Junction) string {
	var b strings.Builder
	for _, m := range j.Marks() {
		b.WriteString(m.Symbol())
	}
	return b.String()
}

// T7 and T8, the cases of cases.md: J1 to J9 resolve the junction of the
// leaf a000 at design, and each gives its marks.
func TestJunctionCases(t *testing.T) {
	const (
		agent    = "{ design: { contributor: bot@x, model: m } }"
		reviewed = "{ design: { reviewer: rev@x } }"
		exempt   = "{ design: { applies: false } }"
	)
	none := Field{}
	for _, tc := range []struct {
		name                         string
		files                        map[string]string
		kind                         model.JunctionKind
		contributor, model, reviewer Field
		sources                      []string
		entry, marks                 string
	}{
		{name: "J1", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, agent),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, "{ design: { contributor: carol@x } }"),
		}, contributor: Field{V: "carol@x", By: ByTask, Task: "a000"}, model: none, reviewer: none,
			sources: []string{"a000"}, marks: "🧑"},
		{name: "J2", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, agent),
		}, contributor: Field{V: "bot@x", By: ByTask, Task: "a000"}, model: Field{V: "m", By: ByTask, Task: "a000"},
			reviewer: Field{V: "rev@x", By: ByTask, Task: "r000"}, sources: []string{"a000", "r000"}, marks: "🤖👀"},
		{name: "J3", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, agent),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, ""),
		}, contributor: Field{V: "bot@x", By: ByTask, Task: "r000"}, model: Field{V: "m", By: ByTask, Task: "r000"},
			reviewer: Field{V: "a@x", By: ByAssignee}, sources: []string{"r000"}, marks: "🤖👀"},
		{name: "J4", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, agent),
			"tasks/a000.yaml": leafOf("bot@x", "r000", 1, ""),
		}, contributor: Field{V: "bot@x", By: ByTask, Task: "r000"}, model: Field{V: "m", By: ByTask, Task: "r000"},
			reviewer: Field{V: "bot@x", By: ByAssignee}, sources: []string{"r000"}, marks: "🤖"},
		{name: "J5", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/p000.yaml": leafOf("p@x", "r000", 1, exempt),
			"tasks/a000.yaml": leafOf("a@x", "p000", 1, "{ design: {} }"),
		}, contributor: Field{V: "a@x", By: ByAssignee}, model: none, reviewer: none, sources: []string{"a000"}, marks: "🧑"},
		{name: "J6", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/p000.yaml": leafOf("p@x", "r000", 1, exempt),
			"tasks/a000.yaml": leafOf("a@x", "p000", 1, ""),
		}, kind: model.NotApplicable, sources: []string{"p000"}, entry: "p000", marks: "—"},
		{name: "J7", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/p000.yaml": leafOf("p@x", "r000", 1, "{ design: { subproject: { url: lib } } }"),
			"tasks/a000.yaml": leafOf("a@x", "p000", 1, ""),
		}, contributor: Field{V: "a@x", By: ByAssignee}, model: none, reviewer: Field{V: "rev@x", By: ByTask, Task: "r000"},
			sources: []string{"r000"}, marks: "🧑👀"},
		{name: "J8", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, "{ design: { applies: false, contributor: carol@x } }"),
		}, contributor: Field{V: "a@x", By: ByAssignee}, model: none, reviewer: Field{V: "rev@x", By: ByTask, Task: "r000"},
			sources: []string{"r000"}, marks: "🧑👀"},
		{name: "J9", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, ""),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, "{ design: { reviewer: a@x } }"),
		}, contributor: Field{V: "a@x", By: ByAssignee}, model: none, reviewer: Field{V: "a@x", By: ByTask, Task: "a000"},
			sources: []string{"a000"}, marks: "🧑"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := mem(t, tc.files)
			j := f.Junction("a000", "design")
			if j == nil || j.Task != "a000" || j.Gate != "design" || j.Kind != tc.kind || j.Entry != tc.entry {
				t.Fatalf("the junction is %+v; want the kind %v by %q", j, tc.kind, tc.entry)
			}
			for _, field := range []struct {
				name      string
				got, want Field
			}{{"contributor", j.Contributor, tc.contributor}, {"model", j.Model, tc.model}, {"reviewer", j.Reviewer, tc.reviewer}} {
				if (field.got.Node != nil) != (field.want.By == ByTask) {
					t.Errorf("the %s carries the node %v; want one exactly when a task states it", field.name, field.got.Node)
				}
				if field.got.Node != nil && field.got.Node.Text != field.got.V {
					t.Errorf("the %s's node reads %q; want %q", field.name, field.got.Node.Text, field.got.V)
				}
				field.got.Node = nil
				if field.got != field.want {
					t.Errorf("the %s is %+v; want %+v", field.name, field.got, field.want)
				}
			}
			if !reflect.DeepEqual(j.Sources(), tc.sources) {
				t.Errorf("the sources are %v; want %v", j.Sources(), tc.sources)
			}
			if got := symbols(j); got != tc.marks {
				t.Errorf("the marks are %s; want %s", got, tc.marks)
			}
		})
	}
}

// T7: the entries that supply nothing, and the gates around a junction.
func TestJunctionWalk(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/r000.yaml": leafOf("r@x", "", 0, "{ undefined: { reviewer: rev@x }, nowhere: { reviewer: rev@x }, "+
			"mockup: { applies: true }, function: { applies: false }, release: { model: m, references: [{ url: a.md }] } }"),
		"tasks/a000.yaml": leafOf("a@x", "r000", 1, "{ function: { references: [{ url: b.md, text: B }] }, release: { reviewer: rev@x }, design: { applies: false } }"),
		"tasks/b000.yaml": leafOf("b@x", "r000", 2, ""),
	})
	if f.Junction("a000", "undefined") != nil || f.Junction("a000", "nowhere") != nil || f.Junction("none", "design") != nil {
		t.Error("a junction resolves at undefined, at a key gates.yaml lacks or for a task the tree lacks")
	}
	// applies: true states nothing, so mockup stays the plain default.
	if j := f.Junction("a000", "mockup"); j.Kind != model.Plain || len(j.Sources()) != 0 || j.Contributor != (Field{V: "a@x", By: ByAssignee}) {
		t.Errorf("under applies: true the junction is %+v; want the plain default", j)
	}
	// a000 states its own entry under the root's exemption: plain, by a000.
	if j := f.Junction("a000", "function"); j.Kind != model.Plain || j.ReferencesFrom != "a000" || len(j.References) != 1 ||
		!reflect.DeepEqual(j.Sources(), []string{"a000"}) {
		t.Errorf("under the exemption the junction is %+v; want a000's own references", j)
	}
	if j := f.Junction("b000", "function"); j.Kind != model.NotApplicable || j.Entry != "r000" {
		t.Errorf("b000 at function is %+v; want exempt by r000", j)
	}
	// A model with no contributor beside it: the assignee contributes and reviews.
	j := f.Junction("b000", "release")
	if j.Model.V != "m" || j.Contributor != (Field{V: "b@x", By: ByAssignee}) || j.Reviewer != (Field{V: "b@x", By: ByAssignee}) ||
		j.ReferencesFrom != "r000" || symbols(j) != "🤖" {
		t.Errorf("b000 at release is %+v", j)
	}
	if j := f.Junction("a000", "release"); j.Reviewer.V != "rev@x" || !reflect.DeepEqual(j.Sources(), []string{"a000", "r000"}) {
		t.Errorf("a000 at release is %+v with the sources %v", j, j.Sources())
	}
	if got, want := f.Applicable("a000"), []string{"undefined", "defined", "mockup", "function", "release"}; !reflect.DeepEqual(got, want) {
		t.Errorf("a000's applicable gates are %v; want %v", got, want)
	}
	if got := len(f.Junctions("a000")); got != 5 {
		t.Errorf("a000 has %d junctions; want one per gate after undefined", got)
	}
	if f.First("a000") != "defined" || f.Last("a000") != "release" || f.Next("a000", "function") != "release" ||
		f.Next("a000", "release") != "" || f.Next("a000", "nowhere") != "" || f.Next("a000", "undefined") != "defined" ||
		f.Next("b000", "mockup") != "design" || f.Applicable("none") != nil || f.Last("none") != "" {
		t.Error("First, Last or Next disagrees with the applicable gates")
	}
	if f.Index("design") != 4 || f.Index("nowhere") != -1 || f.Severity("stalled") != 3 || f.Severity("paused") != 0 || len(f.Gates()) != 6 {
		t.Error("Index, Severity or Gates disagrees with gates.yaml")
	}
	for mark, symbol := range map[Mark]string{MarkPerson: "🧑", MarkAgent: "🤖", MarkReviewer: "👀", MarkSubproject: "🪆", MarkExempt: "—", "none": ""} {
		if mark.Symbol() != symbol {
			t.Errorf("the mark %s draws as %q; want %q", mark, mark.Symbol(), symbol)
		}
	}
}

// T8: the marks of c07d in the weather station.
func TestMarksOfTheFirmware(t *testing.T) {
	f := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	for gate, want := range map[string]string{"mockup": "🧑", "reliability": "—", "implementation": "🪆", "unit": "🤖👀"} {
		if got := symbols(f.Junction("c07d", gate)); got != want {
			t.Errorf("c07d at %s draws %s; want %s", gate, got, want)
		}
	}
	snapshots := f.Snapshots("c07d")
	if len(snapshots) != 1 || f.Junction("c07d", "implementation").Snapshot != snapshots[0] {
		t.Fatalf("c07d has the snapshots %+v", snapshots)
	}
	s := snapshots[0]
	if s.Task != "c07d" || s.Target != "f1a0" || s.Why != Determined || !reflect.DeepEqual(s.Gates, []string{"implementation"}) ||
		s.Link == nil || s.Facts != f.Sub(s.Link) || s.Facts.Root() != "b7d2" || s.Entry.URL.V != "firmware" {
		t.Errorf("the snapshot is %+v, of the project rooted at %q", s, s.Facts.Root())
	}
	// One snapshot serves the ten junctions that name one target.
	g := derived(t, load.Options{}, builtAt(t, "submodule-subproject"), "main", "")
	if list := g.Snapshots("c100"); len(list) != 1 || len(list[0].Gates) != 10 || list[0].Target != "5a00" {
		t.Errorf("c100 has the snapshots %+v; want one of ten gates at the subproject's root", list)
	}
	if g.Snapshots("c000") != nil || g.Sub(nil) != nil || g.Sub(&model.Link{}) != nil {
		t.Error("a parent has a snapshot, or a link with no project has facts")
	}
}
