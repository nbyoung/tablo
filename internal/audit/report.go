package audit

import (
	"github.com/nbyoung/tablo/internal/model"
	"github.com/nbyoung/tablo/internal/validate"
)

// Row is the findings that agree in key, listed severity, gate, message, act
// and resolver, each of another task, in the order Run raises them. It is
// never empty.
type Row []*Finding

// Counts are the findings by listed severity.
type Counts struct{ Errors, Warnings, Information int }

// Resolving is the number of findings that ask an act of one person.
type Resolving struct {
	Email string
	Count int
}

// Silence is a kind the audit examines and finds nothing of.
type Silence struct{ Key, Rule, Kind string }

// Keep returns the report with the findings keep accepts, in their order. Now,
// Stale and the kinds examined stay.
func (r *Report) Keep(keep func(*Finding) bool) *Report {
	kept := &Report{Now: r.Now, Stale: r.Stale, history: r.history, trunk: r.trunk}
	for i := range r.Findings {
		if keep(&r.Findings[i]) {
			kept.Findings = append(kept.Findings, r.Findings[i])
		}
	}
	return kept
}

// joint is what the findings of one row agree in.
type joint struct {
	key      string
	severity model.Severity
	gate     string
	message  string
	act      Act
	resolver string
}

// Rows returns the findings whose listed severity is minimum or graver, with
// those that one kind hits at one gate for one reason joined in one row, in
// the order of the first finding of each. A finding with a task joins the
// first such row that does not hold its task, and a finding with no task
// never joins.
func (r *Report) Rows(minimum model.Severity) []Row {
	var rows []Row
	open := map[joint][]int{} // the rows that agree, by their place in rows
	held := map[int]map[string]bool{}
	for i := range r.Findings {
		x := &r.Findings[i]
		if x.Severity > minimum {
			continue
		}
		if x.Task == "" {
			rows = append(rows, Row{x})
			continue
		}
		key := joint{x.Key, x.Severity, x.Gate, x.Message, x.Act, x.Resolver}
		at := -1
		for _, row := range open[key] {
			if !held[row][x.Task] {
				at = row
				break
			}
		}
		if at < 0 {
			at = len(rows)
			rows = append(rows, nil)
			open[key] = append(open[key], at)
			held[at] = map[string]bool{}
		}
		rows[at] = append(rows[at], x)
		held[at][x.Task] = true
	}
	return rows
}

// Counts returns the findings by listed severity, one per finding of
// Findings, whatever a caller's minimum.
func (r *Report) Counts() Counts {
	var c Counts
	for i := range r.Findings {
		switch r.Findings[i].Severity {
		case model.Error:
			c.Errors++
		case model.Warning:
			c.Warnings++
		default:
			c.Information++
		}
	}
	return c
}

// Most returns the person the most findings ask an act of, the first in the
// findings' order among equals, or nil when none does. A finding whose act
// is None asks nothing and counts for nobody.
func (r *Report) Most() *Resolving {
	counts := map[string]int{}
	var most *Resolving
	for i := range r.Findings {
		x := &r.Findings[i]
		if x.Act != None && x.Resolver != "" {
			counts[x.Resolver]++
		}
	}
	for i := range r.Findings {
		x := &r.Findings[i]
		if n := counts[x.Resolver]; x.Act != None && n > 0 && (most == nil || n > most.Count) {
			most = &Resolving{Email: x.Resolver, Count: n}
		}
	}
	return most
}

// Silent returns the kinds that the run examines and that no finding of
// Findings has, in the order of the design: the rules the audit view names,
// then the proposed tasks, the stale statuses and the sole reviews. A run
// examines a rule of the history only with a history, and the proposed
// tasks only with a trunk.
func (r *Report) Silent() []Silence {
	has := map[string]bool{}
	for i := range r.Findings {
		has[r.Findings[i].Key] = true
	}
	var list []Silence
	for _, id := range silent {
		known, _ := validate.RuleOf(id)
		if has[id] || (known.Tier == validate.TierHistory && !r.history) {
			continue
		}
		list = append(list, Silence{Key: id, Rule: id, Kind: known.Title})
	}
	for _, key := range []string{Proposed, Stale, SoleReview} {
		if has[key] || (key == Proposed && !r.trunk) {
			continue
		}
		list = append(list, Silence{Key: key, Kind: kinds[key].kind})
	}
	return list
}
