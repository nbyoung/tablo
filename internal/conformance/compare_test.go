package conformance

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/nbyoung/tablo/internal/corpustest"
	"gopkg.in/yaml.v3"
)

// The comparison walks the expected document and never looks at an envelope.
// Its rules are those of design 4b4f, "Comparison"; a comment at each names the
// rule it applies.

// A result is how one fact fares.
type result int

const (
	agreed result = iota
	differed
	undecided
)

// A fact is one comparison, and the unit every count uses.
type fact struct {
	path   string // tasks.4e2b.status.gate
	kind   string // the key of the fact's kind, as the report counts it
	line   int    // the line in expected.yaml
	want   string
	got    string
	result result
	rule   string // a finding fact: the rule it states
	call   string // the call that supplies the observed value
}

// A finding is one finding, as expected.yaml states it or as validate reports it.
type finding struct {
	rule, severity, task, file, gate, commit, trailer, message string
	line                                                       int
	text                                                       string // an unexpected finding, as a message shows it
}

// An outcome is the comparison of one entry.
type outcome struct {
	facts      []fact
	unexpected []finding // observed findings no expected finding takes
	observed   int       // the findings validate reported
}

// mismatches counts the facts that differ and the unexpected findings.
func (o *outcome) mismatches() int {
	n := len(o.unexpected)
	for _, f := range o.facts {
		if f.result == differed {
			n++
		}
	}
	return n
}

// labelKeys are the keys whose expected value is a label (rule 2).
var labelKeys = map[string]bool{"commit": true, "old": true, "new": true, "pin": true}

// eventKeys are the keys of an event that compare (rule 7).
var eventKeys = []string{"date", "commit", "by", "task", "event", "gate", "state", "reason", "note", "url", "old", "new", "subproject"}

// inputKeys compare nothing (rule 10).
var inputKeys = map[string]bool{"entry": true, "language": true, "trunk": true, "ref": true, "source": true, "replace": true}

// A comparer holds the state of one comparison.
type comparer struct {
	entry *corpustest.Entry
	call  func(path string) string
	facts []fact
	err   error
	obs   map[string]any
}

// compare compares the expected document of an entry with the observed one.
// The error is a corpus error: a label no file holds (rule 2).
func compare(e *corpustest.Entry, top *yaml.Node, observed map[string]any, call func(path string) string) (*outcome, error) {
	c := &comparer{entry: e, call: call, obs: observed}
	out := &outcome{}
	if call == nil {
		c.call = func(string) string { return "" }
	}
	for i := 0; i+1 < len(top.Content); i += 2 {
		k, v := top.Content[i].Value, top.Content[i+1]
		switch {
		case inputKeys[k]:
		case k == "findings":
			out.unexpected, out.observed = c.findings(v)
		case k == "tasks":
			c.tasks("tasks", v, asMap(observed["tasks"]))
		case k == "refs":
			c.refs(v, asMap(observed["refs"]))
		case k == "valid" || k == "owner" || k == "root" || k == "count" || k == "order":
			c.value(k, kindOf(k), v, observed[k], k)
		default:
			c.value(k, k, v, observed[k], k)
		}
	}
	out.facts = c.facts
	return out, c.err
}

// kindOf is the kind a top-level key counts under.
func kindOf(key string) string {
	switch key {
	case "owner", "root", "count", "order":
		return "project"
	}
	return key
}

// add records a fact.
func (c *comparer) add(f fact) {
	f.call = c.call(f.path)
	c.facts = append(c.facts, f)
}

// fail records the first corpus error.
func (c *comparer) fail(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf("corpus: "+format, args...)
	}
}

func (c *comparer) tasks(prefix string, n *yaml.Node, got map[string]any) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		id := n.Content[i].Value
		c.task(prefix+"."+id, n.Content[i+1], asMap(got[id]))
	}
}

// task compares the facts of one task.
func (c *comparer) task(prefix string, n *yaml.Node, got map[string]any) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k, v := n.Content[i].Value, n.Content[i+1]
		path := prefix + "." + k
		switch k {
		case "events":
			c.events(path, v, got[k])
		case "requires", "reviews":
			c.keyed(path, k, v, got[k])
		default:
			c.value(path, k, v, got[k], k)
		}
	}
}

// refs compares the facts of each branch the file names.
func (c *comparer) refs(n *yaml.Node, got map[string]any) {
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		branch, v := n.Content[i].Value, n.Content[i+1]
		g := asMap(got[branch])
		prefix := "refs." + branch
		for j := 0; j+1 < len(v.Content); j += 2 {
			k, w := v.Content[j].Value, v.Content[j+1]
			switch k {
			case "tasks":
				c.tasksAt(prefix+".tasks", w, asMap(g["tasks"]))
			default:
				c.value(prefix+"."+k, "refs", w, g[k], k)
			}
		}
	}
}

// tasksAt is tasks with every fact counted under refs.
func (c *comparer) tasksAt(prefix string, n *yaml.Node, got map[string]any) {
	before := len(c.facts)
	c.tasks(prefix, n, got)
	for i := before; i < len(c.facts); i++ {
		c.facts[i].kind = "refs"
	}
}

// value compares one value of any shape, by the rules that fit its shape.
func (c *comparer) value(path, kind string, n *yaml.Node, got any, name string) {
	switch n.Kind {
	case yaml.MappingNode:
		// Rule 9: a map that holds undecided compares nothing.
		if u := mapKey(n, "undecided"); u != nil {
			c.add(fact{path: path, kind: kind, line: n.Line, want: "undecided " + u.Value, got: show(got), result: undecided})
			return
		}
		g := asMap(got)
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i].Value
			c.value(path+"."+k, kind, n.Content[i+1], g[k], k)
		}
	case yaml.SequenceNode:
		switch {
		case allScalars(n):
			c.scalars(path, kind, n, got, name == "source")
		default:
			c.ordered(path, kind, n, got)
		}
	case yaml.ScalarNode:
		if name == "source" {
			// Rule 5: a single string stands for a set of one.
			c.scalars(path, kind, &yaml.Node{Kind: yaml.SequenceNode, Line: n.Line, Content: []*yaml.Node{n}}, got, true)
			return
		}
		c.scalar(path, kind, n, got, name)
	case yaml.AliasNode:
		c.value(path, kind, n.Alias, got, name)
	}
}

func allScalars(n *yaml.Node) bool {
	for _, item := range n.Content {
		if item.Kind != yaml.ScalarNode {
			return false
		}
	}
	return true
}

// scalar compares one scalar (rules 1 to 3).
func (c *comparer) scalar(path, kind string, n *yaml.Node, got any, name string) {
	f := fact{path: path, kind: kind, line: n.Line, want: n.Value, got: show(got)}
	switch {
	case n.Tag == "!!null" || n.Value == "":
		f.want = "(absent)"
		f.result = verdict(absent(got))
	case labelKeys[name]:
		hash, ok := c.entry.Hash(n.Value)
		if !ok {
			c.fail("%s:%d: the label %s names no commit of entry %s", c.entry.Name, n.Line, n.Value, c.entry.Name)
			hash = n.Value
		}
		f.want = withLabel(hash, n.Value)
		g, _ := got.(string)
		f.got = c.withLabel(g)
		f.result = verdict(g == hash)
	case n.Tag == "!!timestamp":
		g := text(got)
		f.result = verdict(len(g) >= 10 && g[:10] == n.Value[:min(10, len(n.Value))])
	default:
		f.result = verdict(!absent(got) && text(got) == n.Value)
	}
	c.add(f)
}

func verdict(ok bool) result {
	if ok {
		return agreed
	}
	return differed
}

// withLabel shows a hash and the label that names it.
func withLabel(hash, label string) string {
	if hash == label || label == "" {
		return hash
	}
	return hash + " (" + label + ")"
}

func (c *comparer) withLabel(hash string) string {
	if hash == "" {
		return "(absent)"
	}
	return withLabel(hash, c.entry.Label(hash))
}

// scalars compares a list of scalars as one fact, whole and in order, or as a
// set (rule 5).
func (c *comparer) scalars(path, kind string, n *yaml.Node, got any, set bool) {
	var want []string
	for _, item := range n.Content {
		want = append(want, item.Value)
	}
	have := textList(got)
	f := fact{path: path, kind: kind, line: n.Line, want: list(want), got: list(have)}
	if len(want) == 0 {
		f.want = "(absent)"
	}
	if len(have) == 0 {
		f.got = "(absent)"
	}
	if set {
		a, b := append([]string(nil), want...), append([]string(nil), have...)
		sort.Strings(a)
		sort.Strings(b)
		f.result = verdict(strings.Join(a, "\x00") == strings.Join(b, "\x00"))
	} else {
		f.result = verdict(strings.Join(want, "\x00") == strings.Join(have, "\x00"))
	}
	c.add(f)
}

// ordered compares a list of maps in order: its length is one fact, and each
// stated key of each item is one fact.
func (c *comparer) ordered(path, kind string, n *yaml.Node, got any) {
	items := asList(got)
	c.length(path, kind, n, len(items))
	for i, item := range n.Content {
		if i >= len(items) {
			break
		}
		c.value(fmt.Sprintf("%s[%d]", path, i), kind, item, items[i], "")
	}
}

// length records the length of a list as one fact.
func (c *comparer) length(path, kind string, n *yaml.Node, have int) {
	c.add(fact{path: path + ".length", kind: kind, line: n.Line, want: strconv.Itoa(len(n.Content)), got: strconv.Itoa(have),
		result: verdict(len(n.Content) == have)})
}

// events compares a list of events in order (rule 7).
func (c *comparer) events(path string, n *yaml.Node, got any) {
	items := asList(got)
	c.length(path, "events", n, len(items))
	for i, item := range n.Content {
		if i >= len(items) {
			break
		}
		g := asMap(items[i])
		p := fmt.Sprintf("%s[%d]", path, i)
		for _, k := range eventKeys {
			w := mapKey(item, k)
			switch {
			case w != nil:
				c.scalar(p+"."+k, "events", w, g[k], k)
			case !absent(g[k]):
				// A key the file leaves out must be absent.
				c.add(fact{path: p + "." + k, kind: "events", line: item.Line, want: "(absent)", got: show(g[k]), result: differed})
			}
		}
	}
}

// keyed compares requires and reviews as sets of items (rule 6).
func (c *comparer) keyed(path, name string, n *yaml.Node, got any) {
	items := asList(got)
	c.length(path, name, n, len(items))
	taken := make([]bool, len(items))
	for _, item := range n.Content {
		key := nodeKey(name, item)
		at := -1
		for j, g := range items {
			if !taken[j] && obsKey(name, asMap(g)) == key {
				at, taken[j] = j, true
				break
			}
		}
		p := path + "[" + key + "]"
		if at < 0 {
			c.add(fact{path: p, kind: name, line: item.Line, want: "present", got: "(absent)", result: differed})
			continue
		}
		g := asMap(items[at])
		for i := 0; i+1 < len(item.Content); i += 2 {
			k, v := item.Content[i].Value, item.Content[i+1]
			switch {
			case k == "id" || (name == "reviews" && k == "gate"):
			case k == "subproject" && v.Kind == yaml.MappingNode:
				sub := asMap(g[k])
				for j := 0; j+1 < len(v.Content); j += 2 {
					if sk := v.Content[j].Value; sk != "url" && sk != "id" {
						c.value(p+"."+k+"."+sk, name, v.Content[j+1], sub[sk], sk)
					}
				}
			default:
				c.value(p+"."+k, name, v, g[k], k)
			}
		}
	}
}

// nodeKey is the key of an expected item of requires or reviews.
func nodeKey(name string, item *yaml.Node) string {
	if name == "reviews" {
		return textOf(mapKey(item, "gate"))
	}
	if sub := mapKey(item, "subproject"); sub != nil {
		return textOf(mapKey(sub, "url")) + "@" + textOf(mapKey(sub, "id"))
	}
	return textOf(mapKey(item, "id"))
}

// obsKey is the key of an observed item.
func obsKey(name string, item map[string]any) string {
	if name == "reviews" {
		return text(item["gate"])
	}
	if sub := asMap(item["subproject"]); len(sub) > 0 {
		return text(sub["url"]) + "@" + text(sub["id"])
	}
	return text(item["id"])
}

func textOf(n *yaml.Node) string {
	if n == nil {
		return ""
	}
	return n.Value
}

// mapKey returns the value of a key of a mapping node, or nil.
func mapKey(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// statedFindings reads the findings an expected.yaml states, with their lines.
func statedFindings(list *yaml.Node) []finding {
	var out []finding
	if list == nil {
		return nil
	}
	for _, item := range list.Content {
		out = append(out, finding{
			rule: textOf(mapKey(item, "rule")), severity: textOf(mapKey(item, "severity")),
			task: textOf(mapKey(item, "task")), file: textOf(mapKey(item, "file")), gate: textOf(mapKey(item, "gate")),
			commit: textOf(mapKey(item, "commit")), trailer: textOf(mapKey(item, "trailer")),
			message: textOf(mapKey(item, "message")), line: item.Line,
		})
	}
	return out
}

// findings compares the findings as a multiset (rule 8) and returns the observed
// findings no expected finding takes, and the number observed.
func (c *comparer) findings(n *yaml.Node) ([]finding, int) {
	observed := asList(c.obs["findings"])
	have := make([]finding, len(observed))
	for i, o := range observed {
		m := asMap(o)
		have[i] = finding{rule: text(m["rule"]), severity: text(m["severity"]), task: text(m["task"]), file: text(m["file"]),
			gate: text(m["gate"]), commit: text(m["commit"]), trailer: text(m["trailer"]), message: text(m["message"])}
	}
	want := statedFindings(n)
	// The expected findings take their observed finding in the order of most
	// keys first.
	order := make([]int, len(want))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(a, b int) bool { return stated(want[order[a]]) > stated(want[order[b]]) })
	taken := make([]bool, len(have))
	matched := make([]int, len(want))
	for i := range matched {
		matched[i] = -1
	}
	for _, i := range order {
		w := want[i]
		hash := w.commit
		if hash != "" {
			h, ok := c.entry.Hash(w.commit)
			if !ok {
				c.fail("%s:%d: the label %s names no commit of entry %s", c.entry.Name, w.line, w.commit, c.entry.Name)
			}
			hash = h
		}
		for j, h := range have {
			if taken[j] || h.rule != w.rule || h.severity != w.severity {
				continue
			}
			if (w.task != "" && h.task != w.task) || (w.file != "" && h.file != w.file) || (w.gate != "" && h.gate != w.gate) ||
				(w.trailer != "" && h.trailer != w.trailer) || (hash != "" && h.commit != hash) {
				continue
			}
			taken[j], matched[i] = true, j
			break
		}
	}
	for i, w := range want {
		f := fact{path: "findings", kind: "findings", line: w.line, rule: w.rule, want: c.describe(w, true)}
		if matched[i] >= 0 {
			f.result, f.got = agreed, c.describe(have[matched[i]], false)
		} else {
			f.result, f.got = differed, "(no such finding)"
		}
		c.add(f)
	}
	var left []finding
	for j, h := range have {
		if !taken[j] {
			h.text = c.describe(h, false)
			left = append(left, h)
		}
	}
	sort.SliceStable(left, func(a, b int) bool {
		x, y := left[a], left[b]
		if x.rule != y.rule {
			return x.rule < y.rule
		}
		if x.file != y.file {
			return x.file < y.file
		}
		return x.task < y.task
	})
	return left, len(have)
}

// stated counts the keys of a finding beyond rule and severity that it states.
func stated(f finding) int {
	n := 0
	for _, v := range []string{f.task, f.file, f.gate, f.commit, f.trailer} {
		if v != "" {
			n++
		}
	}
	return n
}

// describe writes a finding as "R9 warning task=c07d file=tasks/c07d.yaml".
func (c *comparer) describe(f finding, expected bool) string {
	parts := []string{f.rule, f.severity}
	for _, kv := range [][2]string{{"task", f.task}, {"file", f.file}, {"gate", f.gate}, {"commit", f.commit}, {"trailer", f.trailer}} {
		if kv[1] == "" {
			continue
		}
		v := kv[1]
		if kv[0] == "commit" {
			if expected {
				if h, ok := c.entry.Hash(v); ok {
					v = withLabel(h, v)
				}
			} else {
				v = c.withLabel(v)
			}
		}
		parts = append(parts, kv[0]+"="+v)
	}
	s := strings.Join(parts, " ")
	if !expected && f.message != "" {
		s += ": " + f.message
	}
	return s
}

// Helpers over the decoded JSON of the observed document.

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func asList(v any) []any {
	switch l := v.(type) {
	case []any:
		return l
	case []string:
		out := make([]any, len(l))
		for i, s := range l {
			out[i] = s
		}
		return out
	}
	return nil
}

// absent is rule 3: observed absent, null, "" and [] are one value.
func absent(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	case []string:
		return len(x) == 0
	case map[string]any:
		return len(x) == 0
	}
	return false
}

// text renders a scalar as the text the file would hold.
func text(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		if x == float64(int64(x)) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	}
	return fmt.Sprint(v)
}

func textList(v any) []string {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	}
	var out []string
	for _, item := range asList(v) {
		out = append(out, text(item))
	}
	return out
}

func list(items []string) string { return "[" + strings.Join(items, ", ") + "]" }

// show renders an observed value for a message.
func show(v any) string {
	if absent(v) {
		return "(absent)"
	}
	switch x := v.(type) {
	case []any, []string:
		return list(textList(x))
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + ": " + show(x[k])
		}
		return "{" + strings.Join(parts, ", ") + "}"
	}
	return text(v)
}
