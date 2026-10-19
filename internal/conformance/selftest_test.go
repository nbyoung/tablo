package conformance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/nbyoung/tablo/internal/corpustest"
	"gopkg.in/yaml.v3"
)

// The self-tests prove the harness on values in the test's own source and on
// recorded envelopes. They call the command only through the stub.

// parse reads a YAML snippet as the mapping at the root of an expected.yaml.
func parse(t *testing.T, snippet string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(snippet), &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Content[0]
}

// weather returns the entry whose labels the self-tests resolve.
func weather(t *testing.T) *corpustest.Entry {
	t.Helper()
	return corpustest.Open(t).Entry(t, "weather-station")
}

// differing returns the paths of the facts that differ, and the number of facts.
func differing(o *outcome) []string {
	var paths []string
	for _, f := range o.facts {
		if f.result == differed {
			paths = append(paths, f.path)
		}
	}
	return paths
}

// run compares a snippet with an observed document.
func run(t *testing.T, e *corpustest.Entry, snippet string, observed map[string]any) (*outcome, error) {
	t.Helper()
	return compare(e, parse(t, snippet), observed, nil)
}

type row struct {
	name     string
	snippet  string
	observed map[string]any
	differ   []string // the paths that must differ
	facts    int      // the facts it counts
}

func checkRows(t *testing.T, e *corpustest.Entry, rows []row) {
	t.Helper()
	for _, r := range rows {
		t.Run(r.name, func(t *testing.T) {
			o, err := run(t, e, r.snippet, r.observed)
			if err != nil {
				t.Fatal(err)
			}
			if got := differing(o); !reflect.DeepEqual(got, r.differ) && (len(got) != 0 || len(r.differ) != 0) {
				t.Errorf("differ at %v, want %v", got, r.differ)
			}
			if r.facts != 0 && len(o.facts) != r.facts {
				t.Errorf("%d facts, want %d", len(o.facts), r.facts)
			}
			if len(o.unexpected) != 0 {
				t.Errorf("unexpected findings %v", o.unexpected)
			}
		})
	}
}

func task(m map[string]any) map[string]any { return map[string]any{"tasks": map[string]any{"a1c0": m}} }

// TestScalarsLabelsAbsence is T9: rules 1 to 4.
func TestScalarsLabelsAbsence(t *testing.T) {
	e := weather(t)
	w1, _ := e.Hash("W1")
	checkRows(t, e, []row{
		{name: "string agrees", snippet: "owner: ada@example.org", observed: map[string]any{"owner": "ada@example.org"}, facts: 1},
		{name: "string differs", snippet: "owner: ada@example.org", observed: map[string]any{"owner": "ben@example.org"}, differ: []string{"owner"}},
		{name: "boolean agrees", snippet: "valid: true", observed: map[string]any{"valid": true}},
		{name: "false is not absent", snippet: "valid: false", observed: map[string]any{}, differ: []string{"valid"}},
		{name: "false agrees", snippet: "valid: false", observed: map[string]any{"valid": false}},
		{name: "integer", snippet: "count: { tasks: 6, leaves: 4 }", observed: map[string]any{"count": map[string]any{"tasks": 6, "leaves": 5}},
			differ: []string{"count.leaves"}, facts: 2},
		{name: "date takes ten characters", snippet: "tasks:\n  a1c0:\n    status: { date: 2026-09-17 }",
			observed: task(map[string]any{"status": map[string]any{"date": "2026-09-17T12:00:00Z"}})},
		{name: "date differs", snippet: "tasks:\n  a1c0:\n    status: { date: 2026-09-17 }",
			observed: task(map[string]any{"status": map[string]any{"date": "2026-09-18"}}), differ: []string{"tasks.a1c0.status.date"}},
		{name: "quoted and unquoted ids agree", snippet: "tasks:\n  a1c0:\n    parent: \"4e2b\"", observed: task(map[string]any{"parent": "4e2b"})},
		{name: "null equals absent", snippet: "tasks:\n  a1c0:\n    parent: null", observed: task(map[string]any{"parent": nil})},
		{name: "null equals the empty string", snippet: "tasks:\n  a1c0:\n    parent: null", observed: task(map[string]any{"parent": ""})},
		{name: "null differs from a value", snippet: "tasks:\n  a1c0:\n    parent: null", observed: task(map[string]any{"parent": "4e2b"}),
			differ: []string{"tasks.a1c0.parent"}},
		{name: "an empty list equals absent", snippet: "tasks:\n  a1c0:\n    authorities: []", observed: task(map[string]any{})},
		{name: "a label resolves", snippet: "tasks:\n  a1c0:\n    authorisation: { commit: W1 }",
			observed: task(map[string]any{"authorisation": map[string]any{"commit": w1}})},
		{name: "a label resolves to another commit", snippet: "tasks:\n  a1c0:\n    authorisation: { commit: W1 }",
			observed: task(map[string]any{"authorisation": map[string]any{"commit": strings.Repeat("a", 40)}}),
			differ:   []string{"tasks.a1c0.authorisation.commit"}},
		{name: "a hash passes through", snippet: "tasks:\n  a1c0:\n    authorisation: { commit: " + w1 + " }",
			observed: task(map[string]any{"authorisation": map[string]any{"commit": w1}})},
		{name: "a partial map passes more keys", snippet: "tasks:\n  a1c0:\n    authorisation: { state: authorised }",
			observed: task(map[string]any{"authorisation": map[string]any{"state": "authorised", "by": "ada@example.org", "way": "merge"}}), facts: 1},
	})

	// A label no file holds is the corpus's fault, not the command's.
	_, err := run(t, e, "tasks:\n  a1c0:\n    authorisation: { commit: Z9 }", task(map[string]any{}))
	if err == nil || !strings.HasPrefix(err.Error(), "corpus:") {
		t.Errorf("an unknown label gives %v", err)
	}
}

// TestLists is T10: rules 5 to 7, the three kinds of list.
func TestLists(t *testing.T) {
	e := weather(t)
	w1, _ := e.Hash("W1")
	checkRows(t, e, []row{
		{name: "a list compares whole", snippet: "order: [a1c0, 4e2b]", observed: map[string]any{"order": []any{"a1c0", "4e2b"}}, facts: 1},
		{name: "an order swapped", snippet: "order: [a1c0, 4e2b]", observed: map[string]any{"order": []any{"4e2b", "a1c0"}}, differ: []string{"order"}},
		{name: "a list is longer", snippet: "order: [a1c0]", observed: map[string]any{"order": []any{"a1c0", "4e2b"}}, differ: []string{"order"}},
		{name: "authorities in order", snippet: "tasks:\n  a1c0:\n    authorities: [ben@example.org, ada@example.org]",
			observed: task(map[string]any{"authorities": []any{"ada@example.org", "ben@example.org"}}), differ: []string{"tasks.a1c0.authorities"}},
		{name: "a source as a string", snippet: "tasks:\n  a1c0:\n    junctions:\n      mockup: { source: \"4e2b\" }",
			observed: task(map[string]any{"junctions": map[string]any{"mockup": map[string]any{"source": []any{"4e2b"}}}})},
		{name: "a source as a string differs", snippet: "tasks:\n  a1c0:\n    junctions:\n      mockup: { source: \"4e2b\" }",
			observed: task(map[string]any{"junctions": map[string]any{"mockup": map[string]any{"source": []any{"a1c0"}}}}),
			differ:   []string{"tasks.a1c0.junctions.mockup.source"}},
		{name: "a source as a set", snippet: "tasks:\n  a1c0:\n    junctions:\n      unit: { source: [\"a110\", \"a100\"] }",
			observed: task(map[string]any{"junctions": map[string]any{"unit": map[string]any{"source": []any{"a100", "a110"}}}})},
		{name: "requires as a set", snippet: "tasks:\n  a1c0:\n    requires:\n      - { id: \"9f31\", met: false }\n      - { id: \"7b2e\", met: true }",
			observed: task(map[string]any{"requires": []any{map[string]any{"id": "7b2e", "met": true}, map[string]any{"id": "9f31", "met": false}}}), facts: 3},
		{name: "a requirement is missing", snippet: "tasks:\n  a1c0:\n    requires:\n      - { id: \"9f31\", met: false }\n      - { id: \"7b2e\", met: true }",
			observed: task(map[string]any{"requires": []any{map[string]any{"id": "9f31", "met": false}}}),
			differ:   []string{"tasks.a1c0.requires.length", "tasks.a1c0.requires[7b2e]"}},
		{name: "a cross-project requirement", snippet: "tasks:\n  a1c0:\n    requires:\n      - { subproject: { url: sub, id: \"5a00\", pin: W1 }, met: false }",
			observed: task(map[string]any{"requires": []any{map[string]any{"subproject": map[string]any{"url": "sub", "id": "5a00", "pin": w1}, "met": false}}}), facts: 3},
		{name: "a review is missing", snippet: "tasks:\n  a1c0:\n    reviews:\n      - { gate: mockup, reviewer: ben@example.org, commit: W1 }",
			observed: task(map[string]any{}), differ: []string{"tasks.a1c0.reviews.length", "tasks.a1c0.reviews[mockup]"}},
		{name: "a review agrees", snippet: "tasks:\n  a1c0:\n    reviews:\n      - { gate: mockup, reviewer: ben@example.org, commit: W1 }",
			observed: task(map[string]any{"reviews": []any{map[string]any{"gate": "mockup", "reviewer": "ben@example.org", "commit": w1}}}), facts: 3},
		{name: "events compare in order", snippet: "tasks:\n  a1c0:\n    events:\n      - { date: 2026-09-01, task: a1c0, event: task }\n      - { date: 2026-09-02, task: a1c0, event: authorised }",
			observed: task(map[string]any{"events": []any{
				map[string]any{"date": "2026-09-02", "task": "a1c0", "event": "authorised"},
				map[string]any{"date": "2026-09-01", "task": "a1c0", "event": "task"}}}),
			differ: []string{"tasks.a1c0.events[0].date", "tasks.a1c0.events[0].event", "tasks.a1c0.events[1].date", "tasks.a1c0.events[1].event"}},
		{name: "an event states a reason the file leaves out", snippet: "tasks:\n  a1c0:\n    events:\n      - { date: 2026-09-01, task: a1c0, event: status, gate: design }",
			observed: task(map[string]any{"events": []any{map[string]any{"date": "2026-09-01", "task": "a1c0", "event": "status", "gate": "design", "reason": "blocked"}}}),
			differ:   []string{"tasks.a1c0.events[0].reason"}},
		{name: "an event key outside the list passes", snippet: "tasks:\n  a1c0:\n    events:\n      - { date: 2026-09-01, task: a1c0, event: task }",
			observed: task(map[string]any{"events": []any{map[string]any{"date": "2026-09-01", "task": "a1c0", "event": "task", "title": "x"}}})},
		{name: "events differ in length", snippet: "tasks:\n  a1c0:\n    events:\n      - { date: 2026-09-01, task: a1c0, event: task }",
			observed: task(map[string]any{"events": []any{}}), differ: []string{"tasks.a1c0.events.length"}},
		{name: "references compare in order", snippet: "tasks:\n  a1c0:\n    junctions:\n      mockup: { references: [{ url: a.md, text: A }] }",
			observed: task(map[string]any{"junctions": map[string]any{"mockup": map[string]any{"references": []any{map[string]any{"url": "b.md", "text": "A"}}}}}),
			differ:   []string{"tasks.a1c0.junctions.mockup.references[0].url"}},
	})
}

// observedFinding builds an observed finding.
func observedFinding(rule, severity, taskID, file, message string) any {
	return map[string]any{"rule": rule, "severity": severity, "task": taskID, "file": file, "message": message}
}

// TestFindings is T11: rule 8, the multiset of findings, on tree-no-root.
func TestFindings(t *testing.T) {
	c := corpustest.Open(t)
	e := c.Entry(t, "tree-no-root")
	t8 := observedFinding("T8", "error", "", "", "no root")
	e4 := observedFinding("T10", "error", "e4a1", "tasks/e4a1.yaml", "chain does not end at the root")
	b2 := observedFinding("T10", "error", "b2c9", "tasks/b2c9.yaml", "chain does not end at the root")
	cases := []struct {
		name       string
		observed   []any
		differ     int
		unexpected int
	}{
		{"three against three", []any{t8, e4, b2}, 0, 0},
		{"in another order", []any{b2, t8, e4}, 0, 0},
		{"two against three", []any{t8, e4}, 1, 0},
		{"four against three", []any{t8, e4, b2, observedFinding("S1", "error", "", "status/zzzz.yaml", "x")}, 0, 1},
		{"a message that differs", []any{observedFinding("T8", "error", "", "", "other words"), e4, b2}, 0, 0},
		{"a duplicate takes no second slot", []any{t8, e4, e4}, 1, 1},
		{"a severity that differs", []any{observedFinding("T8", "warning", "", "", ""), e4, b2}, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := compare(e, e.Expected, map[string]any{"valid": false, "findings": tc.observed}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(differing(o)); got != tc.differ {
				t.Errorf("%d findings missing, want %d", got, tc.differ)
			}
			if got := len(o.unexpected); got != tc.unexpected {
				t.Errorf("%d unexpected, want %d", got, tc.unexpected)
			}
		})
	}
	// A valid entry with findings: [] fails on any diagnostic at all.
	o, err := run(t, e, "valid: true\nfindings: []", map[string]any{"valid": true, "findings": []any{observedFinding("R9", "warning", "", "", "")}})
	if err != nil || len(o.unexpected) != 1 {
		t.Errorf("findings: [] with a diagnostic: %v, %d unexpected", err, len(o.unexpected))
	}
	// A finding about a commit compares the label's hash.
	w1, _ := weather(t).Hash("W1")
	we := weather(t)
	o, err = run(t, we, "findings:\n  - { rule: H1, severity: warning, commit: W1, trailer: \"Authorised: zzzz\" }",
		map[string]any{"findings": []any{map[string]any{"rule": "H1", "severity": "warning", "commit": w1, "trailer": "Authorised: zzzz"}}})
	if err != nil || len(differing(o)) != 0 || len(o.unexpected) != 0 {
		t.Errorf("a finding with a commit: %v %v", err, differing(o))
	}
}

// TestUndecidedAndInputs is T12: rules 9 and 10.
func TestUndecidedAndInputs(t *testing.T) {
	e := weather(t)
	// The corpus marks no fact undecided now (8118 decision 11); the mechanism
	// stays, and a fixture of this test's own proves it.
	o, err := run(t, e, "tasks:\n  a210:\n    junctions:\n      release: { undecided: F7 }", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if len(o.facts) != 1 || o.facts[0].result != undecided || o.mismatches() != 0 {
		t.Errorf("an undecided map gives %+v", o.facts)
	}
	if o.facts[0].want != "undecided F7" {
		t.Errorf("the undecided fact reads %q", o.facts[0].want)
	}
	// Inputs compare nothing.
	o, err = run(t, e, "entry: weather-station\nlanguage: 0.3.1\ntrunk: { stated: main }\nref: main\nsource: { repository: x, ref: y }\nreplace: { u: v }", map[string]any{})
	if err != nil || len(o.facts) != 0 {
		t.Errorf("the inputs give %d facts, %v", len(o.facts), err)
	}
}

// failureFacts builds the 131 facts of the draft's weather-station: three differ.
func failureFacts() []fact {
	var facts []fact
	for i := 0; i < 128; i++ {
		facts = append(facts, fact{path: "filler", kind: "status", result: agreed})
	}
	facts = append(facts,
		fact{path: "tasks.4e2b.status.gate", kind: "status", line: 44, want: "function", got: "design", result: differed, call: "task 4e2b"},
		fact{path: "tasks.c07d.events[6].new", kind: "events", line: 114, want: "cf5b16964dba7f8f1c8842a8202878fd572ad835 (F2)", got: "(absent)", result: differed, call: "history --task c07d"},
		fact{path: "findings", kind: "findings", line: 11, rule: "R9", want: "R9 warning task=c07d file=tasks/c07d.yaml", got: "(no such finding)", result: differed, call: "validate"},
	)
	return facts
}

// golden reads a golden file of testdata.
func golden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestFailureText is T13: the four mismatches of the draft, formatted.
func TestFailureText(t *testing.T) {
	o := &outcome{facts: failureFacts(), unexpected: []finding{{rule: "S11", severity: "error", task: "c07d", file: "status/c07d.yaml",
		text: "S11 error task=c07d file=status/c07d.yaml: c07d passes mockup with no Reviewed commit"}}}
	if got, want := failureText("weather-station", "main", o, false, nil), golden(t, "failure.golden"); got != want {
		t.Errorf("failure text:\n%s\nwant:\n%s", got, want)
	}
	keep := failureText("weather-station", "main", o, true, map[string]string{"validate": "tablo -C repo --ref main --json validate"})
	if !strings.Contains(keep, "got   (no such finding)  [tablo -C repo --ref main --json validate]") {
		t.Errorf("with TABLO_KEEP the got line shows the command line:\n%s", keep)
	}
	if strings.Contains(failureText("x", "main", &outcome{facts: failureFacts()[:129]}, false, nil), "mismatches in") {
		t.Error("one mismatch reads as many")
	}
}

// many returns n facts of one kind.
func many(kind string, n int, r result, rule string) []fact {
	facts := make([]fact, n)
	for i := range facts {
		facts[i] = fact{path: kind, kind: kind, result: r, rule: rule}
	}
	return facts
}

// TestReport is T14: three results and four rules give the report.
func TestReport(t *testing.T) {
	rules := []corpustest.Rule{
		{ID: "T8", Severity: "error", Entries: []string{"tree-two-roots"}},
		{ID: "R9", Severity: "warning", Entries: []string{"unmet-requirement", "weather-station"}},
		{ID: "S11", Severity: "error", Entries: []string{"status-unreviewed-gate"}},
		{ID: "H1", Severity: "warning", Entries: []string{"unknown-trailer"}},
	}
	var weatherFacts []fact
	weatherFacts = append(weatherFacts, many("valid", 1, agreed, "")...)
	weatherFacts = append(weatherFacts, many("findings", 1, differed, "R9")...)
	weatherFacts = append(weatherFacts, many("project", 9, agreed, "")...)
	weatherFacts = append(weatherFacts, many("parent", 6, agreed, "")...)
	weatherFacts = append(weatherFacts, many("authorities", 6, agreed, "")...)
	weatherFacts = append(weatherFacts, many("authorisation", 18, agreed, "")...)
	weatherFacts = append(weatherFacts, many("junctions", 30, agreed, "")...)
	weatherFacts = append(weatherFacts, many("junctions", 1, undecided, "")...)
	weatherFacts = append(weatherFacts, many("applicable", 2, agreed, "")...)
	weatherFacts = append(weatherFacts, many("requires", 3, agreed, "")...)
	weatherFacts = append(weatherFacts, many("reviews", 2, agreed, "")...)
	weatherFacts = append(weatherFacts, many("subproject", 3, agreed, "")...)
	weatherFacts = append(weatherFacts, many("status", 32, agreed, "")...)
	weatherFacts = append(weatherFacts, many("status", 1, differed, "")...)
	weatherFacts = append(weatherFacts, many("events", 9, agreed, "")...)
	weatherFacts = append(weatherFacts, many("events", 1, differed, "")...)
	weatherFacts = append(weatherFacts, many("refs", 6, agreed, "")...)
	results := []entryResult{
		{name: "weather-station", status: failed, reason: "tasks.4e2b.status.gate: want function, got design; and 3 more",
			facts: weatherFacts, unexpected: []finding{{rule: "S11"}}, stated: []string{"R9"}},
		{name: "unknown-trailer", status: skipped, reason: "language 1.0.0: tablo reads major 0", stated: []string{"H1"}},
		{name: "tree-two-roots", status: passed, stated: []string{"T8"},
			facts: append(many("valid", 1, agreed, ""), many("findings", 1, agreed, "T8")...)},
	}
	got := report(results, rules, "0.1.0", "0.3", "testdata/corpus at tableaux ccb3947")
	if want := golden(t, "report.golden"); got != want {
		t.Errorf("report:\n%s\nwant:\n%s", got, want)
	}

	// TABLO_REPORT writes the same bytes, through the call TestCorpus makes;
	// without the variable nothing is written.
	path := filepath.Join(t.TempDir(), "report.md")
	t.Setenv("TABLO_REPORT", "")
	if err := writeReport(got); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("without TABLO_REPORT the report reaches a file")
	}
	t.Setenv("TABLO_REPORT", path)
	if err := writeReport(got); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != golden(t, "report.golden") {
		t.Errorf("the file TABLO_REPORT names differs from report.golden: %v", err)
	}
	if s := summary(got); !strings.Contains(s, "3 entries: 1 pass, 1 fail, 0 known, 1 skipped.") || !strings.Contains(s, "| total ") {
		t.Errorf("summary:\n%s", s)
	}

	// An observed code RULES.md lacks counts outside the rules.
	out := report([]entryResult{{name: "x", status: failed, outside: 2}}, rules, "0.1.0", "0.3", "c")
	if !strings.Contains(out, "Findings outside the rules: 2.") {
		t.Errorf("outside the rules:\n%s", out)
	}
	// A rule no entry states reads no entry.
	out = report(nil, []corpustest.Rule{{ID: "Z1", Severity: "error"}}, "0.1.0", "0.3", "c")
	if !strings.Contains(out, "| Z1   | error    | 0      | 0       | 0       | 0          | no entry |") {
		t.Errorf("no entry:\n%s", out)
	}
}

// TestKnown is T15: the known list.
func TestKnown(t *testing.T) {
	cases := []struct {
		listed, mismatched bool
		want               ruling
	}{
		{false, false, vPass}, {false, true, vFail}, {true, true, vKnown}, {true, false, vStale},
	}
	for _, tc := range cases {
		if got := judge(tc.listed, tc.mismatched); got != tc.want {
			t.Errorf("judge(listed %v, mismatched %v) = %d, want %d", tc.listed, tc.mismatched, got, tc.want)
		}
	}
	items, err := loadKnown([]byte("known:\n  - { entry: a, reason: r, owes: 8118 }\n  - { entry: gone, reason: r, owes: fcec, finding: C1 }\n"))
	if err != nil || len(items) != 2 || items[1].Finding != "C1" {
		t.Fatalf("loadKnown: %v %v", items, err)
	}
	if got := absentFromCorpus(items, []string{"a", "b"}); !reflect.DeepEqual(got, []string{"gone"}) {
		t.Errorf("absent from the corpus: %v", got)
	}
	if _, err := loadKnown([]byte("known:\n  - { entry: a }\n")); err == nil {
		t.Error("an item without reason and owes loads")
	}
	data, err := os.ReadFile("known.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadKnown(data); err != nil {
		t.Errorf("known.yaml: %v", err)
	}
}

// prune drops the absent values of a decoded document, so a test states only
// what the document holds.
func prune(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range x {
			if p := prune(val); !absent(p) {
				out[k] = p
			}
		}
		return out
	case []any:
		var out []any
		for _, val := range x {
			out = append(out, prune(val))
		}
		return out
	}
	return v
}

// envelope reads a recorded envelope.
func envelope(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "envelopes", name))
	if err != nil {
		t.Fatal(err)
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatal(err)
	}
	return env
}

// TestObserve is T16: observe reads the command's data. The three envelopes
// are recorded from the implemented command on weather-station, as
//
//	tablo -C weather-station --ref main --json validate
//	tablo -C weather-station --ref main --json task c07d --level provenance
//	tablo -C weather-station --ref main --json history --task c07d --level provenance
//
// with GIT_AUTHOR_EMAIL set to observer@example.org.
func TestObserve(t *testing.T) {
	findings := prune(observeFindings(envelope(t, "validate.json")))
	wantFindings := []any{map[string]any{
		"rule": "R9", "severity": "warning", "file": "tasks/c07d.yaml", "task": "c07d",
		"message": "the requirement on 9f31 from design is due at implementation and unmet: 9f31 stands at function",
	}}
	if !reflect.DeepEqual(findings, wantFindings) {
		t.Errorf("findings:\n%v\nwant\n%v", findings, wantFindings)
	}

	const (
		w2  = "22a758bec2a079fd908bba970db5eb51ecccd0c6"
		w11 = "97e423122790a250e4e067d7d33bbf88622b00d5"
		f1  = "79e0237eea866c2951968e2e401552dde5b6f9b8"
		f2  = "cf5b16964dba7f8f1c8842a8202878fd572ad835"
	)
	got := prune(observeTask(envelope(t, "task.json")))
	want := map[string]any{
		"parent":        "4e2b",
		"authorities":   []any{"ben@example.org", "ada@example.org"},
		"authorisation": map[string]any{"state": "authorised", "commit": w2, "by": "ben@example.org"},
		"junctions": map[string]any{
			"default":        map[string]any{"kind": "plain", "contributor": "ben@example.org"},
			"defined":        map[string]any{"kind": "plain", "contributor": "ben@example.org"},
			"mockup":         map[string]any{"kind": "plain", "contributor": "ben@example.org", "reviewer": "ben@example.org", "source": []any{"4e2b"}},
			"function":       map[string]any{"kind": "plain", "contributor": "ben@example.org"},
			"performance":    map[string]any{"kind": "plain", "contributor": "ben@example.org"},
			"reliability":    map[string]any{"kind": "not_applicable", "source": []any{"c07d"}},
			"design":         map[string]any{"kind": "plain", "contributor": "ben@example.org"},
			"implementation": map[string]any{"kind": "recursive", "source": []any{"c07d"}, "subproject": map[string]any{"url": "firmware", "id": "f1a0", "commit": f2}},
			"unit": map[string]any{"kind": "plain", "contributor": "opus@example.org", "model": "claude-opus-5-5",
				"reviewer": "ben@example.org", "source": []any{"c07d"}},
			"integrate": map[string]any{"kind": "plain", "contributor": "ben@example.org"},
			"validate":  map[string]any{"kind": "plain", "contributor": "ben@example.org"},
			"release":   map[string]any{"kind": "plain", "contributor": "ben@example.org"},
		},
		"applicable": []any{"undefined", "defined", "mockup", "function", "performance", "design", "implementation", "unit", "integrate", "validate", "release"},
		"requires":   []any{map[string]any{"id": "9f31", "from": "design", "to": "implementation", "met": false, "due": true, "condition": "unmet"}},
		"reviews":    []any{map[string]any{"gate": "mockup", "reviewer": "ben@example.org", "commit": w11}},
		"subproject": map[string]any{"url": "firmware", "pin": f2, "task": "f1a0"},
		"status": map[string]any{"gate": "design", "state": "nominal", "note": "Sleep scheduler in progress", "date": "2026-09-17",
			"recorder": "ben@example.org", "derived": false, "commit": f2},
	}
	// prune keeps false, which is a value; json decoding makes the booleans.
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.MarshalIndent(got, "", " ")
		wj, _ := json.MarshalIndent(want, "", " ")
		t.Errorf("task:\n%s\nwant\n%s", gj, wj)
	}

	events := prune(observeEvents(envelope(t, "history.json"), "c07d"))
	wantEvents := []any{
		map[string]any{"date": "2026-09-10", "commit": f1, "by": "ben@example.org", "task": "c07d", "event": "task", "subproject": "f1a0"},
		map[string]any{"date": "2026-09-10", "commit": f1, "by": "ben@example.org", "task": "c07d", "event": "status", "gate": "defined", "state": "nominal", "subproject": "f1a0"},
		map[string]any{"date": "2026-09-16", "commit": w2, "by": "ben@example.org", "task": "c07d", "event": "task"},
		map[string]any{"date": "2026-09-17", "commit": f2, "by": "ben@example.org", "task": "c07d", "event": "status", "gate": "design", "state": "nominal",
			"note": "Sleep scheduler in progress", "subproject": "f1a0"},
		map[string]any{"date": "2026-09-26", "commit": w11, "by": "ben@example.org", "task": "c07d", "event": "status", "gate": "design"},
		map[string]any{"date": "2026-09-26", "commit": w11, "by": "ben@example.org", "task": "c07d", "event": "reviewed", "gate": "mockup"},
		map[string]any{"date": "2026-09-26", "commit": w11, "by": "ben@example.org", "task": "c07d", "event": "pin", "url": "firmware", "new": f2},
	}
	if !reflect.DeepEqual(events, any(wantEvents)) {
		gj, _ := json.MarshalIndent(events, "", " ")
		wj, _ := json.MarshalIndent(wantEvents, "", " ")
		t.Errorf("events:\n%s\nwant\n%s", gj, wj)
	}
}

// stubRunner returns a runner on a copy of the stub, with the stub's own
// variables in its environment. The stub stands where TABLO_BIN names a binary.
func stubRunner(t *testing.T, c *corpustest.Corpus, mode, lang string) (*runner, string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a shell script")
	}
	dir := t.TempDir()
	data, err := os.ReadFile(filepath.Join("testdata", "stub", "tablo"))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "tablo")
	if err := os.WriteFile(bin, data, 0o755); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(dir, "calls.log")
	env := append(c.Env(), "STUB_LOG="+log, "STUB_MODE="+mode, "STUB_LANG="+lang)
	return &runner{bin: bin, env: env, timeout: 2 * time.Second}, log
}

// firstCall returns the first logged call, with the corpus's work directory named.
func firstCall(t *testing.T, log, work string) string {
	t.Helper()
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	line, _, _ := strings.Cut(string(data), "\n")
	return strings.ReplaceAll(line, work, "<work>")
}

// TestRunner is T17: the options and their order, and what a command that does
// not answer gives.
func TestRunner(t *testing.T) {
	c := corpustest.Open(t)
	calls := []struct{ entry, want string }{
		{"weather-station", "-C <work>/build/weather-station --ref main --json validate"},
		{"no-tableaux-directory", "-C <work>/build/no-tableaux-directory --ref main --trunk main --json validate"},
		{"subproject-by-url", "-C <work>/build/subproject-by-url --ref main --replace https://example.org/lib.git=<work>/build/subproject-by-url.lib --json validate"},
		{"trunk-stated", "-C <work>/build/trunk-stated --ref develop --json validate"},
	}
	for _, tc := range calls {
		t.Run(tc.entry, func(t *testing.T) {
			r, log := stubRunner(t, c, "ok", "0.3")
			e := c.Entry(t, tc.entry)
			o := newObserver(r, e)
			o.observe(e.Expected)
			if len(o.errs) != 0 {
				t.Fatalf("errors: %v", o.errs)
			}
			if got := firstCall(t, log, c.Work); got != tc.want {
				t.Errorf("first call\n%s\nwant\n%s", got, tc.want)
			}
		})
	}

	// The task and history calls follow the same order, and name the level.
	r, log := stubRunner(t, c, "ok", "0.3")
	e := c.Entry(t, "weather-station")
	o := newObserver(r, e)
	o.observe(e.Expected)
	data, _ := os.ReadFile(log)
	for _, want := range []string{
		"--ref main --json task c07d --level provenance",
		"--ref main --json history --task c07d --level provenance",
		"--ref sensor-board --json task c07d --level provenance",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("no call %q in\n%s", want, data)
		}
	}

	for _, mode := range []string{"exit3", "hang", "text", "schema2"} {
		t.Run(mode, func(t *testing.T) {
			r, _ := stubRunner(t, c, mode, "0.3")
			r.timeout = 300 * time.Millisecond
			if _, err := r.run("-C", "x", "--json", "validate"); err == nil || !strings.HasPrefix(err.Error(), "command:") {
				t.Errorf("%s gives %v", mode, err)
			}
			o := newObserver(r, e)
			o.observe(e.Expected)
			if len(o.errs) == 0 || !strings.HasPrefix(o.errs[0], "command:") {
				t.Errorf("%s: observer errors %v", mode, o.errs)
			}
		})
	}
	r, _ = stubRunner(t, c, "exit3", "0.3")
	if _, err := r.run("validate"); err == nil || !strings.Contains(err.Error(), "cannot read the repository") {
		t.Errorf("exit 3 names the first line of standard error: %v", err)
	}
}

// TestLanguageSkip is T18: a command of another major version skips every entry.
func TestLanguageSkip(t *testing.T) {
	c := corpustest.Open(t)
	r, _ := stubRunner(t, c, "ok", "1.0")
	language, err := r.language()
	if err != nil || language != "1.0" {
		t.Fatalf("the stub's version answers %q, %v", language, err)
	}
	names := c.Names()
	if len(names) == 0 {
		t.Fatal("no entries")
	}
	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(c.Work, "entries", n, "expected.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		if major(textOf(mapKey(doc.Content[0], "language"))) == major(language) {
			t.Errorf("%s would run against a command of major %s", n, major(language))
		}
	}
	r, _ = stubRunner(t, c, "ok", "0.3")
	if language, _ := r.language(); major(language) != "0" {
		t.Errorf("the default stub answers %q", language)
	}
}
