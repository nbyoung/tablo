package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/nbyoung/tablo/schemas"
)

// Diag is one finding: a rule, a severity and a position in a file.
type Diag struct {
	File     string // relative to .tableaux
	Pos      Pos
	Severity string // error or warning
	Rule     string // a RULES.md id, or "parse"
	Task     string
	Gate     string
	Msg      string
}

func (d Diag) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s %s", d.File, d.Pos.Line, d.Pos.Col, d.Severity, d.Rule, d.Msg)
}

type task struct {
	id, file string
	doc      *Node
	parent   string
	children int
}

type checker struct {
	diags []Diag
	tasks map[string]*task
	gates map[string]bool
}

func (c *checker) add(file string, at Pos, sev, rule, id, gate, format string, args ...any) {
	c.diags = append(c.diags, Diag{file, at, sev, rule, id, gate, fmt.Sprintf(format, args...)})
}

func loadSchema(kind string) Schema {
	b, err := schemas.FS.ReadFile(kind + ".schema.yaml")
	if err != nil {
		panic(err)
	}
	n, err := ParseYAML(string(b))
	if err != nil {
		panic(kind + " schema: " + err.Error())
	}
	return Schema{n}
}

var taskFileRe = regexp.MustCompile(`^[0-9a-f]{4}\.yaml$`)

// Validate reads the .tableaux directory under root and returns its
// diagnostics, sorted by file and position.
func Validate(root string) ([]Diag, error) {
	dir := filepath.Join(root, ".tableaux")
	if _, err := os.Stat(dir); err != nil {
		return []Diag{{File: ".tableaux", Pos: Pos{1, 1}, Severity: "error", Rule: "P1", Msg: "no .tableaux directory"}}, nil
	}
	c := &checker{tasks: map[string]*task{}, gates: map[string]bool{}}
	c.file(dir, "version.yaml", "version")
	if g := c.file(dir, "gates.yaml", "gates"); g != nil {
		for _, e := range g.Get("gates").itemsOrNil() {
			c.gates[e.Get("key").strOrEmpty()] = true
		}
	}
	names, _ := fs.Glob(os.DirFS(dir), "tasks/*.yaml")
	for _, f := range names {
		id := strings.TrimSuffix(filepath.Base(f), ".yaml")
		doc := c.file(dir, f, "task")
		if !taskFileRe.MatchString(filepath.Base(f)) {
			c.add(f, Pos{1, 1}, "error", "T1", id, "", "file name is not four lowercase hexadecimal digits")
			continue
		}
		if doc != nil {
			c.tasks[id] = &task{id: id, file: f, doc: doc}
		}
	}
	names, _ = fs.Glob(os.DirFS(dir), "status/*.yaml")
	for _, f := range names {
		c.file(dir, f, "status")
	}
	c.tree()
	c.requires()
	c.junctions()
	sort.SliceStable(c.diags, func(i, j int) bool {
		a, b := c.diags[i], c.diags[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Pos.Line < b.Pos.Line || (a.Pos.Line == b.Pos.Line && a.Pos.Col < b.Pos.Col)
	})
	return c.diags, nil
}

func (n *Node) strOrEmpty() string {
	if n == nil {
		return ""
	}
	return n.Str
}

// file parses one file and validates it against its embedded schema.
func (c *checker) file(dir, rel, kind string) *Node {
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		return nil
	}
	id := ""
	if kind != "gates" {
		id = strings.TrimSuffix(filepath.Base(rel), ".yaml")
	}
	doc, err := ParseYAML(string(b))
	if err != nil {
		c.add(rel, Pos{1, 1}, "error", "parse", id, "", "%v", err)
		return nil
	}
	if kind == "task" {
		c.unquoted(rel, id, doc)
	}
	vs := loadSchema(kind).Validate(doc)
	if kind == "status" && doc.Get("gate") == nil {
		vs = dropThen(vs) // a missing gate would otherwise also fail the undefined rule
	}
	for _, v := range vs {
		rule, gate := classify(kind, v, doc)
		c.add(rel, v.Pos, "error", rule, id, gate, "%s: %s", v.Keyword, v.Msg)
	}
	return doc
}

func dropThen(vs []Violation) []Violation {
	var out []Violation
	for _, v := range vs {
		if !v.Then {
			out = append(out, v)
		}
	}
	return out
}

// unquoted warns (T7) of an id written without quotes and reads it as a string.
func (c *checker) unquoted(rel, id string, doc *Node) {
	fix := func(n *Node) {
		if n != nil && !n.Quoted && (n.Kind == Str || n.Kind == Int) {
			c.add(rel, n.Pos, "warning", "T7", id, "", "id %s is not a quoted string", n.Str)
			n.Kind = Str
		}
	}
	fix(doc.Get("parent").Get("id"))
	for _, r := range doc.Get("requires").itemsOrNil() {
		fix(r.Get("id"))
	}
	for _, j := range doc.Get("junctions").entries() {
		fix(j.Val.Get("subproject").Get("id"))
	}
}

func (n *Node) entries() []Entry {
	if n == nil {
		return nil
	}
	return n.Entries
}

// classify names the RULES.md rule that a schema violation stands for.
func classify(kind string, v Violation, doc *Node) (rule, gate string) {
	seg := strings.Split(strings.TrimPrefix(v.Path, "/"), "/")
	top := seg[0]
	switch kind {
	case "task":
		switch {
		case v.Keyword == "format":
			return "T4", ""
		case v.Path == "" && v.Keyword == "additionalProperties":
			return "T3", ""
		case v.Path == "" || top == "title" || top == "description" || top == "assignee":
			return "T2", ""
		case top == "references":
			return "T5", ""
		case top == "parent" && len(seg) > 1 && seg[1] == "id" && v.Keyword == "pattern":
			return "T6", ""
		case top == "parent":
			return "T11", ""
		case top == "requires" && len(seg) > 2 && seg[2] == "id" && v.Keyword == "pattern":
			return "T6", ""
		case top == "requires" && len(seg) > 2 && (seg[2] == "from" || seg[2] == "to"):
			return "R6", ""
		case top == "requires":
			return "R8", ""
		case top == "junctions" && len(seg) > 1:
			gate = seg[1]
			return classifyJunction(gate, v, doc.Get("junctions").Get(gate)), gate
		}
	case "version":
		return "P3", ""
	case "status":
		switch {
		case v.Then:
			return "S4", ""
		case top == "gate" && v.Keyword == "pattern":
			return "S5", ""
		case top == "state" || top == "reason":
			return "S6", ""
		default:
			return "S3", ""
		}
	case "gates":
		if items := doc.Get("gates").itemsOrNil(); top == "gates" && len(seg) > 1 {
			if i, err := strconv.Atoi(seg[1]); err == nil && i < len(items) {
				gate = items[i].Get("key").strOrEmpty()
			}
		}
		switch {
		case v.Path == "" && v.Keyword == "additionalProperties":
			return "G11", gate
		case v.Path == "" && v.Keyword == "required":
			return "G7", gate
		case top == "gates" && len(seg) > 2 && seg[2] == "key" && v.Keyword == "pattern":
			return "G5", gate
		case top == "gates" && v.Keyword == "const":
			return "G2", gate
		case top == "gates" && len(seg) == 1:
			return "G3", gate
		case top == "gates":
			return "G4", gate
		case top == "states" && v.Keyword == "contains":
			return "G12", ""
		case top == "states" && len(seg) > 2 && seg[2] == "severity":
			return "G8", ""
		case top == "states":
			return "G7", ""
		case top == "reasons":
			return "G10", ""
		}
	}
	return "schema", ""
}

func classifyJunction(gate string, v Violation, e *Node) string {
	if e == nil || e.Kind != Map {
		return "J4"
	}
	if gate == "undefined" && v.Keyword == "not" {
		if e.Get("applies") != nil {
			return "J2"
		}
		return "J11"
	}
	plain := e.Get("contributor") != nil || e.Get("model") != nil || e.Get("reviewer") != nil || e.Get("references") != nil
	kinds := 0
	for _, b := range []bool{e.Get("applies") != nil, e.Get("subproject") != nil, plain} {
		if b {
			kinds++
		}
	}
	switch {
	case kinds > 1:
		return "J4"
	case e.Get("applies") != nil:
		return "J6"
	case e.Get("subproject") != nil:
		return "J7"
	case e.Get("model") != nil && e.Get("contributor") == nil:
		return "J5"
	}
	return "J10"
}

func (c *checker) ids() []string {
	var ids []string
	for id := range c.tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// tree checks one root and parent chains that end at it (T8 to T10).
func (c *checker) tree() {
	var roots []string
	for _, id := range c.ids() {
		t := c.tasks[id]
		if p := t.doc.Get("parent"); p == nil {
			roots = append(roots, id)
		} else if pid := p.Get("id"); pid != nil && pid.Kind == Str {
			t.parent = pid.Str
			if c.tasks[pid.Str] == nil {
				c.add(t.file, pid.Pos, "error", "T9", id, "", "parent %s names no task", pid.Str)
			} else {
				c.tasks[pid.Str].children++
			}
		}
	}
	if len(roots) != 1 {
		at := "tasks"
		if len(roots) > 1 {
			at = c.tasks[roots[1]].file
		}
		c.add(at, Pos{1, 1}, "error", "T8", "", "", "%d tasks have no parent, want exactly one", len(roots))
	}
	for _, id := range c.ids() {
		if c.tasks[id].parent == "" {
			continue
		}
		seen := map[string]bool{id: true}
		for cur := c.tasks[id].parent; c.tasks[cur] != nil && c.tasks[cur].parent != ""; cur = c.tasks[cur].parent {
			if seen[cur] {
				p := c.tasks[id].doc.Get("parent").Get("id")
				c.add(c.tasks[id].file, p.Pos, "error", "T10", id, "", "parent chain loops through %s and never reaches the root", cur)
				break
			}
			seen[cur] = true
		}
	}
}

// ancestor reports whether a is a proper ancestor of b.
func (c *checker) ancestor(a, b string) bool {
	seen := map[string]bool{}
	for cur := b; c.tasks[cur] != nil && !seen[cur]; cur = c.tasks[cur].parent {
		seen[cur] = true
		if c.tasks[cur].parent == a {
			return true
		}
	}
	return false
}

type edge struct {
	to  string
	pos Pos
}

// requires checks requirement targets and cycles (R1 to R5).
func (c *checker) requires() {
	graph := map[string][]edge{}
	for _, id := range c.ids() {
		t := c.tasks[id]
		for _, r := range t.doc.Get("requires").itemsOrNil() {
			n := r.Get("id")
			if n == nil || n.Kind != Str {
				continue
			}
			add := func(rule, format string, args ...any) {
				c.add(t.file, n.Pos, "error", rule, id, "", format, args...)
			}
			switch {
			case c.tasks[n.Str] == nil:
				add("R1", "requires %s, which names no task", n.Str)
			case n.Str == id:
				add("R3", "a task never requires itself")
			case c.ancestor(n.Str, id):
				add("R4", "requires its ancestor %s", n.Str)
			case c.ancestor(id, n.Str):
				add("R5", "requires its descendant %s", n.Str)
			default:
				graph[id] = append(graph[id], edge{n.Str, n.Pos})
			}
		}
	}
	reach := func(from, target string) bool {
		seen := map[string]bool{}
		stack := []string{from}
		for len(stack) > 0 {
			u := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if u == target {
				return true
			}
			if seen[u] {
				continue
			}
			seen[u] = true
			for _, e := range graph[u] {
				stack = append(stack, e.to)
			}
		}
		return false
	}
	for _, id := range c.ids() {
		for _, e := range graph[id] {
			if reach(e.to, id) {
				c.add(c.tasks[id].file, e.pos, "error", "R2", id, "", "requirement on %s closes a cycle", e.to)
			}
		}
	}
}

// junctions checks that keys name gates (J1) and that no parent is recursive (J3).
func (c *checker) junctions() {
	for _, id := range c.ids() {
		t := c.tasks[id]
		for _, j := range t.doc.Get("junctions").entries() {
			if len(c.gates) > 0 && !c.gates[j.Key] {
				c.add(t.file, j.KeyPos, "error", "J1", id, j.Key, "junction key %s names no gate", j.Key)
			}
			if t.children > 0 && j.Val.Get("subproject") != nil {
				c.add(t.file, j.KeyPos, "error", "J3", id, j.Key, "a parent states a recursive junction")
			}
		}
	}
}
