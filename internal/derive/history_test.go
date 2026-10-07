package derive

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// view derives the facts of a built corpus entry at main.
func view(t *testing.T, name string) (*Facts, map[string]string) {
	t.Helper()
	return derived(t, load.Options{}, builtAt(t, name), "main", ""), labels(t, name)
}

// T12: hand-offs and the effects of a review, in the corpus.
func TestHandoffsAndEffects(t *testing.T) {
	inferred, hashes := view(t, "handoff-inferred")
	if h := inferred.Handoff("b2c9"); h == nil || h.Kind != Implied || h.Gate != "design" || h.Reviewer != "olive@example.org" ||
		h.Self || h.Commit == nil || h.Commit.ID != hashes["I2"] || h.Task != "b2c9" {
		t.Errorf("handoff-inferred: the hand-off is %+v; want it implied at design by I2", h)
	}
	if inferred.Handoff("e4a1") != nil || inferred.Handoff("zzzz") != nil {
		t.Error("handoff-inferred: a parent or a task the tree lacks has a hand-off")
	}
	stale, hashes := view(t, "handoff-stale")
	if h := stale.Handoff("b2c9"); h == nil || h.Kind != Stale || h.Commit.ID != hashes["S3"] {
		t.Errorf("handoff-stale: the hand-off is %+v; want it stale by S3", h)
	}
	if r := stale.Accepted("b2c9", "design"); r == nil || r.Commit == nil || r.Commit.ID != hashes["S3"] || r.By != "olive@example.org" || r.ByAuthorisation {
		t.Errorf("handoff-stale: the review of design is %+v; want S3, though the status does not pass it yet", r)
	}

	// A review from the wrong hand is an event with no effect, and the
	// history implies no hand-off from it.
	wrong, hashes := view(t, "review-by-non-reviewer")
	events := wrong.Events("b2c9")
	if last := events[len(events)-1]; last.Kind != Reviewed || last.Effect != WrongHand || last.Commit.ID != hashes["R2"] || last.Gate != "design" {
		t.Errorf("review-by-non-reviewer: the last event is %+v; want a review from the wrong hand at R2", last)
	}
	if r := wrong.Accepted("b2c9", "design"); r == nil || r.Commit != nil || r.By != "" || r.Reviewer != "olive@example.org" {
		t.Errorf("review-by-non-reviewer: the review is %+v; want none to accept", r)
	}
	if h := wrong.Handoff("b2c9"); h == nil || h.Kind != NoHandoff || h.Commit != nil {
		t.Errorf("review-by-non-reviewer: the hand-off is %+v; want none", h)
	}

	kinds, hashes := view(t, "junction-kinds")
	effects := map[string]Effect{}
	for _, e := range kinds.History() {
		if e.Kind == Reviewed {
			effects[e.Task+" "+e.Gate] = e.Effect
		} else if e.Effect != NoEffect {
			t.Errorf("junction-kinds: the %s event of %s has the effect %v", e.Kind, e.Task, e.Effect)
		}
	}
	if want := map[string]Effect{"a110 function": Accepts, "a110 design": Accepts, "a300 design": Accepts}; !reflect.DeepEqual(effects, want) {
		t.Errorf("junction-kinds: the reviews have the effects %v; want %v", effects, want)
	}
	// The agent reviews itself: the hand-off of its next junction says so.
	if r := kinds.Accepted("a300", "design"); r.Commit.ID != hashes["K8"] || r.By != "bot@example.org" {
		t.Errorf("junction-kinds: a300's design is accepted by %+v; want K8, the commit that records the status", r)
	}
	if kinds.Accepted("a110", "defined") != nil || kinds.Accepted("a400", "implementation") != nil || kinds.Accepted("a110", "nowhere") != nil {
		t.Error("junction-kinds: a junction with no reviewer has a review")
	}

	// At defined the authorisation stands as the review.
	byAuthorisation, hashes := view(t, "status-defined-by-authorisation")
	if r := byAuthorisation.Accepted("b2c9", "defined"); r == nil || !r.ByAuthorisation || r.Commit.ID != hashes["only"] || r.By != "olive@example.org" {
		t.Errorf("status-defined-by-authorisation: the review of defined is %+v", r)
	}
	// A status past a reviewed junction that nothing accepts: the review has no commit.
	unreviewed, _ := view(t, "status-unreviewed-gate")
	if list := unreviewed.Reviews("b2c9"); len(list) != 1 || list[0].Gate != "design" || list[0].Commit != nil || list[0].ByAuthorisation {
		t.Errorf("status-unreviewed-gate: the reviews are %+v; want design with no commit", list)
	}
}

// T15: the model check, at the junction each commit is at.
func TestModelCheck(t *testing.T) {
	readings := func(f *Facts, id string, hashes map[string]string) map[string]Reading {
		names := map[string]string{}
		for name, hash := range hashes {
			names[hash] = name
		}
		got := map[string]Reading{}
		for _, e := range f.Events(id) {
			if (e.Reading != nil) != (e.Kind == StatusSet || e.Kind == Reaffirmed) {
				t.Errorf("the %s event of %s carries the reading %v", e.Kind, id, e.Reading)
			}
			if e.Reading != nil {
				got[names[e.Commit.ID]] = *e.Reading
			}
		}
		return got
	}
	mismatch, hashes := view(t, "model-mismatch")
	got := readings(mismatch, "b2c9", hashes)
	if r := got["M2"]; r != (Reading{Gate: "design", Contributor: "bot@example.org", Stated: "claude-fable", Trailer: "claude-sonnet-5", Verdict: Mismatch}) {
		t.Errorf("model-mismatch: M2 reads %+v; want a mismatch at design", r)
	}
	// M4 leaves the gate alone, so it is at the next junction, which states no model.
	if r := got["M4"]; r.Gate != "release" || r.Verdict != Unstated || r.Trailer != "claude-fable-5-1" {
		t.Errorf("model-mismatch: M4 reads %+v; want a trailer at a junction that states no model", r)
	}
	if r := got["M1"]; r.Gate != "defined" || r.Verdict != NoModel || r.Contributor != "pat@example.org" {
		t.Errorf("model-mismatch: M1 reads %+v; want no model at defined", r)
	}

	missing, hashes := view(t, "model-trailer-missing")
	got = readings(missing, "b2c9", hashes)
	if r := got["M2"]; r.Gate != "design" || r.Verdict != Exempt || r.Stated != "claude-fable" || r.Trailer != "" {
		t.Errorf("model-trailer-missing: M2 reads %+v; want it exempt at design, under 0.2.0", r)
	}
	if r := got["M4"]; r.Gate != "design" || r.Verdict != Missing {
		t.Errorf("model-trailer-missing: M4 reads %+v; want the trailer missing at design", r)
	}

	kinds, hashes := view(t, "junction-kinds")
	got = readings(kinds, "a110", hashes)
	if r := got["K6"]; r.Gate != "design" || r.Verdict != Match || r.Stated != "claude-fable" || r.Trailer != "claude-fable-5-1" {
		t.Errorf("junction-kinds: K6 reads %+v; want a match by prefix at design", r)
	}
	// pat records function: another hand than no agent's, and no model there.
	if r := got["K4"]; r.Gate != "function" || r.Verdict != NoModel {
		t.Errorf("junction-kinds: K4 reads %+v; want no model at function", r)
	}
	if r := readings(kinds, "a300", hashes)["K8"]; r.Gate != "design" || r.Verdict != Mismatch || r.Stated != "claude-fable-5-1" {
		t.Errorf("junction-kinds: K8 reads %+v; want a mismatch at design", r)
	}

	// A reaffirmation is at the next junction, and another hand's commit with
	// no trailer is no agent's.
	station, hashes := view(t, "weather-station")
	if r := readings(station, "9f31", hashes)["W13"]; r.Gate != "performance" || r.Verdict != NoModel || r.Contributor != "ada@example.org" {
		t.Errorf("weather-station: W13 reads %+v; want the reaffirmation at performance", r)
	}
	for v, want := range map[*model.Version]bool{
		nil: false, {WellFormed: true, Minor: 2}: true, {WellFormed: true, Minor: 2, Patch: 1}: false,
		{WellFormed: true, Minor: 1, Patch: 9}: true, {WellFormed: true, Minor: 3}: false,
		{WellFormed: true, Major: 1}: false, {Minor: 1}: false,
	} {
		if before021(v) != want {
			t.Errorf("before021(%+v) is %v", v, !want)
		}
	}
}

// T16: unread trailers make no event.
func TestUnreadTrailers(t *testing.T) {
	f, hashes := view(t, "unknown-trailer")
	var got []Unread
	for _, u := range f.Unread() {
		if u.Commit.ID != hashes["U2"] {
			t.Errorf("the trailer %q is on %s; want U2", u.Line, u.Commit.ID)
		}
		got = append(got, Unread{Line: u.Line, NoTask: u.NoTask, NoGate: u.NoGate})
	}
	want := []Unread{{Line: "Authorised: zzzz", NoTask: true}, {Line: "Reviewed: 9f31 nowhere", NoTask: true, NoGate: true}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("the unread trailers are %+v; want %+v", got, want)
	}
	for _, e := range f.History() {
		if e.Commit.ID != hashes["U1"] {
			t.Errorf("the %s event of %s is on %s; want every event on U1", e.Kind, e.Task, e.Commit.ID)
		}
	}
	if len(f.History()) != 3 || f.Events("zzzz") != nil || f.Events("9f31") != nil {
		t.Errorf("the history holds %d events; want the three of U1 and none for a task the project lacks", len(f.History()))
	}
	// A review of a gate that does not apply, and one of no gate, are unread too.
	kinds, _ := view(t, "junction-kinds")
	if len(kinds.Unread()) != 0 {
		t.Errorf("junction-kinds has the unread trailers %+v", kinds.Unread())
	}
}

// T17: snapshots and pins.
func TestPins(t *testing.T) {
	off, _ := view(t, "subproject-pin-off-trunk")
	sub := labels(t, "subproject-pin-off-trunk.sub")
	link := off.Project().Links[0]
	if pin := off.Pin(link); pin.Commit != sub["S2"] || pin.OnTrunk != No || pin.Behind != Unknown || pin.Tip == "" || pin.Tip == pin.Commit || pin.Link != link {
		t.Errorf("subproject-pin-off-trunk: the pin is %+v; want S2 off its trunk", pin)
	}
	// The pinned task still reads, and its commit reads as a proposal there.
	if s := off.Status("b2c9"); s.Kind != Snapshotted || s.Commit == nil || s.Commit.ID != sub["S2"] || s.Commit.Trunk != -1 || s.Why != Determined {
		t.Errorf("subproject-pin-off-trunk: the status is %+v", s)
	}
	if a := off.Sub(link).Authorisation("5a00"); a.Authorised || a.Why != OffTrunk {
		t.Errorf("subproject-pin-off-trunk: the task read there is %+v; want it proposed off the trunk", a)
	}

	behind, hashes := view(t, "submodule-subproject")
	sub = labels(t, "submodule-subproject.sub")
	link = behind.Project().Links[0]
	want := Pin{Link: link, Commit: sub["S3"], Recorded: sub["S3"], Tip: sub["S4"], OnTrunk: Yes, Behind: Yes}
	if pin := behind.Pin(link); *pin != want {
		t.Errorf("submodule-subproject: the pin is %+v; want %+v", pin, want)
	}
	s := behind.Status("c100")
	if s.Own == nil || s.Own.ID != hashes["U4"] || s.Of == nil || s.Of.Task != "5a00" || s.Of.Kind != Recorded || s.Commit != s.Of.Commit || s.Snapshot.Facts != behind.Sub(link) {
		t.Errorf("submodule-subproject: the status is %+v; want its own deciding commit U4 beside the subproject's", s)
	}
	if chain := behind.Chain("c000"); len(chain) != 3 || chain[2] != s.Of {
		t.Errorf("submodule-subproject: the chain holds %d statuses; want the root, c100 and the task read there", len(chain))
	}

	url := derived(t, load.Options{Replace: map[string]string{"https://example.org/lib.git": builtAt(t, "subproject-by-url.lib")}},
		builtAt(t, "subproject-by-url"), "main", "")
	lib := labels(t, "subproject-by-url.lib")
	if pin := url.Pin(url.Project().Links[0]); pin.Commit != lib["L2"] || pin.OnTrunk != Yes || pin.Behind != Yes || pin.Recorded != "" || pin.Moved {
		t.Errorf("subproject-by-url: the pin is %+v; want L2 on its trunk and behind its tip", pin)
	}
	// With no clone the link has no project: the gate stands and the state is empty.
	unmapped := derived(t, load.Options{}, builtAt(t, "subproject-by-url"), "main", "")
	if s := unmapped.Status("b2c9"); s.Kind != Snapshotted || s.Why != NoLink || s.Gate != "defined" || s.State != "" || s.Of != nil || s.Own == nil {
		t.Errorf("subproject-by-url with no clone: the status is %+v; want NoLink", s)
	}
	if pin := unmapped.Pin(unmapped.Project().Links[0]); pin.OnTrunk != Unknown || pin.Tip != "" || pin.Commit != lib["L2"] {
		t.Errorf("subproject-by-url with no clone: the pin is %+v", pin)
	}

	same, _ := view(t, "subproject-same-repository")
	if pin := same.Pin(same.Project().Links[0]); pin.Commit != "" || pin.OnTrunk != Unknown || pin.Behind != Unknown || pin.Tip != same.Log().Source {
		t.Errorf("subproject-same-repository: the pin is %+v; want no commit and the trunk's tip", pin)
	}

	missing, _ := view(t, "subproject-path-missing")
	if list := missing.Snapshots("b2c9"); len(list) != 1 || list[0].Why != NoLink || list[0].Facts != nil {
		t.Fatalf("subproject-path-missing: the snapshots are %+v; want one with NoLink", list)
	}
	// The junction that leads nowhere is not the next one, so the file's status stands.
	if s := missing.Status("b2c9"); s.Kind != Recorded || s.Why != Determined || s.State != "nominal" {
		t.Errorf("subproject-path-missing: the status is %+v; want the file's own", s)
	}
	noTask, _ := view(t, "subproject-task-missing")
	if list := noTask.Snapshots("b2c9"); len(list) != 1 || list[0].Why != NoTask || list[0].Facts == nil {
		t.Errorf("subproject-task-missing: the snapshots are %+v; want one with NoTask", list)
	}
	if off.Pin(nil) != nil {
		t.Error("no link has a pin")
	}
	// The condition of an entry whose commit lags the trunk: unmet there, met at the tip.
	lags := derived(t, load.Options{Replace: map[string]string{"https://example.org/lib.git": builtAt(t, "requires-commit-behind.lib")}},
		builtAt(t, "requires-commit-behind"), "main", "")
	if c := lags.Requires("b2c9")[0]; c.Word() != "unmet" || c.AtTrunk != Yes || c.Stands != "defined" {
		t.Errorf("requires-commit-behind: the condition is %+v; want it unmet at the commit and met at the trunk's tip", c)
	}
	across := derived(t, load.Options{Replace: map[string]string{"https://example.org/umbrella.git": builtAt(t, "requires-across-projects.umbrella")}},
		builtAt(t, "requires-across-projects"), "main", "")
	if c := across.Requires("c3d7")[0]; c.Word() != "pending" || c.AtTrunk == Unknown {
		t.Errorf("requires-across-projects: the downward condition is %+v", c)
	}
	station, _ := view(t, "weather-station")
	if c := station.Requires("c07d")[0]; c.AtTrunk != Unknown {
		t.Errorf("weather-station: a local entry is met at a trunk's tip: %+v", c)
	}
}

// T18: the past, and the authorisation at a commit of the trunk's line.
func TestThePast(t *testing.T) {
	station, hashes := view(t, "weather-station")
	past := station.At(hashes["W10"])
	if past == nil || past.Refused() != nil || past.Log() != nil || past.Project().Where.Commit != hashes["W10"] {
		t.Fatalf("weather-station at W10 is %+v", past)
	}
	// c07d has no status before W11, so 9f31 alone has a severity.
	if s := past.Status("4e2b"); s.Gate != "function" || s.State != "stalled" || s.From != "9f31" || !reflect.DeepEqual(s.Considered, []string{"9f31"}) || s.Date != "" {
		t.Errorf("at W10 4e2b rolls up to %+v; want function, stalled from 9f31 alone, with no date", s)
	}
	if s := past.Status("c07d"); s.Kind != Absent || s.Commit != nil {
		t.Errorf("at W10 c07d is %+v; want no status and no commit", s)
	}
	// The firmware is no submodule yet: the junction's link leads nowhere then.
	if list := past.Snapshots("c07d"); len(list) != 1 || list[0].Why != NoLink || list[0].Link == nil {
		t.Errorf("at W10 c07d's snapshots are %+v; want the link in view and NoLink", list)
	}
	if a := past.Authorisation("9f31"); a == nil || a.Why != NoHistory || a.Authorised {
		t.Errorf("at W10 the authorisation is %+v; want no fact of the history", a)
	}
	if past.Events("9f31") != nil || past.History() != nil || past.Unread() != nil || len(past.Reviews("9f31")) != 1 || past.Reviews("9f31")[0].Commit != nil {
		t.Error("at W10 the past gives a fact of the history")
	}
	if station.At(hashes["W10"]) != past || past.At(hashes["W13"]) != station.At(hashes["W13"]) || station.At("none") != nil {
		t.Error("At makes the facts of one commit twice, or of a commit the pass lacks")
	}
	if got := station.At(hashes["W2"]).Order(); !reflect.DeepEqual(got, []string{"a1c0", "4e2b", "c07d", "7b2e"}) {
		t.Errorf("at W2 the order is %v", got)
	}

	// The past follows a submodule to the commit the gitlink names then.
	module, hashes := view(t, "submodule-subproject")
	sub := labels(t, "submodule-subproject.sub")
	then := module.At(hashes["U3"])
	s := then.Status("c100")
	if s.Kind != Snapshotted || s.Gate != "mockup" || s.Note != "Mockup shown to the client" || s.Of == nil ||
		s.Snapshot.Facts.Project().Where.Commit != sub["S2"] || s.Snapshot.Link != module.Project().Links[0] {
		t.Errorf("at U3 c100 is %+v; want the subproject read at S2", s)
	}
	if r := then.Status("c000"); r.Gate != "mockup" || r.From != "c100" {
		t.Errorf("at U3 the root rolls up to %+v", r)
	}
	if s := module.At(hashes["U1"]).Status("c100"); s.Kind != Absent {
		t.Errorf("at U1 c100 is %+v; want no status", s)
	}

	// The past follows an absolute URL to the commit the file states then.
	replace := map[string]string{"https://example.org/lib.git": builtAt(t, "subproject-by-url.lib")}
	url := derived(t, load.Options{Replace: replace}, builtAt(t, "subproject-by-url"), "main", "")
	hashes, lib := labels(t, "subproject-by-url"), labels(t, "subproject-by-url.lib")
	if s := url.At(hashes["U1"]).Status("b2c9"); s.Of == nil || s.Snapshot.Facts.Project().Where.Commit != lib["L1"] || s.State != "nominal" || s.Note != "" {
		t.Errorf("at U1 b2c9 is %+v; want the library read at L1", s)
	}
	// And a directory of the same repository at the same commit.
	same, hashes := view(t, "subproject-same-repository")
	if s := same.At(hashes["only"]).Status("b2c9"); s.Of == nil || s.Note != "Shares the parent's commit" {
		t.Errorf("at its one commit b2c9 is %+v; want the directory's task", s)
	}
}

// T18, the cases: the authorisation as it stands at a commit of the line.
func TestAuthorisationAt(t *testing.T) {
	repo, hashes, _ := cases(t)
	f := derived(t, load.Options{}, repo, "main", "")
	if a := f.AuthorisationAt("c3d7", hashes["C5"]); a.Authorised || a.Commit.ID != hashes["C5"] || a.Way != ByTrailer ||
		a.By != "pat@example.org" || a.Author || a.Committer || !reflect.DeepEqual(a.Judges, []string{"olive@example.org"}) {
		t.Errorf("at C5 c3d7 is %+v; want it proposed by pat's trailer", a)
	}
	if a := f.AuthorisationAt("c3d7", hashes["C4"]); !a.Authorised || a.Commit.ID != hashes["C3"] || a.Way != ByMerge {
		t.Errorf("at C4 c3d7 is %+v; want it authorised by the merge C3", a)
	}
	if a := f.AuthorisationAt("c3d7", hashes["C1"]); a.Authorised || a.Why != NoCommit || a.Commit != nil {
		t.Errorf("at C1 c3d7 is %+v; want no deciding commit", a)
	}
	if a := f.AuthorisationAt("c3d7", hashes["C2"]); a.Why != OffTrunk {
		t.Errorf("at C2, off the line, c3d7 is %+v", a)
	}
	if a := f.AuthorisationAt("c3d7", "none"); a.Why != NoHistory {
		t.Errorf("at a commit the pass lacks c3d7 is %+v", a)
	}
	// In view the hand-over stands: the outgoing owner accepts it (decision 3).
	if a := f.Authorisation("e4a1"); !a.Authorised || a.Commit.ID != hashes["C6"] || a.By != "olive@example.org" || a.Differs || a.Uncommitted {
		t.Errorf("the root is %+v; want the hand-over C6 accepted by olive", a)
	}
	if f.Authorisation("zzzz") != nil {
		t.Error("a task the tree lacks has an authorisation")
	}
	// On the side branch every task is proposed, and b2c9's file is the trunk's.
	side := derived(t, load.Options{}, repo, "side", "")
	if a := side.Authorisation("b2c9"); a.Why != OffTrunk || a.Differs || side.OnTrunk() {
		t.Errorf("on the side branch b2c9 is %+v", a)
	}
	// On the proposal branch c3d7 is at the trunk's tip as proposed there.
	early := derived(t, load.Options{}, repo, hashes["C8"], "")
	if a := early.Authorisation("b2c9"); !a.Differs || !a.Authorised {
		t.Errorf("at C8 b2c9 is %+v; want a file that differs from the trunk's tip", a)
	}
}

// T19: the working tree. The files come from disk and the history from HEAD.
func TestWorkingTree(t *testing.T) {
	repo := clone(t, "submodule-subproject")
	hashes, sub := labels(t, "submodule-subproject"), labels(t, "submodule-subproject.sub")
	clean := derived(t, load.Options{}, repo, "", "")
	if s := clean.Status("c100"); s.Uncommitted || s.Own.ID != hashes["U4"] || clean.Authorisation("c100").Uncommitted || clean.Pin(clean.Project().Links[0]).Moved {
		t.Fatalf("the clean working tree reads as %+v", s)
	}

	// A comment changes nothing; a new gate does, and the history stays HEAD's.
	status := filepath.Join(repo, ".tableaux", "status", "c100.yaml")
	if err := os.WriteFile(status, []byte("# a comment\ngate:   function\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s := derived(t, load.Options{}, repo, "", "").Status("c100"); s.Uncommitted {
		t.Errorf("a comment makes the status uncommitted: %+v", s)
	}
	write(t, repo, map[string]string{
		".tableaux/status/c100.yaml": "gate: mockup\n",
		".tableaux/tasks/d200.yaml":  leafOf("ben@example.org", "c000", 2, ""),
		".tableaux/tasks/c000.yaml":  "title: Edited\nassignee: olive@example.org\n",
	})
	gitRun(t, filepath.Join(repo, "sub"), "checkout", "-q", sub["S2"])
	f := derived(t, load.Options{}, repo, "", "")
	if s := f.Status("c100"); !s.Uncommitted || s.Gate != "mockup" || s.Own == nil || s.Own.ID != hashes["U4"] {
		t.Errorf("the edited status is %+v; want it uncommitted, with the deciding commit of the history", s)
	}
	if a := f.Authorisation("d200"); a.Authorised || a.Why != NoCommit || !a.Uncommitted || a.Commit != nil || !a.Differs {
		t.Errorf("the untracked task is %+v; want it proposed with no commit", a)
	}
	if s := f.Status("d200"); s.Kind != Absent || s.Commit != nil || s.Date != "" || s.Uncommitted {
		t.Errorf("the untracked task's status is %+v; want no commit and no date", s)
	}
	if a := f.Authorisation("c000"); !a.Uncommitted || !a.Authorised || a.Commit.ID != hashes["U1"] || !a.Differs {
		t.Errorf("the edited root is %+v; want it uncommitted, decided by U1", a)
	}
	if a := f.Authorisation("c100"); a.Uncommitted || a.Differs {
		t.Errorf("the untouched task is %+v", a)
	}
	want := Pin{Link: f.Project().Links[0], Commit: sub["S2"], Recorded: sub["S3"], Moved: true, Tip: sub["S4"], OnTrunk: Yes, Behind: Yes}
	if pin := f.Pin(f.Project().Links[0]); *pin != want {
		t.Errorf("the moved checkout's pin is %+v; want %+v", pin, want)
	}
	// A status file removed from disk: undefined in view, and not HEAD's.
	if err := os.Remove(status); err != nil {
		t.Fatal(err)
	}
	if s := derived(t, load.Options{}, repo, "", "").Status("c100"); s.Kind != Absent || !s.Uncommitted {
		t.Errorf("the removed status is %+v; want it absent and uncommitted", s)
	}
}

// T20: no history. A tree and an unborn branch have no commit: the facts of
// the files stand, and every fact of the history is undetermined.
func TestNoHistory(t *testing.T) {
	repo, _, _ := cases(t)
	tree := gitRun(t, repo, "rev-parse", "HEAD^{tree}")
	unborn := filepath.Join(t.TempDir(), "unborn")
	copyTree(t, filepath.Join(repo, ".tableaux"), filepath.Join(unborn, ".tableaux"))
	gitRun(t, unborn, "init", "-q", "-b", "main")
	for name, f := range map[string]*Facts{
		"a tree":           derived(t, load.Options{}, repo, tree, "main"),
		"an unborn branch": derived(t, load.Options{}, unborn, "", "main"),
	} {
		if f.Refused() != nil || f.Log() != nil || f.OnTrunk() || f.Trunk().How() != "undetermined" {
			t.Fatalf("%s: refused %+v with the pass %v", name, f.Refused(), f.Log())
		}
		// The files stand: the junction, the status, the roll-up.
		if j := f.Junction("b2c9", "design"); j == nil || j.Reviewer.V != "dan@example.org" {
			t.Errorf("%s: the junction is %+v", name, j)
		}
		if s := f.Status("b2c9"); s.Gate != "design" || s.Note != "From both" || s.Commit != nil || s.Date != "" || s.Recorder != "" || s.Uncommitted {
			t.Errorf("%s: the status is %+v; want the file's with no date", name, s)
		}
		if s := f.Status("e4a1"); s.Gate != "design" || s.From != "b2c9" || s.Date != "" {
			t.Errorf("%s: the roll-up is %+v", name, s)
		}
		for _, id := range f.Order() {
			if a := f.Authorisation(id); a.Authorised || a.Why != NoHistory || a.Commit != nil || a.Differs || a.Uncommitted {
				t.Errorf("%s: %s is %+v; want NoHistory", name, id, a)
			}
			if f.Events(id) != nil {
				t.Errorf("%s: %s has events", name, id)
			}
		}
		if a := f.AuthorisationAt("b2c9", "none"); a.Why != NoHistory {
			t.Errorf("%s: AuthorisationAt gives %+v", name, a)
		}
		if list := f.Reviews("b2c9"); len(list) != 1 || list[0].Commit != nil {
			t.Errorf("%s: the reviews are %+v; want design with no commit", name, list)
		}
		if f.History() != nil || f.Unread() != nil || f.At("none") != nil || f.Handoff("b2c9") != nil {
			t.Errorf("%s: a fact of the history stands", name)
		}
		exercise(f, 0)
	}
}
