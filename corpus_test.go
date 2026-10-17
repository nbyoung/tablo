package tablo

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/nbyoung/tablo/internal/audit"
	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/model"
)

// A corpusEntry is one entry of the corpus that the build makes a
// repository of: what a test needs to read it as the corpus's README.md
// says a tool does.
type corpusEntry struct {
	name    string
	dir     string            // the repository
	refs    []string          // the ref in view, then the others the entry states, by name
	trunk   string            // the branch the caller names, or ""
	replace map[string]string // absolute URL to clone
	names   map[string]string // commit to label, the entry's and its subprojects'
}

// str returns a scalar of expected.yaml as a string.
func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// corpusEntries returns the built entries of the corpus, by name, as the
// Audit's tests read them. Their expected.yaml files lie beside the build,
// under ../entries. It leaves out an entry that reads a plan in place, whose
// facts drift.
func corpusEntries(t testing.TB) []*corpusEntry {
	t.Helper()
	build := corpus(t)
	root := filepath.Join(filepath.Dir(build), "entries")
	dirs, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("the corpus's expectations are absent: %v", err)
	}
	var list []*corpusEntry
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(root, d.Name(), "expected.yaml"))
		if err != nil {
			continue
		}
		var want map[string]any
		if err := yaml.Unmarshal(data, &want); err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		e := &corpusEntry{name: d.Name(), dir: filepath.Join(build, d.Name()), refs: []string{"main"},
			replace: map[string]string{}, names: map[string]string{}}
		if _, inPlace := want["source"]; inPlace {
			continue
		}
		if _, err := os.Stat(e.dir); err != nil {
			continue
		}
		if ref := str(want["ref"]); ref != "" {
			e.refs[0] = ref
		}
		if m, ok := want["trunk"].(map[string]any); ok {
			e.trunk = str(m["caller"])
		}
		if m, ok := want["replace"].(map[string]any); ok {
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
				e.names[line[2]] = line[1]
			}
		}
		if m, ok := want["refs"].(map[string]any); ok {
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

// sourceDate returns the author date of the source commit, the date the
// corpus tests pass, or 2026-09-01 where the source has no commit.
func sourceDate(f *derive.Facts) string {
	if log := f.Log(); log != nil {
		if c := log.Commit(log.Source); c != nil {
			return c.Date()
		}
	}
	return "2026-09-01"
}

// samples names the entries the golden file shows though each of their
// findings resolves by a revision.
var samples = map[string]bool{"file-duplicate-key": true, "gates-bad-key": true, "task-bad-filename": true,
	"siblings-same-order": true, "status-on-parent": true, "status-no-task": true, "subproject-path-missing": true}

// factNames names the typed facts a finding holds, in the order of Facts.
func factNames(f audit.Facts) []string {
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

// auditOfTheCorpus writes what the Audit reports on the corpus through
// snapshot.go, in the form of the Audit's own corpus test: the facts and the
// diagnostics of a snapshot, the Validator's from the history among them,
// where that test gives the history rules as literals (dada decision 10).
// One block per entry and ref, at the minimum information, with Now the
// author date of the source commit; an entry prints when one of its findings
// has an act other than revise and is no sole review, or is P5, or when
// samples names it; a second block prints the stale statuses at three days,
// where there are any; an entry the Derivation refuses prints "refused". A
// commit prints as the label the build gives it.
func auditOfTheCorpus(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, e := range corpusEntries(t) {
		for _, ref := range e.refs {
			s := loadAt(t, e.dir, ref, Options{Trunk: e.trunk, Replace: e.replace})
			f := s.facts
			for _, stale := range []int{0, 3} {
				rep, err := audit.Run(audit.Input{Facts: f, Diagnostics: s.diagnostics, Now: sourceDate(f), Stale: stale})
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
					only := rep.Keep(func(x *audit.Finding) bool { return x.Key == audit.Stale })
					if len(only.Findings) == 0 {
						continue
					}
					rep = only
				}
				plain := true
				for _, x := range rep.Findings {
					plain = plain && (x.Act == audit.Revise || x.Key == audit.SoleReview) && x.Key != "P5"
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
					if x.Key == audit.SoleReview {
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

// historyRules are the rules the Validator raises from the history, whose
// diagnostics the Audit's own corpus test gives as literals with the
// messages of the corpus's expected.yaml.
var historyRules = map[string]bool{"P5": true, "S11": true, "J13": true, "R13": true,
	"H1": true, "H2": true, "H3": true, "H4": true, "H5": true, "H6": true}

// findingKey matches the first line of a row of auditOfTheCorpus and gives
// its key.
var findingKey = regexp.MustCompile(`^  (\S+) (error|warning|information) \[`)

// normalised sets aside the message of each row of a history rule, which
// the Validator words where dada's file holds expected.yaml's wording.
func normalised(text string) string {
	lines := strings.Split(text, "\n")
	history := false
	for i, line := range lines {
		if m := findingKey.FindStringSubmatch(line); m != nil {
			history = historyRules[m[1]]
		}
		if history && strings.HasPrefix(line, "    message: ") {
			lines[i] = "    message: (the Validator's)"
		}
	}
	return strings.Join(lines, "\n")
}

// TestAuditOfTheCorpus is decision 16 (b): the audit of each corpus entry
// through snapshot.go, with the diagnostics the Validator raises from the
// history, equals dada's corpus.expected.txt, which testdata/audit.corpus.txt
// copies as it stands. Two things differ, each of the literals that file
// was made from: a row of a history rule carries the Validator's message,
// not expected.yaml's; and the R13 of requires-commit-behind carries the
// requirement fact, since the Validator's diagnostic stands at the entry's
// position, where the literal named its file alone (dada words.md, "the
// entry the diagnostic stands at").
func TestAuditOfTheCorpus(t *testing.T) {
	got := auditOfTheCorpus(t)
	data, err := os.ReadFile(filepath.Join("testdata", "audit.corpus.txt"))
	if err != nil {
		t.Fatal(err)
	}
	r13 := "  R13 warning [b2c9] at \"\" in [.tableaux/tasks/b2c9.yaml]: move_pin -> pat@example.org\n" +
		"    message: The requirement on 5a00 at https://example.org/lib.git is unmet at L1 and met at main in that repository, L2\n" +
		"    commits: \n"
	want := string(data)
	if strings.Count(want, r13) != 1 {
		t.Fatalf("testdata/audit.corpus.txt holds the R13 of requires-commit-behind %d times", strings.Count(want, r13))
	}
	want = strings.Replace(want, r13, r13+"    facts b2c9: requirement\n", 1)
	if normalised(got) != normalised(want) {
		t.Errorf("the audit of the corpus differs from testdata/audit.corpus.txt; it is in %s", writeOut(t, "audit.corpus.txt", []byte(got)))
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
		t.Errorf("the audit reports on %d entries and has no report for %d; dada's design states 24 and 18", blocks, refused)
	}
}
