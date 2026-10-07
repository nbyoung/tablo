package load

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// The smallest files a project needs for a test that makes its own.
const (
	versionYAML = "tableaux: 0.3.1\ntrunk: main\n"
	gatesYAML   = "gates:\n" +
		"  - { key: undefined, symbol: ❔, name: Undefined, criteria: No one has started work on the definition }\n" +
		"  - { key: design, symbol: 📐, name: Design, criteria: A model and sufficient tests exist }\n" +
		"states:\n" +
		"  - { key: undefined, symbol: ⚪, severity: 0, synopsis: The work has not yet been defined }\n" +
		"  - { key: nominal, symbol: 🟢, severity: 1, synopsis: The work is proceeding as expected }\n"
)

// plan writes a project of one root task into dir of a repository. junction
// is the root's design junction, such as "{ subproject: { url: lib } }".
func plan(t testing.TB, repo, dir, junction string) {
	t.Helper()
	task := "title: Root\ndescription: The root of a test project.\nassignee: olive@example.org\n"
	if junction != "" {
		task += "junctions:\n  design: " + junction + "\n"
	}
	write(t, repo, filepath.ToSlash(filepath.Join(dir, ".tableaux", "version.yaml")), versionYAML)
	write(t, repo, filepath.ToSlash(filepath.Join(dir, ".tableaux", "gates.yaml")), gatesYAML)
	write(t, repo, filepath.ToSlash(filepath.Join(dir, ".tableaux", "tasks", "a000.yaml")), task)
}

// commit stages everything in a repository, commits it, and returns the commit.
func commit(t testing.TB, repo, subject string) string {
	t.Helper()
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "--allow-empty", "-m", subject)
	return gitRun(t, repo, "rev-parse", "HEAD")
}

// repository makes an empty repository named name in a temporary directory.
func repository(t testing.TB, name string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q", "-b", "main")
	return repo
}

// gate returns the gate the status of a task states in the project of a link.
func gate(t testing.TB, l *model.Link, id string) string {
	t.Helper()
	if l.Problem != model.Resolved || l.Project == nil || l.Project.Statuses[id] == nil {
		t.Fatalf("the link %q: %s, %s; no status of %s", l.URL, l.Problem, l.Detail, id)
	}
	return l.Project.Statuses[id].Gate.V
}

// T15: a submodule reads at its pin.
func TestSubmoduleAtItsPin(t *testing.T) {
	l := loader(t, Options{})
	parent, sub := labels(t, "submodule-subproject"), labels(t, "submodule-subproject.sub")
	repo := entry(t, "submodule-subproject")

	main := load(t, l, repo, "main")
	pinned := link(t, main, "sub")
	// The subproject's trunk stands at S4; the parent pins S3.
	if pinned.Form != model.Submodule || pinned.Commit != sub["S3"] || pinned.Checkout != "" || gate(t, pinned, "5a00") != "function" {
		t.Errorf("at main: %+v", pinned)
	}
	if note := pinned.Project.Statuses["5a00"].Note.V; note != "Prototype runs on the bench" {
		t.Errorf("at main: the note %q", note)
	}
	if where := pinned.Project.Where; where.Commit != sub["S3"] || where.Worktree || where.Dir != "" ||
		where.GitDir != filepath.Join(repo, ".git", "modules", "sub") {
		t.Errorf("at main: the subproject is read at %+v", where)
	}
	// Every field that names the submodule shares the one link.
	delegate := main.Tasks["c100"]
	if len(main.Links) != 1 || len(delegate.Junctions) != 10 {
		t.Fatalf("at main: %d links, %d junctions", len(main.Links), len(delegate.Junctions))
	}
	for _, j := range delegate.Junctions {
		if j.Subproject.Link != pinned {
			t.Errorf("the junction %s has a link of its own", j.Gate)
		}
	}

	early := link(t, load(t, l, repo, parent["U2"]), "sub")
	if early.Commit != sub["S1"] || gate(t, early, "5a00") != "defined" {
		t.Errorf("at U2: %+v", early)
	}

	station := load(t, l, entry(t, "weather-station"), "main")
	firmware := link(t, station, "firmware")
	if firmware.Form != model.Submodule || firmware.Problem != model.Resolved || firmware.Commit != labels(t, "weather-station.firmware")["F2"] {
		t.Errorf("weather-station: %+v", firmware)
	}
	if station.Tasks["c07d"].Junctions[1].Subproject.Link != firmware || firmware.Project.Tasks["f1a0"] == nil {
		t.Errorf("weather-station: the junction's link leads to %v", firmware.Project.TaskIDs())
	}
}

// T16: a submodule reads with no checkout, from modules/<name> under the Git
// directory, and has no clone where the repository holds none.
func TestSubmoduleWithoutCheckout(t *testing.T) {
	l := loader(t, Options{})
	pin := labels(t, "submodule-subproject.sub")["S3"]
	repo := clone(t, "submodule-subproject")
	if err := os.RemoveAll(filepath.Join(repo, "sub")); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"HEAD", ""} {
		// With the checkout gone, the working tree's pin is the gitlink of the index.
		got := link(t, load(t, l, repo, ref), "sub")
		if got.Form != model.Submodule || got.Commit != pin || got.Checkout != "" || gate(t, got, "5a00") != "function" {
			t.Errorf("with no checkout, at %q: %+v", ref, got)
		}
	}

	// A submodule whose name differs from its path.
	lib := repository(t, "lib")
	plan(t, lib, "", "")
	libCommit := commit(t, lib, "Plan the library")
	parent := repository(t, "parent")
	plan(t, parent, "", "{ subproject: { url: vendor/lib } }")
	gitRun(t, parent, "submodule", "add", "-q", "--name", "engine", lib, "vendor/lib")
	commit(t, parent, "Pin the library")
	if !exists(filepath.Join(parent, ".git", "modules", "engine")) {
		t.Fatal("git keeps the submodule's clone elsewhere than .git/modules/engine")
	}
	for _, ref := range []string{"HEAD", ""} {
		got := link(t, load(t, l, parent, ref), "vendor/lib")
		if got.Form != model.Submodule || got.Problem != model.Resolved || got.Commit != libCommit || (got.Checkout != "") != (ref == "") {
			t.Errorf("with a checkout, at %q: %+v", ref, got)
		}
	}
	// A linked worktree has no checkout and shares the clone under the common Git directory.
	linked := filepath.Join(t.TempDir(), "linked")
	gitRun(t, parent, "worktree", "add", "-q", "--detach", linked)
	for _, ref := range []string{"HEAD", ""} {
		got := link(t, load(t, l, linked, ref), "vendor/lib")
		if got.Form != model.Submodule || got.Problem != model.Resolved || got.Commit != libCommit || got.Checkout != "" {
			t.Errorf("in a linked worktree, at %q: %+v, %s", ref, got, got.Detail)
		}
	}
	if err := os.RemoveAll(filepath.Join(parent, "vendor")); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"HEAD", ""} {
		got := link(t, load(t, l, parent, ref), "vendor/lib")
		if got.Form != model.Submodule || got.Problem != model.Resolved || got.Commit != libCommit || got.Project.Tasks["a000"] == nil {
			t.Errorf("by its name, at %q: %+v, %s", ref, got, got.Detail)
		}
	}

	// A plain clone holds the gitlink and no clone of the submodule.
	plain := filepath.Join(t.TempDir(), "plain")
	gitRun(t, parent, "clone", "-q", parent, plain)
	for _, ref := range []string{"HEAD", ""} {
		got := link(t, load(t, l, plain, ref), "vendor/lib")
		if got.Form != model.Submodule || got.Problem != model.NoClone || got.Project != nil || got.Commit != libCommit {
			t.Errorf("in a plain clone, at %q: %+v", ref, got)
		}
	}
}

// T17: the working tree's pin is the checkout's HEAD, and the Loader says
// which checkout it read (decision 8).
func TestWorkingTreePinIsCheckoutHead(t *testing.T) {
	l := loader(t, Options{})
	sub := labels(t, "submodule-subproject.sub")
	repo := clone(t, "submodule-subproject")
	checkout := filepath.Join(repo, "sub")

	before := link(t, load(t, l, repo, ""), "sub")
	if before.Commit != sub["S3"] || before.Checkout != checkout || gate(t, before, "5a00") != "function" {
		t.Errorf("before the move: %+v", before)
	}
	// Move the checkout and commit nothing, not even to the index.
	gitRun(t, checkout, "checkout", "-q", sub["S1"])
	moved := link(t, load(t, l, repo, ""), "sub")
	if moved.Form != model.Submodule || moved.Commit != sub["S1"] || moved.Checkout != checkout || gate(t, moved, "5a00") != "defined" {
		t.Errorf("after the move, the working tree: %+v", moved)
	}
	if where := moved.Project.Where; where.Worktree || where.Commit != sub["S1"] {
		t.Errorf("the subproject reads at a commit, not from its disk: %+v", where)
	}
	head := link(t, load(t, l, repo, "HEAD"), "sub")
	if head.Commit != sub["S3"] || head.Checkout != "" || gate(t, head, "5a00") != "function" {
		t.Errorf("after the move, HEAD: %+v", head)
	}
	// The files of the checkout are not what the Loader reads: the pin is.
	write(t, checkout, ".tableaux/status/5a00.yaml", "gate: release\nstate: complete\n")
	if edited := link(t, load(t, l, repo, ""), "sub"); gate(t, edited, "5a00") != "defined" {
		t.Errorf("an uncommitted edit in the checkout shows: %+v", edited.Project.Statuses["5a00"])
	}
}

// T18: a directory reads at the same commit, or from the same working tree.
func TestDirectoryOfTheSameRepository(t *testing.T) {
	l := loader(t, Options{})
	repo := clone(t, "subproject-same-repository")
	for _, ref := range []string{"HEAD", ""} {
		p := load(t, l, repo, ref)
		lib := link(t, p, "lib")
		if lib.Form != model.Directory || lib.Problem != model.Resolved || lib.Commit != "" || lib.Checkout != "" || lib.Project.Tasks["5a00"] == nil {
			t.Fatalf("at %q: %+v", ref, lib)
		}
		where := p.Where
		where.Dir = "lib"
		if ref == "HEAD" {
			// A linked project carries the commit as its revision and no working tree.
			where.Ref, where.Top = p.Where.Commit, ""
		}
		if lib.Project.Where != where {
			t.Errorf("at %q: the directory is read at %+v, want %+v", ref, lib.Project.Where, where)
		}
	}
	write(t, repo, "lib/.tableaux/tasks/5a00.yaml", "title: Edited and not committed\n")
	write(t, repo, "lib/.tableaux/tasks/5a01.yaml", "title: Untracked\n")
	tree := link(t, load(t, l, repo, ""), "lib").Project
	head := link(t, load(t, l, repo, "HEAD"), "lib").Project
	if tree.Tasks["5a00"].Title.V != "Edited and not committed" || tree.Tasks["5a01"] == nil {
		t.Errorf("the working tree: %q, tasks %v", tree.Tasks["5a00"].Title.V, tree.TaskIDs())
	}
	if head.Tasks["5a00"].Title.V == "Edited and not committed" || head.Tasks["5a01"] != nil {
		t.Errorf("HEAD: %q, tasks %v", head.Tasks["5a00"].Title.V, head.TaskIDs())
	}
}

// T19: a URL reads through the mapping at its commit.
func TestURLThroughTheMapping(t *testing.T) {
	const url = "https://example.org/lib.git"
	lib := entry(t, "subproject-by-url.lib")
	pin := labels(t, "subproject-by-url.lib")["L2"]
	check := func(name string, p *model.Project) {
		t.Helper()
		got := link(t, p, url)
		// The library's trunk stands at L3, complete at release; the file names L2.
		if got.Form != model.URL || got.Commit != pin || got.Checkout != "" || gate(t, got, "5a00") != "design" {
			t.Errorf("%s: %+v", name, got)
		}
		if where := got.Project.Where; where.Commit != pin || where.GitDir != filepath.Join(lib, ".git") || where.Worktree {
			t.Errorf("%s: the project is read at %+v", name, where)
		}
	}
	replace := loader(t, Options{Replace: map[string]string{url: lib}})
	for _, ref := range []string{"HEAD", ""} {
		check("Replace at "+ref, load(t, replace, entry(t, "subproject-by-url"), ref))
	}
	// A bare clone serves as well.
	bare := filepath.Join(t.TempDir(), "lib.git")
	gitRun(t, lib, "clone", "-q", "--bare", lib, bare)
	if got := link(t, load(t, loader(t, Options{Replace: map[string]string{url: bare}}), entry(t, "subproject-by-url"), "HEAD"), url); got.Commit != pin || gate(t, got, "5a00") != "design" {
		t.Errorf("Replace to a bare clone: %+v", got)
	}

	// The same through Git's configuration, under tableaux.<url>.path.
	repo := clone(t, "subproject-by-url")
	gitRun(t, repo, "config", "tableaux."+url+".path", lib)
	for _, ref := range []string{"HEAD", ""} {
		check("the configuration at "+ref, load(t, loader(t, Options{}), repo, ref))
	}

	// Replace wins over the configuration, in both directions.
	nowhere := filepath.Join(t.TempDir(), "nowhere")
	if got := link(t, load(t, loader(t, Options{Replace: map[string]string{url: nowhere}}), repo, "HEAD"), url); got.Problem != model.NoClone || got.Project != nil {
		t.Errorf("Replace to nowhere over a good configuration: %+v", got)
	}
	gitRun(t, repo, "config", "tableaux."+url+".path", nowhere)
	check("Replace over a configuration to nowhere", load(t, replace, repo, "HEAD"))
	if got := link(t, load(t, loader(t, Options{}), repo, "HEAD"), url); got.Problem != model.NoClone {
		t.Errorf("a configuration to nowhere: %+v", got)
	}
}

// T20: each problem has its value.
func TestLinkProblems(t *testing.T) {
	const url = "https://example.org/lib.git"
	l := loader(t, Options{})
	problem := func(name string, got *model.Link, form model.Form, want model.Problem) {
		t.Helper()
		if got.Form != form || got.Problem != want || (got.Project != nil) != (want == model.Resolved) || (got.Detail == "") != (want == model.Resolved) {
			t.Errorf("%s: %s, %s, detail %q, project %v; want %s, %s", name, got.Form, got.Problem, got.Detail, got.Project != nil, form, want)
		}
	}
	for _, ref := range []string{"HEAD", ""} {
		problem("subproject-path-missing", link(t, load(t, l, entry(t, "subproject-path-missing"), ref), "elsewhere"), model.NoForm, model.Missing)
		problem("subproject-url-without-commit", link(t, load(t, l, entry(t, "subproject-url-without-commit"), ref), url), model.URL, model.BadCommit)
		// A commit stated on a path never changes what the Loader reads there.
		malformed := link(t, load(t, l, entry(t, "subproject-commit-malformed"), ref), "sub")
		problem("subproject-commit-malformed", malformed, model.Submodule, model.Resolved)
		if malformed.Commit != labels(t, "subproject-commit-malformed.sub")["S1"] {
			t.Errorf("subproject-commit-malformed: read at %s", malformed.Commit)
		}
		offPin := link(t, load(t, l, entry(t, "subproject-commit-off-pin"), ref), "sub")
		problem("subproject-commit-off-pin", offPin, model.Submodule, model.Resolved)
		withCommit := link(t, load(t, l, entry(t, "subproject-directory-with-commit"), ref), "lib")
		problem("subproject-directory-with-commit", withCommit, model.Directory, model.Resolved)
		if withCommit.Commit != "" {
			t.Errorf("subproject-directory-with-commit: read at %q", withCommit.Commit)
		}
		// No mapping and an empty cache.
		problem("subproject-by-url with no mapping", link(t, load(t, l, entry(t, "subproject-by-url"), ref), url), model.URL, model.NoClone)
		// A clone that lacks the commit, and a mapping to what is no repository.
		other := loader(t, Options{Replace: map[string]string{url: entry(t, "weather-station")}})
		problem("a clone without the commit", link(t, load(t, other, entry(t, "subproject-by-url"), ref), url), model.URL, model.CommitAbsent)
		none := loader(t, Options{Replace: map[string]string{url: t.TempDir()}})
		problem("a mapping to no repository", link(t, load(t, none, entry(t, "subproject-by-url"), ref), url), model.URL, model.NoClone)
	}

	// The forms a corpus entry does not hold, in one project.
	repo := repository(t, "problems")
	plan(t, repo, "", "")
	write(t, repo, ".tableaux/tasks/a000.yaml", `title: Root
description: A project whose links each lead nowhere.
assignee: olive@example.org
requires:
  - { subproject: { url: ../x, id: "b000" } }
  - { subproject: { url: /abs, id: "b000" } }
  - { subproject: { url: "", id: "b000" } }
  - { subproject: { url: lib/../.., id: "b000" } }
  - { subproject: { url: empty, id: "b000" } }
  - { subproject: { url: README.md, id: "b000" } }
  - { subproject: { url: "https://example.org/short.git", id: "b000", commit: abc123 } }
  - { subproject: { url: "https://example.org/upper.git", id: "b000", commit: 438D39DE8534B02F59EE7A195F5E3AFBFF5C7FA2 } }
  - { subproject: { url: "https://example.org/list.git", id: "b000", commit: [a] } }
  - { subproject: { url: [lib], id: "b000" } }
  - { subproject: { id: "b000" } }
  - { subproject: lib }
`)
	write(t, repo, "empty/keep", "A directory with no plan.\n")
	write(t, repo, "README.md", "A file.\n")
	commit(t, repo, "Plan the problems")
	for _, ref := range []string{"HEAD", ""} {
		p := load(t, l, repo, ref)
		for _, url := range []string{"../x", "/abs", "", "lib/../.."} {
			problem(fmt.Sprintf("url %q", url), link(t, p, url), model.NoForm, model.Outside)
		}
		problem("a directory with no .tableaux", link(t, p, "empty"), model.Directory, model.NoProject)
		problem("a path that is a file", link(t, p, "README.md"), model.NoForm, model.Missing)
		for _, name := range []string{"short", "upper", "list"} {
			problem("a commit that is no full hash: "+name, link(t, p, "https://example.org/"+name+".git"), model.URL, model.BadCommit)
		}
		// A url that is no scalar, and a subproject with none, have no link.
		requires := p.Tasks["a000"].Requires
		if len(p.Links) != 9 || len(requires) != 12 || requires[9].Subproject.Link != nil || requires[10].Subproject.Link != nil || requires[11].Subproject.Link != nil {
			t.Errorf("at %q: %d links for %d requirements", ref, len(p.Links), len(requires))
		}
		for i, r := range requires[:9] {
			if r.Subproject.Link == nil {
				t.Errorf("at %q: requirement %d has no link", ref, i)
			}
		}
	}

	// A chain of nine links: the ninth is too deep.
	chain := repository(t, "chain")
	plan(t, chain, "", "{ subproject: { url: d1 } }")
	for i := 1; i <= 9; i++ {
		junction := fmt.Sprintf("{ subproject: { url: d%d } }", i+1)
		if i == 9 {
			junction = ""
		}
		plan(t, chain, fmt.Sprintf("d%d", i), junction)
	}
	commit(t, chain, "Plan the chain")
	fromRoot := func(ref string) {
		t.Helper()
		p := load(t, l, chain, ref)
		for i := 1; i <= 8; i++ {
			next := link(t, p, fmt.Sprintf("d%d", i))
			problem(fmt.Sprintf("at %q, link %d", ref, i), next, model.Directory, model.Resolved)
			if next.Project == nil {
				return
			}
			p = next.Project
		}
		problem(fmt.Sprintf("at %q, the ninth link", ref), link(t, p, "d9"), model.NoForm, model.TooDeep)
	}
	fromRoot("HEAD")
	fromRoot("")
	// From d1 the same project d9 is eight links away and resolves.
	for _, ref := range []string{"HEAD", ""} {
		p := load(t, l, filepath.Join(chain, "d1"), ref)
		for i := 2; i <= 9; i++ {
			next := link(t, p, fmt.Sprintf("d%d", i))
			problem(fmt.Sprintf("from d1 at %q, link %d", ref, i), next, model.Directory, model.Resolved)
			if next.Project == nil {
				break
			}
			p = next.Project
		}
	}
	// The memo now holds d2 with all it reaches, seven links deep. From the
	// root d2 stands two links away, so the Loader reads it anew, and the
	// ninth link stays too deep whatever the memo holds.
	fromRoot("HEAD")
}

// T21: links close and share.
func TestLinksCloseAndShare(t *testing.T) {
	const url = "https://example.org/umbrella.git"
	l := loader(t, Options{Replace: map[string]string{url: entry(t, "requires-across-projects.umbrella")}})
	for _, ref := range []string{"HEAD", ""} {
		p := load(t, l, entry(t, "requires-across-projects"), ref)
		if len(p.Links) != 2 || p.Links[0].URL != url || p.Links[1].URL != "sub" {
			t.Fatalf("at %q: the links are not by URL: %+v", ref, p.Links)
		}
		upward, downward := p.Links[0], p.Links[1]
		if upward.Form != model.URL || upward.Problem != model.Resolved || upward.Commit != labels(t, "requires-across-projects.umbrella")["M2"] || upward.Project.Tasks["9e10"] == nil {
			t.Errorf("at %q: the requirement by URL: %+v", ref, upward)
		}
		if downward.Form != model.Submodule || downward.Problem != model.Resolved || downward.Commit != labels(t, "requires-across-projects.sub")["S1"] || downward.Project.Tasks["5a00"] == nil {
			t.Errorf("at %q: the requirement on sub: %+v", ref, downward)
		}
		if p.Tasks["b2c9"].Requires[0].Subproject.Link != upward || p.Tasks["c3d7"].Requires[0].Subproject.Link != downward {
			t.Errorf("at %q: the requirements do not hold the project's links", ref)
		}
	}

	// Two directories that name each other hold each other's pointer, and two
	// spellings of one path share one link.
	repo := repository(t, "pair")
	plan(t, repo, "a", "{ subproject: { url: b } }")
	plan(t, repo, "b", "{ subproject: { url: ./a/ } }")
	write(t, repo, "b/.tableaux/tasks/a001.yaml", "title: Second\nrequires:\n  - { subproject: { url: a, id: a000, commit: abc123 } }\n  - { subproject: { url: ., id: a000 } }\n  - { subproject: { url: b/.., id: a000 } }\n")
	plan(t, repo, "", "{ subproject: { url: . } }")
	plan(t, repo, "c", "{ subproject: { url: a } }")
	commit(t, repo, "Plan the pair")
	// Read from c first, the memo holds a and b, each with the other. A load
	// from a then reads b anew, so that b holds the pointer of the a it returns.
	if third := load(t, l, filepath.Join(repo, "c"), "HEAD"); link(t, third, "a").Project == nil {
		t.Fatalf("from c: %+v", third.Links)
	}
	for _, ref := range []string{"HEAD", "", "HEAD"} {
		a := load(t, l, filepath.Join(repo, "a"), ref)
		b := link(t, a, "b").Project
		if b == nil || a.Where.Dir != "a" || b.Where.Dir != "b" {
			t.Fatalf("at %q: a links to %+v", ref, a.Links)
		}
		if len(b.Links) != 2 || b.Links[0].URL != "." || b.Links[1].URL != "a" {
			t.Fatalf("at %q: b has the links %+v", ref, b.Links)
		}
		if back := b.Links[1]; back.Project != a || b.Tasks["a000"].Junctions[0].Subproject.Link != back || b.Tasks["a001"].Requires[0].Subproject.Link != back {
			t.Errorf("at %q: b does not hold a's pointer through one link", ref)
		}
		root := b.Links[0]
		if root.Form != model.Directory || root.Project == nil || root.Project.Where.Dir != "" ||
			b.Tasks["a001"].Requires[1].Subproject.Link != root || b.Tasks["a001"].Requires[2].Subproject.Link != root {
			t.Errorf("at %q: the root project by two spellings: %+v", ref, root)
		}
		// The root names itself.
		if root.Project != nil && (len(root.Project.Links) != 1 || root.Project.Links[0].Project != root.Project) {
			t.Errorf("at %q: the root does not hold its own pointer", ref)
		}
	}

	// One absolute URL at two commits is two links, by commit.
	two := repository(t, "two")
	hashes := labels(t, "requires-across-projects.umbrella")
	plan(t, two, "", "")
	write(t, two, ".tableaux/tasks/a000.yaml", "title: Root\nrequires:\n"+
		"  - { subproject: { url: "+url+", id: \"9e10\", commit: "+max(hashes["M1"], hashes["M2"])+" } }\n"+
		"  - { subproject: { url: "+url+", id: \"9e10\", commit: "+min(hashes["M1"], hashes["M2"])+" } }\n"+
		"  - { subproject: { url: "+url+", id: \"9e00\", commit: "+max(hashes["M1"], hashes["M2"])+" } }\n")
	p := load(t, l, two, "")
	requires := p.Tasks["a000"].Requires
	if len(p.Links) != 2 || p.Links[0].Commit >= p.Links[1].Commit || p.Links[0].Project == p.Links[1].Project ||
		requires[0].Subproject.Link != p.Links[1] || requires[1].Subproject.Link != p.Links[0] || requires[2].Subproject.Link != p.Links[1] {
		t.Errorf("one URL at two commits: %+v", p.Links)
	}
	if !strings.HasPrefix(p.Links[0].URL, "https://") || p.Links[0].Problem != model.Resolved || p.Links[1].Problem != model.Resolved {
		t.Errorf("one URL at two commits: %+v, %+v", p.Links[0], p.Links[1])
	}
}

// T22: another repository reads as itself, under the variables a hook inherits.
func TestHookEnvironment(t *testing.T) {
	repo := clone(t, "submodule-subproject")
	sub := labels(t, "submodule-subproject.sub")
	gitRun(t, filepath.Join(repo, "sub"), "checkout", "-q", sub["S1"])
	t.Setenv("GIT_DIR", filepath.Join(repo, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(repo, ".git", "index"))
	t.Setenv("GIT_PREFIX", "")
	l := loader(t, Options{})
	// Without care, git reads the parent's HEAD where the Loader asks for the submodule's.
	main := link(t, load(t, l, repo, "main"), "sub")
	if main.Form != model.Submodule || main.Commit != sub["S3"] || gate(t, main, "5a00") != "function" {
		t.Errorf("at main, under a hook's variables: %+v", main)
	}
	if where := main.Project.Where; where.GitDir != filepath.Join(repo, ".git", "modules", "sub") {
		t.Errorf("the subproject is read in %s", where.GitDir)
	}
	tree := link(t, load(t, l, repo, ""), "sub")
	if tree.Commit != sub["S1"] || gate(t, tree, "5a00") != "defined" {
		t.Errorf("the working tree, under a hook's variables: %+v", tree)
	}
}
