package conformance

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablo/internal/corpustest"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) {
	code := corpustest.Main(m)
	if binDir != "" {
		_ = os.RemoveAll(binDir)
	}
	os.Exit(code)
}

// A knownItem is one item of known.yaml.
type knownItem struct {
	Entry   string `yaml:"entry"`
	Reason  string `yaml:"reason"`
	Owes    string `yaml:"owes"`
	Finding string `yaml:"finding"`
}

// loadKnown reads known.yaml.
func loadKnown(data []byte) ([]knownItem, error) {
	var file struct {
		Known []knownItem `yaml:"known"`
	}
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	for _, k := range file.Known {
		if k.Entry == "" || k.Reason == "" || k.Owes == "" {
			return nil, fmt.Errorf("known.yaml: an item of %q lacks entry, reason or owes", k.Entry)
		}
	}
	return file.Known, nil
}

// A ruling is what a subtest does with the outcome of an entry.
type ruling int

const (
	vPass  ruling = iota // the entry passes and is not listed
	vFail                // the entry fails and is not listed
	vKnown               // the entry fails and is listed: log, then skip
	vStale               // the entry passes and is listed: fail, so the item goes
)

// judge applies known.yaml to one outcome.
func judge(listed, mismatched bool) ruling {
	switch {
	case !listed && !mismatched:
		return vPass
	case !listed:
		return vFail
	case mismatched:
		return vKnown
	}
	return vStale
}

// absentFromCorpus lists the items that name no entry of the corpus.
func absentFromCorpus(items []knownItem, names []string) []string {
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
	}
	var out []string
	for _, k := range items {
		if !have[k.Entry] {
			out = append(out, k.Entry)
		}
	}
	return out
}

// A collector gathers the results of the subtests.
type collector struct {
	mu      sync.Mutex
	results []entryResult
}

func (c *collector) add(r entryResult) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.results = append(c.results, r)
}

// TestCorpus is T20 and T21: the command run over every entry of the corpus.
func TestCorpus(t *testing.T) {
	c := corpustest.Open(t)
	bin := binary(t)
	env := append(c.Env(), "GIT_AUTHOR_EMAIL="+observerEmail, "GIT_COMMITTER_EMAIL="+observerEmail)
	r := &runner{bin: bin, env: env}
	language, err := r.language()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("known.yaml")
	if err != nil {
		t.Fatal(err)
	}
	items, err := loadKnown(data)
	if err != nil {
		t.Fatal(err)
	}
	listed := map[string]knownItem{}
	for _, k := range items {
		listed[k.Entry] = k
	}
	rules := c.Rules(t)
	ruleIDs := map[string]bool{}
	for _, rule := range rules {
		ruleIDs[rule.ID] = true
	}
	keep := os.Getenv("TABLO_KEEP") != ""

	col := &collector{}
	t.Cleanup(func() {
		for _, name := range absentFromCorpus(items, c.Names()) {
			t.Errorf("known.yaml lists %s, which the corpus does not hold; remove the item", name)
		}
		_, commit, _, _ := corpustest.Lock()
		origin := "testdata/corpus at tableaux " + commit[:min(7, len(commit))]
		if !c.Vendored {
			origin = c.Origin
		}
		rep := report(col.results, rules, tabloVersion(r), language, origin)
		t.Logf("conformance report\n%s", summary(rep))
		if path := os.Getenv("TABLO_REPORT"); path != "" {
			if err := os.WriteFile(path, []byte(rep), 0o644); err != nil {
				t.Errorf("TABLO_REPORT: %v", err)
			}
		}
	})

	for _, name := range c.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			res := entryResult{name: name, status: skipped, reason: "read in place, and the copy has no tableaux checkout beside it; set TABLO_CORPUS"}
			defer func() {
				if res.reason != "" || res.status != skipped || t.Skipped() {
					col.add(res)
				}
			}()
			e := c.Entry(t, name)
			for _, f := range statedFindings(mapKey(e.Expected, "findings")) {
				res.stated = append(res.stated, f.rule)
			}
			if major(e.Language) != major(language) {
				res.reason = fmt.Sprintf("language %s: tablo reads major %s", e.Language, major(language))
				t.Skip(res.reason)
			}
			obs := newObserver(r, e)
			doc := obs.observe(e.Expected)
			if len(obs.errs) > 0 {
				res.status, res.reason = failed, obs.errs[0]
				t.Error(strings.Join(obs.errs, "\n"))
				return
			}
			out, err := compare(e, e.Expected, doc, obs.callFor)
			if err != nil {
				res.status, res.reason = failed, err.Error()
				t.Fatal(err)
			}
			res.facts, res.unexpected = out.facts, out.unexpected
			for _, d := range asList(doc["findings"]) {
				if !ruleIDs[text(asMap(d)["rule"])] {
					res.outside++
				}
			}
			_, isListed := listed[name]
			switch judge(isListed, out.mismatches() > 0) {
			case vPass:
				res.status, res.reason = passed, ""
			case vFail:
				res.status, res.reason = failed, firstMismatch(out)
				t.Error("\n" + failureText(name, e.Ref, out, keep, obs.lines))
			case vKnown:
				res.status, res.reason = known, firstMismatch(out)
				item := listed[name]
				t.Logf("\n%s", failureText(name, e.Ref, out, keep, obs.lines))
				t.Skipf("known, owes %s: %s", item.Owes, item.Reason)
			case vStale:
				res.status, res.reason = failed, "listed in known.yaml and passes"
				t.Errorf("%s passes, and known.yaml lists it: remove the item", name)
			}
		})
	}
}

// tabloVersion reads the version of the command, or "?".
func tabloVersion(r *runner) string {
	rep, err := r.run("version", "--json")
	if err != nil {
		return "?"
	}
	return text(asMap(rep.env["data"])["version"])
}

// TestRules is T19: the corpus is consistent with RULES.md. It needs no command.
func TestRules(t *testing.T) {
	c := corpustest.Open(t)
	rules := c.Rules(t)
	byID := map[string]corpustest.Rule{}
	for _, r := range rules {
		byID[r.ID] = r
	}
	names := map[string]bool{}
	for _, n := range c.Names() {
		names[n] = true
	}
	stated := map[string]bool{}
	for _, n := range c.Names() {
		data, err := os.ReadFile(filepath.Join(c.Work, "entries", n, "expected.yaml"))
		if err != nil {
			t.Fatal(err)
		}
		var doc yaml.Node
		if err := yaml.Unmarshal(data, &doc); err != nil || len(doc.Content) != 1 {
			t.Fatalf("corpus: %s: expected.yaml: %v", n, err)
		}
		for _, f := range statedFindings(mapKey(doc.Content[0], "findings")) {
			r, ok := byID[f.rule]
			switch {
			case !ok:
				t.Errorf("%s:%d states the rule %s, which RULES.md lacks", n, f.line, f.rule)
			case r.Severity != f.severity:
				t.Errorf("%s:%d states %s as %s, and RULES.md says %s", n, f.line, f.rule, f.severity, r.Severity)
			}
			stated[f.rule] = true
		}
	}
	var unstated []string
	for _, r := range rules {
		if !stated[r.ID] {
			unstated = append(unstated, r.ID)
		}
		for _, e := range r.Entries {
			if !names[e] {
				t.Errorf("RULES.md names the entry %s for %s, which the corpus lacks", e, r.ID)
			}
		}
	}
	sort.Strings(unstated)
	if len(unstated) > 0 {
		t.Errorf("no expected.yaml states a finding of %s", strings.Join(unstated, ", "))
	}
}
