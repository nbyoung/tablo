package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Pos is a 1-based line and column (in characters) in a source file.
type Pos struct{ Line, Col int }

// Kind is the type of a parsed YAML node.
type Kind int

// The node kinds.
const (
	Null Kind = iota
	Str
	Int
	Bool
	Map
	Seq
)

// Entry is one key of a mapping, with the position of the key.
type Entry struct {
	Key    string
	KeyPos Pos
	Val    *Node
}

// Node is a parsed YAML value that remembers where it started.
type Node struct {
	Kind    Kind
	Pos     Pos
	Str     string
	Int     int
	Bool    bool
	Quoted  bool
	Entries []Entry
	Items   []*Node
}

// Get returns the value of key in a mapping, or nil.
func (n *Node) Get(key string) *Node {
	if n == nil {
		return nil
	}
	for _, e := range n.Entries {
		if e.Key == key {
			return e.Val
		}
	}
	return nil
}

type line struct {
	no   int
	text []rune
}

type parser struct {
	lines []line
	i     int
}

// ParseYAML parses the YAML subset that Tableaux files use: block and flow
// mappings and sequences, plain, quoted and folded scalars, and comments.
// It rejects anything else (anchors, tags, multiple documents) by failing
// to parse or by reading it as a plain scalar.
func ParseYAML(src string) (n *Node, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	p := &parser{}
	for i, l := range strings.Split(strings.ReplaceAll(src, "\r\n", "\n"), "\n") {
		p.lines = append(p.lines, line{i + 1, []rune(l)})
	}
	n = p.node(0)
	if p.skip() {
		l := p.lines[p.i]
		panic(fmt.Sprintf("%d: unexpected content", l.no))
	}
	return n, nil
}

func indentOf(r []rune) int {
	n := 0
	for n < len(r) && r[n] == ' ' {
		n++
	}
	return n
}

func stripComment(r []rune) []rune {
	var q rune
	for i, c := range r {
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case (c == '"' || c == '\'') && (i == 0 || strings.ContainsRune(" \t[{,:", r[i-1])):
			q = c
		case c == '#' && (i == 0 || r[i-1] == ' ' || r[i-1] == '\t'):
			return r[:i]
		}
	}
	return r
}

// skip moves to the next line with content and reports whether one exists.
func (p *parser) skip() bool {
	for ; p.i < len(p.lines); p.i++ {
		if strings.TrimSpace(string(stripComment(p.lines[p.i].text))) != "" {
			return true
		}
	}
	return false
}

func (p *parser) cur() (indent int, t []rune) {
	l := p.lines[p.i]
	t = stripComment(l.text)
	return indentOf(t), []rune(strings.TrimRight(string(t), " \t"))
}

func isDash(t []rune, ind int) bool {
	return ind < len(t) && t[ind] == '-' && (ind+1 == len(t) || t[ind+1] == ' ')
}

// node parses the block node whose first line has indent at least min.
func (p *parser) node(min int) *Node {
	if !p.skip() {
		return &Node{}
	}
	ind, t := p.cur()
	if ind < min {
		return &Node{}
	}
	switch {
	case isDash(t, ind):
		return p.seq(ind)
	case isKeyLine(t[ind:]):
		return p.mapping(ind)
	}
	no := p.lines[p.i].no
	p.i++
	return p.inline(t[ind:], ind, no)
}

func (p *parser) seq(ind int) *Node {
	n := &Node{Kind: Seq, Pos: Pos{p.lines[p.i].no, ind + 1}}
	for p.skip() {
		i, t := p.cur()
		if i != ind || !isDash(t, ind) {
			break
		}
		p.lines[p.i].text[ind] = ' ' // the item now reads as a block node at ind+1 or deeper
		n.Items = append(n.Items, p.node(ind+1))
	}
	return n
}

func (p *parser) mapping(ind int) *Node {
	n := &Node{Kind: Map, Pos: Pos{p.lines[p.i].no, ind + 1}}
	for p.skip() {
		i, t := p.cur()
		if i != ind || !isKeyLine(t[ind:]) {
			break
		}
		no := p.lines[p.i].no
		key, after := splitKey(t, ind)
		p.i++
		rest := []rune(strings.TrimLeft(string(t[after:]), " "))
		col := len(t) - len(rest)
		e := Entry{Key: key, KeyPos: Pos{no, ind + 1}}
		switch {
		case len(rest) == 0:
			if p.skip() {
				ni, nt := p.cur()
				if ni > ind || (ni == ind && isDash(nt, ind)) {
					e.Val = p.node(ind)
					break
				}
			}
			e.Val = &Node{Pos: Pos{no, col + 1}}
		case rest[0] == '>' || rest[0] == '|':
			e.Val = p.block(rest, ind, no, col)
		default:
			e.Val = p.inline(rest, col, no)
		}
		n.Entries = append(n.Entries, e)
	}
	return n
}

// block reads a folded or literal scalar whose lines are indented deeper than ind.
func (p *parser) block(head []rune, ind, no, col int) *Node {
	var parts []string
	for ; p.i < len(p.lines); p.i++ {
		r := p.lines[p.i].text
		if strings.TrimSpace(string(r)) != "" && indentOf(r) <= ind {
			break
		}
		parts = append(parts, strings.TrimSpace(string(r)))
	}
	sep := " "
	if head[0] == '|' {
		sep = "\n"
	}
	return &Node{Kind: Str, Pos: Pos{no, col + 1}, Str: strings.TrimSpace(strings.Join(parts, sep))}
}

// isKeyLine reports whether t begins a "key: value" mapping entry.
func isKeyLine(t []rune) bool {
	if len(t) == 0 || t[0] == '{' || t[0] == '[' {
		return false
	}
	if t[0] == '"' || t[0] == '\'' {
		for i := 1; i < len(t); i++ {
			if t[i] == t[0] {
				j := i + 1
				return j < len(t) && t[j] == ':' && (j+1 == len(t) || t[j+1] == ' ')
			}
		}
		return false
	}
	for i := 0; i < len(t); i++ {
		if t[i] == ':' && (i+1 == len(t) || t[i+1] == ' ') {
			return true
		}
	}
	return false
}

// splitKey reads the key at t[ind:] and returns it with the index after its colon.
func splitKey(t []rune, ind int) (string, int) {
	if t[ind] == '"' || t[ind] == '\'' {
		i := ind + 1
		for t[i] != t[ind] {
			i++
		}
		return string(t[ind+1 : i]), i + 2
	}
	for i := ind; ; i++ {
		if t[i] == ':' && (i+1 == len(t) || t[i+1] == ' ') {
			return strings.TrimSpace(string(t[ind:i])), i + 1
		}
	}
}

// inline parses a scalar or flow collection that starts at column col
// (0-based) of line no. A flow collection may continue over later lines.
func (p *parser) inline(t []rune, col, no int) *Node {
	if len(t) > 0 && (t[0] == '{' || t[0] == '[') {
		for depth(t) > 0 && p.skip() {
			_, nt := p.cur()
			t = append(t, ' ')
			t = append(t, []rune(strings.TrimSpace(string(nt)))...)
			p.i++
		}
	}
	f := &flow{r: t, no: no, col: col}
	n := f.value(false)
	f.ws()
	if f.p < len(f.r) {
		panic(fmt.Sprintf("%d:%d: unexpected %q", no, col+f.p+1, string(f.r[f.p:])))
	}
	return n
}

func depth(t []rune) int {
	d := 0
	var q rune
	for i, c := range t {
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case (c == '"' || c == '\'') && (i == 0 || strings.ContainsRune(" \t[{,:", t[i-1])):
			q = c
		case c == '{' || c == '[':
			d++
		case c == '}' || c == ']':
			d--
		}
	}
	return d
}

type flow struct {
	r       []rune
	p       int
	no, col int
}

func (f *flow) pos() Pos { return Pos{f.no, f.col + f.p + 1} }

func (f *flow) ws() {
	for f.p < len(f.r) && f.r[f.p] == ' ' {
		f.p++
	}
}

func (f *flow) value(inFlow bool) *Node {
	f.ws()
	if f.p >= len(f.r) {
		return &Node{Pos: f.pos()}
	}
	at := f.pos()
	switch c := f.r[f.p]; c {
	case '{':
		f.p++
		n := &Node{Kind: Map, Pos: at}
		for {
			f.ws()
			if f.p >= len(f.r) {
				panic(fmt.Sprintf("%d: unclosed {", f.no))
			}
			if f.r[f.p] == '}' {
				f.p++
				return n
			}
			kp := f.pos()
			key := f.scalarKey()
			f.ws()
			e := Entry{Key: key, KeyPos: kp}
			if f.p < len(f.r) && f.r[f.p] == ':' {
				f.p++
				e.Val = f.value(true)
			} else {
				e.Val = &Node{Pos: f.pos()}
			}
			n.Entries = append(n.Entries, e)
			f.ws()
			if f.p < len(f.r) && f.r[f.p] == ',' {
				f.p++
			}
		}
	case '[':
		f.p++
		n := &Node{Kind: Seq, Pos: at}
		for {
			f.ws()
			if f.p >= len(f.r) {
				panic(fmt.Sprintf("%d: unclosed [", f.no))
			}
			if f.r[f.p] == ']' {
				f.p++
				return n
			}
			n.Items = append(n.Items, f.value(true))
			f.ws()
			if f.p < len(f.r) && f.r[f.p] == ',' {
				f.p++
			}
		}
	case '"', '\'':
		return &Node{Kind: Str, Pos: at, Str: f.quoted(), Quoted: true}
	}
	start := f.p
	for f.p < len(f.r) {
		c := f.r[f.p]
		if inFlow && (c == ',' || c == '}' || c == ']') {
			break
		}
		f.p++
	}
	n := resolve(strings.TrimSpace(string(f.r[start:f.p])))
	n.Pos = at
	return n
}

func (f *flow) quoted() string {
	q := f.r[f.p]
	f.p++
	var b strings.Builder
	for f.p < len(f.r) {
		c := f.r[f.p]
		switch {
		case c == '\\' && q == '"' && f.p+1 < len(f.r):
			f.p++
			switch f.r[f.p] {
			case 'n':
				b.WriteRune('\n')
			default:
				b.WriteRune(f.r[f.p])
			}
		case c == q && q == '\'' && f.p+1 < len(f.r) && f.r[f.p+1] == '\'':
			b.WriteRune('\'')
			f.p++
		case c == q:
			f.p++
			return b.String()
		default:
			b.WriteRune(c)
		}
		f.p++
	}
	panic(fmt.Sprintf("%d: unterminated string", f.no))
}

func (f *flow) scalarKey() string {
	f.ws()
	if f.p < len(f.r) && (f.r[f.p] == '"' || f.r[f.p] == '\'') {
		return f.quoted()
	}
	start := f.p
	for f.p < len(f.r) && f.r[f.p] != ':' && f.r[f.p] != ',' && f.r[f.p] != '}' {
		f.p++
	}
	return strings.TrimSpace(string(f.r[start:f.p]))
}

var intRe = regexp.MustCompile(`^-?[0-9]+$`)

func resolve(s string) *Node {
	switch {
	case s == "" || s == "~" || s == "null":
		return &Node{}
	case s == "true" || s == "false":
		return &Node{Kind: Bool, Bool: s == "true"}
	case intRe.MatchString(s):
		if v, err := strconv.Atoi(s); err == nil {
			return &Node{Kind: Int, Int: v, Str: s}
		}
	}
	return &Node{Kind: Str, Str: s}
}
