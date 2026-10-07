# Design dada: Audit

The Audit turns what the Validator and the Derivation already know into findings a person acts on. It wraps every diagnostic of the envelope in a record that adds the action that resolves it and the person who takes it, and it raises the two disagreements that no rule states: a proposed task and a stale status. It reads no history and derives no fact; every fact comes from a method of `derive.Facts`, and every rule finding from a diagnostic it carries unchanged. Eight choices shape it, each a decision for the owner (D*n* is decision *n* under [Decisions at review](#decisions-at-review)). The Audit raises two kinds and no third for an agent that reviews its own work (D1). An action has one of ten acts, the eight of VIEWS.md and two more (D2), and four history rules resolve by nothing, since the files at a commit judge the commit (D3). One table names who resolves each rule (D4). A status is stale only when a commit records it, it is not complete and no subproject states it (D5). Off the trunk the Audit reports the tasks a branch changes, not every task (D6). The library reads no clock: with no date it counts from the commit in view (D7). The record is one Go type whose JSON the three front ends read (D8).

| Draft                                                  | Lands as                                    | Holds                                                                         |
|--------------------------------------------------------|---------------------------------------------|-------------------------------------------------------------------------------|
| [`dada/model.md`](dada/model.md)                       | The declarations of `internal/audit`        | Every type and function, with its doc comment                                 |
| [`dada/rules.md`](dada/rules.md)                       | `table` in `internal/audit/rules.go`        | Each of the 81 rules: act, resolver, action, sentence and source              |
| [`dada/words.md`](dada/words.md)                       | The texts in `internal/audit/words.go`      | The two kinds, and every action, fact, commit list and command that takes a value |
| [`dada/corpus.expected.txt`](dada/corpus.expected.txt) | `internal/audit/testdata/corpus.expected.txt` | What the Audit reports on 24 corpus entries, and the 18 it has no report for |
| [`dada/corpus_test.md`](dada/corpus_test.md)           | `internal/audit/corpus_test.go`             | The test that writes that file, and how it builds diagnostics before 4ed9     |
| [`dada/audit.json`](dada/audit.json)                   | `internal/audit/testdata/audit.json`        | The audit data of `weather-station` at `main`: rows, counts, most, silent     |

## The model

### The package

| Path             | Holds                                                    | Imports                                   |
|------------------|----------------------------------------------------------|-------------------------------------------|
| `internal/audit` | `Run`, `Report`, `Finding` and the table of the 81 rules | `model`, `history`, `derive`, `validate`  |

The task exports nothing from `github.com/nbyoung/tablo`. `snapshot.go` (4ed9) calls `Run` once per snapshot, and the audit view (8ed1) selects, joins and encodes what it returns (A1 to A3). The package imports `validate` for `Rules` alone: a rule's title is the finding's kind. It imports `time` for `time.Parse` alone.

```go
// Input is what one audit reads.
type Input struct {
	Facts       *derive.Facts      // the facts of the project in view
	Diagnostics []model.Diagnostic // the Loader's and the Validator's, as snapshot.go sorts them
	Now         string             // the date staleness counts from, YYYY-MM-DD; "" is the source commit's author date
	Stale       int                // the age in days; 0 is DefaultStale, 7
}

// Run audits a project. It returns nil when in.Facts is nil or refuses.
func Run(in Input) *Report

// Report is the audit of one project at one source. It is immutable.
type Report struct {
	Now      string    // the date staleness counts from, as resolved
	Stale    int       // the age in days, as resolved
	Findings []Finding // one per diagnostic, proposed task and stale status, in order
}

func (r *Report) Keep(keep func(*Finding) bool) *Report // the findings keep accepts
func (r *Report) Rows() []Finding                       // the tasks one kind hits at one gate for one reason, joined
func (r *Report) Counts() Counts                        // by severity, one per finding
func (r *Report) Most() *Resolving                      // who resolves the most
func (r *Report) Silent() []Silence                     // the kinds examined that no finding has
```

[`dada/model.md`](dada/model.md) declares the rest. `Run` does five things in order: it wraps each diagnostic by its row of the table; it raises `proposed` for each task in display order; it raises `stale` for each leaf in display order; it gives every finding its commits, facts and commands; and it sorts.

### What the Audit adds, finding by finding

The line stands as `8118` decisions 1 and 13 and `27a3` decision 8 draw it: a finding with an id in RULES.md is the Validator's diagnostic, and the Audit raises only what has no id. No finding has two owners. The first six rows are the task description's list.

| Finding                                         | Raised by                | The Audit adds                                                     | From `derive.Facts`                                   |
|-------------------------------------------------|--------------------------|--------------------------------------------------------------------|-------------------------------------------------------|
| A proposed task                                 | The Audit, `proposed`    | The whole finding                                                  | `Authorisation`, `Authorities`, `Owner`, `Trunk`, `Order` |
| A status past an unreviewed junction (S11)      | The Validator            | `review` by the junction's reviewer; the status's deciding commit  | `Junction`, `Status`                                  |
| An unmet requirement (R9, R13)                  | The Validator            | `advance` by whoever works on the originating task; `move_pin` by the assignee | `Requires`, `Chain`, `Leaf`, `Next`, `Junction`, `Pin` |
| A stale status                                  | The Audit, `stale`       | The whole finding                                                  | `Status`, `Log`, `Next`, `Junction`                   |
| An agent contribution without a human reviewer  | Nobody: no finding (D1)  | The action of H4 and the Junction fact say that the contributor reviews its own work | `Handoff`, `Junction`               |
| A subproject that does not resolve (J8, J9), a pin or `commit` off its place (J13 to J17) | The Validator | `checkout` by nobody or `revise` by the keeper; `move_pin` by the assignee; the Pin fact | `Junction` and its `Snapshot`, `Requires`, `Pin` |
| A trailer that names nothing (H1)               | The Validator            | `none`, the commit's author                                        | `Log`                                                 |
| A review from the wrong hand (H2)               | The Validator            | `review` by the reviewer until a review accepts, then `none`       | `Accepted`, `Junction`                                |
| A model outside the one stated, or none (H3, H6) | The Validator           | `none`, the commit's author; the Model fact                        | `Events` and each `Reading`                           |
| A hand-off the history implies, or a stale one (H4, H5) | The Validator    | `record_handoff` or `clear_handoff` by the contributor             | `Handoff`, `Junction`, `Status`                       |
| An undetermined trunk (P5)                      | The Validator            | `revise` by the owner; P5 stands for every proposed task           | `Owner`, `Trunk`                                      |
| Every other rule                                | The Loader, the Validator | `revise` by the keeper of the file                                | `Authorities`, `Status`, `Owner`                      |

The Audit derives nothing itself. It compares a date with `Now`, which `27a3` gives it to do, and it finds the requirement or the link a diagnostic concerns by the diagnostic's position, since a diagnostic carries no index (A5). `Unread`, `Reviews` and each event's `Effect` reach it only through the Validator's diagnostics.

### The two kinds

**A proposed task.** For each task of `Order()`, with `a := Authorisation(id)`:

- No history, or no trunk (`Trunk().Tip` is empty): no finding. Without a trunk rule P5 already says that every task reads as proposed, so one fault gives one finding.
- On the trunk: a finding when `a.Authorised` is false, with the deciding commit, or with `NoCommit` for a task file that no commit of the trunk holds, as an untracked file in the working tree is.
- Off the trunk (`a.Why` is `OffTrunk`): a finding only when `a.Differs`, so a branch reports the tasks it adds or changes and stays silent about the plan it inherits (D6).

The resolver is the task's nearest authority in view, since a new `Authorised:` commit reads the tree as it then stands; for the root it is the judge of the deciding commit, the outgoing owner (`27a3` decision 3). An authorised task whose file has an uncommitted edit gives no finding: the edit is no commit yet.

**A stale status.** For each leaf of `Order()`, with `s := Status(id)`: a finding when `s.Kind` is `Recorded`, `s.Commit` is not nil, `s.Uncommitted` is false, `s.State` is not `complete`, and the days from `s.Date` to `Now` exceed `Stale`. Both are calendar dates, so the count takes no zone: a status dated 2026-09-22 is stale at three days on 2026-09-26 and not on 2026-09-25. `Status.Date` already is the newest of the file's change and a `Reaffirmed:` commit, so "no later reaffirmation" needs no second look. The resolver is the recorder, the person whose queue lists the reaffirmation (VIEWS.md, the queue's fourth kind). A leaf with no status file has recorded nothing to confirm; a snapshot's date is the subproject's, and that project's own audit reports it (`8118` decision 11); a complete task waits for nothing (D5).

[`dada/words.md`](dada/words.md#the-two-kinds) gives each field of both.

### The clock

The package reads no clock. `Stale` 0 means seven days, as VIEWS.md's Q6 rules. `Now` empty means the author date of the source commit, `Log().Commit(Log().Source).Date()`; with no history nothing is stale. A `Now` that is no date gives no stale finding and no `stale` among the kinds examined; `Params.Check` already refuses one. The command keeps its default as `4ed9` decision 7 accepts it: `cli.Run` fills `--now` with the host's local date, so `tablo audit` moves with the day. A library caller that passes nothing gets an audit that is a function of the commit, which is what the static export wants (6160 decision 8), and `Report.Now` and `Report.Stale` give the envelope the values to echo (D7).

### The action and the resolver

An **act** is the kind of an action, a closed list of ten keys; the **action** is its worded text. Eight acts are the list of VIEWS.md#audit: `authorise`, `review`, `reaffirm`, `record_handoff`, `clear_handoff`, `revise`, `move_pin`, `checkout`. Two are new (D2): `advance`, for an unmet requirement, which resolves when the originating task passes a gate and by no edit of the file that reports it; and `none`.

`none` is the act of H1, H3, H6 and of an H2 whose junction a review has since accepted. Each reports a commit, the files at a commit judge the commit (`27a3` decision 2), and a commit does not change: no later commit and no edit of the plan removes the finding. Their action reads "Nothing resolves it: the commit stands in the history", and their resolver is the commit's author, who learns of it (D3). The audit mockup draws "Revise the file: state the model that ran" for H3; under decision 2 that revision changes the next commit's reading and leaves the finding. The mockup stays as drawn.

The **resolver** is one email, or empty where the files name nobody. Each rule names a role ([`dada/rules.md`](dada/rules.md)), and a role resolves as follows (D4); `id` and `gate` are the diagnostic's.

| Role        | The email                                                                                                                              |
|-------------|----------------------------------------------------------------------------------------------------------------------------------------|
| owner       | `Owner()`                                                                                                                              |
| assignee    | The task's assignee as its file states it; the owner for an id that names no task or a task with none                                  |
| keeper      | By the diagnostic's file. `tasks/<id>.yaml` of a task of the project: the first of `Authorities(id)`, the owner for the root, since an authority's commit is proposal and acceptance at once. `status/<id>.yaml` of a task of the project: the recorder. Any other file, or none: the owner |
| recorder    | `Status(id).Recorder`; when it is empty, the contributor at `Next(id, Status(id).Gate)`                                                |
| contributor | `Junction(id, gate).Contributor.V` of a plain junction; else the assignee                                                              |
| reviewer    | `Junction(id, gate).Reviewer.V`; else the assignee                                                                                     |
| author      | The author's email of the commit the diagnostic names; the owner when the pass holds no such commit                                    |
| origin      | Of the condition the diagnostic stands at, in the originating project's facts (`Condition.Facts`): the first status of `Chain(Origin)` whose task is a leaf there gives the contributor of that leaf's next junction when it is plain, else the leaf's assignee. With no condition or no such leaf: the assignee |
| nobody      | The empty string: `checkout` falls to whoever holds the clone                                                                          |

The **condition** a diagnostic stands at is the entry of `Requires(id)` whose `Entry.Node`, or whose `Subproject` node, `URL` node or `Commit` node, has the diagnostic's position. The **link** a diagnostic concerns is `Junction(id, gate).Snapshot.Link` when it names a gate, and that condition's `Link` when it names none.

### The finding record

`Finding` is the one record, for both a single finding and a row. Its JSON keys, in order: `key`, `rule`, `severity`, `kind`, `tasks`, `gate`, `files`, `message`, `act`, `action`, `resolver`, `sentence`, `source`, `facts`, `commands`; the audit view adds `commits` last (A4). Every key is always present, a string that does not apply is empty, and no list is null. [`dada/audit.json`](dada/audit.json) is the data of one audit.

**Beside the diagnostic.** The envelope's `diagnostics` stay as the Validator sorts them, and the audit changes neither them nor the exit code (`4ed9`, the exit codes). Each diagnostic gives exactly one finding, which holds a pointer to it and copies five of its fields: `rule` is its code, `severity` its severity, `tasks` its task, `gate` its gate, and `message` its message byte for byte, so a consumer joins the two lists on those. `files` holds `Pos.File`, below `.tableaux`, where the envelope writes `path` with the prefix. `kind` is the rule's `Title` from `validate.Rules`. The two kinds have an empty `rule` and a `key` of their own, `proposed` and `stale`; for a rule finding `key` equals `rule`. The Audit raises no finding under a rule id (`8118` A10).

**Order.** `Findings` sorts by: severity, errors first, then warnings, then information; the kind, in the order of `validate.Rules`, then `proposed`, then `stale`; the gate, none first, then the order of `gates.yaml`, then any other key by bytes; the task, none first, then display order, then any other id by bytes; the oldest commit, none first, then the oldest first; the diagnostic's file, line and column; the message; the trailer. The sort is stable over the order `Run` raises in.

**Rows.** `Rows` joins findings that agree in `key`, `severity`, `gate`, `message`, `action` and `resolver`. A finding with one task joins the first row of its key that does not hold that task; a finding with no task never joins. A row appends the task, each file once, each fact once and each commit once, the oldest first, and keeps the commands and the diagnostic of its first finding. So the twenty tasks that require one task at one gate make one row, and two commits that carry one unread trailer make two.

**Counts and most.** `Counts` counts the findings of `Findings`, not the rows: one per task a kind hits and one for a finding with no task, so the three counts sum to the number of diagnostics plus the proposed tasks and the stale statuses. `Most` counts findings per resolver and skips the empty one.

**Silent.** `Silent` lists, in the order above, each kind the run examines and no finding of the report has: the rules VIEWS.md#audit names (P5, R9, R12, R13, J8, J9, J13 to J17, H1 to H6), `proposed` and `stale`, each with its key, rule and kind. A run examines a rule of `validate.TierHistory` only with a history, `proposed` only with a trunk, and `stale` only with a date. It words no reason for a silence.

**Provenance.** `sentence` and `source` come from the rule's row: the sentence RULES.md quotes from README.md or SYNTAX.md, or the rule as RULES.md words it where a schema alone states it, and the address of the section, absolute. `facts`, `Commits` and `commands` follow [`dada/words.md`](dada/words.md). A finding holds all three levels; the view drops what a level does not show (A3).

### Subprojects, errors, and what is deterministic

The Audit reports the project in view. It follows no link: a subproject's proposed tasks and stale statuses are findings of its own audit, as its diagnostics are (`8118` decision 11). `Run` returns no error and never panics; a diagnostic whose task, gate or commit the project lacks takes the fallback its role names. With a refusal it returns nil, and `data` is null as `27a3` decision 6 rules. The same input gives equal reports on every host: every list has a stated order, no map is ranged over unsorted, and the package reads neither the clock nor the environment.

### From the prototype

[`prototype/dada`](../prototype/dada/README.md) shows that findings with a task, a gate, a commit and an action come from one reading of the history. The design keeps its finding shape and its action for a proposed task and a missing review. It drops its own `git log`, its YAML reader, its stand-ins for the Loader and the Derivation, and its S11, H1, H2 and H3, which the Validator now raises. Its six questions have these answers.

| The prototype asks                                             | The answer                                                                                              | Settled by                                   |
|----------------------------------------------------------------|---------------------------------------------------------------------------------------------------------|----------------------------------------------|
| Is `c07d` in `weather-station` a corpus error?                 | Yes: W11 now carries `Reviewed: c07d mockup`, and the audit reports nothing there                       | The ruling of 2026-09-30, tableaux `1b0c002` |
| How does a `Model:` trailer attribute over several tasks?      | One reading per junction the commit is at                                                               | `27a3`, the events; `8118` decision 15       |
| Is a `Reviewed:` or `Reaffirmed:` commit work at the gate?     | `Reaffirmed:` is at the task's next junction; `Reviewed:` is at none; an edit of a task file is at none | README.md#junctions; tabloio `b618` decision 8 |
| Does H2 report at a junction with no reviewer, and at `defined`? | No with no reviewer. At `defined` the Validator on `main` reports it; see the findings below           | `8118` decision 10; the code of `8118`       |
| Is an agent a junction with a `model`?                         | Yes                                                                                                     | README.md#junctions; `Junction.Marks`        |
| Which commit does S11 name?                                    | The diagnostic names none; the finding's commit is the status's deciding commit, as the prototype has it | This design, [`words.md`](dada/words.md#commits) |

## Conformance to the texts

The Audit implements VIEWS.md#audit: its Data paragraph, finding by finding in the table above; its `stale` parameter and the seven days; "each finding names the action that resolves it and the person who takes it". It implements README.md#proposed-and-authorised-tasks for the proposed task and README.md#status, "Date and recorder", for the stale one, and it shows the sentences of RULES.md. It draws no view. Where a text is silent, a mockup is stale or two texts disagree, the design reads as follows.

- **The acts.** VIEWS.md lists eight and gives an unmet requirement none; the example's "Resolves when" column reads "`e9c6` passes design". D2 adds `advance` and `none`.
- **"The action that resolves it"** has no answer for H1, H3 and H6 (D3). An old project keeps every such finding for as long as its history stands.
- **An agent with no human reviewer** is in the task's description and in no rule and no sentence of VIEWS.md. README.md#junctions lets an agent review its own work and gives the junction the contributor's mark alone (D1).
- **Which status is stale.** VIEWS.md says "a status whose date is older than an age" and does not exclude a complete one, a snapshot or a leaf with no file (D5).
- **Who reaffirms.** PLAN.md gives reaffirming to the contributor and VIEWS.md lists reaffirmations for "statuses the person recorded". The design takes the recorder (D4).
- **Off the trunk.** README.md says every task is proposed there. The fact stands in `Authorisation`; the finding is narrower (D6).
- **The audit mockup.** `docs/mockups/audit.md` draws a "Silent" section with a worded reason per rule, "As above" in an action cell, H3 at `defined` for an edit of a task file, an alternative in the action of S11 and `status/<id>.yaml` for a row. The data holds the silent kinds without reasons, each action in full, one action per finding and every file. The mockup stays as drawn.
- **The corpus.** No entry has an `audit` section and none states a stale status, so the corpus fixes the proposed task alone, as the fact `authorisation.state` (RULES.md, "Derived facts, not rules"): `weather-station` (`3c5d`, and every task on `sensor-board`), `trunk-inferred` and `trunk-stated` (`c3d7`), `trunk-undetermined`. The findings the Audit wraps have their entries in RULES.md. The corpus exercises no self-review under H4, no H2 that a later review accepts, no J8 for a missing clone and no valid project with a complete status; tests T7 to T9 build these.

## Assumed interfaces

What other designs state about this task:

| Design, row                            | Verdict   | Detail                                                                                                                               |
|----------------------------------------|-----------|--------------------------------------------------------------------------------------------------------------------------------------|
| tablo `8118` A10, decisions 1 and 13   | Confirmed | A rule finding carries its diagnostic unchanged, and the Audit raises nothing under a rule id                                         |
| tablo `27a3` A3, as decision 8 amends it | Confirmed | No history of its own; `Authorisation` and `Status.Date` for the two kinds. It also reads `Handoff`, `Accepted`, `Pin`, `Requires` and `Events` to word an action or a fact |
| `27a3` decision 3                      | Confirmed | The resolver of a proposed root is the judge of its deciding commit                                                                   |
| tablo `4ed9` A8                        | Amended   | `audit` carries `findings`, takes one ref and no range, and counts from `Now` over `Stale` days. `Now` is not required of a library caller: empty is the source commit's date (D7), and the comment in `params.go` changes |
| `4ed9`, the exit codes; decision 7     | Confirmed | A finding of the two kinds changes no exit code; `cli.Run` defaults `--now` to the host's date                                         |
| tablo `4b4f` A3, decision 3            | Confirmed | `validate` alone reports every rule; the conformance run compares no audit data, since the corpus states none                          |
| tabloio `9167` A3                      | Amended   | The data holds rows in the stated order with the fields of `view.Finding`, and two more keys, `key` and `act`. A count is one per task of a row and one for a finding with no task; the renderer's glance sums `len(tasks)` and so counts H1 and P5 as none |
| `9167` A4                              | Amended   | `kind` is the rule's title. `message` of a rule finding is the Validator's, lower case and without backticks; `action`, the facts and the two kinds' messages are worded text. `files` stand below `.tableaux`. `source.url` is absolute |
| `9167` T10                             | Amended   | On `weather-station` at `main` the proposed task is `3c5d`, and it is stale only below six days; the fixtures regenerate at the integrate gate |
| tabloio `5ca9` A11                     | Confirmed | An absent age is seven days. The command counts from today; the library, given no date, from the commit                               |
| tabloio `b618` A11                     | Confirmed | H3 and H6 arrive from the Validator, which reads `Reading.Gate`                                                                        |
| tableaud `9a9c` A8                     | Amended   | One record holds every item the row lists; the kind of action is `act`. A group's anchor takes `key`, since `rule` is empty for two kinds, and its titles gain "Advance" and one for `none` |
| `9a9c` A9                              | Amended   | The silent kinds arrive with key, rule and kind and no sentence. Staleness counts from the date the Server passes, or from the commit when it passes none |
| `9a9c` A4; tableaud `438a` A12         | Amended   | The model check and a review's effect are the Derivation's, through the history and the task view; the Audit gives those views nothing |
| tableaud `49ce` A7, A3                 | Amended   | The audit reads no clock at all: it takes the date from the request, and the commit's when the request has none. One `Report` holds the three levels |
| tableaud `6160` A9, decision 8         | Confirmed | The export passes the commit's author date, or nothing                                                                                |
| tablotui `171b` A11                    | Confirmed | A new day changes an audit only through the `Now` its caller passes                                                                    |

What this design assumes:

| #   | Of                      | Assumption                                                                                                                                            |
|-----|-------------------------|-------------------------------------------------------------------------------------------------------------------------------------------------------|
| A1  | tablo `4ed9`            | `snapshot.go` calls `audit.Run` once per snapshot with the `*derive.Facts`, the sorted `[]model.Diagnostic` it holds before it writes the envelope's, and `Params.Now` and `Params.Stale` as given. It passes `validate.Derived` only the conditions whose `Why` is `Determined` |
| A2  | tablo `4ed9`            | `data` is null when `Run` returns nil. The resolved `params` echo `Report.Now` and `Report.Stale`. `cli.Run` alone reads the clock                      |
| A3  | tablo `8ed1`            | The `audit` deriver selects with `Keep`: a task's subtree by `Tasks`, a finding with no task under the root alone, and a person by `Resolver`. It then writes `findings` from `Rows`, `errors`, `warnings` and `information` from `Counts`, `most` from `Most` and `silent` from `Silent`, and empties `sentence`, `source`, `facts`, `commands` and `commits` below provenance. It derives nothing |
| A4  | tablo `8ed1`, `493e`    | The view tasks own the commit fact every view shares. The audit view writes each row's `Commits` as that fact under `commits`, after the keys of `Finding`, through a struct that embeds it |
| A5  | tablo `8118`, on `main` | R9, R13, J8, J13 and J17 stand at a node of the entry or of its `subproject`, as `derived.go` and `junctions.go` place them; H4 and H5 stand in the status file where the leaf has one |
| A6  | tablo `493e`, `886d`    | The queue lists a reaffirmation for `Status.Recorder`, so the resolver of a stale finding finds it in the queue                                         |
| A7  | tablo `4b4f`            | `corpustest` gives the built entries and their labels; until it stands the tests copy the stand-in of `internal/derive`                                 |
| A8  | tabloio `9167`, tableaud `9a9c` | Each adapter reads `key` and `act` and prints a message as it arrives; both objects stay open, so the two new keys break no reader              |

## Tests

Go tests in `internal/audit`. They read the built corpus through the Loader, the history reader and `derive.New`, read-only, or build a repository under `t.TempDir()` with the fixed identity of the Derivation's tests. None uses the network, a browser or the clock: every test passes `Now` or takes the commit's date.

| #   | Proves                                             | How                                                                                                                                         |
|-----|----------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------|
| T1  | The table is total and true to the texts           | `table` has a row for every rule of `validate.Rules` and no other; every act is one of the ten; every sentence stands in that rule's row of the corpus's RULES.md, or is its Rule column; every source address starts with the method's |
| T2  | The Audit on the corpus                            | [`corpus_test.md`](dada/corpus_test.md) against [`corpus.expected.txt`](dada/corpus.expected.txt): 24 entries, 18 refusals, and the stale statuses at three days |
| T3  | A proposed task agrees with the corpus (D6)        | For each valid entry at each ref it states: on the trunk, the `proposed` findings name exactly the tasks whose `authorisation.state` is `proposed` (`3c5d`; `c3d7` in `trunk-inferred`); `weather-station` at `sensor-board` gives none, `trunk-stated` at `main` gives `c3d7`, `trunk-undetermined` gives P5 and none |
| T4  | Staleness and the clock (D5, D7)                   | `weather-station` at `main`: with no `Now`, `Report.Now` is 2026-09-28; at five days `3c5d` is stale and at six it is not; with `Now` 2026-10-06 `3c5d` and `9f31` are, and `c07d`, a snapshot, and `7b2e`, with no file, never are; a `Now` of `tomorrow` gives none and no `stale` in `Silent` |
| T5  | Each role resolves                                 | Literal diagnostics on `weather-station` and `junction-kinds`, one per role and per fallback of the table: an unknown task, the root, a parent's status file, a commit the pass lacks, a requirement on a parent |
| T6  | The order                                          | A shuffled slice of literal findings sorts to the stated order; two runs give deep-equal reports                                             |
| T7  | The acts that take a condition                     | A built repository: an H2 before and after the reviewer's commit gives `review` then `none`; an H4 where the agent is its own reviewer gives the self-review text; `submodule-subproject` cloned without its submodule gives `checkout` with no resolver and the submodule command, and `subproject-path-missing` gives `revise` |
| T8  | The working tree                                   | In a copy of `weather-station`: an untracked task file is proposed with `NoCommit`; an edited status is not stale; an edited authorised task gives no finding |
| T9  | A complete status and a refusal                    | A built repository with a status at its last gate, complete, a year old: no stale finding. `gates-missing`: `Run` returns nil. A project composed in memory with no pass: rule findings alone, and `Silent` holds no history rule, `proposed` or `stale` |
| T10 | Rows                                               | Twenty literal R9 findings of one message make one row of twenty tasks and count twenty; two findings of one task make two rows; a finding with no task never joins; files, facts and commits appear once |
| T11 | Counts, most, keep and silent                      | Over the report of T2 for `weather-station`: the counts, `ada@example.org` with two, `Keep` by resolver and by task, and `Silent` before and after                |
| T12 | The JSON                                           | `weather-station` at `main` marshals to [`audit.json`](dada/audit.json) through the wrapper of A4 with full hashes: the keys in order, no null                    |
| T13 | Pure, deterministic and safe to share              | Two clones at two paths under two `TZ` values give equal bytes for T2; eight goroutines call the methods of one `Report` under `-race`; the package's source names neither `time.Now` nor `os.` |

**Tried.** A throwaway trial of about 800 lines, in a copy of the module at `250ef85` outside the repository, implements every exported declaration of [`model.md`](dada/model.md) and the table of [`rules.md`](dada/rules.md). It compiles and vets, and it wrote [`corpus.expected.txt`](dada/corpus.expected.txt) and [`audit.json`](dada/audit.json); two runs under two `TZ` values give the same bytes. On the corpus it gives `3c5d` proposed for `ada@example.org` with W12, `c3d7` in `trunk-inferred` and off the trunk in `trunk-stated`, no proposed task on `sensor-board` or beside P5, and `3c5d` stale at three days for `dan@example.org` with W8. A script over RULES.md writes the sentence and the source of all 81 rows.

**Unverified.** No adapter fills `validate.Facts` from `derive.Facts` before `4ed9`, so the trial takes the diagnostics of the ten history rules from `expected.yaml`, with the corpus's messages and no positions: the Pin and Requirement facts of R13 have no trial. T5 to T10 and T13's race test have no trial. The trial does not run `checkout`, the self-review text, an accepted H2, the working tree or a complete status. `tableaux-tooling`, which the audit mockup draws, did not run. Nothing ran on macOS or Windows.

**Later gates.** Unit: T1 to T13 pass. Integrate: `4ed9`'s adapter gives real history diagnostics, the golden file takes the Validator's messages, and `8ed1`'s view passes its data to the three front ends, whose fixtures regenerate. Validate: the owner reads the audit of the tooling plan against the plan.

## Implementation notes

- **Order.** `rules.go` with the table and T1; `audit.go` with `Run`, the roles and T5; the two kinds with T3, T4, T8 and T9; `words.go` with the facts, the commits and the commands, and T2 and T7; the order and T6; `report.go` with `Keep`, `Rows`, `Counts`, `Most`, `Silent` and T10 to T13. Each step builds alone.
- **Files.** New: `internal/audit/{audit,rules,words,report}.go`, their tests, `helpers_test.go` and `testdata/` from the drafts. Changed: `README.md` gains the package in its layout; the comment on `Params.Now` in `params.go` reads `"" is the source commit's author date`.
- **The prototype.** `prototype/dada/` goes when the task records `implementation`, as the README states.
- **`go.mod`.** No new dependency.
- **The plan.** The `requires` entries are right. `8ed1` requires this task at design and may start.
- **Findings for other tasks**, none edited here:
  - `derive.Condition.Why` is `NoGate` only for a key that `gates.yaml` lacks. A `from` gate that does not apply to the originating task leaves the condition determined and due, so an adapter that passes every condition draws R9 beside R6 on `requires-from-not-applicable`, which states R6 alone. A1 asks `4ed9` to pass determined conditions; the doc comment of `Why` says "or none applies" (`27a3`).
  - At `defined` `derive` gives every `Reviewed:` trailer the effect `AtDefined`, and `validate` raises H2 there for a wrong hand. The history then shows a review the authorisation stands for, and the audit a warning about the same commit (`27a3`, `8118`).
  - `27a3` decision 3 judges a hand-over of the root by the owner before the deciding commit. A second commit by the new assignee with `Authorised: <root>` has the new owner at its first parent and authorises the root. The scheme is audit-only, and the Audit reports the first commit only until the second lands.
  - tabloio's fixture `audit/in/weather.json` names `9f31` proposed; the corpus has `3c5d`.

## Decisions at review

1. **Two kinds, and none for an agent that reviews its own work.** The Audit raises `proposed` and `stale`, both warnings, and says of a self-review only what the action of H4 and the Junction fact say. The alternative follows the task's description and reports, as information, every junction where an agent contributes and no other person reviews, which README.md#junctions allows and marks. Binds `8ed1`; the description stays as authorised.
2. **Ten acts.** The eight of VIEWS.md#audit, as the keys `authorise`, `review`, `reaffirm`, `record_handoff`, `clear_handoff`, `revise`, `move_pin` and `checkout`, with `advance` for R9 and `none`. The alternative keeps eight and files an unmet requirement under `revise`, though no revision of the file that reports it is meant. VIEWS.md#audit gains the two. Binds `8ed1`, tabloio `9167`, tableaud `9a9c` (its group titles).
3. **Nothing resolves H1, H3, H6 or an accepted H2.** Each stands with the act `none` and the commit's author as resolver. The alternative draws "Revise the file" as the mockup does, which names a resolver who cannot clear the finding; or the method ages such findings out, which is a rule for `8118` and README.md. Binds VIEWS.md#audit, whose sentence promises an action for each finding.
4. **Who resolves.** The nearest authority keeps a task file, the recorder a status file and a stale status, the owner everything else; the contributor hands off, the reviewer reviews, the assignee moves a pin, and nobody is named for a checkout. The alternative gives a task file to its assignee, who can only propose the fix. Binds `8ed1` (the `person` parameter), `886d` (A6).
5. **Which status is stale.** A recorded, committed status that is not complete. The alternative counts every leaf, so a finished project reports each complete task every week and a parent project reports its subprojects' dates. Binds VIEWS.md#audit, which gains the three exclusions.
6. **Off the trunk, the tasks the branch changes.** A branch reports as proposed the tasks whose file differs from the trunk's tip, and an undetermined trunk reports P5 alone. The alternative reports every task, as README.md states the fact, and a branch of a hundred tasks then shows a hundred warnings for one proposal. Binds `8ed1`.
7. **No date means the commit's date, in the library.** `Run` with an empty `Now` counts from the source commit's author date; the command still defaults `--now` to today (`4ed9` decision 7). The alternative makes `Now` required of a library caller, as `params.go` words it, and a front end that forgets it gets a usage error. Binds `4ed9`, tabloio `5ca9` (A11), tableaud `49ce` (A7) and `6160`, tablotui `171b`.
8. **The record.** One type, `Finding`, with the keys of tabloio's `view.Finding` and two more, `key` and `act`; a rule finding's `message` is the Validator's, byte for byte; the address of a source is absolute; a row joins tasks by `key`, `severity`, `gate`, `message`, `action` and `resolver`; a count is one per finding, a finding with no task included; `silent` lists kinds and words no reason. The alternative rewords each message for the audit with backticks and a capital, and then the audit and `validate` say one finding two ways. Binds `8ed1`, tabloio `9167` (A3, A4), tableaud `9a9c` (A8, A9).
