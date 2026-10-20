# The rules that language 0.4.0 adds or changes

The rows of [`design/8118/rules.md`](../8118/rules.md) and [`design/dada/rules.md`](../dada/rules.md) that this task adds or changes, in the columns those drafts use. Every other row of both stands. Tier, At, Fields and Message read as `8118/rules.md` defines them; Act, Resolver, Source and Sentence as `dada/rules.md` defines them. RULES.md is the umbrella's at `1d1cb02`, and the corpus entries are its.

## The Validator

`validate.Rules` holds 88 rules, in the order of RULES.md: A1 and A2 follow G12, R14 follows R13, J18 and J19 follow J17, and H7 and H8 follow H6.

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| A1 | error | files | A malformed agents file | schema: `agents.schema.yaml` over `Project.Agents.File`, after the gating rules and before the tree; every leaf maps to A1 | the leaf's node, else file `agents.yaml` | | schema; `uniqueItems` gives `agents states an item twice` | `agents-not-email` |
| A2 | error | files | A model beside a contributor the agents file does not list | `Project.Agents != nil`, and a plain entry with no schema leaf states a scalar `model` and a scalar `contributor` that `Agents.Lists` does not hold | `contributor` | t, g | `the % junction states the model % beside %, whom agents.yaml does not list` | `agents-unlisted-model` |
| T4 | error | files | An address that is no email | schema, amended: a leaf under `reviewer` is T4 whatever its keyword; the two leaves of its `oneOf` give one message | the leaf's node | t, g | `% % is neither an email address nor false`: the path and the value | `task-bad-email`, `junction-reviewer-true` |
| T6, J7, J14 | error | files | as they stand | schema, amended: a leaf under the task-level `subproject` maps as one under `junctions.<gate>.subproject` does, with no gate: `id` is T6, `commit` J14, any other J7 | the leaf's node | t | schema | `subproject-field` (none fires) |
| T7 | warning | files | An unquoted id | amended: the task-level `subproject.id` is a fifth place | the id | t | `subproject.id is written …`, as the other four | `unquoted-id`, `unquoted-id-text` |
| R12 | warning | files | A requirement on what a junction reads | amended: the task-level field reads as a junction: the same `*Link` and the same task there | the requirement's `subproject` | t | `the requirement on % at % names the task the subproject field reads` | `requires-junction-target` |
| R14 | information | files | An unpinned requirement | `Subproject.Unpinned()`: a requirement's field on an absolute URL with no `commit`; raised before J8, which still runs | the requirement's `subproject` | t | `the requirement on % at % states no commit and reads the tip of %, %`: id, url, `Link.Trunk`, `Link.Commit`; with no project, `… states no commit and reads no trunk: %`, the `Link.Problem` | `requires-unpinned` |
| J15 | error | files | An absolute URL without a commit | amended: on a junction's field and on the task-level field alone (`Subproject.On != OnRequirement`) | `subproject` | t, g | as it stands | `subproject-url-without-commit` |
| J18 | error | files | A task-level subproject on a parent | the task states the field and `shape.isParent`; the field's own rules then run as for a leaf's | the field's value | t | `a parent states a task-level subproject field` | `subproject-field-on-parent` |
| J19 | warning | files | A subproject whose language lags | after J8 passes on a junction's or the task-level field: both `version.yaml` well formed and the linked one `Before` this project's major, minor and patch; once per task and `*Link` | the `url` of the first field of the task that reaches the rule: the task-level field, then the junctions in file order | t | `the project at % states tableaux %, before this project's %` | `subproject-language-behind`, `tableaux-tooling` |
| H6 | warning | history | No model trailer | amended in code only: the exemption reads `model.Before(Commit.Tableaux, model.ModelTrailer)` | project | t, g, c | as it stands | `model-trailer-missing` |
| H7 | warning | history | An agent's work under a person's identity | a commit at a junction of `Commit.At` that is plain as the files stand there, with `Resolved.Agent`, whose `AuthorAgent` is false, and which carries a `Model` trailer or names an agent in `CoAgents`; silent where `model.Before(Commit.Tableaux, model.AgentIdentity)` | project | t, g, c | `% authors the agent's work at the % junction, whose contributor is %` | `agent-work-under-person` |
| H8 | warning | history | A reaffirmation that confirms nothing | a `reaffirmed` event of `Commit.Events` whose `Moot` is set, on a task the project holds | project | t, c, tr | `Reaffirmed: % confirms nothing: %`, then by `Moot`: `% is a parent, so its status derives from its children`; `the next junction, %, is recursive, so the status is the subproject's`; `% has no status file, so its date is its task file's` | `reaffirmed-snapshot`, `undefined-leaf-reaffirmed` |

**Order inside a task file.** `task` runs the schema, T7, then the task-level field (J18, then the subproject rows R14, J15, J8, J19, J16, J17, J9 with the codes the schema raised under `subproject`), then each junction entry (A2 for a plain one), then each requirement, then J12. The subproject rows of a junction and of a requirement gain R14 before J15 and J19 after J8.

**What D5 of 8118 keeps.** P4 still ends the run, so a project at 0.5.0 gets P4 alone. Every rule above applies whatever version a project states, but H6 and H7, whose exemptions read the version at the commit (decision 1).

## The Audit

`table` gains seven rows and changes five sentences; `silent` gains R14, J19, H7 and H8, in the order of `validate.Rules`. `Act` gains two values, decision 5.

| Id | Act | Resolver | Source | Sentence |
|----|-----|----------|--------|----------|
| A1 | revise | keeper | corpus/RULES.md, agents.yaml | `` `agents.yaml`, when present, matches its schema: `agents`, a non-empty list of distinct email addresses, and no other field `` |
| A2 | revise | keeper | README.md, Roles | `` A project that keeps the file lists every agent there, and a validator rejects a junction that states a `model` beside a contributor the file does not list. `` |
| R14 | state_commit | assignee | README.md, Tasks | `It reports an unpinned entry as information, since its condition moves with the other project's trunk` |
| J18 | revise | keeper | README.md, Junctions | `` a validator rejects one on a parent, and a task-level `subproject` field there too `` |
| J19 | raise_language | assignee | README.md, Version | `` A validator warns of a project that a recursive junction or a task's `subproject` field names whose `version.yaml`, read at the commit the linkage fixes, states a version before the parent's own, patch included `` |
| H7 | none | author | README.md, Roles | `` The audit reports a commit at an agent's junction whose author is a person and which carries a `Model:` trailer or a `Co-Authored-By:` trailer that names an agent `` |
| H8 | none | author | README.md, Status | `` a `Reaffirmed:` trailer naming it changes nothing a tool derives; the history keeps the event, and the audit reports the trailer `` |
| L4 | revise | keeper | corpus/RULES.md, Reading a file | the rule as RULES.md now words it, with `agents.yaml` among the paths |
| G7 | revise | keeper | corpus/RULES.md, gates.yaml | the rule as RULES.md now words it: `at least two states` |
| T4 | revise | keeper | corpus/RULES.md, tasks/<id>.yaml | the rule as RULES.md now words it, with `reviewer` `false` |
| T7 | revise | keeper | SYNTAX.md, tasks/<id>.yaml | `A tool takes an id as the scalar's text as the file writes it, whatever YAML yields` |
| J15 | revise | keeper | README.md, Junctions | `` A validator rejects … a `commit` that is absent on a junction's absolute URL `` |

The kind `SoleReview` takes the sentence README.md#junctions now states: `` A person's plain junction with no stated reviewer has none, and its status completes the gate on the contributor's word. `` H7 and H8 take the act `none`, so the demotion lists them as information: H7 once its junction is historical, H8, which names a task and no gate, once its task is complete.
