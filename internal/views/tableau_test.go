package views

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// columnsOf writes each column of a tableau as its gates and its count: a
// folded column in brackets, with the count of each of its gates.
func columnsOf(v *view.Tableau) []string {
	var out []string
	for _, c := range v.Columns {
		text := fmt.Sprintf("%s=%d", strings.Join(c.Gates, ","), c.Count)
		if c.Folded {
			text = fmt.Sprintf("[%s]=%d %v", strings.Join(c.Gates, ","), c.Count, c.ByGate)
		}
		out = append(out, text)
	}
	return out
}

// gridOf writes each row of a tableau as its id, its depth, the task it
// stands under, its count of hidden tasks and its label.
func gridOf(v *view.Tableau) []string {
	out := []string{}
	for _, r := range v.Rows {
		out = append(out, strings.TrimSpace(fmt.Sprintf("%s %d %s %d %s", r.ID, r.Depth, r.Under, r.Hidden, r.Label)))
	}
	return out
}

// rowOf returns the row of a task, and fails the test without one.
func rowOf(t testing.TB, v *view.Tableau, id string) view.GridRow {
	t.Helper()
	for _, r := range v.Rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("the tableau holds no row for %s", id)
	return view.GridRow{}
}

// cellOf returns the cell of a row at a gate.
func cellOf(t testing.TB, v *view.Tableau, id, gate string) view.Cell {
	t.Helper()
	row := rowOf(t, v, id)
	if len(row.Cells) != len(v.Legend.Gates) {
		t.Fatalf("%s holds %d cells for %d gates", id, len(row.Cells), len(v.Legend.Gates))
	}
	for i, g := range v.Legend.Gates {
		if g.Key == gate {
			return row.Cells[i]
		}
	}
	t.Fatalf("the legend holds no gate %s", gate)
	return view.Cell{}
}

// T4: the window. It runs one column either side of the next gates of the
// focus; the spine and the siblings count where they fold and do not widen
// it.
func TestWindow(t *testing.T) {
	f, _ := station(t, "main")
	shown := "undefined=1 defined=2 mockup=0 function=2 performance=0 reliability=0 design=1 implementation=0 unit=0"
	if got, want := columnsOf(Tableau(f, Query{Window: 1})), append(strings.Fields(shown), "[integrate,validate,release]=0 [0 0 0]"); !reflect.DeepEqual(got, want) {
		t.Errorf("the tableau's columns are %q; want %q", got, want)
	}
	want := []string{"[undefined]=1 [1]", "defined=2", "mockup=0", "function=2", "performance=0", "reliability=0", "design=1", "implementation=0", "[unit,integrate,validate,release]=0 [0 0 0 0]"}
	if got := columnsOf(Tableau(f, Query{Window: 0})); !reflect.DeepEqual(got, want) {
		t.Errorf("at window 0 the columns are %q; want %q", got, want)
	}
	want = []string{"[undefined,defined,mockup]=3 [1 2 0]", "function=2", "performance=0", "reliability=0", "design=1", "implementation=0", "unit=0", "[integrate,validate,release]=0 [0 0 0]"}
	for _, level := range []Level{Glance, Detail, Provenance} {
		if got := columnsOf(Context(f, Query{Task: "4e2b", Window: 1, Level: level})); !reflect.DeepEqual(got, want) {
			t.Errorf("under 4e2b at %s the columns are %q; want %q", levels[level], got, want)
		}
	}
	// The person form spans the person's own tasks, 4e2b and c07d, as the
	// task form spans the subtree.
	if got := columnsOf(Context(f, Query{Person: "ben@example.org", Window: 1})); !reflect.DeepEqual(got, want) {
		t.Errorf("for ben the columns are %q; want %q", got, want)
	}
	// A window wider than the gates shows them all.
	if got := columnsOf(Tableau(f, Query{Window: 20})); len(got) != 12 || strings.Contains(strings.Join(got, " "), "[") {
		t.Errorf("at window 20 the columns are %q", got)
	}
}

// T5: the columns list shows the gates it names, in gate order, and folds
// each run between them; with no next gate in the focus nothing folds.
func TestColumnsList(t *testing.T) {
	f, _ := station(t, "main")
	got := columnsOf(Tableau(f, Query{Window: 1, Columns: []string{"implementation", "function", "design"}}))
	want := []string{"[undefined,defined,mockup]=3 [1 2 0]", "function=2", "[performance,reliability]=0 [0 0]", "design=1", "implementation=0", "[unit,integrate,validate,release]=0 [0 0 0 0]"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the columns are %q; want %q", got, want)
	}

	// A copy with every leaf at its last gate: no task has a next gate.
	repo := clone(t, "weather-station")
	for _, id := range []string{"9f31", "c07d", "7b2e", "3c5d"} {
		if err := os.WriteFile(filepath.Join(repo, ".tableaux", "status", id+".yaml"), []byte("gate: release\nstate: nominal\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	done := Tableau(derived(t, load.Options{}, repo, "", ""), Query{Window: 1, Level: Detail})
	cols := columnsOf(done)
	if len(cols) != 12 || cols[11] != "release=6" || strings.Contains(strings.Join(cols, " "), "[") {
		t.Errorf("with every leaf at release the columns are %q", cols)
	}
	if c := cellOf(t, done, "9f31", "release"); c.Kind != "status" || c.State != "nominal" {
		t.Errorf("9f31 at release reads %+v", c)
	}
}

// T6: counts. For every repository the corpus builds that the Derivation
// does not refuse, the valid entries among them, the counts of all columns
// sum to the rows of the detail level whose status names a gate, and the
// three levels give equal columns.
func TestCounts(t *testing.T) {
	entries, tableaux := 0, 0
	for _, name := range built(t) {
		f := derived(t, load.Options{}, builtAt(t, name), "HEAD", "")
		if f.Refused() != nil {
			continue
		}
		entries++
		// The tableau, by window and by columns; the contextual tableau of
		// every task and of every assignee.
		type call struct {
			build func(*derive.Facts, Query) *view.Tableau
			q     Query
		}
		all := []call{{Tableau, Query{Window: 1}}, {Tableau, Query{}}, {Tableau, Query{Columns: f.Gates()[:1]}}}
		people := map[string]bool{}
		for _, id := range f.Order() {
			all = append(all, call{Context, Query{Task: id, Window: 1}})
			if email := f.Project().Tasks[id].Assignee.V; email != "" && !people[email] {
				people[email] = true
				all = append(all, call{Context, Query{Person: email, Window: 1}})
			}
		}
		for _, c := range all {
			build, q := c.build, c.q
			q.Level = Detail
			detail := build(f, q)
			named := 0
			for _, row := range detail.Rows {
				if f.Index(row.Status.Gate) >= 0 {
					named++
				}
			}
			sum, gates := 0, 0
			for _, c := range detail.Columns {
				sum += c.Count
				gates += len(c.Gates)
				each := 0
				for _, n := range c.ByGate {
					each += n
				}
				if each != c.Count || len(c.ByGate) != len(c.Gates) || len(c.Gates) == 0 || !c.Folded && len(c.Gates) != 1 {
					t.Errorf("%s: %+v: the column %+v does not add up", name, q, c)
				}
			}
			if sum != named || gates != len(f.Gates()) {
				t.Errorf("%s: %+v: the columns count %d tasks over %d gates; %d rows name one of %d gates", name, q, sum, gates, named, len(f.Gates()))
			}
			for _, level := range []Level{Glance, Provenance} {
				q.Level = level
				if other := build(f, q); !reflect.DeepEqual(other.Columns, detail.Columns) {
					t.Errorf("%s: %+v: the columns at %s differ from those at detail", name, q, levels[level])
				}
			}
			tableaux++
		}
	}
	if entries < 90 || tableaux < 500 {
		t.Errorf("the test reads %d tableaux of %d entries; the corpus holds more", tableaux, entries)
	}
}

// T7: the cell rule.
func TestCells(t *testing.T) {
	f, _ := station(t, "main")
	v := Tableau(f, Query{Window: 1, Level: Detail})
	reads := func(v *view.Tableau, id, gate string) string {
		c := cellOf(t, v, id, gate)
		text := c.Kind
		if c.Historical {
			text = "historical " + text
		}
		return strings.Join(strings.Fields(text+" "+c.State+" "+c.Reason+" "+strings.Join(c.Marks, ",")), " ")
	}
	for _, c := range []struct{ task, gate, want string }{
		{"9f31", "undefined", "historical empty"},
		{"9f31", "mockup", "historical empty"},
		{"9f31", "function", "status stalled blocked"},
		{"9f31", "performance", "marks person"},
		{"9f31", "validate", "marks person,reviewer"},
		{"c07d", "function", "historical empty"},
		{"c07d", "reliability", "historical empty"},
		{"c07d", "design", "status nominal"},
		{"c07d", "implementation", "marks subproject"},
		{"c07d", "unit", "marks agent,reviewer"},
		{"7b2e", "undefined", "status undefined"},
		{"7b2e", "defined", "marks person"},
		{"4e2b", "function", "status stalled blocked"},
		{"4e2b", "mockup", "historical empty"},
		{"a1c0", "defined", "status nominal"},
	} {
		if got := reads(v, c.task, c.gate); got != c.want {
			t.Errorf("%s at %s reads %q; want %q", c.task, c.gate, got, c.want)
		}
	}
	// With the historical junctions, a gate before the status shows its
	// junction, and undefined, which has none, stays empty.
	h := Tableau(f, Query{Window: 1, Level: Detail, Historical: true})
	for _, c := range []struct{ task, gate, want string }{
		{"c07d", "undefined", "historical empty"},
		{"c07d", "defined", "historical marks person"},
		{"c07d", "mockup", "historical marks person"},
		{"c07d", "function", "historical marks person"},
		{"c07d", "performance", "historical marks person"},
		{"c07d", "reliability", "historical exempt"},
		{"c07d", "design", "status nominal"},
		{"c07d", "implementation", "marks subproject"},
		{"9f31", "mockup", "historical marks person,reviewer"},
	} {
		if got := reads(h, c.task, c.gate); got != c.want {
			t.Errorf("with the historical junctions %s at %s reads %q; want %q", c.task, c.gate, got, c.want)
		}
	}
	// A gate that does not apply reads exempt after the status too.
	kinds := Tableau(entry(t, "junction-kinds"), Query{Level: Detail})
	exempt := 0
	for _, row := range kinds.Rows {
		for _, c := range row.Cells {
			if c.Kind == "exempt" {
				exempt++
				if len(c.Marks) != 0 || c.State != "" || c.Acts {
					t.Errorf("junction-kinds: an exempt cell of %s carries %+v", row.ID, c)
				}
			}
		}
	}
	if exempt == 0 {
		t.Error("junction-kinds shows no exempt cell")
	}

	// A parent's cell shows the marks its subtree inherits: an agent with no
	// stated reviewer draws no reviewer on the parent (decision 4), and the
	// child that takes its own assignee as reviewer draws one.
	g := mem(t, map[string]string{
		"tasks/r000.yaml": leafOf("r@x", "", 0, "{ design: { contributor: bot@x, model: m-1 } }"),
		"tasks/a000.yaml": leafOf("a@x", "r000", 1, ""),
	})
	agent := Tableau(g, Query{Level: Detail})
	if parent, child := cellOf(t, agent, "r000", "design"), cellOf(t, agent, "a000", "design"); !reflect.DeepEqual(parent.Marks, []string{"agent"}) || !reflect.DeepEqual(child.Marks, []string{"agent", "reviewer"}) {
		t.Errorf("the parent draws %v and the child %v; want the agent alone on the parent", parent.Marks, child.Marks)
	}
}

// T8: acts. The person in view acts on a leaf's plain junction after the
// status, never on a parent's default and never at or before a status.
func TestActs(t *testing.T) {
	f, _ := station(t, "main")
	acts := func(v *view.Tableau, shownOnly bool) []string {
		out := []string{}
		shown := map[string]bool{}
		for _, c := range v.Columns {
			if !c.Folded {
				shown[c.Gates[0]] = true
			}
		}
		for _, row := range v.Rows {
			for i, c := range row.Cells {
				gate := v.Legend.Gates[i].Key
				if c.Acts && (shown[gate] || !shownOnly) {
					out = append(out, row.ID+" "+gate)
				}
			}
		}
		return out
	}
	ben := Context(f, Query{Person: "ben@example.org", Window: 1, Level: Detail})
	if got := acts(ben, true); !reflect.DeepEqual(got, []string{"c07d unit"}) {
		t.Errorf("in ben's window the cells that act are %v; want unit of c07d alone", got)
	}
	// Beyond the window ben reviews 9f31 at validate and contributes to c07d after unit.
	if got, want := acts(ben, false), []string{"9f31 validate", "c07d unit", "c07d integrate", "c07d validate", "c07d release"}; !reflect.DeepEqual(got, want) {
		t.Errorf("for ben the cells that act are %v; want %v", got, want)
	}
	ada := Tableau(f, Query{Person: "ada@example.org", Window: 1, Level: Detail})
	got := acts(ada, false)
	if len(got) == 0 {
		t.Fatal("no cell acts for ada")
	}
	for _, row := range ada.Rows {
		at := f.Index(row.Status.Gate)
		for i, c := range row.Cells {
			if c.Acts && (row.Parent || i <= at || c.Kind != "marks") {
				t.Errorf("for ada the cell of %s at %s acts: %+v", row.ID, ada.Legend.Gates[i].Key, c)
			}
		}
	}
	if joined := strings.Join(got, ";"); !strings.Contains(joined, "9f31 performance") || !strings.Contains(joined, "7b2e defined") || strings.Contains(joined, "9f31 function") || strings.Contains(joined, "a1c0") {
		t.Errorf("for ada the cells that act are %v", got)
	}
	if got := acts(Tableau(f, Query{Window: 1, Level: Detail}), false); len(got) != 0 {
		t.Errorf("with nobody in view the cells that act are %v", got)
	}
}

// T9: rows, labels, depth and hidden.
func TestRows(t *testing.T) {
	f, _ := station(t, "main")
	for _, c := range []struct {
		name string
		got  *view.Tableau
		want []string
	}{
		{"the tableau at glance", Tableau(f, Query{}), []string{"a1c0 0  0", "4e2b 1 a1c0 2", "7b2e 1 a1c0 0", "3c5d 1 a1c0 0"}},
		{"the tableau at detail", Tableau(f, Query{Level: Detail}), []string{"a1c0 0  0", "4e2b 1 a1c0 0", "9f31 2 4e2b 0", "c07d 2 4e2b 0", "7b2e 1 a1c0 0", "3c5d 1 a1c0 0"}},
		{"under 4e2b at glance", Context(f, Query{Task: "4e2b"}), []string{"4e2b 0 a1c0 0", "9f31 1 4e2b 0", "c07d 1 4e2b 0"}},
		{"under 4e2b at detail", Context(f, Query{Task: "4e2b", Level: Detail}), []string{"a1c0 0  0 spine", "4e2b 1 a1c0 0", "9f31 2 4e2b 0", "c07d 2 4e2b 0", "7b2e 1 a1c0 0 sibling", "3c5d 1 a1c0 0 sibling"}},
		{"under 9f31 at detail", Context(f, Query{Task: "9f31", Level: Detail}), []string{"a1c0 0  2 spine", "4e2b 1 a1c0 0 spine", "9f31 2 4e2b 0", "c07d 2 4e2b 0 sibling"}},
		{"under the root at glance", Context(f, Query{Task: "a1c0"}), []string{"a1c0 0  0", "4e2b 1 a1c0 2", "7b2e 1 a1c0 0", "3c5d 1 a1c0 0"}},
		{"for ben at glance", Context(f, Query{Person: "ben@example.org"}), []string{"4e2b 0 a1c0 1", "c07d 1 4e2b 0"}},
		{"for ben at detail", Context(f, Query{Person: "ben@example.org", Level: Provenance}), []string{"a1c0 0  0 spine", "4e2b 1 a1c0 0", "9f31 2 4e2b 0 sibling", "c07d 2 4e2b 0", "7b2e 1 a1c0 0 sibling", "3c5d 1 a1c0 0 sibling"}},
		{"for ada at glance", Context(f, Query{Person: "ada@example.org"}), []string{"a1c0 0  3", "9f31 1 4e2b 0", "7b2e 1 a1c0 0"}},
		{"for ada at detail", Context(f, Query{Person: "ada@example.org", Level: Detail}), []string{"a1c0 0  0", "4e2b 1 a1c0 0 spine", "9f31 2 4e2b 0", "c07d 2 4e2b 0 sibling", "7b2e 1 a1c0 0", "3c5d 1 a1c0 0 sibling"}},
		{"for a stranger", Context(f, Query{Person: "eve@example.org", Level: Detail}), []string{}},
		{"for nobody", Context(f, Query{Level: Detail}), []string{}},
	} {
		if got := gridOf(c.got); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: the rows are %q; want %q", c.name, got, c.want)
		}
	}
	under := Context(f, Query{Task: "4e2b"})
	if under.Subject == nil || *under.Subject != (view.TaskRef{ID: "4e2b", Title: "Sensor node"}) || Context(f, Query{Person: "ben@example.org"}).Subject != nil || Tableau(f, Query{}).Subject != nil {
		t.Errorf("the subject under 4e2b is %+v, or a tableau without a task has one", under.Subject)
	}
	// A stranger's tableau holds every gate, no row and no count.
	if cols := columnsOf(Context(f, Query{Person: "eve@example.org", Window: 1})); len(cols) != 12 || cols[1] != "defined=0" {
		t.Errorf("a stranger's columns are %q", cols)
	}
	// A parent and a leaf say which they are.
	if v := Tableau(f, Query{Level: Detail}); !rowOf(t, v, "4e2b").Parent || rowOf(t, v, "9f31").Parent {
		t.Error("4e2b reads as a leaf, or 9f31 as a parent")
	}
}

// T10: the leaf at the end of a row's chain, and a row's provenance, which
// holds the fields of the task definition for the row's task.
func TestRowLeafAndProvenance(t *testing.T) {
	f, hashes := station(t, "main")
	firmware := labels(t, "weather-station.firmware")
	q := Query{Person: "ben@example.org", Window: 1, Level: Provenance}
	v := Tableau(f, q)
	for id, want := range map[string]*view.Leaf{
		"a1c0": {Task: view.TaskRef{ID: "3c5d", Title: "Dashboard"}},
		"4e2b": {Task: view.TaskRef{ID: "9f31", Title: "Sensor board"}},
		"c07d": {Task: view.TaskRef{ID: "f1a0", Title: "Weather node image"}, URL: "firmware"},
		"9f31": nil,
		"7b2e": nil,
	} {
		if got := rowOf(t, v, id).Leaf; !reflect.DeepEqual(got, want) {
			t.Errorf("the leaf of %s is %+v; want %+v", id, got, want)
		}
	}
	if from := rowOf(t, v, "a1c0").Status.From; from == nil || from.Kind != "rollup" || from.Task.ID != "3c5d" {
		t.Errorf("the status of a1c0 comes from %+v; want the direct child", from)
	}
	tie := Tableau(entry(t, "roll-up-tie"), Query{Level: Detail})
	if leaf := rowOf(t, tie, "e4a1").Leaf; leaf == nil || leaf.Task.ID != "b2c9" || leaf.URL != "" {
		t.Errorf("roll-up-tie: the leaf of the root is %+v; want the first child", leaf)
	}
	// A roll-up through a subproject names the leaf there.
	kinds := Tableau(entry(t, "junction-kinds"), Query{Level: Detail})
	if leaf := rowOf(t, kinds, "a500").Leaf; leaf == nil || leaf.Task.ID != "5c00" || leaf.URL != "sub" {
		t.Errorf("junction-kinds: the leaf of a500 is %+v; want 5c00 of the subproject", leaf)
	}

	for _, row := range v.Rows {
		if row.RowProvenance == nil {
			t.Fatalf("%s carries no provenance", row.ID)
		}
		task := Task(f, Query{Task: row.ID, Person: q.Person, Level: Provenance})
		if !reflect.DeepEqual(row.Chain, task.Chain) || !reflect.DeepEqual(row.Junctions, task.Junctions) || !reflect.DeepEqual(row.Sources, task.Sources) ||
			!reflect.DeepEqual(row.Linkages, task.Linkages) || !reflect.DeepEqual(row.StatusCommit, task.StatusCommit) || !reflect.DeepEqual(row.OwnCommit, task.OwnCommit) {
			t.Errorf("the provenance of the row of %s differs from the task definition's", row.ID)
		}
		if len(row.Junctions) != 11 || row.Junctions[0].From == nil {
			t.Errorf("%s holds %d junctions, the first from %v", row.ID, len(row.Junctions), row.Junctions[0].From)
		}
	}
	image := rowOf(t, v, "c07d")
	if image.StatusCommit == nil || image.StatusCommit.Hash != firmware["F2"] || image.OwnCommit == nil || image.OwnCommit.Hash != hashes["W11"] || len(image.Linkages) != 1 || len(image.Chain) != 2 {
		t.Errorf("c07d is decided by %+v and %+v, through %d linkages", image.StatusCommit, image.OwnCommit, len(image.Linkages))
	}
	want := "git log --format='%H %as %ae %ce %s' " + hashes["W13"] + " -- .tableaux/status"
	if len(v.Commands) != 1 || v.Commands[0].Text != want {
		t.Errorf("the commands are %+v; want %s", v.Commands, want)
	}
	// Below provenance a row carries none of it, and the view no command.
	if d := Tableau(f, Query{Level: Detail}); d.TableauProvenance != nil || d.Rows[0].RowProvenance != nil {
		t.Error("the tableau at detail carries provenance")
	}
}
