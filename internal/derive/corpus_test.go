package derive

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

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
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
	facts map[string]*Facts // by ref
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
			replace: map[string]string{}, labels: map[string]string{}, facts: map[string]*Facts{}}
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
			// tableaux-tooling names the umbrella at a tag the owner moves, and
			// its facts drift from the tag: T25 reads that plan by the rule.
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
func (e *expectation) at(t testing.TB, ref string) *Facts {
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

// eachTask runs a comparison for every task an entry states facts about, in
// every valid entry, and fails when the corpus states no such fact.
func eachTask(t *testing.T, compare func(c *check, e *expectation, f *Facts, id string, want map[string]any)) {
	t.Helper()
	c := &check{t: t}
	for _, e := range expectations(t) {
		f := e.at(t, e.ref)
		if f.Refused() != nil {
			t.Errorf("%s: refused: %+v", e.name, f.Refused())
			continue
		}
		tasks, _ := e.want["tasks"].(map[string]any)
		for _, id := range sortedKeys(tasks) {
			want, _ := tasks[id].(map[string]any)
			c.where = e.name + " " + id
			compare(c, e, f, id, want)
		}
	}
	if c.n == 0 {
		t.Error("the corpus states no such fact")
	}
	t.Logf("%d facts agree or are reported", c.n)
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// The tree: owner, root, count and order, and each task's parent and authorities.
func TestCorpusTree(t *testing.T) {
	c := &check{t: t}
	for _, e := range expectations(t) {
		f := e.at(t, e.ref)
		if f.Refused() != nil {
			continue
		}
		c.where = e.name
		if want, ok := e.want["owner"]; ok {
			c.eq("the owner", f.Owner(), text(want))
		}
		if want, ok := e.want["root"]; ok {
			c.eq("the root", f.Root(), text(want))
		}
		if want, ok := e.want["order"]; ok {
			c.eq("the order", f.Order(), texts(want))
		}
		if want, ok := e.want["count"].(map[string]any); ok {
			leaves := 0
			for _, id := range f.Order() {
				if f.Leaf(id) {
					leaves++
				}
			}
			c.eq("the count of tasks", len(f.Order()), want["tasks"])
			c.eq("the count of leaves", leaves, want["leaves"])
		}
	}
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		if parent, ok := want["parent"]; ok {
			c.eq("the parent", f.parent[id], text(parent))
		}
		if authorities, ok := want["authorities"]; ok {
			got := []string{}
			for _, a := range f.Authorities(id) {
				got = append(got, a.Email)
			}
			c.eq("the authorities", got, append([]string{}, texts(authorities)...))
		}
	})
	t.Logf("%d project facts agree or are reported", c.n)
}

// T7, the corpus: every junction, source and applicable list it states. The
// applicable lists are also the ones the Validator's own resolution agrees
// with (decision 8 (d)).
func TestCorpusJunctions(t *testing.T) {
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		if applicable, ok := want["applicable"]; ok {
			c.eq("the applicable gates", f.Applicable(id), texts(applicable))
		}
		junctions, _ := want["junctions"].(map[string]any)
		for _, gate := range sortedKeys(junctions) {
			stated, _ := junctions[gate].(map[string]any)
			if gate == "default" {
				// The plain default: the assignee contributes and nobody reviews.
				c.eq("the default's kind", "plain", text(stated["kind"]))
				c.eq("the default's contributor", f.Project().Tasks[id].Assignee.V, text(stated["contributor"]))
				continue
			}
			j := f.Junction(id, gate)
			if j == nil {
				c.eq("the junction at "+gate, nil, stated)
				continue
			}
			at := gate + "'s "
			c.eq(at+"kind", j.Kind.String(), text(stated["kind"]))
			c.eq(at+"sources", j.Sources(), texts(stated["source"]))
			c.eq(at+"contributor", j.Contributor.V, text(stated["contributor"]))
			c.eq(at+"model", j.Model.V, text(stated["model"]))
			c.eq(at+"reviewer", j.Reviewer.V, text(stated["reviewer"]))
			var references []map[string]any
			for _, r := range j.References {
				references = append(references, map[string]any{"url": r.URL.V, "text": r.Text.V})
			}
			var wantReferences []map[string]any
			if list, ok := stated["references"].([]any); ok {
				for _, r := range list {
					wantReferences = append(wantReferences, r.(map[string]any))
				}
			}
			c.eq(at+"references", references, wantReferences)
			if j.Kind != model.Plain {
				c.eq(at+"entry", j.Entry, texts(stated["source"])[0])
			}
			if sub, ok := stated["subproject"].(map[string]any); ok {
				if j.Snapshot == nil {
					c.eq(at+"snapshot", nil, sub)
					continue
				}
				c.eq(at+"url", j.Snapshot.Entry.URL.V, text(sub["url"]))
				if target, ok := sub["id"]; ok {
					c.eq(at+"target", j.Snapshot.Target, text(target))
				}
				if commit, ok := sub["commit"]; ok {
					c.eq(at+"commit", j.Snapshot.Link.Commit, e.hash(text(commit)))
				}
			}
		}
	})
}

// T9, the corpus: every requirement's gates and condition.
func TestCorpusConditions(t *testing.T) {
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		list, ok := want["requires"].([]any)
		if !ok {
			return
		}
		got := f.Requires(id)
		if c.eq("the count of requirements", len(got), len(list)); len(got) != len(list) {
			return
		}
		for i, item := range list {
			stated := item.(map[string]any)
			cond := got[i]
			at := fmt.Sprintf("requirement %d's ", i)
			if sub, ok := stated["subproject"].(map[string]any); ok {
				c.eq(at+"origin", cond.Origin, text(sub["id"]))
				if cond.Link == nil {
					c.eq(at+"link", nil, sub)
					continue
				}
				c.eq(at+"url", cond.Link.URL, text(sub["url"]))
				c.eq(at+"pin", cond.Link.Commit, e.hash(text(sub["pin"])))
			} else {
				c.eq(at+"origin", cond.Origin, text(stated["id"]))
				c.eq(at+"link", cond.Link, (*model.Link)(nil))
			}
			c.eq(at+"from", cond.From, text(stated["from"]))
			c.eq(at+"to", cond.To, text(stated["to"]))
			c.eq(at+"met", cond.Met, stated["met"])
			c.eq(at+"due", cond.Due, stated["due"])
			c.eq(at+"condition", cond.Word(), text(stated["condition"]))
			c.eq(at+"reason", cond.Why, Determined)
		}
	})
}

// compareStatus compares a status: the gate, the state, the reason and the
// note, whether a roll-up gives them and the child they come from; then
// what the history gives, the date, the recorder and the deciding commit.
func compareStatus(c *check, e *expectation, s *Status, stated map[string]any) {
	c.t.Helper()
	c.eq("the status's date", s.Date, text(stated["date"]))
	c.eq("the status's recorder", s.Recorder, text(stated["recorder"]))
	commit := ""
	if s.Commit != nil {
		commit = s.Commit.ID
	}
	if stated["derived"] == true {
		c.eq("the status's commit", commit, "")
	} else {
		c.eq("the status's commit", commit, e.hash(text(stated["commit"])))
	}
	c.eq("the status's gate", s.Gate, text(stated["gate"]))
	if state, ok := stated["state"]; ok {
		c.eq("the status's state", s.State, text(state))
	}
	c.eq("the status's reason", s.Reason, text(stated["reason"]))
	c.eq("the status's note", s.Note, text(stated["note"]))
	c.eq("the status's roll-up", s.Derived(), stated["derived"] == true)
	if from, ok := stated["from"]; ok {
		source := s
		if s.Kind == Snapshotted && s.Of != nil {
			source = s.Of
		}
		c.eq("the status's child", source.From, text(from))
	}
}

// T13, the corpus: every derived status it states, with the date of the
// oldest child considered.
func TestCorpusRollUp(t *testing.T) {
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		stated, ok := want["status"].(map[string]any)
		if !ok || stated["derived"] != true {
			return
		}
		s := f.Status(id)
		if s == nil {
			c.eq("the status", nil, stated)
			return
		}
		compareStatus(c, e, s, stated)
		if f.Leaf(id) {
			c.eq("the status's kind", s.Kind, Snapshotted)
		} else {
			c.eq("the status's kind", s.Kind, RolledUp)
			c.eq("the status's file", s.File, (*model.Status)(nil))
		}
	})
}

// T11, the corpus: every status of a leaf it states, with its date, its
// recorder and its deciding commit: from its file, from the task its next
// junction reads, or undefined without a file and dated by the task file.
func TestCorpusLeafStatus(t *testing.T) {
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		stated, ok := want["status"].(map[string]any)
		if !ok || stated["derived"] == true {
			return
		}
		s := f.Status(id)
		if s == nil {
			c.eq("the status", nil, stated)
			return
		}
		compareStatus(c, e, s, stated)
		if _, recursive := want["subproject"]; recursive && s.Gate != "undefined" {
			c.eq("the status's kind", s.Kind, Snapshotted)
		}
	})
}

// T10, the corpus: every authorisation it states, in view and at each ref.
func TestCorpusAuthorisation(t *testing.T) {
	compare := func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		stated, ok := want["authorisation"].(map[string]any)
		if !ok {
			return
		}
		a := f.Authorisation(id)
		if a == nil {
			c.eq("the authorisation", nil, stated)
			return
		}
		state := "proposed"
		if a.Authorised {
			state = "authorised"
		}
		c.eq("the authorisation", state, text(stated["state"]))
		if commit, ok := stated["commit"]; ok {
			if a.Commit == nil {
				c.eq("the deciding commit", nil, commit)
			} else {
				c.eq("the deciding commit", a.Commit.ID, e.hash(text(commit)))
			}
		}
		if by, ok := stated["by"]; ok {
			c.eq("the authorisation's hand", a.By, text(by))
		}
	}
	eachTask(t, compare)
	c := &check{t: t}
	for _, e := range expectations(t) {
		// How the trunk resolves, as the entry's trunk field states it.
		if trunk, ok := e.want["trunk"].(map[string]any); ok {
			f := e.at(t, e.ref)
			c.where = e.name
			for how, name := range trunk {
				c.eq("how the trunk resolves", f.Trunk().How(), how)
				if how != "undetermined" {
					c.eq("the trunk's name", f.Trunk().Name, text(name))
				}
			}
		}
		refs, _ := e.want["refs"].(map[string]any)
		for _, ref := range sortedKeys(refs) {
			stated := refs[ref].(map[string]any)
			f := e.at(t, ref)
			c.where = e.name + " at " + ref
			c.eq("the source", f.Log().Source, e.hash(text(stated["commit"])))
			tasks, _ := stated["tasks"].(map[string]any)
			for _, id := range sortedKeys(tasks) {
				c.where = e.name + " at " + ref + " " + id
				compare(c, e, f, id, tasks[id].(map[string]any))
				if a := f.Authorisation(id); a != nil && !f.OnTrunk() {
					c.eq("the reason", a.Why, OffTrunk)
				}
			}
		}
	}
	if c.n == 0 {
		t.Error("the corpus states no ref")
	}
	t.Logf("%d facts of the trunks and the refs agree or are reported", c.n)
}

// T12, the corpus: every reviews list it states.
func TestCorpusReviews(t *testing.T) {
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		list, ok := want["reviews"].([]any)
		if !ok {
			return
		}
		var got, stated []string
		for _, r := range f.Reviews(id) {
			commit := "none"
			if r.Commit != nil {
				commit = r.Commit.ID
			}
			got = append(got, r.Gate+" "+r.Reviewer+" "+commit)
			if r.Task != id || r.ByAuthorisation != (r.Gate == "defined") || (r.Commit != nil) != (r.By != "") {
				c.t.Errorf("%s: the review is %+v", c.where, r)
			}
		}
		for _, item := range list {
			r := item.(map[string]any)
			stated = append(stated, text(r["gate"])+" "+text(r["reviewer"])+" "+e.hash(text(r["commit"])))
		}
		c.eq("the reviews", got, stated)
	})
}

// T14, the corpus: every events list it states, in order.
func TestCorpusEvents(t *testing.T) {
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		list, ok := want["events"].([]any)
		if !ok {
			return
		}
		keys := []string{"date", "commit", "by", "task", "event", "subproject", "gate", "state", "reason", "note", "url", "old", "new"}
		var got, stated []string
		for _, ev := range f.Events(id) {
			got = append(got, fmt.Sprint([]string{ev.Commit.Date(), ev.Commit.ID, ev.Commit.Author.Email, ev.Task, string(ev.Kind),
				ev.Sub, ev.Gate, ev.State, ev.Reason, ev.Note, ev.URL, ev.Old, ev.New}))
		}
		for _, item := range list {
			ev := item.(map[string]any)
			var fields []string
			for _, key := range keys {
				value := text(ev[key])
				if key == "commit" || key == "old" || key == "new" {
					value = e.hash(value)
				}
				fields = append(fields, value)
			}
			stated = append(stated, fmt.Sprint(fields))
		}
		if c.n++; len(got) != len(stated) {
			c.t.Errorf("%s: %d events; the corpus states %d\n got  %v\n want %v", c.where, len(got), len(stated), got, stated)
			return
		}
		for i := range got {
			c.eq(fmt.Sprintf("event %d", i), got[i], stated[i])
		}
	})
}

// T17, the corpus: the subproject each recursive junction reads, its pin
// and the task read there.
func TestCorpusSubprojects(t *testing.T) {
	eachTask(t, func(c *check, e *expectation, f *Facts, id string, want map[string]any) {
		stated, ok := want["subproject"].(map[string]any)
		if !ok {
			return
		}
		snapshots := f.Snapshots(id)
		if c.eq("the count of snapshots", len(snapshots), 1); len(snapshots) != 1 {
			return
		}
		s := snapshots[0]
		c.eq("the subproject's url", s.Link.URL, text(stated["url"]))
		c.eq("the subproject's task", s.Target, text(stated["task"]))
		c.eq("the subproject's reason", s.Why, Determined)
		pin := f.Pin(s.Link)
		c.eq("the subproject's pin", pin.Commit, e.hash(text(stated["pin"])))
		if s.Link.Form == model.Submodule {
			c.eq("the gitlink of the source", pin.Recorded, pin.Commit)
			c.eq("the checkout", pin.Moved, false)
		}
	})
}
