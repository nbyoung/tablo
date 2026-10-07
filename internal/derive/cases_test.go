package derive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/load"
)

// label returns the label of a commit, or its id.
func label(names map[string]string, c *history.Commit) string {
	if c == nil {
		return "none"
	}
	if name, ok := names[c.ID]; ok {
		return name
	}
	return c.ID
}

// describeStatus writes a status as cases.expected.txt does: the gate, the
// state, the reason and the note, then the date, the recorder and the commit.
func describeStatus(b *strings.Builder, names map[string]string, s *Status) {
	b.WriteString("  " + s.Task + " status " + s.Gate + " " + s.State)
	if s.Reason != "" {
		b.WriteString(" " + s.Reason)
	}
	if s.Note != "" {
		b.WriteString(` "` + s.Note + `"`)
	}
	b.WriteString(", " + s.Date + " " + s.Recorder + " at " + label(names, s.Commit) + "\n")
}

// describeEvent writes an event as cases.expected.txt does: the date, the
// commit, the author, the task, the kind and what it records; then the
// junction a status or reaffirmed commit is at, or the effect of a review.
func describeEvent(b *strings.Builder, names map[string]string, e *Event) {
	b.WriteString("  event " + e.Commit.Date() + " " + label(names, e.Commit) + " " + e.Commit.Author.Email + " " + e.Task + " " + string(e.Kind))
	for _, word := range []string{e.Gate, e.State, e.Reason} {
		if word != "" {
			b.WriteString(" " + word)
		}
	}
	if e.Note != "" {
		b.WriteString(` "` + e.Note + `"`)
	}
	switch {
	case e.Reading != nil:
		b.WriteString("; at " + e.Reading.Gate)
	case e.Kind == Reviewed:
		b.WriteString("; " + e.Effect.String())
	}
	b.WriteString("\n")
}

// describe writes what the Derivation gives at one label of cases.sh, in
// the form of cases.expected.txt.
func describe(f *Facts, name string, names map[string]string, events bool) string {
	var b strings.Builder
	b.WriteString(name + ": ")
	if f.OnTrunk() {
		b.WriteString("on the trunk\n")
	} else {
		b.WriteString("off the trunk\n")
	}
	for _, id := range f.Order() {
		a := f.Authorisation(id)
		b.WriteString("  " + id + " ")
		if a.Commit == nil {
			b.WriteString("proposed, no deciding commit\n")
			continue
		}
		hands := "neither"
		switch {
		case a.Author && a.Committer:
			hands = "author, committer"
		case a.Author:
			hands = "author"
		case a.Committer:
			hands = "committer"
		}
		state := "proposed"
		if a.Authorised {
			state = "authorised"
		}
		b.WriteString(state + " at " + label(names, a.Commit) + " by " + a.By + " (" + hands + "), way " + a.Way.String() +
			", judges " + strings.Join(a.Judges, " ") + "\n")
	}
	describeStatus(&b, names, f.Status("b2c9"))
	for _, r := range f.Reviews("b2c9") {
		b.WriteString("  b2c9 review " + r.Gate + ", reviewer " + r.Reviewer + ": " + label(names, r.Commit))
		if r.Commit != nil {
			b.WriteString(" by " + r.By)
		}
		b.WriteString("\n")
	}
	if h := f.Handoff("b2c9"); h != nil && h.Kind != NoHandoff {
		b.WriteString("  b2c9 hand-off " + h.Kind.String() + " at " + h.Gate + " to " + h.Reviewer)
		if h.Commit != nil {
			b.WriteString(", " + label(names, h.Commit))
		}
		b.WriteString("\n")
	}
	if events {
		for _, id := range f.Order() {
			for _, e := range f.Events(id) {
				describeEvent(&b, names, e)
			}
		}
	}
	return b.String()
}

// T10, T11, T12 and T14, the cases: what the Derivation gives at each label
// of cases.sh is cases.expected.txt, line for line. The labels carry the
// merge that authorises with no trailer, the committer who is the authority,
// the trailer from no authority, the hand-over and the new owner; a merge
// that writes its own reading and a reaffirmation by another hand; the
// hand-off stated, stale, accepted and repeated, and the review that stands
// after the reviewer changes; and every event at C16.
func TestCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "cases.expected.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if !strings.HasPrefix(line, "#") {
			want = append(want, line)
		}
	}
	repo, hashes, order := cases(t)
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
		text := describe(f, name, names, name == order[len(order)-1])
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
	if len(order) != 17 {
		t.Errorf("cases.sh labels %d commits; want 17", len(order))
	}
}
