package load

import (
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// T6: the typed model holds what the files state.
func TestWeatherStation(t *testing.T) {
	p := load(t, loader(t, Options{}), entry(t, "weather-station"), "HEAD")
	if p.Gating == nil || len(p.Gating.Gates) != 12 || len(p.Gating.States) != 5 || len(p.Gating.Reasons) != 2 {
		t.Fatalf("gating: %+v", p.Gating)
	}
	if len(p.Tasks) != 6 || len(p.Statuses) != 3 {
		t.Errorf("%d tasks and %d statuses, want 6 and 3", len(p.Tasks), len(p.Statuses))
	}
	if got := strings.Join(p.TaskIDs(), " "); got != "3c5d 4e2b 7b2e 9f31 a1c0 c07d" {
		t.Errorf("tasks %s", got)
	}
	if first := p.Gating.Gates[0]; first.Key.V != "undefined" || first.Symbol.V != "❔" || first.Name.V != "Undefined" || first.Criteria.V == "" {
		t.Errorf("the first gate: %+v", first)
	}
	for _, state := range p.Gating.States {
		if !state.Severity.OK || state.Key.V == "" || state.Symbol.V == "" || state.Synopsis.V == "" {
			t.Errorf("state %q: %+v", state.Key.V, state)
		}
	}
	if p.Version == nil || p.Version.Tableaux.V == "" || !p.Version.WellFormed || !p.Version.Accepted || p.Version.Trunk.V != "main" {
		t.Errorf("version: %+v", p.Version)
	}

	firmware := p.Tasks["c07d"]
	if firmware.ID != "c07d" || firmware.Title.V != "Node firmware" || firmware.Assignee.V != "ben@example.org" || firmware.File.Path != "tasks/c07d.yaml" {
		t.Errorf("c07d: %+v", firmware)
	}
	var gates []string
	var kinds []model.JunctionKind
	for _, j := range firmware.Junctions {
		gates = append(gates, j.Gate)
		kinds = append(kinds, j.Kind())
	}
	if !reflect.DeepEqual(gates, []string{"reliability", "implementation", "unit"}) ||
		!reflect.DeepEqual(kinds, []model.JunctionKind{model.NotApplicable, model.Recursive, model.Plain}) {
		t.Errorf("c07d junctions: %v of kinds %v", gates, kinds)
	}
	reliability, implementation, unit := firmware.Junctions[0], firmware.Junctions[1], firmware.Junctions[2]
	if !reliability.Applies.OK || reliability.Applies.V {
		t.Errorf("reliability: %+v", reliability.Applies)
	}
	if sub := implementation.Subproject; sub == nil || sub.URL.V != "firmware" || sub.ID.V != "f1a0" || sub.Commit.Node != nil || !sub.ID.Node.Quoted {
		t.Errorf("implementation: %+v", sub)
	}
	if unit.Contributor.V != "opus@example.org" || unit.Model.V != "claude-opus-5-5" || unit.Reviewer.Node != nil || unit.Subproject != nil {
		t.Errorf("unit: %+v", unit)
	}
	if len(firmware.Requires) != 1 {
		t.Fatalf("c07d requires: %d", len(firmware.Requires))
	}
	if r := firmware.Requires[0]; r.ID.V != "9f31" || r.From.V != "design" || r.To.V != "implementation" || r.Text.V != "Pin map and sensor bus" || r.Subproject != nil {
		t.Errorf("c07d requires: %+v", r)
	}
	if firmware.Parent == nil || firmware.Parent.ID.V != "4e2b" || !firmware.Parent.Order.OK || firmware.Parent.Order.V != 2 {
		t.Errorf("c07d parent: %+v", firmware.Parent)
	}

	root := p.Tasks["a1c0"]
	if root.Parent != nil || root.Junctions != nil || root.Requires != nil {
		t.Errorf("a1c0: parent %+v, junctions %v, requires %v", root.Parent, root.Junctions, root.Requires)
	}
	if len(root.References) != 1 || root.References[0].URL.V != "docs/overview.md" || root.References[0].Text.V != "Project overview" {
		t.Errorf("a1c0 references: %+v", root.References)
	}
	if status := p.Statuses["9f31"]; status == nil || status.ID != "9f31" || status.Gate.V == "" || status.File.Path != "status/9f31.yaml" {
		t.Errorf("status 9f31: %+v", status)
	}
}

// T7: junction entries stay as written.
func TestJunctionsStayAsWritten(t *testing.T) {
	kinds := load(t, loader(t, Options{}), entry(t, "junction-kinds"), "HEAD")
	// Every typed field of an entry has a node exactly when the entry's own
	// mapping states the key, so nothing an ancestor states appears on it.
	entries := 0
	for _, id := range kinds.TaskIDs() {
		task := kinds.Tasks[id]
		written := task.File.Root.Get("junctions")
		if (written == nil) != (task.Junctions == nil) || (written != nil && len(written.Fields) != len(task.Junctions)) {
			t.Errorf("%s: %d junctions against the file", id, len(task.Junctions))
			continue
		}
		for i, j := range task.Junctions {
			entries++
			if j.Gate != written.Fields[i].Key || j.Node != written.Fields[i].Value {
				t.Errorf("%s: junction %d is %q, the file has %q", id, i, j.Gate, written.Fields[i].Key)
			}
			for key, present := range map[string]bool{
				"contributor": j.Contributor.Node != nil,
				"model":       j.Model.Node != nil,
				"reviewer":    j.Reviewer.Node != nil,
				"references":  j.References != nil,
				"subproject":  j.Subproject != nil,
				"applies":     j.Applies.Node != nil,
			} {
				if present != (j.Node.Get(key) != nil) {
					t.Errorf("%s %s: %s present %v, against the file", id, j.Gate, key, present)
				}
			}
		}
	}
	if entries < 10 {
		t.Errorf("junction-kinds: %d entries read", entries)
	}
	// a100 states a reviewer at unit and the root one at release; the leaf a110
	// states neither, and inherits both only in the derivation.
	leaf := kinds.Tasks["a110"]
	if len(leaf.Junctions) != 3 || leaf.Junctions[2].Gate != "unit" || leaf.Junctions[2].Reviewer.Node != nil ||
		leaf.Junctions[2].Contributor.V != "bot@example.org" || leaf.Junctions[2].Model.V != "claude-sonnet" {
		t.Errorf("a110: %+v", leaf.Junctions)
	}
	if function := leaf.Junctions[0]; len(function.References) != 1 || function.References[0].Text.V != "Function demo" || function.Kind() != model.Plain {
		t.Errorf("a110 function: %+v", function)
	}

	mixed := load(t, loader(t, Options{}), entry(t, "junction-mixed-kind"), "HEAD").Tasks["b2c9"].Junctions[0]
	if mixed.Gate != "design" || mixed.Kind() != model.Mixed || !mixed.Applies.OK || mixed.Applies.V || mixed.Contributor.V != "bot@example.org" {
		t.Errorf("junction-mixed-kind: %+v", mixed)
	}
	applies := load(t, loader(t, Options{}), entry(t, "junction-applies-true"), "HEAD").Tasks["b2c9"].Junctions[0]
	if applies.Kind() != model.NotApplicable || !applies.Applies.OK || !applies.Applies.V {
		t.Errorf("junction-applies-true: %+v", applies)
	}
}

// T8: content faults are not the Loader's. Every repository the corpus
// builds, the invalid entries among them, loads with no diagnostic, but the
// two entries of P4 and the four of L1 to L4, which the review of 2026-10-06
// added to RULES.md with a finding each in the entry's expected.yaml.
func TestContentFaultsAreNotTheLoaders(t *testing.T) {
	own := map[string]model.Diagnostic{
		"version-major-mismatch": {Code: "P4", Severity: model.Error, Pos: model.Pos{File: "version.yaml", Line: 1, Col: 11}},
		"version-minor-ahead":    {Code: "P4", Severity: model.Error, Pos: model.Pos{File: "version.yaml", Line: 1, Col: 11}},
		"file-not-yaml":          {Code: "L1", Severity: model.Error, Pos: model.Pos{File: "status/b2c9.yaml", Line: 1}, Task: "b2c9"},
		"file-duplicate-key":     {Code: "L2", Severity: model.Error, Pos: model.Pos{File: "status/b2c9.yaml", Line: 3, Col: 1}, Task: "b2c9"},
		"file-yaml-feature":      {Code: "L3", Severity: model.Error, Pos: model.Pos{File: "status/b2c9.yaml", Line: 3, Col: 1}, Task: "b2c9"},
		"file-stray-path":        {Code: "L4", Severity: model.Warning, Pos: model.Pos{File: "NOTES.md"}},
	}
	names := built(t)
	if len(names) < 100 {
		t.Errorf("the corpus builds %d repositories; it held 119 at the review", len(names))
	}
	met := 0
	for _, name := range names {
		p := load(t, loader(t, Options{}), entry(t, name), "HEAD")
		want, ok := own[name]
		if !ok {
			if len(p.Diagnostics) > 0 {
				t.Errorf("%s: %v, want no diagnostic", name, codes(p.Diagnostics))
			}
			continue
		}
		met++
		if len(p.Diagnostics) != 1 {
			t.Errorf("%s: %v, want %s alone", name, codes(p.Diagnostics), want.Code)
			continue
		}
		got := p.Diagnostics[0]
		got.Message = ""
		if got != want {
			t.Errorf("%s: %+v, want %+v", name, got, want)
		}
	}
	if met != len(own) {
		t.Errorf("the corpus holds %d of the %d entries the Loader reports on", met, len(own))
	}
	// What the model holds all the same, as each entry's expected.yaml says.
	if p := load(t, loader(t, Options{}), entry(t, "file-not-yaml"), "HEAD"); len(p.Statuses) != 0 || len(p.Tasks) != 2 || len(p.Files) != 5 {
		t.Errorf("file-not-yaml: %d statuses, %d tasks, %d files", len(p.Statuses), len(p.Tasks), len(p.Files))
	}
	if p := load(t, loader(t, Options{}), entry(t, "file-duplicate-key"), "HEAD"); p.Statuses["b2c9"] == nil || p.Statuses["b2c9"].Gate.V != "defined" {
		t.Errorf("file-duplicate-key: the status reads %+v, want the first gate, defined", p.Statuses["b2c9"])
	}
	if p := load(t, loader(t, Options{}), entry(t, "file-yaml-feature"), "HEAD"); p.Statuses["b2c9"] == nil || p.Statuses["b2c9"].Gate.V != "defined" {
		t.Errorf("file-yaml-feature: the status reads %+v, want the first document", p.Statuses["b2c9"])
	}
	if p := load(t, loader(t, Options{}), entry(t, "file-stray-path"), "HEAD"); !reflect.DeepEqual(p.Stray, []string{"NOTES.md"}) || len(p.Files) != 5 {
		t.Errorf("file-stray-path: stray %v, %d files", p.Stray, len(p.Files))
	}
}

// T9: presence shows in the model, for the Validator to report.
func TestPresence(t *testing.T) {
	l := loader(t, Options{})
	none, err := l.Load(t.Context(), Source{Dir: entry(t, "no-tableaux-directory"), Ref: "HEAD"})
	if err != nil || none.Exists || none.Diagnostics != nil || none.Tasks != nil || none.Files != nil || none.Version != nil {
		t.Errorf("no-tableaux-directory: %+v, %v", none, err)
	}
	if none != nil && (none.Where.Dir != "" || none.Where.Commit == "" || none.Where.GitDir == "") {
		t.Errorf("no-tableaux-directory: read at %+v", none.Where)
	}
	if tree := load(t, l, entry(t, "no-tableaux-directory"), ""); tree.Exists || len(tree.Diagnostics) > 0 {
		t.Errorf("no-tableaux-directory, the working tree: %+v", tree)
	}
	if p := load(t, l, entry(t, "version-missing"), "HEAD"); !p.Exists || p.Version != nil || p.Gating == nil || len(p.Diagnostics) > 0 {
		t.Errorf("version-missing: version %+v, gating %v", p.Version, p.Gating != nil)
	}
	if p := load(t, l, entry(t, "gates-missing"), "HEAD"); !p.Exists || p.Gating != nil || p.Version == nil || len(p.Diagnostics) > 0 {
		t.Errorf("gates-missing: gating %+v, version %v", p.Gating, p.Version != nil)
	}
	if p := load(t, l, entry(t, "version-bad-pattern"), "HEAD"); p.Version == nil || p.Version.WellFormed || p.Version.Accepted ||
		p.Version.Tableaux.V != "0.2" || len(p.Diagnostics) > 0 {
		t.Errorf("version-bad-pattern: %+v, %v", p.Version, codes(p.Diagnostics))
	}
	if p := load(t, l, entry(t, "task-bad-filename"), "HEAD"); p.Tasks["leaf-two"] == nil || p.Tasks["leaf-two"].ID != "leaf-two" ||
		p.Tasks["leaf-two"].File.Path != "tasks/leaf-two.yaml" || len(p.Stray) > 0 {
		t.Errorf("task-bad-filename: tasks %v, stray %v", p.TaskIDs(), p.Stray)
	}
}
