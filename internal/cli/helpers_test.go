package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// corpus returns the directory of the built conformance corpus, and skips
// the test when it is absent: TABLEAUX_CORPUS, else ../tableaux/corpus/build
// beside the checkout, as the tests of the other packages find it.
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

// entry returns the repository of a built corpus entry.
func entry(t testing.TB, name string) string {
	t.Helper()
	return filepath.Join(corpus(t), name)
}

// quiet fixes what the command reads outside -C for a test: no viewer from
// the environment or the host's Git configuration, a cache of clones that
// is empty, and today's date.
func quiet(t *testing.T) {
	t.Helper()
	t.Setenv("GIT_AUTHOR_EMAIL", "")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	was := today
	today = func() string { return "2026-09-28" }
	t.Cleanup(func() { today = was })
}

// result is what one run of the command gives.
type result struct {
	code           int
	stdout, stderr string
}

// run runs one command line through Run.
func run(t *testing.T, args ...string) result {
	t.Helper()
	var out, errs bytes.Buffer
	code := Run(context.Background(), args, &out, &errs)
	return result{code, out.String(), errs.String()}
}

// people are the identities of the corpus's lib.sh.
var people = map[string]string{
	"ada": "Ada Byron", "ben": "Ben Okafor", "dan": "Dan Reyes", "olive": "Olive Marsh",
	"pat": "Pat Singh", "bot": "Corpus Agent",
}

// A built is a repository a test builds under t.TempDir(), with the fixed
// identities and dates of the corpus's lib.sh, so that its hashes repeat.
type built struct {
	t   testing.TB
	dir string
}

// build starts a repository whose trunk is main, with no host configuration.
func build(t testing.TB) *built {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not on the path")
	}
	b := &built{t: t, dir: t.TempDir()}
	b.git("olive", "2026-09-01", "init", "-q", "-b", "main")
	return b
}

// git runs git in the repository as who, on a date.
func (b *built) git(who, date string, args ...string) string {
	b.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", b.dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+people[who], "GIT_AUTHOR_EMAIL="+who+"@example.org",
		"GIT_COMMITTER_NAME="+people[who], "GIT_COMMITTER_EMAIL="+who+"@example.org",
		"GIT_AUTHOR_DATE="+date+"T12:00:00+00:00", "GIT_COMMITTER_DATE="+date+"T12:00:00+00:00",
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1", "LC_ALL=C", "TZ=UTC",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		b.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// put writes files by their paths below the repository's root.
func (b *built) put(files map[string]string) {
	b.t.Helper()
	for name, text := range files {
		file := filepath.Join(b.dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
			b.t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
			b.t.Fatal(err)
		}
	}
}

// commit stages everything and commits as who on a date, with the trailers
// given, and returns the commit.
func (b *built) commit(who, date, subject string, trailers ...string) string {
	b.t.Helper()
	b.git(who, date, "add", "-A")
	args := []string{"commit", "-q", "--allow-empty", "--no-gpg-sign", "-m", subject}
	for _, tr := range trailers {
		args = append(args, "--trailer", tr)
	}
	b.git(who, date, args...)
	return b.git(who, date, "rev-parse", "HEAD")
}

// The files of the small project the tests build: the base of the corpus,
// with an agent's junction on the leaf b2c9 and a second leaf c3d7 that is
// exempt from release.
const (
	baseVersion = "tableaux: 0.3.1\ntrunk: main\n"
	baseGates   = `gates:
  - { key: undefined, symbol: ❔, name: Undefined, criteria: No one has started work on the definition }
  - { key: defined,   symbol: 📝, name: Defined,   criteria: "Title, description, assignee and references exist" }
  - { key: design,    symbol: 📐, name: Design,    criteria: A model and sufficient tests exist }
  - { key: release,   symbol: 🚀, name: Release,   criteria: All variants documented and approved }
states:
  - { key: undefined, symbol: ⚪, severity: 0, synopsis: The work has not yet been defined }
  - { key: nominal,   symbol: 🟢, severity: 1, synopsis: The work is proceeding as expected }
  - { key: stalled,   symbol: 🔴, severity: 3, synopsis: Practically all progress has stalled }
  - { key: complete,  symbol: ✅, severity: 0, synopsis: All deliverables satisfy their requirements }
reasons:
  - { key: blocked, symbol: ⛔, synopsis: An external resource is unavailable }
`
	baseRoot = `title: Base
description: The smallest valid project.
assignee: olive@example.org
references:
  - { url: README.md }
`
	baseLeaf = `title: Leaf
description: A leaf task with a status.
assignee: pat@example.org
references:
  - { url: README.md }
junctions:
  design: { contributor: bot@example.org, model: claude-fable }
parent: { id: "e4a1", order: 1 }
`
	baseSecond = `title: Second leaf
description: A leaf task that no one has started.
assignee: ben@example.org
junctions:
  release: { applies: false }
parent: { id: "e4a1", order: 2 }
`
)

// small builds the small project in three commits and returns the
// repository: the owner's plan, the leaf at defined, and the agent's start.
func small(t testing.TB) *built {
	t.Helper()
	b := build(t)
	b.put(map[string]string{
		"README.md":                 "# Base\n",
		".tableaux/version.yaml":    baseVersion,
		".tableaux/gates.yaml":      baseGates,
		".tableaux/tasks/e4a1.yaml": baseRoot,
		".tableaux/tasks/b2c9.yaml": baseLeaf,
		".tableaux/tasks/c3d7.yaml": baseSecond,
	})
	b.commit("olive", "2026-09-01", "Plan the base")
	b.put(map[string]string{".tableaux/status/b2c9.yaml": "gate: defined\nstate: nominal\n"})
	b.commit("pat", "2026-09-02", "Define the leaf")
	b.put(map[string]string{".tableaux/status/b2c9.yaml": "gate: defined\nstate: nominal\nnote: Under way\n"})
	b.commit("bot", "2026-09-03", "Start the design", "Model: claude-fable-5-1")
	return b
}
