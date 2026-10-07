package load

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// counter writes a wrapper that counts each process and then runs git, for
// Options.Git. count returns the processes since the call before.
func counter(t testing.TB) (exe string, count func() int) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the counting wrapper is a shell script")
	}
	dir := t.TempDir()
	tally := filepath.Join(dir, "tally")
	exe = filepath.Join(dir, "git")
	script := "#!/bin/sh\nprintf 'x\\n' >> '" + tally + "'\nexec git \"$@\"\n"
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	seen := 0
	return exe, func() int {
		t.Helper()
		data, err := os.ReadFile(tally)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		total := strings.Count(string(data), "\n")
		n := total - seen
		seen = total
		return n
	}
}

// T24: the memo saves processes and never serves a stale link. The counts
// are the design's table of costs.
func TestMemo(t *testing.T) {
	exe, count := counter(t)
	l := loader(t, Options{Git: exe})

	// No subproject: two processes for the working tree, the discovery and the
	// ignored paths, and three for a revision.
	plainEntry := entry(t, "unmet-requirement")
	load(t, l, plainEntry, "")
	if n := count(); n != 2 {
		t.Errorf("the working tree, no subproject: %d processes, want 2", n)
	}
	load(t, l, plainEntry, "HEAD")
	if n := count(); n != 3 {
		t.Errorf("a revision, no subproject: %d processes, want 3", n)
	}

	// A submodule: three processes the first time, one while its pin stands.
	repo := clone(t, "submodule-subproject")
	sub := labels(t, "submodule-subproject.sub")
	first := load(t, l, repo, "")
	if n := count(); n != 2+3 {
		t.Errorf("the working tree and a submodule, first: %d processes, want 5", n)
	}
	second := load(t, l, repo, "")
	if n := count(); n != 2+1 {
		t.Errorf("the working tree and a submodule, again: %d processes, want 3", n)
	}
	if link(t, first, "sub").Project != link(t, second, "sub").Project {
		t.Error("the second load reads the submodule anew")
	}
	if first == second || !reflect.DeepEqual(first, second) {
		t.Error("the second load of the working tree is the first, or differs from it")
	}
	// At a revision: three for the project, one for the paths it links to, one for the pin.
	third := load(t, l, repo, "HEAD")
	if n := count(); n != 3+1+1 {
		t.Errorf("a revision and a submodule the memo holds: %d processes, want 5", n)
	}
	if link(t, third, "sub").Project != link(t, first, "sub").Project {
		t.Error("the load at HEAD reads the submodule anew")
	}
	// A fresh Loader pays for the submodule: three more.
	load(t, loader(t, Options{Git: exe}), repo, "HEAD")
	if n := count(); n != 3+1+3 {
		t.Errorf("a revision and a submodule, first: %d processes, want 7", n)
	}
	// A moved pin is a new project, and the old one stays in the memo.
	gitRun(t, filepath.Join(repo, "sub"), "checkout", "-q", sub["S4"])
	count()
	moved := load(t, l, repo, "")
	if n := count(); n != 2+3 {
		t.Errorf("after the pin moves: %d processes, want 5", n)
	}
	if got := link(t, moved, "sub"); got.Commit != sub["S4"] || gate(t, got, "5a00") != "design" {
		t.Errorf("after the pin moves: %+v", got)
	}
	gitRun(t, filepath.Join(repo, "sub"), "checkout", "-q", sub["S3"])
	count()
	if back := load(t, l, repo, ""); link(t, back, "sub").Project != link(t, first, "sub").Project || count() != 3 {
		t.Error("the pin moved back, and the memo does not serve the project it held")
	}

	// A link that did not resolve is never remembered: it resolves once the mapping exists.
	const url = "https://example.org/lib.git"
	byURL := clone(t, "subproject-by-url")
	if got := link(t, load(t, l, byURL, "HEAD"), url); got.Problem != model.NoClone {
		t.Fatalf("with no mapping: %+v", got)
	}
	gitRun(t, byURL, "config", "tableaux."+url+".path", entry(t, "subproject-by-url.lib"))
	if got := link(t, load(t, l, byURL, "HEAD"), url); got.Problem != model.Resolved || gate(t, got, "5a00") != "design" {
		t.Errorf("once the mapping exists: %+v, %s", got, got.Detail)
	}
	// A project that holds an unresolved link is not remembered either, at any depth.
	outer := repository(t, "outer")
	plan(t, outer, "", "{ subproject: { url: inner } }")
	plan(t, outer, "inner", "{ subproject: { url: later } }")
	commit(t, outer, "Plan two projects, the second with a link to nowhere")
	inner := link(t, load(t, l, outer, "HEAD"), "inner")
	if got := link(t, inner.Project, "later"); got.Problem != model.Missing {
		t.Fatalf("the inner link: %+v", got)
	}
	plan(t, outer, "later", "")
	commit(t, outer, "Plan the third")
	if got := link(t, link(t, load(t, l, outer, "HEAD"), "inner").Project, "later"); got.Problem != model.Resolved {
		t.Errorf("after the commit: %+v", got)
	}
}

// The memo holds 256 projects and drops the one least recently used.
func TestMemoIsBounded(t *testing.T) {
	l := loader(t, Options{})
	remember := func(i int) key {
		k := key{gitDir: "/repository/.git", treeish: fmt.Sprintf("%040x", i)}
		p := &model.Project{Where: model.Location{GitDir: k.gitDir, Ref: k.treeish, Commit: k.treeish}, Exists: true}
		s := &session{loader: l, root: key{gitDir: "/elsewhere/.git"}, keys: map[*model.Project]key{p: k}, fresh: []*node{{project: p, key: k}}}
		s.remember()
		return k
	}
	recall := func(k key) *model.Project {
		s := &session{loader: l, seen: map[key]*model.Project{}, keys: map[*model.Project]key{}}
		return s.recall(k, 1)
	}
	first := remember(0)
	for i := 1; i < memoSize; i++ {
		remember(i)
	}
	if recall(first) == nil {
		t.Fatal("the memo forgets a project before it is full")
	}
	// The first is now the most recently used, so the second goes.
	last := remember(memoSize)
	second := key{gitDir: first.gitDir, treeish: fmt.Sprintf("%040x", 1)}
	if recall(first) == nil || recall(last) == nil || recall(second) != nil {
		t.Error("the memo does not drop the project least recently used")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.memo) != memoSize || l.order.Len() != memoSize {
		t.Errorf("the memo holds %d projects in a list of %d, want %d", len(l.memo), l.order.Len(), memoSize)
	}
}

// ordered checks that every slice of every project p reaches has the order
// its comment states.
func ordered(t *testing.T, name string, p *model.Project) (projects int) {
	t.Helper()
	seen := map[*model.Project]bool{}
	var walk func(*model.Project)
	walk = func(p *model.Project) {
		if p == nil || seen[p] {
			return
		}
		seen[p] = true
		if !sort.SliceIsSorted(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path }) {
			t.Errorf("%s: the files are not by path", name)
		}
		if !sort.StringsAreSorted(p.Stray) || !sort.StringsAreSorted(p.TaskIDs()) {
			t.Errorf("%s: the stray paths or the task ids are not in byte order", name)
		}
		if !sort.SliceIsSorted(p.Links, func(i, j int) bool {
			a, b := p.Links[i], p.Links[j]
			return a.URL < b.URL || (a.URL == b.URL && a.Commit < b.Commit)
		}) {
			t.Errorf("%s: the links are not by URL then commit", name)
		}
		if !sort.SliceIsSorted(p.Diagnostics, func(i, j int) bool {
			a, b := p.Diagnostics[i], p.Diagnostics[j]
			if a.Pos != b.Pos {
				return a.Pos.File < b.Pos.File || (a.Pos.File == b.Pos.File && (a.Pos.Line < b.Pos.Line || (a.Pos.Line == b.Pos.Line && a.Pos.Col < b.Pos.Col)))
			}
			return a.Code < b.Code
		}) {
			t.Errorf("%s: the diagnostics are not by file, line, column, code", name)
		}
		for _, l := range p.Links {
			walk(l.Project)
		}
	}
	walk(p)
	return len(seen)
}

// T25: the output is deterministic and safe to share.
func TestDeterministicAndShared(t *testing.T) {
	const umbrella, lib = "https://example.org/umbrella.git", "https://example.org/lib.git"
	options := Options{CacheDir: t.TempDir(), Replace: map[string]string{
		umbrella: entry(t, "requires-across-projects.umbrella"),
		lib:      entry(t, "subproject-by-url.lib"),
	}}
	names := []string{"weather-station", "requires-across-projects", "subproject-by-url", "subproject-same-repository",
		"submodule-subproject", "junction-kinds", "file-duplicate-key", "file-stray-path", "no-tableaux-directory"}
	type reading struct{ name, ref string }
	var readings []reading
	for _, name := range names {
		readings = append(readings, reading{name, "HEAD"}, reading{name, ""})
	}

	// One Loader, whose second load meets the memo, and a second Loader agree.
	reference := map[reading]*model.Project{}
	one := New(options)
	projects := 0
	for _, r := range readings {
		first := load(t, one, entry(t, r.name), r.ref)
		again := load(t, one, entry(t, r.name), r.ref)
		other := load(t, New(options), entry(t, r.name), r.ref)
		for which, p := range map[string]*model.Project{"the same Loader": again, "another Loader": other} {
			if d := diff(first, p); d != "" || !reflect.DeepEqual(first, p) {
				t.Errorf("%s at %q: a second load by %s differs at %s", r.name, r.ref, which, d)
			}
		}
		projects += ordered(t, r.name, first)
		reference[r] = first
	}
	if projects < len(readings)+8 {
		t.Errorf("the readings reach %d projects; the links add more", projects)
	}
	broken := load(t, one, broken(t), "")
	ordered(t, "broken", broken)

	// Eight concurrent loads on one Loader, for the race detector, each
	// deep-equal to the reference.
	shared := New(options)
	var wg sync.WaitGroup
	failures := make(chan string, 8*len(readings))
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := range readings {
				r := readings[(i+worker)%len(readings)]
				p, err := shared.Load(t.Context(), Source{Dir: entry(t, r.name), Ref: r.ref})
				if err != nil {
					failures <- fmt.Sprintf("%s at %q: %v", r.name, r.ref, err)
				} else if !reflect.DeepEqual(p, reference[r]) {
					failures <- fmt.Sprintf("%s at %q differs at %s", r.name, r.ref, diff(p, reference[r]))
				}
			}
		}(worker)
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
}

// umbrella returns the repository that holds the corpus, which the entry
// tableaux-tooling reads in place: source: { repository: ../../.., ref: corpus/tooling }.
func umbrella(t testing.TB) string {
	t.Helper()
	return filepath.Dir(filepath.Dir(corpus(t)))
}

// T27: the family's own plan loads clean.
func TestOwnPlan(t *testing.T) {
	p, err := loader(t, Options{}).Load(t.Context(), Source{Dir: umbrella(t), Ref: "corpus/tooling"})
	if errors.Is(err, ErrNoRef) || errors.Is(err, ErrNoRepository) {
		t.Skipf("the corpus sits in no repository with the tag corpus/tooling: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 37 || len(p.Diagnostics) > 0 || !p.Exists || p.Version == nil || !p.Version.Accepted || p.Gating == nil {
		t.Errorf("tableaux-tooling: %d tasks, diagnostics %v", len(p.Tasks), codes(p.Diagnostics))
	}
	tablo := link(t, p, "subprojects/tablo")
	if tablo.Form != model.Submodule || tablo.Commit == "" {
		t.Errorf("subprojects/tablo: %+v", tablo)
	}
	// Whether the submodule has a clone is the host's affair.
	t.Logf("subprojects/tablo at %.12s: %s %s", tablo.Commit, tablo.Problem, tablo.Detail)
	if tablo.Problem == model.Resolved && (len(tablo.Project.Tasks) == 0 || len(tablo.Project.Diagnostics) > 0) {
		t.Errorf("subprojects/tablo: %d tasks, diagnostics %v", len(tablo.Project.Tasks), codes(tablo.Project.Diagnostics))
	}
}

// BenchmarkUmbrellaReload prints the cost of a reload of the umbrella plan's
// working tree, with the memo warm as a front end holds it. It asserts nothing.
func BenchmarkUmbrellaReload(b *testing.B) {
	repo := umbrella(b)
	l := loader(b, Options{})
	p, err := l.Load(b.Context(), Source{Dir: repo})
	if err != nil {
		b.Skipf("the umbrella plan does not load: %v", err)
	}
	b.Logf("%d tasks, %d files, %d links", len(p.Tasks), len(p.Files), len(p.Links))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := l.Load(b.Context(), Source{Dir: repo}); err != nil {
			b.Fatal(err)
		}
	}
}
