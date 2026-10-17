package tablo

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// copyTree copies a directory with its symbolic links and file modes, for a
// test that reads a corpus repository's working tree.
func copyTree(t testing.TB, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, p)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		info, err := d.Info()
		if err != nil {
			return err
		}
		switch {
		case d.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode()&fs.ModeSymlink != 0:
			link, err := os.Readlink(p)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm()|0o200)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// linked returns the snapshot a url links to, and fails the test on an error.
func linked(t testing.TB, s *Snapshot, url, commit string) *Snapshot {
	t.Helper()
	sub, err := s.Linked(url, commit)
	if err != nil {
		t.Fatalf("Linked(%q, %q): %v", url, commit, err)
	}
	return sub
}

// sameTask checks that the task view of a linked snapshot equals that of a
// direct load of the linked project, as `tablo -C <dir> --ref <pin> task`
// gives it, with the source aside.
func sameTask(t testing.TB, sub *Snapshot, dir, ref string, opt Options, ids ...string) {
	t.Helper()
	direct := loadAt(t, dir, ref, opt)
	for _, id := range ids {
		for _, level := range []Level{LevelGlance, LevelProvenance} {
			p := Params{Task: id, Level: level}
			a, err := sub.View(context.Background(), Task, p)
			if err != nil {
				t.Fatalf("the linked snapshot's task %s: %v", id, err)
			}
			b, err := direct.View(context.Background(), Task, p)
			if err != nil {
				t.Fatalf("the direct load's task %s: %v", id, err)
			}
			if a.Data == nil || b.Data == nil {
				t.Fatalf("task %s gives no data", id)
			}
			a.Source, b.Source = nil, nil
			if !bytes.Equal(a.JSON(), b.JSON()) {
				t.Errorf("task %s at %s differs from the direct load of %s at %s:\n%s\n---\n%s", id, level, dir, ref, a.JSON(), b.JSON())
			}
		}
	}
}

// TestLinkedSubmodule reads weather-station's firmware, a submodule linked
// by path, at a ref and in the working tree: the linked snapshot names the
// pin, its date and its directory, and its views equal those of a load of
// the checkout at the pin.
func TestLinkedSubmodule(t *testing.T) {
	station := filepath.Join(corpus(t), "weather-station")
	s := loadAt(t, station, "main", Options{})
	pin := ""
	for _, l := range s.project.Links {
		if l.URL == "firmware" {
			pin = l.Commit
		}
	}
	if len(pin) != 40 {
		t.Fatalf("weather-station at main pins firmware at %q", pin)
	}
	sub := linked(t, s, "firmware", "")
	src := sub.Source()
	if src.Commit == nil || *src.Commit != pin || src.Ref != pin || src.Dir != "." || src.Worktree || src.Date == nil {
		t.Errorf("the linked source is %+v; want the pin %s, its date and the directory .", src, pin)
	}
	if sub.Project() == nil || sub.Project().Root != "b7d2" {
		t.Errorf("the linked project is %+v; want the root b7d2", sub.Project())
	}
	if again := linked(t, s, "./firmware/", ""); again != sub {
		t.Errorf("./firmware/ names another snapshot than firmware")
	}
	if got := sub.Checkouts(); got == nil || len(got) != 0 {
		t.Errorf("Checkouts of the linked snapshot at a ref = %#v; want an empty list", got)
	}
	sameTask(t, sub, filepath.Join(station, "firmware"), pin, Options{}, "f1a0", "b7d2")

	// The working tree reads the submodule at its checkout's HEAD.
	copied := filepath.Join(t.TempDir(), "weather-station")
	copyTree(t, station, copied)
	w := loadAt(t, copied, "", Options{})
	checkout := filepath.Join(copied, "firmware")
	if got := w.Checkouts(); !reflect.DeepEqual(got, []string{checkout}) {
		t.Fatalf("Checkouts of the working tree = %v; want the firmware checkout", got)
	}
	head, err := loadAt(t, checkout, "", Options{}).repo.Resolve(context.Background(), "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	ws := linked(t, w, "firmware", "")
	if src := ws.Source(); src.Commit == nil || *src.Commit != head || src.Ref != head || src.Worktree {
		t.Errorf("the linked source of the working tree is %+v; want the checkout's HEAD %s", src, head)
	}
	if got := ws.Checkouts(); got == nil || len(got) != 0 {
		t.Errorf("Checkouts of the linked snapshot of the working tree = %#v; want an empty list", got)
	}
	sameTask(t, ws, checkout, head, Options{}, "f1a0", "b7d2")
}

// TestLinkedByURL reads subproject-by-url's library through Options.Replace,
// at the commit the junction's field names.
func TestLinkedByURL(t *testing.T) {
	build := corpus(t)
	const url = "https://example.org/lib.git"
	lib := filepath.Join(build, "subproject-by-url.lib")
	s := loadAt(t, filepath.Join(build, "subproject-by-url"), "main", Options{Replace: map[string]string{url: lib}})
	l2 := "438d39de8534b02f59ee7a195f5e3afbff5c7fa2"
	sub := linked(t, s, url, "")
	if src := sub.Source(); src.Commit == nil || *src.Commit != l2 || src.Ref != l2 || src.Dir != "." {
		t.Errorf("the linked source is %+v; want L2, %s", src, l2)
	}
	if again := linked(t, s, url, l2); again != sub {
		t.Errorf("the url with its commit names another snapshot")
	}
	sameTask(t, sub, lib, l2, Options{}, "5a00")
	var nf *NotFoundError
	if _, err := s.Linked(url, "68d9021206ec627755e2ead99b6be3f2c06ae815"); !errors.As(err, &nf) || nf.Kind != "subproject" {
		t.Errorf("the url at a commit the files do not link gives %v; want a *NotFoundError", err)
	}
}

// TestLinkedFails covers a link that leads to no project, a url that names
// no link, and an absolute URL linked at two commits.
func TestLinkedFails(t *testing.T) {
	missing := loadAt(t, filepath.Join(corpus(t), "subproject-path-missing"), "main", Options{})
	var le *LinkError
	if _, err := missing.Linked("elsewhere", ""); !errors.As(err, &le) || le.Problem != "missing" || le.URL != "elsewhere" {
		t.Errorf("a path that leads nowhere gives %v; want a *LinkError, missing", err)
	}
	var nf *NotFoundError
	if _, err := missing.Linked("nowhere", ""); !errors.As(err, &nf) || nf.Kind != "subproject" || nf.Name != "nowhere" {
		t.Errorf("a url that names no link gives %v; want a *NotFoundError of kind subproject", err)
	}

	// One URL at two commits needs the commit.
	lib := build(t)
	lib.put(map[string]string{"version.yaml": baseVersion, "gates.yaml": baseGates, "tasks/f1a0.yaml": baseRoot})
	first := lib.commit("ben", "2026-09-01", "Plan the library")
	lib.put(map[string]string{"tasks/f1a0.yaml": baseRoot + "description: The library, described.\n"})
	second := lib.commit("ben", "2026-09-02", "Describe the library")
	const url = "https://example.org/lib.git"
	b := small(t)
	b.put(map[string]string{
		"tasks/b2c9.yaml": baseLeaf + "requires:\n  - { subproject: { url: " + url + ", id: \"f1a0\", commit: " + first + " } }\n",
		"tasks/c3d7.yaml": baseSecond + "requires:\n  - { subproject: { url: " + url + ", id: \"f1a0\", commit: " + second + " } }\n",
	})
	b.commit("olive", "2026-09-04", "Require the library twice")
	s := loadAt(t, b.dir, "main", Options{Replace: map[string]string{url: lib.dir}})
	var u *UsageError
	if _, err := s.Linked(url, ""); !errors.As(err, &u) {
		t.Errorf("a URL linked at two commits gives %v; want a *UsageError", err)
	}
	for _, commit := range []string{first, second} {
		if src := linked(t, s, url, commit).Source(); src.Commit == nil || *src.Commit != commit {
			t.Errorf("the URL at %s gives the source %+v", commit, src)
		}
	}
}

// TestLinkedRange covers a range on a linked snapshot's history whose start
// the pass lacks: the snapshot reads the linked repository once more and
// resolves the start there, not in the parent's repository.
func TestLinkedRange(t *testing.T) {
	lib := build(t)
	lib.put(map[string]string{"version.yaml": baseVersion, "gates.yaml": baseGates, "tasks/f1a0.yaml": baseRoot})
	first := lib.commit("ben", "2026-09-01", "Plan the library")
	lib.git("ben", "2026-09-02", "checkout", "-q", "-b", "side")
	lib.put(map[string]string{"tasks/f1a0.yaml": baseRoot + "description: The library, on a side branch.\n"})
	side := lib.commit("ben", "2026-09-02", "Describe the library aside")
	lib.git("ben", "2026-09-02", "checkout", "-q", "main")
	const url = "https://example.org/lib.git"
	b := small(t)
	b.put(map[string]string{
		"tasks/b2c9.yaml": baseLeaf + "requires:\n  - { subproject: { url: " + url + ", id: \"f1a0\", commit: " + first + " } }\n",
	})
	b.commit("olive", "2026-09-04", "Require the library")
	s := loadAt(t, b.dir, "main", Options{Replace: map[string]string{url: lib.dir}})
	sub := linked(t, s, url, "")
	if sub.facts.Log().Commit(side) != nil {
		t.Fatalf("the pass of the library holds the side branch")
	}
	e, err := sub.View(context.Background(), History, Params{From: "side"})
	if err != nil {
		t.Fatalf("history from side: %v", err)
	}
	if e.Source.From == nil || e.Source.From.Commit != side {
		t.Errorf("history from side: source.from %+v; want %s", e.Source.From, side)
	}
	if len(sub.ranges) != 1 || len(s.ranges) != 0 {
		t.Errorf("the linked snapshot reads %d further passes and the parent %d; want one and none", len(sub.ranges), len(s.ranges))
	}
}

// TestLinkedDirectory reads subproject-same-repository's lib, a directory of
// the same repository, at the parent's own source.
func TestLinkedDirectory(t *testing.T) {
	repo := filepath.Join(corpus(t), "subproject-same-repository")
	s := loadAt(t, repo, "main", Options{})
	sub := linked(t, s, "lib", "")
	src, parent := sub.Source(), s.Source()
	if src.Dir != "lib" || src.Ref != "main" || src.Commit == nil || parent.Commit == nil || *src.Commit != *parent.Commit {
		t.Errorf("the linked source is %+v; want lib at main's commit", src)
	}
	sameTask(t, sub, filepath.Join(repo, "lib"), "main", Options{}, sub.Project().Root)
}
