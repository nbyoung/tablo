package conformance

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/nbyoung/tablo/internal/corpustest"
)

// The entry results.
const (
	passed  = "pass"
	failed  = "fail"
	known   = "known"
	skipped = "skipped"
)

// An entryResult is what one entry's subtest found.
type entryResult struct {
	name       string
	status     string // pass, fail, known or skipped
	reason     string // for an entry that does not pass: the first mismatch, or why it skipped
	facts      []fact
	unexpected []finding
	stated     []string // the rules the entry states, also when it skipped
	outside    int      // findings whose code RULES.md lacks
}

// plural writes "1 mismatch" or "2 mismatches".
func plural(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// A mismatch is one item of a failure: the path, what the file wants and what the command gives.
type mismatch struct {
	path, want, got, call string
	line                  int // in expected.yaml; 0 for an unexpected finding
}

// mismatchesOf lists the mismatches of an outcome: the facts that differ in the
// order of expected.yaml's lines, then the unexpected findings sorted by rule,
// file and task.
func mismatchesOf(o *outcome) []mismatch {
	var out []mismatch
	for _, f := range o.facts {
		if f.result == differed {
			out = append(out, mismatch{path: f.path, want: f.want, got: f.got, call: f.call, line: f.line})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].line < out[j].line })
	for _, u := range o.unexpected {
		out = append(out, mismatch{path: "findings", want: "(no such finding)", got: u.text, call: "validate"})
	}
	return out
}

// failureText writes how a failing entry reads. lines maps a call to its full
// command line; when keep is set each got line shows it.
func failureText(name, ref string, o *outcome, keep bool, lines map[string]string) string {
	ms := mismatchesOf(o)
	var b strings.Builder
	fmt.Fprintf(&b, "%s at %s: %s in %s\n", name, ref, plural(len(ms), "mismatch", "mismatches"), plural(len(o.facts), "fact", "facts"))
	for _, m := range ms {
		fmt.Fprintf(&b, "    %s\n", m.path)
		if m.line > 0 {
			fmt.Fprintf(&b, "        want  %s  [expected.yaml:%d]\n", m.want, m.line)
		} else {
			fmt.Fprintf(&b, "        want  %s\n", m.want)
		}
		call := m.call
		if keep && lines[call] != "" {
			call = lines[call]
		}
		fmt.Fprintf(&b, "        got   %s  [%s]\n", m.got, call)
	}
	fmt.Fprintf(&b, "    rerun: TABLO_KEEP=1 go test ./internal/conformance -run 'TestCorpus/%s$'\n", name)
	return b.String()
}

// firstMismatch is the reason an entry fails: the first mismatch, and how many follow.
func firstMismatch(o *outcome) string {
	ms := mismatchesOf(o)
	if len(ms) == 0 {
		return ""
	}
	m := ms[0]
	s := fmt.Sprintf("%s: want %s, got %s", m.path, m.want, m.got)
	if len(ms) > 1 {
		s += fmt.Sprintf("; and %d more", len(ms)-1)
	}
	return s
}

// A table writes a Markdown table with every column as wide as its widest cell.
func table(header []string, rows [][]string) string {
	widths := make([]int, len(header))
	for i, h := range header {
		widths[i] = utf8.RuneCountInString(h)
	}
	for _, row := range rows {
		for i, c := range row {
			widths[i] = max(widths[i], utf8.RuneCountInString(c))
		}
	}
	line := func(cells []string) string {
		var b strings.Builder
		b.WriteString("|")
		for i, c := range cells {
			b.WriteString(" " + c + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(c)) + " |")
		}
		return b.String() + "\n"
	}
	var b strings.Builder
	b.WriteString(line(header))
	b.WriteString("|")
	for _, w := range widths {
		b.WriteString(strings.Repeat("-", w+2) + "|")
	}
	b.WriteString("\n")
	for _, row := range rows {
		b.WriteString(line(row))
	}
	return b.String()
}

// factKinds is the order the report counts facts in; a kind it lacks follows, sorted.
var factKinds = []string{"valid", "findings", "project", "parent", "authorities", "authorisation", "junctions", "applicable",
	"requires", "reviews", "subproject", "status", "events", "refs"}

// A ruleRow is one rule's line of the report.
type ruleRow struct {
	rule                                 corpustest.Rule
	entries                              []string
	stated, matched, missing, unexpected int
	result                               string
}

// report writes the coverage report of a run: per rule, the findings stated,
// matched, missing and unexpected over the entries that ran; per fact kind, the
// facts stated, agreeing, differing and undecided; and the entries that do not
// pass. results are in any order; the report sorts them.
func report(results []entryResult, rules []corpustest.Rule, version, language, corpus string) string {
	sort.Slice(results, func(i, j int) bool { return results[i].name < results[j].name })
	counts := map[string]int{}
	for _, r := range results {
		counts[r.status]++
	}
	var b strings.Builder
	b.WriteString("# Conformance report\n\n")
	fmt.Fprintf(&b, "tablo %s reads tableaux %s. Corpus: %s.\n\n", version, language, corpus)
	fmt.Fprintf(&b, "%s: %d pass, %d fail, %d known, %d skipped.\n\n",
		plural(len(results), "entry", "entries"), counts[passed], counts[failed], counts[known], counts[skipped])

	// Rules.
	rows := map[string]*ruleRow{}
	var order []*ruleRow
	for _, r := range rules {
		row := &ruleRow{rule: r}
		rows[r.ID] = row
		order = append(order, row)
	}
	outside := 0
	for _, r := range results {
		outside += r.outside
		for _, id := range r.stated {
			if row := rows[id]; row != nil && !contains(row.entries, r.name) {
				row.entries = append(row.entries, r.name)
			}
		}
		if r.status == skipped {
			continue
		}
		for _, f := range r.facts {
			if f.rule == "" {
				continue
			}
			if row := rows[f.rule]; row != nil {
				row.stated++
				if f.result == agreed {
					row.matched++
				}
			}
		}
		for _, u := range r.unexpected {
			if row := rows[u.rule]; row != nil {
				row.unexpected++
			}
		}
	}
	tally := map[string]int{}
	var ruleCells [][]string
	for _, row := range order {
		row.missing = row.stated - row.matched
		switch {
		case row.missing > 0 || row.unexpected > 0:
			row.result = "failing"
		case len(row.entries) == 0:
			row.result = "no entry"
		case row.stated == 0:
			row.result = "not run"
		default:
			row.result = "confirmed"
		}
		tally[row.result]++
		names := make([]string, len(row.entries))
		for i, n := range row.entries {
			names[i] = "`" + n + "`"
		}
		ruleCells = append(ruleCells, []string{row.rule.ID, row.rule.Severity, strconv.Itoa(row.stated), strconv.Itoa(row.matched),
			strconv.Itoa(row.missing), strconv.Itoa(row.unexpected), row.result, strings.Join(names, ", ")})
	}
	b.WriteString("## Rules\n\n")
	fmt.Fprintf(&b, "%s: %d confirmed, %d failing, %d not run, %d without an entry.\n\n",
		plural(len(order), "rule", "rules"), tally["confirmed"], tally["failing"], tally["not run"], tally["no entry"])
	b.WriteString(table([]string{"Rule", "Severity", "Stated", "Matched", "Missing", "Unexpected", "Result", "Entries"}, ruleCells))
	fmt.Fprintf(&b, "\nFindings outside the rules: %d.\n\n", outside)

	// Facts.
	type tallyRow struct{ stated, agree, differ, undecided int }
	kinds := map[string]*tallyRow{}
	for _, k := range factKinds {
		kinds[k] = &tallyRow{}
	}
	for _, r := range results {
		for _, f := range r.facts {
			row := kinds[f.kind]
			if row == nil {
				row = &tallyRow{}
				kinds[f.kind] = row
			}
			row.stated++
			switch f.result {
			case agreed:
				row.agree++
			case differed:
				row.differ++
			case undecided:
				row.undecided++
			}
		}
	}
	names := append([]string(nil), factKinds...)
	var extra []string
	for k := range kinds {
		if !contains(factKinds, k) {
			extra = append(extra, k)
		}
	}
	sort.Strings(extra)
	names = append(names, extra...)
	var total tallyRow
	var factCells [][]string
	for _, k := range names {
		row := kinds[k]
		total.stated += row.stated
		total.agree += row.agree
		total.differ += row.differ
		total.undecided += row.undecided
		factCells = append(factCells, []string{k, strconv.Itoa(row.stated), strconv.Itoa(row.agree), strconv.Itoa(row.differ), strconv.Itoa(row.undecided)})
	}
	factCells = append(factCells, []string{"total", strconv.Itoa(total.stated), strconv.Itoa(total.agree), strconv.Itoa(total.differ), strconv.Itoa(total.undecided)})
	b.WriteString("## Facts\n\n")
	b.WriteString(table([]string{"Fact", "Stated", "Agree", "Differ", "Undecided"}, factCells))

	// The entries that do not pass.
	var rest [][]string
	for _, r := range results {
		if r.status != passed {
			rest = append(rest, []string{"`" + r.name + "`", r.status, r.reason})
		}
	}
	if len(rest) > 0 {
		b.WriteString("\n## Entries that do not pass\n\n")
		b.WriteString(table([]string{"Entry", "Result", "Reason"}, rest))
	}
	return b.String()
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// summary is the report's summary lines, which the test log shows on every run.
func summary(report string) string {
	var out []string
	for _, line := range strings.Split(report, "\n") {
		if strings.Contains(line, " entries: ") || strings.Contains(line, " rules: ") || strings.HasPrefix(line, "Findings outside") ||
			strings.HasPrefix(line, "| total ") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}
