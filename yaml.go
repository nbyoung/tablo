package tablo

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// A node is one JSON value, read with its keys in document order.
type node struct {
	kind  byte // 'm' mapping, 's' sequence, 'v' scalar
	keys  []string
	items []*node // the values of a mapping, or the items of a sequence
	text  string  // the YAML spelling of a scalar
}

var plainKey = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// reservedKeys are the words YAML 1.1 reads as booleans or null, which a key
// spells in quotes.
var reservedKeys = map[string]bool{
	"null": true, "true": true, "false": true,
	"yes": true, "no": true, "on": true, "off": true, "y": true, "n": true,
}

// yamlOfJSON respells one JSON document as block YAML. It walks the tokens of
// the document: block mappings and sequences indented by two spaces, [] and {}
// for empty ones, every string quoted by strconv.Quote, and a key plain when
// it matches ^[a-z][a-z0-9_]*$ and is no reserved word. The input is the
// output of Envelope.JSON, so a malformed document is a fault and panics.
func yamlOfJSON(doc []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	root, err := readNode(dec)
	if err != nil {
		panic("tablo: respelling JSON as YAML: " + err.Error())
	}
	var b bytes.Buffer
	writeNode(&b, root, 0, "")
	return b.Bytes()
}

func readNode(dec *json.Decoder) (*node, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if t == '{' {
			n := &node{kind: 'm'}
			for dec.More() {
				k, err := dec.Token()
				if err != nil {
					return nil, err
				}
				v, err := readNode(dec)
				if err != nil {
					return nil, err
				}
				n.keys = append(n.keys, k.(string))
				n.items = append(n.items, v)
			}
			_, err := dec.Token() // the closing brace
			return n, err
		}
		n := &node{kind: 's'}
		for dec.More() {
			v, err := readNode(dec)
			if err != nil {
				return nil, err
			}
			n.items = append(n.items, v)
		}
		_, err := dec.Token() // the closing bracket
		return n, err
	case string:
		return &node{kind: 'v', text: strconv.Quote(t)}, nil
	case json.Number:
		return &node{kind: 'v', text: t.String()}, nil
	case bool:
		return &node{kind: 'v', text: strconv.FormatBool(t)}, nil
	case nil:
		return &node{kind: 'v', text: "null"}, nil
	}
	return nil, io.ErrUnexpectedEOF
}

func (n *node) empty() bool { return n.kind != 'v' && len(n.items) == 0 }

// inline returns the spelling that follows a key or a dash on the same line:
// a scalar, or [] and {} for an empty collection; ok is false for a block.
func (n *node) inline() (string, bool) {
	switch {
	case n.kind == 'v':
		return n.text, true
	case n.empty() && n.kind == 'm':
		return "{}", true
	case n.empty():
		return "[]", true
	}
	return "", false
}

func yamlKey(k string) string {
	if plainKey.MatchString(k) && !reservedKeys[k] {
		return k
	}
	return strconv.Quote(k)
}

// writeNode writes a block collection whose first line starts with first and
// whose later lines start at the indent; a scalar never reaches it.
func writeNode(b *bytes.Buffer, n *node, indent int, first string) {
	pad := strings.Repeat(" ", indent)
	if first == "" {
		first = pad
	}
	if s, ok := n.inline(); ok {
		b.WriteString(first + s + "\n")
		return
	}
	for i, item := range n.items {
		lead := pad
		if i == 0 {
			lead = first
		}
		if n.kind == 'm' {
			b.WriteString(lead + yamlKey(n.keys[i]) + ":")
			if s, ok := item.inline(); ok {
				b.WriteString(" " + s + "\n")
			} else {
				b.WriteString("\n")
				writeNode(b, item, indent+2, "")
			}
			continue
		}
		if s, ok := item.inline(); ok {
			b.WriteString(lead + "- " + s + "\n")
		} else {
			writeNode(b, item, indent+2, lead+"- ")
		}
	}
}
