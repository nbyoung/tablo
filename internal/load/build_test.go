package load

import (
	"reflect"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// file reads text as the file at path and fails the test when it earns a diagnostic.
func file(t *testing.T, path, text string) *model.File {
	t.Helper()
	root, parsed, diagnostics := readYAML(path, []byte(text))
	if !parsed || len(diagnostics) > 0 {
		t.Fatalf("%s: parsed %v, diagnostics %+v", path, parsed, diagnostics)
	}
	return &model.File{Path: path, Root: root}
}

// The typed fields are lenient where the schema is strict: a field of the
// wrong kind keeps its node and takes no value, and a root that is no mapping
// gives a task with nothing set.
func TestBuildIsLenient(t *testing.T) {
	task := buildTask("a000", file(t, "tasks/a000.yaml", `
title: [a]
description: 5
assignee:
references: { url: x }
requires:
  - plain
  - { id: [b2c9], subproject: lib, from: 3 }
junctions:
  design:
  unit: [a]
  release: { applies: maybe, subproject: { url: [lib], id: 0010, commit: 1e10 }, references: [{ url: a, text: b }, c] }
parent: 7
extra: true
`))
	if task.Title.Node == nil || task.Title.V != "" || task.Description.V != "5" || task.Assignee.Node == nil || task.Assignee.V != "" {
		t.Errorf("the scalars: %+v, %+v, %+v", task.Title, task.Description, task.Assignee)
	}
	if task.References != nil {
		t.Errorf("references that are no sequence: %+v", task.References)
	}
	if len(task.Requires) != 2 || task.Requires[0].Node.Text != "plain" || task.Requires[0].ID.Node != nil {
		t.Fatalf("requires: %+v", task.Requires)
	}
	second := task.Requires[1]
	if second.ID.Node == nil || second.ID.V != "" || second.From.V != "3" || second.Subproject == nil ||
		second.Subproject.URL.Node != nil || second.Subproject.Node.Text != "lib" {
		t.Errorf("the second requirement: %+v, subproject %+v", second, second.Subproject)
	}
	if len(task.Junctions) != 3 {
		t.Fatalf("junctions: %d", len(task.Junctions))
	}
	design, unit, release := task.Junctions[0], task.Junctions[1], task.Junctions[2]
	if design.Gate != "design" || design.Node.Kind != model.Null || design.Kind() != model.Plain || unit.Kind() != model.Plain {
		t.Errorf("design %+v, unit %+v", design, unit)
	}
	if release.Kind() != model.Mixed || release.Applies.Node == nil || release.Applies.OK || len(release.References) != 2 ||
		release.References[0].Text.V != "b" || release.References[1].URL.Node != nil {
		t.Errorf("release: %+v", release)
	}
	sub := release.Subproject
	if sub.URL.Node == nil || sub.URL.V != "" || sub.ID.V != "0010" || sub.ID.Node.Kind != model.String ||
		sub.Commit.V != "1e10" || sub.Commit.Node.Kind != model.Float {
		t.Errorf("the subproject: url %+v, id %+v, commit %+v", sub.URL, sub.ID.Node, sub.Commit.Node)
	}
	if task.Parent == nil || task.Parent.ID.Node != nil || task.Parent.Node.Text != "7" {
		t.Errorf("parent: %+v", task.Parent)
	}
	if task.File.Root.Get("extra") == nil {
		t.Error("a field the schema does not name stays in the root")
	}
	if got := subprojects(task); !reflect.DeepEqual(got, []*model.Subproject{second.Subproject, sub}) {
		t.Errorf("subprojects: %d", len(got))
	}

	for _, text := range []string{"", "- a\n- b\n", "5\n"} {
		root, _, _ := readYAML("tasks/b000.yaml", []byte(text))
		empty := buildTask("b000", &model.File{Path: "tasks/b000.yaml", Root: root})
		if empty.ID != "b000" || empty.Title.Node != nil || empty.Parent != nil || empty.Junctions != nil || empty.Requires != nil {
			t.Errorf("a task of %q: %+v", text, empty)
		}
		status := buildStatus("b000", &model.File{Path: "status/b000.yaml", Root: root})
		if status.ID != "b000" || status.Gate.Node != nil {
			t.Errorf("a status of %q: %+v", text, status)
		}
		if gating := buildGating(&model.File{Path: "gates.yaml", Root: root}); gating.Gates != nil || gating.States != nil {
			t.Errorf("a gating of %q: %+v", text, gating)
		}
	}
}

// Each list of gates.yaml keeps an entry for every item written.
func TestBuildGating(t *testing.T) {
	gating := buildGating(file(t, "gates.yaml", `
gates:
  - { key: undefined, symbol: ❔, name: Undefined, criteria: None }
  - stray
states:
  - { key: nominal, symbol: 🟢, severity: 1, synopsis: Fine }
  - { key: odd, severity: high }
  - { key: half, severity: 1.0 }
reasons:
  - { key: blocked, symbol: ⛔, synopsis: Waiting }
`))
	if len(gating.Gates) != 2 || gating.Gates[0].Criteria.V != "None" || gating.Gates[1].Key.Node != nil || gating.Gates[1].Node.Text != "stray" {
		t.Errorf("gates: %+v", gating.Gates)
	}
	if len(gating.States) != 3 || !gating.States[0].Severity.OK || gating.States[0].Severity.V != 1 ||
		gating.States[1].Severity.OK || gating.States[1].Severity.Node == nil ||
		!gating.States[2].Severity.OK || gating.States[2].Severity.V != 1 {
		t.Errorf("states: %+v", gating.States)
	}
	if len(gating.Reasons) != 1 || gating.Reasons[0].Symbol.V != "⛔" {
		t.Errorf("reasons: %+v", gating.Reasons)
	}
}

// P4 is the one content rule the Loader raises: a well-formed version the
// module does not accept, at the version's position.
func TestBuildVersion(t *testing.T) {
	tests := []struct {
		text                 string
		wellFormed, accepted bool
		want                 []string
	}{
		{"tableaux: 0.3.1\ntrunk: main\n", true, true, nil},
		{"tableaux: 0.1.0\n", true, true, nil},
		{"# a comment\ntableaux:   0.4.0\n", true, false, []string{"version.yaml:2:13 error P4"}},
		{"tableaux: \"1.0.0\"\n", true, false, []string{"version.yaml:1:11 error P4"}},
		{"tableaux: \"0.2\"\n", false, false, nil},
		{"tableaux: 1.0\n", false, false, nil},
		{"tableaux: [0, 3, 1]\n", false, false, nil},
		{"trunk: main\n", false, false, nil},
		{"- 0.3.1\n", false, false, nil},
	}
	for _, tt := range tests {
		version, diagnostics := buildVersion(file(t, "version.yaml", tt.text))
		if version.WellFormed != tt.wellFormed || version.Accepted != tt.accepted {
			t.Errorf("%q: well formed %v, accepted %v", tt.text, version.WellFormed, version.Accepted)
		}
		if got := codes(diagnostics); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q: diagnostics %v, want %v", tt.text, got, tt.want)
		}
	}
	version, _ := buildVersion(file(t, "version.yaml", "tableaux: 0.3.1\ntrunk: develop\n"))
	if version.Major != 0 || version.Minor != 3 || version.Patch != 1 || version.Trunk.V != "develop" || version.Tableaux.V != "0.3.1" {
		t.Errorf("version: %+v", version)
	}
}

// T2, the half that needs no repository: a number that is no id stays a number.
func TestNumbersElsewhereStayNumbers(t *testing.T) {
	root, _, _ := readYAML("tasks/a000.yaml", []byte("title: 1000\nparent: { id: 1000, order: 1000 }\nid: 1000\nrequires:\n  - { from: 1000, text: 1000 }\n"))
	task := buildTask("a000", &model.File{Path: "tasks/a000.yaml", Root: root})
	for _, path := range [][]string{{"title"}, {"parent", "order"}, {"id"}, {"requires", "0", "from"}, {"requires", "0", "text"}} {
		v := root.At(path...)
		if v == nil || v.Kind != model.Int || v.Read != model.Int {
			t.Errorf("%v: %+v, want an Int", path, v)
		} else if n, ok := v.Plain().(int64); !ok || n != 1000 {
			t.Errorf("%v: Plain gives %#v", path, v.Plain())
		}
	}
	if task.Title.V != "1000" || task.Title.Node.Kind != model.Int {
		t.Errorf("title: %q, %s", task.Title.V, task.Title.Node.Kind)
	}
	if v := root.At("parent", "id"); v.Kind != model.String || v.Read != model.Int {
		t.Errorf("parent.id: kind %s, read %s", v.Kind, v.Read)
	}
}
