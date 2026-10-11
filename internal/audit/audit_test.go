package audit

import (
	"errors"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// T4: the date is required. An empty Now and one that is no date each give
// a *DateError and no report, before the run reads the facts: with nil
// facts too.
func TestDateIsRequired(t *testing.T) {
	facts := mem(t, map[string]string{"tasks/e4a1.yaml": "title: Root\nassignee: olive@example.org\n"})
	for _, now := range []string{"", "tomorrow", "2026-9-1", "2026-09-31", "2026-09-01T12:00:00Z", "26-09-01"} {
		for _, f := range []*derive.Facts{facts, nil} {
			rep, err := Run(Input{Facts: f, Now: now})
			var bad *DateError
			if rep != nil || !errors.As(err, &bad) || bad.Now != now || bad.Error() == "" {
				t.Errorf("Run with the date %q gives %v, %v; want a *DateError", now, rep, err)
			}
		}
	}
	if rep, err := Run(Input{Now: "2026-09-01"}); rep != nil || err != nil {
		t.Errorf("Run with a date and nil facts gives %v, %v; want nil, nil", rep, err)
	}
	rep := audited(t, Input{Facts: facts, Now: "2026-09-01"})
	if rep.Now != "2026-09-01" || rep.Stale != DefaultStale || DefaultStale != 7 {
		t.Errorf("the report states the date %q and the age %d", rep.Now, rep.Stale)
	}
	if rep := audited(t, Input{Facts: facts, Now: "2026-09-01", Stale: 3}); rep.Stale != 3 {
		t.Errorf("the report states the age %d, want 3", rep.Stale)
	}
}

// resolved audits one literal diagnostic and returns its finding.
func resolved(t *testing.T, f *derive.Facts, d model.Diagnostic) *Finding {
	t.Helper()
	list := []model.Diagnostic{d}
	rep := audited(t, Input{Facts: f, Diagnostics: list, Now: "2026-09-28"})
	found := of(rep, d.Code)
	if len(found) != 1 {
		t.Fatalf("%s gives %d findings", d.Code, len(found))
	}
	x := found[0]
	if x.Diagnostic == nil || *x.Diagnostic != d || x.Rule != d.Code || x.Message != d.Message || x.Task != d.Task || x.Gate != d.Gate {
		t.Errorf("the finding of %s does not carry its diagnostic unchanged: %+v", d.Code, x)
	}
	return x
}

// T5: each role resolves, and each falls back as the design states, on
// literal diagnostics over weather-station and junction-kinds.
func TestRolesResolve(t *testing.T) {
	station := derived(t, load.Options{}, builtAt(t, "weather-station"), "main", "")
	kinds := derived(t, load.Options{}, builtAt(t, "junction-kinds"), "main", "")
	w12 := station.Authorisation("3c5d").Commit.ID
	in := func(file string) model.Pos { return model.Pos{File: file, Line: 1, Col: 1} }
	cases := []struct {
		name string
		f    *derive.Facts
		d    model.Diagnostic
		act  Act
		who  string
	}{
		// The owner.
		{"the owner", station, model.Diagnostic{Code: "P5", Pos: in("version.yaml")}, Revise, "ada@example.org"},
		{"the owner, for the tree", station, model.Diagnostic{Code: "T8", Pos: in("tasks/9f31.yaml"), Task: "9f31"}, Revise, "ada@example.org"},
		// The keeper, by the file.
		{"a task file: the nearest authority", station, model.Diagnostic{Code: "T2", Pos: in("tasks/9f31.yaml"), Task: "9f31"}, Revise, "ben@example.org"},
		{"the root's task file: the owner", station, model.Diagnostic{Code: "T2", Pos: in("tasks/a1c0.yaml"), Task: "a1c0"}, Revise, "ada@example.org"},
		{"a status file: the recorder", station, model.Diagnostic{Code: "S6", Pos: in("status/3c5d.yaml"), Task: "3c5d"}, Revise, "dan@example.org"},
		{"a status file a reaffirmation dates", station, model.Diagnostic{Code: "S6", Pos: in("status/9f31.yaml"), Task: "9f31"}, Revise, "ada@example.org"},
		{"a parent's status file: the contributor of its next junction", station, model.Diagnostic{Code: "S2", Pos: in("status/4e2b.yaml"), Task: "4e2b"}, Revise, "ben@example.org"},
		{"a status file of no task", station, model.Diagnostic{Code: "S1", Pos: in("status/zzzz.yaml"), Task: "zzzz"}, Revise, "ada@example.org"},
		{"a task file whose name is no id", station, model.Diagnostic{Code: "T1", Pos: in("tasks/leaf-two.yaml"), Task: "leaf-two"}, Revise, "ada@example.org"},
		{"the gates file", station, model.Diagnostic{Code: "G5", Pos: in("gates.yaml"), Gate: "Design"}, Revise, "ada@example.org"},
		{"no file", station, model.Diagnostic{Code: "P1"}, Revise, "ada@example.org"},
		{"a code the table lacks", station, model.Diagnostic{Code: "X1", Pos: in("tasks/c07d.yaml"), Task: "c07d"}, Revise, "ben@example.org"},
		// The assignee.
		{"the assignee", station, model.Diagnostic{Code: "T12", Pos: in("tasks/4e2b.yaml"), Task: "4e2b"}, Revise, "ben@example.org"},
		{"the assignee of an unknown task", station, model.Diagnostic{Code: "T12", Task: "zzzz"}, Revise, "ada@example.org"},
		{"the assignee moves a pin", kinds, model.Diagnostic{Code: "J13", Pos: in("tasks/a400.yaml"), Task: "a400", Gate: "implementation"}, MovePin, "ben@example.org"},
		// The contributor.
		{"the contributor", station, model.Diagnostic{Code: "H4", Task: "c07d", Gate: "unit"}, RecordHandoff, "opus@example.org"},
		{"the contributor by default", station, model.Diagnostic{Code: "H5", Task: "9f31", Gate: "mockup"}, ClearHandoff, "ada@example.org"},
		{"the contributor of a recursive junction", station, model.Diagnostic{Code: "H4", Task: "c07d", Gate: "implementation"}, RecordHandoff, "ben@example.org"},
		{"the contributor at an unknown gate", station, model.Diagnostic{Code: "H4", Task: "9f31", Gate: "nowhere"}, RecordHandoff, "ada@example.org"},
		{"the contributor of an unknown task", station, model.Diagnostic{Code: "H4", Task: "zzzz", Gate: "unit"}, RecordHandoff, "ada@example.org"},
		{"an agent contributes", kinds, model.Diagnostic{Code: "H4", Task: "a110", Gate: "design"}, RecordHandoff, "bot@example.org"},
		// The reviewer.
		{"the reviewer", station, model.Diagnostic{Code: "S11", Pos: in("status/9f31.yaml"), Task: "9f31", Gate: "mockup"}, Review, "ben@example.org"},
		{"no reviewer: the assignee", station, model.Diagnostic{Code: "S11", Pos: in("status/9f31.yaml"), Task: "9f31", Gate: "function"}, Review, "ada@example.org"},
		{"the assignee reviews an agent", kinds, model.Diagnostic{Code: "S11", Task: "a110", Gate: "design"}, Review, "ben@example.org"},
		{"an ancestor states the reviewer", kinds, model.Diagnostic{Code: "S11", Task: "a110", Gate: "unit"}, Review, "olive@example.org"},
		{"a review from the wrong hand, not yet accepted", station, model.Diagnostic{Code: "H2", Task: "9f31", Gate: "validate", Commit: w12}, Review, "ben@example.org"},
		{"a review from the wrong hand, accepted since", station, model.Diagnostic{Code: "H2", Task: "9f31", Gate: "mockup", Commit: w12}, None, "dan@example.org"},
		// The author.
		{"the author", station, model.Diagnostic{Code: "H1", Commit: w12, Trailer: "Reviewed: 9f31 nowhere"}, None, "dan@example.org"},
		{"a commit the pass lacks", station, model.Diagnostic{Code: "H3", Task: "c07d", Gate: "unit", Commit: "0000000000000000000000000000000000000000"}, None, "ada@example.org"},
		{"no commit", station, model.Diagnostic{Code: "H6", Task: "c07d", Gate: "unit"}, None, "ada@example.org"},
		// The origin, without a condition.
		{"no condition: the assignee", station, model.Diagnostic{Code: "R9", Pos: in("tasks/c07d.yaml"), Task: "c07d"}, Await, "ben@example.org"},
		{"no condition and no task", station, model.Diagnostic{Code: "R9", Task: "zzzz"}, Await, "ada@example.org"},
		// A subproject the tool reads resolves by a revision; nobody is named for a clone the host lacks.
		{"a link the files can mend", kinds, model.Diagnostic{Code: "J8", Pos: in("tasks/a400.yaml"), Task: "a400", Gate: "implementation"}, Revise, "olive@example.org"},
		{"no link found", kinds, model.Diagnostic{Code: "J8", Pos: in("tasks/a110.yaml"), Task: "a110", Gate: "design"}, Revise, "ben@example.org"},
	}
	for _, c := range cases {
		x := resolved(t, c.f, c.d)
		if x.Act != c.act || x.Resolver != c.who {
			t.Errorf("%s: %s by %q, want %s by %q", c.name, x.Act, x.Resolver, c.act, c.who)
		}
	}

	// The origin, at the position of the entry. c07d requires 9f31, a leaf
	// whose next junction is plain: its contributor works on it.
	station9 := station.Requires("c07d")[0]
	x := resolved(t, station, model.Diagnostic{Code: "R9", Pos: station9.Entry.Node.Pos, Task: "c07d"})
	if x.Act != Await || x.Resolver != "ada@example.org" {
		t.Errorf("the requirement of c07d on 9f31: %s by %q", x.Act, x.Resolver)
	}
}

// T5: the origin of a requirement on a parent, on a leaf whose next
// junction is recursive and across a link, in projects composed in memory.
func TestOriginResolves(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/e4a1.yaml": "title: Root\nassignee: olive@example.org\n",
		"tasks/a100.yaml": "title: Parent\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 1 }\n",
		"tasks/a110.yaml": "title: Early\nassignee: ben@example.org\nparent: { id: \"a100\", order: 1 }\njunctions:\n  mockup: { contributor: ada@example.org }\n",
		"tasks/a120.yaml": "title: Late\nassignee: ben@example.org\nparent: { id: \"a100\", order: 2 }\n",
		"tasks/a130.yaml": "title: Exempt\nassignee: eve@example.org\nparent: { id: \"a100\", order: 3 }\njunctions:\n  mockup: { applies: false }\n  function: { applies: false }\n  design: { applies: false }\n  release: { applies: false }\n",
		"tasks/b200.yaml": "title: Dependent\nassignee: dan@example.org\nparent: { id: \"e4a1\", order: 2 }\n" +
			"requires:\n  - { id: \"a100\", from: design, to: mockup }\n  - { id: \"a130\", from: defined, to: release }\n  - { id: \"zzzz\" }\n",
		"status/a110.yaml": "gate: defined\nstate: nominal\n",
		"status/a120.yaml": "gate: function\nstate: nominal\n",
		"status/b200.yaml": "gate: defined\nstate: nominal\n",
	})
	requires := f.Requires("b200")
	if len(requires) != 3 {
		t.Fatalf("b200 has %d conditions", len(requires))
	}
	// The parent rolls up from a110, the child at the earliest gate, whose
	// next junction, mockup, states its contributor.
	x := resolved(t, f, model.Diagnostic{Code: "R9", Pos: requires[0].Entry.Node.Pos, Task: "b200"})
	if x.Act != Await || x.Resolver != "ada@example.org" {
		t.Errorf("a requirement on a parent: %s by %q, want await by ada@example.org", x.Act, x.Resolver)
	}
	// a130 has no file and stands at undefined; its next junction, defined, is
	// plain and its assignee contributes.
	x = resolved(t, f, model.Diagnostic{Code: "R9", Pos: requires[1].Entry.Node.Pos, Task: "b200"})
	if x.Resolver != "eve@example.org" {
		t.Errorf("a requirement on a leaf with no status: %q, want eve@example.org", x.Resolver)
	}
	// The entry names no task: the chain is empty, and the assignee waits.
	x = resolved(t, f, model.Diagnostic{Code: "R9", Pos: requires[2].Entry.Node.Pos, Task: "b200"})
	if x.Resolver != "dan@example.org" {
		t.Errorf("a requirement on no task: %q, want the assignee, dan@example.org", x.Resolver)
	}
	// With no history every rule finding stands, and the commit's author is the owner.
	x = resolved(t, f, model.Diagnostic{Code: "H1", Commit: "0000000000000000000000000000000000000000"})
	if x.Act != None || x.Resolver != "olive@example.org" {
		t.Errorf("a commit with no pass: %s by %q", x.Act, x.Resolver)
	}
}
