package derive

import (
	"reflect"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// refusals lists the eighteen entries of cases.md by the part the Derivation
// cannot read.
var refusals = map[string]string{
	"no-tableaux-directory":  "project",
	"version-missing":        "version",
	"version-bad-pattern":    "version",
	"version-major-mismatch": "version",
	"version-minor-ahead":    "version",

	"gates-missing":             "gates",
	"gates-only-undefined":      "gates",
	"gates-first-not-undefined": "gates",
	"gates-duplicate-gate":      "gates",

	"gates-no-states":                 "states",
	"gates-negative-severity":         "states",
	"gates-duplicate-state":           "states",
	"gates-no-undefined-state":        "states",
	"gates-complete-nonzero-severity": "states",

	"tree-no-root":        "tree",
	"tree-two-roots":      "tree",
	"tree-parent-missing": "tree",
	"tree-parent-cycle":   "tree",
}

// exercise calls every method of the facts with every task, gate and link
// the project names and with ones it lacks, and follows each link. A fault
// is a panic.
func exercise(f *Facts, depth int) int {
	calls := 0
	p := f.Project()
	_, _, _, _, _ = f.Log(), f.Trunk(), f.OnTrunk(), f.Root(), f.Owner()
	ids := append(f.Order(), "zzzz", "")
	if p != nil {
		ids = append(ids, p.TaskIDs()...)
		for id := range p.Statuses {
			ids = append(ids, id)
		}
	}
	gates := append(f.Gates(), "nowhere", "")
	for _, id := range ids {
		_, _, _ = f.Children(id), f.Leaf(id), f.Authorities(id)
		_, _, _, _ = f.Junctions(id), f.Applicable(id), f.First(id), f.Last(id)
		_, _, _ = f.Status(id), f.Chain(id), f.Snapshots(id)
		for _, c := range f.Requires(id) {
			_ = c.Word()
		}
		_ = f.Dependents(id)
		_, _, _, _ = f.Authorisation(id), f.Reviews(id), f.Handoff(id), f.Events(id)
		if log := f.Log(); log != nil {
			_, _ = f.AuthorisationAt(id, log.Source), f.AuthorisationAt(id, "none")
		}
		for _, gate := range gates {
			_ = f.Accepted(id, gate)
			if j := f.Junction(id, gate); j != nil {
				_, _ = j.Marks(), j.Sources()
			}
			_ = f.Next(id, gate)
			_, _ = f.Index(gate), f.Severity(gate)
			calls++
		}
	}
	_, _ = f.History(), f.Unread()
	if p == nil || depth > 3 {
		return calls
	}
	if log := f.Log(); log != nil && depth == 0 {
		// The past gives every fact the files alone give, at any commit.
		for _, c := range log.Commits {
			if past := f.At(c.ID); past != nil {
				calls += exercise(past, 3)
			}
		}
	}
	for _, link := range append(p.Links, nil, &model.Link{}) {
		_ = f.Pin(link)
		if sub := f.Sub(link); sub != nil {
			calls += exercise(sub, depth+1)
		}
	}
	return calls
}

// T21: refusal and tolerance. The Derivation refuses the eighteen entries
// by their part and derives every fact of the other built repositories
// without a fault, whatever their errors (decision 6).
func TestRefusalAndTolerance(t *testing.T) {
	names := built(t)
	if len(names) < 100 {
		t.Errorf("the corpus builds %d repositories; it held 119 at the design", len(names))
	}
	refused, calls := 0, 0
	for _, name := range names {
		f := derived(t, load.Options{}, builtAt(t, name), "HEAD", "")
		part, refuses := refusals[name]
		switch r := f.Refused(); {
		case refuses && (r == nil || r.Part != part || len(r.Rules) == 0):
			t.Errorf("%s: the refusal is %+v; want the part %s", name, r, part)
		case !refuses && r != nil:
			t.Errorf("%s: refused for its %s; want every fact to stand", name, r.Part)
		}
		if refuses {
			refused++
			if f.Root() != "" || f.Owner() != "" || f.Order() != nil || f.Gates() != nil || f.Status("b2c9") != nil ||
				f.Junction("b2c9", "design") != nil || f.Requires("b2c9") != nil || f.Applicable("b2c9") != nil || f.Sub(nil) != nil {
				t.Errorf("%s: a refused project gives a fact", name)
			}
		}
		calls += exercise(f, 0)
	}
	if refused != len(refusals) {
		t.Errorf("the corpus holds %d of the %d entries the Derivation refuses", refused, len(refusals))
	}
	t.Logf("%d repositories, %d refused, %d junctions asked for", len(names), refused, calls)
}

// T21, the refusals the corpus has no entry for, and what does not refuse.
func TestRefusalParts(t *testing.T) {
	root := leafOf("r@x", "", 0, "")
	for name, tc := range map[string]struct {
		files map[string]string
		part  string
	}{
		"a version that is no scalar":  {map[string]string{"version.yaml": "tableaux: [0, 3]\n"}, "version"},
		"a gate key stated twice":      {map[string]string{"gates.yaml": "gates:\n  - { key: undefined }\n  - { key: a }\n  - { key: a }\nstates: []\n"}, "gates"},
		"a gate key that is empty":     {map[string]string{"gates.yaml": "gates:\n  - { key: undefined }\n  - { key: \"\" }\n"}, "gates"},
		"a gate key that is no scalar": {map[string]string{"gates.yaml": "gates:\n  - { key: undefined }\n  - { key: [a] }\n"}, "gates"},
		"no gate after undefined":      {map[string]string{"gates.yaml": "gates:\n  - { key: undefined }\n"}, "gates"},
		"a severity that is no integer": {map[string]string{"gates.yaml": "gates:\n  - { key: undefined }\n  - { key: a }\n" +
			"states:\n  - { key: undefined, severity: 0 }\n  - { key: complete, severity: 0 }\n  - { key: slow, severity: high }\n"}, "states"},
		"a state with no severity": {map[string]string{"gates.yaml": "gates:\n  - { key: undefined }\n  - { key: a }\n" +
			"states:\n  - { key: undefined, severity: 0 }\n  - { key: complete }\n"}, "states"},
		"no task":                    {map[string]string{}, "tree"},
		"a parent of itself":         {map[string]string{"tasks/a000.yaml": leafOf("a@x", "a000", 1, "")}, "tree"},
		"a parent that is no scalar": {map[string]string{"tasks/a000.yaml": "assignee: a@x\nparent: { id: [r000] }\n"}, "tree"},
		// Every other fault leaves the facts standing.
		"a status at an unknown gate": {map[string]string{"tasks/a000.yaml": leafOf("a@x", "r000", 1, ""), "status/a000.yaml": at("nowhere", "odd")}, ""},
		"a requirement on no task":    {map[string]string{"tasks/a000.yaml": leafOf("a@x", "r000", 1, "") + "requires: [{ id: \"zzzz\" }]\n"}, ""},
		"a task with no assignee":     {map[string]string{"tasks/a000.yaml": "title: None\nparent: { id: \"r000\" }\n"}, ""},
		"a recursive junction on a parent": {map[string]string{"tasks/r000.yaml": leafOf("r@x", "", 0, "{ design: { subproject: { url: lib } } }"),
			"tasks/a000.yaml": leafOf("a@x", "r000", 1, "")}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := tc.files["tasks/r000.yaml"]; !ok && name != "no task" {
				tc.files["tasks/r000.yaml"] = root
			}
			f := New(compose(t, tc.files), nil)
			got := ""
			if r := f.Refused(); r != nil {
				got = r.Part
			}
			if got != tc.part {
				t.Errorf("the refusal's part is %q; want %q", got, tc.part)
			}
			exercise(f, 0)
		})
	}
	if r := New(&model.Project{}, nil).Refused(); r == nil || r.Part != "project" || !reflect.DeepEqual(r.Rules, []string{"P1"}) {
		t.Errorf("a project with no .tableaux is refused as %+v; want the project, by P1", r)
	}
	var none *Facts
	if none.Refused() != nil || none.Status("a000") != nil || none.Root() != "" || none.Sub(nil) != nil || none.Order() != nil {
		t.Error("nil facts give a fact")
	}
}

// T23: a cycle ends. Two directories of one repository hold a project each,
// and the leaf of each reads the other's root through a recursive junction.
func TestCycleEnds(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q", "-b", "main")
	for dir, other := range map[string]string{"one": "two", "two": "one"} {
		write(t, repo, map[string]string{
			dir + "/.tableaux/version.yaml":     memVersion,
			dir + "/.tableaux/gates.yaml":       memGates,
			dir + "/.tableaux/tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
			dir + "/.tableaux/tasks/a000.yaml":  leafOf("a@x", "r000", 1, "{ design: { subproject: { url: "+other+" } } }"),
			dir + "/.tableaux/status/a000.yaml": "gate: function\n",
			dir + "/.tableaux/tasks/b000.yaml":  leafOf("b@x", "r000", 2, ""),
			dir + "/.tableaux/status/b000.yaml": at("release", "complete"),
		})
	}
	for _, first := range []string{"one", "two"} {
		f := derived(t, load.Options{}, repo+"/"+first, "", "")
		if f.Refused() != nil || len(f.Project().Links) != 1 {
			t.Fatalf("from %s: refused %+v with the links %v", first, f.Refused(), f.Project().Links)
		}
		other := f.Sub(f.Project().Links[0])
		for name, g := range map[string]*Facts{"the project in view": f, "the other project": other} {
			s := g.Status("a000")
			if s.Kind != Snapshotted || s.Why != Cycle || s.Gate != "function" || s.State != "" || s.Of != nil || s.Snapshot == nil {
				t.Errorf("from %s, %s: the leaf's status is %+v; want the gate of its file and Cycle", first, name, s)
			}
			// The root rolls its two leaves up: the gate stands and the state is empty.
			if r := g.Status("r000"); r.Kind != RolledUp || r.Gate != "function" || r.State != "" || r.From != "a000" || r.Why != Cycle {
				t.Errorf("from %s, %s: the root's status is %+v", first, name, r)
			}
			if chain := g.Chain("r000"); len(chain) != 2 {
				t.Errorf("from %s, %s: the chain holds %d statuses; want the root and its leaf", first, name, len(chain))
			}
		}
		exercise(f, 0)
	}
}
