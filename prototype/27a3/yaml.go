package main

import (
	"strings"
)

// A value is nil, a string, a []any or a map[string]any. Every scalar is a
// string, so ids stay ids and numbers stay text.

type yline struct {
	indent int
	text   string
}

// parseYAML reads the subset the Tableaux files and expected.yaml use: block
// mappings and sequences, flow maps and lists, quoted and plain scalars,
// folded and literal blocks, and comments.
func parseYAML(src string) any {
	var ls []yline
	raws := strings.Split(src, "\n")
	for k := 0; k < len(raws); k++ {
		t := stripComment(strings.TrimRight(raws[k], " \t\r"))
		for depth(t) > 0 && k+1 < len(raws) { // a flow collection continues on the next line
			k++
			t += " " + strings.TrimSpace(stripComment(raws[k]))
		}
		if strings.TrimSpace(t) == "" {
			continue
		}
		n := len(t) - len(strings.TrimLeft(t, " "))
		ls = append(ls, yline{n, t[n:]})
	}
	v, _ := parseBlock(ls, 0)
	return v
}

func stripComment(s string) string {
	q := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			if i == 0 || strings.ContainsRune(" [{,:", rune(s[i-1])) {
				q = c
			}
		case c == '#' && (i == 0 || s[i-1] == ' '):
			return strings.TrimRight(s[:i], " ")
		}
	}
	return s
}

func parseBlock(ls []yline, i int) (any, int) {
	var indent int
	if i >= len(ls) {
		return nil, i
	}
	indent = ls[i].indent
	if strings.HasPrefix(ls[i].text, "- ") || ls[i].text == "-" {
		var seq []any
		for i < len(ls) && ls[i].indent == indent && (strings.HasPrefix(ls[i].text, "- ") || ls[i].text == "-") {
			rest := strings.TrimSpace(strings.TrimPrefix(ls[i].text, "-"))
			switch {
			case rest == "":
				var v any
				v, i = parseBlock(ls, i+1)
				seq = append(seq, v)
			case rest[0] == '{' || rest[0] == '[' || rest[0] == '"' || splitKey(rest) < 0:
				seq = append(seq, parseScalar(rest))
				i++
			default: // "- key: value" opens a mapping two columns in
				ls[i] = yline{indent + 2, rest}
				var v any
				v, i = parseBlock(ls, i)
				seq = append(seq, v)
			}
		}
		return seq, i
	}
	m := map[string]any{}
	for i < len(ls) && ls[i].indent == indent && !strings.HasPrefix(ls[i].text, "- ") {
		k := splitKey(ls[i].text)
		if k < 0 {
			i++
			continue
		}
		key := unquote(strings.TrimSpace(ls[i].text[:k]))
		rest := strings.TrimSpace(ls[i].text[k+1:])
		i++
		switch {
		case rest == "":
			if i < len(ls) && (ls[i].indent > indent || (ls[i].indent == indent && strings.HasPrefix(ls[i].text, "- "))) {
				m[key], i = parseBlock(ls, i)
			} else {
				m[key] = nil
			}
		case rest[0] == '>' || rest[0] == '|':
			var parts []string
			for i < len(ls) && ls[i].indent > indent {
				parts = append(parts, ls[i].text)
				i++
			}
			m[key] = strings.Join(parts, " ")
		default:
			m[key] = parseScalar(rest)
		}
	}
	return m, i
}

// splitKey returns the index of the colon that ends a mapping key, or -1.
func splitKey(s string) int {
	q := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			q = c
		case c == '{' || c == '[':
			return -1
		case c == ':' && (i+1 == len(s) || s[i+1] == ' '):
			return i
		}
	}
	return -1
}

func unquote(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

func parseScalar(s string) any {
	v, _ := parseFlow(s, 0)
	return v
}

func parseFlow(s string, i int) (any, int) {
	i = skipSpace(s, i)
	if i >= len(s) {
		return "", i
	}
	switch s[i] {
	case '{':
		m := map[string]any{}
		i = skipSpace(s, i+1)
		for i < len(s) && s[i] != '}' {
			j := i
			for j < len(s) && (s[j] != ':' || (j+1 < len(s) && s[j+1] != ' ')) {
				j++
			}
			key := unquote(strings.TrimSpace(s[i:j]))
			var v any
			v, i = parseFlow(s, j+1)
			m[key] = v
			i = skipSpace(s, i)
			if i < len(s) && s[i] == ',' {
				i = skipSpace(s, i+1)
			}
		}
		return m, i + 1
	case '[':
		var l []any
		i = skipSpace(s, i+1)
		for i < len(s) && s[i] != ']' {
			var v any
			v, i = parseFlow(s, i)
			l = append(l, v)
			i = skipSpace(s, i)
			if i < len(s) && s[i] == ',' {
				i = skipSpace(s, i+1)
			}
		}
		return l, i + 1
	case '"', '\'':
		j := strings.IndexByte(s[i+1:], s[i])
		if j < 0 {
			return s[i+1:], len(s)
		}
		return s[i+1 : i+1+j], i + 2 + j
	}
	j := i
	for j < len(s) && s[j] != ',' && s[j] != '}' && s[j] != ']' {
		j++
	}
	t := strings.TrimSpace(s[i:j])
	if t == "null" {
		return nil, j
	}
	return t, j
}

func skipSpace(s string, i int) int {
	for i < len(s) && s[i] == ' ' {
		i++
	}
	return i
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

// depth counts the flow brackets left open outside quotes.
func depth(s string) int {
	d, q := 0, byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case q != 0:
			if c == q {
				q = 0
			}
		case c == '"' || c == '\'':
			if i == 0 || strings.ContainsRune(" [{,:", rune(s[i-1])) {
				q = c
			}
		case c == '{' || c == '[':
			d++
		case c == '}' || c == ']':
			d--
		}
	}
	return d
}
