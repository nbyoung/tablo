package derive

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// T22: the roles README.md#roles gives the people of the weather station.
func TestRoles(t *testing.T) {
	f, _ := view(t, "weather-station")
	for email, want := range map[string][]Role{
		"ada@example.org":  {Owner, AuthorityOf, Assignee, Contributor},
		"ben@example.org":  {AuthorityOf, Assignee, Contributor, Reviewer},
		"opus@example.org": {Contributor, Agent},
		"dan@example.org":  {Assignee, Contributor},
		"eve@example.org":  {Observer},
		"":                 {Observer},
	} {
		if got := f.Roles(email); !reflect.DeepEqual(got, want) {
			t.Errorf("%q holds %v; want %v", email, got, want)
		}
	}
	// A parent's junctions are defaults: the reviewer the root states at
	// release holds the role through the leaves that inherit it.
	kinds, _ := view(t, "junction-kinds")
	if got, want := kinds.Roles("olive@example.org"), []Role{Owner, AuthorityOf, Assignee, Reviewer}; !reflect.DeepEqual(got, want) {
		t.Errorf("olive holds %v; want %v", got, want)
	}
	if got, want := kinds.Roles("bot@example.org"), []Role{Assignee, Contributor, Agent, Reviewer}; !reflect.DeepEqual(got, want) {
		t.Errorf("bot holds %v; want %v", got, want)
	}
}

// id returns a commit's id, or "none".
func id(c *history.Commit) string {
	if c == nil {
		return "none"
	}
	return c.ID
}

// dump writes every fact of a project and of the projects its links reach,
// with each commit by its id and no path of the host.
func dump(f *Facts, depth int) string {
	var b strings.Builder
	p := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }
	trunk := f.Trunk()
	p("project at %s: refused %v, trunk %+v, on it %v, root %s, owner %s, order %v, gates %v",
		f.Project().Where.Commit, f.Refused(), trunk, f.OnTrunk(), f.Root(), f.Owner(), f.Order(), f.Gates())
	if log := f.Log(); log != nil {
		for _, c := range log.Commits {
			p("commit %+v", *c)
		}
	}
	emails := map[string]bool{}
	for _, task := range f.Order() {
		p("task %s: children %v, leaf %v, authorities %v, applicable %v, first %s, last %s",
			task, f.Children(task), f.Leaf(task), f.Authorities(task), f.Applicable(task), f.First(task), f.Last(task))
		emails[f.Project().Tasks[task].Assignee.V] = true
		for _, j := range f.Junctions(task) {
			field := func(x Field) string { return fmt.Sprintf("%s/%d/%s/%v", x.V, x.By, x.Task, x.Node != nil) }
			p("  junction %s: %v %s %s %s, %d references from %q, entry %q, sources %v, marks %v, next %s",
				j.Gate, j.Kind, field(j.Contributor), field(j.Model), field(j.Reviewer), len(j.References), j.ReferencesFrom,
				j.Entry, j.Sources(), j.Marks(), f.Next(task, j.Gate))
			emails[j.Contributor.V], emails[j.Reviewer.V] = true, true
			if r := f.Accepted(task, j.Gate); r != nil {
				p("    accepted: %s by %q at %s, by authorisation %v", r.Reviewer, r.By, id(r.Commit), r.ByAuthorisation)
			}
		}
		for _, s := range f.Snapshots(task) {
			p("  snapshot %v: %s %s, %v, facts %v", s.Gates, s.Entry.URL.V, s.Target, s.Why, s.Facts != nil)
		}
		for _, s := range f.Chain(task) {
			p("  status %s: kind %d, %s %s %q %q, %v, from %q of %v, file %v, of %v, %s %s %q, own %s, uncommitted %v, derived %v",
				s.Task, s.Kind, s.Gate, s.State, s.Reason, s.Note, s.Why, s.From, s.Considered, s.File != nil, s.Of != nil,
				id(s.Commit), s.Date, s.Recorder, id(s.Own), s.Uncommitted, s.Derived())
		}
		for _, list := range [][]*Condition{f.Requires(task), f.Dependents(task)} {
			for _, c := range list {
				p("  condition %s on %s: %s to %s, stands %s, met %v, due %v, %q, %v, at trunk %v, link %v",
					c.Task, c.Origin, c.From, c.To, c.Stands, c.Met, c.Due, c.Word(), c.Why, c.AtTrunk, c.Link != nil)
			}
		}
		a := f.Authorisation(task)
		p("  authorisation: %v, %v, %s, way %v, judges %v, %v %v, by %q, differs %v, uncommitted %v",
			a.Authorised, a.Why, id(a.Commit), a.Way, a.Judges, a.Author, a.Committer, a.By, a.Differs, a.Uncommitted)
		for _, r := range f.Reviews(task) {
			p("  review %s: %s by %q at %s", r.Gate, r.Reviewer, r.By, id(r.Commit))
		}
		if h := f.Handoff(task); h != nil {
			p("  hand-off: %v at %s to %s, self %v, %s", h.Kind, h.Gate, h.Reviewer, h.Self, id(h.Commit))
		}
		for _, e := range f.Events(task) {
			p("  event %s %s %s %s: %s %s %q %q, pin %q %q %q, link %v, sub %q, reading %v, effect %v",
				e.Commit.Date(), e.Commit.ID, e.Task, e.Kind, e.Gate, e.State, e.Reason, e.Note, e.URL, e.Old, e.New,
				e.Link != nil, e.Sub, e.Reading, e.Effect)
		}
	}
	for _, e := range f.History() {
		p("history %s %s %s %q", e.Commit.ID, e.Task, e.Kind, e.Sub)
	}
	for _, u := range f.Unread() {
		p("unread %s %q %v %v", u.Commit.ID, u.Line, u.NoTask, u.NoGate)
	}
	var people []string
	for email := range emails {
		people = append(people, email)
	}
	sort.Strings(people)
	for _, email := range people {
		p("roles of %q: %v", email, f.Roles(email))
	}
	if log := f.Log(); log != nil && depth == 0 {
		for _, c := range log.Commits {
			if past := f.At(c.ID); past != nil {
				for _, task := range past.Order() {
					s := past.Status(task)
					p("at %s %s: %s %s %q from %q, %v", c.ID, task, s.Gate, s.State, s.Note, s.From, s.Why)
				}
			}
			for _, task := range f.Order() {
				a := f.AuthorisationAt(task, c.ID)
				p("at %s %s: authorised %v, %v, %s", c.ID, task, a.Authorised, a.Why, id(a.Commit))
			}
		}
	}
	for _, link := range f.Project().Links {
		pin := f.Pin(link)
		p("link %s %v %s: recorded %q, moved %v, tip %q, on trunk %v, behind %v",
			link.URL, link.Form, pin.Commit, pin.Recorded, pin.Moved, pin.Tip, pin.OnTrunk, pin.Behind)
		if sub := f.Sub(link); sub != nil && depth < 3 {
			b.WriteString(dump(sub, depth+1))
		}
	}
	return b.String()
}

// T24: deterministic and safe to share. Two clones at two paths, read
// under two time zones, give the same facts; fifty goroutines read one
// Facts; and the package reads no clock and no environment.
func TestDeterministic(t *testing.T) {
	for _, name := range []string{"weather-station", "junction-kinds", "submodule-subproject"} {
		t.Setenv("TZ", "Pacific/Kiritimati")
		first := derived(t, load.Options{}, clone(t, name), "main", "")
		t.Setenv("TZ", "America/Los_Angeles")
		second := derived(t, load.Options{}, clone(t, name), "main", "")
		if first.Project().Where.GitDir == second.Project().Where.GitDir {
			t.Fatalf("%s: the two clones share a path", name)
		}
		a, b := dump(first, 0), dump(second, 0)
		if a != b {
			t.Errorf("%s: two clones give different facts", name)
			x, y := strings.Split(a, "\n"), strings.Split(b, "\n")
			for i := 0; i < len(x) && i < len(y); i++ {
				if x[i] != y[i] {
					t.Fatalf("line %d:\n one %s\n two %s", i+1, x[i], y[i])
				}
			}
		}
		if len(a) < 2000 || strings.Contains(a, first.Project().Where.GitDir) {
			t.Errorf("%s: the dump holds %d bytes, or a path of the host", name, len(a))
		}

		// Fifty goroutines read one Facts that has computed nothing yet.
		shared := derived(t, load.Options{}, builtAt(t, name), "main", "")
		dumps := make([]string, 50)
		var wg sync.WaitGroup
		for i := range dumps {
			wg.Add(1)
			go func() {
				defer wg.Done()
				dumps[i] = dump(shared, 0)
			}()
		}
		wg.Wait()
		for i, d := range dumps {
			if d != a {
				t.Fatalf("%s: goroutine %d reads other facts than a lone reader", name, i)
			}
		}
		// A fact is computed once and kept.
		root := shared.Root()
		status, junction, authorisation := shared.Status(root), shared.Junction(root, "defined"), shared.Authorisation(root)
		if shared.Status("zzzz") != nil || status != shared.Status(root) || junction != shared.Junction(root, "defined") ||
			authorisation != shared.Authorisation(root) {
			t.Errorf("%s: a fact is computed twice", name)
		}
	}
}

// T24: derive imports neither os nor time, and runs no process.
func TestImports(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("the package's files: %v, %v", files, err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			switch imported, _ := strconv.Unquote(spec.Path.Value); {
			case imported == "os" || imported == "time" || strings.HasPrefix(imported, "os/"),
				imported == "github.com/nbyoung/tablo/internal/git", imported == "github.com/nbyoung/tablo/internal/load",
				imported == "github.com/nbyoung/tablo/internal/validate":
				t.Errorf("%s imports %s", name, imported)
			}
		}
	}
}

// tooling returns the family's own plan, the umbrella that holds the corpus,
// read in place and never written: the repository two directories above the
// built corpus, at its trunk. The test skips when it is not there.
func tooling(t testing.TB) (dir string) {
	t.Helper()
	dir = filepath.Dir(filepath.Dir(corpus(t)))
	if _, err := os.Stat(filepath.Join(dir, ".tableaux")); err != nil {
		t.Skipf("the family's plan is absent at %s", dir)
	}
	if _, err := load.New(load.Options{CacheDir: t.TempDir()}).Load(context.Background(), load.Source{Dir: dir, Ref: "main"}); err != nil {
		t.Skipf("the family's plan does not load at main: %v", err)
	}
	return dir
}

// T25: the family's own plan, tableaux-tooling, read at its trunk. The
// design counts 37 tasks, none proposed and four pins on their trunks on its
// day; the plan moves, so the test asserts what the rules give and logs the
// counts.
func TestTooling(t *testing.T) {
	f := derived(t, load.Options{}, tooling(t), "main", "")
	if f.Refused() != nil {
		t.Fatalf("the plan is refused: %+v", f.Refused())
	}
	if !f.OnTrunk() || f.Trunk().How() != "stated" || f.Trunk().Name != "main" {
		t.Errorf("the trunk is %+v, and the source on it %v; want main, stated", f.Trunk(), f.OnTrunk())
	}
	proposed, leaves := 0, 0
	for _, task := range f.Order() {
		a := f.Authorisation(task)
		switch {
		case a.Commit == nil || a.Commit.Trunk < 0 || a.Why != Determined:
			t.Errorf("%s: the authorisation is %+v; want a deciding commit on the trunk's line", task, a)
		case a.Authorised != (a.Author || a.Committer) || a.Authorised && !contains(a.Judges, a.By):
			t.Errorf("%s: the authorisation is %+v; want it accepted by a judge", task, a)
		}
		if !a.Authorised {
			proposed++
			t.Logf("%s is proposed: decided by %s, judges %v", task, a.By, a.Judges)
		}
		s := f.Status(task)
		if f.Leaf(task) {
			leaves++
			if s.Kind == RolledUp || s.Kind != Absent && s.File == nil {
				t.Errorf("%s: a leaf's status is %+v", task, s)
			}
		} else if s.Kind != RolledUp || s.From != "" && !contains(f.Children(task), s.From) {
			t.Errorf("%s: a parent's status is %+v", task, s)
		}
		for _, r := range f.Reviews(task) {
			if r.Commit == nil && !r.ByAuthorisation {
				t.Logf("%s passes %s with no review by %s", task, r.Gate, r.Reviewer)
			}
		}
	}
	events := f.History()
	for i := 1; i < len(events); i++ {
		if events[i].Commit.Time < events[i-1].Commit.Time {
			t.Fatalf("event %d comes before its elder", i)
		}
	}
	pins, onTrunk, subTasks, subProposed := 0, 0, 0, 0
	for _, link := range f.Project().Links {
		if link.Form != model.Submodule {
			continue
		}
		pins++
		pin := f.Pin(link)
		if pin.OnTrunk == Yes {
			onTrunk++
		}
		t.Logf("pin %s at %s: on its trunk %v, behind %v, %v", link.URL, pin.Commit, pin.OnTrunk, pin.Behind, link.Problem)
		if sub := f.Sub(link); sub != nil && sub.Refused() == nil {
			for _, task := range sub.Order() {
				subTasks++
				if !sub.Authorisation(task).Authorised {
					subProposed++
				}
			}
			// A linked repository takes the remote-tracking branch first (decision 5).
			if ref := sub.Trunk().Ref; ref != "" && !strings.HasPrefix(ref, "refs/remotes/") && pin.Tip != "" {
				if log := sub.Log(); log.Commit(pin.Tip) == nil {
					t.Errorf("pin %s: the trunk %s is no commit of the pass", link.URL, ref)
				}
			}
		}
	}
	exercise(f, 0)
	t.Logf("%d tasks (the design counts 37), %d leaves, %d proposed (none), %d commits, %d events", len(f.Order()), leaves, proposed, len(f.Log().Commits), len(events))
	t.Logf("%d pins, %d on their trunks (four of four); %d subproject tasks, %d proposed (none of 42)", pins, onTrunk, subTasks, subProposed)
}

// facts asks every task of a project for every fact.
func facts(f *Facts) int {
	n := 0
	for _, task := range f.Order() {
		_, _, _, _ = f.Junctions(task), f.Status(task), f.Requires(task), f.Authorisation(task)
		_, _, _ = f.Reviews(task), f.Handoff(task), f.Events(task)
		n += len(f.Events(task))
	}
	_, _ = f.History(), f.Unread()
	return n
}

// The cost on the family's plan: a benchmark prints it and asserts nothing.
// Passes runs every process of a first load; Reload runs the ref listings
// alone; Facts derives every fact of every project from the passes.
func BenchmarkTooling(b *testing.B) {
	dir := tooling(b)
	ctx := context.Background()
	p := loaded(b, load.Options{}, dir, "main")
	b.Run("Passes", func(b *testing.B) {
		for b.Loop() {
			if _, err := history.NewReader(history.Options{}).Read(ctx, p, ""); err != nil {
				b.Fatal(err)
			}
		}
	})
	reader := history.NewReader(history.Options{})
	set, err := reader.Read(ctx, p, "")
	if err != nil {
		b.Fatal(err)
	}
	b.Run("Reload", func(b *testing.B) {
		for b.Loop() {
			if _, err := reader.Read(ctx, p, ""); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("Facts", func(b *testing.B) {
		tasks, events := 0, 0
		for b.Loop() {
			f := New(p, set)
			tasks, events = len(f.Order()), facts(f)
			for _, link := range p.Links {
				if sub := f.Sub(link); sub != nil {
					tasks += len(sub.Order())
					events += facts(sub)
				}
			}
		}
		b.ReportMetric(float64(tasks), "tasks")
		b.ReportMetric(float64(events), "events")
	})
}
