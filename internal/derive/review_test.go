package derive

import (
	"testing"

	"github.com/nbyoung/tablo/internal/load"
	"github.com/nbyoung/tablo/internal/model"
)

// Design dada, A9 and T12: the authorisation stands as the review of a plain
// junction at defined, whether or not it states a reviewer, and of no other
// junction; a parent's junction is plain and reads the same.
func TestByAuthorisation(t *testing.T) {
	f := mem(t, map[string]string{
		"tasks/e4a1.yaml": leafOf("olive@example.org", "", 0, ""),
		"tasks/b2c9.yaml": leafOf("pat@example.org", "e4a1", 1, "{ defined: { reviewer: olive@example.org }, design: { reviewer: olive@example.org } }"),
		"tasks/c3d7.yaml": leafOf("pat@example.org", "e4a1", 2, "{ mockup: { applies: false } }"),
		"tasks/d4e8.yaml": leafOf("pat@example.org", "e4a1", 3, "{ defined: { applies: false } }"),
	})
	cases := []struct {
		id, gate string
		want     bool
	}{
		{"b2c9", "defined", true},    // with a reviewer
		{"c3d7", "defined", true},    // a person's plain junction that states none
		{"e4a1", "defined", true},    // a parent's default
		{"b2c9", "design", false},    // a reviewed junction after defined
		{"c3d7", "design", false},    // a plain junction after defined
		{"c3d7", "mockup", false},    // a not-applicable junction
		{"d4e8", "defined", false},   // defined does not apply: no junction to review
		{"b2c9", "undefined", false}, // no junction at undefined
		{"b2c9", "nowhere", false},   // no such gate
		{"zzzz", "defined", false},   // no such task
	}
	for _, c := range cases {
		if got := f.ByAuthorisation(c.id, c.gate); got != c.want {
			t.Errorf("ByAuthorisation(%s, %s) is %v, want %v", c.id, c.gate, got, c.want)
		}
		if review := f.Accepted(c.id, c.gate); review != nil && review.ByAuthorisation != c.want {
			t.Errorf("Accepted(%s, %s).ByAuthorisation is %v beside %v", c.id, c.gate, review.ByAuthorisation, c.want)
		}
	}
	if f.Accepted("c3d7", "defined") != nil {
		t.Error("Accepted returns a review for a junction that states no reviewer")
	}
	var none *Facts
	if none.ByAuthorisation("b2c9", "defined") {
		t.Error("nil facts hold an authorisation")
	}
}

// Design dada, T12: on every junction of the corpus that has a reviewer,
// ByAuthorisation equals Accepted(id, gate).ByAuthorisation; on one that has
// none it holds at defined alone; and it holds for no junction that is not
// plain.
func TestByAuthorisationOnTheCorpus(t *testing.T) {
	reviewed, unreviewed, others := 0, 0, 0
	for _, name := range built(t) {
		f := derived(t, load.Options{}, builtAt(t, name), "HEAD", "")
		if f.Refused() != nil {
			continue
		}
		for _, id := range f.Order() {
			for _, j := range f.Junctions(id) {
				got := f.ByAuthorisation(id, j.Gate)
				review := f.Accepted(id, j.Gate)
				switch {
				case j.Kind != model.Plain:
					others++
					if got || review != nil {
						t.Errorf("%s: the %s junction of %s at %s reads %v, %v", name, j.Kind, id, j.Gate, got, review)
					}
				case review != nil:
					reviewed++
					if got != review.ByAuthorisation {
						t.Errorf("%s: ByAuthorisation(%s, %s) is %v and the review says %v", name, id, j.Gate, got, review.ByAuthorisation)
					}
				default:
					unreviewed++
					if got != (j.Gate == defined) {
						t.Errorf("%s: ByAuthorisation(%s, %s) is %v at a junction with no reviewer", name, id, j.Gate, got)
					}
				}
			}
		}
	}
	if reviewed < 20 || unreviewed < 500 || others < 20 {
		t.Errorf("the corpus gives %d reviewed, %d unreviewed and %d other junctions; it builds more", reviewed, unreviewed, others)
	}
	t.Logf("%d reviewed, %d unreviewed and %d other junctions agree", reviewed, unreviewed, others)
}
