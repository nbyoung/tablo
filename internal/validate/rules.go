package validate

import "github.com/nbyoung/tablo/internal/model"

// Tier says what a rule needs.
type Tier int

// The tiers of a rule.
const (
	TierLoad    Tier = iota // the Loader raises it: L1 to L4, P4
	TierFiles               // Files raises it from the model
	TierFacts               // Derived raises it from Facts.Requirements: R9
	TierHistory             // Derived raises it from Facts.History
)

// Rule is one rule of RULES.md as the module knows it.
type Rule struct {
	ID       string
	Severity model.Severity
	Tier     Tier
	Title    string // a noun phrase, as the Title column of rules.md gives it
}

// Rules holds every rule of RULES.md, in its order. RuleOf finds one by id.
var Rules = []Rule{
	{"P1", model.Error, TierFiles, "No project"},
	{"P2", model.Error, TierFiles, "No version file"},
	{"P3", model.Error, TierFiles, "A malformed version file"},
	{"P4", model.Error, TierLoad, "A version the tool does not accept"},
	{"P5", model.Warning, TierHistory, "An undetermined trunk"},

	{"L1", model.Error, TierLoad, "A file that is not YAML"},
	{"L2", model.Error, TierLoad, "A key stated twice"},
	{"L3", model.Error, TierLoad, "A YAML feature the syntax lacks"},
	{"L4", model.Warning, TierLoad, "A stray path"},

	{"G1", model.Error, TierFiles, "No gates file"},
	{"G2", model.Error, TierFiles, "A first gate other than undefined"},
	{"G3", model.Error, TierFiles, "Fewer than two gates"},
	{"G4", model.Error, TierFiles, "A malformed gate"},
	{"G5", model.Error, TierFiles, "A malformed key"},
	{"G6", model.Error, TierFiles, "A gate key stated twice"},
	{"G7", model.Error, TierFiles, "No states, or a malformed state"},
	{"G8", model.Error, TierFiles, "A severity that is no integer of at least 0"},
	{"G9", model.Error, TierFiles, "A state key stated twice"},
	{"G10", model.Error, TierFiles, "A malformed reason, or a reason key stated twice"},
	{"G11", model.Error, TierFiles, "An unknown field in the gates file"},
	{"G12", model.Error, TierFiles, "No undefined or complete state at severity 0"},

	{"T1", model.Error, TierFiles, "A task file name that is no id"},
	{"T2", model.Error, TierFiles, "A task without title, description or assignee"},
	{"T3", model.Error, TierFiles, "An unknown field in a task file"},
	{"T4", model.Error, TierFiles, "An address that is no email"},
	{"T5", model.Error, TierFiles, "A malformed reference"},
	{"T6", model.Error, TierFiles, "A malformed id"},
	{"T7", model.Warning, TierFiles, "An unquoted id"},
	{"T8", model.Error, TierFiles, "Not one root"},
	{"T9", model.Error, TierFiles, "A parent that is no task"},
	{"T10", model.Error, TierFiles, "A parent chain that loops"},
	{"T11", model.Error, TierFiles, "A malformed parent"},
	{"T12", model.Warning, TierFiles, "Siblings with one order"},

	{"R1", model.Error, TierFiles, "A requirement on no task"},
	{"R2", model.Error, TierFiles, "A requirement cycle"},
	{"R3", model.Error, TierFiles, "A requirement on itself"},
	{"R4", model.Error, TierFiles, "A requirement on an ancestor"},
	{"R5", model.Error, TierFiles, "A requirement on a descendant"},
	{"R6", model.Error, TierFiles, "A from gate that does not apply"},
	{"R7", model.Error, TierFiles, "A to gate that does not apply"},
	{"R8", model.Error, TierFiles, "A malformed requirement"},
	{"R9", model.Warning, TierFacts, "An unmet requirement"},
	{"R10", model.Error, TierFiles, "A to gate of undefined"},
	{"R11", model.Error, TierFiles, "A malformed cross-project requirement"},
	{"R12", model.Warning, TierFiles, "A requirement on what a junction reads"},
	{"R13", model.Warning, TierHistory, "A requirement its commit alone leaves unmet"},

	{"J1", model.Error, TierFiles, "A junction key that names no gate"},
	{"J2", model.Error, TierFiles, "A not-applicable entry at undefined"},
	{"J3", model.Error, TierFiles, "A recursive junction on a parent"},
	{"J4", model.Error, TierFiles, "An entry of no one kind"},
	{"J5", model.Error, TierFiles, "A model without a contributor"},
	{"J6", model.Error, TierFiles, "An applies other than false"},
	{"J7", model.Error, TierFiles, "A malformed subproject"},
	{"J8", model.Error, TierFiles, "A subproject the tool cannot read"},
	{"J9", model.Error, TierFiles, "A subproject task that does not exist"},
	{"J10", model.Error, TierFiles, "An unknown field in a plain entry"},
	{"J11", model.Error, TierFiles, "An entry at undefined"},
	{"J12", model.Error, TierFiles, "No gate after undefined applies"},
	{"J13", model.Warning, TierHistory, "A commit off the subproject's trunk"},
	{"J14", model.Error, TierFiles, "A malformed commit"},
	{"J15", model.Error, TierFiles, "An absolute URL without a commit"},
	{"J16", model.Error, TierFiles, "A commit on a same-repository path"},
	{"J17", model.Error, TierFiles, "A commit off the submodule's pin"},

	{"S1", model.Error, TierFiles, "A status for no task"},
	{"S2", model.Error, TierFiles, "A status on a parent"},
	{"S3", model.Error, TierFiles, "A malformed status"},
	{"S4", model.Error, TierFiles, "Undefined on one side only"},
	{"S5", model.Error, TierFiles, "A status gate that names no gate"},
	{"S6", model.Error, TierFiles, "A state or reason that names none"},
	{"S7", model.Error, TierFiles, "A status at a gate that does not apply"},
	{"S8", model.Error, TierFiles, "The last gate without complete"},
	{"S9", model.Error, TierFiles, "More than the gate before a recursive junction"},
	{"S10", model.Error, TierFiles, "No state before a plain junction"},
	{"S11", model.Error, TierHistory, "A status past a reviewed junction with no review"},
	{"S12", model.Error, TierFiles, "Complete before the last gate"},

	{"H1", model.Warning, TierHistory, "A trailer that names nothing"},
	{"H2", model.Warning, TierHistory, "A review from the wrong hand"},
	{"H3", model.Warning, TierHistory, "A model outside the one stated"},
	{"H4", model.Information, TierHistory, "A hand-off the history implies"},
	{"H5", model.Warning, TierHistory, "A stale hand-off"},
	{"H6", model.Warning, TierHistory, "No model trailer"},
}

// byID indexes Rules.
var byID = func() map[string]int {
	index := make(map[string]int, len(Rules))
	for i, rule := range Rules {
		index[rule.ID] = i
	}
	return index
}()

// RuleOf returns the rule with the id given.
func RuleOf(id string) (Rule, bool) {
	i, ok := byID[id]
	if !ok {
		return Rule{}, false
	}
	return Rules[i], true
}
