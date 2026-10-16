package views

import (
	"sort"

	"github.com/nbyoung/tablo/internal/audit"
	"github.com/nbyoung/tablo/internal/derive"
	"github.com/nbyoung/tablo/internal/history"
	"github.com/nbyoung/tablo/view"
)

// Audit returns the audit of the subtree under q.Task, the root when it is
// "", from r, the report of the project f holds: under the root every
// finding, under another task those whose task lies in its subtree; of them
// those whose resolver is q.Person when it names one. The counts, Most and
// Silent read every finding kept; the rows list those at or above
// q.Minimum. r is not nil.
func Audit(f *derive.Facts, r *audit.Report, q Query) *view.Audit {
	b := builder{f, q}
	root := q.Task == "" || q.Task == f.Root()
	in := map[string]bool{}
	if !root {
		for _, id := range b.subtree(q.Task) {
			in[id] = true
		}
	}
	kept := r.Keep(func(x *audit.Finding) bool {
		return (root || in[x.Task]) && (q.Person == "" || x.Resolver == q.Person)
	})
	c := kept.Counts()
	out := &view.Audit{Legend: b.legend(), Errors: c.Errors, Warnings: c.Warnings, Information: c.Information, Findings: []view.Finding{}}
	if most := kept.Most(); most != nil {
		out.Most = &view.Resolving{Email: most.Email, Count: most.Count}
	}
	for _, row := range kept.Rows(q.Minimum) {
		out.Findings = append(out.Findings, b.finding(row))
	}
	if q.Level >= Detail {
		out.AuditDetail = &view.AuditDetail{Silent: []view.Silence{}}
		for _, s := range kept.Silent() {
			out.Silent = append(out.Silent, view.Silence{Key: s.Key, Rule: s.Rule, Kind: s.Kind})
		}
	}
	return out
}

// finding returns one row of the audit, through the builders, as dada's
// model.md, "From a finding to the data", states.
func (b builder) finding(row audit.Row) view.Finding {
	x := row[0]
	out := view.Finding{
		Key: x.Key, Rule: x.Rule, Severity: x.Severity.String(), Demoted: x.Demoted, Kind: x.Kind,
		Tasks: []view.TaskRef{}, Gate: x.Gate, Act: string(x.Act), Resolver: x.Resolver,
	}
	for _, y := range row {
		if y.Task != "" {
			out.Tasks = append(out.Tasks, b.ref(y.Task))
		}
	}
	if b.q.Level < Detail {
		return out
	}
	d := &view.FindingDetail{Files: []string{}, Message: x.Message}
	seen := map[string]bool{}
	for _, y := range row {
		if y.File != "" && !seen[y.File] {
			seen[y.File] = true
			d.Files = append(d.Files, y.File)
		}
	}
	out.FindingDetail = d
	if b.q.Level < Provenance {
		return out
	}
	p := &view.FindingProvenance{
		Sentence: x.Sentence, Source: view.Reference{Text: x.Source.Text, URL: x.Source.URL},
		Facts: []view.FindingFacts{}, Commits: []view.Commit{}, Commands: []view.Command{},
	}
	var commits []*history.Commit
	held := map[string]bool{}
	for _, y := range row {
		if facts, ok := b.findingFacts(y); ok {
			p.Facts = append(p.Facts, facts)
		}
		for _, c := range y.Commits {
			if !held[c.ID] {
				held[c.ID] = true
				commits = append(commits, c)
			}
		}
	}
	sort.SliceStable(commits, func(i, j int) bool { return commits[i].Seq > commits[j].Seq })
	for _, c := range commits {
		p.Commits = append(p.Commits, *b.commit(c))
	}
	for _, c := range x.Commands {
		p.Commands = append(p.Commands, view.Command{Text: c.Text, Comment: c.Comment})
	}
	out.FindingProvenance = p
	return out
}

// findingFacts returns the typed facts of one finding, and false where it
// carries none.
func (b builder) findingFacts(x *audit.Finding) (view.FindingFacts, bool) {
	facts := x.Facts
	out := view.FindingFacts{Task: x.Task}
	carries := false
	if facts.Junction != nil {
		j := b.junction(facts.Junction, true)
		out.Junction, carries = &j, true
	}
	if facts.Status != nil {
		s := b.status(b.f, facts.Status)
		out.Status, carries = &s, true
	}
	if facts.Requirement != nil {
		r := b.requirement(facts.Requirement, false)
		out.Requirement, carries = &r, true
	}
	if facts.Authorisation != nil {
		a := b.authorisation(x.Task)
		out.Authorisation, carries = &a, true
	}
	if facts.Snapshot != nil {
		carries = true
		for _, l := range b.linkages(x.Task) {
			if has(l.Gates, x.Gate) {
				out.Linkage = &l
				break
			}
		}
	}
	if facts.Reading != nil && len(x.Commits) > 0 {
		out.Model, carries = &view.ModelCheck{
			Commit: x.Commits[len(x.Commits)-1].ID, Gate: facts.Reading.Gate, Stated: facts.Reading.Stated,
			Trailer: facts.Reading.Trailer, Reading: reading(facts.Reading.Verdict),
		}, true
	}
	return out, carries
}
