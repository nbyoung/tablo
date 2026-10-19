package tablo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/model"
)

// tooling returns the repository and the ref of the corpus entry
// tableaux-tooling, which reads the family's own plan in place, in the
// corpus that TABLO_CORPUS names, as the conformance run reaches that
// entry. It skips without the variable: the embedded copy of the corpus has
// no tableaux checkout beside it.
func tooling(b testing.TB) (dir, ref string) {
	b.Helper()
	corpus := os.Getenv("TABLO_CORPUS")
	if corpus == "" {
		b.Skip("tableaux-tooling is read in place, and only a corpus that TABLO_CORPUS names has a tableaux checkout beside it")
	}
	entry := filepath.Join(corpus, "entries", "tableaux-tooling")
	data, err := os.ReadFile(filepath.Join(entry, "expected.yaml"))
	if err != nil {
		b.Fatalf("TABLO_CORPUS=%s: %v", corpus, err)
	}
	var want struct {
		Source struct {
			Repository string `yaml:"repository"`
			Ref        string `yaml:"ref"`
		} `yaml:"source"`
	}
	if err := yaml.Unmarshal(data, &want); err != nil {
		b.Fatalf("tableaux-tooling: %v", err)
	}
	if want.Source.Repository == "" || want.Source.Ref == "" {
		b.Fatal("tableaux-tooling states no source")
	}
	dir, err = filepath.Abs(filepath.Join(entry, filepath.FromSlash(want.Source.Repository)))
	if err != nil {
		b.Fatal(err)
	}
	return dir, want.Source.Ref
}

// tasks counts the tasks of a project and of every subproject it reads.
func tasks(p *model.Project) int {
	n := len(p.Tasks)
	for _, l := range p.Links {
		if l.Project != nil {
			n += tasks(l.Project)
		}
	}
	return n
}

// BenchmarkLoad prints the cost of Load on the family's own plan, the corpus
// entry tableaux-tooling at corpus/tooling, the reload cost a linked front
// end pays (design 4ed9, "Library or executable"). It asserts nothing.
// First opens a repository and loads it, as a front end does at its start;
// Unmoved loads once more on the same repository after a change that moves
// no commit, as a front end does on each change it detects.
func BenchmarkLoad(b *testing.B) {
	dir, ref := tooling(b)
	ctx := context.Background()
	b.Run("First", func(b *testing.B) {
		for b.Loop() {
			r, err := Open(ctx, dir, Options{})
			if err != nil {
				b.Fatal(err)
			}
			if _, err := r.Load(ctx, ref); err != nil {
				b.Fatal(err)
			}
		}
	})
	r, err := Open(ctx, dir, Options{})
	if err != nil {
		b.Fatal(err)
	}
	s, err := r.Load(ctx, ref)
	if err != nil {
		b.Fatal(err)
	}
	b.Logf("%s at %.12s: %d tasks with its subprojects, %d files, %d commits", ref, s.project.Where.Commit, tasks(s.project), len(s.project.Files), len(s.facts.Log().Commits))
	b.Run("Unmoved", func(b *testing.B) {
		for b.Loop() {
			if _, err := r.Load(ctx, ref); err != nil {
				b.Fatal(err)
			}
		}
	})
}
