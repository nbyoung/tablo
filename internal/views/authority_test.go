package views

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// rowsOf returns the rows of an authority delegation as "id depth".
func rowsOf(d *view.Delegation) []string {
	out := []string{}
	for _, row := range d.Rows {
		out = append(out, row.ID+" "+string(rune('0'+row.Depth)))
	}
	return out
}

// T14: the authority rows and the two filters.
func TestAuthorityRows(t *testing.T) {
	f, _ := station(t, "main")
	all := Authority(f, Query{Task: "a1c0"})
	if all.DelegationDetail != nil || all.DelegationProvenance != nil {
		t.Error("the glance holds the keys of a level below")
	}
	if want := strings.Split("a1c0 0,4e2b 1,9f31 2,c07d 2,7b2e 1,3c5d 1", ","); !reflect.DeepEqual(rowsOf(all), want) {
		t.Errorf("the rows are %v; want %v", rowsOf(all), want)
	}
	want := []view.DelegationRow{
		{TaskRef: view.TaskRef{ID: "a1c0", Title: "Weather station"}, Assignee: "ada@example.org", Parent: true},
		{TaskRef: view.TaskRef{ID: "4e2b", Title: "Sensor node"}, Depth: 1, Assignee: "ben@example.org", Parent: true, Delegated: true},
		{TaskRef: view.TaskRef{ID: "9f31", Title: "Sensor board"}, Depth: 2, Assignee: "ada@example.org", Delegated: true},
		{TaskRef: view.TaskRef{ID: "c07d", Title: "Node firmware"}, Depth: 2, Assignee: "ben@example.org"},
		{TaskRef: view.TaskRef{ID: "7b2e", Title: "Gateway"}, Depth: 1, Assignee: "ada@example.org"},
		{TaskRef: view.TaskRef{ID: "3c5d", Title: "Dashboard"}, Depth: 1, Assignee: "dan@example.org", Delegated: true, Proposed: true},
	}
	if !reflect.DeepEqual(all.Rows, want) {
		t.Errorf("the rows are %+v; want %+v", all.Rows, want)
	}
	if root := Authority(f, Query{}); !reflect.DeepEqual(root.Rows, all.Rows) {
		t.Errorf("an empty task gives %v; want the root's rows", rowsOf(root))
	}
	for _, tc := range []struct {
		name string
		q    Query
		want string
	}{
		{"the subtree of 4e2b", Query{Task: "4e2b"}, "4e2b 0,9f31 1,c07d 1"},
		{"the owner", Query{Person: "ada@example.org"}, "a1c0 0,4e2b 1,9f31 2,c07d 2,7b2e 1,3c5d 1"},
		{"ben", Query{Person: "ben@example.org"}, "a1c0 0,4e2b 1,9f31 2,c07d 2"},
		{"dan", Query{Person: "dan@example.org"}, "a1c0 0,3c5d 1"},
		{"a stranger", Query{Person: "eve@example.org"}, ""},
		{"an agent, who is no assignee", Query{Person: "opus@example.org"}, ""},
		{"ben under 9f31", Query{Task: "9f31", Person: "ben@example.org"}, "9f31 0"},
		{"dan under 4e2b", Query{Task: "4e2b", Person: "dan@example.org"}, ""},
		{"the proposed", Query{Proposed: true}, "a1c0 0,3c5d 1"},
		{"the proposed of ben", Query{Proposed: true, Person: "ben@example.org"}, ""},
		{"the proposed of dan", Query{Proposed: true, Person: "dan@example.org"}, "a1c0 0,3c5d 1"},
		{"the proposed under 4e2b", Query{Task: "4e2b", Proposed: true}, ""},
	} {
		for level := range levels {
			tc.q.Level = level
			got := Authority(f, tc.q)
			want := []string{}
			if tc.want != "" {
				want = strings.Split(tc.want, ",")
			}
			if !reflect.DeepEqual(rowsOf(got), want) {
				t.Errorf("%s at %s: the rows are %v; want %v", tc.name, levels[level], rowsOf(got), want)
			}
			if got.Rows == nil {
				t.Errorf("%s: the rows are null", tc.name)
			}
		}
	}
	// A filter keeps an ancestor as it stands: the root is not proposed.
	if proposed := Authority(f, Query{Proposed: true}); proposed.Rows[0].Proposed || !proposed.Rows[1].Proposed {
		t.Errorf("the proposed rows are %+v", proposed.Rows)
	}

	// Off the trunk every row reads proposed.
	branch, _ := station(t, "sensor-board")
	off := Authority(branch, Query{Level: Provenance})
	if len(off.Rows) != 5 {
		t.Fatalf("at sensor-board the rows are %v; want five", rowsOf(off))
	}
	for _, row := range off.Rows {
		// The files at W3 equal the trunk's: no row differs from it.
		if !row.Proposed || row.DiffersFromTrunk || row.Uncommitted {
			t.Errorf("at sensor-board %s reads %+v", row.ID, row)
		}
	}
	if !reflect.DeepEqual(off.Undecided, []view.Undecided{{Why: "off_trunk", Tasks: []string{"a1c0", "4e2b", "9f31", "c07d", "7b2e"}}}) || len(off.Deciding) != 0 || off.Deciding == nil {
		t.Errorf("at sensor-board the undecided are %+v and the deciding %+v", off.Undecided, off.Deciding)
	}
	if proposed := Authority(branch, Query{Proposed: true}); len(proposed.Rows) != 5 {
		t.Errorf("at sensor-board the proposed are %v", rowsOf(proposed))
	}
}

// T15: the deciding commits.
func TestAuthorityDeciding(t *testing.T) {
	f, hashes := station(t, "main")
	d := Authority(f, Query{Level: Provenance})
	type group struct {
		commit, way, merged, by, email string
		tasks                          string
	}
	groups := func(d *view.Delegation) []group {
		var out []group
		for _, g := range d.Deciding {
			out = append(out, group{g.Commit.Hash, g.Way, g.Merged, g.By, g.Email, strings.Join(g.Tasks, " ")})
		}
		return out
	}
	want := []group{
		{hashes["W12"], "change", "", "", "dan@example.org", "3c5d"},
		{hashes["W4"], "merge", hashes["W3"], "both", "ben@example.org", "9f31"},
		{hashes["W2"], "change", "", "both", "ben@example.org", "c07d"},
		{hashes["W1"], "change", "", "both", "ada@example.org", "a1c0 4e2b 7b2e"},
	}
	if got := groups(d); !reflect.DeepEqual(got, want) {
		t.Errorf("the deciding commits are %+v; want %+v", got, want)
	}
	if d.Undecided == nil || len(d.Undecided) != 0 {
		t.Errorf("the undecided are %#v; want []", d.Undecided)
	}
	if want := []view.Command{{Text: "git log --first-parent --format='%H %as %ae %ce %P' " + hashes["W13"] + " -- .tableaux/tasks"}}; !reflect.DeepEqual(d.Commands, want) {
		t.Errorf("the commands are %+v; want %+v", d.Commands, want)
	}
	// A filter narrows the groups with the rows.
	if got := groups(Authority(f, Query{Person: "dan@example.org", Level: Provenance})); len(got) != 2 || got[0].tasks != "3c5d" || got[1].tasks != "a1c0" {
		t.Errorf("dan's deciding commits are %+v", got)
	}

	// The cases of design 27a3: a committer's hand, a trailer from no
	// authority, a hand-over and a trailer from the new owner.
	repo, labelled, _ := cases(t)
	at := func(label string) map[string]group {
		t.Helper()
		d := Authority(derived(t, load.Options{}, repo, labelled[label], ""), Query{Level: Provenance})
		if len(d.Undecided) != 0 {
			t.Errorf("at %s the undecided are %+v", label, d.Undecided)
		}
		out := map[string]group{}
		for _, g := range groups(d) {
			for _, task := range strings.Fields(g.tasks) {
				out[task] = g
			}
		}
		return out
	}
	if g := at("C3")["c3d7"]; g != (group{labelled["C3"], "merge", labelled["C2"], "both", "olive@example.org", "c3d7"}) {
		t.Errorf("at C3 a merge decides c3d7 as %+v", g)
	}
	if g := at("C4")["b2c9"]; g != (group{labelled["C4"], "change", "", "committer", "olive@example.org", "b2c9"}) {
		t.Errorf("at C4 the committer's hand decides b2c9 as %+v", g)
	}
	if g := at("C5")["c3d7"]; g != (group{labelled["C5"], "trailer", "", "", "pat@example.org", "c3d7"}) {
		t.Errorf("at C5 a trailer from no authority decides c3d7 as %+v", g)
	}
	if g := at("C6")["e4a1"]; g != (group{labelled["C6"], "change", "", "both", "olive@example.org", "e4a1"}) {
		t.Errorf("at C6 the hand-over decides the root as %+v; want the outgoing owner", g)
	}
	if g := at("C7")["c3d7"]; g != (group{labelled["C7"], "trailer", "", "both", "pat@example.org", "c3d7"}) {
		t.Errorf("at C7 the new owner's trailer decides c3d7 as %+v", g)
	}
	// The task definition states the same authorisation: the committer's hand.
	if a := Task(derived(t, load.Options{}, repo, labelled["C4"], ""), Query{Task: "b2c9", Level: Provenance}).Authorisation; a.Commit == nil || a.Commit.Hash != labelled["C4"] ||
		a.Way != "change" || a.By != "committer" || a.Email != "olive@example.org" || !reflect.DeepEqual(a.Judges, []string{"olive@example.org"}) || a.Why != "" ||
		a.Commit.Author.Email != "pat@example.org" {
		t.Errorf("at C4 the authorisation of b2c9 is %+v", a)
	}
	// Newest first: at C7 the groups run C7, C6, C4.
	var order []string
	for _, g := range groups(Authority(derived(t, load.Options{}, repo, labelled["C7"], ""), Query{Level: Provenance})) {
		order = append(order, g.commit)
	}
	if want := []string{labelled["C7"], labelled["C6"], labelled["C4"]}; !reflect.DeepEqual(order, want) {
		t.Errorf("at C7 the deciding commits run %v; want %v", order, want)
	}
	// A branch off the trunk: no commit decides.
	if side := Authority(derived(t, load.Options{}, repo, labelled["C2"], ""), Query{Level: Provenance}); len(side.Deciding) != 0 || len(side.Undecided) != 1 || side.Undecided[0].Why != "off_trunk" || len(side.Undecided[0].Tasks) != 3 {
		t.Errorf("at C2 the undecided are %+v", side.Undecided)
	}
}

// T16: chains and defaults.
func TestAuthorityChains(t *testing.T) {
	f, _ := station(t, "main")
	d := Authority(f, Query{Level: Detail})
	if d.DelegationProvenance != nil || len(d.Parents) != 2 || d.Parents[0].ID != "a1c0" || d.Parents[1].ID != "4e2b" {
		t.Fatalf("the parents are %+v", d.Parents)
	}
	root := d.Parents[0]
	if !reflect.DeepEqual(root.Authorities, []view.Authority{{Email: "ada@example.org", By: "a1c0"}}) || !reflect.DeepEqual(root.Children, []string{"4e2b", "7b2e", "3c5d"}) ||
		root.Defaults == nil || len(root.Defaults) != 0 || root.Sources == nil || len(root.Sources) != 0 {
		t.Errorf("the root gives %+v", root)
	}
	node := d.Parents[1]
	if !reflect.DeepEqual(node.Authorities, []view.Authority{{Email: "ben@example.org", By: "4e2b"}, {Email: "ada@example.org", By: "a1c0"}}) ||
		!reflect.DeepEqual(node.Children, []string{"9f31", "c07d"}) || node.Assignee != "ben@example.org" {
		t.Errorf("4e2b gives %+v", node)
	}
	if len(node.Defaults) != 1 {
		t.Fatalf("4e2b has the defaults %+v", node.Defaults)
	}
	if j := node.Defaults[0]; j.Gate != "mockup" || j.Kind != "plain" || j.Reviewer != "ben@example.org" || j.ReviewerByDefault || j.Stands != "" ||
		!reflect.DeepEqual(j.Marks, []string{"person"}) || !reflect.DeepEqual(j.From, []string{"4e2b"}) {
		t.Errorf("the default of 4e2b is %+v", j)
	}
	if want := []view.Supplier{{Gate: "mockup", Field: "reviewer", Value: "ben@example.org",
		Stated: view.Stated{By: "4e2b", Key: "junctions.mockup.reviewer", File: ".tableaux/tasks/4e2b.yaml", Line: 10, Col: 23}}}; !reflect.DeepEqual(node.Sources, want) {
		t.Errorf("the default of 4e2b comes from %+v", node.Sources)
	}
	// A filter keeps the parents of its rows alone.
	if dan := Authority(f, Query{Person: "dan@example.org", Level: Detail}); len(dan.Parents) != 1 || dan.Parents[0].ID != "a1c0" || len(dan.Parents[0].Children) != 3 {
		t.Errorf("dan's parents are %+v; want the root with every child", dan.Parents)
	}

	// junction-kinds: a parent's exemption, and a reviewer an entry states.
	kinds := Authority(entry(t, "junction-kinds"), Query{Level: Detail})
	parents := map[string]view.Chain{}
	for _, p := range kinds.Parents {
		parents[p.ID] = p
	}
	exempt := parents["a200"]
	if len(exempt.Defaults) != 1 || exempt.Defaults[0].Gate != "release" || exempt.Defaults[0].Kind != "not_applicable" || !reflect.DeepEqual(exempt.Defaults[0].Marks, []string{"exempt"}) ||
		len(exempt.Sources) != 1 || exempt.Sources[0].Field != "applies" || exempt.Sources[0].Value != "false" || exempt.Sources[0].By != "a200" || exempt.Sources[0].Line == 0 {
		t.Errorf("a200 gives the defaults %+v from %+v; want its exemption at release", exempt.Defaults, exempt.Sources)
	}
	stated := parents["a100"]
	if len(stated.Defaults) != 1 || !reflect.DeepEqual(stated.Defaults[0].Marks, []string{"person", "reviewer"}) || stated.Defaults[0].Reviewer != "olive@example.org" {
		t.Errorf("a100 gives the defaults %+v; want a reviewer its entry states", stated.Defaults)
	}

	// An agent's default: the parent's own assignee takes the review of the
	// parent's junction, and no child inherits that review, so the default
	// carries no reviewer mark (decision 13). A reviewer an entry states
	// shows. The corpus states no agent on a parent.
	agents := mem(t, map[string]string{
		"tasks/r000.yaml": leafOf("r@x", "", 0, "{ design: { contributor: bot@x, model: m }, release: { contributor: bot@x, model: m, reviewer: rev@x } }"),
		"tasks/a000.yaml": leafOf("a@x", "r000", 1, ""),
	})
	p := Authority(agents, Query{Level: Provenance}).Parents[0]
	if len(p.Defaults) != 2 {
		t.Fatalf("the agent's defaults are %+v", p.Defaults)
	}
	if j := p.Defaults[0]; j.Gate != "design" || !reflect.DeepEqual(j.Marks, []string{"agent"}) || j.Model != "m" || j.Reviewer != "r@x" || !j.ReviewerByDefault {
		t.Errorf("the agent's default at design is %+v; want the mark agent alone", j)
	}
	if j := p.Defaults[1]; j.Gate != "release" || !reflect.DeepEqual(j.Marks, []string{"agent", "reviewer"}) || j.ReviewerByDefault {
		t.Errorf("the agent's default at release is %+v; want the reviewer its entry states", j)
	}
	var fields []string
	for _, s := range p.Sources {
		fields = append(fields, s.Gate+" "+s.Field+" "+s.By)
	}
	if want := []string{"design contributor r000", "design model r000", "design reviewer assignee", "release contributor r000", "release model r000", "release reviewer r000"}; !reflect.DeepEqual(fields, want) {
		t.Errorf("the agent's defaults come from %v; want %v", fields, want)
	}
	// The leaf beneath keeps the marks of its resolved junction.
	if leaf := Task(agents, Query{Task: "a000", Level: Detail}).Junctions; !reflect.DeepEqual(leaf[3].Marks, []string{"agent", "reviewer"}) || leaf[3].Gate != "design" {
		t.Errorf("the leaf's junction at design is %+v", leaf[3])
	}
}

// levels names the three levels as an envelope's params.level does.
var levels = map[Level]string{Glance: "glance", Detail: "detail", Provenance: "provenance"}
