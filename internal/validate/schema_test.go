package validate

import (
	"sort"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"

	"github.com/nbyoung/tablo/internal/model"
)

// The files a fixture of the leaf table changes, each valid as it stands.
const (
	leafGates = `gates:
  - { key: undefined, symbol: U, name: Undefined, criteria: None }
  - { key: defined, symbol: D, name: Defined, criteria: Some }
states:
  - { key: undefined, symbol: U, severity: 0, synopsis: Not defined }
  - { key: complete, symbol: C, severity: 0, synopsis: Done }
reasons:
  - { key: blocked, symbol: B, synopsis: Waiting }
`
	leafTask = `title: Leaf
description: A leaf.
assignee: pat@example.org
parent: { id: "e4a1", order: 1 }
`
)

// schemaLeaves returns where the schema leaves of one file of a project sit,
// sorted: the leaves of the file and, for a task file, of each junction entry
// that is a mapping of one kind, as Files gathers them.
func schemaLeaves(t *testing.T, p *model.Project, path string) []string {
	t.Helper()
	var file *model.File
	for _, f := range p.Files {
		if f.Path == path {
			file = f
		}
	}
	if file == nil {
		t.Fatalf("the project holds no %s", path)
	}
	var found []leaf
	dir, name, _ := strings.Cut(path, "/")
	id := strings.TrimSuffix(name, ".yaml")
	switch dir {
	case "version.yaml":
		found = checkFile(versionFile, file, "")
	case "gates.yaml":
		found = checkFile(gatesFile, file, "")
	case "status":
		found = checkFile(statusFile, file, id)
	default:
		found = checkFile(taskFile, file, id)
		for _, j := range p.Tasks[id].Junctions {
			if k := j.Kind(); k != model.Mixed && j.Node != nil && j.Node.Kind == model.Map {
				found = append(found, checkEntry(file, id, j, k)...)
			}
		}
	}
	var lines []string
	for _, l := range found {
		lines = append(lines, where(l.diagnostic))
	}
	sort.Strings(lines)
	return lines
}

// TestLeafTable is T4: one fixture for each row of schema.md gives the rule,
// the gate and the position the row states, and nothing else from the schema.
func TestLeafTable(t *testing.T) {
	gates := func(old, new string) string { return strings.Replace(leafGates, old, new, 1) }
	task := func(old, new string) string { return strings.Replace(leafTask, old, new, 1) }
	more := func(lines string) string { return leafTask + lines }
	cases := []struct {
		row, path, content string
		want               []string
	}{
		// version.yaml
		{"version: any", "version.yaml", "tableaux: 0.3.1\nowner: olive@example.org\n", []string{"version.yaml:2:1: error: P3"}},

		// gates.yaml
		{"gates: root, additionalProperties", "gates.yaml", leafGates + "title: Gates\n", []string{"gates.yaml:9:1: error: G11"}},
		{"gates: root, required, states alone", "gates.yaml", leafGates[:strings.Index(leafGates, "states:")], []string{"gates.yaml:1:1: error: G7"}},
		{"gates: root, any other", "gates.yaml", leafGates[strings.Index(leafGates, "states:"):], []string{"gates.yaml:1:1: error: G3"}},
		{"gates: gates", "gates.yaml", "gates: []\n" + leafGates[strings.Index(leafGates, "states:"):], []string{"gates.yaml:1:8: error: G3"}},
		{"gates: gates/0/key, prefixItems", "gates.yaml", gates("key: undefined, symbol: U, name", "key: first, symbol: U, name"), []string{"gates.yaml:2:12: error: G2"}},
		{"gates: gates/<i>/key", "gates.yaml", gates("key: defined", "key: Defined"), []string{"gates.yaml:3:12: error: G5 gate=Defined"}},
		{"gates: gates/<i>", "gates.yaml", gates(", criteria: Some", ""), []string{"gates.yaml:3:5: error: G4 gate=defined"}},
		{"gates: gates/<i>/<field>", "gates.yaml", gates("name: Defined", `name: ""`), []string{"gates.yaml:3:38: error: G4 gate=defined"}},
		{"gates: states/<i>/key", "gates.yaml", gates("key: complete", "key: Complete"), []string{"gates.yaml:6:12: error: G5"}},
		{"gates: states/<i>/severity", "gates.yaml", gates("severity: 0, synopsis: Done", "severity: -1, synopsis: Done"), []string{"gates.yaml:6:43: error: G8"}},
		{"gates: states", "gates.yaml", gates("  - { key: complete, symbol: C, severity: 0, synopsis: Done }\n", ""), []string{"gates.yaml:5:3: error: G7"}},
		{"gates: states/<i>", "gates.yaml", gates(", synopsis: Done", ""), []string{"gates.yaml:6:5: error: G7"}},
		{"gates: states/<i>/<field>", "gates.yaml", gates("symbol: C", `symbol: ""`), []string{"gates.yaml:6:30: error: G7"}},
		{"gates: reasons/<i>/key", "gates.yaml", gates("key: blocked", "key: Blocked"), []string{"gates.yaml:8:12: error: G5"}},
		{"gates: reasons", "gates.yaml", gates("reasons:\n  - { key: blocked, symbol: B, synopsis: Waiting }", "reasons: none"), []string{"gates.yaml:7:10: error: G10"}},
		{"gates: reasons/<i>", "gates.yaml", gates(", synopsis: Waiting", ""), []string{"gates.yaml:8:5: error: G10"}},
		{"gates: reasons/<i>/<field>", "gates.yaml", gates("symbol: B", `symbol: ""`), []string{"gates.yaml:8:29: error: G10"}},

		// tasks/<id>.yaml
		{"task: root, additionalProperties", "tasks/b2c9.yaml", more("id: b2c9\n"), []string{"tasks/b2c9.yaml:5:1: error: T3 task=b2c9"}},
		{"task: root, any other", "tasks/b2c9.yaml", task("assignee: pat@example.org\n", ""), []string{"tasks/b2c9.yaml:1:1: error: T2 task=b2c9"}},
		{"task: title", "tasks/b2c9.yaml", task("title: Leaf", `title: ""`), []string{"tasks/b2c9.yaml:1:8: error: T2 task=b2c9"}},
		{"task: description", "tasks/b2c9.yaml", task("description: A leaf.", "description: [a]"), []string{"tasks/b2c9.yaml:2:14: error: T2 task=b2c9"}},
		{"task: assignee, format", "tasks/b2c9.yaml", task("pat@example.org", "nobody"), []string{"tasks/b2c9.yaml:3:11: error: T4 task=b2c9"}},
		{"task: assignee, any other", "tasks/b2c9.yaml", task("pat@example.org", "[pat@example.org]"), []string{"tasks/b2c9.yaml:3:11: error: T2 task=b2c9"}},
		{"task: references", "tasks/b2c9.yaml", more("references: README.md\n"), []string{"tasks/b2c9.yaml:5:13: error: T5 task=b2c9"}},
		{"task: references/…", "tasks/b2c9.yaml", more("references: [{ text: Read me }]\n"), []string{"tasks/b2c9.yaml:5:14: error: T5 task=b2c9"}},
		{"task: parent/id", "tasks/b2c9.yaml", task(`"e4a1"`, `"XYZ1"`), []string{"tasks/b2c9.yaml:4:15: error: T6 task=b2c9"}},
		{"task: parent", "tasks/b2c9.yaml", task(`id: "e4a1", `, ""), []string{"tasks/b2c9.yaml:4:9: error: T11 task=b2c9"}},
		{"task: parent/order", "tasks/b2c9.yaml", task("order: 1", "order: 0"), []string{"tasks/b2c9.yaml:4:30: error: T11 task=b2c9"}},
		{"task: requires", "tasks/b2c9.yaml", more("requires: e4a1\n"), []string{"tasks/b2c9.yaml:5:11: error: R8 task=b2c9"}},
		{"task: requires/<i>, oneOf, neither", "tasks/b2c9.yaml", more("requires: [{ text: Nothing }]\n"), []string{"tasks/b2c9.yaml:5:12: error: R8 task=b2c9", "tasks/b2c9.yaml:5:12: error: R8 task=b2c9"}},
		{"task: requires/<i>, oneOf, both", "tasks/b2c9.yaml", more("requires: [{ id: \"c3d7\", subproject: { url: lib, id: \"a000\" } }]\n"), []string{"tasks/b2c9.yaml:5:12: error: R11 task=b2c9"}},
		{"task: requires/<i>/id", "tasks/b2c9.yaml", more("requires: [{ id: \"XYZ1\" }]\n"), []string{"tasks/b2c9.yaml:5:18: error: T6 task=b2c9"}},
		{"task: requires/<i>/from", "tasks/b2c9.yaml", more("requires: [{ id: \"c3d7\", from: Late }]\n"), []string{"tasks/b2c9.yaml:5:32: error: R6 task=b2c9 gate=Late"}},
		{"task: requires/<i>/to", "tasks/b2c9.yaml", more("requires: [{ id: \"c3d7\", to: Late }]\n"), []string{"tasks/b2c9.yaml:5:30: error: R7 task=b2c9 gate=Late"}},
		{"task: requires/<i>/subproject/id", "tasks/b2c9.yaml", more("requires: [{ subproject: { url: lib, id: \"XYZ1\" } }]\n"), []string{"tasks/b2c9.yaml:5:42: error: T6 task=b2c9"}},
		{"task: requires/<i>/subproject/commit", "tasks/b2c9.yaml", more("requires: [{ subproject: { url: lib, id: \"a000\", commit: abc123 } }]\n"), []string{"tasks/b2c9.yaml:5:58: error: J14 task=b2c9"}},
		{"task: requires/<i>/subproject", "tasks/b2c9.yaml", more("requires: [{ subproject: { url: lib } }]\n"), []string{"tasks/b2c9.yaml:5:26: error: R11 task=b2c9"}},
		{"task: requires/<i>/subproject/url", "tasks/b2c9.yaml", more("requires: [{ subproject: { url: \"\", id: \"a000\" } }]\n"), []string{"tasks/b2c9.yaml:5:33: error: R11 task=b2c9"}},
		{"task: requires/<i>, any other", "tasks/b2c9.yaml", more("requires: [{ id: \"c3d7\", why: Because }]\n"), []string{"tasks/b2c9.yaml:5:26: error: R8 task=b2c9"}},
		{"task: requires/<i>/text", "tasks/b2c9.yaml", more("requires: [{ id: \"c3d7\", text: \"\" }]\n"), []string{"tasks/b2c9.yaml:5:32: error: R8 task=b2c9"}},
		{"task: junctions", "tasks/b2c9.yaml", more("junctions: none\n"), []string{"tasks/b2c9.yaml:5:12: error: J4 task=b2c9"}},
		{"task: junctions/<gate>, dependentRequired", "tasks/b2c9.yaml", more("junctions: { design: { model: claude-fable } }\n"), []string{"tasks/b2c9.yaml:5:31: error: J5 task=b2c9 gate=design"}},
		{"task: junctions/<gate>, plain", "tasks/b2c9.yaml", more("junctions: { design: { colour: red } }\n"), []string{"tasks/b2c9.yaml:5:24: error: J10 task=b2c9 gate=design"}},
		{"task: junctions/<gate>, recursive", "tasks/b2c9.yaml", more("junctions: { design: { subproject: { url: lib }, colour: red } }\n"), []string{"tasks/b2c9.yaml:5:50: error: J4 task=b2c9 gate=design"}},
		{"task: junctions/<gate>, not applicable", "tasks/b2c9.yaml", more("junctions: { design: { applies: false, colour: red } }\n"), []string{"tasks/b2c9.yaml:5:40: error: J6 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/contributor", "tasks/b2c9.yaml", more("junctions: { design: { contributor: nobody } }\n"), []string{"tasks/b2c9.yaml:5:37: error: T4 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/reviewer", "tasks/b2c9.yaml", more("junctions: { design: { reviewer: [olive@example.org] } }\n"), []string{"tasks/b2c9.yaml:5:34: error: T4 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/references/…", "tasks/b2c9.yaml", more("junctions: { design: { references: [{ text: Read me }] } }\n"), []string{"tasks/b2c9.yaml:5:37: error: T5 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/model", "tasks/b2c9.yaml", more("junctions: { design: { contributor: bot@example.org, model: \"\" } }\n"), []string{"tasks/b2c9.yaml:5:61: error: J10 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/applies", "tasks/b2c9.yaml", more("junctions: { design: { applies: true } }\n"), []string{"tasks/b2c9.yaml:5:33: error: J6 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/subproject/id", "tasks/b2c9.yaml", more("junctions: { design: { subproject: { url: lib, id: \"XYZ1\" } } }\n"), []string{"tasks/b2c9.yaml:5:52: error: T6 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/subproject/commit", "tasks/b2c9.yaml", more("junctions: { design: { subproject: { url: lib, commit: abc123 } } }\n"), []string{"tasks/b2c9.yaml:5:56: error: J14 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/subproject", "tasks/b2c9.yaml", more("junctions: { design: { subproject: { id: \"a000\" } } }\n"), []string{"tasks/b2c9.yaml:5:36: error: J7 task=b2c9 gate=design"}},
		{"task: junctions/<gate>/subproject/url", "tasks/b2c9.yaml", more("junctions: { design: { subproject: { url: \"\" } } }\n"), []string{"tasks/b2c9.yaml:5:43: error: J7 task=b2c9 gate=design"}},

		// status/<id>.yaml
		{"status: then, no scalar gate", "status/b2c9.yaml", "gate: [defined]\nstate: undefined\n", []string{"status/b2c9.yaml:1:7: error: S5 task=b2c9"}},
		{"status: then", "status/b2c9.yaml", "gate: undefined\nstate: nominal\n", []string{"status/b2c9.yaml:2:8: error: S4 task=b2c9"}},
		{"status: gate", "status/b2c9.yaml", "gate: Design\nstate: nominal\n", []string{"status/b2c9.yaml:1:7: error: S5 task=b2c9 gate=Design"}},
		{"status: state", "status/b2c9.yaml", "gate: defined\nstate: Nominal\n", []string{"status/b2c9.yaml:2:8: error: S6 task=b2c9"}},
		{"status: reason", "status/b2c9.yaml", "gate: defined\nstate: nominal\nreason: Blocked\n", []string{"status/b2c9.yaml:3:9: error: S6 task=b2c9"}},
		{"status: root", "status/b2c9.yaml", "gate: defined\nstate: nominal\ncolour: red\n", []string{"status/b2c9.yaml:3:1: error: S3 task=b2c9"}},
		{"status: note", "status/b2c9.yaml", "gate: defined\nstate: nominal\nnote: \"\"\n", []string{"status/b2c9.yaml:3:7: error: S3 task=b2c9"}},
	}
	for _, c := range cases {
		p := project(t, with(base(), "gates.yaml", leafGates, "tasks/b2c9.yaml", leafTask, c.path, c.content))
		if got := schemaLeaves(t, p, c.path); strings.Join(got, "\n") != strings.Join(c.want, "\n") {
			t.Errorf("%s:\n got  %q\n want %q", c.row, got, c.want)
		}
	}
}

// TestLeafTableIsTotal is T4, second part: a leaf no row names takes the last
// row of its file kind, and the row of a recursive entry that lacks its
// subproject, which no file reaches, is J7.
func TestLeafTableIsTotal(t *testing.T) {
	root := &model.Value{Kind: model.Map}
	var other jsonschema.ErrorKind = &kind.Type{Got: "string", Want: []string{"object"}}
	cases := []struct {
		kind    fileKind
		at      []string
		keyword string
		failed  jsonschema.ErrorKind
		want    string
	}{
		{versionFile, []string{"anything", "0"}, "/x/type", other, "P3"},
		{gatesFile, []string{"other"}, "/x/type", other, "G11"},
		{taskFile, []string{"other", "0"}, "/x/type", other, "T3"},
		{taskFile, []string{"junctions", "design", "other"}, "/x/type", other, "T3"},
		{statusFile, []string{"other"}, "/x/type", other, "S3"},
		{statusFile, nil, "/type", other, "S3"},
		{taskFile, []string{"junctions", "design"}, "/$defs/recursive/required", &kind.Required{Missing: []string{"subproject"}}, "J7"},
	}
	for _, c := range cases {
		if got, _, ok := row(c.kind, root, c.at, c.keyword, c.failed); !ok || got != c.want {
			t.Errorf("row(%d, %v, %s) = %s, %v; want %s", c.kind, c.at, c.keyword, got, ok, c.want)
		}
	}
	// The one leaf a table drops: the undefined rule of a status with no scalar gate.
	if _, _, ok := row(statusFile, root, []string{"gate"}, "/allOf/1/then/properties/gate/const", other); ok {
		t.Error("the table takes a then leaf of a status with no scalar gate")
	}
}
