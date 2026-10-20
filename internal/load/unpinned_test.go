package load

import (
	"path/filepath"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// unpinnedOf returns the unpinned link of p and the pinned one to the same
// URL, nil where p has none.
func unpinnedOf(p *model.Project, url string) (tip, pinned *model.Link) {
	for _, l := range p.Links {
		switch {
		case l.URL != url:
		case l.Unpinned:
			tip = l
		default:
			pinned = l
		}
	}
	return tip, pinned
}

// requiring writes a parent project whose root requires a000 of the project
// at url with no commit, and, when pin is not "", once more at that commit.
func requiring(t testing.TB, url, pin string) string {
	t.Helper()
	parent := repository(t, "parent")
	plan(t, parent, "", "")
	task := "title: Root\ndescription: The root of a test project.\nassignee: olive@example.org\n" +
		"requires:\n  - { subproject: { url: \"" + url + "\", id: \"a000\" } }\n"
	if pin != "" {
		task += "  - { subproject: { url: \"" + url + "\", id: \"a000\", commit: " + pin + " } }\n"
	}
	write(t, parent, ".tableaux/tasks/a000.yaml", task)
	commit(t, parent, "Require the library")
	return parent
}

// T5 of design cdb0, on the corpus: requires-unpinned reads the library at
// the tip of main, L2, through Replace.
func TestUnpinnedOnTheCorpus(t *testing.T) {
	const url = "https://example.org/lib.git"
	l := loader(t, Options{Replace: map[string]string{url: entry(t, "requires-unpinned.lib")}})
	p := load(t, l, entry(t, "requires-unpinned"), "HEAD")
	tip, pinned := unpinnedOf(p, url)
	if tip == nil || pinned != nil || tip.Form != model.URL || tip.Trunk != "main" || tip.Commit != labels(t, "requires-unpinned.lib")["L2"] ||
		tip.Project == nil || gate(t, tip, "5a00") != "design" {
		t.Errorf("the unpinned link: %+v", tip)
	}
	if field := p.Tasks["b2c9"].Requires[0].Subproject; !field.Unpinned() || field.Link != tip {
		t.Errorf("the field is unpinned %v, with the link %+v", field.Unpinned(), field.Link)
	}
}

// T5 of design cdb0, built: the tip of a clone's trunk, origin's first; a
// trunk no ref carries; no clone; FetchTip over a file:// URL; and a second
// load that reads the tip the other trunk moved to, while a pinned link of
// the same load stays in the memo.
func TestUnpinnedRead(t *testing.T) {
	lib := repository(t, "lib")
	plan(t, lib, "", "")
	first := commit(t, lib, "Plan the library")
	const url = "https://example.org/lib.git"

	// A clone whose origin/main is ahead of its main reads origin/main.
	mirror := filepath.Join(t.TempDir(), "mirror")
	gitRun(t, t.TempDir(), "clone", "-q", lib, mirror)
	second := commit(t, lib, "Advance the library")
	gitRun(t, mirror, "fetch", "-q", "origin")
	parent := requiring(t, url, "")
	tip, _ := unpinnedOf(load(t, loader(t, Options{Replace: map[string]string{url: mirror}}), parent, "HEAD"), url)
	if tip == nil || tip.Problem != model.Resolved || tip.Trunk != "main" || tip.Commit != second || tip.Project.Where.Commit != second {
		t.Errorf("the clone's origin/main: %+v; want %s", tip, second)
	}

	// version.yaml names a branch no ref carries: no trunk.
	other := repository(t, "other")
	plan(t, other, "", "")
	write(t, other, ".tableaux/version.yaml", "tableaux: 0.3.1\ntrunk: release\n")
	commit(t, other, "Name a trunk that does not exist")
	tip, _ = unpinnedOf(load(t, loader(t, Options{Replace: map[string]string{url: other}}), parent, "HEAD"), url)
	if tip == nil || tip.Problem != model.NoTrunk || tip.Problem.String() != "no trunk" || tip.Trunk != "release" || tip.Project != nil {
		t.Errorf("a trunk no ref carries: %+v", tip)
	}
	// version.yaml names no trunk: the default branch itself.
	write(t, other, ".tableaux/version.yaml", "tableaux: 0.3.1\n")
	head := commit(t, other, "Name no trunk")
	tip, _ = unpinnedOf(load(t, loader(t, Options{Replace: map[string]string{url: other}}), parent, "HEAD"), url)
	if tip == nil || tip.Problem != model.Resolved || tip.Trunk != "HEAD" || tip.Commit != head {
		t.Errorf("no trunk named: %+v", tip)
	}

	// No mapping and no cache: no clone.
	tip, _ = unpinnedOf(load(t, loader(t, Options{}), parent, "HEAD"), url)
	if tip == nil || tip.Problem != model.NoClone || tip.Project != nil {
		t.Errorf("no clone: %+v", tip)
	}

	// FetchTip over a file:// URL clones the library and reads its tip; a
	// second load of the same Loader reads the tip the library moved to, and
	// the pinned link of the load is the memo's.
	fileURL := "file://" + filepath.ToSlash(lib)
	parent = requiring(t, fileURL, first)
	l := loader(t, Options{Fetch: true})
	tip, pinned := unpinnedOf(load(t, l, parent, "HEAD"), fileURL)
	if tip == nil || tip.Commit != second || tip.Trunk != "main" || pinned == nil || pinned.Commit != first || pinned.Project == nil {
		t.Fatalf("FetchTip: the tip %+v, the pinned link %+v", tip, pinned)
	}
	third := commit(t, lib, "Advance the library again")
	again, pinnedAgain := unpinnedOf(load(t, l, parent, "HEAD"), fileURL)
	if again == nil || again.Commit != third || again.Project == tip.Project {
		t.Errorf("the second load reads the tip %+v; want %s", again, third)
	}
	if pinnedAgain == nil || pinnedAgain.Project != pinned.Project {
		t.Error("the pinned link of the second load is not the memo's project")
	}
}
