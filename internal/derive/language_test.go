package derive

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// The facts of language 0.4.0 (design cdb0).

// TestTaskLevelField is T11: the task-level field makes c100 recursive at
// every gate after defined where its file states no entry, Delegated and
// sourced from c100, lifting the root's exemption at performance; its own
// entries replace it at reliability and release. One snapshot reads the
// subproject, and each move of the field's path is a pin event. On a parent
// the field makes the parent's junctions recursive and the child's plain.
func TestTaskLevelField(t *testing.T) {
	f, hashes := view(t, "subproject-field")
	var kinds []string
	for _, j := range f.Junctions("c100") {
		word := j.Kind.String()
		if j.Delegated {
			word += " delegated"
			if j.Entry != "c100" || fmt.Sprint(j.Sources()) != "[c100]" || j.Snapshot == nil {
				t.Errorf("c100 %s: entry %s, sources %v, snapshot %v", j.Gate, j.Entry, j.Sources(), j.Snapshot)
			}
		}
		kinds = append(kinds, j.Gate+" "+word)
	}
	want := "defined plain, mockup recursive delegated, function recursive delegated, performance recursive delegated, " +
		"reliability not_applicable, design recursive delegated, implementation recursive delegated, unit recursive delegated, " +
		"integrate recursive delegated, validate recursive delegated, release plain"
	if got := strings.Join(kinds, ", "); got != want {
		t.Errorf("c100's junctions:\n got  %s\n want %s", got, want)
	}
	snapshots := f.Snapshots("c100")
	if len(snapshots) != 1 || snapshots[0].Entry != f.Project().Tasks["c100"].Subproject || snapshots[0].Target != "5a00" || len(snapshots[0].Gates) != 8 {
		t.Errorf("c100's snapshots: %+v", snapshots)
	}
	var pins []string
	for _, e := range f.Events("c100") {
		if e.Kind == Pinned {
			pins = append(pins, e.Commit.ID+" "+e.URL)
		}
	}
	if got, want := strings.Join(pins, ", "), hashes["U2"]+" sub, "+hashes["U3"]+" sub, "+hashes["U4"]+" sub"; got != want {
		t.Errorf("c100's pin events: %s; want U2, U3 and U4 at sub", got)
	}

	parent, _ := view(t, "subproject-field-on-parent")
	for _, j := range parent.Junctions("e4a1") {
		if j.Gate != "defined" && (j.Kind != model.Recursive || !j.Delegated) {
			t.Errorf("the parent's %s junction is %s, delegated %v; want recursive by the field", j.Gate, j.Kind, j.Delegated)
		}
	}
	for _, j := range parent.Junctions("b2c9") {
		if j.Kind != model.Plain {
			t.Errorf("the child's %s junction is %s; want plain, since the field names one task's work", j.Gate, j.Kind)
		}
	}
}

// supplier names what supplies a reviewer as crossing.expected.txt writes it.
func supplier(f Field) string {
	switch f.By {
	case ByTask:
		return "task " + f.Task
	case ByAssignee:
		return "assignee"
	case ByNearest:
		return "nearest " + f.Task
	}
	return "default"
}

// describeCrossing writes what the Derivation gives at one label of
// crossing.sh, in the form of crossing.expected.txt without its validate
// lines: the language, each junction of b2c9 and d3a6 with its reviewer's
// supplier and its marks, each review the status passes, and at the last
// label every event of the two leaves with the agent a person's commit
// names in Co-Authored-By:.
func describeCrossing(f *Facts, name string, names map[string]string, events bool) string {
	var b strings.Builder
	b.WriteString(name + ": tableaux " + f.Project().Version.Tableaux.V + "\n")
	for _, id := range []string{"b2c9", "d3a6"} {
		for _, j := range f.Junctions(id) {
			reviewer := j.Reviewer.V
			if reviewer == "" {
				reviewer = "none"
			}
			var marks []string
			for _, m := range j.Marks() {
				marks = append(marks, string(m))
			}
			fmt.Fprintf(&b, "  %s %s %s, reviewer %s by %s, marks %s\n", id, j.Gate, j.Contributor.V, reviewer, supplier(j.Reviewer), strings.Join(marks, " "))
		}
		for _, r := range f.Reviews(id) {
			b.WriteString("  " + id + " review " + r.Gate + ", reviewer " + r.Reviewer + ": " + label(names, r.Commit))
			if r.Commit != nil {
				b.WriteString(" by " + r.By)
			}
			b.WriteString("\n")
		}
	}
	if events {
		for _, id := range []string{"b2c9", "d3a6"} {
			for _, e := range f.Events(id) {
				describeEvent(&b, names, e)
				if e.Reading != nil && e.Reading.CoAuthor != "" {
					b.WriteString("    co-author " + e.Reading.CoAuthor + "\n")
				}
			}
		}
	}
	return b.String()
}

// crossingLines reads crossing.expected.txt without its comments and its
// validate lines, which the root package's test reads.
func crossingLines(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "crossing.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	validating := false
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		switch {
		case strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "  validate:"):
			validating = true
		case validating && strings.HasPrefix(line, "    "):
		default:
			validating = false
			want = append(want, line)
		}
	}
	return want
}

// TestCrossing is T10: one history whose version.yaml rises from 0.3.2 to
// 0.4.0, read at each label of crossing.sh, gives crossing.expected.txt line
// for line. At X2 bot reviews its own design as the assignee and the review
// stays accepted after the raise; at X3 pat's commit of bot's work names bot
// as co-author; from X4 bot is an agent at every junction by the pairing,
// and the nearest person, olive, reviews it.
func TestCrossing(t *testing.T) {
	want := crossingLines(t)
	repo, hashes, order := scripted(t, "crossing")
	names := map[string]string{}
	for name, hash := range hashes {
		names[hash] = name
	}
	var got []string
	for _, name := range order {
		f := derived(t, load.Options{}, repo, hashes[name], "")
		if f.Refused() != nil {
			t.Fatalf("%s: refused: %+v", name, f.Refused())
		}
		text := describeCrossing(f, name, names, name == order[len(order)-1])
		got = append(got, strings.Split(strings.TrimRight(text, "\n"), "\n")...)
	}
	for i := 0; i < len(got) || i < len(want); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(want) {
			w = want[i]
		}
		if g != w {
			t.Errorf("line %d:\n got  %s\n want %s", i+1, g, w)
		}
	}
	if len(order) != 6 {
		t.Errorf("crossing.sh labels %d commits; want 6", len(order))
	}
}

// v040 is the version.yaml of a project at language 0.4.0.
const v040 = "tableaux: 0.4.0\ntrunk: main\n"

// TestIdentityAndReviewer is T9: the kind of an identity and an agent's
// reviewer, built in memory. The file decides where it exists, so x is an
// agent and y, whom an entry pairs with a model, is not, though y's junction
// is still an agent's by its model. An agent assignee under an agent parent
// under a person takes the root's assignee as the nearest person; three
// agents leave the assignee. A waiver at the root reaches a leaf, and the
// leaf's reviewer restores review; true supplies nothing. A parent's default
// marks drop the nearest person's review. Before 0.4.0 the model alone makes
// an agent, and the assignee reviews.
func TestIdentityAndReviewer(t *testing.T) {
	listed := mem(t, map[string]string{
		"version.yaml":    v040,
		"agents.yaml":     "agents: [x@x]\n",
		"tasks/r000.yaml": leafOf("r@x", "", 0, ""),
		"tasks/a000.yaml": leafOf("x@x", "r000", 1, "{ design: { contributor: y@x, model: m-1 } }"),
	})
	if !listed.Agent("x@x") || listed.Agent("y@x") || listed.Agent("r@x") {
		t.Errorf("with the file: x %v, y %v, r %v; want x alone", listed.Agent("x@x"), listed.Agent("y@x"), listed.Agent("r@x"))
	}
	if j := listed.Junction("a000", "design"); !j.Agent || j.Reviewer.V != "r@x" || j.Reviewer.By != ByNearest || j.Reviewer.Task != "r000" {
		t.Errorf("y's junction: agent %v, reviewer %+v; want an agent's by its model, the root's assignee nearest", j.Agent, j.Reviewer)
	}
	if j := listed.Junction("a000", "release"); !j.Agent || j.Contributor.V != "x@x" || j.Reviewer.By != ByNearest {
		t.Errorf("x's default: agent %v, contributor %s, reviewer %+v", j.Agent, j.Contributor.V, j.Reviewer)
	}

	chain := func(version string) *Facts {
		return mem(t, map[string]string{
			"version.yaml":    version,
			"tasks/r000.yaml": leafOf("olive@x", "", 0, ""),
			"tasks/p000.yaml": leafOf("bot@x", "r000", 1, "{ design: { contributor: bot@x, model: m-1 } }"),
			"tasks/a000.yaml": leafOf("bot@x", "p000", 1, ""),
		})
	}
	f := chain(v040)
	if j := f.Junction("a000", "release"); !j.Agent || j.Reviewer.V != "olive@x" || j.Reviewer.By != ByNearest || j.Reviewer.Task != "r000" ||
		j.Reviewer.Node == nil || j.Reviewer.Node.Text != "olive@x" {
		t.Errorf("an agent under an agent under a person: agent %v, reviewer %+v", j.Agent, j.Reviewer)
	}
	if j := f.Junction("a000", "defined"); fmt.Sprint(j.Marks()) != "[agent reviewer]" {
		t.Errorf("the agent's defined junction draws %v", j.Marks())
	}
	if p := f.Junction("p000", "release"); fmt.Sprint(p.Marks()) != "[agent reviewer]" || fmt.Sprint(p.DefaultMarks()) != "[agent]" {
		t.Errorf("the parent's junction draws %v, as a default %v", p.Marks(), p.DefaultMarks())
	}
	if roles := fmt.Sprint(f.Roles("bot@x")); !strings.Contains(roles, "agent") {
		t.Errorf("bot holds the roles %s", roles)
	}
	old := chain("tableaux: 0.3.2\ntrunk: main\n")
	if j := old.Junction("a000", "release"); j.Agent || j.Reviewer.V != "" || fmt.Sprint(j.Marks()) != "[person]" {
		t.Errorf("at 0.3.2 the default is a person's: agent %v, reviewer %+v", j.Agent, j.Reviewer)
	}
	if j := old.Junction("a000", "design"); !j.Agent || j.Reviewer.V != "bot@x" || j.Reviewer.By != ByAssignee {
		t.Errorf("at 0.3.2 the inherited agent is reviewed by %+v; want the assignee", j.Reviewer)
	}

	agents := mem(t, map[string]string{
		"version.yaml":    v040,
		"agents.yaml":     "agents: [r@x, p@x, a@x]\n",
		"tasks/r000.yaml": leafOf("r@x", "", 0, ""),
		"tasks/p000.yaml": leafOf("p@x", "r000", 1, ""),
		"tasks/a000.yaml": leafOf("a@x", "p000", 1, ""),
	})
	if j := agents.Junction("a000", "design"); j.Reviewer.V != "a@x" || j.Reviewer.By != ByAssignee {
		t.Errorf("three agents: the reviewer is %+v; want the assignee", j.Reviewer)
	}

	waived := mem(t, map[string]string{
		"version.yaml":    v040,
		"tasks/r000.yaml": leafOf("olive@x", "", 0, "{ design: { reviewer: false }, release: { reviewer: true } }"),
		"tasks/a000.yaml": leafOf("bot@x", "r000", 1, "{ design: { contributor: bot@x, model: m-1 }, release: { contributor: bot@x, model: m-1 } }"),
		"tasks/b000.yaml": leafOf("bot@x", "r000", 2, "{ design: { contributor: bot@x, model: m-1, reviewer: pat@x } }"),
	})
	if j := waived.Junction("a000", "design"); j.Reviewer.V != "" || j.Reviewer.By != ByTask || j.Reviewer.Task != "r000" || fmt.Sprint(j.Marks()) != "[agent]" {
		t.Errorf("a waiver at the root: the leaf's reviewer is %+v, marks %v", j.Reviewer, j.Marks())
	}
	if waived.Accepted("a000", "design") != nil {
		t.Error("a waived junction has a review")
	}
	if j := waived.Junction("b000", "design"); j.Reviewer.V != "pat@x" || j.Reviewer.Task != "b000" {
		t.Errorf("the leaf's reviewer restores review: %+v", j.Reviewer)
	}
	if j := waived.Junction("a000", "release"); j.Reviewer.V != "olive@x" || j.Reviewer.By != ByNearest {
		t.Errorf("reviewer: true supplies %+v; want nothing, so the nearest person reviews", j.Reviewer)
	}
}

// TestUnpinnedCondition is T13: requires-unpinned reads the library at the
// tip of its trunk, met and due there, AtTrunk yes, with no pin event; the
// facts at the entry's commit read the requirement at the tip too. A history
// that then states the commit has a pin event with no old commit.
func TestUnpinnedCondition(t *testing.T) {
	const url = "https://example.org/lib.git"
	options := load.Options{Replace: map[string]string{url: builtAt(t, "requires-unpinned.lib")}}
	f := derived(t, options, builtAt(t, "requires-unpinned"), "main", "")
	hashes, lib := labels(t, "requires-unpinned"), labels(t, "requires-unpinned.lib")
	conditions := f.Requires("b2c9")
	if len(conditions) != 1 {
		t.Fatalf("b2c9 has %d conditions", len(conditions))
	}
	c := conditions[0]
	if !c.Met || !c.Due || c.AtTrunk != Yes || c.Why != Determined || c.Link == nil || !c.Link.Unpinned || c.Link.Commit != lib["L2"] {
		t.Errorf("the condition: met %v, due %v, at the trunk %s, why %s, link %+v", c.Met, c.Due, c.AtTrunk, c.Why, c.Link)
	}
	for _, e := range f.Events("b2c9") {
		if e.Kind == Pinned {
			t.Errorf("an unpinned requirement makes the pin event %+v", e)
		}
	}
	past := f.At(hashes["only"])
	if p := past.Requires("b2c9"); len(p) != 1 || !p[0].Met || p[0].Facts == nil || p[0].Facts.Project().Where.Commit != lib["L2"] {
		t.Errorf("the past reads the requirement %+v", p)
	}

	// A later commit states the commit: a pin event with no old.
	repo := filepath.Join(t.TempDir(), "repo")
	gitRun(t, t.TempDir(), "clone", "-q", builtAt(t, "requires-unpinned"), repo)
	task := filepath.Join(repo, ".tableaux", "tasks", "b2c9.yaml")
	data, err := os.ReadFile(task)
	if err != nil {
		t.Fatal(err)
	}
	pinned := strings.Replace(string(data), `id: "5a00" }`, `id: "5a00", commit: `+lib["L2"]+` }`, 1)
	if pinned == string(data) {
		t.Fatal("the task file holds no unpinned requirement to pin")
	}
	if err := os.WriteFile(task, []byte(pinned), 0o644); err != nil {
		t.Fatal(err)
	}
	gitRun(t, repo, "commit", "-q", "-a", "-m", "State the library's commit")
	g := derived(t, options, repo, "HEAD", "main")
	var pins []string
	for _, e := range g.Events("b2c9") {
		if e.Kind == Pinned {
			pins = append(pins, e.URL+" "+e.Old+" "+e.New)
		}
	}
	if got := strings.Join(pins, ", "); got != url+"  "+lib["L2"] {
		t.Errorf("the pin events are %q; want one at %s with no old commit", got, url)
	}
}

// TestMergeAuthorises is T12: on authorised-by-merge, olive's merge A4 on the
// trunk's first-parent line brings in c3d4's file: an authorised event, the
// deciding commit by merge, while A3 keeps the status's date and recorder.
func TestMergeAuthorises(t *testing.T) {
	f, hashes := view(t, "authorised-by-merge")
	names := map[string]string{}
	for name, hash := range hashes {
		names[hash] = name
	}
	var events []string
	for _, e := range f.Events("c3d4") {
		events = append(events, label(names, e.Commit)+" "+e.Commit.Author.Email+" "+string(e.Kind))
	}
	if got := strings.Join(events, ", "); got != "A2 pat@example.org task, A3 pat@example.org status, A4 olive@example.org authorised" {
		t.Errorf("c3d4's events: %s", got)
	}
	if a := f.Authorisation("c3d4"); a == nil || !a.Authorised || a.Way != ByMerge || label(names, a.Commit) != "A4" {
		t.Errorf("c3d4's authorisation: %+v", a)
	}
	if s := f.Status("c3d4"); s == nil || label(names, s.Commit) != "A3" || s.Recorder != "pat@example.org" {
		t.Errorf("c3d4's status: %+v", s)
	}
}

// TestMoot is T8, the Derivation's half: a reaffirmed event says why it
// confirms nothing, as the files stand at its commit: a leaf whose next
// junction is recursive, a parent, a leaf with no status file; and nothing
// for a leaf with a status of its own.
func TestMoot(t *testing.T) {
	moots := func(f *Facts, id string) string {
		var out []string
		for _, e := range f.Events(id) {
			if e.Kind == Reaffirmed {
				out = append(out, e.Moot.String())
			}
		}
		return fmt.Sprintf("%q", out)
	}
	snapshot, _ := view(t, "reaffirmed-snapshot")
	if got := moots(snapshot, "b2c9"); got != `["recursive"]` {
		t.Errorf("reaffirmed-snapshot b2c9: %s", got)
	}
	if got := moots(snapshot, "e4a1"); !strings.Contains(got, `"parent"`) {
		t.Errorf("reaffirmed-snapshot e4a1: %s", got)
	}
	undefined, _ := view(t, "undefined-leaf-reaffirmed")
	if got := moots(undefined, "b2c9"); got != `["absent"]` {
		t.Errorf("undefined-leaf-reaffirmed b2c9: %s", got)
	}
	station, _ := view(t, "weather-station")
	if got := moots(station, "9f31"); got != `[""]` {
		t.Errorf("weather-station 9f31: %s; want one reaffirmation that confirms its status", got)
	}
}
