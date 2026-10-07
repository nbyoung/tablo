package git

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// run runs the git executable in dir with a fixed identity and date and
// returns what it prints, trimmed. It skips the test when the host has no git.
func run(t *testing.T, dir string, args ...string) string {
	t.Helper()
	exe, err := exec.LookPath("git")
	if err != nil {
		t.Skip("the host has no git executable")
	}
	cmd := exec.Command(exe, append([]string{"-c", "commit.gpgsign=false", "-c", "protocol.file.allow=always"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Olive Marsh", "GIT_AUTHOR_EMAIL=olive@example.org",
		"GIT_COMMITTER_NAME=Olive Marsh", "GIT_COMMITTER_EMAIL=olive@example.org",
		"GIT_AUTHOR_DATE=2026-09-01T12:00:00+00:00", "GIT_COMMITTER_DATE=2026-09-01T12:00:00+00:00",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// repository makes a repository with one commit: a file, a nested file and
// a symbolic link. It returns its path, resolved, and the commit.
func repository(t *testing.T) (dir, commit string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	run(t, dir, "init", "-q", "-b", "main")
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, text := range map[string]string{"one.txt": "one\n", "a/b/two *.txt": "two\n\n", "a/empty": ""} {
		if err := os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("one.txt", filepath.Join(dir, "link")); err != nil {
		t.Skipf("the host makes no symbolic link: %v", err)
	}
	run(t, dir, "add", "-A")
	run(t, dir, "commit", "-q", "-m", "One commit")
	return dir, run(t, dir, "rev-parse", "HEAD")
}

func TestDiscover(t *testing.T) {
	ctx := context.Background()
	dir, commit := repository(t)
	var r Runner

	got, err := r.Discover(ctx, Repo{Dir: filepath.Join(dir, "a", "b")}, "HEAD")
	want := Discovery{
		GitDir: filepath.Join(dir, ".git"), CommonDir: filepath.Join(dir, ".git"),
		InWorkTree: true, Top: dir, Prefix: "a/b", Commit: commit,
	}
	if err != nil || got != want {
		t.Errorf("in a/b: %+v, %v; want %+v", got, err, want)
	}

	// A revision that names no commit is no error, whatever it looks like.
	for _, rev := range []string{"nowhere", "--help", "-q", "HEAD:one.txt", "HEAD^{tree}"} {
		got, err := r.Discover(ctx, Repo{Dir: dir}, rev)
		if err != nil || got.Commit != "" || got.Detail == "" || got.Top != dir || got.Prefix != "" {
			t.Errorf("at %q: %+v, %v", rev, got, err)
		}
	}
	for _, rev := range []string{"main", commit[:8], commit} {
		if got, err := r.Discover(ctx, Repo{Dir: dir}, rev); err != nil || got.Commit != commit {
			t.Errorf("at %q: %+v, %v", rev, got, err)
		}
	}

	// A linked worktree has its own Git directory and shares the common one.
	linked := filepath.Join(filepath.Dir(dir), filepath.Base(dir)+"-linked")
	run(t, dir, "worktree", "add", "-q", "--detach", linked)
	t.Cleanup(func() { _ = os.RemoveAll(linked) })
	got, err = r.Discover(ctx, Repo{Dir: linked}, "HEAD")
	if err != nil || got.CommonDir != filepath.Join(dir, ".git") || got.GitDir == got.CommonDir ||
		!strings.HasPrefix(got.GitDir, filepath.Join(dir, ".git", "worktrees")) || got.Top != linked {
		t.Errorf("in a linked worktree: %+v, %v", got, err)
	}

	// A bare clone has no working tree.
	bare := filepath.Join(t.TempDir(), "bare.git")
	run(t, dir, "clone", "-q", "--bare", dir, bare)
	bare, _ = filepath.EvalSymlinks(bare)
	for _, repo := range []Repo{{Dir: bare}, {GitDir: bare}} {
		got, err = r.Discover(ctx, repo, "main")
		if err != nil || got.Bare != (repo.Dir != "") || got.InWorkTree || got.Top != "" || got.GitDir != bare || got.Commit != commit {
			t.Errorf("in a bare clone, %+v: %+v, %v", repo, got, err)
		}
	}

	// A directory outside any repository, and one that is absent.
	outside := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))
	for _, repo := range []Repo{{Dir: outside}, {Dir: filepath.Join(outside, "absent")}, {GitDir: outside}} {
		if _, err := r.Discover(ctx, repo, "HEAD"); !errors.Is(err, ErrNoRepository) {
			t.Errorf("%+v: %v, want ErrNoRepository", repo, err)
		}
	}

	// An executable that is not there.
	missing := Runner{Exe: filepath.Join(outside, "no-such-git")}
	if _, err := missing.Discover(ctx, Repo{Dir: dir}, "HEAD"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a missing executable: %v, want ErrNotFound", err)
	}
}

func TestTreesAndBlobs(t *testing.T) {
	ctx := context.Background()
	dir, commit := repository(t)
	var r Runner
	repo := Repo{Dir: filepath.Join(dir, "a")} // the paths stay relative to the root

	all, err := r.LsTree(ctx, repo, commit, true, "a", "link", "one.txt", "absent")
	if err != nil {
		t.Fatal(err)
	}
	var paths, ids []string
	for _, e := range all {
		paths = append(paths, e.Mode+" "+e.Path)
		ids = append(ids, e.ID)
	}
	if want := []string{"100644 a/b/two *.txt", "100644 a/empty", "120000 link", "100644 one.txt"}; !reflect.DeepEqual(paths, want) {
		t.Errorf("ls-tree -r: %q, want %q", paths, want)
	}
	if !all[0].Regular() || all[2].Regular() {
		t.Error("Regular tells a file from a symbolic link")
	}
	// A path named below another leaves the other's own entry in the list.
	own, err := r.LsTree(ctx, repo, commit, false, "a", "a/b", "one.txt")
	modes := map[string]string{}
	for _, e := range own {
		modes[e.Path] = e.Mode
	}
	if err != nil || modes["a"] != ModeTree || modes["a/b"] != ModeTree || modes["one.txt"] != "100644" {
		t.Errorf("ls-tree of two directories and a file: %+v, %v", own, err)
	}
	// A path is literal: the star names itself and nothing else.
	if star, err := r.LsTree(ctx, repo, commit, true, "a/b/*"); err != nil || len(star) != 0 {
		t.Errorf("ls-tree of a/b/*: %+v, %v", star, err)
	}

	blobs, err := r.Blobs(ctx, repo, ids)
	if err != nil {
		t.Fatal(err)
	}
	if want := [][]byte{[]byte("two\n\n"), {}, []byte("one.txt"), []byte("one\n")}; !reflect.DeepEqual(blobs, want) {
		t.Errorf("blobs: %q, want %q", blobs, want)
	}
	if none, err := r.Blobs(ctx, repo, nil); err != nil || none != nil {
		t.Errorf("no blobs: %q, %v", none, err)
	}
	if _, err := r.Blobs(ctx, repo, []string{strings.Repeat("0", 40)}); err == nil {
		t.Error("a missing object is no error")
	}

	if tree, ok, err := r.Tree(ctx, repo, commit); err != nil || !ok || tree != run(t, dir, "rev-parse", "HEAD^{tree}") {
		t.Errorf("Tree(commit) = %q, %v, %v", tree, ok, err)
	}
	for _, rev := range []string{"nowhere", "--help", "HEAD:one.txt"} {
		if tree, ok, err := r.Tree(ctx, repo, rev); err != nil || ok {
			t.Errorf("Tree(%q) = %q, %v, %v", rev, tree, ok, err)
		}
	}

	index, err := r.LsFiles(ctx, Repo{Dir: dir}, "a/b/two *.txt", "link", "absent")
	if err != nil || len(index) != 2 || index[0].Path != "a/b/two *.txt" || index[0].ID != ids[0] || index[1].Mode != "120000" {
		t.Errorf("ls-files: %+v, %v", index, err)
	}
}

func TestConfig(t *testing.T) {
	ctx := context.Background()
	dir, commit := repository(t)
	var r Runner
	repo := Repo{Dir: dir}
	url := "https://example.org/Lib.git"
	run(t, dir, "config", "tableaux."+url+".path", "/clones/lib")
	run(t, dir, "config", "tableaux.project", "lib")

	got, err := r.Config(ctx, repo, nil, `^tableaux\..*\.path$`)
	if want := map[string]string{"tableaux." + url + ".path": "/clones/lib"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Config: %v, %v; want %v", got, err, want)
	}
	if got, err := r.Config(ctx, repo, nil, `^nothing\.`); err != nil || len(got) != 0 {
		t.Errorf("no key: %v, %v", got, err)
	}
	if got, err := r.Config(ctx, repo, []string{"--blob", commit + ":.gitmodules"}, `^submodule\.`); err != nil || len(got) != 0 {
		t.Errorf("an absent blob: %v, %v", got, err)
	}
	modules := filepath.Join(dir, ".gitmodules")
	if err := os.WriteFile(modules, []byte("[submodule \"Name With Space\"]\n\tpath = sub/dir\n\turl = ../x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = r.Config(ctx, repo, []string{"--file", modules}, `^submodule\..*\.path$`)
	if want := map[string]string{"submodule.Name With Space.path": "sub/dir"}; err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Config of a file: %v, %v; want %v", got, err, want)
	}
}

// A command that addresses another repository runs without the variables that
// bind a process to a repository; one in the repository asked for keeps them.
func TestForeignEnvironment(t *testing.T) {
	ctx := context.Background()
	dir, commit := repository(t)
	other, _ := repository(t)
	if err := os.WriteFile(filepath.Join(other, "more"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	run(t, other, "add", "-A")
	run(t, other, "commit", "-q", "-m", "A second commit")
	otherCommit := run(t, other, "rev-parse", "HEAD")
	if otherCommit == commit {
		t.Fatal("the two repositories stand at one commit")
	}

	// The list holds what git itself names.
	want := strings.Fields(run(t, dir, "rev-parse", "--local-env-vars"))
	have := append([]string(nil), localEnv...)
	sort.Strings(want)
	sort.Strings(have)
	if !reflect.DeepEqual(have, want) {
		t.Logf("git names %v; the list holds %v", want, have)
		for _, name := range want {
			if i := sort.SearchStrings(have, name); i == len(have) || have[i] != name {
				t.Errorf("the list lacks %s", name)
			}
		}
	}

	t.Setenv("GIT_DIR", filepath.Join(dir, ".git"))
	t.Setenv("GIT_INDEX_FILE", filepath.Join(dir, ".git", "index"))
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "core.bare")
	t.Setenv("GIT_CONFIG_VALUE_0", "true")
	var r Runner
	got, err := r.Discover(ctx, Repo{GitDir: filepath.Join(other, ".git")}, "HEAD")
	if err != nil || got.Commit != otherCommit || got.CommonDir != filepath.Join(other, ".git") {
		t.Errorf("another repository under a hook's variables: %+v, %v; want %s", got, err, otherCommit)
	}
	got, err = r.Discover(ctx, Repo{Dir: other}, "HEAD")
	if err != nil || got.Commit != commit {
		t.Errorf("the repository asked for follows GIT_DIR: %+v, %v; want %s", got, err, commit)
	}
}

// A submodule's clone reads through its gitfile, and still reads once the
// checkout its configuration names as the working tree is gone.
func TestSubmoduleStore(t *testing.T) {
	ctx := context.Background()
	sub, commit := repository(t)
	parent, _ := repository(t)
	run(t, parent, "submodule", "add", "-q", sub, "vendor/sub")
	run(t, parent, "commit", "-q", "-m", "Pin the submodule")
	store := filepath.Join(parent, ".git", "modules", "vendor", "sub")
	var r Runner
	for _, repo := range []Repo{{GitDir: filepath.Join(parent, "vendor", "sub", ".git")}, {GitDir: store}} {
		got, err := r.Discover(ctx, repo, "HEAD")
		if err != nil || got.Commit != commit || got.GitDir != store || got.CommonDir != store {
			t.Errorf("%+v: %+v, %v", repo, got, err)
		}
	}
	if err := os.RemoveAll(filepath.Join(parent, "vendor")); err != nil {
		t.Fatal(err)
	}
	got, err := r.Discover(ctx, Repo{GitDir: store}, commit)
	if err != nil || got.Commit != commit {
		t.Errorf("with the checkout gone: %+v, %v", got, err)
	}
	if entries, err := r.LsTree(ctx, Repo{GitDir: store}, commit, true, "a"); err != nil || len(entries) != 2 {
		t.Errorf("with the checkout gone: %+v, %v", entries, err)
	}
}

func TestRunErrors(t *testing.T) {
	dir, _ := repository(t)
	var r Runner
	_, err := r.Run(context.Background(), Repo{Dir: dir}, nil, "cat-file", "-e", "nowhere")
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code == 0 || exit.Stderr == "" || !strings.Contains(exit.Error(), "cat-file") {
		t.Errorf("a failing command: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.Run(ctx, Repo{Dir: dir}, nil, "status"); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled context: %v", err)
	}
}

// built returns a repository of the built conformance corpus and the commits
// its build labels, and skips the test when the corpus is absent:
// TABLEAUX_CORPUS, else ../tableaux/corpus/build beside the checkout.
func built(t *testing.T, entry string) (dir string, labels map[string]string) {
	t.Helper()
	root := os.Getenv("TABLEAUX_CORPUS")
	if root == "" {
		root = filepath.Join("..", "..", "..", "tableaux", "corpus", "build")
	}
	data, err := os.ReadFile(filepath.Join(root, entry+".labels.txt"))
	if err != nil {
		t.Skipf("the built corpus is absent at %s; set TABLEAUX_CORPUS", root)
	}
	labels = map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if label, hash, ok := strings.Cut(line, " "); ok {
			labels[label] = hash
		}
	}
	return filepath.Join(root, entry), labels
}

// T1: Log reads a record. The weather station holds a merge, a commit with a
// trailer and no change, a first pin and a root commit.
func TestLog(t *testing.T) {
	ctx := context.Background()
	dir, labels := built(t, "weather-station")
	var r Runner
	commits, err := r.Log(ctx, Repo{Dir: dir}, []string{labels["W13"]}, []string{".tableaux", "firmware"})
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 13 || commits[0].ID != labels["W13"] {
		t.Fatalf("Log returns %d commits from %v; want the thirteen, W13 first", len(commits), commits[0].ID)
	}
	by := map[string]LogCommit{}
	for i, c := range commits {
		by[c.ID] = c
		for _, parent := range c.Parents {
			if _, seen := by[parent]; seen {
				t.Errorf("commit %d, %s, follows its parent %s", i, c.ID, parent)
			}
		}
	}
	paths := func(c LogCommit) []string {
		var list []string
		for _, change := range c.Changes {
			list = append(list, change.Path)
		}
		return list
	}

	// The merge: two parents, the change against the first, and its trailer.
	w4 := by[labels["W4"]]
	if want := []string{labels["W2"], labels["W3"]}; !reflect.DeepEqual(w4.Parents, want) {
		t.Errorf("W4 has the parents %v; want %v", w4.Parents, want)
	}
	if got := paths(w4); !reflect.DeepEqual(got, []string{".tableaux/tasks/9f31.yaml"}) {
		t.Errorf("W4 changes %v; want the task file of 9f31 alone", got)
	}
	if !reflect.DeepEqual(w4.Trailers, []string{"Authorised: 9f31"}) {
		t.Errorf("W4 carries %q; want Authorised: 9f31", w4.Trailers)
	}
	if c := w4.Changes[0]; c.OldMode != "000000" || c.NewMode != "100644" || strings.Trim(c.Old, "0") != "" || len(c.New) != 40 {
		t.Errorf("W4 changes the file as %+v; want an addition with full ids", c)
	}

	// A trailer and no change.
	w6 := by[labels["W6"]]
	if len(w6.Changes) != 0 || !reflect.DeepEqual(w6.Trailers, []string{"Authorised: 3c5d"}) {
		t.Errorf("W6 has the changes %v and the trailers %q; want none and Authorised: 3c5d", w6.Changes, w6.Trailers)
	}
	if w6.AuthorEmail != "ada@example.org" || w6.AuthorName != "Ada Byron" || w6.CommitterEmail != "ada@example.org" ||
		w6.AuthorZone != 0 || w6.AuthorTime != 1789992000 || w6.Subject == "" {
		t.Errorf("W6 reads as %+v", w6)
	}

	// The first pin is a gitlink beside the status file.
	w11 := by[labels["W11"]]
	if got := paths(w11); !reflect.DeepEqual(got, []string{".tableaux/status/c07d.yaml", "firmware"}) {
		t.Fatalf("W11 changes %v; want the status of c07d and the gitlink", got)
	}
	if c := w11.Changes[1]; c.NewMode != ModeGitlink || c.OldMode != "000000" {
		t.Errorf("W11 changes firmware as %+v; want a new gitlink", c)
	}

	// The root commit lists every file it holds among the paths.
	w1 := by[labels["W1"]]
	listed, err := r.LsTree(ctx, Repo{Dir: dir}, labels["W1"], true, ".tableaux")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, e := range listed {
		want = append(want, e.Path)
	}
	if got := paths(w1); len(w1.Parents) != 0 || len(got) == 0 || !reflect.DeepEqual(got, want) {
		t.Errorf("W1 has the parents %v and changes %v; want none and %v", w1.Parents, got, want)
	}
}

func TestLogZone(t *testing.T) {
	dir, _ := repository(t)
	if err := os.WriteFile(filepath.Join(dir, "one.txt"), []byte("two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "-c", "commit.gpgsign=false", "commit", "-q", "-a", "-m", "East of UTC",
		"-m", "Reviewed: 9f31 design\nModel: one\n  folded")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Pat Singh", "GIT_AUTHOR_EMAIL=pat@example.org",
		"GIT_COMMITTER_NAME=Olive Marsh", "GIT_COMMITTER_EMAIL=olive@example.org",
		"GIT_AUTHOR_DATE=2026-09-02T01:30:00+05:30", "GIT_COMMITTER_DATE=2026-09-03T12:00:00-08:00",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	var r Runner
	commits, err := r.Log(context.Background(), Repo{Dir: dir}, []string{"HEAD"}, []string{"one.txt"})
	if err != nil || len(commits) != 2 {
		t.Fatalf("Log: %d commits, %v; want two", len(commits), err)
	}
	c := commits[0]
	if c.AuthorZone != 330 || c.AuthorTime != 1788292800 || c.AuthorEmail != "pat@example.org" || c.CommitterName != "Olive Marsh" {
		t.Errorf("the commit reads as %+v; want the author's time and zone, +05:30", c)
	}
	if want := []string{"Reviewed: 9f31 design", "Model: one folded"}; !reflect.DeepEqual(c.Trailers, want) {
		t.Errorf("the trailers are %q; want %q", c.Trailers, want)
	}
}

func TestRefs(t *testing.T) {
	ctx := context.Background()
	dir, commit := repository(t)
	run(t, dir, "branch", "side")
	run(t, dir, "update-ref", "refs/remotes/origin/main", commit)
	run(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/main")
	run(t, dir, "tag", "v1")
	var r Runner
	got, err := r.Refs(ctx, Repo{Dir: dir}, "refs/heads", "refs/remotes")
	want := []Ref{
		{Name: "refs/heads/main", Commit: commit},
		{Name: "refs/heads/side", Commit: commit},
		{Name: "refs/remotes/origin/HEAD", Commit: commit, Target: "refs/remotes/origin/main"},
		{Name: "refs/remotes/origin/main", Commit: commit},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Refs: %+v, %v; want %+v", got, err, want)
	}
	if got, err := r.Refs(ctx, Repo{Dir: dir}, "refs/nowhere"); err != nil || len(got) != 0 {
		t.Errorf("Refs of an empty prefix: %+v, %v; want none", got, err)
	}
}
