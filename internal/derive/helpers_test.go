package derive

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

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

// labelsOf returns the commits a labels file names.
func labelsOf(t testing.TB, file string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	hashes := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if label, hash, ok := strings.Cut(line, " "); ok {
			hashes[label] = hash
		}
	}
	return hashes
}

// labels returns the commits the build names in <entry>.labels.txt.
func labels(t testing.TB, entry string) map[string]string {
	t.Helper()
	return labelsOf(t, filepath.Join(corpus(t), entry+".labels.txt"))
}

// copyTree copies a directory with its symbolic links and file modes.
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

// clone copies a built corpus repository into a temporary directory, for a
// test that changes it, and returns the copy's path.
func clone(t testing.TB, name string) string {
	t.Helper()
	to := filepath.Join(t.TempDir(), name)
	copyTree(t, builtAt(t, name), to)
	return to
}

// gitEnv is the fixed identity and date of every commit a test makes, with
// no configuration of the host's.
func gitEnv() []string {
	return append(os.Environ(),
		"GIT_AUTHOR_NAME=Olive Marsh", "GIT_AUTHOR_EMAIL=olive@example.org",
		"GIT_COMMITTER_NAME=Olive Marsh", "GIT_COMMITTER_EMAIL=olive@example.org",
		"GIT_AUTHOR_DATE=2026-09-01T12:00:00+00:00", "GIT_COMMITTER_DATE=2026-09-01T12:00:00+00:00",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
	)
}

// gitRun runs the git executable in dir and returns what it prints, trimmed.
// It skips the test when the host has no git.
func gitRun(t testing.TB, dir string, args ...string) string {
	t.Helper()
	exe, err := exec.LookPath("git")
	if err != nil {
		t.Skip("the host has no git executable")
	}
	cmd := exec.Command(exe, append([]string{"-c", "protocol.file.allow=always", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = gitEnv()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// write puts files under dir, each path relative to it.
func write(t testing.TB, dir string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		file := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
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

// built returns every repository the corpus builds, an entry or a subproject
// of one, by name.
func built(t testing.TB) []string {
	t.Helper()
	list, err := os.ReadDir(corpus(t))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range list {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// derived loads a source and derives its facts from the passes of a new Reader.
func derived(t testing.TB, options load.Options, dir, ref, trunk string) *Facts {
	t.Helper()
	p := loaded(t, options, dir, ref)
	set, err := history.NewReader(history.Options{}).Read(context.Background(), p, trunk)
	if err != nil {
		t.Fatalf("Read(%q, %q): %v", dir, ref, err)
	}
	return New(p, set)
}

// The gating of design 27a3's cases, with the version every case states.
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

// leafOf writes a task file: its assignee, its parent and order, and its
// junctions as a flow mapping, or "" for none. A parent of "" makes the root.
func leafOf(assignee, parent string, order int, junctions string) string {
	text := "title: A task\nassignee: " + assignee + "\n"
	if parent != "" {
		text += fmt.Sprintf("parent: { id: %q, order: %d }\n", parent, order)
	}
	if junctions != "" {
		text += "junctions: " + junctions + "\n"
	}
	return text
}

// at writes a status file.
func at(gate, state string) string {
	return "gate: " + gate + "\nstate: " + state + "\n"
}

// compose builds a project in memory from files below .tableaux, through
// load.Parse and load.Compose, with the version and the gating of the cases
// unless the files state their own.
func compose(t testing.TB, files map[string]string) *model.Project {
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
	return p
}

// mem derives the facts of a project built in memory, with no history.
func mem(t testing.TB, files map[string]string) *Facts {
	t.Helper()
	f := New(compose(t, files), nil)
	if f.Refused() != nil {
		t.Fatalf("the case is refused: %+v", f.Refused())
	}
	return f
}

// cases builds the repository of design 27a3's cases under a temporary
// directory and returns its path, the commits it labels and the labels in
// the order the script makes them. It skips the test when the host has no
// shell or no git.
func cases(t testing.TB) (repo string, hashes map[string]string, order []string) {
	t.Helper()
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("the host has no sh executable")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("the host has no git executable")
	}
	script, err := filepath.Abs(filepath.Join("testdata", "cases.sh"))
	if err != nil {
		t.Fatal(err)
	}
	repo = filepath.Join(t.TempDir(), "cases")
	cmd := exec.Command(sh, script, repo)
	cmd.Env = gitEnv()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("cases.sh: %v\n%s", err, out)
	}
	data, err := os.ReadFile(repo + ".labels.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if label, _, ok := strings.Cut(line, " "); ok {
			order = append(order, label)
		}
	}
	return repo, labelsOf(t, repo+".labels.txt"), order
}
