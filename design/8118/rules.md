# The rules, one row each

Every rule of [corpus/RULES.md](https://github.com/nbyoung/tableaux/blob/main/corpus/RULES.md), 81 in all, with its one owner. The row is the implementer's contract: the function raises the rule under the condition in "Raises when", at the position in "At", with the diagnostic fields in "Fields", and with the message in "Message". [`schema.md`](schema.md) holds the rows that a schema leaf decides, marked `schema` here.

**Tier.** `load`: the Loader raises it and the Validator copies it through. `files`: `validate.Files`, from the model alone. `facts`: `validate.Derived`, from `Facts.Requirements`, with or without a history. `history`: `validate.Derived`, only when `Facts.History` is not nil.

**At.** `project`: `Pos{}`, no file. `file`: `Pos{File: path}`, no line. Any other entry names a node, and the position is that node's `Pos`; `key of x` is the `KeyPos` of the field. A node that is absent falls back to the next position the row names, then to `file`.

**Fields.** `t`: `Task` is the id the file's name gives. `g`: `Gate` is the key the row names. `c`: `Commit`. `tr`: `Trailer`. A row sets no other field. A diagnostic in `version.yaml` or `gates.yaml` carries no task.

**Message.** A fixed format; `%` marks a value. A message holds no line, no column and no word of the schema library, so two runs give the same bytes and a message compares across two trees (tabloio `b618`, step 8).

## Project, version and reading

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| P1 | error | files | No project | `!p.Exists`; no other rule then runs | project | | `no .tableaux directory at or above %`: `Where.Dir`, or `the repository's root` | `no-tableaux-directory` |
| P2 | error | files | No version file | `p.Files` holds no `version.yaml` | file `version.yaml` | | `version.yaml is missing` | `version-missing` |
| P3 | error | files | A malformed version file | schema | schema | | schema | `version-bad-pattern`, `version-unknown-field`, `version-trunk-empty` |
| P4 | error | load | A version the tool does not accept | The Loader's; the Validator then applies P2 and P3 and no other rule | `tableaux` | | the Loader's | `version-major-mismatch`, `version-minor-ahead` |
| P5 | warning | history | An undetermined trunk | `History.Trunk == ""` | file `version.yaml` | | `the trunk is undetermined: version.yaml names none, refs/remotes/origin/HEAD is absent and the caller names no branch` | `trunk-undetermined` |
| L1 | error | load | A file that is not YAML | The Loader's | line, no column | t | the Loader's | `file-not-yaml` |
| L2 | error | load | A key stated twice | The Loader's | the second key | t | the Loader's | `file-duplicate-key` |
| L3 | error | load | A YAML feature the syntax lacks | The Loader's | the node | t | the Loader's | `file-yaml-feature` |
| L4 | warning | load | A stray path | The Loader's | file | | the Loader's | `file-stray-path` |

## `gates.yaml`

G1 runs when `p.Files` holds no `gates.yaml`. Every other G rule runs when `p.Gating` is not nil. With `p.Gating` nil, every rule below that names "gating" in its condition stays silent.

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| G1 | error | files | No gates file | `p.Files` holds no `gates.yaml` | file `gates.yaml` | | `gates.yaml is missing` | `gates-missing` |
| G2 | error | files | A first gate other than undefined | schema | schema | | schema | `gates-first-not-undefined` |
| G3 | error | files | Fewer than two gates | schema | schema | | schema | `gates-only-undefined` |
| G4 | error | files | A malformed gate | schema | schema | g: the item's `key` | schema | `gates-gate-missing-criteria` |
| G5 | error | files | A malformed key | schema | schema | g, on a gate's key only | schema | `gates-bad-key` |
| G6 | error | files | A gate key stated twice | A gate's scalar `key` equals an earlier gate's | the later `key` | g | `gate key % is stated twice` | `gates-duplicate-gate` |
| G7 | error | files | No states, or a malformed state | schema | schema | | schema | `gates-no-states` |
| G8 | error | files | A severity that is no integer of at least 0 | schema | schema | | schema | `gates-negative-severity` |
| G9 | error | files | A state key stated twice | As G6, over `states` | the later `key` | | `state key % is stated twice` | `gates-duplicate-state` |
| G10 | error | files | A malformed reason, or a reason key stated twice | schema; and as G6, over `reasons` | schema; the later `key` | | schema; `reason key % is stated twice` | `gates-duplicate-reason` |
| G11 | error | files | An unknown field in the gates file | schema | schema | | schema | `gates-unknown-field` |
| G12 | error | files | No undefined or complete state at severity 0 | `states` holds an item, and for each of `undefined` and `complete`: no state has the key, or the first that has it states a `severity` other than the integer 0 | `states`; the `severity` | | `states lacks %`; `the state % has a severity other than 0` | `gates-no-undefined-state`, `gates-complete-nonzero-severity` |

## Tasks and the tree

`unread` is the set of ids whose task file is in `p.Files` and gave no task (L1). A rule that asks whether a task exists stays silent for an unread id, so one unreadable file gives one error.

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| T1 | error | files | A task file name that is no id | `Task.ID` does not match `^[0-9a-f]{4}$`; the task stays in the tree | file | t | `the file name % is not four lowercase hexadecimal digits` | `task-bad-filename` |
| T2 | error | files | A task without title, description or assignee | schema | schema | t | schema | `task-missing-assignee` |
| T3 | error | files | An unknown field in a task file | schema | schema | t | schema | `task-id-field` |
| T4 | error | files | An address that is no email | schema, on `assignee`, `contributor` and `reviewer` | schema | t; g in a junction | schema | `task-bad-email` |
| T5 | error | files | A malformed reference | schema, on the task's and a junction's `references` | schema | t; g in a junction | schema | `task-reference-without-url` |
| T6 | error | files | A malformed id | schema, at the four places a file names an id | schema | t; g in a junction | schema | `task-bad-id-pattern` |
| T7 | warning | files | An unquoted id | At the four places, the node is a scalar and `!Quoted` | the id | t | `Read` is `String`: `% is written without quotes: %`; else `% is written as the % %, not the string "%"`, with the path, `integer` or `float`, and the text | `unquoted-id` |
| T8 | error | files | Not one root | `unread` is empty and the tasks with a nil `Parent` are not one | project | | none: `no task is the root: every task file states a parent`; several: `% tasks state no parent: %`, the ids in order | `tree-no-root`, `tree-two-roots` |
| T9 | error | files | A parent that is no task | `parent.id` is a scalar, names no task and is not unread | `parent.id` | t | `parent % names no task` | `tree-parent-missing` |
| T10 | error | files | A parent chain that loops | `parent.id` names a task, and the walk from the task up its parents meets a task twice | `parent.id` | t | `the parent chain of % loops and never reaches the root` | `tree-parent-cycle`, `tree-no-root` |
| T11 | error | files | A malformed parent | schema | schema | t | schema | `task-parent-order-not-integer`, `task-parent-order-zero` |
| T12 | warning | files | Siblings with one order | Two or more children of one existing task state one `order` that is `OK`; one diagnostic for each such order, orders ascending | file of the parent | t: the parent | `% share order % under %`, the ids in order joined by ` and ` | `siblings-same-order` |

## Requirements

The function takes each entry of `requires` in file order and stops at the first row that says "stop". An entry that is no mapping, or that states both or neither of `id` and `subproject`, has its schema row and nothing more.

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| R1 | error | files | A requirement on no task | A local `id` is a scalar, names no task and is not unread; stop | `id` | t | `requires %, which names no task` | `requires-missing-task`, `task-bad-id-pattern` |
| R3 | error | files | A requirement on itself | The `id` is the task's own; stop | `id` | t | `% requires itself` | `requires-self` |
| R4 | error | files | A requirement on an ancestor | The `id` is among the task's ancestors; stop | `id` | t | `% requires its ancestor %` | `requires-ancestor` |
| R5 | error | files | A requirement on a descendant | The task is among the ancestors of `id`; stop | `id` | t | `% requires its descendant %` | `requires-descendant` |
| R2 | error | files | A requirement cycle | Over the local entries that pass R1 to R5: the entry's edge lies on a cycle | `id` | t | `the requirement on % closes a cycle` | `requires-cycle` |
| R11 | error | files | A malformed cross-project requirement | schema: both `id` and `subproject`, or a `subproject` with no `url` or no `id`, or an unknown field in it; stop | schema | t | schema | `requires-subproject-without-id` |
| R8 | error | files | A malformed requirement | schema | schema | t | schema | `requires-unknown-field` |
| J14 to J17, J8, J9 | | | | A cross-project entry: the rows under Junctions, with no gate; stop where they stop | | t | | |
| R12 | warning | files | A requirement on what a junction reads | A cross-project entry that passes J9, and a recursive entry of the same task whose `Subproject.Link` is the same `*Link` and whose task there, its `id` or that project's root, is the same | `subproject` | t | `the requirement on % at % names the task the % junction reads`: the id, `Link.URL`, the gate | `requires-junction-target` |
| R6 | error | files | A from gate that does not apply | Gating in both projects; `from` is a scalar that matches the key pattern and names no gate of the originating project, or a gate that does not apply to the originating task | `from` | t, g: `from` | `from % names no gate in the originating project's gates.yaml`; `from % does not apply to %` | `requires-from-unknown-gate`, `requires-from-not-applicable` |
| R10 | error | files | A to gate of undefined | Gating; `to` is `undefined` | `to` | t, g | `to is undefined: no work needs a result before definition` | `requires-to-undefined` |
| R7 | error | files | A to gate that does not apply | Gating; `to` matches the key pattern, is not `undefined`, and names no gate or a gate that does not apply to the task | `to` | t, g: `to` | `to % names no gate in gates.yaml`; `to % does not apply to %` | `requires-to-not-applicable` |
| R9 | warning | facts | An unmet requirement | A `Condition` with `Due` and not `Met`, whose task and index name an entry | the entry | t | `the requirement on % from % is due at % and unmet: % stands at %` | `unmet-requirement`, `weather-station`, `requires-commit-behind` |
| R13 | warning | history | A requirement its commit alone leaves unmet | A `Condition` that raises R9, on a cross-project entry, with `MetAtTip` | `subproject.commit`, else `subproject` | t | `the requirement on % is unmet at its commit and met at the tip of the originating project's trunk` | `requires-commit-behind` |

## Junctions

The function takes each entry of `junctions` in file order. J2, J11 and J1 each end the entry. An entry that is no mapping is J4. Otherwise `Junction.Kind()` selects: `Mixed` is J4; the three others validate the entry against the schema of that kind ([`schema.md`](schema.md)). A recursive entry takes J3, and with no J4 and no J7 it then takes the subproject rows, in the order J14, J15, J8, J16, J17, J9.

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| J2 | error | files | A not-applicable entry at undefined | The key is `undefined` and the entry is `{ applies: false }` and no more | key | t, g | `a not-applicable entry at undefined: the undefined gate always applies` | `junction-undefined-not-applicable` |
| J11 | error | files | An entry at undefined | The key is `undefined`, any other entry | key | t, g | `an entry at undefined: the undefined gate has no work of its own` | `junction-undefined-plain`, `junction-undefined-recursive` |
| J1 | error | files | A junction key that names no gate | The key fails the key pattern; or gating, and no gate has the key | key | t, g | `junction key % is no gate key`; `junction key % names no gate in gates.yaml` | `junction-unknown-gate` |
| J4 | error | files | An entry of no one kind | No mapping; `Mixed`; or schema: an unknown field beside `subproject` | the entry; key; schema | t, g | `the entry is no mapping, so it is none of the three kinds`; `the entry mixes the fields of more than one kind`; schema | `junction-mixed-kind` |
| J5 | error | files | A model without a contributor | schema | `model` | t, g | `% states model and no contributor` | `junction-model-without-contributor` |
| J6 | error | files | An applies other than false | schema | schema | t, g | schema | `junction-applies-true` |
| J7 | error | files | A malformed subproject | schema | schema | t, g | schema | `junction-subproject-without-url` |
| J10 | error | files | An unknown field in a plain entry | schema | schema | t, g | schema | `junction-unknown-field` |
| J3 | error | files | A recursive junction on a parent | The entry is recursive and a task names this one as its parent | key | t, g | `a parent states a recursive junction` | `recursive-on-parent` |
| J14 | error | files | A malformed commit | schema; stop | schema | t, g | schema | `subproject-commit-malformed` |
| J15 | error | files | An absolute URL without a commit | `Link.Form == URL` and no `commit` field; stop | `subproject` | t, g | `the absolute url % states no commit` | `subproject-url-without-commit` |
| J8 | error | files | A subproject the tool cannot read | `Link.Problem != Resolved`, or the linked project's version is well formed and not accepted; stop | `url` | t, g | `url % resolves to no Tableaux project the tool can read: %`, with `Problem.String()`; `url % resolves to a project at tableaux %, which the tool does not read` | `subproject-path-missing` |
| J16 | error | files | A commit on a same-repository path | `Link.Form == Directory` and a `commit` field | `commit` | t, g | `url % is a directory of this repository, which nothing pins` | `subproject-directory-with-commit` |
| J17 | error | files | A commit off the submodule's pin | `Link.Form == Submodule` and the `commit` field differs from `Link.Commit` | `commit` | t, g | `commit % is not the commit the submodule % pins, %` | `subproject-commit-off-pin` |
| J9 | error | files | A subproject task that does not exist | The task read there, the `id` or that project's one root, is no task of the linked project and is not unread there; stop | `id`, else `subproject` | t, g | `id % names no task in the project at %`; `the project at % has no one root task` | `subproject-task-missing` |
| J12 | error | files | No gate after undefined applies | Gating with a gate; no gate but `undefined` applies to the task | file | t | `no gate after undefined applies to %` | `junction-all-not-applicable`, `gates-only-undefined` |
| J13 | warning | history | A commit off the subproject's trunk | Each `History.OffTrunk` that names a `subproject` field with a link | `commit`, else `url` | t; g on a junction | `the commit % of % is not on its trunk, %` | `subproject-pin-off-trunk` |

## Status

The function takes each status in id order and stops at the first row that says "stop". `app` is the task's applicable gates and `last` the last of them.

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| S1 | error | files | A status for no task | The id names no task and is not unread; stop, also when it is unread | file | t | `the file name % is the id of no task` | `status-no-task` |
| S2 | error | files | A status on a parent | A task names this one as its parent | file | t | `% is a parent: its status derives from its children` | `status-on-parent` |
| S3 | error | files | A malformed status | schema; stop after the schema rows when `gate` is no scalar or gating is nil | schema | t | schema | `status-missing-gate`, `status-unknown-field` |
| S4 | error | files | Undefined on one side only | schema | schema | t | schema | `status-undefined-gate-nominal-state`, `status-nominal-gate-undefined-state` |
| S5 | error | files | A status gate that names no gate | schema, on a key that fails the pattern; or no gate has the key; stop | `gate` | t, g | schema; `gate % names no gate in gates.yaml` | `status-unknown-gate` |
| S6 | error | files | A state or reason that names none | schema; or a `state` that matches the pattern and names no state, and the same for `reason`, one diagnostic each | `state`; `reason` | t | schema; `state % names no state in gates.yaml`; `reason % names no reason in gates.yaml` | `status-unknown-state`, `status-unknown-reason` |
| S7 | error | files | A status at a gate that does not apply | The gate is not in `app`; stop | `gate` | t, g | `gate % does not apply to %` | `status-gate-not-applicable` |
| S8 | error | files | The last gate without complete | The gate is `last` and the state is not `complete` | `state`, else `gate` | t, g | `% stands at its last applicable gate, %, without the state complete` | `status-last-gate-not-complete` |
| S12 | error | files | Complete before the last gate | The state is `complete` and the gate is not `last` | `state` | t, g | `the state complete at %, which is not the last applicable gate, %` | `status-complete-early` |
| S9 | error | files | More than the gate before a recursive junction | The gate is neither `undefined` nor `last`, the junction at the next gate of `app` is recursive, and the file states `state`, `reason` or `note`; one diagnostic | key of the first of the three in the file | t, g | `the next junction, %, is recursive, so the file holds only the gate` | `status-state-with-recursive` |
| S10 | error | files | No state before a plain junction | The same gate, the next junction is plain, and the file states no `state` | `gate` | t, g | `the next junction, %, is plain and the file states no state` | `status-no-state-plain` |
| S11 | error | history | A status past a reviewed junction with no review | A leaf with a whole parent chain and a status whose gate is in `app`: each gate of `app` up to and including the status gate, other than `undefined` and `defined`, whose `Facts.Junctions` entry is plain with a reviewer, and no commit by that reviewer carries `Reviewed: <id> <gate>` | `gate` | t, g: the junction's | `the status passes %, whose reviewer is %, and no commit by the reviewer carries Reviewed: % %` | `status-unreviewed-gate`, `status-defined-by-authorisation` |

## Commit trailers

A commit is **by** a person when the person's email is its author's. A commit **by the reviewer** is one whose author or committer is the reviewer, as README.md states for a review. The next junction of a leaf is the one at the gate of `app` after its status gate, `undefined` for a leaf with no status file. A leaf enters S11, H4 and H5 when its parent chain is whole and its status gate, when it has a file, is a scalar in `app`. Every row reads `Facts.History.Commits`, newest first.

| Id | Sev | Tier | Title | Raises when | At | Fields | Message | Entries |
|----|-----|------|-------|-------------|----|--------|---------|---------|
| H1 | warning | history | A trailer that names nothing | Each `Authorised`, `Reaffirmed` or `Reviewed` trailer: its value is not one word, or two for `Reviewed`; or the first names no task and is not unread; or, for `Reviewed`, the second names no gate or a gate that does not apply to the task | project | c, tr | `is malformed`; `% names no task`; `% names no task and % names no gate`; `% names no gate`; `% does not apply to %` | `unknown-trailer` |
| H2 | warning | history | A review from the wrong hand | A `Reviewed` trailer that passes H1, whose junction is plain with a reviewer, on a commit not by the reviewer | project | t, g, c | `% is not the reviewer of the % junction; % is`: the author, the gate, the reviewer | `review-by-non-reviewer` |
| H3 | warning | history | A model outside the one stated | A commit at a plain junction that states a model, with a `Model` trailer whose value does not start with that model; one diagnostic for the commit and junction, naming the first such value | project | t, g, c | `Model % is outside the stated model %` | `model-mismatch`, `junction-kinds` |
| H6 | warning | history | No model trailer | A commit at a plain junction that states a model, by the junction's contributor, with no `Model` trailer, unless `Commit.Tableaux` is a version before 0.2.1 | project | t, g, c | `the contributor's commit at the % junction carries no Model: trailer; the junction states %` | `model-trailer-missing` |
| H5 | warning | history | A stale hand-off | A leaf with a status whose `reason` is `review`, whose next junction is plain with a reviewer, and a commit by that reviewer carries `Reviewed:` for that junction; the commit is the newest such | `reason` | t, g: the next gate, c | `the status still states review after % accepted %` | `handoff-stale` |
| H4 | information | history | A hand-off the history implies | A leaf whose next junction is plain with a reviewer and whose status states no `review`: the newest commit that holds an event of the task, a `reviewed` event that H2 reports aside, is by the junction's contributor | file of the status, else of the task | t, g: the next gate, c | `the newest event is the contributor's and the status states no review; % may have % to review` | `handoff-inferred` |
