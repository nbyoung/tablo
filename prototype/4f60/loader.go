package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nbyoung/tablo"
)

// Pos locates a value in a file at the loaded ref.
type Pos struct {
	File      string // relative to .tableaux
	Line, Col int
}

func (p Pos) String() string { return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col) }

// Diagnostic is a finding tied to a position.
type Diagnostic struct {
	Pos Pos
	Msg string
}

func (d Diagnostic) String() string { return d.Pos.String() + ": " + d.Msg }

// Str is a string that remembers where it was written.
type Str struct {
	V   string
	Pos Pos
}

// Task is the typed model of tasks/<id>.yaml.
type Task struct {
	ID          string
	Title       Str
	Description Str
	Assignee    Str
	ParentID    Str
	ParentOrder Str
	References  []Str // urls
	Requires    []Str // ids
	Junctions   map[string]Pos
}

// Status is the typed model of status/<id>.yaml.
type Status struct {
	ID    string
	Gate  Str
	State Str
}

// Project is what the loader returns for one ref.
type Project struct {
	Ref      string
	Commit   string
	Version  Str
	Trunk    Str
	Gates    []Str // gate keys
	Tasks    map[string]*Task
	Statuses map[string]*Status
	Diags    []Diagnostic
}

// Loader reads a repository through git plumbing only.
type Loader struct{ Dir string }

func (l Loader) git(args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"-C", l.Dir}, args...)...)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// Files lists the blobs under .tableaux at ref, relative to .tableaux.
func (l Loader) Files(ref string) ([]string, error) {
	out, err := l.git("ls-tree", "-r", "-z", "--name-only", ref, "--", ".tableaux")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(string(out), "\x00") {
		if f != "" {
			files = append(files, strings.TrimPrefix(f, ".tableaux/"))
		}
	}
	sort.Strings(files)
	return files, nil
}

// Load reads and parses every file under .tableaux at ref.
func (l Loader) Load(ref string) (*Project, error) {
	commit, err := l.git("rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return nil, err
	}
	files, err := l.Files(ref)
	if err != nil {
		return nil, err
	}
	p := &Project{Ref: ref, Commit: strings.TrimSpace(string(commit)),
		Tasks: map[string]*Task{}, Statuses: map[string]*Status{}}
	if len(files) == 0 {
		p.diag(Pos{File: "."}, "no .tableaux directory at this ref")
		return p, nil
	}
	seen := map[string]*Node{}
	for _, f := range files {
		blob, err := l.git("cat-file", "blob", ref+":.tableaux/"+f)
		if err != nil {
			return nil, err
		}
		n, err := ParseYAML(string(blob))
		if err != nil {
			se := err.(*SyntaxError)
			p.diag(Pos{f, se.Line, se.Col}, "syntax: "+se.Msg)
			continue
		}
		seen[f] = n
	}
	for _, f := range files {
		n := seen[f]
		if n == nil {
			continue
		}
		id := strings.TrimSuffix(path.Base(f), ".yaml")
		switch {
		case f == "version.yaml":
			p.readVersion(n)
		case f == "gates.yaml":
			for _, g := range n.Get("gates").itemsOf() {
				p.Gates = append(p.Gates, str(f, g.Get("key")))
			}
		case strings.HasPrefix(f, "tasks/"):
			p.Tasks[id] = p.readTask(f, id, n)
		case strings.HasPrefix(f, "status/"):
			p.Statuses[id] = &Status{id, str(f, n.Get("gate")), str(f, n.Get("state"))}
		}
	}
	if _, ok := seen["version.yaml"]; !ok {
		p.diag(Pos{File: "version.yaml"}, "version.yaml is missing")
	}
	return p, nil
}

func (n *Node) itemsOf() []*Node {
	if n == nil {
		return nil
	}
	return n.Items
}

func (p *Project) diag(pos Pos, msg string) { p.Diags = append(p.Diags, Diagnostic{pos, msg}) }

func str(file string, n *Node) Str {
	if n == nil {
		return Str{}
	}
	return Str{strings.TrimSpace(n.Value), Pos{file, n.Line, n.Col}}
}

// readVersion applies the version rule of version.go to version.yaml.
func (p *Project) readVersion(n *Node) {
	p.Version = str("version.yaml", n.Get("tableaux"))
	p.Trunk = str("version.yaml", n.Get("trunk"))
	switch _, _, _, ok := tablo.ParseVersion(p.Version.V); {
	case n.Get("tableaux") == nil:
		p.diag(Pos{"version.yaml", 1, 1}, "P3: missing tableaux version")
	case !ok:
		p.diag(p.Version.Pos, fmt.Sprintf("P3: %q is not major.minor.patch", p.Version.V))
	case !tablo.Accepts(p.Version.V):
		p.diag(p.Version.Pos, fmt.Sprintf("P4: version %s not accepted; the module accepts %d.0.0 to %d.%d.x",
			p.Version.V, tablo.AcceptedMajor, tablo.AcceptedMajor, tablo.AcceptedMinor))
	}
}

var email = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func (p *Project) readTask(f, id string, n *Node) *Task {
	t := &Task{ID: id, Title: str(f, n.Get("title")), Description: str(f, n.Get("description")),
		Assignee: str(f, n.Get("assignee")), Junctions: map[string]Pos{}}
	par := n.Get("parent")
	t.ParentID, t.ParentOrder = str(f, par.Get("id")), str(f, par.Get("order"))
	for _, r := range n.Get("references").itemsOf() {
		t.References = append(t.References, str(f, r.Get("url")))
	}
	for _, r := range n.Get("requires").itemsOf() {
		t.Requires = append(t.Requires, str(f, r.Get("id")))
	}
	if j := n.Get("junctions"); j != nil {
		for _, k := range j.Keys {
			v := j.Fields[k]
			t.Junctions[k] = Pos{f, v.Line, v.Col}
		}
	}
	if n.Get("assignee") == nil {
		p.diag(Pos{f, n.Line, n.Col}, "T4: missing assignee")
	} else if !email.MatchString(t.Assignee.V) {
		p.diag(t.Assignee.Pos, fmt.Sprintf("T4: assignee %q is not an email address", t.Assignee.V))
	}
	if t.ParentOrder.V != "" {
		if o, err := strconv.Atoi(t.ParentOrder.V); err != nil || o < 1 {
			p.diag(t.ParentOrder.Pos, "T6: parent order is not a positive integer")
		}
	}
	return t
}
