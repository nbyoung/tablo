package history

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// T2: the pass marks the source and the trunk. W3 sits on the branch
// sensor-board, which the merge W4 brings into main.
func TestSourceAndTrunk(t *testing.T) {
	dir, hashes := entry(t, "weather-station"), labels(t, "weather-station")
	r := NewReader(Options{})

	branch := pass(t, r, loaded(t, load.Options{}, dir, "sensor-board"), "")
	if branch.Source != hashes["W3"] || branch.Trunk.Tip != hashes["W13"] {
		t.Fatalf("at sensor-board the source is %s and the trunk's tip %s", branch.Source, branch.Trunk.Tip)
	}
	if c := branch.Commit(hashes["W3"]); c == nil || !c.InSource || c.Trunk != -1 {
		t.Errorf("at sensor-board W3 reads as %+v; want it in the source and off the line", c)
	}
	if c := branch.Commit(hashes["W4"]); c == nil || c.InSource || c.Trunk != 9 {
		t.Errorf("at sensor-board W4 reads as %+v; want it out of the source, ninth from the tip", c)
	}
	if c := branch.Commit(hashes["W1"]); c == nil || !c.InSource || c.Trunk != 11 || c.Seq != len(branch.Commits)-1 {
		t.Errorf("at sensor-board W1 reads as %+v; want it in the source, last on the line", c)
	}

	trunk := pass(t, r, loaded(t, load.Options{}, dir, "main"), "")
	if len(trunk.Commits) != 13 || trunk.Commits[0].ID != hashes["W13"] || trunk.Commits[0].Trunk != 0 {
		t.Fatalf("at main the pass holds %d commits from %+v", len(trunk.Commits), trunk.Commits[0])
	}
	if c := trunk.Commit(hashes["W3"]); !c.InSource || c.Trunk != -1 {
		t.Errorf("at main W3 reads as %+v; want it in the source and off the line", c)
	}
	if c := trunk.Commit(hashes["W4"]); !c.InSource || c.Trunk != 9 || !c.Merge() {
		t.Errorf("at main W4 reads as %+v; want a merge in the source and on the line", c)
	}
	for i, c := range trunk.Commits {
		if c.Seq != i || trunk.Commit(c.ID) != c {
			t.Errorf("commit %d has the place %d", i, c.Seq)
		}
	}
	if !trunk.Reaches(hashes["W13"], hashes["W3"]) || !trunk.Reaches(hashes["W4"], hashes["W4"]) ||
		trunk.Reaches(hashes["W5"], hashes["W6"]) || trunk.Reaches(hashes["W2"], hashes["W13"]) || trunk.Reaches("none", hashes["W1"]) {
		t.Error("Reaches disagrees with the weather station's history")
	}

	// A commit keeps what a view shows of it.
	w11 := trunk.Commit(hashes["W11"])
	if w11.Author != (Person{"Ben Okafor", "ben@example.org"}) || w11.Committer != w11.Author || w11.Date() != "2026-09-26" ||
		!reflect.DeepEqual(w11.Values("Reviewed"), []string{"c07d mockup"}) || w11.Values("Model") != nil {
		t.Errorf("W11 reads as %+v", w11)
	}
	want := []Change{
		{Path: ".tableaux/status/c07d.yaml", New: trunk.Object(hashes["W11"], ".tableaux/status/c07d.yaml")},
		{Path: "firmware", New: labels(t, "weather-station.firmware")["F2"], Gitlink: true},
	}
	if !reflect.DeepEqual(w11.Changes, want) {
		t.Errorf("W11 changes %+v; want %+v", w11.Changes, want)
	}
}

func TestDateInTheAuthorsZone(t *testing.T) {
	c := &Commit{Time: 1788292800, Zone: 330} // 2026-09-01T20:00:00Z
	if got := c.Date(); got != "2026-09-02" {
		t.Errorf("Date east of UTC is %s; want 2026-09-02", got)
	}
	c.Zone = -600
	if got := c.Date(); got != "2026-09-01" {
		t.Errorf("Date west of UTC is %s; want 2026-09-01", got)
	}
}

// T3: the trunk resolves in README.md's order.
func TestTrunkOrder(t *testing.T) {
	r := NewReader(Options{})
	stated := pass(t, r, loaded(t, load.Options{}, entry(t, "trunk-stated"), "develop"), "main")
	if want := (Trunk{Name: "develop", From: Stated, Ref: "refs/heads/develop", Tip: labels(t, "trunk-stated")["T2"]}); stated.Trunk != want || stated.Trunk.How() != "stated" {
		t.Errorf("trunk-stated resolves %+v, %s; want %+v", stated.Trunk, stated.Trunk.How(), want)
	}
	inferred := pass(t, r, loaded(t, load.Options{}, entry(t, "trunk-inferred"), "main"), "other")
	if tr := inferred.Trunk; tr.Name != "main" || tr.From != Inferred || tr.Ref != "refs/heads/main" || tr.How() != "inferred" {
		t.Errorf("trunk-inferred resolves %+v, %s", tr, tr.How())
	}
	none := loaded(t, load.Options{}, entry(t, "trunk-undetermined"), "main")
	if tr := pass(t, r, none, "").Trunk; tr != (Trunk{}) || tr.How() != "undetermined" {
		t.Errorf("trunk-undetermined resolves %+v, %s", tr, tr.How())
	}
	if tr := pass(t, r, none, "main").Trunk; tr.Name != "main" || tr.From != Caller || tr.Tip != none.Where.Commit || tr.How() != "caller" {
		t.Errorf("trunk-undetermined with a caller's branch resolves %+v, %s", tr, tr.How())
	}
}

// T3, the half that needs no corpus: a stated name that no ref carries, and
// a name that a remote-tracking branch alone carries.
func TestTrunkRefs(t *testing.T) {
	repo := t.TempDir()
	gitRun(t, repo, "init", "-q", "-b", "main")
	write(t, repo, project("trunk: release\n"))
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "Plan")
	head := gitRun(t, repo, "rev-parse", "HEAD")
	r := NewReader(Options{})

	p := loaded(t, load.Options{}, repo, "")
	log := pass(t, r, p, "main")
	if want := (Trunk{Name: "release", From: Stated}); log.Trunk != want || log.Trunk.How() != "undetermined" {
		t.Errorf("a name no ref carries resolves %+v, %s; want %+v, undetermined", log.Trunk, log.Trunk.How(), want)
	}
	if c := log.Commit(head); c == nil || !c.InSource || c.Trunk != -1 {
		t.Errorf("with no trunk the source reads as %+v", c)
	}

	gitRun(t, repo, "update-ref", "refs/remotes/upstream/release", head)
	gitRun(t, repo, "update-ref", "refs/remotes/backup/release", head)
	if tr := pass(t, r, p, "").Trunk; tr.Ref != "refs/remotes/backup/release" || tr.Tip != head {
		t.Errorf("two other remotes resolve %+v; want the first in byte order", tr)
	}
	gitRun(t, repo, "update-ref", "refs/remotes/origin/release", head)
	if tr := pass(t, r, p, "").Trunk; tr.Ref != "refs/remotes/origin/release" || tr.How() != "stated" {
		t.Errorf("a remote-tracking branch alone resolves %+v; want origin's", tr)
	}
	gitRun(t, repo, "branch", "release")
	if tr := pass(t, r, p, "").Trunk; tr.Ref != "refs/heads/release" {
		t.Errorf("the home repository resolves %+v; want the local branch first", tr)
	}
}

// T4: a linked repository takes the remote-tracking branch (D5). The store
// of the submodule holds a local branch where the clone left it, behind
// origin's, and the pin is a commit the local branch does not reach.
func TestLinkedTrunk(t *testing.T) {
	repo := clone(t, "submodule-subproject")
	sub := labels(t, "submodule-subproject.sub")
	store := filepath.Join(repo, ".git", "modules", "sub")
	gitRun(t, repo, "--git-dir="+store, "update-ref", "refs/heads/main", sub["S1"])
	if got := gitRun(t, repo, "--git-dir="+store, "rev-parse", "refs/remotes/origin/main"); got != sub["S4"] {
		t.Fatalf("the store's origin/main is %s; want S4", got)
	}
	p := loaded(t, load.Options{}, repo, "main")
	set, err := NewReader(Options{}).Read(context.Background(), p, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Links) != 1 || p.Links[0].Project == nil {
		t.Fatalf("the links are %+v", p.Links)
	}
	log := set.Of(p.Links[0].Project)
	if log == nil || log.Trunk.Ref != "refs/remotes/origin/main" || log.Trunk.Tip != sub["S4"] || log.Source != sub["S3"] {
		t.Fatalf("the subproject's pass is %+v; want origin's main as the trunk and the pin as the source", log)
	}
	if c := log.Commit(sub["S3"]); c == nil || c.Trunk != 1 || !c.InSource {
		t.Errorf("the pin reads as %+v; want it on the trunk, one behind the tip", c)
	}
	if c := log.Commit(sub["S4"]); c == nil || c.Trunk != 0 || c.InSource {
		t.Errorf("the tip reads as %+v; want it on the trunk and out of the source", c)
	}
	if home := set.Of(p); home.Trunk.Ref != "refs/heads/main" {
		t.Errorf("the home repository's trunk is %+v; want its local branch", home.Trunk)
	}
	if set.Of(&model.Project{}) != nil || (*Set)(nil).Of(p) != nil {
		t.Error("Of gives a pass for a project the set does not hold")
	}
}

// counter makes an executable that counts its runs and then runs git.
func counter(t *testing.T) (exe string, count func() int) {
	t.Helper()
	dir := t.TempDir()
	tally := filepath.Join(dir, "tally")
	exe = filepath.Join(dir, "git-counted")
	script := fmt.Sprintf("#!/bin/sh\nprintf x >> '%s'\nexec git \"$@\"\n", tally)
	if err := os.WriteFile(exe, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return exe, func() int {
		data, err := os.ReadFile(tally)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		if err := os.WriteFile(tally, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		return len(data)
	}
}

// T5: the memo saves processes and serves no stale pass.
func TestMemo(t *testing.T) {
	if strings.ContainsRune(t.TempDir(), '\'') {
		t.Skip("the temporary directory holds a quote")
	}
	repo := clone(t, "submodule-subproject")
	exe, count := counter(t)
	r := NewReader(Options{Git: exe})
	ctx := context.Background()
	loader := load.New(load.Options{CacheDir: t.TempDir()})
	read := func() (*model.Project, *Set) {
		t.Helper()
		p, err := loader.Load(ctx, load.Source{Dir: repo})
		if err != nil {
			t.Fatal(err)
		}
		set, err := r.Read(ctx, p, "")
		if err != nil {
			t.Fatal(err)
		}
		return p, set
	}

	count()
	p, first := read()
	if got := count(); got != 6 {
		t.Errorf("a first read of two projects runs %d processes; want 6", got)
	}
	// An edit under .tableaux moves no commit: the ref listing alone runs.
	write(t, repo, map[string]string{".tableaux/status/c100.yaml": "gate: function\n# edited\n"})
	q, second := read()
	if got := count(); got != 2 {
		t.Errorf("a reload that moves no commit runs %d processes; want 2", got)
	}
	if second.Of(q) != first.Of(p) || second.Of(q.Links[0].Project) != first.Of(p.Links[0].Project) {
		t.Error("the reload makes new passes of the same history")
	}
	// A commit moves HEAD: the home project's pass is new, the subproject's stays.
	gitRun(t, repo, "commit", "-q", "-a", "-m", "Edit the status")
	head := gitRun(t, repo, "rev-parse", "HEAD")
	moved, third := read()
	if got := count(); got != 4 {
		t.Errorf("a reload after HEAD moves runs %d processes; want 4: three for the home project and one for the subproject", got)
	}
	log := third.Of(moved)
	if log == first.Of(p) || log.Source != head || log.Commits[0].ID != head || log.Trunk.Tip != head {
		t.Errorf("after HEAD moves the pass is %+v; want a new one from %s", log, head)
	}
	if third.Of(moved.Links[0].Project) != first.Of(p.Links[0].Project) {
		t.Error("the subproject's pass is new though its pin stands")
	}
	// A further tip is another pass, and it holds the tip.
	if _, err := r.Read(ctx, moved, "", first.Of(p).Source); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 4 {
		t.Errorf("a read with a further tip runs %d processes; want 4", got)
	}
}

func TestMemoIsBounded(t *testing.T) {
	r := NewReader(Options{})
	for i := 0; i < memoSize+10; i++ {
		r.remember(key{dir: fmt.Sprint(i)}, &Log{})
	}
	if len(r.memo) != memoSize || r.order.Len() != memoSize {
		t.Errorf("the memo holds %d passes; want %d", len(r.memo), memoSize)
	}
	if r.recall(key{dir: "0"}) != nil || r.recall(key{dir: fmt.Sprint(memoSize + 9)}) == nil {
		t.Error("the memo keeps the oldest pass and drops the newest")
	}
}

// T6: the past reads. The project a pass composes at a commit is the one a
// Load reads there, file for file.
func TestThePastReads(t *testing.T) {
	dir, hashes := entry(t, "weather-station"), labels(t, "weather-station")
	head := loaded(t, load.Options{}, dir, "main")
	log := pass(t, NewReader(Options{}), head, "")
	for _, label := range []string{"W1", "W7", "W13"} {
		commit := hashes[label]
		want := loaded(t, load.Options{}, dir, commit)
		got := log.Project(commit)
		if got == nil {
			t.Fatalf("%s: the pass composes no project", label)
		}
		if got != log.Project(commit) {
			t.Errorf("%s: two calls compose two projects", label)
		}
		if got.Where != (model.Location{GitDir: want.Where.GitDir, Ref: commit, Commit: commit}) || !got.Exists || got.Links != nil {
			t.Errorf("%s: the project is at %+v with the links %v", label, got.Where, got.Links)
		}
		if !reflect.DeepEqual(got.Files, want.Files) || !reflect.DeepEqual(got.Stray, want.Stray) ||
			!reflect.DeepEqual(got.Diagnostics, want.Diagnostics) {
			t.Errorf("%s: the files, the stray paths or the diagnostics differ from a Load", label)
		}
		if !reflect.DeepEqual(got.Version, want.Version) || !reflect.DeepEqual(got.Gating, want.Gating) ||
			!reflect.DeepEqual(got.Statuses, want.Statuses) || !reflect.DeepEqual(got.TaskIDs(), want.TaskIDs()) {
			t.Errorf("%s: the version, the gating, the statuses or the task ids differ from a Load", label)
		}
		for id, task := range want.Tasks {
			if !reflect.DeepEqual(got.Tasks[id].File, task.File) || got.Tasks[id].Assignee.V != task.Assignee.V {
				t.Errorf("%s: task %s differs from a Load", label, id)
			}
		}
		for _, file := range want.Files {
			p := ".tableaux/" + file.Path
			if log.Object(commit, p) != gitRun(t, dir, "rev-parse", commit+":"+p) {
				t.Errorf("%s: Object(%s) is %s", label, p, log.Object(commit, p))
			}
		}
	}
	if log.Object(hashes["W10"], "firmware") != "" || log.Object(hashes["W11"], "firmware") != labels(t, "weather-station.firmware")["F2"] {
		t.Error("the gitlink is there before W11 or is not F2 from W11")
	}
	if log.Project("none") != nil || log.Object("none", ".tableaux/version.yaml") != "" {
		t.Error("the pass reads a commit it does not hold")
	}
	// W4 merges the branch that adds 9f31: the file is its second parent's.
	task := ".tableaux/tasks/9f31.yaml"
	if log.Changed(log.Commit(hashes["W4"]), task) || !log.Changed(log.Commit(hashes["W3"]), task) || log.Changed(log.Commit(hashes["W5"]), task) {
		t.Error("Changed disagrees on the task file of 9f31 at W3, W4 or W5")
	}
	if !log.Changed(log.Commit(hashes["W1"]), ".tableaux/version.yaml") || log.Changed(log.Commit(hashes["W1"]), task) {
		t.Error("Changed disagrees on the root commit")
	}
}

// T6, the merges: a clean merge changes no file, and a merge that writes its
// own reading does.
func TestChangedAtAMerge(t *testing.T) {
	repo, hashes := cases(t)
	log := pass(t, NewReader(Options{}), loaded(t, load.Options{}, repo, "main"), "")
	if len(log.Commits) != 17 || !log.Commit(hashes["C3"]).Merge() || !log.Commit(hashes["C15"]).Merge() {
		t.Fatalf("the cases hold %d commits; want 17 with the merges C3 and C15", len(log.Commits))
	}
	leaf, status := ".tableaux/tasks/c3d7.yaml", ".tableaux/status/b2c9.yaml"
	if log.Changed(log.Commit(hashes["C3"]), leaf) || !log.Changed(log.Commit(hashes["C2"]), leaf) {
		t.Error("the clean merge C3 changes the task file its branch brings, or C2 does not")
	}
	if got := log.Commit(hashes["C3"]).Changes; len(got) != 1 || got[0].Path != leaf || got[0].Old != "" {
		t.Errorf("C3 changes %+v against its first parent; want the task file, new", got)
	}
	if !log.Changed(log.Commit(hashes["C15"]), status) || !log.Changed(log.Commit(hashes["S1"]), status) {
		t.Error("the merge C15 writes its own reading of the status and Changed misses it")
	}
	if note := log.Project(hashes["C15"]).Statuses["b2c9"].Note.V; note != "From both" {
		t.Errorf("at C15 the note is %q; want From both", note)
	}
	if note := log.Project(hashes["S1"]).Statuses["b2c9"].Note.V; note != "From the side branch" {
		t.Errorf("at S1 the note is %q; want From the side branch", note)
	}
	if owner := log.Project(hashes["C5"]).Tasks["e4a1"].Assignee.V; owner != "olive@example.org" {
		t.Errorf("at C5 the owner is %q; want olive", owner)
	}
}
