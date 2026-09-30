package main

import (
	"fmt"
	"strings"
)

// Kind is the shape of a Node.
type Kind int

// The node kinds.
const (
	Scalar Kind = iota
	Map
	Seq
)

// Node is a parsed YAML value with the position where it starts.
type Node struct {
	Kind   Kind
	Value  string           // Scalar
	Keys   []string         // Map, in file order
	Fields map[string]*Node // Map
	Items  []*Node          // Seq
	Line   int              // 1-based
	Col    int              // 1-based
}

// Get returns the field named k, or nil.
func (n *Node) Get(k string) *Node {
	if n == nil || n.Kind != Map {
		return nil
	}
	return n.Fields[k]
}

// Text returns the scalar value of the field k, or "".
func (n *Node) Text(k string) string {
	if f := n.Get(k); f != nil && f.Kind == Scalar {
		return f.Value
	}
	return ""
}

// SyntaxError is a parse failure with a position.
type SyntaxError struct {
	Line, Col int
	Msg       string
}

func (e *SyntaxError) Error() string { return fmt.Sprintf("%d:%d: %s", e.Line, e.Col, e.Msg) }

type line struct {
	no     int
	indent int
	text   string // without indent
}

type parser struct {
	lines []line
	raw   []string
	pos   int
}

// ParseYAML parses the YAML subset the .tableaux files use: block maps and
// sequences, flow maps and sequences, plain and quoted scalars, folded and
// literal block scalars, and comments. Anchors, tags and multiple documents
// are out of scope.
func ParseYAML(src string) (*Node, error) {
	p := &parser{raw: strings.Split(src, "\n")}
	for i, r := range p.raw {
		t := strings.TrimLeft(r, " ")
		if t == "" || t[0] == '#' {
			continue
		}
		p.lines = append(p.lines, line{i + 1, len(r) - len(t), strings.TrimRight(t, " \r")})
	}
	if len(p.lines) == 0 {
		return &Node{Kind: Map, Fields: map[string]*Node{}, Line: 1, Col: 1}, nil
	}
	n, err := p.block(p.lines[0].indent)
	if err != nil {
		return nil, err
	}
	if p.pos < len(p.lines) {
		l := p.lines[p.pos]
		return nil, &SyntaxError{l.no, l.indent + 1, "unexpected content"}
	}
	return n, nil
}

func (p *parser) block(indent int) (*Node, error) {
	l := p.lines[p.pos]
	if isItem(l.text) {
		return p.seq(indent)
	}
	return p.mapping(indent)
}

func (p *parser) seq(indent int) (*Node, error) {
	n := &Node{Kind: Seq, Line: p.lines[p.pos].no, Col: indent + 1}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent != indent || !isItem(l.text) {
			break
		}
		p.pos++
		rest := strings.TrimLeft(strings.TrimPrefix(l.text, "-"), " ")
		col := indent + 1 + len(l.text) - len(rest)
		v, err := p.value(rest, l.no, col, indent)
		if err != nil {
			return nil, err
		}
		n.Items = append(n.Items, v)
	}
	return n, nil
}

func (p *parser) mapping(indent int) (*Node, error) {
	n := &Node{Kind: Map, Fields: map[string]*Node{}, Line: p.lines[p.pos].no, Col: indent + 1}
	for p.pos < len(p.lines) {
		l := p.lines[p.pos]
		if l.indent < indent {
			break
		}
		if l.indent > indent {
			return nil, &SyntaxError{l.no, l.indent + 1, "unexpected indentation"}
		}
		key, rest, ok := splitKey(l.text)
		if !ok {
			return nil, &SyntaxError{l.no, l.indent + 1, "expected key: value"}
		}
		if _, dup := n.Fields[key]; dup {
			return nil, &SyntaxError{l.no, l.indent + 1, "duplicate key " + key}
		}
		p.pos++
		col := indent + 1 + len(l.text) - len(rest)
		v, err := p.value(rest, l.no, col, indent)
		if err != nil {
			return nil, err
		}
		n.Keys = append(n.Keys, key)
		n.Fields[key] = v
	}
	return n, nil
}

// value parses what follows "key:" or "-" on line no, whose parent block sits
// at indent. An empty rest means a nested block.
func (p *parser) value(rest string, no, col, indent int) (*Node, error) {
	rest = stripComment(rest)
	switch {
	case rest == "":
		if p.pos < len(p.lines) && p.lines[p.pos].indent > indent {
			return p.block(p.lines[p.pos].indent)
		}
		return &Node{Kind: Scalar, Line: no, Col: col}, nil
	case rest[0] == '>' || rest[0] == '|':
		return p.blockScalar(rest[0] == '>', no, col, indent), nil
	case rest[0] == '{' || rest[0] == '[':
		f := &flow{s: rest, no: no, col: col}
		n, err := f.value()
		if err != nil {
			return nil, err
		}
		f.skip()
		if f.i < len(f.s) {
			return nil, &SyntaxError{no, col + f.i, "unexpected content after flow value"}
		}
		return n, nil
	}
	s, _, err := scalarText(rest, no, col)
	if err != nil {
		return nil, err
	}
	return &Node{Kind: Scalar, Value: s, Line: no, Col: col}, nil
}

func (p *parser) blockScalar(fold bool, no, col, indent int) *Node {
	var parts []string
	for i := no; i < len(p.raw); i++ { // p.raw[no] is the line after no
		r := strings.TrimRight(p.raw[i], " \r")
		t := strings.TrimLeft(r, " ")
		if t != "" && len(r)-len(t) <= indent {
			break
		}
		parts = append(parts, t)
	}
	for len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	for p.pos < len(p.lines) && p.lines[p.pos].no <= no+len(parts) {
		p.pos++
	}
	sep := "\n"
	if fold {
		sep = " "
	}
	return &Node{Kind: Scalar, Value: strings.Join(parts, sep) + "\n", Line: no, Col: col}
}

// splitKey splits "key: rest" outside quotes.
func splitKey(t string) (key, rest string, ok bool) {
	if t != "" && (t[0] == '"' || t[0] == '\'') {
		s, n, err := scalarText(t, 0, 0)
		if err != nil || !strings.HasPrefix(strings.TrimLeft(t[n:], " "), ":") {
			return "", "", false
		}
		after := strings.TrimLeft(t[n:], " ")[1:]
		return s, strings.TrimLeft(after, " "), true
	}
	for i := 0; i < len(t); i++ {
		if t[i] == ':' && (i+1 == len(t) || t[i+1] == ' ') {
			return t[:i], strings.TrimLeft(t[i+1:], " "), true
		}
	}
	return "", "", false
}

func stripComment(s string) string {
	q := byte(0)
	for i := 0; i < len(s); i++ {
		switch {
		case q != 0:
			if s[i] == q {
				q = 0
			}
		case s[i] == '"' || s[i] == '\'':
			q = s[i]
		case s[i] == '#' && (i == 0 || s[i-1] == ' '):
			return strings.TrimRight(s[:i], " ")
		}
	}
	return s
}

// scalarText reads a quoted or plain scalar from the start of s and returns
// it with the number of bytes consumed.
func scalarText(s string, no, col int) (string, int, error) {
	if s != "" && (s[0] == '"' || s[0] == '\'') {
		q := s[0]
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			switch {
			case s[i] == q && q == '\'' && i+1 < len(s) && s[i+1] == '\'':
				b.WriteByte('\'')
				i++
			case s[i] == q:
				return b.String(), i + 1, nil
			case s[i] == '\\' && q == '"' && i+1 < len(s):
				i++
				b.WriteByte(s[i])
			default:
				b.WriteByte(s[i])
			}
		}
		return "", 0, &SyntaxError{no, col, "unterminated quoted string"}
	}
	return s, len(s), nil
}

// flow parses flow collections within one line.
type flow struct {
	s   string
	i   int
	no  int
	col int
}

func (f *flow) skip() {
	for f.i < len(f.s) && f.s[f.i] == ' ' {
		f.i++
	}
}

func (f *flow) err(msg string) error { return &SyntaxError{f.no, f.col + f.i, msg} }

func (f *flow) value() (*Node, error) {
	f.skip()
	if f.i >= len(f.s) {
		return nil, f.err("unexpected end of line")
	}
	pos := func() Node { return Node{Line: f.no, Col: f.col + f.i} }
	switch f.s[f.i] {
	case '{':
		n := pos()
		n.Kind, n.Fields = Map, map[string]*Node{}
		f.i++
		for {
			f.skip()
			if f.i < len(f.s) && f.s[f.i] == '}' {
				f.i++
				return &n, nil
			}
			k, err := f.scalar(":,}")
			if err != nil {
				return nil, err
			}
			f.skip()
			if f.i >= len(f.s) || f.s[f.i] != ':' {
				return nil, f.err("expected ':' in flow map")
			}
			f.i++
			v, err := f.value()
			if err != nil {
				return nil, err
			}
			n.Keys = append(n.Keys, k.Value)
			n.Fields[k.Value] = v
			if err := f.sep('}'); err != nil {
				return nil, err
			}
		}
	case '[':
		n := pos()
		n.Kind = Seq
		f.i++
		for {
			f.skip()
			if f.i < len(f.s) && f.s[f.i] == ']' {
				f.i++
				return &n, nil
			}
			v, err := f.value()
			if err != nil {
				return nil, err
			}
			n.Items = append(n.Items, v)
			if err := f.sep(']'); err != nil {
				return nil, err
			}
		}
	}
	return f.scalar(",]}")
}

// sep consumes a comma, or leaves the closing bracket for the caller.
func (f *flow) sep(closer byte) error {
	f.skip()
	if f.i >= len(f.s) {
		return f.err("unterminated flow collection")
	}
	switch f.s[f.i] {
	case ',':
		f.i++
	case closer:
	default:
		return f.err("expected ',' or '" + string(closer) + "'")
	}
	return nil
}

func (f *flow) scalar(stops string) (*Node, error) {
	f.skip()
	n := &Node{Line: f.no, Col: f.col + f.i}
	if f.i < len(f.s) && (f.s[f.i] == '"' || f.s[f.i] == '\'') {
		s, used, err := scalarText(f.s[f.i:], f.no, f.col+f.i)
		if err != nil {
			return nil, err
		}
		n.Value = s
		f.i += used
		return n, nil
	}
	start := f.i
	for f.i < len(f.s) && !strings.ContainsRune(stops, rune(f.s[f.i])) {
		f.i++
	}
	n.Value = strings.TrimSpace(f.s[start:f.i])
	return n, nil
}

func isItem(t string) bool { return t == "-" || strings.HasPrefix(t, "- ") }
