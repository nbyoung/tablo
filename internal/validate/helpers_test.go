package validate

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// corpus returns the directory of the built conformance corpus, and skips
// the test when it is absent. It stands in for the helper of task 4b4f, as
// the Loader's tests do: TABLEAUX_CORPUS, else ../tableaux/corpus/build
// beside the checkout.
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

// beside returns a path of the corpus beside its build directory: the rule
// table RULES.md and the entries with their expected.yaml. It skips the test
// when the path is absent.
func beside(t testing.TB, path ...string) string {
	t.Helper()
	p := filepath.Join(append([]string{filepath.Dir(corpus(t))}, path...)...)
	if _, err := os.Stat(p); err != nil {
		t.Skipf("the corpus holds no %s beside its build", filepath.Join(path...))
	}
	return p
}

// finding is one finding of an entry's expected.yaml.
type finding struct {
	Rule, Severity, Task, File, Gate, Commit, Trailer string
}

// expectation is what the tests read of an entry's expected.yaml.
type expectation struct {
	Ref      string
	Replace  map[string]string
	Findings []finding
	Tasks    map[string]struct {
		Applicable []string
	}
}

// expected reads the expected.yaml of a corpus entry.
func expected(t testing.TB, name string) expectation {
	t.Helper()
	data, err := os.ReadFile(beside(t, "entries", name, "expected.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var e expectation
	if err := yaml.Unmarshal(data, &e); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return e
}

// entry loads a built corpus entry as its expected.yaml says: at its ref,
// main unless it states another, with its replace mapping. A test reads the
// corpus and never changes it.
func entry(t testing.TB, name string) *model.Project {
	t.Helper()
	e := expected(t, name)
	ref := e.Ref
	if ref == "" {
		ref = "main"
	}
	replace := map[string]string{}
	for url, sub := range e.Replace {
		replace[url] = filepath.Join(corpus(t), name+"."+sub)
	}
	return loadAt(t, load.Options{Replace: replace}, filepath.Join(corpus(t), name), ref)
}

// loadAt reads a source with a Loader whose cache is an empty temporary
// directory, and fails the test on a load error.
func loadAt(t testing.TB, options load.Options, dir, ref string) *model.Project {
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

// gitRun runs the git executable in dir with the fixed identity and date of
// the Loader's tests and no configuration of the host's. It skips the test
// when the host has no git.
func gitRun(t testing.TB, dir string, args ...string) string {
	t.Helper()
	exe, err := exec.LookPath("git")
	if err != nil {
		t.Skip("the host has no git executable")
	}
	cmd := exec.Command(exe, append([]string{"-c", "protocol.file.allow=always", "-c", "commit.gpgsign=false", "-c", "init.defaultBranch=main"}, args...)...)
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
		t.Fatalf("git %s in %s: %v\n%s", strings.Join(args, " "), dir, err, stderr.String())
	}
	return strings.TrimSpace(string(out))
}

// The files of the smallest valid project, which a test overlays.
const (
	baseVersion = "tableaux: 0.3.1\ntrunk: main\n"
	baseGates   = `gates:
  - { key: undefined, symbol: "?", name: Undefined, criteria: No one has started work on the definition }
  - { key: defined, symbol: D, name: Defined, criteria: The definition exists }
  - { key: design, symbol: M, name: Design, criteria: A model and sufficient tests exist }
  - { key: release, symbol: R, name: Release, criteria: All variants documented and approved }
states:
  - { key: undefined, symbol: U, severity: 0, synopsis: The work has not yet been defined }
  - { key: nominal, symbol: N, severity: 1, synopsis: The work is proceeding as expected }
  - { key: complete, symbol: C, severity: 0, synopsis: All deliverables satisfy their requirements }
reasons:
  - { key: blocked, symbol: B, synopsis: An external resource is unavailable }
`
	baseRoot = "title: Base\ndescription: The root.\nassignee: olive@example.org\n"
	baseLeaf = "title: Leaf\ndescription: A leaf.\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 1 }\n"
)

// base returns the files of the smallest valid project, by path below
// .tableaux: a root e4a1 and a leaf b2c9 with no status.
func base() map[string]string {
	return map[string]string{
		"version.yaml":    baseVersion,
		"gates.yaml":      baseGates,
		"tasks/e4a1.yaml": baseRoot,
		"tasks/b2c9.yaml": baseLeaf,
	}
}

// with returns files overlaid with pairs of a path and its content; an empty
// content drops the path.
func with(files map[string]string, pairs ...string) map[string]string {
	for i := 0; i+1 < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			delete(files, pairs[i])
		} else {
			files[pairs[i]] = pairs[i+1]
		}
	}
	return files
}

// repository writes files below .tableaux of a new repository with no commit
// under t.TempDir() and returns its path. A path that starts with / is
// relative to the repository's root.
func repository(t testing.TB, files map[string]string) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "init", "-q", "-b", "main")
	for path, content := range files {
		full := filepath.Join(repo, ".tableaux", filepath.FromSlash(path))
		if rest, ok := strings.CutPrefix(path, "/"); ok {
			full = filepath.Join(repo, filepath.FromSlash(rest))
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

// project builds a project from files and loads it from the working tree.
func project(t testing.TB, files map[string]string) *model.Project {
	t.Helper()
	return loadAt(t, load.Options{}, repository(t, files), "")
}

// where writes where a diagnostic sits and what it names, as a line of
// corpus.expected.txt does before its message: the file, line and column, the
// severity, the code, the task and the gate, each part only when the
// diagnostic has it.
func where(d model.Diagnostic) string {
	var b strings.Builder
	if d.Pos.File != "" {
		b.WriteString(d.Pos.File)
		if d.Pos.Line > 0 {
			fmt.Fprintf(&b, ":%d", d.Pos.Line)
			if d.Pos.Col > 0 {
				fmt.Fprintf(&b, ":%d", d.Pos.Col)
			}
		}
		b.WriteString(": ")
	}
	fmt.Fprintf(&b, "%s: %s", d.Severity, d.Code)
	if d.Task != "" {
		b.WriteString(" task=" + d.Task)
	}
	if d.Gate != "" {
		b.WriteString(" gate=" + d.Gate)
	}
	if d.Commit != "" {
		b.WriteString(" commit=" + d.Commit)
	}
	if d.Trailer != "" {
		b.WriteString(" trailer=" + d.Trailer)
	}
	return b.String()
}
