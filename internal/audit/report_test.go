package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/view"
)

// shape writes rows as the tasks of each, one row per bracket.
func shape(rows []Row) string {
	var out []string
	for _, row := range rows {
		var ids []string
		for _, x := range row {
			ids = append(ids, x.Task)
		}
		out = append(out, "["+strings.Join(ids, " ")+"]")
	}
	return strings.Join(out, " ")
}

// T10: rows. Findings that agree in key, listed severity, gate, message,
// act and resolver join, each of another task; a finding with no task never
// joins; and the counts are of findings, not of rows.
func TestRows(t *testing.T) {
	r9 := func(task string) Finding {
		return Finding{Key: "R9", Rule: "R9", Severity: model.Warning, Task: task, Act: Await, Resolver: "ada@example.org",
			Message: "the requirement on 9f31 from design is due at implementation and unmet: 9f31 stands at function"}
	}
	// The twenty tasks that require one task at one gate make one row.
	twenty := &Report{}
	for i := 0; i < 20; i++ {
		twenty.Findings = append(twenty.Findings, r9(fmt.Sprintf("a%03d", i)))
	}
	rows := twenty.Rows(model.Warning)
	if len(rows) != 1 || len(rows[0]) != 20 || twenty.Counts() != (Counts{Warnings: 20}) {
		t.Fatalf("twenty findings of one message make %d rows and count %+v", len(rows), twenty.Counts())
	}
	for i, x := range rows[0] {
		if x != &twenty.Findings[i] {
			t.Errorf("the row holds another finding at %d", i)
		}
	}
	if most := twenty.Most(); most == nil || *most != (Resolving{"ada@example.org", 20}) {
		t.Errorf("the most is %+v", most)
	}

	// Two findings of one task make two rows; a third task joins the first.
	twice := &Report{Findings: []Finding{r9("a001"), r9("a001"), r9("a002"), r9("a002"), r9("a003")}}
	if got := shape(twice.Rows(model.Warning)); got != "[a001 a002 a003] [a001 a002]" {
		t.Errorf("two findings of one task make the rows %s", got)
	}

	// A finding with no task never joins: two commits that carry one unread
	// trailer make two rows.
	h1 := Finding{Key: "H1", Rule: "H1", Severity: model.Warning, Act: None, Resolver: "olive@example.org", Message: "zzzz names no task"}
	alone := &Report{Findings: []Finding{h1, h1, r9("a001"), h1}}
	if got := shape(alone.Rows(model.Warning)); got != "[] [] [a001] []" {
		t.Errorf("findings with no task make the rows %s", got)
	}
	if most := alone.Most(); most == nil || *most != (Resolving{"ada@example.org", 1}) {
		t.Errorf("the most counts a finding nothing resolves: %+v", most)
	}

	// Each key of the join parts two findings.
	base := r9("a001")
	differ := []func(*Finding){
		func(x *Finding) { x.Key = "R13" },
		func(x *Finding) { x.Severity, x.Demoted = model.Information, true }, // a demoted finding does not join one of the rule's own severity
		func(x *Finding) { x.Gate = "design" },
		func(x *Finding) { x.Message = "another" },
		func(x *Finding) { x.Act = Revise },
		func(x *Finding) { x.Resolver = "ben@example.org" },
	}
	for i, change := range differ {
		other := r9("a002")
		change(&other)
		two := &Report{Findings: []Finding{base, other}}
		if rows := two.Rows(model.Information); len(rows) != 2 {
			t.Errorf("the change %d joins the two findings: %s", i, shape(rows))
		}
	}
	// What the join does not read parts none: the file, the facts, the commits.
	other := r9("a002")
	other.File, other.Sentence, other.Kind = ".tableaux/tasks/a002.yaml", "another", "another"
	if rows := (&Report{Findings: []Finding{base, other}}).Rows(model.Warning); len(rows) != 1 {
		t.Errorf("two findings that agree in the six keys make %s", shape(rows))
	}

	// The rows stand in the order of the first finding of each, and the
	// minimum hides rows and no count.
	mixed := &Report{Findings: []Finding{
		{Key: "S11", Severity: model.Error, Task: "a001", Gate: "design", Act: Review, Resolver: "ben@example.org"},
		r9("a001"), h1, r9("a002"),
		{Key: SoleReview, Severity: model.Information, Task: "a001", Gate: "design", Act: None, Resolver: "ada@example.org"},
		{Key: SoleReview, Severity: model.Information, Task: "a002", Gate: "design", Act: None, Resolver: "ada@example.org"},
	}}
	for minimum, want := range map[model.Severity]string{
		model.Error:       "[a001]",
		model.Warning:     "[a001] [a001 a002] []",
		model.Information: "[a001] [a001 a002] [] [a001 a002]",
	} {
		if got := shape(mixed.Rows(minimum)); got != want {
			t.Errorf("at the minimum %s the rows are %s, want %s", minimum, got, want)
		}
	}
	if mixed.Counts() != (Counts{Errors: 1, Warnings: 3, Information: 2}) {
		t.Errorf("the counts are %+v", mixed.Counts())
	}
	if most := mixed.Most(); most == nil || *most != (Resolving{"ada@example.org", 2}) {
		t.Errorf("the most is %+v; the sole reviews ask nothing", most)
	}
	// Among equals the first in the findings' order.
	equals := &Report{Findings: []Finding{mixed.Findings[0], r9("a001")}}
	if most := equals.Most(); most == nil || most.Email != "ben@example.org" || most.Count != 1 {
		t.Errorf("among equals the most is %+v", most)
	}
	empty := &Report{}
	if empty.Most() != nil || len(empty.Rows(model.Information)) != 0 || empty.Counts() != (Counts{}) {
		t.Error("an empty report counts something")
	}
	if most := (&Report{Findings: []Finding{h1, {Key: "J8", Severity: model.Error, Act: Checkout}}}).Most(); most != nil {
		t.Errorf("a report that names nobody has the most %+v", most)
	}
}

// T11: the minimum, with whole counts (decision 1), on weather-station at
// main as the envelope's diagnostics give it.
func TestMinimum(t *testing.T) {
	var e *entry
	for _, each := range entries(t) {
		if each.name == "weather-station" {
			e = each
		}
	}
	if e == nil {
		t.Fatal("the corpus builds no weather-station")
	}
	f := derived(t, load.Options{Replace: e.replace}, e.dir, "main", "")
	rep := audited(t, Input{Facts: f, Diagnostics: diagnostics(f, e.want, e.labels), Now: "2026-09-28"})
	for minimum, want := range map[model.Severity]int{model.Error: 0, model.Warning: 2, model.Information: 21} {
		if rows := rep.Rows(minimum); len(rows) != want {
			t.Errorf("at the minimum %s the audit lists %d rows, want %d", minimum, len(rows), want)
		}
		if rep.Counts() != (Counts{Errors: 0, Warnings: 2, Information: 32}) {
			t.Errorf("beside the minimum %s the counts are %+v", minimum, rep.Counts())
		}
	}
	rows := rep.Rows(model.Warning)
	if rows[0][0].Key != "R9" || rows[1][0].Key != Proposed || rows[0][0].Message != rows[0][0].Diagnostic.Message {
		t.Errorf("at the default minimum the rows are %s and %s", rows[0][0].Key, rows[1][0].Key)
	}
	if most := rep.Most(); most == nil || *most != (Resolving{"ada@example.org", 2}) {
		t.Errorf("the most is %+v, want ada@example.org with two", most)
	}

	// Keep selects by resolver and by task; the date, the age and the kinds examined stay.
	ben := rep.Keep(func(x *Finding) bool { return x.Resolver == "ben@example.org" })
	if ben.Counts() != (Counts{Information: 12}) || ben.Most() != nil || ben.Now != rep.Now || ben.Stale != rep.Stale {
		t.Errorf("ben's findings count %+v, most %+v", ben.Counts(), ben.Most())
	}
	for _, x := range ben.Findings {
		if x.Resolver != "ben@example.org" || x.Key != SoleReview {
			t.Errorf("ben's findings hold %+v", x)
		}
	}
	firmware := rep.Keep(func(x *Finding) bool { return x.Task == "c07d" })
	if firmware.Counts() != (Counts{Warnings: 1, Information: 4}) || shape(firmware.Rows(model.Warning)) != "[c07d]" {
		t.Errorf("the findings of c07d count %+v", firmware.Counts())
	}
	if most := firmware.Most(); most == nil || *most != (Resolving{"ada@example.org", 1}) {
		t.Errorf("the most of c07d is %+v", most)
	}
	if len(rep.Findings) != 34 || len(rep.Keep(func(*Finding) bool { return false }).Findings) != 0 {
		t.Errorf("the report holds %d findings", len(rep.Findings))
	}

	// Silent: the kinds examined that no finding has, hidden or not.
	var silent []string
	for _, quiet := range rep.Silent() {
		silent = append(silent, quiet.Key)
		if (quiet.Rule == "") != (quiet.Key == Stale) || quiet.Kind == "" {
			t.Errorf("the silence reads %+v", quiet)
		}
	}
	if got := strings.Join(silent, " "); got != "P5 R12 R13 R14 J8 J9 J13 J14 J15 J16 J17 J19 H1 H2 H3 H4 H5 H6 H7 H8 stale" {
		t.Errorf("the silent kinds are %s; want stale among them and no sole_review", got)
	}
	// A kind the minimum hides is not silent, and one Keep drops is.
	quiet := map[string]bool{}
	for _, s := range ben.Silent() {
		quiet[s.Key] = true
	}
	if !quiet["R9"] || !quiet[Proposed] || !quiet[Stale] || quiet[SoleReview] {
		t.Errorf("ben's silent kinds are %v", quiet)
	}
	// With no trunk the run examines no proposed task.
	none := derived(t, load.Options{}, builtAt(t, "trunk-undetermined"), "main", "")
	for _, s := range audited(t, Input{Facts: none, Now: "2026-09-01"}).Silent() {
		if s.Key == Proposed {
			t.Error("with no trunk the proposed tasks are silent")
		}
	}
}

// T11 and T15: the golden data of the view agrees with the report it is
// written from, in every field the Audit gives: the counts, the most, the
// rows at the default minimum with their words and their commands, and the
// silent kinds. The typed facts and the commits are the view's to write
// (8ed1).
func TestGoldenDataAgrees(t *testing.T) {
	var e *entry
	for _, each := range entries(t) {
		if each.name == "weather-station" {
			e = each
		}
	}
	if e == nil {
		t.Fatal("the corpus builds no weather-station")
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "view", "testdata", "audit.provenance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var want view.Audit
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	f := derived(t, load.Options{Replace: e.replace}, e.dir, "main", "")
	rep := audited(t, Input{Facts: f, Diagnostics: diagnostics(f, e.want, e.labels), Now: "2026-09-28"})
	c, most := rep.Counts(), rep.Most()
	if c.Errors != want.Errors || c.Warnings != want.Warnings || c.Information != want.Information ||
		most == nil || want.Most == nil || most.Email != want.Most.Email || most.Count != want.Most.Count {
		t.Errorf("the report counts %+v, most %+v; the golden data states %d, %d and %d, most %+v", c, most, want.Errors, want.Warnings, want.Information, want.Most)
	}
	rows := rep.Rows(model.Warning)
	if len(rows) != len(want.Findings) {
		t.Fatalf("the report lists %d rows, the golden data %d", len(rows), len(want.Findings))
	}
	for i, row := range rows {
		x, w := row[0], want.Findings[i]
		var tasks, files, commits, commands, wantTasks, wantCommits, wantCommands []string
		for _, y := range row {
			tasks, files = append(tasks, y.Task), append(files, y.File)
		}
		for _, c := range x.Commits {
			commits = append(commits, c.ID)
		}
		for _, c := range x.Commands {
			commands = append(commands, c.Text+" # "+c.Comment)
		}
		for _, task := range w.Tasks {
			wantTasks = append(wantTasks, task.ID)
		}
		for _, c := range w.Commits {
			wantCommits = append(wantCommits, c.Hash)
		}
		for _, c := range w.Commands {
			wantCommands = append(wantCommands, c.Text+" # "+c.Comment)
		}
		got := fmt.Sprintln(x.Key, x.Rule, x.Severity.String(), x.Demoted, x.Kind, tasks, x.Gate, string(x.Act), x.Resolver, files, x.Message, x.Sentence, x.Source.Text, x.Source.URL, commits, commands)
		golden := fmt.Sprintln(w.Key, w.Rule, w.Severity, w.Demoted, w.Kind, wantTasks, w.Gate, w.Act, w.Resolver, w.Files, w.Message, w.Sentence, w.Source.Text, w.Source.URL, wantCommits, wantCommands)
		if got != golden {
			t.Errorf("the row %d reads\n%s\nand the golden data\n%s", i, got, golden)
		}
		for j, y := range row {
			facts := w.Facts[j]
			has := fmt.Sprint(facts.Junction != nil, facts.Status != nil, facts.Requirement != nil, facts.Authorisation != nil, facts.Linkage != nil, facts.Model != nil)
			carries := fmt.Sprint(y.Facts.Junction != nil, y.Facts.Status != nil, y.Facts.Requirement != nil, y.Facts.Authorisation != nil, y.Facts.Snapshot != nil, y.Facts.Reading != nil)
			if facts.Task != y.Task || has != carries {
				t.Errorf("the row %d carries the facts %s of %s, the golden data %s of %s", i, carries, y.Task, has, facts.Task)
			}
		}
	}
	var silent, wantSilent []string
	for _, s := range rep.Silent() {
		silent = append(silent, s.Key+"|"+s.Rule+"|"+s.Kind)
	}
	for _, s := range want.Silent {
		wantSilent = append(wantSilent, s.Key+"|"+s.Rule+"|"+s.Kind)
	}
	if fmt.Sprint(silent) != fmt.Sprint(wantSilent) {
		t.Errorf("the silent kinds are\n%v\nand the golden data states\n%v", silent, wantSilent)
	}
}

// T16: a report is safe to share. Eight goroutines call its methods at
// once, and each reads what one alone reads.
func TestReportIsSafeToShare(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/e4a1.yaml":  "title: Root\nassignee: olive@example.org\n",
		"tasks/b2c9.yaml":  "title: First\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 1 }\n",
		"tasks/a100.yaml":  "title: Second\nassignee: pat@example.org\nparent: { id: \"e4a1\", order: 2 }\n",
		"status/b2c9.yaml": "gate: mockup\nstate: nominal\n",
	})
	list := []model.Diagnostic{
		{Code: "T12", Severity: model.Warning, Pos: model.Pos{File: "tasks/e4a1.yaml"}, Task: "e4a1", Message: "b2c9 and a100 share order 1 under e4a1"},
		{Code: "S6", Severity: model.Error, Pos: model.Pos{File: "status/b2c9.yaml", Line: 2, Col: 8}, Task: "b2c9", Message: "state nominal names no state"},
	}
	rep := audited(t, Input{Facts: f, Diagnostics: list, Now: "2026-09-01"})
	read := func() string {
		kept := rep.Keep(func(x *Finding) bool { return x.Task != "e4a1" })
		return fmt.Sprint(shape(rep.Rows(model.Information)), rep.Counts(), rep.Most(), rep.Silent(), len(kept.Findings), kept.Counts(), shape(kept.Rows(model.Warning)))
	}
	want := read()
	var wg sync.WaitGroup
	got := make([]string, 8)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 50; n++ {
				got[i] = read()
			}
			again, err := Run(Input{Facts: f, Diagnostics: list, Now: "2026-09-01"})
			if err != nil || !reflect.DeepEqual(again, rep) {
				t.Errorf("a run beside the readers gives another report: %v", err)
			}
		}()
	}
	wg.Wait()
	for i, each := range got {
		if each != want {
			t.Errorf("the goroutine %d reads %s, want %s", i, each, want)
		}
	}
}

// T16: the package is pure. Its source names neither the clock nor the
// operating system, and imports nothing that runs a process or reads a file.
func TestPackageIsPure(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	sources := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		sources++
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, word := range []string{`\btime\.Now\b`, `\bos\.`, `"os"`, `"os/exec"`, `"io"`, `"math/rand"`, `\bGetenv\b`, `\btime\.Local\b`, `internal/git"`, `internal/load"`} {
			if regexp.MustCompile(word).Match(data) {
				t.Errorf("%s names %s", file, word)
			}
		}
	}
	if sources != 4 {
		t.Errorf("the package holds %d source files; the design names audit.go, rules.go, facts.go and report.go", sources)
	}
}

// T16: the audit is deterministic. Two clones of the corpus at two paths,
// read by two processes under two time zones, give the bytes of
// testdata/corpus.expected.txt.
func TestTwoClones(t *testing.T) {
	if testing.Short() {
		t.Skip("the test copies the corpus twice")
	}
	build := corpus(t)
	want, err := os.ReadFile(filepath.Join("testdata", "corpus.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("the test binary has no path: %v", err)
	}
	for i, zone := range []string{"Pacific/Kiritimati", "America/Los_Angeles"} {
		// Two paths of two depths, so that no path of a clone equals another's.
		root := filepath.Join(t.TempDir(), strings.Repeat("deeper/", i)+"corpus")
		copyTree(t, build, filepath.Join(root, "build"))
		copyTree(t, filepath.Join(filepath.Dir(build), "entries"), filepath.Join(root, "entries"))
		out := filepath.Join(t.TempDir(), "audit.txt")
		cmd := exec.Command(exe, "-test.run=^TestCorpus$", "-test.count=1")
		cmd.Env = append(os.Environ(), "TZ="+zone, "TABLEAUX_CORPUS="+filepath.Join(root, "build"), corpusOut+"="+out)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("the run under %s: %v\n%s", zone, err, output)
		}
		got, err := os.ReadFile(out)
		if err != nil {
			t.Fatalf("the run under %s writes nothing: %v", zone, err)
		}
		if string(got) != string(want) {
			t.Errorf("the clone at %s under %s gives other bytes:\n%s", root, zone, got)
		}
	}
}
