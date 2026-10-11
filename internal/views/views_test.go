package views

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
)

// commands names the four structural views as the plumbing command does.
var commands = []string{"gates", "task", "authority", "assignment"}

// everyView returns the JSON of the four views of a project at the three
// levels, with the gate and task definitions of every task and the tree
// views of every person, in one fixed order.
func everyView(t testing.TB, f *derive.Facts) []byte {
	t.Helper()
	var b bytes.Buffer
	for _, level := range []Level{Glance, Detail, Provenance} {
		all := queries(f, level)
		for _, command := range commands {
			for _, q := range all[command] {
				b.WriteString(command + " " + q.Task + " " + q.Person + " " + levels[level] + "\n")
				b.Write(encoded(t, viewOf(f, command, q)))
			}
		}
	}
	return b.Bytes()
}

// walk calls visit for every object of decoded JSON, with the key that
// holds it.
func walk(key string, v any, visit func(key string, object map[string]any)) {
	switch v := v.(type) {
	case map[string]any:
		visit(key, v)
		for k, item := range v {
			walk(k, item, visit)
		}
	case []any:
		for _, item := range v {
			walk(key, item, visit)
		}
	}
}

// T20: the views stand without history. A tree and an unborn branch have no
// commit: every commit is null, every list of commands is empty, and every
// task reads proposed for want of a history.
func TestNoHistory(t *testing.T) {
	repo, _, _ := cases(t)
	tree := gitRun(t, repo, "rev-parse", "HEAD^{tree}")
	unborn := filepath.Join(t.TempDir(), "unborn")
	copyTree(t, filepath.Join(repo, ".tableaux"), filepath.Join(unborn, ".tableaux"))
	gitRun(t, unborn, "init", "-q", "-b", "main")
	s := envelopeSchema(t)
	for name, f := range map[string]*derive.Facts{
		"a tree":           derived(t, load.Options{}, repo, tree, "main"),
		"an unborn branch": derived(t, load.Options{}, unborn, "", "main"),
	} {
		if f.Refused() != nil || f.Log() != nil {
			t.Fatalf("%s is refused as %+v, or has the pass %v", name, f.Refused(), f.Log())
		}
		for _, command := range commands {
			for _, q := range queries(f, Provenance)[command] {
				data := encoded(t, viewOf(f, command, q))
				if err := s.Validate(envelope(t, command, q, data)); err != nil {
					t.Errorf("%s: %s %+v does not validate: %v", name, command, q, err)
				}
				var decoded any
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatal(err)
				}
				walk("", decoded, func(key string, object map[string]any) {
					if _, ok := object["hash"]; ok {
						t.Errorf("%s: %s %+v: %s holds a commit", name, command, q, key)
					}
					for _, k := range []string{"commit", "status_commit", "own_commit", "pinned", "set_by"} {
						if v, ok := object[k]; ok && v != nil && v != "" {
							t.Errorf("%s: %s %+v: %s.%s is %v", name, command, q, key, k, v)
						}
					}
					if list, ok := object["commands"].([]any); ok && len(list) != 0 {
						t.Errorf("%s: %s %+v: the commands are %v", name, command, q, list)
					}
				})
			}
		}
		for _, id := range f.Order() {
			task := Task(f, Query{Task: id, Level: Provenance})
			if a := task.Authorisation; task.Authorised || a.Why != "no_history" || a.Commit != nil || a.Way != "" || a.By != "" || a.Email != "" || a.Judges == nil || a.DiffersFromTrunk || a.Uncommitted {
				t.Errorf("%s: %s: the authorisation is %+v; want it proposed for want of a history", name, id, a)
			}
			if task.Status.Date != "" || task.Status.Recorder != "" || task.EventCount != 0 || len(task.Events) != 0 || len(task.Models) != 0 || task.Commands == nil {
				t.Errorf("%s: %s: the status is dated %q, with %d events", name, id, task.Status.Date, task.EventCount)
			}
		}
		// What the files state stands: the status, the junction and the review it owes.
		leaf := Task(f, Query{Task: "b2c9", Level: Provenance})
		if leaf.Status.Gate != "design" || leaf.Status.Note != "From both" || len(leaf.Junctions) != 3 || leaf.Junctions[1].Reviewer != "dan@example.org" ||
			len(leaf.Sources) != 1 || len(leaf.Reviews) != 1 || leaf.Reviews[0].Effect != "none" || leaf.Reviews[0].Commit != nil {
			t.Errorf("%s: b2c9 reads %+v with the reviews %+v", name, leaf.Status, leaf.Reviews)
		}
		d := Authority(f, Query{Level: Provenance})
		if len(d.Rows) != 3 || len(d.Deciding) != 0 || len(d.Undecided) != 1 || d.Undecided[0].Why != "no_history" || len(d.Undecided[0].Tasks) != 3 || len(d.Commands) != 0 {
			t.Errorf("%s: the authority view holds %d rows, the undecided %+v and the commands %v", name, len(d.Rows), d.Undecided, d.Commands)
		}
		for _, row := range d.Rows {
			if !row.Proposed || row.Uncommitted || row.DiffersFromTrunk {
				t.Errorf("%s: the row of %s is %+v", name, row.ID, row)
			}
		}
		if g := Gates(f, Query{Level: Provenance}); len(g.Files) != 2 || g.Files[0].Commit != nil || g.Files[1].Commit != nil || g.Commands == nil {
			t.Errorf("%s: the gate definition's files are %+v", name, g.Files)
		}
		if got := peopleOf(Assignment(f, Query{})); len(got) != 2 || got[0] != "pat 3 2 0" || got[1] != "dan 0 0 0" {
			t.Errorf("%s: the people are %v", name, got)
		}
	}

	// A project without history that reads a subproject with one: what the
	// subproject's history gives stands, as the Derivation reads it, and the
	// project's own commits are null.
	station := builtAt(t, "weather-station")
	f := derived(t, load.Options{}, station, gitRun(t, station, "rev-parse", "main^{tree}"), "")
	firmware := labels(t, "weather-station.firmware")
	image := Task(f, Query{Task: "c07d", Level: Provenance})
	if image.Authorised || image.Authorisation.Why != "no_history" || image.OwnCommit != nil || len(image.Commands) != 0 ||
		image.StatusCommit == nil || image.StatusCommit.Hash != firmware["F2"] || image.Status.Date != "2026-09-17" {
		t.Errorf("without history c07d reads %+v, decided by %+v", image.Status, image.StatusCommit)
	}
	if l := image.Linkages; len(l) != 1 || l[0].Pinned == nil || l[0].SetBy != nil || l[0].Recorded != "" {
		t.Errorf("without history the linkage of c07d is %+v", l)
	}
	if board := Task(f, Query{Task: "9f31", Level: Provenance}); board.StatusCommit != nil || board.Status.Date != "" || board.EventCount != 0 {
		t.Errorf("without history 9f31 reads %+v", board.Status)
	}
}

// T20: the views stand on the working tree. An edited task file reads
// uncommitted on its row and in its authorisation.
func TestWorkingTree(t *testing.T) {
	repo := clone(t, "weather-station")
	clean := derived(t, load.Options{}, repo, "", "")
	for _, row := range Authority(clean, Query{}).Rows {
		if row.Uncommitted || row.DiffersFromTrunk {
			t.Errorf("the clean working tree reads %s as %+v", row.ID, row)
		}
	}
	file := filepath.Join(repo, ".tableaux", "tasks", "7b2e.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), "title: Gateway", "title: Gateway, revised", 1)
	if edited == string(data) {
		t.Fatal("the task file of 7b2e states no such title")
	}
	if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	f := derived(t, load.Options{}, repo, "", "")
	hashes := labels(t, "weather-station")
	for _, row := range Authority(f, Query{}).Rows {
		if want := row.ID == "7b2e"; row.Uncommitted != want || row.DiffersFromTrunk != want {
			t.Errorf("%s reads uncommitted %v and differing %v; want %v", row.ID, row.Uncommitted, row.DiffersFromTrunk, want)
		}
		if row.ID == "7b2e" && (row.Title != "Gateway, revised" || row.Proposed) {
			t.Errorf("the edited row is %+v; want the title of the file on disk, authorised by the history", row)
		}
	}
	task := Task(f, Query{Task: "7b2e", Level: Provenance})
	if a := task.Authorisation; !a.Uncommitted || !a.DiffersFromTrunk || a.Commit == nil || a.Commit.Hash != hashes["W1"] || !task.Authorised || task.Task.Title != "Gateway, revised" {
		t.Errorf("the edited task's authorisation is %+v", a)
	}
	if other := Task(f, Query{Task: "9f31", Level: Provenance}); other.Authorisation.Uncommitted || other.Status.Uncommitted {
		t.Errorf("an untouched task reads uncommitted: %+v", other.Authorisation)
	}
	// An edited status file reads uncommitted on the status, and the history stays HEAD's.
	if err := os.WriteFile(filepath.Join(repo, ".tableaux", "status", "9f31.yaml"), []byte("gate: performance\nstate: nominal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	board := Task(derived(t, load.Options{}, repo, "", ""), Query{Task: "9f31", Level: Provenance})
	if !board.Status.Uncommitted || board.Status.Gate != "performance" || board.Authorisation.Uncommitted || len(board.Commands) != 1 {
		t.Errorf("the edited status is %+v", board.Status)
	}
}

// T21: deterministic and safe to share. Two clones at two paths under two
// time zones give equal bytes for each view, and fifty goroutines call the
// four functions on one Facts.
func TestDeterministic(t *testing.T) {
	for _, name := range []string{"weather-station", "junction-kinds"} {
		t.Setenv("TZ", "Pacific/Kiritimati")
		first := derived(t, load.Options{}, clone(t, name), "main", "")
		a := everyView(t, first)
		t.Setenv("TZ", "America/Los_Angeles")
		second := derived(t, load.Options{}, clone(t, name), "main", "")
		if first.Project().Where.GitDir == second.Project().Where.GitDir {
			t.Fatalf("%s: the two clones share a path", name)
		}
		if b := everyView(t, second); !bytes.Equal(a, b) {
			x, y := strings.Split(string(a), "\n"), strings.Split(string(b), "\n")
			for i := 0; i < len(x) && i < len(y); i++ {
				if x[i] != y[i] {
					t.Fatalf("%s: two clones give different views; line %d:\n one %s\n two %s", name, i+1, x[i], y[i])
				}
			}
			t.Fatalf("%s: two clones give views of %d and %d bytes", name, len(a), len(b))
		}
		if len(a) < 100000 || bytes.Contains(a, []byte(first.Project().Where.GitDir)) || bytes.Contains(a, []byte(filepath.Dir(first.Project().Where.Top))) {
			t.Errorf("%s: the views hold %d bytes, or a path of the host", name, len(a))
		}
		// The same facts and the same query give the same bytes again.
		if again := everyView(t, first); !bytes.Equal(a, again) {
			t.Errorf("%s: a second call gives other bytes", name)
		}

		// Fifty goroutines read one Facts that has computed nothing yet.
		shared := derived(t, load.Options{}, builtAt(t, name), "main", "")
		views := make([][]byte, 50)
		var wg sync.WaitGroup
		for i := range views {
			wg.Add(1)
			go func() {
				defer wg.Done()
				views[i] = everyView(t, shared)
			}()
		}
		wg.Wait()
		for i, v := range views {
			if !bytes.Equal(v, a) {
				t.Fatalf("%s: goroutine %d reads other views than a lone reader", name, i)
			}
		}
	}
}

// T21: pure. The package view imports nothing, and internal/views imports
// neither os nor a package that reads a file or runs a process, and reads
// no clock: of the package time it names no Now, Since, Until or Local.
func TestImports(t *testing.T) {
	for dir, allowed := range map[string]func(string) bool{
		filepath.Join("..", "..", "view"): func(string) bool { return false },
		".": func(imported string) bool {
			switch imported {
			case "path", "sort", "strconv", "strings", "time",
				"github.com/nbyoung/tablo/view", "github.com/nbyoung/tablo/internal/derive",
				"github.com/nbyoung/tablo/internal/history", "github.com/nbyoung/tablo/internal/model":
				return true
			}
			return false
		},
	} {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil || len(files) == 0 {
			t.Fatalf("the files of %s: %v, %v", dir, files, err)
		}
		checked := 0
		for _, name := range files {
			if strings.HasSuffix(name, "_test.go") {
				continue
			}
			checked++
			parsed, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, spec := range parsed.Imports {
				if imported, _ := strconv.Unquote(spec.Path.Value); !allowed(imported) {
					t.Errorf("%s imports %s", name, imported)
				}
			}
			ast.Inspect(parsed, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "time" {
					switch sel.Sel.Name {
					case "Unix", "FixedZone", "RFC3339", "Parse", "DateOnly":
					default:
						t.Errorf("%s names time.%s", name, sel.Sel.Name)
					}
				}
				return true
			})
		}
		if checked < 5 {
			t.Errorf("%s holds %d files", dir, checked)
		}
	}
}
