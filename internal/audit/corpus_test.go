package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/internal/validate"
)

// str returns a scalar of expected.yaml as a string.
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// entry is one entry of the corpus that the build makes a repository of:
// its expected.yaml and what a test needs to read it as the corpus's
// README.md says a tool does.
type entry struct {
	name    string
	want    map[string]any
	dir     string            // the repository
	refs    []string          // the ref in view, then the others the entry states, by name
	trunk   string            // the branch the caller names, or ""
	replace map[string]string // absolute URL to clone
	labels  map[string]string // label to commit, the entry's and its subprojects'
	names   map[string]string // commit to label
}

// entries returns the built entries of the corpus, by name. Their
// expected.yaml files lie beside the build, under ../entries. It leaves out
// an entry that reads a plan in place, whose facts drift.
func entries(t testing.TB) []*entry {
	t.Helper()
	build := corpus(t)
	root := filepath.Join(filepath.Dir(build), "entries")
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("the corpus's expectations are absent: %v", err)
	}
	var list []*entry
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(root, d.Name(), "expected.yaml"))
		if err != nil {
			continue
		}
		e := &entry{name: d.Name(), dir: filepath.Join(build, d.Name()), refs: []string{"main"},
			replace: map[string]string{}, labels: map[string]string{}, names: map[string]string{}}
		if err := yaml.Unmarshal(data, &e.want); err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		if _, inPlace := e.want["source"]; inPlace {
			continue
		}
		if _, err := os.Stat(e.dir); err != nil {
			continue
		}
		if ref := str(e.want["ref"]); ref != "" {
			e.refs[0] = ref
		}
		if m, ok := e.want["trunk"].(map[string]any); ok {
			e.trunk = str(m["caller"])
		}
		if m, ok := e.want["replace"].(map[string]any); ok {
			for url, name := range m {
				e.replace[url] = filepath.Join(build, d.Name()+"."+str(name))
			}
		}
		files, _ := filepath.Glob(filepath.Join(build, d.Name()+".*labels.txt"))
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range regexp.MustCompile(`(?m)^(\S+) ([0-9a-f]+)$`).FindAllStringSubmatch(string(data), -1) {
				e.labels[line[1]] = line[2]
				e.names[line[2]] = line[1]
			}
		}
		if m, ok := e.want["refs"].(map[string]any); ok {
			for ref := range m {
				e.refs = append(e.refs, ref)
			}
			sort.Strings(e.refs[1:])
		}
		list = append(list, e)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].name < list[j].name })
	return list
}

// at derives the entry's facts at a ref.
func (e *entry) at(t testing.TB, ref string) *derive.Facts {
	t.Helper()
	return derived(t, load.Options{Replace: e.replace}, e.dir, ref, e.trunk)
}

// diagnostics gives what the Validator reports of an entry. No adapter fills
// validate.Facts from derive.Facts before the Plumbing command (4ed9), so
// the test builds them in three parts: validate.Files for real;
// validate.Derived with the determined requirements alone, for R9; and one
// literal diagnostic per finding the entry's expected.yaml states under a
// rule of the history, with the entry's own message, H4 and H5 placed in the
// status file. want is nil at a ref the entry states no finding for.
func diagnostics(f *derive.Facts, want map[string]any, labels map[string]string) []model.Diagnostic {
	p := f.Project()
	out := validate.Files(p)
	facts := &validate.Facts{}
	for _, id := range f.Order() {
		for i, c := range f.Requires(id) {
			if c.Why != derive.Determined {
				continue
			}
			facts.Requirements = append(facts.Requirements, validate.Condition{Task: id, Index: i, Origin: c.Origin,
				From: c.From, To: c.To, Stands: c.Stands, Met: c.Met, Due: c.Due, MetAtTip: c.AtTrunk == derive.Yes})
		}
	}
	out = append(out, validate.Derived(p, facts)...)
	list, _ := want["findings"].([]any)
	for _, item := range list {
		m := item.(map[string]any)
		id := str(m["rule"])
		known, _ := validate.RuleOf(id)
		if known.Tier != validate.TierHistory {
			continue
		}
		d := model.Diagnostic{Code: id, Severity: known.Severity, Task: str(m["task"]), Gate: str(m["gate"]),
			Commit: labels[str(m["commit"])], Trailer: str(m["trailer"]), Message: str(m["message"])}
		d.Pos.File = str(m["file"])
		if id == "H4" || id == "H5" {
			d.Pos.File = "status/" + d.Task + ".yaml"
		}
		out = append(out, d)
	}
	validate.Sort(out)
	return out
}

// now returns the author date of the source commit, the date the corpus
// tests pass, or 2026-09-01 where the source has no commit.
func now(f *derive.Facts) string {
	if log := f.Log(); log != nil {
		if c := log.Commit(log.Source); c != nil {
			return c.Date()
		}
	}
	return "2026-09-01"
}

// tasksOf returns the tasks of the findings with a key, in their order.
func tasksOf(rep *Report, key string) []string {
	var ids []string
	for _, x := range of(rep, key) {
		ids = append(ids, x.Task)
	}
	return ids
}

// T3: a proposed task agrees with the corpus (decision 6). On the trunk the
// proposed findings name exactly the tasks whose authorisation.state the
// entry states as proposed. Off the trunk they name the tasks the branch
// changes, and with no trunk none: P5 stands for them.
func TestProposedAgreesWithTheCorpus(t *testing.T) {
	onTrunk, proposed := 0, 0
	for _, e := range entries(t) {
		if e.want["valid"] != true {
			continue
		}
		f := e.at(t, e.refs[0])
		if f.Refused() != nil || !f.OnTrunk() {
			continue
		}
		tasks, _ := e.want["tasks"].(map[string]any)
		var want []string
		for _, id := range f.Order() {
			task, _ := tasks[id].(map[string]any)
			if a, _ := task["authorisation"].(map[string]any); str(a["state"]) == "proposed" {
				want = append(want, id)
			}
		}
		rep := audited(t, Input{Facts: f, Now: now(f)})
		if got := tasksOf(rep, Proposed); fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("%s at %s: the proposed tasks are %v, the corpus states %v", e.name, e.refs[0], got, want)
		}
		onTrunk++
		proposed += len(want)
		for _, x := range of(rep, Proposed) {
			if x.Severity.String() != "warning" || x.Act != Authorise || x.Rule != "" || x.Diagnostic != nil || x.Gate != "" ||
				x.File != ".tableaux/tasks/"+x.Task+".yaml" || x.Kind != "A proposed task" || x.Sentence == "" || x.Source.URL == "" {
				t.Errorf("%s: the proposed task reads %+v", e.name, x)
			}
		}
	}
	if onTrunk < 20 || proposed != 2 {
		t.Errorf("%d entries on their trunk state %d proposed tasks; the corpus states 3c5d and c3d7 among more than 20", onTrunk, proposed)
	}

	cases := []struct {
		entry, ref string
		want       string
		message    string
	}{
		{"weather-station", "main", "[3c5d]", "The task stands proposed: dan@example.org, the author of its deciding commit, is none of its authorities"},
		{"trunk-inferred", "main", "[c3d7]", "The task stands proposed: pat@example.org, the author of its deciding commit, is none of its authorities"},
		// Off the trunk: every task reads as proposed, and the audit reports those the branch changes.
		{"weather-station", "sensor-board", "[]", ""},
		{"trunk-stated", "develop", "[]", ""},
		{"trunk-stated", "main", "[c3d7]", "The task stands proposed: the source changes its file and is off the trunk `develop`"},
		// No trunk: P5 says it once.
		{"trunk-undetermined", "main", "[]", ""},
	}
	for _, c := range cases {
		f := derived(t, load.Options{}, builtAt(t, c.entry), c.ref, "")
		rep := audited(t, Input{Facts: f, Now: now(f)})
		found := of(rep, Proposed)
		if got := fmt.Sprint(tasksOf(rep, Proposed)); got != c.want {
			t.Errorf("%s at %s: the proposed tasks are %s, want %s", c.entry, c.ref, got, c.want)
			continue
		}
		if len(found) == 1 && (found[0].Message != c.message || found[0].Resolver != f.Owner()) {
			t.Errorf("%s at %s: %q for %q", c.entry, c.ref, found[0].Message, found[0].Resolver)
		}
	}
	off := derived(t, load.Options{}, builtAt(t, "weather-station"), "sensor-board", "")
	if a := off.Authorisation("9f31"); a.Authorised || a.Why != derive.OffTrunk {
		t.Errorf("9f31 at sensor-board reads %+v; the fact stands and the finding is narrower", a)
	}
	none := derived(t, load.Options{}, builtAt(t, "trunk-undetermined"), "main", "")
	if none.Trunk().Tip != "" {
		t.Errorf("trunk-undetermined has the trunk %+v", none.Trunk())
	}
}
