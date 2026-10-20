package validate

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
)

// The rules of language 0.4.0 on projects built in memory (design cdb0, T6).

// leaf04 is the leaf of base() with a junction entry at design.
func leaf04(entry string) string {
	return baseLeaf + "junctions:\n  design: " + entry + "\n"
}

// lines writes each diagnostic of a code, or of every code for "", as its
// place and its message.
func lines(t *testing.T, files map[string]string, code string) []string {
	t.Helper()
	var out []string
	for _, d := range Files(project(t, files)) {
		if code == "" || d.Code == code {
			out = append(out, where(d)+": "+d.Message)
		}
	}
	return out
}

// TestAgentsFileRules is T6 for A1 and A2: the schema judges agents.yaml,
// every leaf A1; a model beside a contributor the file does not list is A2,
// where the project keeps the file and the entry has no schema leaf.
func TestAgentsFileRules(t *testing.T) {
	for _, c := range []struct {
		name, agents, want string
	}{
		{"listed", "agents: [opus@example.org]\n", ""},
		{"no email", "agents:\n  - Corpus Agent\n", "agents.yaml:2:5: error: A1: agents.0 Corpus Agent is no email address"},
		{"twice", "agents: [opus@example.org, opus@example.org]\n", "agents.yaml:1:9: error: A1: agents states an item twice"},
		{"empty", "agents: []\n", "agents.yaml:1:9: error: A1: agents holds 0, fewer than 1"},
		{"unknown field", "agents: [opus@example.org]\nmodels: [claude]\n", "agents.yaml:2:1: error: A1: models is no field of the file"},
		{"no list", "agents: opus@example.org\n", "agents.yaml:1:9: error: A1: agents is a string, not a sequence"},
	} {
		got := strings.Join(lines(t, with(base(), "agents.yaml", c.agents), "A1"), "\n")
		if got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
	for _, c := range []struct {
		name, agents, entry, want string
	}{
		{"unlisted", "agents: [opus@example.org]\n", "{ contributor: bot@example.org, model: claude-fable }",
			"tasks/b2c9.yaml:6:26: error: A2 task=b2c9 gate=design: the design junction states the model claude-fable beside bot@example.org, whom agents.yaml does not list"},
		{"listed", "agents: [bot@example.org]\n", "{ contributor: bot@example.org, model: claude-fable }", ""},
		{"no file", "", "{ contributor: bot@example.org, model: claude-fable }", ""},
		{"no model", "agents: [opus@example.org]\n", "{ contributor: bot@example.org }", ""},
		{"a schema leaf", "agents: [opus@example.org]\n", "{ contributor: bot@example.org, model: claude-fable, colour: red }", ""},
		{"a broken file", "agents: [unclosed\n", "{ contributor: bot@example.org, model: claude-fable }", ""},
	} {
		got := strings.Join(lines(t, with(base(), "agents.yaml", c.agents, "tasks/b2c9.yaml", leaf04(c.entry)), "A2"), "\n")
		if got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}
}

// TestReviewerIsEmailOrFalse is T6 for T4: reviewer: false passes, and true
// gives one T4, since the two leaves of the oneOf give one message.
func TestReviewerIsEmailOrFalse(t *testing.T) {
	if got := lines(t, with(base(), "tasks/b2c9.yaml", leaf04("{ reviewer: false }")), ""); len(got) != 0 {
		t.Errorf("reviewer: false gives %q", got)
	}
	want := "tasks/b2c9.yaml:6:23: error: T4 task=b2c9 gate=design: junctions.design.reviewer true is neither an email address nor false"
	if got := lines(t, with(base(), "tasks/b2c9.yaml", leaf04("{ reviewer: true }")), ""); strings.Join(got, "\n") != want {
		t.Errorf("reviewer: true gives\n %s\nwant\n %s", strings.Join(got, "\n "), want)
	}
}

// manyGates is a gating of ten gates, for a task whose link many junctions name.
const manyGates = `gates:
  - { key: undefined, symbol: "?", name: Undefined, criteria: Nothing yet }
  - { key: defined, symbol: D, name: Defined, criteria: The definition exists }
  - { key: g2, symbol: A, name: Two, criteria: Two }
  - { key: g3, symbol: B, name: Three, criteria: Three }
  - { key: g4, symbol: C, name: Four, criteria: Four }
  - { key: g5, symbol: E, name: Five, criteria: Five }
  - { key: g6, symbol: F, name: Six, criteria: Six }
  - { key: g7, symbol: G, name: Seven, criteria: Seven }
  - { key: g8, symbol: H, name: Eight, criteria: Eight }
  - { key: g9, symbol: I, name: Nine, criteria: Nine }
states:
  - { key: undefined, symbol: U, severity: 0, synopsis: Not defined }
  - { key: nominal, symbol: N, severity: 1, synopsis: Proceeding }
  - { key: complete, symbol: C, severity: 0, synopsis: Done }
`

// lib is a subproject in the directory lib of the same repository, whose
// version.yaml states version.
func lib(files map[string]string, version string) map[string]string {
	return with(files,
		"/lib/.tableaux/version.yaml", "tableaux: "+version+"\ntrunk: main\n",
		"/lib/.tableaux/gates.yaml", baseGates,
		"/lib/.tableaux/tasks/5a00.yaml", "title: Lib\ndescription: The subproject.\nassignee: pat@example.org\n",
	)
}

// TestSubprojectFieldRules is T6 for the task-level field: its schema rows
// with no gate, T7 at subproject.id, J18 on a parent with the field's other
// rows beside it, R12 on a requirement the field already reads, and J19 once
// per task and link, silent for a requirement and an unreadable version.
func TestSubprojectFieldRules(t *testing.T) {
	field := func(text string) string { return baseLeaf + "subproject: " + text + "\n" }
	for _, c := range []struct {
		name, leaf, code, want string
	}{
		{"T6", field(`{ url: lib, id: "XYZ1" }`), "T6", `tasks/b2c9.yaml:5:29: error: T6 task=b2c9: subproject.id XYZ1 does not match ^[0-9a-f]{4}$`},
		{"J14", field(`{ url: lib, commit: abc123 }`), "J14", "tasks/b2c9.yaml:5:33: error: J14 task=b2c9: subproject.commit abc123 does not match ^[0-9a-f]{40}$"},
		{"J7", field(`{ id: "5a00" }`), "J7", "tasks/b2c9.yaml:5:13: error: J7 task=b2c9: subproject lacks url"},
		{"J7, unknown field", field(`{ url: lib, colour: red }`), "J7", "tasks/b2c9.yaml:5:25: error: J7 task=b2c9: colour is no field of subproject"},
		{"T7", field(`{ url: lib, id: 5a00 }`), "T7", "tasks/b2c9.yaml:5:29: warning: T7 task=b2c9: subproject.id is written without quotes: 5a00"},
		{"J8", field(`{ url: nowhere }`), "J8", "tasks/b2c9.yaml:5:20: error: J8 task=b2c9: url nowhere resolves to no Tableaux project the tool can read: missing"},
		{"J16", field(`{ url: lib, commit: 0123456789abcdef0123456789abcdef01234567 }`), "J16", "tasks/b2c9.yaml:5:33: error: J16 task=b2c9: url lib is a directory of this repository, which nothing pins"},
		{"R12", field(`{ url: lib, id: "5a00" }`) + "requires: [{ subproject: { url: lib, id: \"5a00\" } }]\n", "R12",
			"tasks/b2c9.yaml:6:26: warning: R12 task=b2c9: the requirement on 5a00 at lib names the task the subproject field reads"},
	} {
		got := strings.Join(lines(t, lib(with(base(), "tasks/b2c9.yaml", c.leaf), "0.3.1"), c.code), "\n")
		if got != c.want {
			t.Errorf("%s:\n got  %s\n want %s", c.name, got, c.want)
		}
	}

	// J18 on a parent, and the field's other rows still run.
	parent := lib(with(base(), "tasks/e4a1.yaml", baseRoot+"subproject: { url: lib, id: \"9999\" }\n"), "0.3.1")
	if got, want := strings.Join(lines(t, parent, ""), "\n"),
		"tasks/e4a1.yaml:4:13: error: J18 task=e4a1: a parent states a task-level subproject field\n"+
			"tasks/e4a1.yaml:4:29: error: J9 task=e4a1: id 9999 names no task in the project at lib"; got != want {
		t.Errorf("a field on a parent:\n got  %s\n want %s", got, want)
	}

	// J19: once for a task whose link the field and eight entries name, at the field's url.
	var entries []string
	for _, gate := range []string{"g2", "g3", "g4", "g5", "g6", "g7", "g8", "g9"} {
		entries = append(entries, gate+": { subproject: { url: lib } }")
	}
	behind := lib(with(base(), "gates.yaml", manyGates, "tasks/b2c9.yaml",
		field("{ url: lib }")+"junctions: { "+strings.Join(entries, ", ")+" }\n"), "0.3.0")
	if got, want := strings.Join(lines(t, behind, "J19"), "\n"),
		"tasks/b2c9.yaml:5:20: warning: J19 task=b2c9: the project at lib states tableaux 0.3.0, before this project's 0.3.1"; got != want {
		t.Errorf("J19 on eight junctions and the field:\n got  %s\n want %s", got, want)
	}
	junctions := lib(with(base(), "gates.yaml", manyGates, "tasks/b2c9.yaml",
		baseLeaf+"junctions: { "+strings.Join(entries, ", ")+" }\n"), "0.2.9")
	if got := lines(t, junctions, "J19"); len(got) != 1 || !strings.Contains(got[0], "tasks/b2c9.yaml:5:39:") {
		t.Errorf("J19 on eight junctions: %q; want one, at the first junction's url", got)
	}
	for name, files := range map[string]map[string]string{
		"a requirement":         lib(with(base(), "tasks/b2c9.yaml", baseLeaf+"requires: [{ subproject: { url: lib, id: \"5a00\" } }]\n"), "0.3.0"),
		"an unreadable version": lib(with(base(), "tasks/b2c9.yaml", field("{ url: lib }")), "0.3"),
		"the same version":      lib(with(base(), "tasks/b2c9.yaml", field("{ url: lib }")), "0.3.1"),
		"a later version":       lib(with(base(), "tasks/b2c9.yaml", field("{ url: lib }")), "0.4.0"),
	} {
		if got := lines(t, files, "J19"); len(got) != 0 {
			t.Errorf("J19 on %s: %q", name, got)
		}
	}
}

// TestFieldResolvesAsTheDerivation is T7: the Validator's own resolution of
// the task-level field gives the Derivation's kinds and applicable gates, on
// subproject-field and subproject-field-on-parent.
func TestFieldResolvesAsTheDerivation(t *testing.T) {
	for _, name := range []string{"subproject-field", "subproject-field-on-parent"} {
		p := entry(t, name)
		s, f := newShape(p), derive.New(p, nil)
		for _, id := range p.TaskIDs() {
			if got, want := s.applicable(id), f.Applicable(id); !reflect.DeepEqual(got, want) {
				t.Errorf("%s %s: applicable %v; the Derivation gives %v", name, id, got, want)
			}
			if s.isParent(id) {
				continue // a parent's junctions are defaults, which kindAt does not read
			}
			for _, j := range f.Junctions(id) {
				want := map[model.JunctionKind]resolved{model.Plain: plain, model.Recursive: recursive, model.NotApplicable: notApplicable}[j.Kind]
				if got := s.kindAt(id, j.Gate); got != want {
					t.Errorf("%s %s %s: kindAt %d; the Derivation gives %s", name, id, j.Gate, got, j.Kind)
				}
			}
		}
	}
}
