package main

// A stand-in YAML reader for the subset the Tableaux files use: block
// mappings, block lists of flow items, flow maps and lists, quoted and plain
// scalars, folded scalars and comments. The implementation needs a real YAML
// module (gopkg.in/yaml.v3) that keeps line positions.

import (
	"fmt"
	"strconv"
	"strings"
)

type yline struct {
	ind  int
	text string
}

func stripComment(s string) string {
	var q byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			q = c
		case c == '#' && (i == 0 || s[i-1] == ' '):
			return s[:i]
		}
	}
	return s
}

func parseYAML(src string) (any, error) {
	var ls []yline
	for _, raw := range strings.Split(src, "\n") {
		t := strings.TrimRight(stripComment(raw), " \r\t")
		if strings.TrimSpace(t) == "" {
			continue
		}
		ls = append(ls, yline{len(t) - len(strings.TrimLeft(t, " ")), strings.TrimSpace(t)})
	}
	if len(ls) == 0 {
		return map[string]any{}, nil
	}
	p := &yp{ls: ls}
	return p.node(ls[0].ind)
}

type yp struct {
	ls []yline
	i  int
}

func isItem(s string) bool { return s == "-" || strings.HasPrefix(s, "- ") }

func (p *yp) node(ind int) (any, error) {
	if isItem(p.ls[p.i].text) {
		var out []any
		for p.i < len(p.ls) && p.ls[p.i].ind == ind && isItem(p.ls[p.i].text) {
			v, err := parseInline(strings.TrimSpace(strings.TrimPrefix(p.ls[p.i].text, "-")))
			if err != nil {
				return nil, err
			}
			out = append(out, v)
			p.i++
		}
		return out, nil
	}
	m := map[string]any{}
	for p.i < len(p.ls) && p.ls[p.i].ind == ind {
		text := p.ls[p.i].text
		k, rest, ok := splitKey(text)
		if !ok {
			return nil, fmt.Errorf("yaml: not a mapping line: %q", text)
		}
		p.i++
		switch rest {
		case ">", "|", ">-":
			var parts []string
			for p.i < len(p.ls) && p.ls[p.i].ind > ind {
				parts = append(parts, p.ls[p.i].text)
				p.i++
			}
			m[k] = strings.Join(parts, " ")
		case "":
			switch {
			case p.i < len(p.ls) && p.ls[p.i].ind > ind:
				v, err := p.node(p.ls[p.i].ind)
				if err != nil {
					return nil, err
				}
				m[k] = v
			case p.i < len(p.ls) && p.ls[p.i].ind == ind && isItem(p.ls[p.i].text):
				v, err := p.node(ind)
				if err != nil {
					return nil, err
				}
				m[k] = v
			default:
				m[k] = nil
			}
		default:
			v, err := parseInline(rest)
			if err != nil {
				return nil, err
			}
			m[k] = v
		}
	}
	return m, nil
}

func splitKey(s string) (k, rest string, ok bool) {
	if i := strings.Index(s, ": "); i >= 0 {
		return strings.TrimSpace(s[:i]), strings.TrimSpace(s[i+2:]), true
	}
	if strings.HasSuffix(s, ":") {
		return strings.TrimSpace(strings.TrimSuffix(s, ":")), "", true
	}
	return "", "", false
}

func parseInline(s string) (any, error) {
	f := &fp{s: s}
	v, err := f.value()
	if err != nil {
		return nil, err
	}
	return v, nil
}

type fp struct {
	s string
	i int
}

func (f *fp) ws() {
	for f.i < len(f.s) && f.s[f.i] == ' ' {
		f.i++
	}
}

func (f *fp) value() (any, error) {
	f.ws()
	if f.i >= len(f.s) {
		return nil, nil
	}
	switch f.s[f.i] {
	case '{':
		f.i++
		m := map[string]any{}
		for {
			f.ws()
			if f.i >= len(f.s) {
				return nil, fmt.Errorf("yaml: unclosed { in %q", f.s)
			}
			if f.s[f.i] == '}' {
				f.i++
				return m, nil
			}
			j := strings.IndexByte(f.s[f.i:], ':')
			if j < 0 {
				return nil, fmt.Errorf("yaml: no key in %q", f.s)
			}
			k := strings.Trim(strings.TrimSpace(f.s[f.i:f.i+j]), `"'`)
			f.i += j + 1
			v, err := f.value()
			if err != nil {
				return nil, err
			}
			m[k] = v
			f.ws()
			if f.i < len(f.s) && f.s[f.i] == ',' {
				f.i++
			}
		}
	case '[':
		f.i++
		var l []any
		for {
			f.ws()
			if f.i >= len(f.s) {
				return nil, fmt.Errorf("yaml: unclosed [ in %q", f.s)
			}
			if f.s[f.i] == ']' {
				f.i++
				return l, nil
			}
			v, err := f.value()
			if err != nil {
				return nil, err
			}
			l = append(l, v)
			f.ws()
			if f.i < len(f.s) && f.s[f.i] == ',' {
				f.i++
			}
		}
	case '"', '\'':
		q := f.s[f.i]
		j := f.i + 1
		for j < len(f.s) && f.s[j] != q {
			if q == '"' && f.s[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(f.s) {
			return nil, fmt.Errorf("yaml: unclosed quote in %q", f.s)
		}
		raw := f.s[f.i : j+1]
		f.i = j + 1
		if q == '"' {
			return strconv.Unquote(raw)
		}
		return raw[1 : len(raw)-1], nil
	}
	j := f.i
	for j < len(f.s) && !strings.ContainsRune(",}]", rune(f.s[j])) {
		j++
	}
	raw := strings.TrimSpace(f.s[f.i:j])
	f.i = j
	return plain(raw), nil
}

func plain(s string) any {
	switch s {
	case "null", "~":
		return nil
	case "true":
		return true
	case "false":
		return false
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return s
}

func str(v any, keys ...string) string {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = m[k]
	}
	s, _ := v.(string)
	return s
}

func num(v any, keys ...string) int {
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			return 0
		}
		v = m[k]
	}
	n, _ := v.(int)
	return n
}

func list(v any, key string) []any {
	m, _ := v.(map[string]any)
	l, _ := m[key].([]any)
	return l
}

func amap(v any, key string) map[string]any {
	m, _ := v.(map[string]any)
	r, _ := m[key].(map[string]any)
	return r
}
