package views

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/view"
)

// named returns the label of each line's commit, its task and, for a line
// taken from a subproject, @ and the subproject task: "W1 a1c0", "F1
// c07d@f1a0". A commit no label names reads as its first seven characters.
func named(h *view.History, hashes map[string]string) []string {
	byHash := map[string]string{}
	for label, hash := range hashes {
		byHash[hash] = label
	}
	var out []string
	for _, l := range h.Lines {
		e := l.Events[0]
		label, ok := byHash[e.Commit]
		if !ok {
			label = e.Commit[:7]
		}
		text := label + " " + l.Task.ID
		if e.Subproject != nil {
			text += "@" + e.Subproject.Task
		}
		out = append(out, text)
	}
	return out
}

// kindsOf returns the kinds of a line's events.
func eventsOf(l view.Line) []string {
	var kinds []string
	for _, e := range l.Events {
		kinds = append(kinds, e.Kind)
	}
	return kinds
}

// counted returns the counts of a history as one text: the events, the
// commits, the six kinds and the days.
func counted(h *view.History) string {
	var kinds []int
	for _, k := range h.Kinds {
		kinds = append(kinds, k.Count)
	}
	return fmt.Sprint(h.Events, h.Commits, kinds, len(h.Days))
}

// lineAt returns the line of a commit and a task of the project, or fails.
func lineAt(t *testing.T, h *view.History, commit, task string) view.Line {
	t.Helper()
	for _, l := range h.Lines {
		if l.Events[0].Commit == commit && l.Task.ID == task && l.Events[0].Subproject == nil {
			return l
		}
	}
	t.Fatalf("the history holds no line of %s at %.7s", task, commit)
	return view.Line{}
}

// entryLabels returns the labels of a corpus entry and of its subprojects.
func entryLabels(t testing.TB, name string) map[string]string {
	t.Helper()
	for _, e := range expectations(t) {
		if e.name == name {
			return e.labels
		}
	}
	return labels(t, name)
}

// T1: an empty history. A project with no history has no line, no day and
// six kinds that count nothing, and no command at provenance.
func TestEmptyHistory(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/e4a1.yaml": leafOf("olive@example.org", "", 0, ""),
		"tasks/b2c9.yaml": leafOf("pat@example.org", "e4a1", 1, ""),
	})
	h := History(f, Query{Level: Provenance})
	data := string(encoded(t, h))
	for _, want := range []string{`"lines": []`, `"days": []`, `"commands": []`, `"events": 0`, `"commits": 0`} {
		if !strings.Contains(data, want) {
			t.Errorf("the empty history holds no %s:\n%s", want, data)
		}
	}
	if len(h.Kinds) != 6 || h.Subject.ID != "e4a1" {
		t.Errorf("the empty history names %+v and counts %+v", h.Subject, h.Kinds)
	}
	for i, k := range h.Kinds {
		if k.Count != 0 || k.Kind != string(eventKinds[i]) {
			t.Errorf("kind %d is %+v", i, k)
		}
	}
}

// T4: the corpus's events. For every entry whose expected.yaml states the
// events of a task, the history of that task at provenance, flattened as
// 4b4f A9 reads it, equals the list by rule 7 of 4b4f.
func TestCorpusEventsFromLines(t *testing.T) {
	keys := []string{"date", "commit", "by", "task", "event", "gate", "state", "reason", "note", "url", "old", "new", "subproject"}
	lists, entries := 0, 0
	for _, e := range expectations(t) {
		tasks, _ := e.want["tasks"].(map[string]any)
		stated := false
		for _, id := range sortedKeys(tasks) {
			want, _ := tasks[id].(map[string]any)
			list, ok := want["events"].([]any)
			if !ok {
				continue
			}
			stated = true
			lists++
			h := History(e.at(t, e.ref), Query{Task: id, Level: Provenance})
			var got, wanted []string
			for _, l := range h.Lines {
				if l.Task.ID != id {
					continue
				}
				for _, ev := range l.Events {
					observed := map[string]string{"date": ev.Date, "commit": ev.Commit, "by": ev.By, "task": l.Task.ID, "event": ev.Kind, "gate": ev.Gate}
					if r := ev.Recorded; r != nil {
						observed["gate"], observed["state"], observed["reason"], observed["note"] = r.Gate, r.State, r.Reason, r.Note
					}
					if p := ev.Pin; p != nil {
						observed["url"], observed["old"], observed["new"] = p.URL, p.Old, p.New
					}
					if s := ev.Subproject; s != nil {
						observed["subproject"] = s.Task
					}
					var fields []string
					for _, key := range keys {
						fields = append(fields, observed[key])
					}
					got = append(got, fmt.Sprint(fields))
				}
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
				wanted = append(wanted, fmt.Sprint(fields))
			}
			if !reflect.DeepEqual(got, wanted) {
				t.Errorf("%s: %s: the events of the lines are\n %v\nand the corpus states\n %v", e.name, id, got, wanted)
			}
		}
		if stated {
			entries++
		}
	}
	// The design counts 18 in 13; the corpus at 0.4.0 adds twelve lists in nine entries (cdb0).
	if lists != 30 || entries != 22 {
		t.Errorf("the corpus states %d lists of events in %d entries; want 30 in 22", lists, entries)
	}
}

// T5: lines and counts. The weather station at glance: 15 lines of 17
// events in 13 commits over 12 days; W1 gives three lines, W11 one line of
// three events.
func TestLinesAndCounts(t *testing.T) {
	f, hashes := station(t, "main")
	h := History(f, Query{Level: Glance})
	if len(h.Lines) != 15 || counted(h) != "17 13 [7 2 4 1 2 1] 12" {
		t.Errorf("the history holds %d lines and counts %s", len(h.Lines), counted(h))
	}
	if h.Subject.ID != "a1c0" || h.HistoryProvenance != nil {
		t.Errorf("the history names %+v", h.Subject)
	}
	for _, l := range h.Lines {
		if l.Events[0].Subproject != nil || l.LineDetail != nil || l.LineProvenance != nil {
			t.Errorf("a line at glance is %+v", l)
		}
	}
	first := 0
	for _, l := range h.Lines {
		if l.Events[0].Commit == hashes["W1"] {
			first++
		}
	}
	if first != 3 {
		t.Errorf("W1 gives %d lines", first)
	}
	w11 := lineAt(t, h, hashes["W11"], "c07d")
	if kinds := eventsOf(w11); !reflect.DeepEqual(kinds, []string{"status", "reviewed", "pin"}) {
		t.Errorf("W11 holds %v", kinds)
	}
	if pin := w11.Events[2].Pin; pin == nil || pin.URL != "firmware" || pin.Old != "" || pin.New == "" {
		t.Errorf("the pin of W11 is %+v", pin)
	}
	for i, d := range h.Days {
		if i > 0 && d.Date <= h.Days[i-1].Date {
			t.Errorf("day %d, %s, follows %s", i, d.Date, h.Days[i-1].Date)
		}
		for _, k := range d.Kinds {
			if k.Count == 0 {
				t.Errorf("the day %s counts a zero", d.Date)
			}
		}
	}
	if d := h.Days[9]; d.Date != "2026-09-26" || d.Events != 3 || d.Commits != 1 || len(d.Kinds) != 3 {
		t.Errorf("the day of W11 is %+v", d)
	}
}

// T6: a subproject's lines (D2). They join from detail on, in the
// Derivation's order, each event marked by its subproject; the counts stay
// the project's own at every level.
func TestSubprojectLines(t *testing.T) {
	f, hashes := station(t, "main")
	firmware := labels(t, "weather-station.firmware")
	for k, v := range firmware {
		hashes[k] = v
	}
	glance := History(f, Query{Task: "c07d", Level: Glance})
	if got := named(glance, hashes); !reflect.DeepEqual(got, []string{"W2 c07d", "W11 c07d"}) {
		t.Errorf("c07d at glance holds %v", got)
	}
	for _, level := range []Level{Detail, Provenance} {
		h := History(f, Query{Task: "c07d", Level: level})
		if got := named(h, hashes); !reflect.DeepEqual(got, []string{"F1 c07d@f1a0", "W2 c07d", "F2 c07d@f1a0", "W11 c07d"}) {
			t.Errorf("c07d at %s holds %v", levels[level], got)
		}
		for _, l := range h.Lines {
			for _, e := range l.Events {
				if taken := e.Commit == hashes["F1"] || e.Commit == hashes["F2"]; taken != (e.Subproject != nil) || taken && (e.Subproject.Task != "f1a0" || e.Subproject.URL != "firmware") {
					t.Errorf("an event of %.7s is marked %+v", e.Commit, e.Subproject)
				}
			}
		}
		if counted(h) != counted(glance) {
			t.Errorf("c07d at %s counts %s, at glance %s", levels[level], counted(h), counted(glance))
		}
	}
	same := entry(t, "subproject-same-repository")
	h := History(same, Query{Level: Detail})
	if got := named(h, labels(t, "subproject-same-repository")); !reflect.DeepEqual(got, []string{"only e4a1", "only b2c9", "only b2c9@5a00"}) {
		t.Errorf("subproject-same-repository holds %v", got)
	}
}

// T7: the range (D3). A range keeps what its start does not reach; a
// subproject's event, what the commit its link reads at the start does not
// reach there.
func TestRange(t *testing.T) {
	_, hashes := station(t, "main")
	firmware := labels(t, "weather-station.firmware")
	for k, v := range firmware {
		hashes[k] = v
	}
	at6, _ := station(t, hashes["W6"])
	if got := named(History(at6, Query{From: hashes["W2"]}), hashes); !reflect.DeepEqual(got, []string{"W3 9f31", "W4 9f31", "W5 3c5d", "W6 3c5d"}) {
		t.Errorf("W2..W6 holds %v", got)
	}
	f, _ := station(t, "main")
	if got := named(History(f, Query{From: hashes["W8"]}), hashes); !reflect.DeepEqual(got, []string{"W9 9f31", "W10 9f31", "W11 c07d", "W12 3c5d", "W13 9f31"}) {
		t.Errorf("W8..main at glance holds %v", got)
	}
	if got := named(History(f, Query{From: hashes["W8"], Level: Detail}), hashes); !reflect.DeepEqual(got, []string{"F1 c07d@f1a0", "F2 c07d@f1a0", "W9 9f31", "W10 9f31", "W11 c07d", "W12 3c5d", "W13 9f31"}) {
		t.Errorf("W8..main at detail holds %v", got)
	}
	sub := entry(t, "submodule-subproject")
	marks := entryLabels(t, "submodule-subproject")
	for from, want := range map[string][]string{
		"U2": {"S2 c100@5a00", "U3 c100", "S3 c100@5a00", "U4 c100"},
		"U3": {"S3 c100@5a00", "U4 c100"},
	} {
		if got := named(History(sub, Query{From: marks[from], Level: Detail}), marks); !reflect.DeepEqual(got, want) {
			t.Errorf("submodule-subproject from %s holds %v; want %v", from, got, want)
		}
	}
}

// T8: proposals. Off the trunk a line is a proposal when the trunk's tip
// does not reach its commit; a branch the trunk merged with --no-ff and a
// project with no trunk give none.
func TestProposals(t *testing.T) {
	repo := clone(t, "weather-station")
	gitRun(t, repo, "checkout", "-q", "-b", "task/7b2e")
	if err := os.WriteFile(filepath.Join(repo, ".tableaux", "status", "7b2e.yaml"), []byte("gate: defined\nstate: nominal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", ".tableaux/status/7b2e.yaml")
	gitRun(t, repo, "commit", "-q", "-m", "Define the gateway")
	commit := gitRun(t, repo, "rev-parse", "HEAD")
	f := derived(t, load.Options{}, repo, "task/7b2e", "")
	proposals := func(h *view.History) []string {
		var out []string
		for _, l := range h.Lines {
			if l.Proposal {
				out = append(out, l.Events[0].Commit+" "+l.Task.ID)
			}
		}
		return out
	}
	if got := proposals(History(f, Query{})); !reflect.DeepEqual(got, []string{commit + " 7b2e"}) {
		t.Errorf("the branch proposes %v", got)
	}
	h := History(f, Query{From: f.Trunk().Tip, Level: Detail})
	if len(h.Lines) != 1 || !h.Lines[0].Proposal || h.Lines[0].Events[0].Commit != commit {
		t.Errorf("the range from the trunk's tip holds %v", named(h, nil))
	}
	board, _ := station(t, "sensor-board")
	if board.OnTrunk() || len(History(board, Query{}).Lines) == 0 {
		t.Fatal("sensor-board stands on the trunk, or holds no line")
	}
	for name, g := range map[string]*derive.Facts{"sensor-board": board, "trunk-undetermined": entry(t, "trunk-undetermined")} {
		if got := proposals(History(g, Query{Level: Detail})); len(got) != 0 {
			t.Errorf("%s proposes %v", name, got)
		}
	}
}

// T9: the person. The author selects; a stranger selects nothing.
func TestPerson(t *testing.T) {
	f, _ := station(t, "main")
	h := History(f, Query{Person: "ada@example.org"})
	if len(h.Lines) != 8 || h.Events != 8 || h.Commits != 6 {
		t.Errorf("ada's history holds %d lines of %d events in %d commits", len(h.Lines), h.Events, h.Commits)
	}
	for _, l := range h.Lines {
		if l.Events[0].By != "ada@example.org" {
			t.Errorf("a line of ada's history is by %s", l.Events[0].By)
		}
	}
	stranger := History(f, Query{Person: "nobody@example.org", Level: Detail})
	if len(stranger.Lines) != 0 || counted(stranger) != "0 0 [0 0 0 0 0 0] 0" {
		t.Errorf("a stranger's history holds %d lines and counts %s", len(stranger.Lines), counted(stranger))
	}
}

// T10: the subtree and the replay. A task's history holds its subtree; the
// subject's status after each line of another task of the project.
func TestSubtreeAndReplay(t *testing.T) {
	f, hashes := station(t, "main")
	h := History(f, Query{Task: "4e2b", Level: Detail})
	tasks := map[string]bool{}
	for _, l := range h.Lines {
		tasks[l.Task.ID] = true
		own := l.Events[0].Subproject == nil
		if (l.Task.ID == "4e2b" || !own) != (l.SubjectAfter == nil) {
			t.Errorf("the line of %s at %.7s has the subject's status %+v", l.Task.ID, l.Events[0].Commit, l.SubjectAfter)
		}
	}
	if len(tasks) != 3 || !tasks["4e2b"] || !tasks["c07d"] || !tasks["9f31"] || h.Subject.ID != "4e2b" {
		t.Errorf("the subtree of 4e2b holds the tasks %v", tasks)
	}
	if s := lineAt(t, h, hashes["W10"], "9f31").SubjectAfter; s == nil || s.Gate != "function" || s.State != "stalled" {
		t.Errorf("after W10 4e2b stands at %+v", s)
	}
	root := History(f, Query{Level: Detail})
	for _, l := range root.Lines {
		own := l.Events[0].Subproject == nil
		if want := own && l.Task.ID != "a1c0"; want != (l.SubjectAfter != nil) {
			t.Errorf("at the root the line of %s at %.7s has the subject's status %+v", l.Task.ID, l.Events[0].Commit, l.SubjectAfter)
		}
	}
}

// T11: after and unchanged. The status after a line is the files' at its
// commit, with no date; a line changes nothing when the files at one of its
// commit's parents state the same.
func TestAfterAndUnchanged(t *testing.T) {
	f, hashes := station(t, "main")
	h := History(f, Query{Level: Detail})
	for _, c := range []struct {
		label, task, after string
		unchanged          bool
	}{
		{"W9", "9f31", "recorded mockup nominal ", true},
		{"W10", "9f31", "recorded function stalled blocked", false},
		{"W4", "9f31", "absent undefined undefined ", true},
		{"W13", "9f31", "recorded function stalled blocked", true},
		{"W2", "c07d", "absent undefined undefined ", false},
	} {
		l := lineAt(t, h, hashes[c.label], c.task)
		after := "null"
		if l.After != nil {
			after = l.After.Kind + " " + l.After.Gate + " " + l.After.State + " " + l.After.Reason
		}
		if after != c.after || l.Unchanged != c.unchanged {
			t.Errorf("%s: after %q, unchanged %v; want %q, %v", c.label, after, l.Unchanged, c.after, c.unchanged)
		}
	}
	for _, l := range h.Lines {
		for _, s := range []*view.Status{l.After, l.SubjectAfter} {
			if s != nil && (s.Date != "" || s.Recorder != "") {
				t.Errorf("the line of %s at %.7s states a status dated %q by %q", l.Task.ID, l.Events[0].Commit, s.Date, s.Recorder)
			}
		}
	}
}

// T12: the effect (D4), by the table of model.md.
func TestEffect(t *testing.T) {
	f, hashes := station(t, "main")
	h := History(f, Query{Level: Detail})
	for _, c := range []struct{ label, task, effect string }{
		{"W1", "a1c0", "authorised_commit"}, {"W2", "c07d", "authorised_commit"},
		{"W3", "9f31", "proposed"}, {"W5", "3c5d", "proposed"},
		{"W4", "9f31", "authorised_merge"}, {"W6", "3c5d", "authorised_trailer"},
		{"W9", "9f31", "accepts"}, {"W12", "3c5d", "proposed"}, {"W7", "9f31", ""},
	} {
		if got := lineAt(t, h, hashes[c.label], c.task).Effect; got != c.effect {
			t.Errorf("%s: the effect is %q; want %q", c.label, got, c.effect)
		}
	}
	for _, c := range []struct {
		entry   string
		effects []string // of the lines of b2c9, in order
	}{
		{"handoff-stale", []string{"authorised_commit", "handoff", "accepts"}},
		{"review-by-non-reviewer", []string{"authorised_commit", "none"}},
		{"trunk-undetermined", []string{""}},
	} {
		var got []string
		for _, l := range History(entry(t, c.entry), Query{Level: Detail}).Lines {
			if l.Task.ID == "b2c9" {
				got = append(got, l.Effect)
			}
		}
		if !reflect.DeepEqual(got, c.effects) {
			t.Errorf("%s: the effects of b2c9 are %q; want %q", c.entry, got, c.effects)
		}
	}
	// A review at defined, of a task whose junction there names a reviewer:
	// the authorisation stands as the review.
	repo := clone(t, "weather-station")
	file := filepath.Join(repo, ".tableaux", "tasks", "7b2e.yaml")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, append(data, "junctions: { defined: { reviewer: ben@example.org } }\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "commit", "-q", "-a", "-m", "Name the gateway's reviewer")
	gitRun(t, repo, "commit", "-q", "--allow-empty", "--author", "Ben Okafor <ben@example.org>", "-m", "Review the gateway", "-m", "Reviewed: 7b2e defined")
	review := gitRun(t, repo, "rev-parse", "HEAD")
	built := derived(t, load.Options{}, repo, "main", "")
	if j := built.Junction("7b2e", "defined"); j == nil || j.Reviewer.V != "ben@example.org" {
		t.Fatalf("the junction of 7b2e at defined is %+v", j)
	}
	l := lineAt(t, History(built, Query{Level: Detail}), review, "7b2e")
	if kinds := eventsOf(l); !reflect.DeepEqual(kinds, []string{"reviewed"}) || l.Effect != "stands" {
		t.Errorf("the review at defined holds %v with the effect %q", kinds, l.Effect)
	}
}

// T13: the model and the committer.
func TestModelAndCommitter(t *testing.T) {
	kinds := entry(t, "junction-kinds")
	marks := labels(t, "junction-kinds")
	h := History(kinds, Query{Level: Detail})
	k6 := lineAt(t, h, marks["K6"], "a110")
	if k6.Model != "claude-fable-5-1" || k6.Check == nil || k6.Check.Gate != "design" || k6.Check.Reading != "agrees" || k6.Check.Commit != marks["K6"] {
		t.Errorf("K6 reads the model %q with the check %+v", k6.Model, k6.Check)
	}
	if k8 := lineAt(t, h, marks["K8"], "a300"); k8.Check == nil || k8.Check.Reading != "differs" {
		t.Errorf("K8 reads the check %+v", k8.Check)
	}
	if w7 := lineAt(t, History(entry(t, "weather-station"), Query{Level: Detail}), labels(t, "weather-station")["W7"], "9f31"); w7.Model != "" || w7.Check != nil {
		t.Errorf("W7 reads the model %q with the check %+v", w7.Model, w7.Check)
	}
	// A commit whose committer is not its author names the committer.
	repo := clone(t, "weather-station")
	if err := os.WriteFile(filepath.Join(repo, ".tableaux", "status", "7b2e.yaml"), []byte("gate: defined\nstate: nominal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "add", ".tableaux/status/7b2e.yaml")
	gitRun(t, repo, "commit", "-q", "--author", "Ada Byrne <ada@example.org>", "-m", "Define the gateway")
	other := gitRun(t, repo, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(repo, ".tableaux", "status", "7b2e.yaml"), []byte("gate: defined\nstate: stalled\nreason: blocked\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "commit", "-q", "-a", "-m", "Stall the gateway")
	own := gitRun(t, repo, "rev-parse", "HEAD")
	built := History(derived(t, load.Options{}, repo, "main", ""), Query{Level: Detail})
	if l := lineAt(t, built, other, "7b2e"); l.Committer != "olive@example.org" || l.Events[0].By != "ada@example.org" {
		t.Errorf("a commit by ada that olive commits names the committer %q", l.Committer)
	}
	if l := lineAt(t, built, own, "7b2e"); l.Committer != "" {
		t.Errorf("a commit olive authors and commits names the committer %q", l.Committer)
	}
}

// T14: provenance. Each line carries its commit; the view carries the two
// commands of README.md#history over the subtree and the range.
func TestHistoryProvenance(t *testing.T) {
	f, hashes := station(t, "main")
	h := History(f, Query{Level: Provenance})
	b := builder{f: f}
	for _, l := range h.Lines {
		log := f.Log()
		if sub := l.Events[0].Subproject; sub != nil {
			for _, link := range f.Project().Links {
				if link.URL == sub.URL {
					log = f.Sub(link).Log()
				}
			}
		}
		if want := b.commit(log.Commit(l.Events[0].Commit)); want == nil || !reflect.DeepEqual(l.Commit, want) {
			t.Errorf("the line of %s at %.7s carries the commit %+v", l.Task.ID, l.Events[0].Commit, l.Commit)
		}
	}
	source := hashes["W13"]
	format := "git log --format='%H %as %ae %s' "
	for _, c := range []struct {
		q    Query
		want []string
	}{
		{Query{Task: "c07d", Level: Provenance}, []string{
			format + source + " -- .tableaux/tasks/c07d.yaml .tableaux/status/c07d.yaml firmware",
			format + "-E --grep='^(Authorised|Reaffirmed): (c07d)$' --grep='^Reviewed: (c07d) ' " + source,
		}},
		{Query{Level: Provenance}, []string{
			format + source + " -- .tableaux/tasks .tableaux/status firmware",
			format + "-E --grep='^(Authorised|Reaffirmed): ([0-9a-f]{4})$' --grep='^Reviewed: ([0-9a-f]{4}) ' " + source,
		}},
		{Query{Task: "4e2b", From: hashes["W8"], Level: Provenance}, []string{
			format + hashes["W8"] + ".." + source + " -- .tableaux/tasks/4e2b.yaml .tableaux/status/4e2b.yaml .tableaux/tasks/9f31.yaml .tableaux/status/9f31.yaml .tableaux/tasks/c07d.yaml .tableaux/status/c07d.yaml firmware",
			format + "-E --grep='^(Authorised|Reaffirmed): (4e2b|9f31|c07d)$' --grep='^Reviewed: (4e2b|9f31|c07d) ' " + hashes["W8"] + ".." + source,
		}},
	} {
		var got []string
		for _, command := range History(f, c.q).Commands {
			got = append(got, command.Text)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%+v: the commands are\n %q\nwant\n %q", c.q, got, c.want)
		}
	}
	none := mem(t, map[string]string{"tasks/e4a1.yaml": leafOf("olive@example.org", "", 0, "")})
	if h := History(none, Query{Level: Provenance}); h.Commands == nil || len(h.Commands) != 0 {
		t.Errorf("with no history the commands are %v", h.Commands)
	}
	if glance := History(f, Query{}); glance.HistoryProvenance != nil {
		t.Error("the history at glance carries commands")
	}
}

// tooling returns the facts of the family's own plan, the umbrella that holds
// the corpus, at a commit, read in place and never written. The test skips
// where the umbrella or the commit is absent.
func tooling(t *testing.T, commit string) *derive.Facts {
	t.Helper()
	dir := filepath.Dir(filepath.Dir(corpus(t)))
	if _, err := os.Stat(filepath.Join(dir, ".tableaux")); err != nil {
		t.Skipf("the family's plan is absent at %s", dir)
	}
	if _, err := load.New(load.Options{CacheDir: t.TempDir()}).Load(t.Context(), load.Source{Dir: dir, Ref: commit}); err != nil {
		t.Skipf("the family's plan does not load at %s: %v", commit, err)
	}
	return derived(t, load.Options{}, dir, commit, "")
}

// T17: the tooling plan at 3cdae52. The history of e9c6 and the root's over
// 0704a09..3cdae52, picture 2 of the mockup.
func TestToolingPlan(t *testing.T) {
	f := tooling(t, "3cdae52")
	e9c6 := History(f, Query{Task: "e9c6", Level: Detail})
	var effects []string
	for _, l := range e9c6.Lines {
		if l.Task.ID == "e9c6" && l.Events[0].Subproject == nil {
			effects = append(effects, l.Effect)
		}
	}
	if e9c6.Events != 7 || e9c6.Commits != 7 || !reflect.DeepEqual(effects, []string{"authorised_commit", "", "stands", "handoff", "handoff", "accepts", ""}) {
		t.Errorf("e9c6 counts %d events in %d commits, with the effects %q", e9c6.Events, e9c6.Commits, effects)
	}
	from := ""
	for _, c := range f.Log().Commits {
		if strings.HasPrefix(c.ID, "0704a09") {
			from = c.ID
		}
	}
	if from == "" {
		t.Fatal("the pass holds no 0704a09")
	}
	root := History(f, Query{From: from, Level: Detail})
	if root.Events != 13 || root.Commits != 7 || root.Kinds[2].Count != 1 || root.Kinds[5].Count != 12 {
		t.Errorf("the root over 0704a09..3cdae52 counts %s", counted(root))
	}
}

// A subproject event's title (decision 8): the subproject task's title as
// the subproject's files at the pin state it, in the task view's events and
// in the history's lines alike; "" where those files lack the task.
func TestSubEventTitle(t *testing.T) {
	f, _ := station(t, "main")
	taken := 0
	for _, l := range History(f, Query{Task: "c07d", Level: Detail}).Lines {
		for _, e := range l.Events {
			if e.Subproject == nil {
				continue
			}
			taken++
			if *e.Subproject != (view.SubEvent{URL: "firmware", Task: "f1a0", Title: "Weather node image"}) {
				t.Errorf("a line taken at %.7s names %+v", e.Commit, e.Subproject)
			}
		}
	}
	for _, e := range Task(f, Query{Task: "c07d", Level: Provenance}).Events {
		if e.Subproject != nil {
			taken++
			if e.Subproject.Title != "Weather node image" {
				t.Errorf("the task view's event at %.7s names %+v", e.Commit, e.Subproject)
			}
		}
	}
	if taken < 4 {
		t.Errorf("%d events are taken from the firmware; want two lines and two events at least", taken)
	}
	for _, l := range History(entry(t, "subproject-same-repository"), Query{Level: Detail}).Lines {
		if s := l.Events[0].Subproject; s != nil && (s.Task != "5a00" || s.Title != "Subproject") {
			t.Errorf("a line taken from the same repository names %+v", s)
		}
	}
	// Over the corpus, each taken event's title is the subproject's own.
	for _, name := range built(t) {
		f := derived(t, load.Options{}, builtAt(t, name), "HEAD", "")
		if f.Refused() != nil {
			continue
		}
		for _, e := range f.History() {
			if e.Sub == "" {
				continue
			}
			if want := refIn(f.Sub(e.Link), e.Sub).Title; event(e).Subproject.Title != want {
				t.Errorf("%s: the event of %s at %.7s names the title %q; the subproject states %q", name, e.Sub, e.Commit.ID, event(e).Subproject.Title, want)
			}
		}
	}
	// Where the subproject's files lack the task, the title is "".
	for _, e := range f.Events("c07d") {
		if e.Sub == "" {
			continue
		}
		lacking := *e
		lacking.Sub, lacking.SubTitle = "ffff", refIn(f.Sub(e.Link), "ffff").Title
		if s := event(&lacking).Subproject; s.Task != "ffff" || s.Title != "" || s.URL != "firmware" {
			t.Errorf("an event of a task the subproject lacks names %+v", s)
		}
		break
	}
}
