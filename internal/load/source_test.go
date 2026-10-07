package load

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// labels returns the commits the build names in <entry>.labels.txt.
func labels(t testing.TB, entry string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(corpus(t), entry+".labels.txt"))
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

// clone copies a built corpus repository into a temporary directory, for a
// test that changes it, and returns the copy's path.
func clone(t testing.TB, name string) string {
	t.Helper()
	to := filepath.Join(t.TempDir(), name)
	copyTree(t, entry(t, name), to)
	return to
}

// write writes a file below dir, with the directories it needs.
func write(t testing.TB, dir, name, text string) {
	t.Helper()
	file := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// forget clears, in every project p reaches, what says where the Loader read
// it: the Location and each link's checkout. Two readings of the same files
// are then deep-equal. The test owns the projects it changes so.
func forget(p *model.Project) {
	seen := map[*model.Project]bool{}
	var walk func(*model.Project)
	walk = func(p *model.Project) {
		if p == nil || seen[p] {
			return
		}
		seen[p] = true
		p.Where = model.Location{}
		for _, l := range p.Links {
			l.Checkout = ""
			walk(l.Project)
		}
	}
	walk(p)
}

// diff returns the path of the first difference between two values, or "".
// It follows pointers once each, so a graph that loops ends.
func diff(a, b any) string {
	type pair struct{ a, b uintptr }
	seen := map[pair]bool{}
	var walk func(path string, a, b reflect.Value) string
	walk = func(path string, a, b reflect.Value) string {
		if a.IsValid() != b.IsValid() {
			return path + ": one is absent"
		}
		if !a.IsValid() {
			return ""
		}
		if a.Type() != b.Type() {
			return fmt.Sprintf("%s: %s against %s", path, a.Type(), b.Type())
		}
		switch a.Kind() {
		case reflect.Pointer:
			if a.IsNil() || b.IsNil() {
				if a.IsNil() != b.IsNil() {
					return path + ": one is nil"
				}
				return ""
			}
			p := pair{a.Pointer(), b.Pointer()}
			if seen[p] {
				return ""
			}
			seen[p] = true
			return walk(path, a.Elem(), b.Elem())
		case reflect.Interface:
			if a.IsNil() || b.IsNil() {
				if a.IsNil() != b.IsNil() {
					return path + ": one is nil"
				}
				return ""
			}
			return walk(path, a.Elem(), b.Elem())
		case reflect.Struct:
			for i := 0; i < a.NumField(); i++ {
				if d := walk(path+"."+a.Type().Field(i).Name, a.Field(i), b.Field(i)); d != "" {
					return d
				}
			}
			return ""
		case reflect.Slice:
			if a.Len() != b.Len() {
				return fmt.Sprintf("%s: %d items against %d", path, a.Len(), b.Len())
			}
			for i := 0; i < a.Len(); i++ {
				if d := walk(fmt.Sprintf("%s[%d]", path, i), a.Index(i), b.Index(i)); d != "" {
					return d
				}
			}
			return ""
		case reflect.Map:
			if a.Len() != b.Len() {
				return fmt.Sprintf("%s: %d keys against %d", path, a.Len(), b.Len())
			}
			for _, k := range a.MapKeys() {
				if d := walk(fmt.Sprintf("%s[%v]", path, k), a.MapIndex(k), b.MapIndex(k)); d != "" {
					return d
				}
			}
			return ""
		}
		if !reflect.DeepEqual(a.Interface(), b.Interface()) {
			return fmt.Sprintf("%s: %v against %v", path, a.Interface(), b.Interface())
		}
		return ""
	}
	return walk("", reflect.ValueOf(a), reflect.ValueOf(b))
}

// link returns the link of p whose URL is url, and fails the test when p has none.
func link(t testing.TB, p *model.Project, url string) *model.Link {
	t.Helper()
	for _, l := range p.Links {
		if l.URL == url {
			return l
		}
	}
	t.Fatalf("the project has no link %q among %d", url, len(p.Links))
	return nil
}

// The layout names four kinds of file; every other path is stray.
func TestLayout(t *testing.T) {
	tests := []struct {
		path string
		kind int
		id   string
	}{
		{"version.yaml", versionFile, ""},
		{"gates.yaml", gatesFile, ""},
		{"tasks/a1c0.yaml", taskFile, "a1c0"},
		{"tasks/leaf-two.yaml", taskFile, "leaf-two"},
		{"status/9f31.yaml", statusFile, "9f31"},
		{"tasks/g666.yml", strayFile, ""},
		{"tasks/sub/a1c0.yaml", strayFile, ""},
		{"notes.txt", strayFile, ""},
		{"other/a1c0.yaml", strayFile, ""},
		{"a1c0.yaml", strayFile, ""},
		{"tasks", strayFile, ""},
	}
	for _, tt := range tests {
		if kind, id := layout(tt.path); kind != tt.kind || id != tt.id {
			t.Errorf("layout(%q) = %d, %q; want %d, %q", tt.path, kind, id, tt.kind, tt.id)
		}
	}
}

// T10: a revision and the working tree give one model. Every repository the
// corpus builds stands clean at its HEAD, so the two loads are deep-equal but
// for where the Loader read them.
func TestRevisionEqualsWorkingTree(t *testing.T) {
	for _, name := range built(t) {
		// Two Loaders, so that forget changes no project the other holds, over
		// one empty cache, which a link without a clone names.
		cache := t.TempDir()
		head := load(t, New(Options{CacheDir: cache}), entry(t, name), "HEAD")
		tree := load(t, New(Options{CacheDir: cache}), entry(t, name), "")
		if head.Where.Worktree || !tree.Where.Worktree || head.Where.Ref != "HEAD" || tree.Where.Ref != "" ||
			head.Where.Commit == "" || head.Where.Commit != tree.Where.Commit || head.Where.GitDir != tree.Where.GitDir ||
			head.Where.Top != entry(t, name) || tree.Where.Top != head.Where.Top || head.Where.GitDir != filepath.Join(entry(t, name), ".git") {
			t.Errorf("%s: read at %+v and at %+v", name, head.Where, tree.Where)
		}
		forget(head)
		forget(tree)
		if d := diff(head, tree); d != "" {
			t.Errorf("%s: HEAD and the working tree differ at %s", name, d)
		}
		if !reflect.DeepEqual(head, tree) {
			t.Errorf("%s: HEAD and the working tree are not deep-equal", name)
		}
	}
}

// The working tree is what `git add -A` would record: a path under .tableaux
// that the ignore rules exclude is neither read nor stray, whether its name is
// one the layout names or not, and a tracked file stays read whatever the
// rules say.
func TestWorkingTreeLeavesIgnoredPathsUnread(t *testing.T) {
	repo := clone(t, "subproject-same-repository")
	write(t, repo, ".gitignore", "*.swp\nlocal/\ndddd.yaml\nb2c9.yaml\n")
	write(t, repo, ".tableaux/tasks/dddd.yaml", "title: Ignored\n")
	write(t, repo, ".tableaux/tasks/.a1c0.yaml.swp", "An editor's file.\n")
	write(t, repo, ".tableaux/local/notes.md", "An ignored directory.\n")
	write(t, repo, ".tableaux/tasks/cccc.yaml", "title: Untracked and not ignored\n")
	write(t, repo, ".tableaux/scratch.md", "A stray path, not ignored.\n")
	write(t, repo, "lib/.tableaux/tasks/dddd.yaml", "title: Ignored in a directory project\n")
	l := loader(t, Options{})
	before := load(t, l, repo, "HEAD")
	tree := load(t, l, repo, "")
	for _, id := range before.TaskIDs() {
		if tree.Tasks[id] == nil {
			t.Errorf("the working tree drops the tracked task %s", id)
		}
	}
	if tree.Tasks["dddd"] != nil || tree.Tasks["cccc"] == nil {
		t.Errorf("the working tree: tasks %v", tree.TaskIDs())
	}
	if want := []string{"scratch.md"}; !reflect.DeepEqual(tree.Stray, want) {
		t.Errorf("stray: %v, want %v", tree.Stray, want)
	}
	// From a directory below the root, and in a project a link reads from disk.
	if lib := load(t, l, filepath.Join(repo, "lib"), ""); lib.Tasks["dddd"] != nil || len(lib.Stray) != 0 {
		t.Errorf("the project in lib: tasks %v, stray %v", lib.TaskIDs(), lib.Stray)
	}
	for _, link := range tree.Links {
		if link.Form == model.Directory && link.Project != nil && link.Project.Tasks["dddd"] != nil {
			t.Errorf("the linked project in %s reads the ignored task", link.Project.Where.Dir)
		}
	}
}

// T11: the working tree shows what is not committed, and a revision does not.
func TestWorkingTreeShowsUncommitted(t *testing.T) {
	repo := clone(t, "weather-station")
	write(t, repo, ".tableaux/status/9f31.yaml", "gate: design\nstate: nominal\nnote: Edited and not committed\n")
	write(t, repo, ".tableaux/tasks/ffff.yaml", "title: Untracked\n")
	write(t, repo, ".tableaux/scratch/notes.md", "A stray path, untracked.\n")
	if err := os.Remove(filepath.Join(repo, ".tableaux", "tasks", "7b2e.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("a1c0.yaml", filepath.Join(repo, ".tableaux", "tasks", "eeee.yaml")); err != nil {
		t.Skipf("the host makes no symbolic link: %v", err)
	}
	l := loader(t, Options{})
	tree, head := load(t, l, repo, ""), load(t, l, repo, "HEAD")
	if got := strings.Join(tree.TaskIDs(), " "); got != "3c5d 4e2b 9f31 a1c0 c07d ffff" {
		t.Errorf("the working tree: tasks %s", got)
	}
	if got := strings.Join(head.TaskIDs(), " "); got != "3c5d 4e2b 7b2e 9f31 a1c0 c07d" {
		t.Errorf("HEAD: tasks %s", got)
	}
	if tree.Statuses["9f31"].Note.V != "Edited and not committed" || head.Statuses["9f31"].Note.V == "Edited and not committed" {
		t.Errorf("the note: %q in the working tree, %q at HEAD", tree.Statuses["9f31"].Note.V, head.Statuses["9f31"].Note.V)
	}
	// A symbolic link is stray on disk as it is in a tree, and stays unread.
	if want := []string{"scratch/notes.md", "tasks/eeee.yaml"}; !reflect.DeepEqual(tree.Stray, want) || head.Stray != nil {
		t.Errorf("stray: %v in the working tree, want %v; %v at HEAD", tree.Stray, want, head.Stray)
	}
	if got := codes(tree.Diagnostics); !reflect.DeepEqual(got, []string{"scratch/notes.md warning L4", "tasks/eeee.yaml warning L4"}) {
		t.Errorf("the working tree: %v", got)
	}
	if tree.Where.Commit != head.Where.Commit || tree.Where.Commit != labels(t, "weather-station")["W13"] {
		t.Errorf("the working tree reads beside %s, HEAD at %s", tree.Where.Commit, head.Where.Commit)
	}

	// Once committed, the revision holds the same, the symbolic link included.
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "Commit the edits")
	head = load(t, loader(t, Options{}), repo, "HEAD")
	tree = load(t, loader(t, Options{}), repo, "")
	forget(head)
	forget(tree)
	if d := diff(head, tree); d != "" {
		t.Errorf("after the commit, HEAD and the working tree differ at %s", d)
	}
}

// T12: any revision reads without a checkout.
func TestAnyRevision(t *testing.T) {
	hashes := labels(t, "weather-station")
	l := loader(t, Options{})
	station := entry(t, "weather-station")
	for label, tasks := range map[string]int{"W1": 3, "W6": 6} {
		p := load(t, l, station, hashes[label])
		if len(p.Tasks) != tasks || p.Where.Commit != hashes[label] || p.Where.Ref != hashes[label] || p.Where.Worktree {
			t.Errorf("at %s: %d tasks, want %d; read at %+v", label, len(p.Tasks), tasks, p.Where)
		}
	}
	short := hashes["W1"][:10]
	if p := load(t, l, station, short); len(p.Tasks) != 3 || p.Where.Commit != hashes["W1"] || p.Where.Ref != short {
		t.Errorf("at the short hash: %d tasks at %+v", len(p.Tasks), p.Where)
	}

	repo := clone(t, "weather-station")
	branch := gitRun(t, repo, "rev-parse", "sensor-board")
	gitRun(t, repo, "tag", "first", hashes["W1"])
	gitRun(t, repo, "tag", "-a", "-m", "An annotated tag", "sixth", hashes["W6"])
	parent := gitRun(t, repo, "rev-parse", "main~1")
	for ref, want := range map[string]string{"sensor-board": branch, "first": hashes["W1"], "sixth": hashes["W6"], "main~1": parent, "refs/heads/main": hashes["W13"]} {
		if p := load(t, l, repo, ref); p.Where.Commit != want || p.Where.Ref != ref || !p.Exists {
			t.Errorf("at %s: read at %+v, want the commit %s", ref, p.Where, want)
		}
	}

	// The tree a pre-commit hook writes: no commit, and the files of the index.
	write(t, repo, ".tableaux/tasks/ffff.yaml", "title: Staged\n")
	gitRun(t, repo, "add", "-A")
	tree := gitRun(t, repo, "write-tree")
	staged := load(t, l, repo, tree)
	if staged.Where.Commit != "" || staged.Where.Ref != tree || len(staged.Tasks) != 7 || staged.Tasks["ffff"] == nil {
		t.Errorf("at the tree: %d tasks at %+v", len(staged.Tasks), staged.Where)
	}
	// Its submodule reads at the gitlink of the tree, as at a commit.
	if firmware := link(t, staged, "firmware"); firmware.Problem != model.Resolved || firmware.Commit != labels(t, "weather-station.firmware")["F2"] {
		t.Errorf("at the tree: firmware %+v", firmware)
	}

	// A bare clone serves, and has no clone of the submodule.
	bare := filepath.Join(t.TempDir(), "station.git")
	gitRun(t, repo, "clone", "-q", "--bare", station, bare)
	p := load(t, l, bare, "main")
	if len(p.Tasks) != 6 || p.Where.Top != "" || p.Where.Commit != hashes["W13"] || p.Where.GitDir == "" {
		t.Errorf("the bare clone: %d tasks at %+v", len(p.Tasks), p.Where)
	}
	if firmware := link(t, p, "firmware"); firmware.Form != model.Submodule || firmware.Problem != model.NoClone || firmware.Project != nil {
		t.Errorf("the bare clone: firmware %+v", firmware)
	}
	if p := load(t, l, bare, hashes["W1"]); len(p.Tasks) != 3 {
		t.Errorf("the bare clone at W1: %d tasks", len(p.Tasks))
	}
}

// T13: the four errors, and nothing else is one.
func TestLoadErrors(t *testing.T) {
	station := entry(t, "weather-station")
	outside := t.TempDir()
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(outside))
	bare := filepath.Join(t.TempDir(), "station.git")
	gitRun(t, outside, "clone", "-q", "--bare", station, bare)
	nowhere := filepath.Join(outside, "no-such-git")
	tests := []struct {
		name    string
		options Options
		src     Source
		want    error
	}{
		{"a directory outside any repository", Options{}, Source{Dir: outside}, ErrNoRepository},
		{"a directory outside any repository, at a revision", Options{}, Source{Dir: outside, Ref: "main"}, ErrNoRepository},
		{"a directory that is absent", Options{}, Source{Dir: filepath.Join(outside, "absent")}, ErrNoRepository},
		{"an unknown revision", Options{}, Source{Dir: station, Ref: "no-such-branch"}, ErrNoRef},
		{"a revision that starts with -", Options{}, Source{Dir: station, Ref: "--upload-pack=x"}, ErrNoRef},
		{"a revision that is one dash option", Options{}, Source{Dir: station, Ref: "-h"}, ErrNoRef},
		{"a revision that is a blob", Options{}, Source{Dir: station, Ref: "HEAD:README.md"}, ErrNoRef},
		{"a bare clone with no revision", Options{}, Source{Dir: bare}, ErrNoWorktree},
		{"an executable that is no file", Options{Git: nowhere}, Source{Dir: station}, ErrNoGit},
		{"an executable that is no file, at a revision", Options{Git: nowhere}, Source{Dir: station, Ref: "main"}, ErrNoGit},
	}
	for _, tt := range tests {
		p, err := loader(t, tt.options).Load(t.Context(), tt.src)
		var failure *Error
		if p != nil || !errors.Is(err, tt.want) || !errors.As(err, &failure) {
			t.Errorf("%s: %v, %v; want %v", tt.name, p, err, tt.want)
			continue
		}
		if failure.Dir != tt.src.Dir || failure.Ref != tt.src.Ref || !strings.Contains(failure.Error(), tt.want.Error()) {
			t.Errorf("%s: the error names %q at %q: %v", tt.name, failure.Dir, failure.Ref, failure)
		}
		for _, other := range []error{ErrNoGit, ErrNoRepository, ErrNoRef, ErrNoWorktree} {
			if other != tt.want && errors.Is(err, other) {
				t.Errorf("%s: the error is %v too", tt.name, other)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(station, "--upload-pack=x")); err == nil {
		t.Error("a revision ran as an option")
	}
	// A cancelled load is no load error.
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	var failure *Error
	if _, err := loader(t, Options{}).Load(ctx, Source{Dir: station}); !errors.Is(err, context.Canceled) || errors.As(err, &failure) {
		t.Errorf("a cancelled load: %v", err)
	}
}

// T14: the nearest .tableaux at or above the directory named is the project,
// up to the repository's root (decision 7).
func TestNearestProject(t *testing.T) {
	repo := clone(t, "subproject-same-repository")
	for _, dir := range []string{"lib/x/y", "other/deep"} {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	l := loader(t, Options{})
	tests := []struct {
		from, dir, task string
	}{
		{"", "", "b2c9"},
		{"lib", "lib", "5a00"},
		{"lib/x", "lib", "5a00"},
		{"lib/x/y", "lib", "5a00"},
		{"lib/.tableaux/tasks", "lib", "5a00"},
		{"other", "", "b2c9"},
		{"other/deep", "", "b2c9"},
		{".tableaux", "", "b2c9"},
	}
	for _, ref := range []string{"", "HEAD"} {
		for _, tt := range tests {
			p := load(t, l, filepath.Join(repo, filepath.FromSlash(tt.from)), ref)
			if !p.Exists || p.Where.Dir != tt.dir || p.Tasks[tt.task] == nil || p.Where.Top != repo {
				t.Errorf("from %q at %q: the project in %q with tasks %v at %+v; want the one in %q", tt.from, ref, p.Where.Dir, p.TaskIDs(), p.Where, tt.dir)
			}
		}
	}
	// The directory named need not be in the revision: lib/x is on disk alone.
	// A plan added below it is the nearest in the working tree and not at HEAD.
	write(t, repo, "lib/x/.tableaux/version.yaml", "tableaux: 0.3.1\n")
	if p := load(t, l, filepath.Join(repo, "lib", "x", "y"), ""); p.Where.Dir != "lib/x" || len(p.Tasks) != 0 || p.Version == nil {
		t.Errorf("from lib/x/y with a plan in lib/x: the project in %q", p.Where.Dir)
	}
	if p := load(t, l, filepath.Join(repo, "lib", "x", "y"), "HEAD"); p.Where.Dir != "lib" {
		t.Errorf("from lib/x/y at HEAD: the project in %q", p.Where.Dir)
	}

	// No .tableaux at or above the directory stays a project that does not
	// exist, for the Validator's P1, and is no load error.
	bare := clone(t, "no-tableaux-directory")
	if err := os.MkdirAll(filepath.Join(bare, "src", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"", "HEAD"} {
		p, err := l.Load(t.Context(), Source{Dir: filepath.Join(bare, "src", "deep"), Ref: ref})
		if err != nil || p.Exists || p.Where.Dir != "src/deep" || len(p.Diagnostics) > 0 {
			t.Errorf("no plan above src/deep at %q: %+v, %v", ref, p, err)
		}
	}
}

// Parse reads one file alone, and Compose joins the pieces: the project they
// state has no link, and a path the layout does not name gives no piece.
func TestParseAndCompose(t *testing.T) {
	if Parse("notes.txt", []byte("a: 1\n")) != nil || Parse("tasks/deep/a000.yaml", nil) != nil {
		t.Error("Parse gives a piece for a path the layout does not name")
	}
	task := []byte("title: Leaf\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 1 }\n" +
		"junctions:\n  design: { subproject: { url: lib } }\n")
	pieces := []*Piece{
		Parse("gates.yaml", []byte("gates:\n  - { key: undefined }\n  - { key: design }\n")),
		Parse("status/b2c9.yaml", []byte("gate: design\n")),
		Parse("tasks/b2c9.yaml", task),
		Parse("tasks/e4a1.yaml", []byte("title: [unclosed\n")),
		Parse("version.yaml", []byte("tableaux: 9.0.0\n")),
	}
	where := model.Location{Dir: "lib", Commit: "c0ffee"}
	p := Compose(where, pieces, []string{"notes.txt"})
	if !p.Exists || p.Where != where || len(p.Files) != 5 || !reflect.DeepEqual(p.Stray, []string{"notes.txt"}) {
		t.Fatalf("Compose gives %+v", p)
	}
	if len(p.Gating.Gates) != 2 || p.Statuses["b2c9"].Gate.V != "design" || p.Version.Accepted || p.Tasks["e4a1"] != nil {
		t.Errorf("Compose builds the gating %+v, the status %+v, the version %+v and the task %+v",
			p.Gating, p.Statuses["b2c9"], p.Version, p.Tasks["e4a1"])
	}
	leaf := p.Tasks["b2c9"]
	if leaf == nil || leaf.Junctions[0].Subproject == nil || leaf.Junctions[0].Subproject.Link != nil || p.Links != nil {
		t.Errorf("Compose gives the task %+v and the links %v; want a subproject field with no link", leaf, p.Links)
	}
	var codes []string
	for _, d := range p.Diagnostics {
		codes = append(codes, d.Pos.File+" "+d.Code+" "+d.Task)
	}
	if want := []string{"notes.txt L4 ", "tasks/e4a1.yaml L1 e4a1", "version.yaml P4 "}; !reflect.DeepEqual(codes, want) {
		t.Errorf("the diagnostics are %q; want %q", codes, want)
	}
	// The same pieces compose twice: a Piece is immutable.
	if q := Compose(where, pieces, []string{"notes.txt"}); !reflect.DeepEqual(p, q) {
		t.Error("two compositions of the same pieces differ")
	}
	if empty := Compose(where, nil, nil); empty.Exists || empty.Tasks == nil {
		t.Errorf("Compose of nothing gives %+v; want a project that does not exist", empty)
	}
}
