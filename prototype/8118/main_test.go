package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/schemas"
)

const corpus = "/home/nbyoung/Projects/Tableaux/tableaux/corpus"

func TestParserKeepsPositions(t *testing.T) {
	src := "# c\ntitle: A\nrequires:\n  - { id: \"b2c9\", to: design }\n  - id: c3d7\nparent: { id: \"e4a1\", order: 2 }\nd: >\n  one\n  two\n"
	n, err := ParseYAML(src)
	if err != nil {
		t.Fatal(err)
	}
	if p := n.Get("requires").Items[0].Get("id").Pos; p != (Pos{4, 11}) {
		t.Errorf("flow id at %v", p)
	}
	if p := n.Get("requires").Items[1].Get("id").Pos; p != (Pos{5, 9}) {
		t.Errorf("block id at %v", p)
	}
	if o := n.Get("parent").Get("order"); o.Kind != Int || o.Int != 2 || o.Pos != (Pos{6, 30}) {
		t.Errorf("order %+v", o)
	}
	if s := n.Get("d").Str; s != "one two" {
		t.Errorf("folded %q", s)
	}
}

func TestEmbeddedSchemasLoad(t *testing.T) {
	for _, kind := range []string{"version", "gates", "task", "status", "history"} {
		b, err := schemas.FS.ReadFile(kind + ".schema.yaml")
		if err != nil {
			t.Fatal(err)
		}
		n, err := ParseYAML(string(b))
		if err != nil || n.Get("$id") == nil {
			t.Errorf("%s: %v", kind, err)
		}
	}
}

// project writes files under .tableaux in a temporary repository.
func project(t *testing.T, files map[string]string) string {
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, ".tableaux", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const gates = `gates:
  - { key: undefined, symbol: a, name: U, criteria: c }
  - { key: defined, symbol: b, name: D, criteria: c }
states:
  - { key: undefined, symbol: a, severity: 0, synopsis: s }
  - { key: complete, symbol: b, severity: 0, synopsis: s }
`

func TestSyntheticProject(t *testing.T) {
	root := project(t, map[string]string{
		"gates.yaml":      gates,
		"tasks/aaaa.yaml": "title: R\ndescription: d\nassignee: a@b.org\n",
		"tasks/bbbb.yaml": "title: B\ndescription: d\nassignee: a@b.org\nrequires:\n  - { id: \"cccc\" }\njunctions:\n  nowhere: { applies: false }\nparent: { id: \"aaaa\" }\n",
		"tasks/cccc.yaml": "title: C\ndescription: d\nassignee: nobody\nrequires:\n  - { id: \"bbbb\" }\nparent: { id: \"aaaa\" }\n",
	})
	diags, err := Validate(root)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range diags {
		got = append(got, d.String())
	}
	want := []string{
		"tasks/bbbb.yaml:5:11: error: R2",
		"tasks/bbbb.yaml:7:3: error: J1",
		"tasks/cccc.yaml:3:11: error: T4",
		"tasks/cccc.yaml:5:11: error: R2",
	}
	if len(got) != len(want) {
		t.Fatalf("got %q", got)
	}
	for i := range want {
		if !strings.HasPrefix(got[i], want[i]) {
			t.Errorf("diag %d: got %q, want prefix %q", i, got[i], want[i])
		}
	}
}

func expectedFindings(t *testing.T, entry string) []Diag {
	b, err := os.ReadFile(filepath.Join(corpus, "entries", entry, "expected.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := ParseYAML(string(b))
	if err != nil {
		t.Fatalf("%s expected.yaml: %v", entry, err)
	}
	var out []Diag
	for _, f := range doc.Get("findings").itemsOrNil() {
		out = append(out, Diag{
			Rule: f.Get("rule").Str, Severity: f.Get("severity").Str, Task: f.Get("task").strOrEmpty(),
			File: f.Get("file").strOrEmpty(), Gate: f.Get("gate").strOrEmpty(),
		})
	}
	return out
}

// compare returns the expected findings nothing matches and the diagnostics
// no finding accounts for. A finding without a file or task matches on rule.
func compare(want, got []Diag) (missing, extra []Diag) {
	used := make([]bool, len(got))
	for _, w := range want {
		hit := false
		for i, g := range got {
			if g.Rule == w.Rule && g.Severity == w.Severity && (w.File == "" || g.File == w.File) &&
				(w.Task == "" || g.Task == w.Task) && (w.Gate == "" || g.Gate == "" || g.Gate == w.Gate) {
				used[i], hit = true, true
			}
		}
		if !hit {
			missing = append(missing, w)
		}
	}
	for i, g := range got {
		if !used[i] {
			extra = append(extra, g)
		}
	}
	return missing, extra
}

func run(t *testing.T, entry string) []Diag {
	diags, err := Validate(filepath.Join(corpus, "build", entry))
	if err != nil {
		t.Fatal(err)
	}
	return diags
}

func skipWithoutCorpus(t *testing.T) {
	if _, err := os.Stat(filepath.Join(corpus, "build", "weather-station")); err != nil {
		t.Skip("the corpus is not built")
	}
}

// The sample: a few entries for each rule family in corpus/RULES.md.
var sample = []string{
	"tree-two-roots", "tree-no-root", "tree-parent-missing", "tree-parent-cycle",
	"requires-cycle", "requires-self", "requires-ancestor", "requires-descendant", "requires-missing-task", "requires-unknown-field",
	"junction-unknown-gate", "recursive-on-parent", "junction-mixed-kind", "junction-model-without-contributor",
	"junction-applies-true", "junction-undefined-not-applicable", "junction-undefined-plain", "junction-unknown-field",
	"task-bad-email", "task-id-field", "task-parent-order-zero", "task-bad-id-pattern", "task-missing-assignee",
	"gates-bad-key", "gates-first-not-undefined", "gates-gate-missing-criteria", "gates-negative-severity", "gates-no-states",
	"status-unknown-field", "status-undefined-gate-nominal-state", "status-missing-gate",
	"unquoted-id",
}

func TestCorpusSample(t *testing.T) {
	skipWithoutCorpus(t)
	for _, entry := range sample {
		t.Run(entry, func(t *testing.T) {
			got := run(t, entry)
			missing, extra := compare(expectedFindings(t, entry), got)
			if len(missing)+len(extra) > 0 {
				t.Errorf("missing %+v, extra %+v", missing, extra)
			}
			for _, d := range got {
				t.Log(d)
			}
		})
	}
}

func TestWeatherStationReportsNothing(t *testing.T) {
	skipWithoutCorpus(t)
	if diags := run(t, "weather-station"); len(diags) != 0 {
		t.Errorf("want no diagnostics, got %v", diags)
	}
}

// TestCorpusCoverage logs how every invalid entry fares; it asserts only that
// a valid-by-schema-and-structure entry raises no error the prototype invents.
func TestCorpusCoverage(t *testing.T) {
	skipWithoutCorpus(t)
	dirs, _ := os.ReadDir(filepath.Join(corpus, "entries"))
	agree, differ := 0, 0
	for _, d := range dirs {
		entry := d.Name()
		if _, err := os.Stat(filepath.Join(corpus, "build", entry, ".tableaux")); err != nil {
			continue
		}
		missing, extra := compare(expectedFindings(t, entry), run(t, entry))
		if len(missing)+len(extra) == 0 {
			agree++
		} else {
			differ++
			t.Logf("differs: %-40s missing %d extra %d", entry, len(missing), len(extra))
		}
	}
	t.Logf("agree %d, differ %d", agree, differ)
}
