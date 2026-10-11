package views

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// station derives the weather station at a ref and returns its labels.
func station(t testing.TB, ref string) (*derive.Facts, map[string]string) {
	t.Helper()
	return derived(t, load.Options{}, builtAt(t, "weather-station"), ref, ""), labels(t, "weather-station")
}

// T4: the legend of the weather station, and the reason the method reserves.
func TestLegend(t *testing.T) {
	f, _ := station(t, "main")
	l := builder{f: f}.legend()
	var gates, states, reasons, drawn []string
	for _, g := range l.Gates {
		gates = append(gates, g.Key)
		if g.Symbol == "" || g.Name == "" || g.Criteria == "" {
			t.Errorf("the gate %s lacks a symbol, a name or its criteria: %+v", g.Key, g)
		}
	}
	if want := strings.Fields("undefined defined mockup function performance reliability design implementation unit integrate validate release"); !reflect.DeepEqual(gates, want) {
		t.Errorf("the gates are %v; want the twelve of the file, in its order", gates)
	}
	severities := map[string]int{}
	for _, s := range l.States {
		states = append(states, s.Key)
		severities[s.Key] = s.Severity
	}
	if want := strings.Fields("undefined nominal at_risk stalled complete"); !reflect.DeepEqual(states, want) {
		t.Errorf("the states are %v; want %v", states, want)
	}
	if want := map[string]int{"undefined": 0, "nominal": 1, "at_risk": 2, "stalled": 3, "complete": 0}; !reflect.DeepEqual(severities, want) {
		t.Errorf("the severities are %v; want %v", severities, want)
	}
	for _, r := range l.Reasons {
		reasons = append(reasons, r.Key)
		if r.Reserved {
			t.Errorf("the reason %s reads as reserved", r.Key)
		}
	}
	if want := []string{"overloaded", "blocked"}; !reflect.DeepEqual(reasons, want) {
		t.Errorf("the reasons are %v; want %v", reasons, want)
	}
	for _, m := range l.Marks {
		drawn = append(drawn, m.Key+" "+m.Symbol+" "+m.Junction)
		if m.Meaning == "" {
			t.Errorf("the mark %s has no meaning", m.Key)
		}
	}
	if want := []string{"person 🧑 plain", "agent 🤖 plain", "reviewer 👀 plain", "subproject 🪆 recursive", "exempt — not_applicable"}; !reflect.DeepEqual(drawn, want) {
		t.Errorf("the marks are %v; want %v", drawn, want)
	}

	// The weather station states no reason review; the cases of design 27a3 do.
	repo, _, _ := cases(t)
	reserved := builder{f: derived(t, load.Options{}, repo, "main", "")}.legend().Reasons
	if len(reserved) != 1 || reserved[0].Key != "review" || !reserved[0].Reserved || reserved[0].Symbol != "👓" {
		t.Errorf("the reasons of the cases are %+v; want review, reserved", reserved)
	}
}

// T9: where a junction stands, who acts and what is reviewed.
func TestJunctionStands(t *testing.T) {
	f, _ := station(t, "main")
	for _, tc := range []struct {
		task, gate, person string
		stands             string
		acts, reviewed     bool
	}{
		{"c07d", "defined", "", standsPassed, false, false},
		{"c07d", "mockup", "ben@example.org", standsPassed, true, true}, // contributor and reviewer
		{"c07d", "reliability", "ben@example.org", standsExempt, false, false},
		{"c07d", "design", "ben@example.org", standsHere, true, false},
		{"c07d", "implementation", "ben@example.org", standsNext, false, false}, // recursive: nobody of this project acts
		{"c07d", "unit", "opus@example.org", standsLater, true, false},          // the contributor
		{"c07d", "unit", "ben@example.org", standsLater, true, false},           // the reviewer, by default
		{"c07d", "unit", "ada@example.org", standsLater, false, false},
		{"c07d", "release", "", standsLater, false, false},
		{"9f31", "mockup", "ben@example.org", standsPassed, true, true}, // the reviewer
		{"9f31", "mockup", "", standsPassed, false, true},
		{"9f31", "function", "ada@example.org", standsHere, true, false},
		{"9f31", "function", "ben@example.org", standsHere, false, false},
		{"9f31", "performance", "ada@example.org", standsNext, true, false},
		{"9f31", "validate", "ben@example.org", standsLater, true, false},
		{"4e2b", "mockup", "ben@example.org", "", true, false}, // a parent: its junctions are defaults
		{"4e2b", "design", "", "", false, false},
		{"a1c0", "defined", "", "", false, false},
	} {
		b := builder{f: f, q: Query{Person: tc.person}}
		j := b.junction(f.Junction(tc.task, tc.gate), false)
		if j.Gate != tc.gate || j.Stands != tc.stands || j.Acts != tc.acts || j.Reviewed != tc.reviewed {
			t.Errorf("%s at %s for %q stands %q, acts %v, reviewed %v; want %q, %v, %v", tc.task, tc.gate, tc.person, j.Stands, j.Acts, j.Reviewed, tc.stands, tc.acts, tc.reviewed)
		}
		if j.From != nil {
			t.Errorf("%s at %s carries from where the view shows no suppliers", tc.task, tc.gate)
		}
	}
	b := builder{f: f}
	unit := b.junction(f.Junction("c07d", "unit"), true)
	if !reflect.DeepEqual(unit.Marks, []string{"agent", "reviewer"}) || unit.Model != "claude-opus-5-5" || !unit.ReviewerByDefault || unit.Reviewer != "ben@example.org" || unit.Subproject != nil {
		t.Errorf("c07d at unit is %+v; want an agent whom the assignee reviews", unit)
	}
	sub := b.junction(f.Junction("c07d", "implementation"), true)
	if sub.Kind != "recursive" || sub.Subproject == nil || sub.Subproject.URL != "firmware" || sub.Subproject.Task != (view.TaskRef{ID: "f1a0", Title: "Weather node image"}) ||
		sub.Subproject.Form != "submodule" || sub.Subproject.Why != "" || sub.Contributor != "" {
		t.Errorf("c07d at implementation is %+v with %+v", sub, sub.Subproject)
	}
	if exempt := b.junction(f.Junction("c07d", "reliability"), true); exempt.Kind != "not_applicable" || !reflect.DeepEqual(exempt.Marks, []string{"exempt"}) {
		t.Errorf("c07d at reliability is %+v", exempt)
	}
	if b.subRef(nil) != nil {
		t.Error("no snapshot gives a reference")
	}
}

// textAt returns what a project file holds at a line and a column, both from 1,
// the column in code points.
func textAt(t testing.TB, text string, line, col int) string {
	t.Helper()
	lines := strings.Split(text, "\n")
	if line < 1 || line > len(lines) {
		t.Fatalf("the file has no line %d", line)
	}
	runes := []rune(lines[line-1])
	if col < 1 || col > len(runes) {
		t.Fatalf("line %d has no column %d", line, col)
	}
	return string(runes[col-1:])
}

// pointsAt checks that a supplier a task states names its key and its file
// and points at the value in the file, or at the entry's key for a value
// the entry as a whole states.
func pointsAt(t testing.TB, where string, s view.Supplier, file func(path string) string) {
	t.Helper()
	switch s.By {
	case "assignee":
		if s.Stated != (view.Stated{By: "assignee", Key: "assignee"}) {
			t.Errorf("%s: the assignee supplies %+v", where, s)
		}
		return
	case "default", "own", "":
		t.Errorf("%s: a supplier reads %+v", where, s)
		return
	}
	if s.Key != "junctions."+s.Gate+"."+s.Field || s.File != ".tableaux/tasks/"+s.By+".yaml" {
		t.Errorf("%s: the supplier names %q in %q", where, s.Key, s.File)
	}
	want := s.Value
	if s.Field == "references" || s.Field == "subproject" || s.Field == "applies" {
		want = s.Gate + ":"
	}
	if got := textAt(t, file(s.File), s.Line, s.Col); !strings.HasPrefix(got, want) {
		t.Errorf("%s: %s at %d:%d of %s reads %q; want %q there", where, s.Key, s.Line, s.Col, s.File, got, want)
	}
}

// T10: suppliers. The six rows of c07d, every supplier of the weather
// station against its file, and the cases J1 to J9 of design 27a3.
func TestSuppliers(t *testing.T) {
	f, _ := station(t, "main")
	b := builder{f: f}
	var got []view.Supplier
	for _, j := range f.Junctions("c07d") {
		got = append(got, b.suppliers(j)...)
	}
	var golden struct {
		Sources []view.Supplier `json:"sources"`
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "design", "493e", "golden", "task-c07d.provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(got) != 6 || !reflect.DeepEqual(got, golden.Sources) {
		t.Errorf("the suppliers of c07d are %+v; want the six rows of the design's golden file", got)
	}
	if last := got[len(got)-1]; last.Gate != "unit" || last.Field != "reviewer" || last.Value != "ben@example.org" || last.By != "assignee" {
		t.Errorf("the reviewer of unit comes from %+v; want the assignee", last)
	}
	onDisk := func(path string) string {
		data, err := os.ReadFile(filepath.Join(builtAt(t, "weather-station"), filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	stated := 0
	for _, id := range f.Order() {
		for _, j := range f.Junctions(id) {
			for _, s := range b.suppliers(j) {
				stated++
				pointsAt(t, id+" at "+j.Gate, s, onDisk)
			}
		}
	}
	if stated < 10 {
		t.Errorf("the weather station states %d suppliers", stated)
	}

	const (
		agent    = "{ design: { contributor: bot@x, model: m } }"
		reviewed = "{ design: { reviewer: rev@x } }"
		exempt   = "{ design: { applies: false } }"
	)
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  []string // field, value and supplier of each row
	}{
		{"J1", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, agent),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, "{ design: { contributor: carol@x } }"),
		}, []string{"contributor carol@x a000"}},
		{"J2", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, agent),
		}, []string{"contributor bot@x a000", "model m a000", "reviewer rev@x r000"}},
		{"J3", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, agent),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, ""),
		}, []string{"contributor bot@x r000", "model m r000", "reviewer a@x assignee"}},
		{"J4", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, agent),
			"tasks/a000.yaml": leafOf("bot@x", "r000", 1, ""),
		}, []string{"contributor bot@x r000", "model m r000", "reviewer bot@x assignee"}},
		{"J5", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/p000.yaml": leafOf("p@x", "r000", 1, exempt),
			"tasks/a000.yaml": leafOf("a@x", "p000", 1, "{ design: {} }"),
		}, nil},
		{"J6", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/p000.yaml": leafOf("p@x", "r000", 1, exempt),
			"tasks/a000.yaml": leafOf("a@x", "p000", 1, ""),
		}, []string{"applies false p000"}},
		{"J7", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/p000.yaml": leafOf("p@x", "r000", 1, "{ design: { subproject: { url: lib } } }"),
			"tasks/a000.yaml": leafOf("a@x", "p000", 1, ""),
		}, []string{"reviewer rev@x r000"}},
		{"J8", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, reviewed),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, "{ design: { applies: false, contributor: carol@x } }"),
		}, []string{"reviewer rev@x r000"}},
		{"J9", map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, ""),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, "{ design: { reviewer: a@x } }"),
		}, []string{"reviewer a@x a000"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := mem(t, tc.files)
			b := builder{f: f}
			j := f.Junction("a000", "design")
			shown := b.junction(j, true)
			if shown.From == nil || !reflect.DeepEqual(shown.From, append([]string{}, j.Sources()...)) {
				t.Errorf("from is %v; the Derivation gives %v", shown.From, j.Sources())
			}
			var rows []string
			for _, s := range b.suppliers(j) {
				rows = append(rows, s.Field+" "+s.Value+" "+s.By)
				pointsAt(t, tc.name, s, func(path string) string { return tc.files[strings.TrimPrefix(path, ".tableaux/")] })
			}
			if !reflect.DeepEqual(rows, tc.want) {
				t.Errorf("the suppliers are %v; want %v", rows, tc.want)
			}
		})
	}

	// The references of an entry, and the subproject of a recursive junction
	// with no project behind its link.
	refs := b.suppliers(f.Junction("9f31", "performance"))
	if len(refs) != 1 || refs[0].Field != "references" || refs[0].Value != "1" || refs[0].By != "9f31" || refs[0].Line != 11 || refs[0].Col != 3 {
		t.Errorf("the references of 9f31 at performance come from %+v", refs)
	}
	lost := mem(t, map[string]string{"tasks/a000.yaml": leafOf("a@x", "", 0, "{ design: { subproject: { url: lib } } }")})
	n := builder{f: lost}.suppliers(lost.Junction("a000", "design"))
	if len(n) != 1 || n[0].Field != "subproject" || n[0].Value != "lib" || n[0].By != "a000" {
		t.Errorf("a link with no project gives the suppliers %+v", n)
	}
	if ref := (builder{f: lost}).subRef(lost.Junction("a000", "design").Snapshot); ref == nil || ref.Why != "no_link" || ref.Task != (view.TaskRef{}) || ref.URL != "lib" {
		t.Errorf("a link with no project gives the reference %+v", ref)
	}
}

// The builders the views share beyond a junction: the tree, a commit, a
// status with its chain, and a requirement from either end.
func TestSharedBuilders(t *testing.T) {
	f, hashes := station(t, "main")
	b := builder{f: f}
	if got := b.subtree(f.Root()); !reflect.DeepEqual(got, f.Order()) {
		t.Errorf("the subtree of the root is %v; the order is %v", got, f.Order())
	}
	if got := b.subtree("4e2b"); !reflect.DeepEqual(got, []string{"4e2b", "9f31", "c07d"}) {
		t.Errorf("the subtree of 4e2b is %v", got)
	}
	if b.parent("c07d") != "4e2b" || b.parent("4e2b") != "a1c0" || b.parent("a1c0") != "" {
		t.Errorf("the parents are %q, %q and %q", b.parent("c07d"), b.parent("4e2b"), b.parent("a1c0"))
	}
	if b.ref("9f31") != (view.TaskRef{ID: "9f31", Title: "Sensor board"}) || b.ref("zzzz") != (view.TaskRef{ID: "zzzz"}) {
		t.Errorf("the references are %+v and %+v", b.ref("9f31"), b.ref("zzzz"))
	}
	if b.commit(nil) != nil {
		t.Error("no commit gives a commit fact")
	}
	merge := b.commit(f.Log().Commit(hashes["W4"]))
	if merge.Hash != hashes["W4"] || merge.Date != "2026-09-19" || !strings.HasPrefix(merge.AuthorTime, "2026-09-19T") ||
		!reflect.DeepEqual(merge.Parents, []string{hashes["W2"], hashes["W3"]}) || !reflect.DeepEqual(merge.Trailers, []string{"Authorised: 9f31"}) ||
		!reflect.DeepEqual(merge.Files, []view.File{{Path: ".tableaux/tasks/9f31.yaml", Change: "added"}}) ||
		merge.Author != (view.Person{Name: "Ben Okafor", Email: "ben@example.org"}) || merge.Committer != merge.Author {
		t.Errorf("the merge W4 is %+v", merge)
	}
	if first := b.commit(f.Log().Commit(hashes["W1"])); first.Parents == nil || len(first.Parents) != 0 || first.Trailers == nil || len(first.Trailers) != 0 {
		t.Errorf("the first commit's parents and trailers are %#v and %#v; want empty lists", first.Parents, first.Trailers)
	}
	if pin := b.commit(f.Log().Commit(hashes["W11"])); !reflect.DeepEqual(pin.Files, []view.File{{Path: ".tableaux/status/c07d.yaml", Change: "added"}, {Path: "firmware", Change: "pin"}}) {
		t.Errorf("W11 changes %+v", pin.Files)
	}

	chain := b.chain("a1c0")
	if len(chain) != 2 || chain[0].Task.ID != "a1c0" || chain[0].Status.Kind != "rollup" || chain[0].Status.From == nil ||
		chain[0].Status.From.Task != (view.TaskRef{ID: "3c5d", Title: "Dashboard"}) || chain[1].Task.ID != "3c5d" || chain[1].Status.Kind != "recorded" || chain[1].URL != "" {
		t.Errorf("the chain of the root is %+v", chain)
	}
	if s := b.status(f, nil); s != (view.Status{}) {
		t.Errorf("no status gives %+v", s)
	}

	requires := f.Requires("c07d")
	if len(requires) != 1 {
		t.Fatalf("c07d has %d requirements", len(requires))
	}
	r := b.requirement(requires[0], false)
	if r.Task.ID != "9f31" || r.Task.Title != "Sensor board" || r.Condition != "unmet" || !r.Due || r.Met || r.Status == nil || r.Status.Gate != "function" || r.Text != "Pin map and sensor bus" {
		t.Errorf("the requirement of c07d is %+v", r)
	}
	if d := b.requirement(f.Dependents("9f31")[0], true); d.Task.ID != "c07d" || d.Status != nil || d.Condition != "unmet" {
		t.Errorf("the dependent of 9f31 is %+v", d)
	}
}
