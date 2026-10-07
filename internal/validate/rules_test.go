package validate

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/nbyoung/tablo/internal/model"
)

// ruleRow matches a rule row of the corpus's RULES.md: the id and the severity.
var ruleRow = regexp.MustCompile(`^\| ([A-Z][0-9]+) +\| (error|warning|information) +\|`)

// TestRulesEqualTheCorpus is T1, first part: Rules equals the rule rows of
// the corpus's RULES.md in id, order and severity, and RuleOf finds each.
func TestRulesEqualTheCorpus(t *testing.T) {
	data, err := os.ReadFile(beside(t, "RULES.md"))
	if err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, line := range strings.Split(string(data), "\n") {
		if m := ruleRow.FindStringSubmatch(line); m != nil {
			rows = append(rows, m[1]+" "+m[2])
		}
	}
	var rules []string
	for _, rule := range Rules {
		rules = append(rules, rule.ID+" "+rule.Severity.String())
	}
	if got, want := strings.Join(rules, "\n"), strings.Join(rows, "\n"); got != want {
		t.Errorf("Rules:\n%s\nRULES.md:\n%s", got, want)
	}
}

// TestRuleTable holds the table to the design without the corpus: 81 rules,
// each id once, each with a title, and the count of each tier.
func TestRuleTable(t *testing.T) {
	tiers := map[Tier]int{}
	seen := map[string]bool{}
	for _, rule := range Rules {
		if seen[rule.ID] || rule.Title == "" {
			t.Errorf("%s: stated twice, or with no title", rule.ID)
		}
		seen[rule.ID] = true
		tiers[rule.Tier]++
		if got, ok := RuleOf(rule.ID); !ok || got != rule {
			t.Errorf("RuleOf(%s) = %+v, %v", rule.ID, got, ok)
		}
	}
	if len(Rules) != 81 || tiers[TierLoad] != 5 || tiers[TierFiles] != 65 || tiers[TierFacts] != 1 || tiers[TierHistory] != 10 {
		t.Errorf("%d rules by tier %v; want 81: 5, 65, 1 and 10", len(Rules), tiers)
	}
	if _, ok := RuleOf("Z9"); ok {
		t.Error("RuleOf finds Z9")
	}
	if rule, _ := RuleOf("H4"); rule.Severity != model.Information {
		t.Errorf("H4 is %s", rule.Severity)
	}
}
