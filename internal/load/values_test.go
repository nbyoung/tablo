package load

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/model"
)

// T2: an id is its source text at the four places a task file names one, and
// nowhere else.
func TestIDsAreSourceText(t *testing.T) {
	p := load(t, loader(t, Options{}), broken(t), "")
	id := func(v *model.Value, text string, line, col int, read model.Kind) {
		t.Helper()
		if v == nil {
			t.Errorf("the id %s at %d:%d is absent", text, line, col)
			return
		}
		if v.Kind != model.String || v.Text != text || v.Pos.Line != line || v.Pos.Col != col || v.Read != read || v.Quoted {
			t.Errorf("id: kind %s text %q at %d:%d read %s quoted %v; want string %q at %d:%d read %s, not quoted",
				v.Kind, v.Text, v.Pos.Line, v.Pos.Col, v.Read, v.Quoted, text, line, col, read)
		}
		if plain, ok := v.Plain().(string); !ok || plain != text {
			t.Errorf("id %s: Plain gives %#v, want the string", text, v.Plain())
		}
	}
	f555 := p.Tasks["f555"]
	if f555 == nil || len(f555.Requires) != 3 || len(f555.Junctions) != 1 || f555.Parent == nil {
		t.Fatalf("f555: %+v", f555)
	}
	id(f555.Requires[0].ID.Node, "07e0", 6, 11, model.Float)
	id(f555.Requires[1].ID.Node, "40e8", 7, 11, model.Float)
	id(f555.Requires[2].Subproject.ID.Node, "1000", 8, 35, model.Int)
	id(f555.Junctions[0].Subproject.ID.Node, "0010", 10, 42, model.Int)
	id(f555.Parent.ID.Node, "a000", 11, 15, model.String)
	if f555.Requires[0].ID.V != "07e0" || f555.Junctions[0].Subproject.ID.V != "0010" {
		t.Errorf("f555: the typed ids are %q and %q", f555.Requires[0].ID.V, f555.Junctions[0].Subproject.ID.V)
	}
	e444 := p.Tasks["e444"]
	if e444 == nil || e444.Parent == nil {
		t.Fatalf("e444: %+v", e444)
	}
	id(e444.Parent.ID.Node, "1000", 7, 15, model.Int)

	// The corpus entry: parent 1000, read as an integer, at tasks/1a00.yaml:5:15.
	q := load(t, loader(t, Options{}), entry(t, "unquoted-id"), "HEAD")
	leaf := q.Tasks["1a00"]
	if leaf == nil || leaf.Parent == nil {
		t.Fatalf("unquoted-id: 1a00 is %+v", leaf)
	}
	id(leaf.Parent.ID.Node, "1000", 5, 15, model.Int)
	if leaf.Parent.ID.Node.Pos.File != "tasks/1a00.yaml" || leaf.Parent.ID.V != "1000" {
		t.Errorf("unquoted-id: parent %q in %q", leaf.Parent.ID.V, leaf.Parent.ID.Node.Pos.File)
	}
}

// T3: positions are the value's own, one-based, in code points.
func TestPositions(t *testing.T) {
	at := func(what string, pos model.Pos, file string, line, col int) {
		t.Helper()
		if want := (model.Pos{File: file, Line: line, Col: col}); pos != want {
			t.Errorf("%s at %+v, want %+v", what, pos, want)
		}
	}
	email := load(t, loader(t, Options{}), entry(t, "task-bad-email"), "HEAD")
	at("the assignee", email.Tasks["b2c9"].Assignee.Node.Pos, "tasks/b2c9.yaml", 4, 11)

	station := load(t, loader(t, Options{}), entry(t, "weather-station"), "HEAD")
	// Every plain scalar of gates.yaml starts at its column, counted in code
	// points, so the field after each emoji symbol stands where it is written.
	data, err := os.ReadFile(filepath.Join(entry(t, "weather-station"), ".tableaux", "gates.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	checked := 0
	var walk func(v *model.Value)
	walk = func(v *model.Value) {
		for _, item := range v.Items {
			walk(item)
		}
		for _, field := range v.Fields {
			line := []rune(lines[field.KeyPos.Line-1])
			if got := string(line[field.KeyPos.Col-1:]); !strings.HasPrefix(got, field.Key) {
				t.Errorf("the key %q at %d:%d stands before %q", field.Key, field.KeyPos.Line, field.KeyPos.Col, got)
			}
			walk(field.Value)
		}
		if v.Scalar() && !v.Quoted {
			line := []rune(lines[v.Pos.Line-1])
			if got := string(line[v.Pos.Col-1:]); !strings.HasPrefix(got, v.Text) {
				t.Errorf("the scalar %q at %d:%d stands before %q", v.Text, v.Pos.Line, v.Pos.Col, got)
			}
			checked++
		}
	}
	walk(station.Gating.File.Root)
	if checked < 60 {
		t.Errorf("gates.yaml: %d plain scalars checked", checked)
	}
	for _, gate := range station.Gating.Gates {
		if gate.Symbol.V == "" || gate.Name.Node == nil {
			t.Fatalf("gate %q: symbol %q, name %+v", gate.Key.V, gate.Symbol.V, gate.Name.Node)
		}
		// symbol: <symbol>, name: <name>
		want := gate.Symbol.Node.Pos.Col + len([]rune(gate.Symbol.V)) + len(", name: ")
		if gate.Name.Node.Pos.Col != want || gate.Name.Node.Pos.Line != gate.Symbol.Node.Pos.Line {
			t.Errorf("gate %q: name at column %d, want %d", gate.Key.V, gate.Name.Node.Pos.Col, want)
		}
	}
	// A block scalar stands at its indicator.
	description := station.Tasks["a1c0"].Description.Node
	at("the folded description", description.Pos, "tasks/a1c0.yaml", 3, 14)
	if !description.Quoted || description.Kind != model.String || !strings.HasPrefix(description.Text, "A solar-powered sensor node,") {
		t.Errorf("the folded description: %+v", description)
	}
	// A junction keeps the position of its key, and a block mapping stands at its first key.
	firmware := station.Tasks["c07d"]
	at("the junctions mapping", firmware.File.Root.Get("junctions").Pos, "tasks/c07d.yaml", 8, 3)
	for i, want := range []struct {
		gate      string
		line, col int
	}{{"reliability", 8, 3}, {"implementation", 9, 3}, {"unit", 10, 3}} {
		j := firmware.Junctions[i]
		if j.Gate != want.gate {
			t.Errorf("junction %d is %q, want %q", i, j.Gate, want.gate)
		}
		at("the junction key "+want.gate, j.KeyPos, "tasks/c07d.yaml", want.line, want.col)
		at("the junction entry "+want.gate, j.Node.Pos, "tasks/c07d.yaml", want.line, 19)
	}
}

// T4: one bad file hides nothing. The fixture gives the nine diagnostics of
// broken.expected.txt in order and the model it lists, from the working tree
// and from a commit alike.
func TestBrokenFixture(t *testing.T) {
	expected, err := os.ReadFile(filepath.Join("testdata", "broken.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, line := range strings.Split(string(expected), "\n") {
		if strings.HasPrefix(line, "# What the model holds") {
			break
		}
		if fields := strings.Fields(line); len(fields) == 3 && !strings.HasPrefix(line, "#") {
			want = append(want, strings.Join(fields, " "))
		}
	}
	if len(want) != 9 {
		t.Fatalf("broken.expected.txt lists %d diagnostics, want 9", len(want))
	}
	repo := broken(t)
	check := func(name string, p *model.Project) {
		t.Helper()
		if got := codes(p.Diagnostics); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: diagnostics\n%s\nwant\n%s", name, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		for _, d := range p.Diagnostics {
			// A diagnostic of a task or status file names the task, as the corpus entries of L1 to L3 state it.
			wantTask := ""
			if kind, id := layout(d.Pos.File); kind == taskFile || kind == statusFile {
				wantTask = id
			}
			if d.Task != wantTask || d.Message == "" || d.Gate != "" {
				t.Errorf("%s: %s %s names the task %q, want %q; message %q", name, d.Code, d.Pos.File, d.Task, wantTask, d.Message)
			}
		}
		if got := strings.Join(p.TaskIDs(), " "); got != "a000 c222 d333 e444 f555" {
			t.Errorf("%s: tasks %s", name, got)
		}
		if len(p.Statuses) != 1 || p.Statuses["c222"] == nil {
			t.Errorf("%s: statuses %v", name, p.Statuses)
		}
		if got := strings.Join(p.Stray, " "); got != "notes.txt tasks/g666.yml" {
			t.Errorf("%s: stray %s", name, got)
		}
		// b111 has a file with no root and no task; g666 has neither.
		var paths []string
		for _, f := range p.Files {
			paths = append(paths, f.Path)
			if (f.Root == nil) != (f.Path == "tasks/b111.yaml") {
				t.Errorf("%s: %s has root %v", name, f.Path, f.Root != nil)
			}
		}
		if got := strings.Join(paths, " "); got != "gates.yaml status/c222.yaml tasks/a000.yaml tasks/b111.yaml tasks/c222.yaml tasks/d333.yaml tasks/e444.yaml tasks/f555.yaml version.yaml" {
			t.Errorf("%s: files %s", name, got)
		}
		c222 := p.Tasks["c222"]
		if c222.Assignee.V != "pat@example.org" || c222.Assignee.Node.Pos.Line != 4 || c222.Assignee.Node.Pos.Col != 11 {
			t.Errorf("%s: c222 assignee %q at %+v", name, c222.Assignee.V, c222.Assignee.Node.Pos)
		}
		if c222.Parent.ID.V != "a000" || c222.Parent.ID.Node.Pos.Line != 5 || c222.Parent.ID.Node.Pos.Col != 15 {
			t.Errorf("%s: c222 parent %q at %+v", name, c222.Parent.ID.V, c222.Parent.ID.Node.Pos)
		}
		if len(c222.File.Root.Fields) != 4 || len(c222.Parent.Node.Fields) != 2 {
			t.Errorf("%s: c222 keeps %d and %d fields, want 4 and 2", name, len(c222.File.Root.Fields), len(c222.Parent.Node.Fields))
		}
		if got := p.Tasks["d333"].Title.V; got != "Two documents" {
			t.Errorf("%s: d333 title %q", name, got)
		}
		e444 := p.Tasks["e444"]
		if e444.Assignee.V != "pat@example.org" || e444.Assignee.Node.Pos.Line != 4 || e444.Assignee.Node.Pos.Col != 11 {
			t.Errorf("%s: e444 assignee %q at %+v", name, e444.Assignee.V, e444.Assignee.Node.Pos)
		}
		reviewer := e444.Junctions[0].Reviewer
		if e444.Junctions[0].Gate != "release" || reviewer.Node == nil || reviewer.Node.Kind != model.Null ||
			reviewer.Node.Pos.Line != 6 || reviewer.Node.Pos.Col != 24 || reviewer.V != "" {
			t.Errorf("%s: e444 release.reviewer %+v", name, reviewer.Node)
		}
		note := p.Statuses["c222"].Note
		if note.V != "2026-09-01" || note.Node.Kind != model.String || note.Node.Quoted {
			t.Errorf("%s: the note %q is %s", name, note.V, note.Node.Kind)
		}
		if p.Version == nil || !p.Version.Accepted || p.Gating == nil || len(p.Gating.Gates) != 3 || len(p.Gating.States) != 3 {
			t.Errorf("%s: version %+v, gating %+v", name, p.Version, p.Gating)
		}
	}
	tree := load(t, loader(t, Options{}), repo, "")
	check("the working tree", tree)
	if tree.Where.Commit != "" || !tree.Where.Worktree {
		t.Errorf("an unborn branch: %+v", tree.Where)
	}
	gitRun(t, repo, "add", "-A")
	gitRun(t, repo, "commit", "-q", "-m", "Add the broken fixture")
	check("the commit", load(t, loader(t, Options{}), repo, "HEAD"))
}

// T5: Plain and At serve a schema validator. For every file of every corpus
// repository, Plain equals what yaml.v3 decodes into any, once the numbers
// take one Go type, except at an id; At of each path returns the node.
func TestPlainAgreesWithYAML(t *testing.T) {
	files, ids := 0, 0
	for _, name := range built(t) {
		plan := filepath.Join(entry(t, name), ".tableaux")
		l, err := listDisk(entry(t, name), "")
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range l.files {
			root, parsed, diagnostics := readYAML(c.path, c.data)
			if !parsed || len(diagnostics) > 0 {
				continue // yaml.v3 refuses a repeated key and reads an alias; the fixtures of L1 to L3 hold these
			}
			if kind, id := layout(c.path); kind == taskFile {
				buildTask(id, &model.File{Path: c.path, Root: root})
			}
			var want any
			if err := yaml.Unmarshal(c.data, &want); err != nil {
				t.Errorf("%s: yaml.v3 fails where the Loader reads: %v", filepath.Join(plan, c.path), err)
				continue
			}
			files++
			where := name + "/" + c.path
			ids += agree(t, where, root, root, nil, want)
		}
	}
	if files < 500 {
		t.Errorf("%d files compared; the corpus holds more", files)
	}
	if ids != 1 {
		t.Errorf("%d ids differ from what YAML reads; the corpus holds one, in unquoted-id", ids)
	}
}

// agree compares a value's Plain form with what yaml.v3 decoded, checks that
// At finds the value by its path, and returns how many ids it met that YAML
// reads as no string.
func agree(t *testing.T, where string, root, v *model.Value, path []string, want any) (ids int) {
	t.Helper()
	at := where + " /" + strings.Join(path, "/")
	if root.At(path...) != v {
		t.Errorf("%s: At does not return the node", at)
	}
	got := v.Plain()
	switch {
	case v == nil:
		if want != nil {
			t.Errorf("%s: an empty file against %#v", at, want)
		}
	case v.Kind == model.Map:
		keys := map[string]any{}
		switch m := want.(type) {
		case map[string]any:
			keys = m
		case map[any]any:
			for k, x := range m {
				keys[fmt.Sprint(k)] = x
			}
		default:
			t.Errorf("%s: a mapping against %#v", at, want)
			return 0
		}
		if len(keys) != len(v.Fields) || len(got.(map[string]any)) != len(v.Fields) {
			t.Errorf("%s: %d fields against %d", at, len(v.Fields), len(keys))
		}
		for _, field := range v.Fields {
			x, ok := keys[field.Key]
			if !ok {
				t.Errorf("%s: yaml.v3 has no key %q", at, field.Key)
				continue
			}
			ids += agree(t, where, root, field.Value, append(path[:len(path):len(path)], field.Key), x)
		}
	case v.Kind == model.Seq:
		s, ok := want.([]any)
		if !ok || len(s) != len(v.Items) {
			t.Errorf("%s: a sequence of %d against %#v", at, len(v.Items), want)
			return 0
		}
		for i, item := range v.Items {
			ids += agree(t, where, root, item, append(path[:len(path):len(path)], strconv.Itoa(i)), s[i])
		}
	case v.Kind == model.String && v.Read != model.String:
		// An id: the Loader hands the schema the source text, and YAML a number.
		if got != v.Text {
			t.Errorf("%s: the id gives %#v, want its text %q", at, got, v.Text)
		}
		if _, isString := want.(string); isString {
			t.Errorf("%s: yaml.v3 reads the id as a string, and Read says %s", at, v.Read)
		}
		return 1
	default:
		if a, b := number(got), number(want); !reflect.DeepEqual(a, b) {
			if x, ok := a.(float64); !ok || !math.IsNaN(x) {
				t.Errorf("%s: %#v against %#v", at, got, want)
			}
		}
	}
	return ids
}

// number gives every number one Go type.
func number(x any) any {
	switch n := x.(type) {
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint64:
		return float64(n)
	}
	return x
}
