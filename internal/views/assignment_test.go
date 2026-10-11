package views

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// peopleOf returns the rows of an assignment as "email assigned
// contributes_next reviews_next models".
func peopleOf(a *view.Assignment) []string {
	out := []string{}
	for _, p := range a.People {
		out = append(out, strings.TrimSpace(fmt.Sprintf("%s %d %d %d %s", strings.TrimSuffix(p.Email, "@example.org"), p.Assigned, p.ContributesNext, p.ReviewsNext, strings.Join(p.Models, " "))))
	}
	return out
}

// T17: the assignment's people and counts.
func TestAssignmentPeople(t *testing.T) {
	f, _ := station(t, "main")
	all := Assignment(f, Query{Task: "a1c0"})
	if want := []string{"ada 3 2 0", "ben 2 0 0", "dan 1 1 0", "opus 0 0 0 claude-opus-5-5"}; !reflect.DeepEqual(peopleOf(all), want) {
		t.Errorf("the people are %v; want %v", peopleOf(all), want)
	}
	for i, p := range all.People {
		if p.Owner != (i == 0) || p.Models == nil {
			t.Errorf("%s reads owner %v with the models %#v", p.Email, p.Owner, p.Models)
		}
	}
	if !reflect.DeepEqual(all.Recursive, []string{"c07d"}) || all.AssignmentDetail != nil {
		t.Errorf("the recursive leaves are %v, with the detail %v", all.Recursive, all.AssignmentDetail)
	}
	if root := Assignment(f, Query{}); !reflect.DeepEqual(peopleOf(root), peopleOf(all)) {
		t.Errorf("an empty task gives %v", peopleOf(root))
	}
	for _, tc := range []struct {
		name string
		q    Query
		want []string
	}{
		{"ben alone", Query{Person: "ben@example.org"}, []string{"ben 2 0 0"}},
		{"the agent alone", Query{Person: "opus@example.org"}, []string{"opus 0 0 0 claude-opus-5-5"}},
		{"a stranger", Query{Person: "eve@example.org"}, []string{}},
		{"the subtree of 4e2b", Query{Task: "4e2b"}, []string{"ben 2 0 0", "ada 1 1 0", "opus 0 0 0 claude-opus-5-5"}},
		{"dan under 4e2b", Query{Task: "4e2b", Person: "dan@example.org"}, []string{}},
		{"a leaf", Query{Task: "3c5d"}, []string{"dan 1 1 0"}},
	} {
		for level := range levels {
			tc.q.Level = level
			got := Assignment(f, tc.q)
			if !reflect.DeepEqual(peopleOf(got), tc.want) || got.People == nil || got.Recursive == nil {
				t.Errorf("%s at %s: the people are %v; want %v", tc.name, levels[level], peopleOf(got), tc.want)
			}
			if level >= Detail && (got.Sections == nil || len(got.Sections) != len(got.People)) {
				t.Errorf("%s at %s: %d sections for %d people", tc.name, levels[level], len(got.Sections), len(got.People))
			}
		}
	}
	// The recursive leaves are those of the tasks in view, whoever is asked for.
	if got := Assignment(f, Query{Person: "dan@example.org"}).Recursive; !reflect.DeepEqual(got, []string{"c07d"}) {
		t.Errorf("for dan the recursive leaves are %v", got)
	}
	if got := Assignment(f, Query{Task: "3c5d"}).Recursive; len(got) != 0 {
		t.Errorf("under 3c5d the recursive leaves are %v", got)
	}
}

// T18: the assignment's lists. Every count is the length of a list the data
// holds, in every project the corpus builds.
func TestAssignmentLists(t *testing.T) {
	projects := 0
	for _, name := range built(t) {
		f := derived(t, load.Options{}, builtAt(t, name), "HEAD", "")
		if f.Refused() != nil {
			continue
		}
		projects++
		a := Assignment(f, Query{Level: Detail})
		if len(a.Sections) != len(a.People) {
			t.Errorf("%s: %d sections for %d people", name, len(a.Sections), len(a.People))
			continue
		}
		for i, s := range a.Sections {
			p, where := a.People[i], name+" "+a.People[i].Email
			next := func(rows []view.JunctionRow) (n int) {
				for _, r := range rows {
					if r.Stands == "next" {
						n++
					}
				}
				return n
			}
			if s.Email != p.Email || p.Assigned != len(s.Assigned) || p.ContributesNext != next(s.Contributes) || p.ReviewsNext != next(s.Reviews) {
				t.Errorf("%s: the row %+v counts other lists than its section holds", where, p)
			}
			var sum view.GateCount
			for j, c := range s.Counts {
				if c.Gate != f.Gates()[j] {
					t.Errorf("%s: the counts run %s at %d", where, c.Gate, j)
				}
				at := func(rows []view.JunctionRow, onlyNext bool) (n int) {
					for _, r := range rows {
						if r.Gate == c.Gate && (!onlyNext || r.Stands == "next") {
							n++
						}
					}
					return n
				}
				standing := 0
				for _, row := range s.Assigned {
					if row.Status.Gate == c.Gate {
						standing++
					}
				}
				if c.Standing != standing || c.Contributes != at(s.Contributes, false) || c.ContributesNext != at(s.Contributes, true) ||
					c.Reviews != at(s.Reviews, false) || c.ReviewsNext != at(s.Reviews, true) {
					t.Errorf("%s: the counts at %s are %+v", where, c.Gate, c)
				}
				sum.Standing, sum.Contributes, sum.Reviews = sum.Standing+c.Standing, sum.Contributes+c.Contributes, sum.Reviews+c.Reviews
				sum.ContributesNext, sum.ReviewsNext = sum.ContributesNext+c.ContributesNext, sum.ReviewsNext+c.ReviewsNext
			}
			if len(s.Counts) != len(f.Gates()) || sum.Contributes != len(s.Contributes) || sum.Reviews != len(s.Reviews) ||
				sum.ContributesNext != p.ContributesNext || sum.ReviewsNext != p.ReviewsNext || sum.Standing > len(s.Assigned) {
				t.Errorf("%s: the counts sum to %+v over %d assigned, %d contributions and %d reviews", where, sum, len(s.Assigned), len(s.Contributes), len(s.Reviews))
			}
			models, junctions := []string{}, 0
			for _, m := range s.Models {
				junctions += m.Junctions
				if !has(models, m.Model) {
					models = append(models, m.Model)
				}
				if m.Model == "" || m.StatedBy == "" || m.Next > m.Junctions {
					t.Errorf("%s: the model row is %+v", where, m)
				}
			}
			stating := 0
			for _, r := range s.Contributes {
				if r.Model != "" {
					stating++
				}
				if r.Contributor != p.Email || r.Kind != "plain" || !f.Leaf(r.Task.ID) {
					t.Errorf("%s: contributes at %s %s, which is %+v", where, r.Task.ID, r.Gate, r.Junction)
				}
			}
			for _, r := range s.Reviews {
				if r.Reviewer != p.Email || r.Kind != "plain" || !f.Leaf(r.Task.ID) {
					t.Errorf("%s: reviews at %s %s, which is %+v", where, r.Task.ID, r.Gate, r.Junction)
				}
			}
			if junctions != stating || !reflect.DeepEqual(models, p.Models) {
				t.Errorf("%s: the models %v count %d junctions; the row reads %v over %d", where, models, junctions, p.Models, stating)
			}
			for _, row := range s.Assigned {
				if (row.Next == nil) != (row.NextGate == "") || row.Next != nil && (row.Next.Gate != row.NextGate || row.Next.Stands != "next") {
					t.Errorf("%s: %s reads the next gate %q with %+v", where, row.ID, row.NextGate, row.Next)
				}
			}
			for _, r := range s.Recursive {
				if !has(a.Recursive, r.Task.ID) {
					t.Errorf("%s: %s is recursive in the section alone", where, r.Task.ID)
				}
			}
			if s.SectionProvenance != nil {
				t.Errorf("%s: the detail holds positions", where)
			}
		}
	}
	if projects < 90 {
		t.Errorf("the corpus builds %d projects the Derivation reads", projects)
	}

	f, _ := station(t, "main")
	a := Assignment(f, Query{Level: Detail})
	section := map[string]view.PersonSection{}
	for _, s := range a.Sections {
		section[s.Email] = s
	}
	ben, ada := section["ben@example.org"], section["ada@example.org"]
	var reviews []string
	for _, r := range ben.Reviews {
		reviews = append(reviews, r.Task.ID+" "+r.Gate+" "+r.Stands)
	}
	if want := []string{"9f31 mockup passed", "c07d mockup passed", "c07d unit later", "9f31 validate later"}; !reflect.DeepEqual(reviews, want) {
		t.Errorf("ben reviews %v; want %v, his own mockup on c07d among them", reviews, want)
	}
	mine := false
	for _, r := range ben.Contributes {
		mine = mine || r.Task.ID == "c07d" && r.Gate == "mockup"
	}
	if !mine {
		t.Error("ben does not contribute the mockup he reviews on c07d")
	}
	for _, c := range ben.Counts {
		if want := map[string]int{"function": 1, "design": 1}[c.Gate]; c.Standing != want {
			t.Errorf("ben stands %d at %s; want %d: 4e2b at function by its roll-up, c07d at design", c.Standing, c.Gate, want)
		}
	}
	if want := []view.Subtree{{Task: view.TaskRef{ID: "4e2b", Title: "Sensor node"}, Descendants: 2}}; !reflect.DeepEqual(ben.Authority, want) {
		t.Errorf("ben's authority is %+v; want %+v", ben.Authority, want)
	}
	if want := []view.Subtree{{Task: view.TaskRef{ID: "a1c0", Title: "Weather station"}, Descendants: 5}}; !reflect.DeepEqual(ada.Authority, want) {
		t.Errorf("ada's authority is %+v; want %+v", ada.Authority, want)
	}
	var assigned []string
	for _, row := range ben.Assigned {
		assigned = append(assigned, row.ID+" under "+row.Under+" at "+row.Status.Gate+" next "+row.NextGate)
	}
	if want := []string{"4e2b under a1c0 at function next ", "c07d under 4e2b at design next implementation"}; !reflect.DeepEqual(assigned, want) {
		t.Errorf("ben is assigned %v; want %v", assigned, want)
	}
	if len(ben.Recursive) != 1 || ben.Recursive[0].Task.ID != "c07d" || ben.Recursive[0].Gate != "implementation" || ben.Recursive[0].Subproject.URL != "firmware" ||
		ben.Recursive[0].Status == nil || ben.Recursive[0].Status.Gate != "design" || len(ada.Recursive) != 0 {
		t.Errorf("ben's recursive rows are %+v", ben.Recursive)
	}
	if want := []view.ModelRow{{Gate: "unit", Model: "claude-opus-5-5", StatedBy: "c07d", Junctions: 1}}; !reflect.DeepEqual(section["opus@example.org"].Models, want) {
		t.Errorf("the agent's models are %+v; want %+v", section["opus@example.org"].Models, want)
	}

	// An agent's models group by gate, model and the task that states it, and
	// a task assigned beneath a task of the same email is no second subtree.
	kinds := Assignment(entry(t, "junction-kinds"), Query{Level: Detail})
	for _, s := range kinds.Sections {
		if s.Email != "bot@example.org" {
			continue
		}
		var rows []string
		for _, m := range s.Models {
			rows = append(rows, fmt.Sprintf("%s %s %s %d", m.Gate, m.Model, m.StatedBy, m.Junctions))
		}
		if want := []string{"design claude-fable a110 1", "design claude-fable-5-1 a300 1", "unit claude-sonnet a110 1"}; !reflect.DeepEqual(rows, want) {
			t.Errorf("junction-kinds: the agent's models are %v; want %v", rows, want)
		}
	}
	nested := mem(t, map[string]string{
		"tasks/r000.yaml": leafOf("r@x", "", 0, ""),
		"tasks/p000.yaml": leafOf("r@x", "r000", 1, ""),
		"tasks/a000.yaml": leafOf("a@x", "p000", 1, ""),
	})
	if got := Assignment(nested, Query{Level: Detail}).Sections[0].Authority; len(got) != 1 || got[0].Task.ID != "r000" || got[0].Descendants != 2 {
		t.Errorf("the authority of a nested assignee is %+v; want the root alone", got)
	}
	if got := Assignment(nested, Query{Task: "p000", Level: Detail}).Sections[0].Authority; len(got) != 1 || got[0].Task.ID != "p000" || got[0].Descendants != 1 {
		t.Errorf("under p000 the authority is %+v; want p000, whose ancestor is out of view", got)
	}
}

// T19: the positions.
func TestAssignmentPositions(t *testing.T) {
	for _, name := range []string{"weather-station", "junction-kinds"} {
		f := entry(t, name)
		if detail := Assignment(f, Query{Level: Detail}); len(detail.Sections) == 0 || detail.Sections[0].SectionProvenance != nil {
			t.Errorf("%s: the detail holds positions", name)
		}
		for _, s := range Assignment(f, Query{Level: Provenance}).Sections {
			where := name + " " + s.Email
			if s.SectionProvenance == nil || s.Positions == nil {
				t.Fatalf("%s: the provenance holds no positions", where)
			}
			kinds := ""
			groups := map[string]int{} // kind, gate and task: the groups that hold it
			for _, p := range s.Positions {
				if !strings.HasSuffix(kinds, p.Kind+" ") {
					kinds += p.Kind + " "
				}
				for _, task := range p.Tasks {
					groups[p.Kind+" "+p.Gate+" "+task]++
				}
				if len(p.Tasks) == 0 {
					t.Errorf("%s: a position holds no task: %+v", where, p)
				}
				for _, from := range []view.Stated{p.From, p.ReviewerFrom} {
					switch from.By {
					case "", "default":
						if from != (view.Stated{By: from.By}) {
							t.Errorf("%s: a source reads %+v", where, from)
						}
					case "own", "assignee":
						if from != (view.Stated{By: from.By, Key: "assignee"}) {
							t.Errorf("%s: a source reads %+v", where, from)
						}
					default:
						// A task states it: the line of its file holds the key's last word.
						task := f.Project().Tasks[from.By]
						if task == nil || from.File != ".tableaux/tasks/"+from.By+".yaml" || from.Line < 1 || from.Col < 1 {
							t.Errorf("%s: a source reads %+v", where, from)
							continue
						}
						word := from.Key[strings.LastIndex(from.Key, ".")+1:]
						if strings.HasSuffix(from.Key, ".subproject") {
							word = p.Gate
						}
						if line := lineOf(t, f, from); !strings.Contains(line, word+":") {
							t.Errorf("%s: %s points at the line %q", where, from.Key, line)
						}
					}
				}
			}
			if !inOrder(kinds) {
				t.Errorf("%s: the kinds run %q", where, kinds)
			}
			// Every row of contributes and reviews falls in one group.
			for kind, rows := range map[string][]view.JunctionRow{"contributes": s.Contributes, "reviews": s.Reviews} {
				for _, r := range rows {
					key := kind + " " + r.Gate + " " + r.Task.ID
					if groups[key] != 1 {
						t.Errorf("%s: %s falls in %d groups", where, key, groups[key])
					}
					delete(groups, key)
				}
			}
			for _, row := range s.Assigned {
				if groups["assignee  "+row.ID] != 1 {
					t.Errorf("%s: the assignment of %s falls in %d groups", where, row.ID, groups["assignee  "+row.ID])
				}
				delete(groups, "assignee  "+row.ID)
			}
			for _, a := range s.Authority {
				delete(groups, "authority  "+a.Task.ID)
			}
			for _, r := range s.Recursive {
				delete(groups, "recursive "+r.Gate+" "+r.Task.ID)
			}
			if len(groups) != 0 {
				t.Errorf("%s: positions the lists do not hold: %v", where, groups)
			}
		}
	}

	f, _ := station(t, "main")
	positions := map[string][]view.Position{}
	for _, s := range Assignment(f, Query{Level: Provenance}).Sections {
		positions[s.Email] = s.Positions
	}
	find := func(email, kind, gate string) view.Position {
		t.Helper()
		for _, p := range positions[email] {
			if p.Kind == kind && p.Gate == gate {
				return p
			}
		}
		t.Fatalf("%s holds no position %s at %q", email, kind, gate)
		return view.Position{}
	}
	assignee := view.Stated{By: "assignee", Key: "assignee"}
	if p := find("ben@example.org", "assignee", ""); !reflect.DeepEqual(p.Tasks, []string{"4e2b", "c07d"}) || p.From != (view.Stated{By: "own", Key: "assignee"}) {
		t.Errorf("ben's assignments are %+v", p)
	}
	if p := find("ben@example.org", "authority", ""); !reflect.DeepEqual(p.Tasks, []string{"4e2b"}) || p.From != (view.Stated{By: "4e2b", Key: "assignee", File: ".tableaux/tasks/4e2b.yaml", Line: 6, Col: 11}) {
		t.Errorf("ben's authority is %+v", p)
	}
	// An assignee's default reads assignee; a reviewer an ancestor states names its entry.
	if p := find("ben@example.org", "contributes", "mockup"); p.From != assignee || p.Reviewer != "ben@example.org" ||
		p.ReviewerFrom != (view.Stated{By: "4e2b", Key: "junctions.mockup.reviewer", File: ".tableaux/tasks/4e2b.yaml", Line: 10, Col: 23}) {
		t.Errorf("ben's contribution at mockup is %+v", p)
	}
	if p := find("ben@example.org", "reviews", "mockup"); !reflect.DeepEqual(p.Tasks, []string{"9f31", "c07d"}) || p.From != assignee || p.ReviewerFrom.By != "4e2b" {
		t.Errorf("ben's reviews at mockup are %+v; want one group of two", p)
	}
	if p := find("ben@example.org", "reviews", "unit"); p.ReviewerFrom != assignee || p.From.By != "c07d" || p.From.Key != "junctions.unit.contributor" {
		t.Errorf("ben's review at unit is %+v; want the assignee's, of the agent c07d states", p)
	}
	if p := find("ben@example.org", "recursive", "implementation"); p.URL != "firmware" || len(p.Pin) != 40 || !reflect.DeepEqual(p.Tasks, []string{"c07d"}) ||
		p.From != (view.Stated{By: "c07d", Key: "junctions.implementation.subproject", File: ".tableaux/tasks/c07d.yaml", Line: 9, Col: 3}) {
		t.Errorf("ben's recursive position is %+v", p)
	}
	if p := find("ada@example.org", "contributes", "function"); !reflect.DeepEqual(p.Tasks, []string{"9f31", "7b2e"}) || p.From != assignee || p.ReviewerFrom != (view.Stated{By: "default"}) || p.Reviewer != "" {
		t.Errorf("ada's contributions at function are %+v; want one group by the plain default", p)
	}
	if p := find("opus@example.org", "contributes", "unit"); p.From.By != "c07d" || p.From.Line != 10 || p.Reviewer != "ben@example.org" || p.ReviewerFrom != assignee {
		t.Errorf("the agent's contribution is %+v", p)
	}
}

// inOrder reports whether the kinds of a section's positions run in the
// order of the design, each at most once.
func inOrder(kinds string) bool {
	at := 0
	all := []string{"assignee", "authority", "contributes", "reviews", "recursive"}
	for _, kind := range strings.Fields(kinds) {
		for at < len(all) && all[at] != kind {
			at++
		}
		if at == len(all) {
			return false
		}
		at++
	}
	return true
}

// built returns every repository the corpus builds, an entry or a subproject
// of one, by name.
func built(t testing.TB) []string {
	t.Helper()
	list, err := os.ReadDir(corpus(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// lineOf returns the line of a project file that a source names, from the
// project as the Loader read it.
func lineOf(t testing.TB, f *derive.Facts, from view.Stated) string {
	t.Helper()
	dir := f.Project().Where.Top
	if dir == "" {
		t.Fatalf("the project has no working tree to read %s from", from.File)
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Project().Where.Dir), filepath.FromSlash(from.File)))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	if from.Line > len(lines) {
		t.Fatalf("%s has no line %d", from.File, from.Line)
	}
	return lines[from.Line-1]
}
