package audit

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// corpus returns the directory of the built conformance corpus, and skips
// the test when it is absent. It carries the Loader's stand-in for the
// helper of task 4b4f (assumption A7): TABLEAUX_CORPUS, else
// ../tableaux/corpus/build beside the checkout.
func corpus(t testing.TB) string {
	t.Helper()
	dir := os.Getenv("TABLEAUX_CORPUS")
	if dir == "" {
		dir = filepath.Join("..", "..", "..", "tableaux", "corpus", "build")
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Skipf("the built corpus is absent at %s; set TABLEAUX_CORPUS", dir)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// builtAt returns the path of a built corpus repository, which a test reads
// and never changes.
func builtAt(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join(corpus(t), name)
}

// loaded reads a source with a Loader whose cache is an empty temporary
// directory, and fails the test on a load error.
func loaded(t testing.TB, options load.Options, dir, ref string) *model.Project {
	t.Helper()
	if options.CacheDir == "" {
		options.CacheDir = t.TempDir()
	}
	p, err := load.New(options).Load(context.Background(), load.Source{Dir: dir, Ref: ref})
	if err != nil {
		t.Fatalf("Load(%q, %q): %v", dir, ref, err)
	}
	return p
}

// derived loads a source and derives its facts from the passes of a new Reader.
func derived(t testing.TB, options load.Options, dir, ref, trunk string) *derive.Facts {
	t.Helper()
	p := loaded(t, options, dir, ref)
	set, err := history.NewReader(history.Options{}).Read(context.Background(), p, trunk)
	if err != nil {
		t.Fatalf("Read(%q, %q): %v", dir, ref, err)
	}
	return derive.New(p, set)
}

// The version and the gating of a project a test composes in memory.
const (
	memVersion = "tableaux: 0.3.1\ntrunk: main\n"
	memGates   = `gates:
  - { key: undefined }
  - { key: defined }
  - { key: mockup }
  - { key: function }
  - { key: design }
  - { key: release }
states:
  - { key: undefined, severity: 0 }
  - { key: nominal, severity: 1 }
  - { key: stalled, severity: 3 }
  - { key: complete, severity: 0 }
`
)

// mem derives the facts of a project composed in memory from files below
// .tableaux, through load.Parse and load.Compose, with no history.
func mem(t testing.TB, files map[string]string) *derive.Facts {
	t.Helper()
	all := map[string]string{"version.yaml": memVersion, "gates.yaml": memGates}
	for name, text := range files {
		all[name] = text
	}
	var names []string
	for name := range all {
		names = append(names, name)
	}
	sort.Strings(names)
	var pieces []*load.Piece
	for _, name := range names {
		piece := load.Parse(name, []byte(all[name]))
		if piece == nil {
			t.Fatalf("the layout names no file %s", name)
		}
		pieces = append(pieces, piece)
	}
	p := load.Compose(model.Location{}, pieces, nil)
	for _, d := range p.Diagnostics {
		t.Fatalf("the case does not read: %+v", d)
	}
	f := derive.New(p, nil)
	if f.Refused() != nil {
		t.Fatalf("the case is refused: %+v", f.Refused())
	}
	return f
}

// audited runs the audit and fails the test on an error or on no report.
func audited(t testing.TB, in Input) *Report {
	t.Helper()
	rep, err := Run(in)
	if err != nil || rep == nil {
		t.Fatalf("Run: %v, %v", rep, err)
	}
	return rep
}

// of returns the findings of a report with a key.
func of(rep *Report, key string) []*Finding {
	var list []*Finding
	for i := range rep.Findings {
		if rep.Findings[i].Key == key {
			list = append(list, &rep.Findings[i])
		}
	}
	return list
}
