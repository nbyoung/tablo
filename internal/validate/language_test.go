package validate

import (
	"strings"
	"testing"
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
