package main

import (
	"os"
	"strings"
	"testing"
)

const corpus = "/home/nbyoung/Projects/Tableaux/tableaux/corpus/build"

func labelled(t *testing.T, entry, label string) (Loader, string) {
	t.Helper()
	if _, err := os.Stat(corpus + "/" + entry); err != nil {
		t.Skip("corpus not built")
	}
	h, err := lookup(corpus+"/"+entry+".labels.txt", label)
	if err != nil {
		t.Fatal(err)
	}
	return Loader{corpus + "/" + entry}, h
}

func TestParseYAMLPositions(t *testing.T) {
	src := "# c\ntitle: A\nlist:\n  - { url: \"x: y\", text: T }\n  - { url: z }\nd: >\n  one\n  two\nend: 1\n"
	n, err := ParseYAML(src)
	if err != nil {
		t.Fatal(err)
	}
	if got := n.Get("title"); got.Value != "A" || got.Line != 2 || got.Col != 8 {
		t.Errorf("title = %+v", got)
	}
	if got := n.Get("list").Items[0].Get("url"); got.Value != "x: y" || got.Line != 4 || got.Col != 12 {
		t.Errorf("url = %+v", got)
	}
	if got := n.Get("d").Value; got != "one two\n" {
		t.Errorf("folded = %q", got)
	}
	if got := n.Get("end"); got.Value != "1" || got.Line != 9 {
		t.Errorf("end = %+v", got)
	}
}

func TestWeatherStationAtTwoCommits(t *testing.T) {
	l, w1 := labelled(t, "weather-station", "W1")
	_, w6 := labelled(t, "weather-station", "W6")
	p1, err := l.Load(w1)
	if err != nil {
		t.Fatal(err)
	}
	p6, err := l.Load(w6)
	if err != nil {
		t.Fatal(err)
	}
	if len(p1.Tasks) != 3 || len(p6.Tasks) != 6 {
		t.Errorf("tasks W1 %d, W6 %d; want 3 and 6", len(p1.Tasks), len(p6.Tasks))
	}
	if p1.Tasks["9f31"] != nil || p6.Tasks["9f31"] == nil {
		t.Error("9f31 exists only at W6")
	}
	if len(p6.Tasks["9f31"].References) != 2 || len(p6.Tasks["c07d"].Requires) != 1 {
		t.Error("list fields not typed")
	}
	if len(p1.Diags)+len(p6.Diags) != 0 {
		t.Errorf("unexpected diagnostics %v %v", p1.Diags, p6.Diags)
	}
}

func TestReadsRefNotWorktree(t *testing.T) {
	l, w1 := labelled(t, "weather-station", "W1")
	// The worktree holds the tip; the loader must still return W1's tree.
	files, err := l.Files(w1)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if strings.HasPrefix(f, "status/") {
			t.Errorf("W1 has no status files, got %s", f)
		}
	}
}

func TestInvalidEntryPosition(t *testing.T) {
	l, h := labelled(t, "task-bad-email", "only")
	p, err := l.Load(h)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Diags) != 1 {
		t.Fatalf("diags = %v", p.Diags)
	}
	if d := p.Diags[0]; d.Pos.File != "tasks/b2c9.yaml" || d.Pos.Line != 4 || d.Pos.Col != 11 {
		t.Errorf("position = %v", d.Pos)
	}
}

func TestVersionRule(t *testing.T) {
	for entry, want := range map[string]string{
		"version-minor-ahead":    "P4",
		"version-major-mismatch": "P4",
		"version-bad-pattern":    "P3",
	} {
		l, h := labelled(t, entry, "only")
		p, err := l.Load(h)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Diags) != 1 || !strings.HasPrefix(p.Diags[0].Msg, want) || p.Diags[0].Pos.File != "version.yaml" {
			t.Errorf("%s: diags = %v, want %s", entry, p.Diags, want)
		}
	}
}
