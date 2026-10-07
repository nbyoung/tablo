package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// entryName matches the line that opens an entry's block in corpus.expected.txt.
var entryName = regexp.MustCompile(`^# ([a-z0-9-]+)$`)

// blocks reads testdata/corpus.expected.txt: the lines of each entry, by name.
func blocks(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "corpus.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	all := map[string][]string{}
	name := ""
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		switch m := entryName.FindStringSubmatch(line); {
		case m != nil:
			name = m[1]
			all[name] = []string{}
		case strings.HasPrefix(line, "#"):
		case name == "":
			t.Fatalf("corpus.expected.txt: a line before the first entry: %s", line)
		default:
			all[name] = append(all[name], line)
		}
	}
	return all
}

// TestFilesOnTheCorpus is T2: for every built entry Files reports what
// corpus.expected.txt states, in its order: file, line, column, severity,
// code, task and gate. It is T6's second part too: no message holds the
// schema library's words.
func TestFilesOnTheCorpus(t *testing.T) {
	want := blocks(t)
	names := built(t)
	if len(names) != len(want) {
		t.Errorf("the corpus builds %d entries and corpus.expected.txt states %d", len(names), len(want))
	}
	for _, name := range names {
		lines, stated := want[name]
		if !stated {
			t.Errorf("%s: corpus.expected.txt states no such entry", name)
			continue
		}
		found := Files(entry(t, name))
		got := places(found)
		same := len(got) == len(lines)
		for i := 0; same && i < len(got); i++ {
			same = strings.HasPrefix(lines[i], got[i]+": ")
		}
		if !same {
			t.Errorf("%s:\n got  %s\n want %s", name, strings.Join(got, "\n      "), strings.Join(lines, "\n      "))
		}
		for _, d := range found {
			if strings.Contains(d.Message, "additional properties") || strings.Contains(d.Message, "additionalProperties") {
				t.Errorf("%s: a message holds the schema library's words: %s", name, d.Message)
			}
		}
	}
}

// TestFilesAgreesWithTheCorpus is T3: for each built entry the findings its
// expected.yaml states in the load and files tiers match what Files reports,
// by the rule of the conformance run (4b4f, rule 8), and Files reports
// nothing more.
func TestFilesAgreesWithTheCorpus(t *testing.T) {
	for _, name := range built(t) {
		var want []finding
		for _, f := range expected(t, name).Findings {
			rule, ok := RuleOf(f.Rule)
			if !ok {
				t.Errorf("%s: expected.yaml states the rule %s, which Rules lacks", name, f.Rule)
			}
			if rule.Tier == TierLoad || rule.Tier == TierFiles {
				want = append(want, f)
			}
		}
		got := Files(entry(t, name))
		if missing, extra := match(want, got); len(missing)+len(extra) > 0 {
			t.Errorf("%s: no diagnostic for %+v; no finding for %v", name, missing, places(extra))
		}
	}
}

// match compares findings with diagnostics as a multiset. Each finding takes
// one diagnostic not yet taken that agrees on the rule, the severity and each
// of task, file, gate, commit and trailer it states; the findings that state
// most match first. It returns the findings that take none and the
// diagnostics left over.
func match(want []finding, got []model.Diagnostic) (missing []finding, extra []model.Diagnostic) {
	keys := func(f finding) int {
		n := 0
		for _, key := range []string{f.Task, f.File, f.Gate, f.Commit, f.Trailer} {
			if key != "" {
				n++
			}
		}
		return n
	}
	want = append([]finding{}, want...)
	sort.SliceStable(want, func(i, j int) bool { return keys(want[i]) > keys(want[j]) })
	taken := make([]bool, len(got))
	for _, f := range want {
		found := false
		for i, d := range got {
			agrees := !taken[i] && d.Code == f.Rule && d.Severity.String() == f.Severity &&
				(f.Task == "" || f.Task == d.Task) && (f.File == "" || f.File == d.Pos.File) &&
				(f.Gate == "" || f.Gate == d.Gate) && (f.Commit == "" || f.Commit == d.Commit) &&
				(f.Trailer == "" || f.Trailer == d.Trailer)
			if agrees {
				taken[i], found = true, true
				break
			}
		}
		if !found {
			missing = append(missing, f)
		}
	}
	for i, d := range got {
		if !taken[i] {
			extra = append(extra, d)
		}
	}
	return missing, extra
}

// TestOwners is T1, second part: over every built entry and the Loader's
// broken fixture, Files adds no diagnostic with a code the Loader owns, and
// every code is a rule of the files or the load tier with its severity.
func TestOwners(t *testing.T) {
	check := func(name string, p *model.Project) {
		loaders := map[model.Diagnostic]int{}
		for _, d := range p.Diagnostics {
			loaders[d]++
		}
		for _, d := range Files(p) {
			rule, ok := RuleOf(d.Code)
			switch {
			case !ok || rule.Severity != d.Severity:
				t.Errorf("%s: %s is no rule, or not at its severity", name, where(d))
			case rule.Tier == TierLoad && loaders[d] == 0:
				t.Errorf("%s: Files adds %s, which the Loader owns", name, where(d))
			case rule.Tier == TierLoad:
				loaders[d]--
			case rule.Tier != TierFiles:
				t.Errorf("%s: Files raises %s, a rule of Derived", name, where(d))
			}
		}
		for d, n := range loaders {
			if n > 0 {
				t.Errorf("%s: Files drops the Loader's %s", name, where(d))
			}
		}
	}
	check("broken", worktree(t, broken(t)))
	for _, name := range built(t) {
		check(name, entry(t, name))
	}
}

// broken copies the Loader's fixture testdata/broken into a new repository
// with no commit and returns its path.
func broken(t testing.TB) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "broken")
	copyTree(t, filepath.Join("..", "load", "testdata", "broken"), repo)
	gitRun(t, repo, "init", "-q", "-b", "main")
	return repo
}

// TestMessagesRepeat is T6: a gates file with three unknown fields gives, in
// twenty runs, equal diagnostics, one for each field in the order of the
// file, in the Validator's own words.
func TestMessagesRepeat(t *testing.T) {
	p := project(t, with(base(), "gates.yaml", baseGates+"zeta: 1\nalpha: 2\nmiddle: 3\n"))
	first := Files(p)
	var messages []string
	for _, d := range first {
		messages = append(messages, d.Message)
	}
	want := []string{"zeta is no field of the file", "alpha is no field of the file", "middle is no field of the file"}
	if codes(first) != "G11 G11 G11" || !reflect.DeepEqual(messages, want) {
		t.Fatalf("got %v %q", places(first), messages)
	}
	for i := 0; i < 20; i++ {
		if again := Files(p); !reflect.DeepEqual(again, first) {
			t.Fatalf("run %d differs: %v", i, places(again))
		}
	}
}

// TestUnquotedIDs is T7: the Loader's broken fixture gives six T7 warnings,
// the unquoted a000 among them, with the two forms of the message.
func TestUnquotedIDs(t *testing.T) {
	var got []string
	for _, d := range Files(worktree(t, broken(t))) {
		if d.Code == "T7" {
			got = append(got, where(d)+": "+d.Message)
		}
	}
	want := []string{
		`tasks/e444.yaml:7:15: warning: T7 task=e444: parent.id is written as the integer 1000, not the string "1000"`,
		`tasks/f555.yaml:6:11: warning: T7 task=f555: requires.0.id is written as the float 07e0, not the string "07e0"`,
		`tasks/f555.yaml:7:11: warning: T7 task=f555: requires.1.id is written as the float 40e8, not the string "40e8"`,
		`tasks/f555.yaml:8:35: warning: T7 task=f555: requires.2.subproject.id is written as the integer 1000, not the string "1000"`,
		`tasks/f555.yaml:10:42: warning: T7 task=f555: junctions.release.subproject.id is written as the integer 0010, not the string "0010"`,
		`tasks/f555.yaml:11:15: warning: T7 task=f555: parent.id is written without quotes: a000`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// TestPartlyRead is T8: one unreadable file gives one error. The broken
// fixture gains a child of b111, a requirement on it and a status of it: L1
// alone names b111, and no T8, T9, R1 or S1 follows. The two corpus entries a
// reading rule rejects give that rule alone.
func TestPartlyRead(t *testing.T) {
	repo := broken(t)
	write(t, repo, "tasks/abcd.yaml", "title: Child\ndescription: A child of what is unread.\nassignee: pat@example.org\nrequires:\n  - { id: \"b111\" }\nparent: { id: \"b111\", order: 1 }\n")
	write(t, repo, "status/b111.yaml", "gate: defined\nstate: nominal\n")
	found := Files(worktree(t, repo))
	for _, d := range found {
		about := d.Task == "b111" || d.Task == "abcd" || strings.Contains(d.Message, "b111")
		if d.Code == "T8" || (about && d.Code != "L1") {
			t.Errorf("an unread task gives %s: %s", where(d), d.Message)
		}
	}
	if !strings.Contains(strings.Join(places(found), "\n"), "tasks/b111.yaml:3: error: L1 task=b111") {
		t.Errorf("no L1 for b111 in %v", places(found))
	}
	for name, want := range map[string]string{"file-duplicate-key": "L2", "file-yaml-feature": "L3", "file-not-yaml": "L1"} {
		if got := codes(Files(entry(t, name))); got != want {
			t.Errorf("%s: %s; want %s alone", name, got, want)
		}
	}
}

// TestSilencedRules is T9: P4 ends the run, and a project with no gates file
// gets G1 and no rule that names a gate.
func TestSilencedRules(t *testing.T) {
	// A malformed version file silences nothing.
	if got := codes(Files(project(t, with(base(), "version.yaml", "tableaux: 0.3\n", "status/b2c9.yaml", "gate: nonesuch\n")))); got != "S5 P3" {
		t.Errorf("a malformed version: %s; want S5 P3", got)
	}
	// No project: P1 and nothing else.
	if got := places(Files(worktree(t, repository(t, map[string]string{"/README.md": "No plan.\n"})))); !reflect.DeepEqual(got, []string{"error: P1"}) {
		t.Errorf("no project: %v", got)
	}
	if Files(nil) != nil {
		t.Error("Files(nil) reports")
	}

	ahead := copyOf(t, "version-minor-ahead")
	write(t, ahead, "tasks/b2c9.yaml", strings.Replace(read(t, ahead, "tasks/b2c9.yaml"), "pat@example.org", "nobody", 1))
	write(t, ahead, "status/zzzz.yaml", "state: nominal\n")
	if got := codes(Files(worktree(t, ahead))); got != "P4" {
		t.Errorf("a version the module does not accept: %s; want P4 alone", got)
	}

	missing := copyOf(t, "gates-missing")
	write(t, missing, "tasks/b2c9.yaml", strings.Replace(read(t, missing, "tasks/b2c9.yaml"),
		"parent:", "requires:\n  - { id: \"e4a1\", to: nonesuch }\njunctions:\n  nonesuch: { reviewer: olive@example.org }\nparent:", 1))
	write(t, missing, "status/b2c9.yaml", "gate: nonesuch\nstate: nonesuch\n")
	if got := places(Files(worktree(t, missing))); !reflect.DeepEqual(got, []string{"gates.yaml: error: G1", "tasks/b2c9.yaml:8:11: error: R4 task=b2c9"}) {
		t.Errorf("no gates file: %v; want G1 and the one rule that names no gate", got)
	}

	// J2, J11 and the key pattern of J1 need no gates file.
	keys := copyOf(t, "gates-missing")
	write(t, keys, "tasks/b2c9.yaml", strings.Replace(read(t, keys, "tasks/b2c9.yaml"),
		"parent:", "junctions:\n  undefined: { applies: false }\n  Design: { reviewer: olive@example.org }\nparent:", 1))
	if got := codes(Files(worktree(t, keys))); got != "G1 J2 J1" {
		t.Errorf("no gates file and two bad keys: %s; want G1 J2 J1", got)
	}
}

// TestSubprojects is T11: the rules of a subproject field, on a junction with
// its gate and on a requirement with none.
func TestSubprojects(t *testing.T) {
	const url = "https://example.org/lib.git"
	const hash = "438d39de8534b02f59ee7a195f5e3afbff5c7fa2"
	sub := func(dir string, pairs ...string) []string {
		files := []string{
			"/" + dir + "/.tableaux/version.yaml", baseVersion,
			"/" + dir + "/.tableaux/gates.yaml", baseGates,
			"/" + dir + "/.tableaux/tasks/5a00.yaml", "title: Library\ndescription: The root.\nassignee: olive@example.org\njunctions: { design: { applies: false } }\n",
		}
		for i := 0; i+1 < len(pairs); i += 2 {
			files = append(files, "/"+dir+"/.tableaux/"+pairs[i], pairs[i+1])
		}
		return files
	}
	var files []string
	files = append(files, sub("lib")...)
	// A subproject with faults of its own, which stay in its own run.
	files = append(files, sub("faulty", "tasks/5a00.yaml", "title: Library\nassignee: nobody\n", "status/zzzz.yaml", "gate: nonesuch\n")...)
	files = append(files, sub("old", "version.yaml", "tableaux: 1.0.0\n")...)
	files = append(files, sub("forest", "tasks/5b00.yaml", "title: Second\ndescription: A second root.\nassignee: olive@example.org\n")...)
	files = append(files, "/empty/keep", "A directory with no plan.\n", "/README.md", "A file.\n")

	// A want names a rule and the field of the subproject it sits at: url,
	// id, commit, or subproject for the field itself.
	cases := []struct {
		name, field string
		junction    []string // what the field gives on the junction at design
		requirement []string // and on a requirement
	}{
		{"a directory", `{ url: lib, id: "5a00" }`, nil, nil},
		{"a directory's root", `{ url: lib }`, nil, []string{"R11 subproject"}},
		{"a subproject with faults of its own", `{ url: faulty, id: "5a00" }`, nil, nil},
		{"outside", `{ url: ../x, id: "5a00" }`, []string{"J8 url"}, []string{"J8 url"}},
		{"absolute", `{ url: /abs, id: "5a00" }`, []string{"J8 url"}, []string{"J8 url"}},
		{"missing", `{ url: nowhere, id: "5a00" }`, []string{"J8 url"}, []string{"J8 url"}},
		{"a file", `{ url: README.md, id: "5a00" }`, []string{"J8 url"}, []string{"J8 url"}},
		{"no project", `{ url: empty, id: "5a00" }`, []string{"J8 url"}, []string{"J8 url"}},
		{"a version the module does not accept", `{ url: old, id: "5a00" }`, []string{"J8 url"}, []string{"J8 url"}},
		{"a URL with no clone", `{ url: "` + url + `", id: "5a00", commit: ` + hash + ` }`, []string{"J8 url"}, []string{"J8 url"}},
		{"a URL with no commit", `{ url: "` + url + `", id: "5a00" }`, []string{"J15 subproject"}, []string{"J15 subproject"}},
		{"a URL with a short commit", `{ url: "` + url + `", id: "5a00", commit: abc123 }`, []string{"J14 commit"}, []string{"J14 commit"}},
		{"a URL with a commit in upper case", `{ url: "` + url + `", id: "5a00", commit: ` + strings.ToUpper(hash) + ` }`, []string{"J14 commit"}, []string{"J14 commit"}},
		{"a directory with a commit", `{ url: lib, id: "5a00", commit: ` + hash + ` }`, []string{"J16 commit"}, []string{"J16 commit"}},
		{"a directory with a commit and no such task", `{ url: lib, id: "5c99", commit: ` + hash + ` }`, []string{"J9 id", "J16 commit"}, []string{"J9 id", "J16 commit"}},
		{"no such task", `{ url: lib, id: "5c99" }`, []string{"J9 id"}, []string{"J9 id"}},
		{"no one root", `{ url: forest }`, []string{"J9 subproject"}, []string{"R11 subproject"}},
		{"one root of two, by id", `{ url: forest, id: "5b00" }`, nil, nil},
	}
	// at returns where a want sits in the one line that holds the field.
	at := func(line, want, mark string) string {
		code, field, _ := strings.Cut(want, " ")
		col := strings.Index(line, field+": ") + len(field) + 3
		return fmt.Sprintf("tasks/b2c9.yaml:1:%d: error: %s%s", col, code, mark)
	}
	for _, c := range cases {
		for _, on := range []string{"junction", "requirement"} {
			line, wants, mark := "junctions: { design: { subproject: "+c.field+" } }\n", c.junction, " task=b2c9 gate=design"
			if on == "requirement" {
				line, wants, mark = "requires: [{ subproject: "+c.field+" }]\n", c.requirement, " task=b2c9"
			}
			p := project(t, with(base(), append([]string{"tasks/b2c9.yaml", line + baseLeaf}, files...)...))
			want := []string{}
			for _, w := range wants {
				want = append(want, at(line, w, mark))
			}
			if got := places(Files(p)); !reflect.DeepEqual(got, want) {
				t.Errorf("%s on a %s:\n got  %v\n want %v", c.name, on, got, want)
			}
		}
	}

	// R6 across projects reads the gates of the originating project and what
	// applies to the task there.
	across := func(from string) []string {
		line := "requires: [{ subproject: { url: lib, id: \"5a00\" }, from: " + from + " }]\n"
		got := places(Files(project(t, with(base(), append([]string{"tasks/b2c9.yaml", line + baseLeaf}, files...)...))))
		for i := range got {
			got[i] = strings.TrimPrefix(got[i], at(line, "R6 from", " task=b2c9"))
		}
		return got
	}
	if got := across("release"); len(got) != 0 {
		t.Errorf("from release across projects: %v", got)
	}
	if got := across("design"); !reflect.DeepEqual(got, []string{" gate=design"}) {
		t.Errorf("from a gate that does not apply there: %v", got)
	}
	if got := across("nonesuch"); !reflect.DeepEqual(got, []string{" gate=nonesuch"}) {
		t.Errorf("from no gate there: %v", got)
	}
}

// TestSubmoduleCommit is T11 for J17 and R12: a commit off the submodule's
// pin on a requirement, and a requirement on the task a junction reads
// through the same link.
func TestSubmoduleCommit(t *testing.T) {
	repo := copyOf(t, "subproject-commit-off-pin")
	task := read(t, repo, "tasks/b2c9.yaml")
	junction := task[strings.Index(task, "  design: "):strings.Index(task, "parent:")]
	field := strings.TrimSuffix(strings.TrimPrefix(junction, "  design: { subproject: { "), " } }\n")
	if got := places(Files(worktree(t, repo))); !reflect.DeepEqual(got, []string{"tasks/b2c9.yaml:8:45: error: J17 task=b2c9 gate=design"}) {
		t.Fatalf("the entry's working tree: %v", got)
	}
	write(t, repo, "tasks/b2c9.yaml", strings.Replace(task, "junctions:\n"+junction,
		"requires:\n  - { subproject: { "+field+", id: \"5a00\" } }\njunctions:\n"+junction, 1))
	want := []string{
		"tasks/b2c9.yaml:8:19: warning: R12 task=b2c9",
		"tasks/b2c9.yaml:8:39: error: J17 task=b2c9",
		"tasks/b2c9.yaml:10:45: error: J17 task=b2c9 gate=design",
	}
	if got := places(Files(worktree(t, repo))); !reflect.DeepEqual(got, want) {
		t.Errorf("a requirement and a junction on one submodule:\n got  %v\n want %v", got, want)
	}
}

// TestCascades is T12: each rule that ends a sequence silences the rules
// after it, so one fault gives one error.
func TestCascades(t *testing.T) {
	leaf := func(lines string) []string { return []string{"tasks/b2c9.yaml", lines + baseLeaf} }
	status := func(content string) []string { return []string{"status/b2c9.yaml", content} }
	exempt := "junctions: { release: { applies: false } }\n"
	cases := []struct {
		name  string
		files []string
		want  string
	}{
		{"J1: an entry at an unknown gate with a bad email", leaf("junctions: { nonesuch: { reviewer: nobody, colour: red } }\n"), "J1"},
		{"J1: a key that is no key", leaf("junctions: { Design: { applies: true } }\n"), "J1"},
		{"J2: a not-applicable entry at undefined", leaf("junctions: { undefined: { applies: false } }\n"), "J2"},
		{"J11: any other entry at undefined", leaf("junctions: { undefined: { applies: true, reviewer: nobody } }\n"), "J11"},
		{"J4: a mixed entry with a bad email", leaf("junctions: { design: { applies: false, reviewer: nobody } }\n"), "J4"},
		{"J4: an entry that is no mapping", leaf("junctions: { design: plain }\n"), "J4"},
		{"J4: an unknown field beside a subproject that is missing", leaf("junctions: { design: { subproject: { url: nowhere }, colour: red } }\n"), "J4"},
		{"J7: a subproject with no url", leaf("junctions: { design: { subproject: { id: \"5a00\" } } }\n"), "J7"},
		{"R1: a requirement on no task, from no gate", leaf("requires: [{ id: \"0c99\", from: nonesuch, to: undefined }]\n"), "R1"},
		{"R3: a requirement on itself, from no gate", leaf("requires: [{ id: \"b2c9\", from: nonesuch, to: undefined }]\n"), "R3"},
		{"R4: a requirement on an ancestor, from no gate", leaf("requires: [{ id: \"e4a1\", from: nonesuch, to: undefined }]\n"), "R4"},
		{"R5: a requirement on a descendant", []string{"tasks/e4a1.yaml", "requires: [{ id: \"b2c9\", from: nonesuch }]\n" + baseRoot}, "R5"},
		{"R11: both id and subproject", leaf("requires: [{ id: \"0c99\", subproject: { url: nowhere, id: \"5a00\" }, to: undefined }]\n"), "R11"},
		{"R11: a subproject with no id", leaf("requires: [{ subproject: { url: nowhere }, to: undefined }]\n"), "R11"},
		{"R8: neither id nor subproject", leaf("requires: [{ to: undefined }]\n"), "R8"},
		{"J8 on a requirement ends it", leaf("requires: [{ subproject: { url: nowhere, id: \"5a00\" }, to: undefined }]\n"), "J8"},
		{"R10 and R6 on one entry", append(leaf("requires: [{ id: \"c3d7\", from: nonesuch, to: undefined }]\n"),
			"tasks/c3d7.yaml", "title: Other\ndescription: Another leaf.\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 2 }\n"), "R6 R10"},
		{"S1: a status for no task", []string{"status/0c99.yaml", "gate: nonesuch\nstate: nonesuch\ncolour: red\n"}, "S1"},
		{"S3: a status with no gate", status("state: nonesuch\n"), "S3"},
		{"S5: a gate that names no gate, and a state that names none", status("gate: nonesuch\nstate: nonesuch\n"), "S5"},
		{"S5: a gate that is no key", status("gate: Design\nstate: nonesuch\n"), "S5"},
		{"S7: a status at an exempt gate", append(leaf(exempt), "status/b2c9.yaml", "gate: release\nstate: nominal\n"), "S7"},
		{"S8: the last gate without complete", status("gate: release\nstate: nominal\n"), "S8"},
		{"S8: the last gate with no state", status("gate: release\n"), "S8"},
		{"S12: complete before the last gate", status("gate: design\nstate: complete\n"), "S12"},
		{"S10: no state before a plain junction", status("gate: design\n"), "S10"},
		{"S4: undefined on one side, and no S10", status("gate: undefined\n"), "S4"},
	}
	for _, c := range cases {
		if got := codes(Files(project(t, with(base(), c.files...)))); got != c.want {
			t.Errorf("%s: %s; want %s", c.name, got, c.want)
		}
	}
}
