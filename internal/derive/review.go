package derive

import (
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
)

// defined is the gate whose junction the authorisation reviews.
const defined = "defined"

// Review is one reviewed junction and the commit that accepts it.
type Review struct {
	Task, Gate      string
	Reviewer        string          // the junction's reviewer in view
	Commit          *history.Commit // nil while nothing accepts
	By              string          // the reviewer's email on that commit
	ByAuthorisation bool            // at defined: the authorisation stands as the review
}

// trailerReview is one commit that carries Reviewed: for a task and a gate,
// and what the trailer does.
type trailerReview struct {
	commit *history.Commit
	effect Effect
	by     string // Accepts and Repeats: the reviewer's email on the commit
}

// Reviews returns the reviewed junctions the task's status passes, in gate
// order: each junction with a reviewer at an applicable gate up to the gate
// a leaf's file states, with the commit that accepts it or none. A review
// with no commit at a gate other than defined is what rule S11 rejects.
func (f *Facts) Reviews(id string) []*Review {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	file := f.p.Statuses[id]
	if f.task(id) == nil || len(f.children[id]) > 0 || file == nil {
		return nil
	}
	stands, named := f.index[file.Gate.V]
	if !named {
		return nil
	}
	var list []*Review
	for _, gate := range f.gates[1 : stands+1] {
		if r := f.accepted(id, gate); r != nil {
			list = append(list, r)
		}
	}
	return list
}

// Accepted returns the review of one junction, passed or not, or nil when
// the junction has no reviewer.
func (f *Facts) Accepted(id, gate string) *Review {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	return f.accepted(id, gate)
}

// accepted finds the commit that accepts a junction with a reviewer: the
// oldest one the source reaches that carries Reviewed: for it and whose
// author or committer is the junction's reviewer as the files stand at that
// commit (decision 2). At defined the authorisation stands as the review:
// the deciding commit accepts when the task is authorised, and no trailer
// counts.
func (f *Facts) accepted(id, gate string) *Review {
	j := f.junction(id, gate)
	if j == nil || j.Kind != model.Plain || j.Reviewer.V == "" {
		return nil
	}
	r := &Review{Task: id, Gate: gate, Reviewer: j.Reviewer.V, ByAuthorisation: gate == defined}
	if r.ByAuthorisation {
		if a := f.authorisation(id); a != nil && a.Authorised {
			r.Commit, r.By = a.Commit, a.By
		}
		return r
	}
	for _, t := range f.reviews(id, gate) {
		if t.effect == Accepts {
			r.Commit, r.By = t.commit, t.by
		}
	}
	return r
}

// reviews returns the commits the source reaches that carry Reviewed: for a
// task and a gate, the oldest first, each with its effect. The reviewer a
// commit must be is the junction's as the files stand at that commit.
func (f *Facts) reviews(id, gate string) []trailerReview {
	key := id + " " + gate
	if list, ok := f.reviewed[key]; ok {
		return list
	}
	commits := f.indexed().trailers["Reviewed\x00"+key]
	list := make([]trailerReview, 0, len(commits))
	accepted := false
	for i := len(commits) - 1; i >= 0; i-- {
		c := commits[i]
		t := trailerReview{commit: c}
		reviewer := ""
		if j := f.past(c.ID).junction(id, gate); j != nil && j.Kind == model.Plain {
			reviewer = j.Reviewer.V
		}
		switch {
		case gate == defined:
			t.effect = AtDefined
		case reviewer == "":
			t.effect = NoReviewer
		case c.Author.Email != reviewer && c.Committer.Email != reviewer:
			t.effect = WrongHand
		default:
			t.effect, t.by = Accepts, reviewer
			if accepted {
				t.effect = Repeats
			}
			accepted = true
		}
		list = append(list, t)
	}
	f.reviewed[key] = list
	return list
}

// HandoffKind is the form of a hand-off.
type HandoffKind int

// The forms.
const (
	NoHandoff HandoffKind = iota
	Stated                // the status states the reason review
	Implied               // the history implies it and the status does not state it
	Stale                 // the status states it after the reviewer's Reviewed: commit
)

// String returns "stated", "implied" or "stale", or "none".
func (k HandoffKind) String() string {
	switch k {
	case Stated:
		return "stated"
	case Implied:
		return "implied"
	case Stale:
		return "stale"
	}
	return "none"
}

// Handoff is the hand-off of a leaf's next junction to its reviewer.
type Handoff struct {
	Task, Gate string
	Kind       HandoffKind
	Reviewer   string
	Self       bool            // the reviewer is the junction's contributor
	Commit     *history.Commit // Implied: the newest event's; Stale: the accepting commit
}

// Handoff reads the next junction of a leaf, or returns nil when it is no
// plain junction with a reviewer. The hand-off is Stated when the status
// states the reason review; Stale when it does so and the reviewer's commit
// already accepts the junction; and Implied when the status states no
// review, nothing accepts yet, and the task's newest own event is by the
// junction's contributor (decision 7). A Reviewed: trailer from the wrong
// hand has no effect and is no such event.
func (f *Facts) Handoff(id string) *Handoff {
	if !f.enter() {
		return nil
	}
	defer f.leave()
	if f.task(id) == nil || len(f.children[id]) > 0 {
		return nil
	}
	gate := f.next(id, f.status(id).Gate)
	j := f.junction(id, gate)
	if j == nil || j.Kind != model.Plain || j.Reviewer.V == "" {
		return nil
	}
	h := &Handoff{Task: id, Gate: gate, Reviewer: j.Reviewer.V, Self: j.Reviewer.V == j.Contributor.V}
	review := f.accepted(id, gate)
	if file := f.p.Statuses[id]; file != nil && file.Reason.V == "review" {
		h.Kind = Stated
		if review.Commit != nil {
			h.Kind, h.Commit = Stale, review.Commit
		}
		return h
	}
	if review.Commit != nil {
		return h
	}
	var newest *history.Commit
	for _, e := range f.ownEvents()[id] {
		if e.Effect != WrongHand && (newest == nil || e.Commit.Seq < newest.Seq) {
			newest = e.Commit
		}
	}
	if newest != nil && newest.Author.Email == j.Contributor.V {
		h.Kind, h.Commit = Implied, newest
	}
	return h
}
