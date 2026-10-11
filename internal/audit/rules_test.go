package audit

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/validate"
)

// method is where the method's texts stand.
const method = "https://github.com/nbyoung/tableaux/blob/main/"

// T1: the table is total. It holds a row for every rule of validate.Rules
// and no other; every act is one of the ten and none is advance; every role
// is one the design defines; every source address starts with the method's.
func TestTableIsTotal(t *testing.T) {
	acts := map[Act]bool{Authorise: true, Review: true, Reaffirm: true, RecordHandoff: true, ClearHandoff: true,
		Revise: true, MovePin: true, Checkout: true, Await: true, None: true}
	if len(acts) != 10 || acts["advance"] {
		t.Fatalf("the acts are %v", acts)
	}
	roles := map[role]bool{owner: true, authority: true, assignee: true, keeper: true, recorder: true,
		contributor: true, reviewer: true, author: true, origin: true, nobody: true}
	if len(validate.Rules) != 81 || len(table) != len(validate.Rules) {
		t.Errorf("the table holds %d rows for %d rules; RULES.md states 81", len(table), len(validate.Rules))
	}
	check := func(id string, row rule) {
		t.Helper()
		if !acts[row.act] {
			t.Errorf("%s: the act %q is none of the ten", id, row.act)
		}
		if !roles[row.who] {
			t.Errorf("%s: the role %q is none of the design's", id, row.who)
		}
		if !strings.HasPrefix(row.source.URL, method) || !strings.Contains(row.source.URL, ".md#") {
			t.Errorf("%s: the source %q is no section of the method's texts", id, row.source.URL)
		}
		if row.source.Text == "" || row.sentence == "" {
			t.Errorf("%s: no sentence or no source", id)
		}
		if file, _, _ := strings.Cut(row.source.Text, ","); !strings.Contains(row.source.URL, "/"+file+"#") {
			t.Errorf("%s: the source %q does not stand at %s", id, row.source.Text, row.source.URL)
		}
	}
	for _, known := range validate.Rules {
		row, ok := table[known.ID]
		if !ok {
			t.Errorf("the table holds no row for %s", known.ID)
			continue
		}
		check(known.ID, row)
	}
	for _, key := range []string{Proposed, Stale, SoleReview} {
		kind, ok := kinds[key]
		if !ok || kind.kind == "" {
			t.Errorf("the kind %s has no row", key)
		}
		if _, isRule := validate.RuleOf(key); isRule {
			t.Errorf("the kind %s is a rule id", key)
		}
		check(key, kind.rule)
	}
	// The rows the design states one by one.
	for id, want := range map[string]rule{
		"R9": {act: Await, who: origin}, "R13": {act: MovePin, who: assignee}, "S11": {act: Review, who: reviewer},
		"J8": {act: Revise, who: keeper}, "J13": {act: MovePin, who: assignee}, "J17": {act: MovePin, who: assignee},
		"H1": {act: None, who: author}, "H2": {act: Review, who: reviewer}, "H3": {act: None, who: author},
		"H4": {act: RecordHandoff, who: contributor}, "H5": {act: ClearHandoff, who: contributor}, "H6": {act: None, who: author},
		"P5": {act: Revise, who: owner}, "T8": {act: Revise, who: owner}, "T12": {act: Revise, who: assignee},
	} {
		if got := table[id]; got.act != want.act || got.who != want.who {
			t.Errorf("%s: %s by the %s, want %s by the %s", id, got.act, got.who, want.act, want.who)
		}
	}
	others := 0
	for id, row := range table {
		if row.act == Revise && row.who == keeper {
			others++
		} else if row.act == Revise && row.who != owner && row.who != assignee {
			t.Errorf("%s: revise by the %s", id, row.who)
		}
	}
	if others != 67 {
		t.Errorf("%d rules resolve by a revision of the keeper's, want 67", others)
	}
	// The silent rules are rules, in the order of validate.Rules.
	at := -1
	for _, id := range silent {
		i := -1
		for j, known := range validate.Rules {
			if known.ID == id {
				i = j
			}
		}
		if i <= at {
			t.Errorf("the silent rule %s is out of the order of validate.Rules, or no rule", id)
		}
		at = i
	}
	if len(silent) != 17 {
		t.Errorf("%d silent rules, want the 17 of VIEWS.md#audit", len(silent))
	}
}

// T1: the table is true to the texts. Every sentence stands in that rule's
// row of the corpus's RULES.md, or in the row above where the row says "Same
// sentence".
func TestTableIsTrueToTheTexts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(corpus(t)), "RULES.md"))
	if err != nil {
		t.Skipf("the corpus's RULES.md is absent: %v", err)
	}
	rows := map[string]string{}
	above := ""
	for _, line := range regexp.MustCompile(`(?m)^\| ([A-Z][0-9]+) +\|.*$`).FindAllStringSubmatch(string(data), -1) {
		row := line[0]
		if strings.Contains(row, "| Same sentence") {
			row += above
		} else {
			above = row
		}
		rows[line[1]] = row
	}
	if len(rows) != len(validate.Rules) {
		t.Errorf("RULES.md holds %d rows for %d rules", len(rows), len(validate.Rules))
	}
	for _, known := range validate.Rules {
		row, ok := rows[known.ID]
		if !ok {
			t.Errorf("RULES.md holds no row for %s", known.ID)
			continue
		}
		if sentence := table[known.ID].sentence; !strings.Contains(row, sentence) {
			t.Errorf("%s: the sentence %q stands nowhere in its row of RULES.md", known.ID, sentence)
		}
		cells := strings.Split(row, "|")
		if text := table[known.ID].source.Text; len(cells) > 4 && strings.HasPrefix(text, "corpus/RULES.md") {
			if rule := strings.TrimSpace(cells[3]); table[known.ID].sentence != rule {
				t.Errorf("%s: the sentence %q is not the Rule column, %q", known.ID, table[known.ID].sentence, rule)
			}
		}
	}
}
