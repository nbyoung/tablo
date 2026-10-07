package derive

import (
	"reflect"
	"testing"
)

// T13, the cases of cases.md: R1 to R10 roll the children of r000 up.
func TestRollUpCases(t *testing.T) {
	root := leafOf("r@x", "", 0, "")
	child := func(order int, junctions string) string { return leafOf("c@x", "r000", order, junctions) }
	for _, tc := range []struct {
		name              string
		files             map[string]string
		gate, state, from string
		considered        []string
		end               string // the leaf the chain ends at
	}{
		{name: "R1", files: map[string]string{
			"tasks/a000.yaml": child(1, ""), "status/a000.yaml": at("function", "nominal"),
			"tasks/b000.yaml":  child(2, "{ mockup: { applies: false }, function: { applies: false } }"),
			"status/b000.yaml": at("defined", "nominal"),
		}, gate: "defined", state: "nominal", from: "b000", considered: []string{"a000", "b000"}},
		{name: "R2", files: map[string]string{
			"tasks/a000.yaml": child(1, ""), "status/a000.yaml": at("mockup", "stalled"),
			"tasks/b000.yaml": child(2, "{ mockup: { applies: false } }"), "status/b000.yaml": at("design", "nominal"),
		}, gate: "mockup", state: "stalled", from: "a000", considered: []string{"a000", "b000"}},
		{name: "R3", files: map[string]string{
			"tasks/r000.yaml": leafOf("r@x", "", 0, "{ mockup: { applies: false } }"),
			"tasks/a000.yaml": child(1, "{ mockup: {} }"), "status/a000.yaml": at("mockup", "nominal"),
			"tasks/b000.yaml": child(2, ""), "status/b000.yaml": at("design", "nominal"),
		}, gate: "mockup", state: "nominal", from: "a000", considered: []string{"a000", "b000"}},
		{name: "R4", files: map[string]string{
			"tasks/a000.yaml": child(1, ""), "status/a000.yaml": at("release", "complete"),
			"tasks/b000.yaml": child(2, "{ release: { applies: false } }"), "status/b000.yaml": at("design", "complete"),
		}, gate: "design", state: "complete", from: "b000", considered: []string{"a000", "b000"}},
		{name: "R5", files: map[string]string{
			"tasks/a000.yaml": child(1, ""), "status/a000.yaml": at("release", "complete"),
			"tasks/b000.yaml": child(2, ""),
		}, gate: "undefined", state: "undefined", from: "b000", considered: []string{"a000", "b000"}},
		{name: "R6", files: map[string]string{
			"tasks/a000.yaml": child(1, ""), "status/a000.yaml": at("defined", "nominal"),
			"tasks/b000.yaml": child(2, ""), "status/b000.yaml": at("design", "stalled"),
		}, gate: "defined", state: "nominal", from: "a000", considered: []string{"a000", "b000"}},
		{name: "R7", files: map[string]string{
			"tasks/b000.yaml": child(1, ""), "status/b000.yaml": at("design", "nominal"),
			"tasks/a000.yaml": child(2, ""), "status/a000.yaml": at("design", "nominal"),
		}, gate: "design", state: "nominal", from: "b000", considered: []string{"b000", "a000"}},
		{name: "R8", files: map[string]string{
			"tasks/a000.yaml": child(1, ""), "status/a000.yaml": at("defined", "paused"),
			"tasks/b000.yaml": child(2, ""), "status/b000.yaml": at("nowhere", "nominal"),
			"tasks/c000.yaml": child(3, ""), "status/c000.yaml": at("design", "nominal"),
		}, gate: "design", state: "nominal", from: "c000", considered: []string{"c000"}},
		{name: "R9", files: map[string]string{
			"tasks/a000.yaml": child(1, ""), "status/a000.yaml": at("defined", "paused"),
		}, gate: "defined", state: "paused", from: "a000", considered: []string{"a000"}},
		{name: "R10", files: map[string]string{
			"tasks/p000.yaml": child(1, ""),
			"tasks/a000.yaml": leafOf("c@x", "p000", 1, ""), "status/a000.yaml": at("mockup", "stalled") + "note: Held\n",
			"tasks/b000.yaml": child(2, ""), "status/b000.yaml": at("mockup", "nominal"),
		}, gate: "mockup", state: "stalled", from: "p000", considered: []string{"p000", "b000"}, end: "a000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := tc.files["tasks/r000.yaml"]; !ok {
				tc.files["tasks/r000.yaml"] = root
			}
			f := mem(t, tc.files)
			s := f.Status("r000")
			if s == nil || s.Kind != RolledUp || !s.Derived() || s.File != nil || s.Why != Determined {
				t.Fatalf("the root's status is %+v; want a roll-up", s)
			}
			if s.Gate != tc.gate || s.State != tc.state || s.From != tc.from {
				t.Errorf("r000 rolls up to %s, %s from %s; want %s, %s from %s", s.Gate, s.State, s.From, tc.gate, tc.state, tc.from)
			}
			if !reflect.DeepEqual(s.Considered, tc.considered) {
				t.Errorf("the children considered are %v; want %v", s.Considered, tc.considered)
			}
			chain := f.Chain("r000")
			end := tc.end
			if end == "" {
				end = tc.from
			}
			if last := chain[len(chain)-1]; chain[0] != s || last.Task != end || last.Derived() || last.Note != s.Note {
				t.Errorf("the chain ends at %+v; want the leaf %s, whose file states the note", last, end)
			}
		})
	}
}

// T13: a parent with nothing to roll up, the leaves' own statuses, and a
// task the tree lacks.
func TestStatusOfALeaf(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/r000.yaml": leafOf("r@x", "", 0, ""),
		"tasks/p000.yaml": leafOf("p@x", "r000", 1, ""),
		"tasks/a000.yaml": leafOf("a@x", "p000", 1, ""), "status/a000.yaml": "gate: nowhere\nstate: nominal\n",
		"tasks/b000.yaml": leafOf("b@x", "r000", 2, ""), "status/b000.yaml": "gate: design\nreason: blocked\nnote: Waits\n",
		"tasks/c000.yaml":  leafOf("c@x", "r000", 3, ""),
		"status/p000.yaml": at("release", "complete"), "status/zzzz.yaml": at("design", "nominal"),
	})
	if s := f.Status("p000"); s.Kind != RolledUp || s.Why != NoChildren || s.Gate != "" || s.From != "" || s.File != nil {
		t.Errorf("a parent whose one child stands at no gate is %+v; want NoChildren, whatever its own file states", s)
	}
	if s := f.Status("b000"); s.Kind != Recorded || s.Gate != "design" || s.State != "" || s.Why != NoState ||
		s.Reason != "blocked" || s.Note != "Waits" || s.File == nil || s.Derived() {
		t.Errorf("a file with no state reads as %+v; want NoState", s)
	}
	if s := f.Status("c000"); s.Kind != Absent || s.Gate != "undefined" || s.State != "undefined" || s.File != nil || s.Why != Determined {
		t.Errorf("a leaf with no file reads as %+v; want undefined at undefined", s)
	}
	if f.Status("zzzz") != nil || f.Chain("zzzz") != nil || (*Status)(nil).Derived() {
		t.Error("a task the tree lacks has a status")
	}
	// p000 stays out of the root's roll-up; b000 has no severity and c000 is undefined.
	if s := f.Status("r000"); s.Gate != "undefined" || s.From != "c000" || !reflect.DeepEqual(s.Considered, []string{"b000", "c000"}) {
		t.Errorf("the root rolls up to %+v", s)
	}
	if chain := f.Chain("r000"); len(chain) != 2 || chain[1].Kind != Absent {
		t.Errorf("the chain is %v; want the root and c000", chain)
	}
}
