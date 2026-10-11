package audit

import (
	"path"
	"sort"

	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/internal/model"
)

// The comments of a finding's two commands.
const (
	shows    = "shows the finding"
	resolves = "resolves it"
)

// attach gives a finding its typed facts, its commits and its commands. The
// facts are the Derivation's own values, each from the method that every
// view reads, so a front end renders a fact of a finding as it renders that
// fact anywhere else.
func (r *run) attach(x *Finding) {
	var commits []*history.Commit
	switch x.Key {
	case Proposed:
		x.Facts.Authorisation = r.f.Authorisation(x.Task)
		if a := x.Facts.Authorisation; a != nil {
			commits = append(commits, a.Commit)
		}
	case Stale:
		x.Facts.Status = r.f.Status(x.Task)
		if s := x.Facts.Status; s != nil {
			commits = append(commits, s.Commit)
		}
	case SoleReview:
		x.Facts.Junction = r.f.Junction(x.Task, x.Gate)
	default:
		commits = r.ruleFacts(x)
	}
	x.Commits = oldestFirst(commits)
	x.Commands = r.commands(x)
}

// ruleFacts gives a rule finding the facts its rule carries and returns the
// commits involved: the commit its diagnostic names, the deciding commit of
// the status it shows, the commit that accepts the junction of an H2 nothing
// resolves, and the commit of the hand-off of H4 and H5.
func (r *run) ruleFacts(x *Finding) []*history.Commit {
	d := x.Diagnostic
	commits := []*history.Commit{r.commit(d.Commit)}
	if r.task(x.Task) == nil {
		return commits
	}
	id, gate := x.Task, x.Gate
	if gate != "" {
		x.Facts.Junction = r.f.Junction(id, gate)
	}
	// The status: of a finding in the leaf's status file, and of the three
	// rules that judge a status against the history, wherever they stand.
	if s := r.f.Status(id); s != nil && r.f.Leaf(id) {
		inFile := d.Pos.File == "status/"+id+".yaml" && (s.Kind == derive.Recorded || s.Kind == derive.Snapshotted)
		if inFile || d.Code == "S11" || d.Code == "H4" || d.Code == "H5" {
			x.Facts.Status = s
			commits = append(commits, s.Commit)
			if s.Kind == derive.Snapshotted {
				commits[len(commits)-1] = s.Own
			}
		}
	}
	switch d.Code {
	case "R9", "R12", "R13":
		x.Facts.Requirement = r.condition(d)
	case "J8", "J9", "J13", "J14", "J15", "J16", "J17":
		if j := x.Facts.Junction; j != nil && j.Kind == model.Recursive {
			x.Facts.Snapshot = j.Snapshot
		}
	case "H2":
		if review := r.f.Accepted(id, gate); x.Act == None && review != nil {
			commits = append(commits, review.Commit)
		}
	case "H3", "H6":
		for _, e := range r.f.Events(id) {
			if e.Sub == "" && e.Reading != nil && e.Commit != nil && e.Commit.ID == d.Commit && e.Reading.Gate == gate {
				x.Facts.Reading = e.Reading
				break
			}
		}
	case "H4", "H5":
		if h := r.f.Handoff(id); h != nil {
			commits = append(commits, h.Commit)
		}
	}
	return commits
}

// oldestFirst returns the commits given, each once and none nil, the oldest
// first: a pass counts its commits from the newest.
func oldestFirst(commits []*history.Commit) []*history.Commit {
	var list []*history.Commit
	seen := map[*history.Commit]bool{}
	for _, c := range commits {
		if c != nil && !seen[c] {
			seen[c] = true
			list = append(list, c)
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return list[i].Seq > list[j].Seq })
	return list
}

// commands returns the line that shows a finding, its newest commit or else
// the last change of its file, and then, for four acts, the line that
// resolves it.
func (r *run) commands(x *Finding) []Command {
	var list []Command
	switch {
	case len(x.Commits) > 0:
		list = append(list, Command{"git show --no-patch --format='%as %H %ae %ce%n%(trailers)' " + x.Commits[len(x.Commits)-1].ID, shows})
	case x.File != "":
		list = append(list, Command{"git log -1 --format='%as %H %ae' -- " + path.Join(r.f.Project().Where.Dir, x.File), shows})
	}
	trailer := func(value string) {
		list = append(list, Command{"git commit --allow-empty --trailer '" + value + "'", resolves})
	}
	switch x.Act {
	case Authorise:
		if a := x.Facts.Authorisation; a != nil && a.Why == derive.Determined {
			trailer("Authorised: " + x.Task)
		}
	case Review:
		if x.Task != "" && x.Gate != "" {
			trailer("Reviewed: " + x.Task + " " + x.Gate)
		}
	case Reaffirm:
		trailer("Reaffirmed: " + x.Task)
	case Checkout:
		if link := r.link(x.Diagnostic); link != nil && link.Form == model.Submodule {
			list = append(list, Command{"git submodule update --init " + link.URL, resolves})
		}
	}
	return list
}
