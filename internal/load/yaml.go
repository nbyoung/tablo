package load

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/model"
)

// The rules of RULES.md, "Reading a file", that a file's YAML earns. The
// Loader raises rules of RULES.md and names no code of its own.
const (
	codeNotYAML      = "L1" // a file cannot be read or is not YAML
	codeDuplicateKey = "L2" // a mapping states a key twice
	codeYAMLFeature  = "L3" // a second document, an anchor, an alias, a tag other than !!str, a key that is no scalar
)

// The four patterns of the YAML 1.2 core schema that type a plain scalar; a
// plain scalar that matches none is a string.
var (
	nullPattern  = regexp.MustCompile(`^(null|Null|NULL|~|)$`)
	boolPattern  = regexp.MustCompile(`^(true|True|TRUE|false|False|FALSE)$`)
	intPattern   = regexp.MustCompile(`^([-+]?[0-9]+|0o[0-7]+|0x[0-9a-fA-F]+)$`)
	floatPattern = regexp.MustCompile(`^([-+]?(\.[0-9]+|[0-9]+(\.[0-9]*)?)([eE][-+]?[0-9]+)?|[-+]?\.(inf|Inf|INF)|\.(nan|NaN|NAN))$`)
)

// plainKind returns the kind YAML 1.2 gives a plain scalar with this text.
func plainKind(text string) model.Kind {
	switch {
	case nullPattern.MatchString(text):
		return model.Null
	case boolPattern.MatchString(text):
		return model.Bool
	case intPattern.MatchString(text):
		return model.Int
	case floatPattern.MatchString(text):
		return model.Float
	}
	return model.String
}

// errorLine finds the line a yaml.v3 error names.
var errorLine = regexp.MustCompile(`line ([0-9]+)`)

// reader converts one file's YAML into values and collects what it meets.
type reader struct {
	path        string
	diagnostics []model.Diagnostic
}

// report adds an error diagnostic at a position of the file.
func (r *reader) report(code string, line, col int, message string) {
	r.diagnostics = append(r.diagnostics, model.Diagnostic{
		Code:     code,
		Severity: model.Error,
		Pos:      model.Pos{File: r.path, Line: line, Col: col},
		Message:  message,
	})
}

// readYAML parses data as the file at path, a path below .tableaux. It
// returns the root value, nil for an empty file, and the diagnostics L1 to L3
// the file earns. parsed is false when the file is not YAML: the root is then
// nil and the one diagnostic is L1, with the line the reader names and no
// column. Only this file imports the YAML module.
func readYAML(path string, data []byte) (root *model.Value, parsed bool, diagnostics []model.Diagnostic) {
	r := &reader{path: path}
	first, second, err := decode(data)
	if err != nil {
		line := 0
		if m := errorLine.FindStringSubmatch(err.Error()); m != nil {
			line, _ = strconv.Atoi(m[1])
		}
		r.report(codeNotYAML, line, 0, "the file is not YAML: "+strings.TrimPrefix(err.Error(), "yaml: "))
		return nil, false, r.diagnostics
	}
	if first == nil {
		return nil, true, nil
	}
	if second != nil {
		r.report(codeYAMLFeature, second.Line, second.Column, "the file holds a second YAML document; a tool reads the first")
	}
	return r.value(first), true, r.diagnostics
}

// decode parses the first two documents of data. Each is nil when the stream
// ends before it. A fault anywhere in what it parses is the error.
func decode(data []byte) (first, second *yaml.Node, err error) {
	// yaml.v3 reports most faults as errors; the guard turns any it reports
	// by panic into one too, so that one bad file never stops a load.
	defer func() {
		if p := recover(); p != nil {
			first, second, err = nil, nil, fmt.Errorf("yaml: %v", p)
		}
	}()
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var documents [2]*yaml.Node
	for i := range documents {
		node := new(yaml.Node)
		if err := decoder.Decode(node); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, nil, err
		}
		documents[i] = node
	}
	return documents[0], documents[1], nil
}

// value converts a node. It reports an anchor, an alias and a tag other than
// !!str as L3, reads an alias as null, and reads a tagged node as untagged.
func (r *reader) value(node *yaml.Node) *model.Value {
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		return r.value(node.Content[0])
	}
	v := &model.Value{Pos: model.Pos{File: r.path, Line: node.Line, Col: node.Column}}
	if node.Kind == yaml.AliasNode {
		r.report(codeYAMLFeature, node.Line, node.Column, "the file uses an alias, *"+node.Value+"; a tool reads it as null")
		return v
	}
	str := r.properties(node)
	switch node.Kind {
	case yaml.ScalarNode:
		v.Text = node.Value
		if str || node.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
			v.Kind, v.Quoted = model.String, true
		} else {
			v.Kind = plainKind(node.Value)
		}
		v.Read = v.Kind
	case yaml.SequenceNode:
		v.Kind, v.Read = model.Seq, model.Seq
		v.Items = make([]*model.Value, 0, len(node.Content))
		for _, item := range node.Content {
			v.Items = append(v.Items, r.value(item))
		}
	case yaml.MappingNode:
		v.Kind, v.Read = model.Map, model.Map
		v.Fields = make([]model.Field, 0, len(node.Content)/2)
		seen := make(map[string]bool, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			if key.Kind != yaml.ScalarNode {
				r.report(codeYAMLFeature, key.Line, key.Column, "a mapping key is no scalar; a tool leaves the entry unread")
				continue
			}
			r.properties(key)
			if seen[key.Value] {
				r.report(codeDuplicateKey, key.Line, key.Column,
					fmt.Sprintf("the mapping states the key %q twice; the first stands", key.Value))
				continue
			}
			seen[key.Value] = true
			v.Fields = append(v.Fields, model.Field{
				Key:    key.Value,
				KeyPos: model.Pos{File: r.path, Line: key.Line, Col: key.Column},
				Value:  r.value(value),
			})
		}
	}
	return v
}

// properties reports the anchor and the tag a node carries, as one L3, and
// says whether the node is a scalar under the one tag a file may use, !!str.
func (r *reader) properties(node *yaml.Node) (str bool) {
	var uses []string
	if node.Anchor != "" {
		uses = append(uses, "an anchor, &"+node.Anchor)
	}
	if node.Style&yaml.TaggedStyle != 0 {
		if node.Kind == yaml.ScalarNode && node.ShortTag() == "!!str" {
			str = true
		} else {
			uses = append(uses, "a tag, "+node.Tag)
		}
	}
	if len(uses) > 0 {
		r.report(codeYAMLFeature, node.Line, node.Column, "the file uses "+strings.Join(uses, " and ")+"; a tool reads the node without it")
	}
	return str
}
