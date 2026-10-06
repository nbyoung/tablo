package model

import (
	"math"
	"strconv"
	"strings"
)

// Pos locates a value in a project file.
type Pos struct {
	File string // the path below .tableaux, with forward slashes; "" for the project itself
	Line int    // from 1; 0 when the fault has no line
	Col  int    // from 1, in Unicode code points; 0 when the reader names no column
}

// Kind is the kind of a YAML node under the YAML 1.2 core schema.
type Kind int

// The kinds of a node. A scalar is one of the first five.
const (
	Null Kind = iota
	Bool
	Int
	Float
	String
	Seq
	Map
)

// String returns the kind's name in lower case.
func (k Kind) String() string {
	switch k {
	case Null:
		return "null"
	case Bool:
		return "bool"
	case Int:
		return "int"
	case Float:
		return "float"
	case String:
		return "string"
	case Seq:
		return "seq"
	case Map:
		return "map"
	}
	return "kind(" + strconv.Itoa(int(k)) + ")"
}

// Value is one YAML node as read: the untyped tree a schema validates.
type Value struct {
	Pos    Pos
	Kind   Kind     // what the tool takes the node as
	Read   Kind     // a scalar: the kind YAML 1.2 gives it as written; differs from Kind only at an id
	Text   string   // a scalar: its text as written, without quotes
	Quoted bool     // a scalar: written in quotes, as a block scalar or under !!str
	Items  []*Value // a Seq
	Fields []Field  // a Map, in file order; a repeated key is not here
}

// Field is one key of a mapping and its value.
type Field struct {
	Key    string
	KeyPos Pos
	Value  *Value
}

// Scalar reports whether the value is a scalar: neither a Seq nor a Map.
func (v *Value) Scalar() bool {
	return v != nil && v.Kind != Seq && v.Kind != Map
}

// Get returns the value of key in a Map, or nil.
func (v *Value) Get(key string) *Value {
	if v == nil || v.Kind != Map {
		return nil
	}
	for i := range v.Fields {
		if v.Fields[i].Key == key {
			return v.Fields[i].Value
		}
	}
	return nil
}

// At follows keys and sequence indexes, as a JSON Pointer's segments, and returns nil where the path ends.
func (v *Value) At(path ...string) *Value {
	for _, segment := range path {
		if v == nil {
			return nil
		}
		switch v.Kind {
		case Map:
			v = v.Get(segment)
		case Seq:
			i, ok := index(segment)
			if !ok || i >= len(v.Items) {
				return nil
			}
			v = v.Items[i]
		default:
			return nil
		}
	}
	return v
}

// index parses a sequence index as a JSON Pointer writes it: 0, or digits with no leading zero.
func index(s string) (int, bool) {
	if s == "" || (len(s) > 1 && s[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.Atoi(s)
	return n, err == nil
}

// Plain returns the tree as map[string]any, []any, string, int64, float64, bool and nil,
// the form a JSON Schema validator takes. An Int beyond int64 becomes a float64.
func (v *Value) Plain() any {
	if v == nil {
		return nil
	}
	switch v.Kind {
	case Map:
		m := make(map[string]any, len(v.Fields))
		for i := range v.Fields {
			m[v.Fields[i].Key] = v.Fields[i].Value.Plain()
		}
		return m
	case Seq:
		s := make([]any, len(v.Items))
		for i, item := range v.Items {
			s[i] = item.Plain()
		}
		return s
	case String:
		return v.Text
	case Bool:
		return v.Text == "true" || v.Text == "True" || v.Text == "TRUE"
	case Int:
		if n, ok := integer(v.Text); ok {
			return n
		}
		return float(v.Text)
	case Float:
		return float(v.Text)
	}
	return nil
}

// integer reads the text of an Int: decimal, 0o octal or 0x hexadecimal. ok is false beyond int64.
func integer(text string) (int64, bool) {
	base, digits := 10, text
	switch {
	case strings.HasPrefix(text, "0o"):
		base, digits = 8, text[2:]
	case strings.HasPrefix(text, "0x"):
		base, digits = 16, text[2:]
	}
	n, err := strconv.ParseInt(digits, base, 64)
	return n, err == nil
}

// float reads the text of a Float, or of an Int beyond int64.
func float(text string) float64 {
	switch strings.ToLower(strings.TrimLeft(text, "+-")) {
	case ".inf":
		if strings.HasPrefix(text, "-") {
			return math.Inf(-1)
		}
		return math.Inf(1)
	case ".nan":
		return math.NaN()
	}
	digits := text
	switch {
	case strings.HasPrefix(text, "0o"):
		digits = text[2:]
		f := 0.0
		for i := 0; i < len(digits); i++ {
			f = f*8 + float64(digits[i]-'0')
		}
		return f
	case strings.HasPrefix(text, "0x"):
		digits = "0x" + text[2:] + "p0"
	}
	f, _ := strconv.ParseFloat(digits, 64)
	return f
}

// Str is a typed field as written, whatever scalar states it. Node is nil exactly when the file omits the field.
type Str struct {
	V    string // the text of any scalar; "" for a collection
	Node *Value
}

// IntField is a typed integer field as written. Node is nil exactly when the file omits the field.
type IntField struct {
	V    int
	OK   bool // the node is an Int, or a Float with no fraction, that fits
	Node *Value
}

// BoolField is a typed boolean field as written. Node is nil exactly when the file omits the field.
type BoolField struct {
	V, OK bool // OK when the node is a Bool
	Node  *Value
}

// StrOf returns the typed field a node states; a nil node gives the zero Str.
func StrOf(node *Value) Str {
	if node.Scalar() {
		return Str{V: node.Text, Node: node}
	}
	return Str{Node: node}
}

// IntOf returns the typed field a node states; a nil node gives the zero IntField.
func IntOf(node *Value) IntField {
	f := IntField{Node: node}
	if node == nil {
		return f
	}
	switch node.Kind {
	case Int:
		if n, ok := integer(node.Text); ok && int64(int(n)) == n {
			f.V, f.OK = int(n), true
		}
	case Float:
		x := float(node.Text)
		if x == math.Trunc(x) && math.Abs(x) <= 1<<53 {
			f.V, f.OK = int(x), true
		}
	}
	return f
}

// BoolOf returns the typed field a node states; a nil node gives the zero BoolField.
func BoolOf(node *Value) BoolField {
	f := BoolField{Node: node}
	if node != nil && node.Kind == Bool {
		f.V, f.OK = node.Plain().(bool), true
	}
	return f
}
