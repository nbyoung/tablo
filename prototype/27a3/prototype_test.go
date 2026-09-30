package main

import (
	"os"
	"path/filepath"
	"testing"
)

const corpus = "/home/nbyoung/Projects/Tableaux/tableaux/corpus"

func TestParseYAML(t *testing.T) {
	m := asMap(parseYAML("a: 1 # note\nb: { x: \"q: r\", y: [ u, v ] }\nc:\n  - { k: docs/a.md#f, t: T }\n  - k2: z\nd: >\n  one\n  two\ne: null\n"))
	if str(m["a"]) != "1" || str(asMap(m["b"])["x"]) != "q: r" || len(asList(asMap(m["b"])["y"])) != 2 {
		t.Fatalf("scalars and flow: %v", m)
	}
	if c := asList(m["c"]); len(c) != 2 || str(asMap(c[0])["k"]) != "docs/a.md#f" || str(asMap(c[1])["k2"]) != "z" {
		t.Fatalf("sequence: %v", m["c"])
	}
	if str(m["d"]) != "one two" || m["e"] != nil {
		t.Fatalf("folded and null: %v", m)
	}
}

// entry derives one corpus entry and compares it with its expected.yaml.
func entry(t *testing.T, name, caller string) (*Derived, map[string]string) {
	t.Helper()
	repo := filepath.Join(corpus, "build", name)
	if _, err := os.Stat(repo); err != nil {
		t.Skip("corpus not built")
	}
	lab := readLabels(repo + ".labels.txt")
	d, err := derive(repo, "HEAD", caller)
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(corpus, "entries", name, "expected.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	out, bad := compare(repo, d, lab, asMap(parseYAML(string(src))))
	if bad > 0 {
		t.Errorf("%s disagrees with expected.yaml:\n%s", name, out)
	}
	return d, lab
}

func TestCorpusEntries(t *testing.T) {
	for _, name := range []string{"weather-station", "junction-kinds", "trunk-stated", "trunk-inferred", "trunk-undetermined"} {
		t.Run(name, func(t *testing.T) { entry(t, name, "") })
	}
}

func TestAuthorisationFromFirstParent(t *testing.T) {
	d, lab := entry(t, "weather-station", "")
	// The --no-ff merge W4 is on the first-parent line; the sensor-board commit W3 is not.
	if a := d.Auth["9f31"]; a.State != "authorised" || lbl(lab, a.Commit) != "W4" || a.By != "ben@example.org" {
		t.Errorf("9f31: %+v", a)
	}
	// dan revised the file at W12 after ada's Authorised trailer at W6, so the task is proposed again.
	if a := d.Auth["3c5d"]; a.State != "proposed" || lbl(lab, a.Commit) != "W12" {
		t.Errorf("3c5d: %+v", a)
	}
}

func TestReviewByTheRightPerson(t *testing.T) {
	d, lab := entry(t, "weather-station", "")
	r := d.Reviews["9f31"]
	if len(r) != 1 || r[0].Gate != "mockup" || lbl(lab, r[0].Commit) != "W9" {
		t.Errorf("9f31 reviews: %+v", r)
	}
	// ben is contributor and reviewer of c07d's mockup, so W11, which records the status, carries the trailer.
	if r := d.Reviews["c07d"]; len(r) != 1 || r[0].Gate != "mockup" || lbl(lab, r[0].Commit) != "W11" {
		t.Errorf("c07d reviews: %+v", r)
	}
	k, klab := entry(t, "junction-kinds", "")
	if r := k.Reviews["a300"]; len(r) != 1 || lbl(klab, r[0].Commit) != "K8" || r[0].Reviewer != "bot@example.org" {
		t.Errorf("a300 reviews: %+v", r)
	}
}

func TestExemptionLeavesF7Undecided(t *testing.T) {
	d, _ := entry(t, "junction-kinds", "")
	j := d.View.resolve("a210", "release")
	if j.Kind != "plain" || j.Reviewer != "" || j.Undecided != "F7" {
		t.Errorf("a210 release: %+v", j)
	}
}
