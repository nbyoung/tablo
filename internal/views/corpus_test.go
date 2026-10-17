package views

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
)

// expectation is one valid entry of the corpus: its expected.yaml and what a
// test needs to read the entry as README.md of the corpus says a tool does.
type expectation struct {
	name    string
	want    map[string]any
	dir     string            // the repository
	ref     string            // the branch in view
	trunk   string            // the branch the caller names, or ""
	replace map[string]string // absolute URL to clone
	labels  map[string]string // label to commit, the entry's and its subprojects'

	mu    sync.Mutex
	facts map[string]*derive.Facts // by ref
}

// The valid entries, read once.
var (
	entriesOnce sync.Once
	entriesList []*expectation
	entriesErr  error
)

// expectations returns the built valid entries of the corpus that state a
// derived fact, by name. The entries' expected.yaml files lie beside the build, under
// ../entries; the test skips when they are absent.
func expectations(t testing.TB) []*expectation {
	t.Helper()
	build := corpus(t)
	entriesOnce.Do(func() { entriesList, entriesErr = readExpectations(build) })
	if entriesErr != nil {
		t.Skipf("the corpus's expectations are absent: %v", entriesErr)
	}
	return entriesList
}

func readExpectations(build string) ([]*expectation, error) {
	root := filepath.Join(filepath.Dir(build), "entries")
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var list []*expectation
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(root, d.Name(), "expected.yaml"))
		if err != nil {
			continue
		}
		e := &expectation{name: d.Name(), dir: filepath.Join(build, d.Name()), ref: "main",
			replace: map[string]string{}, labels: map[string]string{}, facts: map[string]*derive.Facts{}}
		if err := yaml.Unmarshal(data, &e.want); err != nil {
			return nil, fmt.Errorf("%s: %v", d.Name(), err)
		}
		if e.want["valid"] != true || e.want["tasks"] == nil {
			continue
		}
		if ref := text(e.want["ref"]); ref != "" {
			e.ref = ref
		}
		if _, inPlace := e.want["source"]; inPlace {
			// tableaux-tooling reads the umbrella in place at a tag the owner
			// moves: the conformance run compares its facts, and T25 reads that
			// plan by the rule.
			continue
		}
		if trunk, ok := e.want["trunk"].(map[string]any); ok {
			e.trunk = text(trunk["caller"])
		}
		if replace, ok := e.want["replace"].(map[string]any); ok {
			for url, name := range replace {
				e.replace[url] = filepath.Join(build, d.Name()+"."+text(name))
			}
		}
		files, _ := filepath.Glob(filepath.Join(build, d.Name()+".*labels.txt"))
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			for _, line := range regexp.MustCompile(`(?m)^(\S+) ([0-9a-f]+)$`).FindAllStringSubmatch(string(data), -1) {
				if known, ok := e.labels[line[1]]; ok && known != line[2] {
					return nil, fmt.Errorf("%s: the label %s names two commits", d.Name(), line[1])
				}
				e.labels[line[1]] = line[2]
			}
		}
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	return list, nil
}

// at derives the entry's facts at a ref and keeps them; a Facts is immutable.
func (e *expectation) at(t testing.TB, ref string) *derive.Facts {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	if f, ok := e.facts[ref]; ok {
		return f
	}
	f := derived(t, load.Options{Replace: e.replace}, e.dir, ref, e.trunk)
	e.facts[ref] = f
	return f
}

// hash returns the commit a label names, or the text itself where it is none.
func (e *expectation) hash(label string) string {
	if hash, ok := e.labels[label]; ok {
		return hash
	}
	return label
}

// text returns a scalar of expected.yaml as a string: a date as YYYY-MM-DD.
func text(v any) string {
	switch v := v.(type) {
	case nil:
		return ""
	case string:
		return v
	case time.Time:
		return v.Format("2006-01-02")
	}
	return fmt.Sprint(v)
}

// texts returns a list of expected.yaml as strings; a scalar is a list of one.
func texts(v any) []string {
	list, ok := v.([]any)
	if !ok {
		if v == nil {
			return nil
		}
		return []string{text(v)}
	}
	out := make([]string, 0, len(list))
	for _, item := range list {
		out = append(out, text(item))
	}
	return out
}

// check counts the facts a test compares and reports each that differs.
type check struct {
	t     testing.TB
	n     int
	where string
}

func (c *check) eq(what string, got, want any) {
	c.t.Helper()
	c.n++
	if !reflect.DeepEqual(got, want) {
		c.t.Errorf("%s: %s is %#v; the corpus states %#v", c.where, what, got, want)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// entry derives a built corpus repository at main, with the clones its
// expected.yaml maps a URL to.
func entry(t testing.TB, name string) *derive.Facts {
	t.Helper()
	for _, e := range expectations(t) {
		if e.name == name {
			return e.at(t, "main")
		}
	}
	return derived(t, load.Options{}, builtAt(t, name), "main", "")
}
