package derive

import (
	"reflect"
	"testing"
)

// T9, the cases the corpus lacks: a task at its last gate has its
// requirement due; a requirement on a task that is not there, on a gate that
// is none, and on a subproject with no link are undetermined.
func TestConditions(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/r000.yaml": leafOf("r@x", "", 0, ""),
		"tasks/a000.yaml": leafOf("a@x", "r000", 1, "") +
			"requires:\n  - { id: \"b000\" }\n  - { id: \"zzzz\" }\n  - { id: \"b000\", from: nowhere }\n" +
			"  - { id: \"b000\", to: nowhere }\n  - { subproject: { url: lib, id: \"5a00\" } }\n  - { id: \"p000\", from: mockup, to: design }\n",
		"status/a000.yaml": at("release", "complete"),
		"tasks/b000.yaml":  leafOf("b@x", "r000", 2, "{ release: { applies: false } }") + "requires:\n  - { id: \"a000\", from: design, to: design }\n",
		"status/b000.yaml": at("mockup", "nominal"),
		"tasks/p000.yaml":  leafOf("p@x", "r000", 3, "") + "requires:\n  - { id: \"a000\" }\n",
		"tasks/c000.yaml":  leafOf("c@x", "p000", 1, ""), "status/c000.yaml": at("function", "nominal"),
		"tasks/d000.yaml": leafOf("d@x", "p000", 2, ""), "status/d000.yaml": at("mockup", "stalled"),
	})
	list := f.Requires("a000")
	if len(list) != 6 {
		t.Fatalf("a000 has %d conditions; want 6", len(list))
	}
	// from defaults to b000's last applicable gate, design; to to a000's first, defined.
	if c := list[0]; c.From != "design" || c.To != "defined" || c.Stands != "mockup" || c.Met || !c.Due || c.Word() != "unmet" ||
		c.Why != Determined || c.Task != "a000" || c.Origin != "b000" || c.Facts != f || c.Link != nil || c.Entry == nil {
		t.Errorf("at its last gate a000's requirement is %+v; want it due and unmet", c)
	}
	for i, why := range map[int]Why{1: NoTask, 2: NoGate, 3: NoGate, 4: NoLink} {
		if c := list[i]; c.Why != why || c.Met || c.Due || c.Word() != "" {
			t.Errorf("condition %d is %+v; want it undetermined with %v", i, c, why)
		}
	}
	// A requirement on a parent reads the parent's roll-up gate: p000 stands at mockup.
	if c := list[5]; c.Stands != "mockup" || !c.Met || !c.Due || c.Word() != "met" {
		t.Errorf("the requirement on a parent is %+v; want it met at the roll-up's gate", c)
	}
	// b000 stands at mockup: function lies between it and design, so the entry is pending.
	if c := f.Requires("b000")[0]; c.Met != true || c.Due || c.Word() != "met" || c.Stands != "release" {
		t.Errorf("b000's requirement is %+v; want it met and not due", c)
	}
	// A requirement a parent holds reads the parent's own roll-up gate too.
	if c := f.Requires("p000")[0]; c.From != "release" || c.To != "defined" || !c.Met || !c.Due {
		t.Errorf("p000's requirement is %+v", c)
	}
	var dependents []string
	for _, c := range f.Dependents("b000") {
		dependents = append(dependents, c.Task+" "+c.From+" "+c.To)
	}
	if want := []string{"a000 design defined", "a000 nowhere defined", "a000 design nowhere"}; !reflect.DeepEqual(dependents, want) {
		t.Errorf("b000's dependents are %q; want %q", dependents, want)
	}
	if got := f.Dependents("a000"); len(got) != 2 || got[0].Task != "b000" || got[1].Task != "p000" {
		t.Errorf("a000's dependents are %v; want b000's and p000's entries, in display order", got)
	}
	if f.Requires("zzzz") != nil || len(f.Requires("r000")) != 0 || f.Dependents("none") != nil {
		t.Error("a task the tree lacks has a condition")
	}
}
