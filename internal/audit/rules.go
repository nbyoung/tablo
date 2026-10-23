package audit

// role names who resolves a finding of a rule: "keeper", "owner", "assignee",
// "contributor", "reviewer", "author", "origin", "authority" or "nobody".
type role string

// The roles, as the design resolves each to one email.
const (
	owner       role = "owner"       // the root's assignee
	authority   role = "authority"   // the task's nearest authority; the owner for the root
	assignee    role = "assignee"    // the task's assignee
	keeper      role = "keeper"      // whoever keeps the diagnostic's file
	recorder    role = "recorder"    // whoever records the task's status
	contributor role = "contributor" // the contributor of the junction
	reviewer    role = "reviewer"    // the reviewer of the junction
	author      role = "author"      // the author of the diagnostic's commit
	origin      role = "origin"      // whoever works on the originating task of a requirement
	nobody      role = "nobody"      // the files name no one
)

// rule is one row of design/dada/rules.md.
type rule struct {
	act      Act
	who      role
	source   Source
	sentence string
}

// table holds a row for every rule of validate.Rules. J8 and H2 take their
// act and their role in code, by the condition rules.md states: the row of
// J8 is that of a link the files can mend, and the row of H2 that of a
// junction no review accepts yet.
var table = map[string]rule{
	"P1": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"A Tableaux project is the `.tableaux` directory at the root of its Git repository, or under a directory of the repository for a project that shares it."},
	"P2": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"`version.yaml`, `gates.yaml`: Fixed"},
	"P3": {Revise, keeper, Source{"corpus/RULES.md, Project and version.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#project-and-versionyaml"},
		"`version.yaml` matches its schema: `tableaux` required, `major.minor.patch`; `trunk`, when present, non-empty; no other field"},
	"P4": {Revise, keeper, Source{"README.md, Version", "https://github.com/nbyoung/tableaux/blob/main/README.md#version"},
		"A tool accepts a project whose major version equals its own and whose minor version does not exceed it."},
	"P5": {Revise, owner, Source{"README.md, Project", "https://github.com/nbyoung/tableaux/blob/main/README.md#project"},
		"otherwise the trunk is undetermined, every task reads as proposed, and a validator warns"},
	"L1": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"Every file is YAML"},
	"L2": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"a mapping states each key once"},
	"L3": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"one document of plain mappings, sequences and scalars"},
	"L4": {Revise, keeper, Source{"corpus/RULES.md, Reading a file", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#reading-a-file"},
		"Every path under `.tableaux` is one the layout names: `version.yaml`, `gates.yaml`, `agents.yaml`, `tasks/<id>.yaml`, `status/<id>.yaml`, each a regular file; a tool leaves any other unread"},
	"G1": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"`version.yaml`, `gates.yaml`: Fixed"},
	"G2": {Revise, keeper, Source{"README.md, Gates", "https://github.com/nbyoung/tableaux/blob/main/README.md#gates"},
		"The first gate is always `undefined`."},
	"G3": {Revise, keeper, Source{"corpus/RULES.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#gatesyaml"},
		"At least two gates"},
	"G4": {Revise, keeper, Source{"corpus/RULES.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#gatesyaml"},
		"Each gate has `key`, `symbol`, `name`, `criteria`, each non-empty, and no other field"},
	"G5": {Revise, keeper, Source{"corpus/RULES.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#gatesyaml"},
		"Every key matches `^[a-z][a-z0-9_-]*$`"},
	"G6": {Revise, keeper, Source{"SYNTAX.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#gatesyaml"},
		"A unique identifier that other files use to name the gate"},
	"G7": {Revise, keeper, Source{"corpus/RULES.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#gatesyaml"},
		"`states` exists with at least two states; each has `key`, `symbol`, `severity`, `synopsis`"},
	"G8": {Revise, keeper, Source{"corpus/RULES.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#gatesyaml"},
		"`severity` is an integer of at least 0"},
	"G9": {Revise, keeper, Source{"SYNTAX.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#gatesyaml"},
		"A unique identifier that status records use to name the state"},
	"G10": {Revise, keeper, Source{"corpus/RULES.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#gatesyaml"},
		"Each reason has `key`, `symbol`, `synopsis`; reason keys are unique"},
	"G11": {Revise, keeper, Source{"corpus/RULES.md, gates.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#gatesyaml"},
		"No field other than `gates`, `states`, `reasons`"},
	"G12": {Revise, keeper, Source{"README.md, Gates", "https://github.com/nbyoung/tableaux/blob/main/README.md#gates"},
		"The states always include `undefined` … and `complete` …, both with severity 0 so that roll-up sets them aside."},
	"A1": {Revise, keeper, Source{"corpus/RULES.md, agents.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#agentsyaml"},
		"`agents.yaml`, when present, matches its schema: `agents`, a non-empty list of distinct email addresses, and no other field"},
	"A2": {Revise, keeper, Source{"README.md, Roles", "https://github.com/nbyoung/tableaux/blob/main/README.md#roles"},
		"A project that keeps the file lists every agent there, and a validator rejects a junction that states a `model` beside a contributor the file does not list."},
	"T1": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"`<id>` is the task id: four lowercase hexadecimal digits chosen at random, `^[0-9a-f]{4}$`"},
	"T2": {Revise, keeper, Source{"corpus/RULES.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#tasksidyaml"},
		"`title`, `description` and `assignee` exist and are non-empty"},
	"T3": {Revise, keeper, Source{"SYNTAX.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#tasksidyaml"},
		"the file holds no `id` field."},
	"T4": {Revise, keeper, Source{"corpus/RULES.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#tasksidyaml"},
		"`assignee`, `contributor` and `reviewer` are email addresses; `reviewer` may instead be `false`, which waives review"},
	"T5": {Revise, keeper, Source{"corpus/RULES.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#tasksidyaml"},
		"Each reference has `url`, a URI reference; `text`, when present, is non-empty"},
	"T6": {Revise, keeper, Source{"corpus/RULES.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#tasksidyaml"},
		"Every id a file names matches `^[0-9a-f]{4}$`"},
	"T7": {Revise, keeper, Source{"SYNTAX.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#tasksidyaml"},
		"A tool takes an id as the scalar's text as the file writes it, whatever YAML yields"},
	"T8": {Revise, owner, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"Exactly one task file has no `parent`; it is the **root**"},
	"T9": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"Every other task names its parent by id."},
	"T10": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"Every parent chain ends at the root."},
	"T11": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"Siblings sort by `order`, a strictly positive integer"},
	"T12": {Revise, assignee, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"A tool warns of two siblings with the same `order`, since only the id then decides."},
	"R1": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"The **requires** relationship names the tasks whose results this task needs"},
	"R2": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"The relation crosses the tree freely but forms no cycle"},
	"R3": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"a task never requires itself, an ancestor or a descendant"},
	"R4": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"a task never requires itself, an ancestor or a descendant"},
	"R5": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"a task never requires itself, an ancestor or a descendant"},
	"R6": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"A validator checks that each gate applies to its task."},
	"R7": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"A validator checks that each gate applies to its task."},
	"R8": {Revise, keeper, Source{"corpus/RULES.md, requires in a task file", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#requires-in-a-task-file"},
		"Each entry has `id` or `subproject`; `text`, when present, is non-empty; no other field"},
	"R9": {Await, origin, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"A validator warns of an unmet requirement"},
	"R10": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"`to` is never `undefined`, since no work needs a result before definition."},
	"R11": {Revise, keeper, Source{"SYNTAX.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#tasksidyaml"},
		"One of `id`, `subproject`"},
	"R12": {Revise, keeper, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"A validator warns of a cross-project entry that names the task one of this task's recursive junctions already reads"},
	"R13": {MovePin, assignee, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"the audit reports … a cross-project requirement that is unmet at its `commit` and met at that trunk's tip"},
	"R14": {StateCommit, assignee, Source{"README.md, Tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#tasks"},
		"It reports an unpinned entry as information, since its condition moves with the other project's trunk"},
	"J1": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"A validator checks that every junction key names a gate in `gates.yaml`."},
	"J2": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"The `undefined` gate always applies and has no work of its own, so a file states no entry of any kind at `undefined`."},
	"J3": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"A recursive junction names one task's work and does not inherit, so a validator rejects one on a parent, and a task-level `subproject` field there too."},
	"J4": {Revise, keeper, Source{"SYNTAX.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#tasksidyaml"},
		"`junctions.<gate>` is one of three kinds, told apart by its fields."},
	"J5": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"an entry that states a `model` states its `contributor` beside it rather than inheriting one."},
	"J6": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"`applies`, takes only the value `false`: the entry's presence exempts the gate, and `applies: true` would restate the default the file omits."},
	"J7": {Revise, keeper, Source{"corpus/RULES.md, junctions in a task file", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#junctions-in-a-task-file"},
		"A recursive entry's `subproject` has `url`; `id`, when present, is an id; no field beyond `url`, `id`, `commit`"},
	"J8": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"A validator rejects a `url` that does not resolve to a Tableaux project it can read"},
	"J9": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"and an `id` that names no task in that project"},
	"J10": {Revise, keeper, Source{"corpus/RULES.md, junctions in a task file", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#junctions-in-a-task-file"},
		"A plain entry has no field beyond `contributor`, `model`, `reviewer`, `references`"},
	"J11": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"The `undefined` gate … has no work of its own, so a file states no entry of any kind at `undefined`."},
	"J12": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"At least one gate after `undefined` applies to every task"},
	"J13": {MovePin, assignee, Source{"README.md, Project", "https://github.com/nbyoung/tableaux/blob/main/README.md#project"},
		"A subproject's own `version.yaml` names its trunk, and the audit reports a commit, pinned by a submodule or named by a `commit` field, that is not on it."},
	"J14": {Revise, keeper, Source{"SYNTAX.md, tasks/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#tasksidyaml"},
		"The full hash of the commit"},
	"J15": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"A validator rejects … a `commit` that is absent on a junction's absolute URL"},
	"J16": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"Nothing pins it, so `commit` is an error there."},
	"J17": {MovePin, assignee, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"when present it restates the pin and must equal it"},
	"J18": {Revise, keeper, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"a validator rejects one on a parent, and a task-level `subproject` field there too"},
	"J19": {RaiseLanguage, assignee, Source{"README.md, Version", "https://github.com/nbyoung/tableaux/blob/main/README.md#version"},
		"A validator warns of a project that a recursive junction or a task's `subproject` field names whose `version.yaml`, read at the commit the linkage fixes, states a version before the parent's own, patch included"},
	"S1": {Revise, keeper, Source{"SYNTAX.md, Layout", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#layout"},
		"`<id>` is the id of a leaf task"},
	"S2": {Revise, keeper, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"A parent has no file; its status derives from its children."},
	"S3": {Revise, keeper, Source{"corpus/RULES.md, status/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md#statusidyaml"},
		"`gate` exists; `note`, when present, is non-empty; no other field"},
	"S4": {Revise, keeper, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"The gate is `undefined` exactly when the state is `undefined`."},
	"S5": {Revise, keeper, Source{"SYNTAX.md, status/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#statusidyaml"},
		"The key of the last gate the task has completed"},
	"S6": {Revise, keeper, Source{"SYNTAX.md, status/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#statusidyaml"},
		"The key of the task's state"},
	"S7": {Revise, keeper, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"The gate applies to the task; a validator rejects a status at a gate that a not-applicable junction exempts."},
	"S8": {Revise, keeper, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"A task at its last applicable gate has the state `complete`, and only there."},
	"S9": {Revise, keeper, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"When the next junction is recursive, the file holds only the gate."},
	"S10": {Revise, keeper, Source{"SYNTAX.md, status/<id>.yaml", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#statusidyaml"},
		"absent only when the next junction is recursive"},
	"S11": {Review, reviewer, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"A validator rejects a status whose gate passes a reviewed junction that has no such commit."},
	"S12": {Revise, keeper, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"A task at its last applicable gate has the state `complete`, and only there."},
	"H1": {None, author, Source{"SYNTAX.md, Commit trailers", "https://github.com/nbyoung/tableaux/blob/main/SYNTAX.md#commit-trailers"},
		"a validator warns of a trailer that names neither, since Git keeps it and the method cannot read it."},
	"H2": {Review, reviewer, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"A `Reviewed:` commit from anyone other than the junction's reviewer has no effect, and the audit reports it."},
	"H3": {None, author, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"the audit reports a commit at the junction whose trailer names a model outside the one stated"},
	"H4": {RecordHandoff, contributor, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"it reports, as information, a task whose next junction has a reviewer and no `Reviewed:` commit from that reviewer yet, whose newest event is the contributor's and whose status states no `review`"},
	"H5": {ClearHandoff, contributor, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"it reports a status that still states `review` after the reviewer's `Reviewed:` commit for that junction"},
	"H6": {None, author, Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"a commit at the junction by its contributor that carries no trailer, unless the project's `version.yaml` at that commit states a language before 0.2.1"},
	"H7": {None, author, Source{"README.md, Roles", "https://github.com/nbyoung/tableaux/blob/main/README.md#roles"},
		"The audit reports a commit at an agent's junction whose author is a person and which carries a `Model:` trailer or a `Co-Authored-By:` trailer that names an agent"},
	"H8": {None, author, Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"a `Reaffirmed:` trailer naming it changes nothing a tool derives; the history keeps the event, and the audit reports the trailer"},
}

// silent lists the rules the audit view names in VIEWS.md#audit, in the
// order of validate.Rules: P5, R9, R12 to R14, J8, J9, J13 to J17, J19, H1
// to H8.
var silent = []string{
	"P5", "R9", "R12", "R13", "R14", "J8", "J9", "J13", "J14", "J15", "J16", "J17", "J19",
	"H1", "H2", "H3", "H4", "H5", "H6", "H7", "H8",
}

// The kinds no rule states, as the audit names them, with the sentence of
// the method each rests on and where it stands.
var kinds = map[string]struct {
	kind string
	rule
}{
	Proposed: {"A proposed task", rule{Authorise, authority,
		Source{"README.md, Proposed and authorised tasks", "https://github.com/nbyoung/tableaux/blob/main/README.md#proposed-and-authorised-tasks"},
		"The task is authorised when the deciding commit's author or committer is one of its authorities, and proposed otherwise."}},
	Stale: {"A stale status", rule{Reaffirm, recorder,
		Source{"README.md, Status", "https://github.com/nbyoung/tableaux/blob/main/README.md#status"},
		"A review that finds no change reaffirms with an empty commit, so a status is never older than its last confirmation."}},
	SoleReview: {"A junction nobody else reviews", rule{None, authority,
		Source{"README.md, Junctions", "https://github.com/nbyoung/tableaux/blob/main/README.md#junctions"},
		"A person's plain junction with no stated reviewer has none, and its status completes the gate on the contributor's word."}},
}
