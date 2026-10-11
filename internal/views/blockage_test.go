package views

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// causesOf writes each cause of a tree as its kind, its task, its gate, its
// resolver, its mark, its act and its count, with what a snapshot advances.
func causesOf(b *view.Blockage) []string {
	out := []string{}
	for _, c := range b.Causes {
		text := fmt.Sprintf("%s %s %s %s %s %s holds %d", c.Kind, c.Task.ID, c.Gate, c.Resolver, c.Mark, c.Act, c.Holds)
		if c.Advance != "" {
			text += " " + c.Advance + " of " + c.URL
		}
		out = append(out, strings.Join(strings.Fields(text), " "))
	}
	return out
}

// heldOf writes the tree beneath a list of held tasks, each as its task,
// its gate and the way it is held, with what it holds in parentheses.
func heldOf(list []view.Held) string {
	var out []string
	for _, h := range list {
		text := h.Task.ID + "@" + h.Gate + " " + h.Via
		if h.Child != "" {
			text += " of " + h.Child
		}
		if len(h.Also) > 0 {
			text += fmt.Sprintf(" also %v", h.Also)
		}
		if len(h.Held) > 0 {
			text += " (" + heldOf(h.Held) + ")"
		}
		out = append(out, text)
	}
	return strings.Join(out, ", ")
}

// kindsOf returns the counts of a tree's kinds, in their order.
func kindsOf(b *view.Blockage) string {
	var out []string
	for _, k := range b.Kinds {
		out = append(out, fmt.Sprintf("%s %d", k.Kind, k.Count))
	}
	return strings.Join(out, ", ")
}

// T16: the causes of the weather station.
func TestCauses(t *testing.T) {
	f, hashes := station(t, "main")
	firmware := labels(t, "weather-station.firmware")
	glance := Blockage(f, Query{Task: "a1c0"})
	want := []string{
		"status 9f31 function ada@example.org person records holds 3",
		"authorisation 3c5d defined ada@example.org authorises holds 2",
		"snapshot c07d design ben@example.org subproject advances holds 1 pin of firmware",
	}
	if got := causesOf(glance); !reflect.DeepEqual(got, want) {
		t.Fatalf("the causes are %q; want %q", got, want)
	}
	if got := kindsOf(glance); got != "requirement 0, status 1, review 0, authorisation 1, snapshot 1" {
		t.Errorf("the kinds are %s", got)
	}
	if glance.NotDueCount != 1 || glance.NotDueUnmet != 1 || glance.BlockageDetail != nil || glance.BlockageProvenance != nil || glance.Causes[0].CauseDetail != nil || glance.Causes[0].CauseProvenance != nil {
		t.Errorf("the glance counts %d requirements not yet due, %d unmet, or carries a level it does not reach", glance.NotDueCount, glance.NotDueUnmet)
	}
	// The firmware's pin is behind its trunk, and the task read stands at
	// implementation at the tip: the pin advances.
	if c := glance.Causes[2]; c.Pin != firmware["F2"] || c.Tip != firmware["F3"] || c.Status.Kind != "snapshot" || c.Task.Title != "Node firmware" {
		t.Errorf("the snapshot cause reads %+v", c)
	}

	detail := Blockage(f, Query{Task: "a1c0", Level: Detail})
	for i, want := range []string{
		"9f31@performance self (4e2b@performance parent of 9f31, c07d@implementation requirement also [3])",
		"3c5d@mockup self (a1c0@mockup parent of 3c5d)",
		"c07d@implementation self also [1]",
	} {
		if got := heldOf(detail.Causes[i].Held); got != want {
			t.Errorf("cause %d holds %q; want %q", i+1, got, want)
		}
	}
	board := detail.Causes[0].Held[0]
	if board.Contributor != "ada@example.org" || board.Parent || board.Requires != nil || !board.Held[0].Parent || board.Held[0].Contributor != "" ||
		board.Held[1].Requires == nil || board.Held[1].Requires.Task.ID != "9f31" || board.Held[1].Requires.Condition != "unmet" || board.Held[1].Contributor != "" {
		t.Errorf("the status cause holds %+v", board)
	}
	if len(detail.NotDue) != 1 || detail.NotDue[0].Task.ID != "3c5d" || detail.NotDue[0].Next != "mockup" || detail.NotDue[0].Status.Gate != "defined" ||
		len(detail.NotDue[0].Requires) != 1 || detail.NotDue[0].Requires[0].Task.ID != "7b2e" || detail.NotDue[0].Requires[0].Condition != "pending" {
		t.Errorf("the requirements not yet due are %+v", detail.NotDue)
	}

	p := Blockage(f, Query{Task: "a1c0", Level: Provenance})
	status, authorisation, snapshot := p.Causes[0], p.Causes[1], p.Causes[2]
	if status.StatusCommit == nil || status.StatusCommit.Hash != hashes["W13"] || status.Junction == nil || status.Junction.Gate != "performance" || status.Junction.From == nil ||
		status.Authorisation != nil || status.Linkage != nil || status.Resolve == nil || len(status.Resolve) != 0 {
		t.Errorf("the status cause's provenance is %+v", status.CauseProvenance)
	}
	if a := authorisation.Authorisation; a == nil || a.Commit == nil || a.Commit.Hash != hashes["W12"] || len(authorisation.Resolve) != 1 ||
		authorisation.Resolve[0].Text != "git commit --allow-empty --trailer 'Authorised: 3c5d'" || authorisation.Junction == nil || authorisation.Junction.Gate != "mockup" {
		t.Errorf("the authorisation cause's provenance is %+v", authorisation.CauseProvenance)
	}
	if l := snapshot.Linkage; l == nil || l.URL != "firmware" || l.Commit != firmware["F2"] || l.Tip != firmware["F3"] || l.Behind != "yes" || snapshot.Junction == nil || snapshot.Junction.Kind != "recursive" ||
		snapshot.StatusCommit == nil || snapshot.StatusCommit.Hash != firmware["F2"] || len(snapshot.Resolve) != 0 {
		t.Errorf("the snapshot cause's provenance is %+v with %+v", snapshot.CauseProvenance, snapshot.Linkage)
	}
	command := "git log --format='%H %as %ae %ce %s' " + hashes["W13"] + " -- .tableaux/status .tableaux/tasks"
	if len(p.Commands) != 1 || p.Commands[0].Text != command {
		t.Errorf("the commands are %+v; want %s", p.Commands, command)
	}
}

// Decision 12: a status is a cause by structure. Any reason but the
// reserved one makes it one, and so does a state more severe than the
// project's mildest working state, whatever gates.yaml calls its states.
func TestStatusCauseByStructure(t *testing.T) {
	gates := `gates:
  - { key: undefined }
  - { key: defined }
  - { key: built }
states:
  - { key: undefined, severity: 0 }
  - { key: steady, severity: 2 }
  - { key: wobbly, severity: 4 }
  - { key: sunk, severity: 7 }
  - { key: complete, severity: 0 }
reasons:
  - { key: review }
  - { key: weather }
`
	project := func(status string) *view.Blockage {
		return Blockage(mem(t, map[string]string{
			"gates.yaml":       gates,
			"tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
			"tasks/a000.yaml":  leafOf("a@x", "r000", 1, ""),
			"status/a000.yaml": status,
		}), Query{Task: "r000"})
	}
	for status, want := range map[string]int{
		"gate: defined\nstate: steady\n":                  0, // the mildest working state
		"gate: defined\nstate: wobbly\n":                  1,
		"gate: defined\nstate: sunk\n":                    1,
		"gate: built\nstate: complete\n":                  0,
		"gate: defined\nstate: steady\nreason: weather\n": 1,
		"gate: defined\nstate: steady\nreason: review\n":  0, // the reserved reason is the hand-off's
	} {
		got := project(status)
		if len(got.Causes) != want {
			t.Errorf("%q gives the causes %q; want %d", status, causesOf(got), want)
		}
		for _, c := range got.Causes {
			if c.Kind != "status" || c.Resolver != "a@x" || c.Act != "records" || c.Holds != 2 {
				t.Errorf("%q gives %q", status, causesOf(got))
			}
		}
	}
	// In the queue the reason holds the work, and the state alone does not.
	queue := func(status string) []string {
		return itemsOf(Queue(mem(t, map[string]string{
			"gates.yaml":       gates,
			"tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
			"tasks/a000.yaml":  leafOf("a@x", "r000", 1, ""),
			"status/a000.yaml": status,
		}), Query{Task: "r000", Person: "a@x"}))
	}
	if got := queue("gate: defined\nstate: steady\nreason: weather\n"); !reflect.DeepEqual(got, []string{"waiting a000 built reason:weather"}) {
		t.Errorf("with a reason the queue is %q", got)
	}
	if got := queue("gate: defined\nstate: sunk\n"); !reflect.DeepEqual(got, []string{"ready a000 built"}) {
		t.Errorf("with a severe state the queue is %q", got)
	}
}

// T17: a requirement's root.
func TestRequirementRoot(t *testing.T) {
	requires := func(entry string) string { return "requires:\n  - " + entry + "\n" }
	files := func(state string) map[string]string {
		return map[string]string{
			"tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
			"tasks/a000.yaml":  leafOf("a@x", "r000", 1, ""),
			"status/a000.yaml": "gate: defined\nstate: " + state + "\n",
			"tasks/b000.yaml":  leafOf("b@x", "r000", 2, "") + requires("{ id: a000, from: function, to: mockup }"),
			"status/b000.yaml": "gate: defined\nstate: nominal\n",
		}
	}
	// A leaf that nothing holds roots the cause at its next gate, where its
	// contributor acts. The cause holds what the leaf holds, not the leaf.
	free := Blockage(mem(t, files("nominal")), Query{Task: "r000", Level: Detail})
	if got, want := causesOf(free), []string{"requirement a000 mockup a@x person contributes holds 2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the causes are %q; want %q", got, want)
	}
	if got, want := heldOf(free.Causes[0].Held), "r000@mockup parent of a000, b000@mockup requirement (r000@mockup parent of b000)"; got != want {
		t.Errorf("the requirement cause holds %q; want %q", got, want)
	}
	if c := free.Causes[0]; c.Status.Gate != "defined" || c.URL != "" || c.Advance != "" || c.Pin != "" {
		t.Errorf("the requirement cause reads %+v", c)
	}
	// The same leaf with a stalled status roots none: the status cause holds
	// the dependent through the leaf, so one fact gives one cause.
	stalled := Blockage(mem(t, files("stalled")), Query{Task: "r000", Level: Detail})
	if got, want := causesOf(stalled), []string{"status a000 defined a@x person records holds 3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("with a stalled leaf the causes are %q; want %q", got, want)
	}
	if got, want := heldOf(stalled.Causes[0].Held), "a000@mockup self (r000@mockup parent of a000, b000@mockup requirement (r000@mockup parent of b000))"; got != want {
		t.Errorf("the status cause holds %q; want %q", got, want)
	}

	// A parent's requirement roots at the child that stands at its gate,
	// among the children its roll-up considers.
	parent := mem(t, map[string]string{
		"tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
		"tasks/f000.yaml":  leafOf("p@x", "r000", 1, ""),
		"tasks/a000.yaml":  leafOf("a@x", "f000", 1, "{ mockup: { contributor: bot@x, model: m-1 } }"),
		"status/a000.yaml": "gate: defined\nstate: nominal\n",
		"tasks/c000.yaml":  leafOf("c@x", "f000", 2, ""),
		"status/c000.yaml": "gate: function\nstate: nominal\n",
		"tasks/b000.yaml":  leafOf("b@x", "r000", 2, "") + requires("{ id: f000, from: function, to: mockup }"),
		"status/b000.yaml": "gate: defined\nstate: nominal\n",
	})
	tree := Blockage(parent, Query{Task: "r000", Level: Detail})
	if got, want := causesOf(tree), []string{"requirement a000 mockup bot@x agent contributes holds 3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("under a parent the causes are %q; want %q", got, want)
	}
	if got, want := heldOf(tree.Causes[0].Held), "f000@mockup parent of a000 (r000@mockup parent of f000, b000@mockup requirement (r000@mockup parent of b000))"; got != want {
		t.Errorf("under a parent the cause holds %q; want %q", got, want)
	}
	// A leaf that waits on another carries that cause on: the chain roots at its far end.
	chain := files("nominal")
	chain["tasks/a000.yaml"] = leafOf("a@x", "r000", 1, "") + requires("{ id: c000, from: function, to: mockup }")
	chain["tasks/c000.yaml"] = leafOf("c@x", "r000", 3, "")
	chain["status/c000.yaml"] = "gate: mockup\nstate: nominal\n"
	far := Blockage(mem(t, chain), Query{Task: "r000", Level: Detail})
	if got, want := causesOf(far), []string{"requirement c000 function c@x person contributes holds 3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("along a chain the causes are %q; want %q", got, want)
	}
	if got, want := heldOf(far.Causes[0].Held), "a000@mockup requirement (r000@mockup parent of a000, b000@mockup requirement (r000@mockup parent of b000))"; got != want {
		t.Errorf("along a chain the cause holds %q; want %q", got, want)
	}
	// The corpus's own case.
	if got, want := causesOf(Blockage(entry(t, "unmet-requirement"), Query{})), 1; len(got) != want || !strings.HasPrefix(got[0], "requirement ") {
		t.Errorf("unmet-requirement: the causes are %q", got)
	}
}

// T18: a pin. A snapshot is a cause in two ways, and a pin that is merely
// behind its trunk is none (decision 11).
func TestSnapshotCauses(t *testing.T) {
	// A due requirement that the commit alone leaves unmet: the pin advances.
	behind := entry(t, "requires-commit-behind")
	lib := labels(t, "requires-commit-behind.lib")
	tree := Blockage(behind, Query{Task: "e4a1", Level: Provenance})
	if got, want := causesOf(tree), []string{"snapshot b2c9 design pat@example.org subproject advances holds 2 pin of https://example.org/lib.git"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("requires-commit-behind: the causes are %q; want %q", got, want)
	}
	c := tree.Causes[0]
	if got, want := heldOf(c.Held), "b2c9@release requirement (e4a1@release parent of b2c9)"; got != want {
		t.Errorf("requires-commit-behind: the cause holds %q; want %q", got, want)
	}
	if c.Pin != lib["L1"] || c.Tip != lib["L2"] || c.Held[0].Requires == nil || c.Held[0].Requires.AtTrunk != "yes" || c.Linkage == nil || c.Linkage.URL != "https://example.org/lib.git" ||
		c.Linkage.Commit != lib["L1"] || c.Linkage.Behind != "yes" || c.Linkage.Pinned == nil || c.Linkage.Pinned.Hash != lib["L1"] {
		t.Errorf("requires-commit-behind: the cause reads %+v with %+v", c, c.Linkage)
	}

	// The same requirement where the tip does not meet it either: the cause
	// is the requirement, at the task of the other project.
	repo := filepath.Join(t.TempDir(), "lib")
	copyTree(t, builtAt(t, "requires-commit-behind.lib"), repo)
	gitRun(t, repo, "update-ref", "refs/heads/main", lib["L1"])
	held := derived(t, load.Options{Replace: map[string]string{"https://example.org/lib.git": repo}}, builtAt(t, "requires-commit-behind"), "main", "")
	if c := held.Requires("b2c9")[0]; !c.Due || c.Met || c.AtTrunk.String() != "no" {
		t.Fatalf("with the library's trunk at L1 the condition is %+v", c)
	}
	other := Blockage(held, Query{Task: "e4a1", Level: Provenance})
	if got := causesOf(other); len(got) != 1 || !strings.HasPrefix(got[0], "requirement 5a00 design ") || !strings.HasSuffix(got[0], " contributes holds 2") {
		t.Fatalf("with the library's trunk at L1 the causes are %q", got)
	}
	far := other.Causes[0]
	if far.URL != "https://example.org/lib.git" || far.Task.Title == "" || far.Status.Gate != "defined" || far.Resolver == "" || far.Mark != "person" || far.Advance != "" ||
		heldOf(far.Held) != "b2c9@release requirement (e4a1@release parent of b2c9)" || far.Junction == nil || far.Junction.Gate != "design" || far.StatusCommit == nil || far.StatusCommit.Hash != lib["L1"] {
		t.Errorf("the cross-project cause reads %+v with %+v", far, far.CauseProvenance)
	}
	// The cause's task is of another project: a subtree of this one keeps
	// the cause for the task it holds.
	if got := causesOf(Blockage(held, Query{Task: "b2c9"})); len(got) != 1 {
		t.Errorf("under b2c9 the causes are %q", got)
	}

	// A requirement not yet due is no cause.
	across := Blockage(entry(t, "requires-across-projects"), Query{Task: "e4a1", Level: Detail})
	if len(across.Causes) != 0 || across.NotDueCount != 1 || across.NotDueUnmet != 1 || len(across.NotDue) != 1 || across.NotDue[0].Task.ID != "c3d7" || across.NotDue[0].Requires[0].Subproject != "sub" {
		t.Errorf("requires-across-projects: the causes are %q, with %d requirements not yet due: %+v", causesOf(across), across.NotDueCount, across.NotDue)
	}

	// junction-kinds reads two tasks of one subproject behind its trunk.
	// The task a500 reads stands elsewhere at the tip: the pin advances.
	// The task a400 reads stands there as at the pin: merely behind, no cause.
	kinds := entry(t, "junction-kinds")
	for _, id := range []string{"a400", "a500"} {
		n := kinds.Snapshots(id)[0]
		if pin := kinds.Pin(n.Link); pin.Behind.String() != "yes" {
			t.Fatalf("junction-kinds: the pin of %s is behind %v", id, pin.Behind)
		}
	}
	if at, of := kinds.AtTip(kinds.Snapshots("a400")[0]), kinds.Status("a400").Of; at == nil || at.Gate != of.Gate || at.State != of.State {
		t.Fatalf("junction-kinds: a400 reads %+v at the tip and %+v at the pin", at, of)
	}
	got := Blockage(kinds, Query{Level: Detail})
	var snapshots []string
	for i, c := range got.Causes {
		if c.Kind == "snapshot" {
			snapshots = append(snapshots, causesOf(got)[i]+": "+heldOf(c.Held))
		}
	}
	if len(snapshots) != 1 || !strings.HasPrefix(snapshots[0], "snapshot a500 implementation ") || !strings.Contains(snapshots[0], " pin of sub: a500@unit self") {
		t.Errorf("junction-kinds: the snapshot causes are %q", snapshots)
	}

	// subproject-by-url: the task read stands at design at the pin, past the
	// next gate of the leaf that reads it, and at release at the tip. Both
	// hold: the leaf's gate advances, and the pin does.
	url := Blockage(derived(t, load.Options{Replace: map[string]string{"https://example.org/lib.git": builtAt(t, "subproject-by-url.lib")}},
		builtAt(t, "subproject-by-url"), "main", ""), Query{Level: Provenance})
	want := []string{
		"snapshot b2c9 defined pat@example.org subproject advances holds 2 pin of https://example.org/lib.git",
		"snapshot b2c9 defined pat@example.org subproject advances holds 1 gate of https://example.org/lib.git",
	}
	if got := causesOf(url); !reflect.DeepEqual(got, want) {
		t.Fatalf("subproject-by-url: the causes are %q; want %q", got, want)
	}
	if pin, gate := heldOf(url.Causes[0].Held), heldOf(url.Causes[1].Held); pin != "b2c9@design self (e4a1@design parent of b2c9 also [2])" || gate != "e4a1@design parent of b2c9 also [1]" {
		t.Errorf("subproject-by-url: the causes hold %q and %q", pin, gate)
	}
	if l := url.Causes[1].Linkage; l == nil || l.Behind != "yes" || url.Causes[1].Pin == "" || url.Causes[1].Tip == url.Causes[1].Pin {
		t.Errorf("subproject-by-url: the gate cause reads the linkage %+v", l)
	}
	// With no clone the link has no project: nothing is read, so no cause.
	if got := causesOf(Blockage(entry(t, "subproject-url-without-commit"), Query{})); len(got) != 0 {
		t.Errorf("subproject-url-without-commit: the causes are %q", got)
	}
}

// T19: a cycle ends, and the selection by task and by person.
func TestBlockageSelection(t *testing.T) {
	cycle := entry(t, "requires-cycle")
	if cycle.Refused() != nil {
		t.Fatalf("requires-cycle is refused: %+v", cycle.Refused())
	}
	for level := range levels {
		got := Blockage(cycle, Query{Task: cycle.Root(), Level: level})
		if got.Causes == nil || len(got.Kinds) != 5 {
			t.Errorf("requires-cycle at %s: the tree is %+v", levels[level], got)
		}
	}
	// Two leaves that hold each other, under a status cause: the tree ends
	// where a task is on the path already.
	loop := mem(t, map[string]string{
		"tasks/r000.yaml":  leafOf("r@x", "", 0, ""),
		"tasks/a000.yaml":  leafOf("a@x", "r000", 1, "") + "requires:\n  - { id: b000, from: function, to: mockup }\n",
		"status/a000.yaml": "gate: defined\nstate: stalled\n",
		"tasks/b000.yaml":  leafOf("b@x", "r000", 2, "") + "requires:\n  - { id: a000, from: function, to: mockup }\n",
		"status/b000.yaml": "gate: defined\nstate: nominal\n",
	})
	tree := Blockage(loop, Query{Task: "r000", Level: Detail})
	if got, want := causesOf(tree), []string{"status a000 defined a@x person records holds 3"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("the loop's causes are %q; want %q", got, want)
	}
	if got, want := heldOf(tree.Causes[0].Held), "a000@mockup self (r000@mockup parent of a000, b000@mockup requirement (r000@mockup parent of b000))"; got != want {
		t.Errorf("the loop's cause holds %q; want %q", got, want)
	}

	f, _ := station(t, "main")
	for _, c := range []struct {
		name   string
		q      Query
		want   []string
		notDue int
	}{
		{"under 9f31", Query{Task: "9f31"}, []string{"status 9f31"}, 0},
		{"under 4e2b", Query{Task: "4e2b"}, []string{"status 9f31", "snapshot c07d"}, 0},
		{"under 3c5d", Query{Task: "3c5d"}, []string{"authorisation 3c5d"}, 1},
		{"under 7b2e", Query{Task: "7b2e"}, []string{}, 0},
		// dan's own task is 3c5d; ada resolves two causes and her own 9f31
		// stands under the first; ben resolves the snapshot, and the status
		// cause holds his own 4e2b and c07d.
		{"for dan", Query{Task: "a1c0", Person: "dan@example.org"}, []string{"authorisation 3c5d"}, 1},
		{"for ada", Query{Task: "a1c0", Person: "ada@example.org"}, []string{"status 9f31", "authorisation 3c5d"}, 0},
		{"for ben", Query{Task: "a1c0", Person: "ben@example.org"}, []string{"status 9f31", "snapshot c07d"}, 0},
		{"for a stranger", Query{Task: "a1c0", Person: "eve@example.org"}, []string{}, 0},
		{"for ada under 4e2b", Query{Task: "4e2b", Person: "ada@example.org"}, []string{"status 9f31"}, 0},
	} {
		c.q.Level = Detail
		got := Blockage(f, c.q)
		kept := []string{}
		for _, cause := range got.Causes {
			kept = append(kept, cause.Kind+" "+cause.Task.ID)
		}
		if !reflect.DeepEqual(kept, c.want) || got.NotDueCount != c.notDue || len(got.NotDue) != c.notDue {
			t.Errorf("%s: the causes are %q with %d requirements not yet due; want %q with %d", c.name, kept, got.NotDueCount, c.want, c.notDue)
		}
		total := 0
		for _, k := range got.Kinds {
			total += k.Count
		}
		if total != len(got.Causes) {
			t.Errorf("%s: the kinds count %d causes of %d", c.name, total, len(got.Causes))
		}
	}
	// A selection renumbers what a held task also stands under.
	ben := Blockage(f, Query{Task: "a1c0", Person: "ben@example.org", Level: Detail})
	if got := heldOf(ben.Causes[0].Held); !strings.Contains(got, "c07d@implementation requirement also [2]") {
		t.Errorf("for ben the status cause holds %q", got)
	}

	// Over the corpus: the causes run largest first, and a cause counts the
	// distinct tasks of its tree.
	trees := 0
	for _, e := range expectations(t) {
		f := e.at(t, e.ref)
		for _, q := range queries(f, Detail)["blockage"] {
			trees++
			got := Blockage(f, q)
			last := -1
			for _, c := range got.Causes {
				tasks := map[string]bool{}
				gather(c.Held, tasks)
				if len(tasks) != c.Holds || last >= 0 && c.Holds > last {
					t.Errorf("%s: %+v: %s of %s holds %d, counts %d, after one that holds %d", e.name, q, c.Kind, c.Task.ID, len(tasks), c.Holds, last)
				}
				last = c.Holds
			}
		}
	}
	if trees < 80 {
		t.Errorf("the test reads %d trees", trees)
	}
}
