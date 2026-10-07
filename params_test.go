package tablo

import (
	"reflect"
	"strings"
	"testing"
)

// taken is the command table of design 4ed9, written out again: the options
// each view takes besides --viewer, --role and --level.
var taken = map[View][]string{
	Gates:      {"--task"},
	Task:       {"--task", "--person"},
	Authority:  {"--task", "--person", "--proposed"},
	Assignment: {"--task", "--person"},
	Queue:      {"--task", "--person", "--brief"},
	Blockage:   {"--task", "--person"},
	Tableau:    {"--person", "--window", "--columns", "--historical"},
	Context:    {"--task", "--person", "--window", "--columns", "--historical"},
	History:    {"--task", "--person", "a range in --ref"},
	Audit:      {"--task", "--person", "--stale", "--now"},
}

// option is one parameter the tests set, with a valid value for it.
type option struct {
	name  string
	apply func(*Params)
}

var options = []option{
	{"--task", func(p *Params) { p.Task = "e9c6" }},
	{"--person", func(p *Params) { p.Person = "ada@example.org" }},
	{"--window", func(p *Params) { w := 2; p.Window = &w }},
	{"--columns", func(p *Params) { p.Columns = []string{"mockup", "design"} }},
	{"--historical", func(p *Params) { p.Historical = true }},
	{"--proposed", func(p *Params) { p.Proposed = true }},
	{"--brief", func(p *Params) { p.Brief = &Brief{Task: "e9c6", Gate: "mockup"} }},
	{"--stale", func(p *Params) { p.Stale = 3 }},
	{"--now", func(p *Params) { p.Now = "2026-10-06" }},
	{"a range in --ref", func(p *Params) { p.From = "0704a09" }},
	{"--level", func(p *Params) { p.Level = LevelDetail }},
	{"--role", func(p *Params) { p.Role = RoleOwner }},
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// subsets calls f with every subset of the options, as a set of names and the
// Params those options make.
func subsets(f func(names map[string]bool, p Params)) {
	for mask := 0; mask < 1<<len(options); mask++ {
		names := map[string]bool{}
		var p Params
		for i, o := range options {
			if mask&(1<<i) != 0 {
				names[o.name] = true
				o.apply(&p)
			}
		}
		f(names, p)
	}
}

// conflict names the rule that two options the view takes break, if any.
func conflict(v View, names map[string]bool) string {
	switch {
	case names["--window"] && names["--columns"]:
		return "--window and --columns"
	case v == Context && names["--task"] && names["--person"]:
		return "--task or --person"
	case names["--brief"] && names["--level"]:
		return "--brief and --level"
	case names["--brief"] && names["--role"]:
		return "--brief and --role"
	}
	return ""
}

// TestCheckAgreesWithTheTable is T3: for each view and each combination of
// options, Check agrees with the command table, and Narrow then Check passes
// unless the view takes a conflicting pair.
func TestCheckAgreesWithTheTable(t *testing.T) {
	for v, takes := range taken {
		if _, ok := specOf(v); !ok {
			t.Fatalf("%s is missing from the command table", v)
		}
		subsets(func(names map[string]bool, p Params) {
			var notTaken string
			for _, o := range options {
				if names[o.name] && o.name != "--level" && o.name != "--role" && !contains(takes, o.name) {
					notTaken = o.name
					break
				}
			}
			needTask := v == Task && !names["--task"]
			err := p.Check(v)
			switch {
			case notTaken != "":
				var u *UsageError
				if !asUsage(err, &u) || !strings.Contains(u.Msg, notTaken) {
					t.Fatalf("%s with %v: Check = %v, want a usage error naming %s", v, names, err, notTaken)
				}
			case needTask:
				if err == nil {
					t.Fatalf("task without an id passed Check")
				}
			default:
				if c := conflict(v, names); c != "" {
					var u *UsageError
					if !asUsage(err, &u) {
						t.Fatalf("%s with %v: Check = %v, want a usage error for %s", v, names, err, c)
					}
				} else if err != nil {
					t.Fatalf("%s with %v: Check = %v, want none", v, names, err)
				}
			}
			q := p.Narrow(v)
			for _, o := range options {
				var probe Params
				o.apply(&probe)
				got := !reflect.DeepEqual(probe.Narrow(v), Params{})
				want := o.name == "--level" || o.name == "--role" || contains(takes, o.name)
				if got != want {
					t.Fatalf("Narrow(%s) keeps %s = %v, want %v", v, o.name, got, want)
				}
			}
			kept := map[string]bool{}
			for n := range names {
				if n == "--level" || n == "--role" || contains(takes, n) {
					kept[n] = true
				}
			}
			err = q.Check(v)
			if needTask || conflict(v, kept) != "" {
				if err == nil {
					t.Fatalf("%s with %v: Narrow then Check passed despite a conflict", v, names)
				}
			} else if err != nil {
				t.Fatalf("%s with %v: Narrow then Check = %v", v, names, err)
			}
		})
	}
}

func asUsage(err error, target **UsageError) bool {
	u, ok := err.(*UsageError)
	*target = u
	return ok
}

// TestCheckMalformedValues covers every malformed value the table lists.
func TestCheckMalformedValues(t *testing.T) {
	neg, zero := -1, 0
	tests := []struct {
		name string
		view View
		p    Params
		want string // the option or value the message names
	}{
		{"task id too short", Gates, Params{Task: "e9c"}, "e9c"},
		{"task id upper case", Gates, Params{Task: "E9C6"}, "E9C6"},
		{"task id not hex", Gates, Params{Task: "e9cz"}, "e9cz"},
		{"task view without id", Task, Params{}, "task id"},
		{"email without at", Queue, Params{Person: "ada"}, "ada"},
		{"email with two at", Queue, Params{Person: "a@b@c"}, "a@b@c"},
		{"email with a space", Queue, Params{Person: "a b@c"}, "a b@c"},
		{"viewer not an email", Gates, Params{Viewer: "nobody"}, "--viewer"},
		{"negative window", Tableau, Params{Window: &neg}, "--window"},
		{"gate key with capital", Tableau, Params{Columns: []string{"Mockup"}}, "Mockup"},
		{"gate key with digit first", Tableau, Params{Columns: []string{"1st"}}, "1st"},
		{"column twice", Tableau, Params{Columns: []string{"mockup", "mockup"}}, "twice"},
		{"brief task", Queue, Params{Brief: &Brief{Task: "zzzz", Gate: "mockup"}}, "zzzz"},
		{"brief gate", Queue, Params{Brief: &Brief{Task: "e9c6", Gate: "Mock up"}}, "Mock up"},
		{"negative stale", Audit, Params{Stale: -1}, "--stale"},
		{"date with a slash", Audit, Params{Now: "2026/10/06"}, "2026/10/06"},
		{"date out of range", Audit, Params{Now: "2026-02-30"}, "2026-02-30"},
		{"date with one digit", Audit, Params{Now: "2026-1-5"}, "2026-1-5"},
		{"range start with hyphen", History, Params{From: "-x"}, "-x"},
		{"unknown role", Gates, Params{Role: "boss"}, "boss"},
		{"unknown level", Gates, Params{Level: "deep"}, "deep"},
		{"window zero is fine but not with columns", Tableau, Params{Window: &zero, Columns: []string{"a"}}, "--columns"},
		{"unknown view", View("status"), Params{}, "status"},
	}
	for _, tc := range tests {
		err := tc.p.Check(tc.view)
		u, ok := err.(*UsageError)
		if !ok {
			t.Errorf("%s: Check = %v, want a *UsageError", tc.name, err)
			continue
		}
		if !strings.Contains(u.Msg, tc.want) {
			t.Errorf("%s: message %q does not name %q", tc.name, u.Msg, tc.want)
		}
	}
	zeroOK := Params{Window: &zero}
	if err := zeroOK.Check(Tableau); err != nil {
		t.Errorf("window 0: %v", err)
	}
	if err := (Params{}).Check(Gates); err != nil {
		t.Errorf("zero Params on gates: %v", err)
	}
}

// TestNarrowKeepsWhatEveryViewTakes checks that Viewer, Role and Level survive.
func TestNarrowKeepsWhatEveryViewTakes(t *testing.T) {
	p := Params{Viewer: "v@example.org", Role: RoleAgent, Level: LevelProvenance, Task: "e9c6", Stale: 9}
	q := p.Narrow(Gates)
	want := Params{Viewer: "v@example.org", Role: RoleAgent, Level: LevelProvenance, Task: "e9c6"}
	if !reflect.DeepEqual(q, want) {
		t.Errorf("Narrow(Gates) = %+v, want %+v", q, want)
	}
	if got := p.Narrow(View("nope")); !reflect.DeepEqual(got, Params{}) {
		t.Errorf("Narrow of an unknown view = %+v, want the zero value", got)
	}
}
