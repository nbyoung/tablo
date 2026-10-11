package derive

import (
	"testing"

	"github.com/nbyoung/tablo/internal/load"
)

// Decision 12 (a): the marks of a junction as a default drop the reviewer
// that the task's own assignee gives, and keep one an entry states.
func TestDefaultMarks(t *testing.T) {
	draws := func(marks []Mark) string {
		out := ""
		for _, m := range marks {
			out += m.Symbol()
		}
		return out
	}
	f := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	for _, c := range []struct{ task, gate, marks, asDefault string }{
		{"c07d", "unit", "🤖👀", "🤖"},          // an agent with no stated reviewer: the assignee reviews
		{"9f31", "mockup", "🧑👀", "🧑👀"},       // 4e2b's entry states the reviewer
		{"9f31", "validate", "🧑👀", "🧑👀"},     // its own entry states one
		{"4e2b", "mockup", "🧑", "🧑"},         // the reviewer is the contributor
		{"c07d", "implementation", "🪆", "🪆"}, // recursive
		{"c07d", "reliability", "—", "—"},    // not applicable
		{"a1c0", "design", "🧑", "🧑"},
	} {
		j := f.Junction(c.task, c.gate)
		if got := draws(j.Marks()); got != c.marks {
			t.Errorf("%s at %s draws %s; want %s", c.task, c.gate, got, c.marks)
		}
		if got := draws(j.DefaultMarks()); got != c.asDefault {
			t.Errorf("%s at %s draws %s as a default; want %s", c.task, c.gate, got, c.asDefault)
		}
	}
	// A parent whose entry states an agent and no reviewer: the Derivation
	// gives the parent's assignee the review, and no child inherits it.
	g := mem(t, map[string]string{
		"tasks/r000.yaml": leafOf("r@x", "", 0, "{ design: { contributor: bot@x, model: m-1 } }"),
		"tasks/a000.yaml": leafOf("a@x", "r000", 1, ""),
		"tasks/b000.yaml": leafOf("b@x", "r000", 2, "{ design: { reviewer: c@x } }"),
	})
	parent := g.Junction("r000", "design")
	if parent.Reviewer.V != "r@x" || parent.Reviewer.By != ByAssignee || draws(parent.Marks()) != "🤖👀" || draws(parent.DefaultMarks()) != "🤖" {
		t.Errorf("the parent's junction is reviewed by %+v and draws %s, as a default %s", parent.Reviewer, draws(parent.Marks()), draws(parent.DefaultMarks()))
	}
	if child := g.Junction("a000", "design"); child.Reviewer.V != "a@x" || draws(child.Marks()) != "🤖👀" {
		t.Errorf("the child's junction is reviewed by %+v and draws %s", child.Reviewer, draws(child.Marks()))
	}
	if stated := g.Junction("b000", "design"); draws(stated.DefaultMarks()) != "🤖👀" {
		t.Errorf("a stated reviewer draws %s as a default", draws(stated.DefaultMarks()))
	}
}

// Decision 12 (b): the status of the task a snapshot reads, at the tip of
// the trunk of the project it reads.
func TestAtTip(t *testing.T) {
	f := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	n := f.Snapshots("c07d")[0]
	pin := f.Pin(n.Link)
	here, tip := f.Status("c07d").Of, f.AtTip(n)
	if pin.Behind != Yes || here.Gate != "design" || tip == nil || tip.Task != "f1a0" || tip.Gate != "implementation" || tip.State != "nominal" {
		t.Fatalf("the firmware's pin is behind %v; f1a0 stands at %s there and reads %+v at the tip", pin.Behind, here.Gate, tip)
	}
	// The tip gives what the files state and no fact of the history.
	if tip.Commit != nil || tip.Date != "" || tip.Recorder != "" {
		t.Errorf("the status at the tip carries the history: %+v", tip)
	}
	if again := f.AtTip(n); again != tip {
		t.Error("a second call derives the status at the tip again")
	}
	// A pin at its tip reads there what it reads at the pin.
	sub := n.Facts
	tipOf := sub.Log().Trunk.Tip
	atTip := derived(t, load.Options{}, builtAt(t, "weather-station.firmware"), tipOf, "")
	if s := atTip.Status("f1a0"); s.Gate != tip.Gate || s.State != tip.State || s.Note != tip.Note {
		t.Errorf("the firmware at its tip reads %+v; AtTip gives %+v", s, tip)
	}

	// Undetermined: no snapshot, a link with no project, and no history.
	if f.AtTip(nil) != nil || f.AtTip(&Snapshot{}) != nil {
		t.Error("a snapshot with no facts has a status at a tip")
	}
	unmapped := derived(t, load.Options{}, builtAt(t, "subproject-by-url"), "main", "")
	if list := unmapped.Snapshots("b2c9"); len(list) != 1 || list[0].Facts != nil || unmapped.AtTip(list[0]) != nil {
		t.Errorf("a link with no project has a status at a tip: %+v", list)
	}
	// A task the tip lacks.
	if s := f.AtTip(&Snapshot{Facts: n.Facts, Target: "0000"}); s != nil {
		t.Errorf("a task the tip lacks reads %+v", s)
	}
}
