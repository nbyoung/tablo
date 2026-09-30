package main

import (
	"fmt"
	"strconv"
	"strings"
)

// parseYAML reads the subset the Tableaux files use: top-level keys whose
// values are a flow value, a folded block scalar, a block list of flow values,
// or a block map of flow values. It is a stand-in for a real YAML parser.
func parseYAML(src string) (map[string]any, error) {
	out := map[string]any{}
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		line := stripComment(lines[i])
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line[0] == ' ' {
			return nil, fmt.Errorf("line %d: unexpected indent", i+1)
		}
		key, rest, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("line %d: no key", i+1)
		}
		rest = strings.TrimSpace(rest)
		var body []string
		for i+1 < len(lines) && (lines[i+1] == "" || lines[i+1][0] == ' ') {
			i++
			body = append(body, lines[i])
		}
		switch rest {
		case ">", "|":
			var parts []string
			for _, b := range body {
				parts = append(parts, strings.TrimSpace(b))
			}
			out[key] = strings.TrimSpace(strings.Join(parts, " "))
		case "":
			v, err := parseBlock(body)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			out[key] = v
		default:
			v, err := parseFlow(rest)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			out[key] = v
		}
	}
	return out, nil
}

func parseBlock(body []string) (any, error) {
	var list []any
	m := map[string]any{}
	for _, b := range body {
		b = strings.TrimSpace(stripComment(b))
		if b == "" {
			continue
		}
		if item, ok := strings.CutPrefix(b, "- "); ok {
			v, err := parseFlow(item)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
			continue
		}
		k, v, _ := strings.Cut(b, ":")
		f, err := parseFlow(strings.TrimSpace(v))
		if err != nil {
			return nil, err
		}
		m[k] = f
	}
	if list != nil {
		return list, nil
	}
	return m, nil
}

func stripComment(s string) string {
	quoted := false
	for i, r := range s {
		switch {
		case r == '"':
			quoted = !quoted
		case r == '#' && !quoted && (i == 0 || s[i-1] == ' '):
			return strings.TrimRight(s[:i], " ")
		}
	}
	return s
}

func parseFlow(s string) (any, error) {
	p := &flow{s: []rune(s)}
	if t := strings.TrimSpace(s); t != "" && !strings.ContainsRune("{[\"", rune(t[0])) {
		p.top = true
	}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	p.skip()
	if p.i < len(p.s) {
		return nil, fmt.Errorf("trailing %q", string(p.s[p.i:]))
	}
	return v, nil
}

type flow struct {
	s   []rune
	i   int
	top bool // a bare top-level scalar runs to the end of the line
}

func (p *flow) skip() {
	for p.i < len(p.s) && p.s[p.i] == ' ' {
		p.i++
	}
}

func (p *flow) value() (any, error) {
	p.skip()
	if p.i >= len(p.s) {
		return "", nil
	}
	switch p.s[p.i] {
	case '{':
		return p.mapping()
	case '[':
		return p.list()
	case '"':
		start := p.i
		p.i++
		for p.i < len(p.s) && p.s[p.i] != '"' {
			if p.s[p.i] == '\\' {
				p.i++
			}
			p.i++
		}
		p.i++
		if p.i > len(p.s) {
			return nil, fmt.Errorf("unterminated string")
		}
		return strconv.Unquote(string(p.s[start:p.i]))
	}
	start := p.i
	for p.i < len(p.s) && (p.top || !strings.ContainsRune(",}]", p.s[p.i])) {
		p.i++
	}
	tok := strings.TrimSpace(string(p.s[start:p.i]))
	switch tok {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	if n, err := strconv.Atoi(tok); err == nil {
		return n, nil
	}
	return tok, nil
}

func (p *flow) mapping() (any, error) {
	m := map[string]any{}
	p.i++
	for {
		p.skip()
		if p.i >= len(p.s) {
			return nil, fmt.Errorf("unterminated map")
		}
		if p.s[p.i] == '}' {
			p.i++
			return m, nil
		}
		start := p.i
		for p.i < len(p.s) && p.s[p.i] != ':' {
			p.i++
		}
		if p.i >= len(p.s) {
			return nil, fmt.Errorf("map key without colon")
		}
		key := strings.TrimSpace(string(p.s[start:p.i]))
		p.i++
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		m[key] = v
		p.skip()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
		}
	}
}

func (p *flow) list() (any, error) {
	l := []any{}
	p.i++
	for {
		p.skip()
		if p.i >= len(p.s) {
			return nil, fmt.Errorf("unterminated list")
		}
		if p.s[p.i] == ']' {
			p.i++
			return l, nil
		}
		v, err := p.value()
		if err != nil {
			return nil, err
		}
		l = append(l, v)
		p.skip()
		if p.i < len(p.s) && p.s[p.i] == ',' {
			p.i++
		}
	}
}

func str(m map[string]any, k string) string {
	s, _ := m[k].(string)
	return s
}

func mp(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func lst(v any) []any {
	l, _ := v.([]any)
	return l
}
