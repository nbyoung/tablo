package corpustest

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) { os.Exit(Main(m)) }

// stop is what the fake test panics with when the code under test ends it.
type stop struct{}

// fake is a testing.TB that records how the code under test ends it, so a
// test can observe a skip or a failure of Open and Entry.
type fake struct {
	testing.TB
	skipped, failed string
}

func (f *fake) Helper() {}
func (f *fake) Skipf(format string, args ...any) {
	f.skipped = fmt.Sprintf(format, args...)
	panic(stop{})
}
func (f *fake) Fatalf(format string, args ...any) {
	f.failed = fmt.Sprintf(format, args...)
	panic(stop{})
}

// run calls f and reports whether it ended the fake test.
func (f *fake) run(fn func()) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(stop); !ok {
				panic(r)
			}
		}
	}()
	fn()
}

// TestVendoredCopy is T1: the copy is the one the lock names, and the lock's
// commit is the commit the task file names.
func TestVendoredCopy(t *testing.T) {
	repository, commit, tooling, digest := Lock()
	if repository == "" {
		t.Error("the lock names no repository")
	}
	hex40 := regexp.MustCompile(`^[0-9a-f]{40}$`)
	for name, v := range map[string]string{"commit": commit, "tooling": tooling} {
		if !hex40.MatchString(v) {
			t.Errorf("the lock's %s is %q, not forty hexadecimal digits", name, v)
		}
	}
	if got := Digest(); got != digest {
		t.Errorf("the copy's digest is %s, and the lock states %s; run corpus.sh refresh", got, digest)
	}

	data, err := os.ReadFile(filepath.Join("..", "..", ".tableaux", "tasks", "4b4f.yaml"))
	if err != nil {
		t.Skipf("the plan is absent: %v", err)
	}
	var task struct {
		Requires []struct {
			Subproject *struct {
				URL    string `yaml:"url"`
				ID     string `yaml:"id"`
				Commit string `yaml:"commit"`
			} `yaml:"subproject"`
		} `yaml:"requires"`
	}
	if err := yaml.Unmarshal(data, &task); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range task.Requires {
		if s := r.Subproject; s != nil && s.ID == "fcec" && s.URL == repository {
			found = true
			if s.Commit != commit {
				t.Errorf("the task file requires fcec at %s, and the lock names %s", s.Commit, commit)
			}
		}
	}
	if !found {
		t.Errorf("the task file states no requirement on fcec at %s", repository)
	}
}

// moduleFiles lists the regular files of the module outside .git.
func moduleFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(filepath.Join("..", ".."), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	return files
}

// TestBuildIsClosed is T3: the build repeats after the host's own
// configuration changes, and writes nothing into the module tree.
func TestBuildIsClosed(t *testing.T) {
	c := Open(t)
	before := moduleFiles(t)
	e := c.Entry(t, "weather-station")
	labels := filepath.Join(c.Work, "build", "weather-station.labels.txt")
	first, err := os.ReadFile(labels)
	if err != nil {
		t.Fatal(err)
	}
	if len(bytes.Fields(first)) < 26 {
		t.Fatalf("the labels file is short: %q", first)
	}

	home := t.TempDir()
	config := filepath.Join(home, "gitconfig")
	body := "[user]\n\tname = Somebody Else\n\temail = else@example.net\n[commit]\n\tgpgsign = true\n[init]\n\tdefaultBranch = trunk\n"
	if err := os.WriteFile(config, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", config)
	t.Setenv("GIT_AUTHOR_NAME", "Somebody Else")
	t.Setenv("GIT_COMMITTER_EMAIL", "else@example.net")
	if err := c.runBuild("weather-station"); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(labels)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("the labels differ after the host's configuration changes:\n%s\nagainst\n%s", first, second)
	}
	if h, ok := e.Hash("W1"); !ok || h == "" {
		t.Errorf("W1 does not resolve")
	}
	if after := moduleFiles(t); strings.Join(after, "\n") != strings.Join(before, "\n") {
		t.Errorf("the module tree changed:\nbefore %d files, after %d files", len(before), len(after))
	}
}

// TestLabels is T4: labels resolve and reverse.
func TestLabels(t *testing.T) {
	c := Open(t)
	e := c.Entry(t, "weather-station")
	w1, ok := e.Hash("W1")
	if !ok || !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(w1) {
		t.Fatalf("W1 resolves to %q, %v", w1, ok)
	}
	f2, ok := e.Hash("F2")
	if !ok || f2 == w1 {
		t.Fatalf("the firmware's F2 resolves to %q, %v", f2, ok)
	}
	if got, ok := e.Hash(w1); !ok || got != w1 {
		t.Errorf("a hash resolves to %q, %v", got, ok)
	}
	if got, ok := e.Hash("Z9"); ok {
		t.Errorf("Z9 resolves to %q", got)
	}
	if got := e.Label(w1); got != "W1" {
		t.Errorf("Label of W1's hash is %q", got)
	}
	if got := e.Label(f2); got != "F2" {
		t.Errorf("Label of F2's hash is %q", got)
	}
	if got := e.Label(strings.Repeat("0", 40)); got != "" {
		t.Errorf("Label of an unknown hash is %q", got)
	}
	if e.Repo != filepath.Join(c.Work, "build", "weather-station") || e.Ref != "main" || e.Language == "" || !e.Valid {
		t.Errorf("the entry reads as %+v", e)
	}
	r := c.Entry(t, "subproject-by-url")
	if len(r.Replace) != 1 || r.Replace["https://example.org/lib.git"] != filepath.Join(c.Work, "build", "subproject-by-url.lib") {
		t.Errorf("the replace field reads as %v", r.Replace)
	}
	if got := c.Entry(t, "trunk-stated").Ref; got != "develop" {
		t.Errorf("trunk-stated reads ref %q", got)
	}
}

// TestHelperStaysOutOfBinary is T5: the command does not link corpustest.
func TestHelperStaysOutOfBinary(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Skip("the host has no go command to list the dependencies")
	}
	cmd := exec.Command(goTool, "list", "-deps", "./cmd/tablo")
	cmd.Dir = filepath.Join("..", "..")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	if !strings.Contains(string(out), "github.com/nbyoung/tablo/internal/model") {
		t.Fatalf("the list of dependencies looks wrong:\n%s", out)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.Contains(dep, "corpustest") {
			t.Errorf("the command depends on %s", dep)
		}
	}
}

// TestRules is T6: Rules reads RULES.md. The counts are those of the copy
// at the locked commit: 81 rules, the 77 of the design's day and L1 to L4.
func TestRules(t *testing.T) {
	c := Open(t)
	rules := c.Rules(t)
	counts := map[string]int{}
	ids := map[string]bool{}
	for _, r := range rules {
		counts[r.Severity]++
		if ids[r.ID] {
			t.Errorf("rule %s appears twice", r.ID)
		}
		ids[r.ID] = true
		if len(r.Entries) == 0 && r.ID != "" {
			t.Errorf("rule %s names no entry", r.ID)
		}
	}
	if len(rules) != 81 || counts["error"] != 67 || counts["warning"] != 13 || counts["information"] != 1 {
		t.Errorf("%d rules: %v", len(rules), counts)
	}
	for _, id := range []string{"P1", "L4", "H6", "S11", "R13"} {
		if !ids[id] {
			t.Errorf("rule %s is missing", id)
		}
	}
	if rules[0].ID != "P1" || rules[len(rules)-1].ID != "H6" {
		t.Errorf("the rules run from %s to %s", rules[0].ID, rules[len(rules)-1].ID)
	}
	for _, r := range rules {
		if r.ID == "P3" {
			want := []string{"version-bad-pattern", "version-unknown-field", "version-trunk-empty"}
			if strings.Join(r.Entries, " ") != strings.Join(want, " ") {
				t.Errorf("P3 names %v", r.Entries)
			}
		}
	}
	if n := len(c.Names()); n != 103 {
		t.Errorf("%d entries", n)
	}
}

// TestInPlaceEntry is T7: a read-in-place entry skips in the copy and runs
// with TABLO_CORPUS.
func TestInPlaceEntry(t *testing.T) {
	t.Setenv("TABLO_CORPUS", "")
	c := Open(t)
	var f fake
	f.run(func() { c.Entry(&f, "tableaux-tooling") })
	if f.skipped == "" || f.failed != "" || !strings.Contains(f.skipped, "TABLO_CORPUS") {
		t.Errorf("without the variable: skipped %q, failed %q", f.skipped, f.failed)
	}

	other := t.TempDir()
	dir := filepath.Join(other, "entries", "plan")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "entry: plan\nlanguage: 0.3.1\nsource: { repository: ../../.., ref: corpus/tooling }\nvalid: true\n"
	if err := os.WriteFile(filepath.Join(dir, "expected.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TABLO_CORPUS", other)
	c = Open(t)
	if c.Vendored || c.Origin != other {
		t.Fatalf("the corpus is %+v", c)
	}
	var g fake
	var e *Entry
	g.run(func() { e = c.Entry(&g, "plan") })
	if g.skipped != "" || g.failed != "" || e == nil {
		t.Fatalf("with the variable: skipped %q, failed %q", g.skipped, g.failed)
	}
	if !e.InPlace || e.Ref != "corpus/tooling" || e.Repo != filepath.Dir(other) {
		t.Errorf("the entry reads as repo %q ref %q in place %v", e.Repo, e.Ref, e.InPlace)
	}
	if got := c.Names(); len(got) != 1 || got[0] != "plan" {
		t.Errorf("names %v", got)
	}
	if _, err := os.Stat(filepath.Join(other, "build")); err == nil {
		t.Error("the corpus directory gained a build directory")
	}
}

// TestMissingTool is T8: a missing tool skips, and fails under CI.
func TestMissingTool(t *testing.T) {
	t.Setenv("PATH", "")
	t.Setenv("CI", "")
	var f fake
	f.run(func() { Open(&f) })
	if f.skipped == "" || f.failed != "" {
		t.Errorf("without CI: skipped %q, failed %q", f.skipped, f.failed)
	}
	t.Setenv("CI", "true")
	var g fake
	g.run(func() { Open(&g) })
	if g.failed == "" || g.skipped != "" {
		t.Errorf("under CI: skipped %q, failed %q", g.skipped, g.failed)
	}
}
