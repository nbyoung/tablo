package main

import (
	"regexp"
	"sort"
	"strings"
)

// Task holds the fields of a task file that the audit reads. Junctions maps a
// gate key to the scalar fields of its entry, and to the raw text of nested
// values such as a subproject.
type Task struct {
	ID        string
	Assignee  string
	Parent    string
	Junctions map[string]map[string]string
}

// Project is the .tableaux directory at one revision.
type Project struct {
	Tasks  map[string]*Task
	Status map[string]string // leaf id to the gate its status file states
	Gates  []string          // gate keys in order
	Trunk  string
}

// Junction is a junction resolved for one task and gate, field by field, from
// the task and then its ancestors.
type Junction struct {
	Contributor, Model, Reviewer string
	Applies, Recursive           bool
}

// splitTop splits s at commas that sit outside brackets, braces and quotes.
func splitTop(s string) []string {
	var parts []string
	depth, start := 0, 0
	quote := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '[' || c == '{':
			depth++
		case c == ']' || c == '}':
			depth--
		case c == ',' && depth == 0:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// parseFlow reads a one-line flow mapping, { a: b, c: [..] }, into its keys
// and raw values. It is the only YAML shape the audit needs beyond scalars.
func parseFlow(s string) map[string]string {
	m := map[string]string{}
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(strings.TrimPrefix(s, "{"), "}")
	for _, part := range splitTop(s) {
		k, v, ok := strings.Cut(part, ":")
		if ok {
			m[strings.TrimSpace(k)] = unquote(v)
		}
	}
	return m
}

func parseTask(id, text string) *Task {
	t := &Task{ID: id, Junctions: map[string]map[string]string{}}
	inJunctions := false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, " ") {
			if inJunctions {
				k, v, _ := strings.Cut(strings.TrimSpace(line), ":")
				t.Junctions[k] = parseFlow(v)
			}
			continue
		}
		inJunctions = false
		k, v, _ := strings.Cut(line, ":")
		switch k {
		case "junctions":
			inJunctions = true
		case "assignee":
			t.Assignee = unquote(v)
		case "parent":
			t.Parent = parseFlow(v)["id"]
		}
	}
	return t
}

// scalar returns the value of a top-level scalar field.
func scalar(text, key string) string {
	for _, line := range strings.Split(text, "\n") {
		if k, v, ok := strings.Cut(line, ":"); ok && k == key {
			return unquote(v)
		}
	}
	return ""
}

var gateKey = regexp.MustCompile(`^\s*-\s*\{\s*key:\s*([a-z_]+)`)

// parseGates returns the gate keys of gates.yaml, which precede its states.
func parseGates(text string) []string {
	var keys []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "states:") {
			break
		}
		if m := gateKey.FindStringSubmatch(line); m != nil {
			keys = append(keys, m[1])
		}
	}
	return keys
}

func (p *Project) index(gate string) int {
	for i, g := range p.Gates {
		if g == gate {
			return i
		}
	}
	return -1
}

func (p *Project) ids() []string {
	ids := make([]string, 0, len(p.Tasks))
	for id := range p.Tasks {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Authorities lists the assignees of the ancestors, nearest first; the root's
// only authority is its own assignee, the owner.
func (p *Project) Authorities(id string) []string {
	t := p.Tasks[id]
	if t == nil {
		return nil
	}
	if t.Parent == "" {
		return []string{t.Assignee}
	}
	var out []string
	for cur := p.Tasks[t.Parent]; cur != nil; cur = p.Tasks[cur.Parent] {
		out = append(out, cur.Assignee)
	}
	return out
}

// Junction resolves the junction at a gate. The contributor defaults to the
// task's assignee, and an agent (a junction with a model) that states no
// reviewer takes the assignee as its reviewer (D4).
func (p *Project) Junction(id, gate string) Junction {
	j := Junction{Applies: true, Contributor: p.Tasks[id].Assignee}
	var appliesSet, contribSet, first = false, false, true
	for cur := p.Tasks[id]; cur != nil; cur = p.Tasks[cur.Parent] {
		f, ok := cur.Junctions[gate]
		if !ok {
			continue
		}
		if _, sub := f["subproject"]; sub && first {
			j.Recursive = true
		}
		first = false
		if v, ok := f["applies"]; ok && !appliesSet {
			j.Applies, appliesSet = v != "false", true
		}
		if v, ok := f["contributor"]; ok && !contribSet {
			j.Contributor, contribSet = v, true
		}
		if v, ok := f["model"]; ok && j.Model == "" {
			j.Model = v
		}
		if v, ok := f["reviewer"]; ok && j.Reviewer == "" {
			j.Reviewer = v
		}
	}
	if j.Reviewer == "" && j.Model != "" {
		j.Reviewer = p.Tasks[id].Assignee
	}
	return j
}

// Applicable reports whether a gate applies to the task.
func (p *Project) Applicable(id, gate string) bool {
	return p.index(gate) > 0 && p.Junction(id, gate).Applies
}
