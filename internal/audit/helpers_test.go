package audit

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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

// gitAs runs the git executable in dir as one person on one day, with no
// configuration of the host's, and returns what it prints, trimmed. The
// identity and the date are fixed, as in the Derivation's tests, so the
// hashes repeat. It skips the test when the host has no git.
func gitAs(t testing.TB, dir, who, day string, args ...string) string {
	t.Helper()
	exe, err := exec.LookPath("git")
	if err != nil {
		t.Skip("the host has no git executable")
	}
	cmd := exec.Command(exe, append([]string{"-c", "protocol.file.allow=always", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+who, "GIT_AUTHOR_EMAIL="+who+"@example.org",
		"GIT_COMMITTER_NAME="+who, "GIT_COMMITTER_EMAIL="+who+"@example.org",
		"GIT_AUTHOR_DATE="+day+"T12:00:00+00:00", "GIT_COMMITTER_DATE="+day+"T12:00:00+00:00",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_SYSTEM="+os.DevNull,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// The files of a repository a test builds: the version, the gating and a
// root that olive owns. The leaf b2c9 is pat's.
var planFiles = map[string]string{
	".tableaux/version.yaml": "tableaux: 0.3.1\ntrunk: main\n",
	".tableaux/gates.yaml": `gates:
  - { key: undefined, symbol: ❔, name: Undefined, criteria: No one has started work on the definition }
  - { key: defined,   symbol: 📝, name: Defined,   criteria: "Title, description, assignee and references exist" }
  - { key: design,    symbol: 📐, name: Design,    criteria: A model and sufficient tests exist }
  - { key: release,   symbol: 🚀, name: Release,   criteria: All variants documented and approved }
states:
  - { key: undefined, symbol: ⚪, severity: 0, synopsis: The work has not yet been defined }
  - { key: nominal,   symbol: 🟢, severity: 1, synopsis: The work is proceeding as expected }
  - { key: complete,  symbol: ✅, severity: 0, synopsis: All deliverables satisfy their requirements }
reasons:
  - { key: review, symbol: 👓, synopsis: The work waits for its reviewer }
`,
	".tableaux/tasks/e4a1.yaml": "title: Cases\ndescription: The cases of design dada.\nassignee: olive@example.org\n",
}

// leaf writes the task file of b2c9, pat's leaf, with its junctions as a
// flow mapping, or "" for none.
func leaf(junctions string) string {
	text := "title: Leaf\ndescription: A leaf.\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 1 }\n"
	if junctions != "" {
		text += "junctions: " + junctions + "\n"
	}
	return text
}

// repository is a repository a test builds under t.TempDir().
type repository struct {
	t   testing.TB
	dir string
}

// newRepository starts a repository on main with the plan files and b2c9,
// which olive commits on 2026-09-01.
func newRepository(t testing.TB, junctions string) *repository {
	t.Helper()
	r := &repository{t: t, dir: filepath.Join(t.TempDir(), "cases")}
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gitAs(t, r.dir, "olive", "2026-09-01", "init", "-q", "-b", "main")
	files := map[string]string{".tableaux/tasks/b2c9.yaml": leaf(junctions)}
	for name, text := range planFiles {
		files[name] = text
	}
	r.commit("olive", "2026-09-01", "Plan the cases", files)
	return r
}

// commit writes files and commits them as one person on one day, with the
// trailers given, and returns the commit in full.
func (r *repository) commit(who, day, subject string, files map[string]string, trailers ...string) string {
	r.t.Helper()
	write(r.t, r.dir, files)
	gitAs(r.t, r.dir, who, day, "add", "-A")
	args := []string{"commit", "-q", "--allow-empty", "-m", subject}
	for _, trailer := range trailers {
		args = append(args, "--trailer", trailer)
	}
	gitAs(r.t, r.dir, who, day, args...)
	return gitAs(r.t, r.dir, who, day, "rev-parse", "HEAD")
}

// facts derives the facts of the repository at main.
func (r *repository) facts() *derive.Facts {
	r.t.Helper()
	return derived(r.t, load.Options{}, r.dir, "main", "")
}
