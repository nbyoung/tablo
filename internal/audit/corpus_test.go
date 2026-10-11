package audit

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
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

// samples names the entries the golden file shows though each of their
// findings resolves by a revision.
var samples = map[string]bool{"file-duplicate-key": true, "gates-bad-key": true, "task-bad-filename": true,
	"siblings-same-order": true, "status-on-parent": true, "status-no-task": true, "subproject-path-missing": true}

// factNames names the typed facts a finding holds, in the order of Facts.
func factNames(f Facts) []string {
	var has []string
	for _, x := range []struct {
		name string
		set  bool
	}{
		{"junction", f.Junction != nil}, {"status", f.Status != nil}, {"requirement", f.Requirement != nil},
		{"authorisation", f.Authorisation != nil}, {"linkage", f.Snapshot != nil}, {"model", f.Reading != nil},
	} {
		if x.set {
			has = append(has, x.name)
		}
	}
	return has
}

// corpusOut names the file TestCorpus writes its text to, in place of the
// comparison: TestTwoClones runs the test binary again to read it.
const corpusOut = "TABLO_AUDIT_CORPUS_OUT"

// auditOfTheCorpus writes what the Audit reports on the corpus: one block per entry and
// ref, at the minimum information so that every row shows, with Now the
// author date of the source commit. An entry prints when one of its findings
// has an act other than revise and is no sole review, or is P5, or when
// samples names it; a second block prints the stale statuses at three days,
// where there are any; an entry the Derivation refuses prints "refused". A
// row prints its typed facts by name, per task, and a row of sole reviews
// prints one line. A commit prints as the label the build gives it.
func auditOfTheCorpus(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, e := range entries(t) {
		for i, ref := range e.refs {
			f := e.at(t, ref)
			var want map[string]any
			if i == 0 {
				want = e.want
			}
			for _, stale := range []int{0, 3} {
				rep, err := Run(Input{Facts: f, Diagnostics: diagnostics(f, want, e.labels), Now: now(f), Stale: stale})
				if err != nil {
					t.Fatalf("%s %s: %v", e.name, ref, err)
				}
				if rep == nil {
					if stale == 0 {
						fmt.Fprintf(&b, "%s %s: refused\n", e.name, ref)
					}
					continue
				}
				if stale == 3 {
					only := rep.Keep(func(x *Finding) bool { return x.Key == Stale })
					if len(only.Findings) == 0 {
						continue
					}
					rep = only
				}
				plain := true
				for _, x := range rep.Findings {
					plain = plain && (x.Act == Revise || x.Key == SoleReview) && x.Key != "P5"
				}
				if plain && !samples[e.name] {
					continue
				}
				c := rep.Counts()
				fmt.Fprintf(&b, "%s %s: now %s, stale %d: %d error(s), %d warning(s), %d information", e.name, ref, rep.Now, rep.Stale, c.Errors, c.Warnings, c.Information)
				if m := rep.Most(); m != nil {
					fmt.Fprintf(&b, "; most %s %d", m.Email, m.Count)
				}
				b.WriteString("\n")
				for _, row := range rep.Rows(model.Information) {
					x := row[0]
					var ids, files, commits []string
					seen := map[string]bool{}
					for _, y := range row {
						if y.Task != "" {
							ids = append(ids, y.Task)
						}
						if y.File != "" && !seen[y.File] {
							seen[y.File] = true
							files = append(files, y.File)
						}
						for _, c := range y.Commits {
							if !seen[c.ID] {
								seen[c.ID] = true
								commits = append(commits, e.names[c.ID])
							}
						}
					}
					fmt.Fprintf(&b, "  %s %s [%s] at %q in [%s]: %s -> %s\n", x.Key, x.Severity, strings.Join(ids, " "), x.Gate, strings.Join(files, " "), x.Act, x.Resolver)
					if x.Sentence == "" || x.Source.URL == "" || x.Kind == "" {
						t.Errorf("%s %s: no sentence, source or kind", e.name, x.Key)
					}
					if x.Key == SoleReview {
						continue
					}
					fmt.Fprintf(&b, "    message: %s\n    commits: %s\n", x.Message, strings.Join(commits, " "))
					for _, y := range row {
						if has := factNames(y.Facts); len(has) > 0 {
							fmt.Fprintf(&b, "    facts %s: %s\n", y.Task, strings.Join(has, " "))
						}
					}
					for _, c := range x.Commands {
						text := c.Text
						for hash, name := range e.names {
							text = strings.ReplaceAll(text, hash, "<"+name+">")
						}
						fmt.Fprintf(&b, "    $ %s  # %s\n", text, c.Comment)
					}
				}
			}
		}
	}
	return b.String()
}

// T2: the Audit on the corpus, against testdata/corpus.expected.txt.
func TestCorpus(t *testing.T) {
	got := auditOfTheCorpus(t)
	if out := os.Getenv(corpusOut); out != "" {
		if err := os.WriteFile(out, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(filepath.Join("testdata", "corpus.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("the audit of the corpus differs from testdata/corpus.expected.txt:\n%s", got)
	}
	blocks, refused := 0, 0
	for _, line := range strings.Split(got, "\n") {
		switch {
		case strings.HasSuffix(line, ": refused"):
			refused++
		case line != "" && !strings.HasPrefix(line, " ") && strings.Contains(line, ", stale 7: "):
			blocks++
		}
	}
	if blocks != 24 || refused != 18 {
		t.Errorf("the audit reports on %d entries and has no report for %d; the design states 24 and 18", blocks, refused)
	}
}
