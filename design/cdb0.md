# Design cdb0: Language 0.4

tablo reads the language at 0.4.0, which the umbrella's method designs [`7166`](https://github.com/nbyoung/tableaux/blob/main/design/7166.md), [`9f3f`](https://github.com/nbyoung/tableaux/blob/main/design/9f3f.md) and [`ac33`](https://github.com/nbyoung/tableaux/blob/main/design/ac33.md) compose and the branch `wave/language-0.4` states at [`1d1cb02`](https://github.com/nbyoung/tableaux/tree/1d1cb02b1e1bb7bbc68ede0a765edc78e6895722). The module accepts minor 4, embeds the six schemas of that commit, and reads `agents.yaml`, the task-level `subproject` field, `reviewer: false` and the unpinned requirement. The Validator gains seven rules; the Derivation reads the identity kind, the waiver, the nearest person and the field, and gains one event; the Audit gains seven rows and two acts; and the views gain four words: `nearest`, `co_author`, `state_commit` and `raise_language`. Five choices shape it. One set of files reads at every accepted version, and the version at a commit decides only the agent's reading and two exemptions, through one helper in `internal/model` (D1). The nearest person is a supplier of its own beside the assignee, and the views name it with the words they have, but for one new source kind (D2). The agent a person's commit names in `Co-Authored-By:` stands beside the model on every model check and history line (D3). The audit's two new actions are acts of their own (D4). The corpus entry `review-nearest-person` omits a review its own default implies, and tablo lists it as a known failure owed to the corpus (D6). D*n* is decision *n* under [Decisions at review](#decisions-at-review).

| Draft | Lands as | Holds |
|-------|----------|-------|
| [`cdb0/model.md`](cdb0/model.md) | The declarations of seven packages | Every type, field, function and doc comment that changes |
| [`cdb0/rules.md`](cdb0/rules.md) | Rows of `validate.Rules`, the rule functions and `audit.table` | Each new or amended rule: tier, condition, position, fields, message, entries; its act, resolver and sentence |
| [`cdb0/crossing.sh`](cdb0/crossing.sh) | `internal/derive/testdata/crossing.sh` | A repository of six commits whose language rises from 0.3.2 to 0.4.0 |
| [`cdb0/crossing.expected.txt`](cdb0/crossing.expected.txt) | `internal/derive/testdata/crossing.expected.txt` | What the Derivation gives and the Validator reports at each of its labels |

## The model

### Where the work lands

| Step | Task's code | Packages | What changes |
|------|-------------|----------|--------------|
| 1 | `4f60`, `8118`, `dada`, `4b4f` | `internal/model`, `schemas`, `internal/validate`, `internal/audit`, `internal/corpustest`, `internal/conformance`, root | The version, the schemas, the rule and audit tables, the lock |
| 2 | `4f60`, `8118` | `internal/load`, `internal/validate` | `agents.yaml`, A1, A2, T4 |
| 3 | `4f60`, `8118`, `27a3`, `493e` | `internal/model`, `internal/load`, `internal/validate`, `internal/derive`, `internal/views` | The task-level field, J18, J19 |
| 4 | `27a3`, `8118`, `4ed9`, `493e`, `886d`, `8ed1`, `4b4f` | `internal/history`, `internal/derive`, `internal/validate`, root, `view`, `internal/views`, `internal/conformance` | The agent's identity, the nearest person, the waiver, H7 |
| 5 | `4f60`, `8118`, `27a3` | `internal/model`, `internal/load`, `internal/validate`, `internal/derive` | The unpinned requirement, R14 |
| 6 | `27a3`, `8118`, `8ed1`, `4ed9` | `internal/derive`, `internal/validate`, root, `internal/views` | The merge's `authorised` event, H8, the third command |

[`cdb0/model.md`](cdb0/model.md) declares everything below; this section says what each piece decides.

### The version at the commit

`model.AcceptedMinor` becomes 4, so `0.4.x` passes and `0.5.0` draws P4; `tablo.AcceptedMinor`, the `version` command and the envelope follow the constant. Beside it `internal/model/version.go` gains `Release`, the two releases from which a reading changes, `ModelTrailer` (0.2.1) and `AgentIdentity` (0.4.0), and `Before`, on a `*Version` and on the text a commit's `version.yaml` states. A nil or malformed version precedes nothing, so the newest reading applies to it, as `before021` does today. `before021` in `internal/derive/events.go` and the inline test of `h6` in `internal/validate/derived.go:393` give way to `Before(ModelTrailer)`.

Each reading that changes asks the version of the files it already reads, so no new plumbing carries a version (D1):

- **The junction** (`derive.resolve`) asks `f.p.Version`: the project in view, or, for `At(commit)`, the project as `Log.Project` composes it at that commit. Before `AgentIdentity` a junction is an agent's only where it states a model, and an agent with no reviewer takes the assignee; from it, `Junction.Agent` also holds where the contributor's identity is an agent's, and the default reviewer is the nearest person. Every fact that reads a junction at a commit, the reviewer a `Reviewed:` commit must be (`review.go:118`), the reading of a status commit and the junctions `commitsOf` gives the Validator, therefore reads the version of that commit. A review the old language accepted stays accepted after the raise.
- **H6 and H7** ask `Commit.Tableaux`, which `commitsOf` already fills.

Nothing else asks the version. The new forms, the agents file, the task-level field, the waiver and the unpinned entry, read the same at every accepted version, and so do F11's event, H8, J18, J19 and R14. The test is [`crossing.sh`](cdb0/crossing.sh): one history whose `version.yaml` rises from 0.3.2 to 0.4.0, where bot's self-review at X2 stays accepted, pat's agent work at X3 is exempt, and from X4 olive reviews as the nearest person and X5 draws H7.

### The Loader (`4f60`)

- **The agents file.** `layout` names `agents.yaml`, so L4 no longer reports it; `parse` builds `model.Agents`, one `StrField` per item, and `Compose` sets `Project.Agents`. L1 to L3 apply to it as to any file. `internal/history` composes it at each commit through `Parse`, so the kind of an identity reads the files at the commit. The schema is the Validator's (A1).
- **The task-level field.** `buildTask` reads `subproject` into `Task.Subproject`; every `Subproject` records its holder (`OnJunction`, `OnRequirement`, `OnTask`); `subprojects` lists a requirement's, then the task's, then a junction's, so the field resolves with the others. The schema at `1d1cb02` accepts the key, and T3 no longer fires.
- **The unpinned requirement.** A requirement's field on an absolute URL with no `commit` key is unpinned: its target key is `"tip\x00" + url`, apart from a link to the same URL at a commit, so a junction's field with no commit still draws BadCommit and J15. `resolveURL` finds the clone as for any URL ([below](#the-unpinned-requirement)).
- **The memo.** `reach` treats an unpinned link as one the memo does not keep, since its tip moves while the commit that names it does not. A load that reaches one reads its links again; every other project stays in the memo.

### The Validator (`8118`)

[`cdb0/rules.md`](cdb0/rules.md) gives each rule its row; 8118's decisions stand, D4's plain data included.

- **A1** runs the embedded `agents.schema.yaml` over the file after the gating rules; every leaf is A1. **A2** checks each plain entry with no schema leaf against `Agents.Lists`, at its `contributor`.
- **T4** takes every leaf under `reviewer` with one message, `… is neither an email address nor false`, so the two leaves of the `oneOf` give one diagnostic, as the requirement's `oneOf` does (`schema.go:440`); `true` is T4 and `false` passes.
- **The task-level field** takes the subproject rows with no gate, T6, J7 and J14 from the schema under `subproject`, T7 at `subproject.id`, and R12 reads it as a junction. `shape.kindAt` resolves it as the Derivation does, so S9 and S10 read it (8118 decision 13 d). **J18** fires on a parent; the field's other rows still run, as `check.py` runs them.
- **J19** compares, after J8 passes on a junction's or the task's field, the linked project's `version.yaml` with this one's by `Before`, once per task and `*Link` (`run.lagging`), at the `url` of the first field the run meets: the task's field, then the junctions in file order. It reads `Link.Project.Version`, the files at the commit the linkage fixes, and leaves a requirement out.
- **J15** skips a requirement's field; **R14** reports it as information, before J8, with the trunk and the tip in the message.
- **H7** reads three facts the Derivation supplies through `snapshot.go`: `Resolved.Agent` at the junction as the files stand at the commit, `Commit.AuthorAgent`, and `Commit.CoAgents`. **H8** reads `Event.Moot` on a `reaffirmed` event: `parent`, `recursive` or `absent`, as the files stand at the commit. Both run in `Derived` with a history.

### The Derivation (`27a3`)

- **The identity kind.** `Facts.Agent(email)` is true where `agents.yaml` lists the email or, with no file, where an entry of any task file states it as `contributor` beside a `model`; the pairing reads every entry, a parent's included, once per facts. `Junction.Agent` applies the version (above); `Marks` draws `agent`, and `Roles` grants `agent`, from it.
- **The waiver.** An entry whose `reviewer` YAML reads as `false` supplies the reviewer `Field{V: "", By: ByTask, Task, Node}`: a stated empty reviewer, which stops the walk for that field as any stated reviewer does, so a descendant restores review only by stating one. A `reviewer` YAML reads as `true` supplies nothing; T4 reports it.
- **The nearest person.** An agent's junction whose walk supplies no reviewer takes `nearest(task)`: the assignee when the assignee is a person (`ByAssignee`); else the first ancestor, nearest first, whose assignee is a person, as `ByNearest` with that ancestor in `Field.Task` and its `assignee` node; else the assignee (`ByAssignee`). The authorities are those of the files the facts read. `DefaultMarks` drops the reviewer where it is `ByAssignee` or `ByNearest`, since a child computes its own (D5).
- **The task-level field.** `resolve` returns a recursive junction, `Entry` the task and `Delegated` set, at each gate whose index in `gates.yaml` is 2 or more where the task's own file states no entry; an own entry of any form there walks as today, as `check.py`'s `resolve_kind` reads it. `resolved` takes the snapshot's field from the task; `fields`, and so `naming` and `commits`, add the field after the requirements', so a pin move of its path or its `commit` is a pin event.
- **The unpinned requirement.** The condition reads `Link.Project`, the project at the tip, so `AtTrunk` equals `Met` and R13 stays silent. `reach` in the past reads an unpinned field at `Link.Commit`, since the past holds no other tip. The `commit` a later file first states is a pin event with no `old`, as `moved` already gives it (`events.go:394`).
- **The merge that authorises (F11).** `ownEvents` adds, before the trailers, an `authorised` event for each task file in `Changes` of a commit with two parents or more whose `Trunk` is 0 or more, which is the deciding commit's test in `decide` (`authorise.go:186–199`); the dedup at `events.go:265` then keeps one event where the merge also carries the trailer. Status files keep the default log's reading, so the merger records nothing.
- **The co-author.** `history.Commit.CoAuthors` parses `Co-Authored-By: Name <email>`; `Reading.CoAuthor` names the first co-author that is an agent where the author is no agent, at every version. A reaffirmed event's `Moot` comes from `moot(c, id)` over `past(c.ID)`.

### The unpinned requirement

`resolveURL` maps the URL to a clone as it does today: `Options.Replace`, then `tableaux.<url>.path` in Git's configuration, then the cache. There `tip` reads `version.yaml` alone at the clone's default branch, `refs/remotes/origin/HEAD` else `HEAD`, as `listTree` reads one file, takes the trunk it names, else that branch, and resolves the name `refs/remotes/origin/<name>` first, then `refs/heads/<name>`, as 27a3 decision 5 orders every linked repository; `Link.Trunk` and `Link.Commit` record it. Under `Options.Fetch`, a URL with no mapped clone goes through `FetchTip`, which clones the repository bare or fetches every branch into the cached clone, once per load; the plumbing command never sets `Fetch`.

**With no network in a test**, the clone is a local repository: the corpus entry `requires-unpinned` maps its URL through `replace`, which the conformance runner passes as `--replace`, and the Loader's tests build a clone under `t.TempDir()`, with a `file://` URL where they exercise `FetchTip`. **When tablo cannot reach a clone**, the link has the problem `NoClone`; with a clone but no branch to read, the new problem `NoTrunk`, named `no trunk`. Either way the requirement draws J8, as any URL without a clone does (4b4f decision 6), and R14 beside it, and its condition is undetermined with `NoLink`, so no view marks it unmet and R9 stays silent.

### The Audit and the views (`dada`, `493e`, `886d`, `8ed1`)

- **The Audit.** [`cdb0/rules.md`](cdb0/rules.md#the-audit) gives the seven rows, the five sentences that RULES.md at `1d1cb02` rewords, and `silent`, which gains R14, J19, H7 and H8. `Act` gains `state_commit` for R14 and `raise_language` for J19 (D4). H7 and H8 take `none` and settle by the demotion as H6 and H1 do. A junction the waiver leaves without a reviewer is a sole review, as the owner ruled on 2026-10-07.
- **The history's third command (7166, implementation note 4).** `historyCommands` appends `git log --format='%H %as %ae %s' --first-parent --merges [<from>..]<trunk> -- <task paths>` where the trunk is determined. The effect `authorised_merge` (`history.go:15`) now appears without a trailer.
- **The `subproject` key on events.** tablo emits it on every taken event already (8ed1); the history schema at `1d1cb02` accepts it, and nothing else changes.
- **The words.** `ReviewerByDefault` holds for the nearest person too; a `ByNearest` field states `By` the ancestor's id at its `assignee`; a brief's source takes the kind `nearest` (D2). A waiver states the reviewer's supplier as the task's entry with the value `false`, and a brief's reviewer as `""` from that task. A delegated junction states its `subproject` supplier at the task's key `subproject`. `LineDetail.CoAuthor` and `ModelCheck.CoAuthor` carry the co-author (D3).

### Errors, and what is deterministic

No function gains an error. The unpinned read is the one fact that moves with another repository's refs: the same commit of this project reads another condition once that trunk moves, which R14 tells the reader. Every other fact stays a function of the commits read, and the passes keep their fixed command lines.

### From the umbrella's reading of tablo

Each "What tablo does today" paragraph of the three designs holds against `main` at `42e3081`, file by file and line by line, with four amendments:

- **9f3f F26, "27a3 Derivation: none for the condition".** Amended: `reach` reads a past at `field.Commit.V` (`derive.go:263`), which is empty for an unpinned field, so `At(commit)` needs the case above.
- **9f3f F4, the Validator.** Amended: beside `j3`, `subproject` and R12, `shape.kindAt` (`shape.go:162`) must read the field, or S10 fires at every delegated gate, the finding 9f3f's trial records for a tablo that does not read it.
- **9f3f F13 and ac33 F1.** Confirmed, with the data they need named: H8 reads `Event.Moot` and H7 the kinds, plain data from the Derivation (8118 decision 4).
- **7166 F11.** Confirmed; its line `history.go:191` is the function `effect`, and the constant `authorised_merge` stands at line 15.

## Conformance to the texts

The design implements, at `1d1cb02`: README.md#version (the accepted minor, J19), #tasks (the unpinned entry, R14), #junctions (the field, the waiver, the nearest person, the version rule), #status (H8, the undefined leaf's date), #history (F11's event and the third command) and #roles (the identity kind, co-authorship, H7); SYNTAX.md#layout, #agentsyaml, #tasksidyaml, #history and #commit-trailers; VIEWS.md's Models and Audit paragraphs and the ruling of 2026-10-19; and the rows of corpus/RULES.md. Where the texts are silent or disagree:

- **A form of 0.4.0 in an older project.** The texts do not say whether a project at 0.3.x may state the field, the waiver, the file or an unpinned entry. tablo reads them at every version and lets the version decide only what README.md#junctions says it decides (D1).
- **The pairing's reach.** README.md#roles says "a junction states it as contributor beside a `model`" and does not say whose junction. Any entry of any task file counts, a parent's included, since kind belongs to the identity.
- **A file that does not list.** README.md#roles makes the file decide wherever it exists. An `agents.yaml` that parses but fails A1 still decides, by its scalar items; one that L1 rejects gives no file, and the pairing decides.
- **The `Co-Authored-By` key's case.** GitHub writes `Co-authored-by`. tablo reads the key in the case SYNTAX.md writes, as 27a3's conformance note reads every trailer.
- **The corpus against README.md.** `review-nearest-person` states b2c9's default junction with the reviewer olive, and its `reviews` list omits the junction at `defined`, which that default makes a reviewed junction the status passes; README.md#status and `status-defined-by-authorisation` list such a review with the authorisation's commit (D6).
- **Two texts on J19's act.** 9f3f's body gives J19 the act `MovePin`; VIEWS.md#audit at `1d1cb02` and its ruling of 2026-10-19 name the action "raise a subproject's language". The ruling stands (D4).

## Assumed interfaces

What other designs state about the tasks this one changes:

| Design, row | Verdict | Detail |
|-------------|---------|--------|
| `8118` A2 | Amended | `Resolved.Reviewer` is the stated or inherited one, `""` where an entry waives review, else, at an agent's junction, the assignee before 0.4.0 and the nearest person from it; `Resolved.Agent` says the junction is an agent's |
| `8118` A12 | Amended | `schemas.FS` holds six files, `agents` among them, byte-equal to the umbrella's `schemas/` at `1d1cb02`, language 0.4.0 |
| `27a3` decision 12 (a) | Amended | `DefaultMarks` drops the reviewer where it is `ByNearest` too (D5) |
| `4b4f`, the observation table | Amended | `junctions.default` takes, beside the contributor, the reviewer of the first plain item of `data.junctions` whose `from` is empty (D7) |
| `4b4f` decisions on the lock | Confirmed | `corpus.sh refresh` moves the lock to `1d1cb02`; `tooling` stays at the tag `corpus/tooling`, `50a373b` |
| tabloio `e4c7` A3 | Amended | A field's `source` also takes the kind `nearest`, with the ancestor's id and title; an agent's reviewer is the nearest person from 0.4.0; a waived reviewer is `""` from a task source; an agent's junction may state no model, so the model line may be empty |
| tabloio `e3ed` A8 | Amended | `reviewer_by_default` also holds for the nearest person; a model check carries `co_author` |
| tabloio `9167` A2 | Amended | A line at detail carries `co_author`; the provenance holds a third command where the trunk is determined |
| tableaud `438a` A11 | Amended | A reviewer's supplier may be the ancestor whose `assignee` reviews, `Stated.By` its id and `Key` `assignee` |
| tableaud `9a9c` A4, A8 | Amended | An event of a person's commit names the agent it co-authors; the kind of action takes `state_commit` and `raise_language` |
| tablotui `0afa` A8 | Amended | A history line also carries `co_author` |
| tableaud `438a` A8 | Confirmed | The five mark keys stand; `agent` now follows the identity's kind from 0.4.0 |

What this design assumes:

| # | Of | Assumption |
|---|----|------------|
| A1 | the umbrella's `fcec` | The corpus at `1d1cb02` stays as it is until the lock next moves; a later commit that amends `review-nearest-person` removes the known item (D6) |
| A2 | the umbrella's CI and `4b4f` | The commit `1d1cb02` is on the umbrella's remote before tablo's CI runs `corpus.sh fetch`, which clones the remote and checks the commit out |
| A3 | tablotui `171b`, tableaud `49ce` | A watcher reloads on a change it sees; an unpinned requirement's other trunk moving shows at the next reload, as 27a3 A9 accepts for a subproject's trunk |
| A4 | tabloio `e4c7`, tableaud `438a`, `e2b6`, `9a9c`, tablotui `0afa`, tabloio `9167` | Each front end's language raise follows this task in a reconcile of its own and ignores a key it does not yet know, as the envelope's rule allows |
| A5 | tabloio `b618`, tablotui `518e` | A write command that records a status at an agent's junction under a person's identity draws H7 from tablo and needs no rule of its own |

## Tests

Go tests in the packages above; none uses the network, a browser or the wall clock. The corpus is the copy at `1d1cb02`, built as today.

| # | Proves | How |
|---|--------|-----|
| T1 | The version | `model.Before` over a table at both releases, a nil and a malformed version; `Accepts` takes 0.4.9 and refuses 0.5.0; `buildVersion` gives P4 for 0.5.0 with `up to minor 4`; the `version` command's golden reads `tableaux 0.4` |
| T2 | The schemas | The six files compile; `agents-not-email` fails `agents.schema.yaml` and `agents-listed` passes it; `schemas/README.md` records `1d1cb02` at 0.4.0, and the implementer copies the six files byte for byte from that commit |
| T3 | The agents file | `agents-listed`: no L4, `Project.Agents` lists opus; `file-stray-path` still gives L4; `Log.Project` at each commit of `agent-work-under-person` holds no file, and a built history that adds one holds it from that commit |
| T4 | The field in the Loader | `subproject-field`: `Task.Subproject` with `OnTask`, one link with the junctions'; the holders of every field of `weather-station` |
| T5 | The unpinned read | `requires-unpinned` through `Replace`: `Unpinned`, `Trunk` main, `Commit` the label L2; under `t.TempDir()`: a clone whose `origin/main` is ahead of its `main` reads `origin/main`; a `version.yaml` naming a branch no ref carries gives `NoTrunk`; no mapping and no cache give `NoClone`; `FetchTip` over a `file://` URL clones and reads the tip; a second load after the other trunk moves reads the new tip, and a pinned link of the same load stays in the memo |
| T6 | The files rules | Every row of [`rules.md`](cdb0/rules.md) on its entries, through `corpus.expected.txt` regenerated at `1d1cb02`; built cases: J19 once for a task whose link eight junctions name, silent for a requirement and for an unreadable version; R14 with J8 where no clone exists; `reviewer: true` gives one T4; T7 at `subproject.id`; R12 on the field |
| T7 | One resolution | `shape.applicable` and `kindAt` equal the Derivation's on `subproject-field` and `subproject-field-on-parent`, as 8118 decision 13 d asks of every entry with no error |
| T8 | The history rules | `facts.yaml` gains literal cases: H7 at 0.3.2 silent, at 0.4.0 with `Model` alone, with `CoAgents` alone, silent where `AuthorAgent`; H8 for each `Moot` and silent for an empty one |
| T9 | The kind and the reviewer | Built in memory: the file lists x and an entry pairs y with a model, so x is an agent and y is not, and y's junction is still an agent's by its model; an agent assignee under an agent parent under a person gives `ByNearest` with the root; three agents give `ByAssignee`; a waiver at the root reaches a leaf and a leaf's reviewer restores review; `true` supplies nothing; `DefaultMarks` on the parent drops the nearest person's review |
| T10 | The crossing | [`crossing.sh`](cdb0/crossing.sh) built under `t.TempDir()`: the Derivation at each label equals [`crossing.expected.txt`](cdb0/crossing.expected.txt) line for line, and `tablo.Validate` at each label gives its `validate` lines |
| T11 | The field in the Derivation | `subproject-field`: kinds, sources, applicable, snapshot and the three pin events of the field's path; `subproject-field-on-parent`: the parent's junctions are recursive and b2c9's plain |
| T12 | F11 | `authorised-by-merge`: three events, A4 `authorised` by olive, the status at A3; 27a3's `cases.expected.txt` gains `event 2026-09-03 C3 olive@example.org c3d7 authorised` before C5's |
| T13 | The unpinned condition | `requires-unpinned`: met, due, `AtTrunk` yes, no R13 and no pin event; the facts `At` the entry's commit `only` read the requirement at the tip; a built history that then states the commit gives a pin event with no `old` |
| T14 | The views | `ReviewerByDefault` and the nearest supplier on `review-nearest-person`; `queue --brief b2c9 release` gives the source kind `nearest`; `co_author` on the X3 and X5 lines of the crossing and on their model checks; the delegated supplier at the key `subproject` on `subproject-field`; the third command on `authorised-by-merge` and none on `trunk-undetermined`; every golden validates against `envelope.schema.yaml`; a finding with the act `move_on` and a source kind `nobody` each fail the schema |
| T15 | The Audit | `table` is total and true to RULES.md at `1d1cb02`; R14 gives `state_commit` and the assignee, J19 `raise_language`; H7 settles once its junction is historical, H8 once its task is complete; `silent` lists the four new rules on a clean project |
| T16 | Conformance | `TestCorpus` on the embedded copy: 119 entries pass, `review-nearest-person` is known, `tableaux-tooling` skips; with `TABLO_CORPUS` on a clone that checks out its submodules, `tableaux-tooling` passes too |
| T17 | The own plan | `TestOwnPlan` of `internal/validate` sets aside J8 on every requirement by absolute URL, as it does for `4b4f` today, and finds nothing else |
| T18 | The failures of today | Each failing entry of the table below passes after the step named, and leaves `known.yaml` in that step |

**The fifteen failures at `1d1cb02`, and the fix of each.** The pinned tablo passes 105 of 120 entries.

| Entry | Fails today on | Fixed by | Step |
|-------|----------------|----------|------|
| `agents-listed` | P4 | the file kind; the kind by the file; the nearest person; D7's observation | 1, 2, 4 |
| `agents-not-email`, `agents-unlisted-model` | P4 | A1; A2 | 1, 2 |
| `junction-reviewer-true` | P4 | T4's one message | 1, 2 |
| `subproject-field`, `subproject-field-on-parent` | P4 | the field; J18 | 1, 3 |
| `subproject-language-behind` | J8, the module refusing 0.4.0 | minor 4; J19 | 1, 3 |
| `tableaux-tooling` | four J19 | J19 | 3 |
| `review-nearest-person` | P4 | the kind by the pairing; the nearest person; the version at the commit; D7; then known (D6) | 1, 4 |
| `junction-review-waived` | P4 | the waiver | 1, 4 |
| `agent-work-under-person` | P4 | H7 | 1, 4 |
| `requires-unpinned` | P4 | the unpinned read; R14 | 1, 5 |
| `reaffirmed-snapshot` | P4 | H8 | 1, 6 |
| `undefined-leaf-reaffirmed` | H8 | H8 for a leaf with no file | 6 |
| `authorised-by-merge` | F11's event | the merge's event | 6 |

**Tried.** A throwaway trial in a copy of the module outside the repository, about 650 changed lines over the declarations of [`model.md`](cdb0/model.md), with the corpus copy refreshed to `1d1cb02` by `corpus.sh refresh`. `TestCorpus` with `TABLO_CORPUS` on a clone of the umbrella at `1d1cb02` with its four submodules gives "120 entries: 119 pass, 1 fail" and "88 rules: 88 confirmed"; the failure is `review-nearest-person`, `tasks.b2c9.reviews.length want 1 got 2`, the review at `defined` of D6. On the embedded copy, 118 pass, 1 fails and `tableaux-tooling` skips. On `main`, the same run gives 105 pass and 15 fail, as the brief counts. Before D7's observation the trial failed `agents-listed` and `review-nearest-person` on `junctions.default.reviewer`, and before T4's one message `junction-reviewer-true` gave two T4. The trial built [`crossing.sh`](cdb0/crossing.sh) and wrote [`crossing.expected.txt`](cdb0/crossing.expected.txt) from its Derivation and its `tablo validate`. Its `tablo -C` on the umbrella's checkout at `1d1cb02` gives no error, sixteen H3, eight H4 and the four J19. On the trial's whole suite, the failing tests are the goldens and tables this design changes, among them 27a3's `cases.expected.txt` at C3 and the Loader's count of unquoted ids in the corpus.

**Unverified.** The trial did not regenerate the goldens, ran no Loader test of `tip` over a clone with `origin/HEAD` or of `FetchTip`, and read `version.yaml` at the default branch through the whole project. Nothing ran on macOS or Windows. **Later gates.** Unit: T1 to T18. Integrate: each front end's reconcile reads the four new words (A4), and the umbrella's validation shows the four J19 until the pins advance.

## Implementation notes

**Order.** Each step builds, passes `go test ./...` and changes the goldens it moves.

1. **tablo reads 0.4.0.** `AcceptedMinor` 4 and `Release`; the six schemas of `1d1cb02` with `schemas/README.md` and `embed.go`'s comment; the seven rows of `validate.Rules` and of `audit.table`, with no function yet, the two acts and the five sentences; `corpus.sh refresh <umbrella> 1d1cb02`; `known.yaml` lists, `owes: cdb0`, every entry of the table above that fails on the embedded copy, which skips `tableaux-tooling`; `TestOwnPlan`'s exemption (T17); README.md's "Versions accepted". **In this commit tablo first reads 0.4.0**: the owner then commits `tasks/cdb0.yaml` with its three requirements at `1d1cb02`, `from: implementation`; the implementer leaves the task file alone.
2. **The agents file**, A1, A2, T4. Three items leave `known.yaml`.
3. **The task-level field**, J18, J19 and the delegated supplier. Three items leave, and `tableaux-tooling` passes where a checkout supplies it.
4. **The agent's identity**: `CoAuthors`, `Agent`, the nearest person, the waiver, `DefaultMarks`, H7, the adapter's facts, the view words, D7's observation and the crossing. Three items leave; `review-nearest-person` stays with `owes: fcec` and its finding (D6).
5. **The unpinned requirement**, R14 and the past read. One item leaves.
6. **The merge's event**, H8, the third command. Three items leave, and `known.yaml` holds one.

**Files.** New: `internal/derive/testdata/crossing.sh` and `crossing.expected.txt`, `schemas/agents.schema.yaml`, the refreshed `internal/corpustest/testdata/corpus`. Changed: the files of each package in the table above, `envelope.schema.yaml`, `internal/conformance/known.yaml` and `observe_test.go`, and the goldens of `internal/validate`, `internal/audit`, `internal/derive`, `internal/views`, `internal/cli`, `view` and `testdata/audit.corpus.txt`. No prototype exists, and `go.mod` does not change.

**The plan.** `go test ./...` fails on `main` at `42e3081`: `TestOwnPlan` reports J8 on the three requirements `f910e8c` gives `cdb0`, since it sets aside J8 on `4b4f` alone; step 1 mends the test. tablo's own `.tableaux/version.yaml` stays at 0.3.1, and raising it is the owner's plan commit. No front-end task requires `cdb0` yet; each reconcile that follows may require it from implementation to integrate.

## Decisions at review

1. **One set of files, and the version decides readings.** tablo reads the agents file, the task-level field, the waiver and the unpinned entry at every accepted version; the version at a commit decides only the agent's reading and the default reviewer, which README.md#junctions names, and the exemptions of H6 and H7. The alternative reads the 0.4.0 forms in a project at 0.3.x as that language did, with T3 for the field, T4 for `false`, L4 for the file and J15 for the entry, four special cases no corpus entry tests. Accepted by the owner on 2026-10-20, as stated.
2. **The nearest person's words.** `derive.ByNearest` names the ancestor whose assignee reviews; the view states it as that task's `assignee`, sets `reviewer_by_default`, and gives a brief's source the new kind `nearest` in `view_source_kind`; a waiver states the reviewer's supplier as the entry with `false`. The alternative adds a word `nearest` to `Stated.By` too. Binds tabloio `e4c7` and `e3ed`, tableaud `438a` and `e2b6`, tablotui `0afa`. Accepted by the owner on 2026-10-20, as stated.
3. **`co_author` on every model check and history line.** VIEWS.md's Models fact names the agent a person's commit co-authors; `view.ModelCheck` and `view.LineDetail` carry it, and the envelope requires the key. The alternative leaves it to the trailers of a commit at provenance, so the fact shows at no lower level. Binds tabloio `9167` and `e3ed`, tableaud `9a9c` and `438a`, tablotui `0afa`. Accepted by the owner on 2026-10-20, as stated.
4. **Two acts, `state_commit` for R14 and `raise_language` for J19,** as VIEWS.md#audit names the two actions; `view_finding_act` widens within `tablo/1`. The alternative gives both `move_pin`, as 9f3f's body does for J19, and adds no word. Binds tabloio `9167`, tableaud `9a9c`, and dada's `rules.md`. Accepted by the owner on 2026-10-20, as stated.
5. **A parent's default marks drop the nearest person's review,** as they drop the assignee's, amending 27a3 decision 12 (a). The alternative draws 👀 on a parent's cell wherever its agent default finds a person, a review no child inherits. Binds `493e` and `886d`. Accepted by the owner on 2026-10-20, as stated.
6. **The corpus omits a review, and tablo lists the entry as known.** At `1d1cb02` `review-nearest-person` lists b2c9's design review alone, though its own default gives the `defined` junction the reviewer olive, which the authorisation accepts. tablo derives that review; `known.yaml` carries the entry, `owes: fcec`, with the finding, so 119 entries pass and one is known. The alternative amends the entry on `wave/language-0.4` first and moves the lock and the three requirements to that commit, against the ruling that pins them to `1d1cb02`, and then all 120 pass. Binds the umbrella's `fcec` and `4b4f`. Decided by the owner on 2026-10-20 for the alternative: the umbrella's `df523e4` on `wave/language-0.4` lists the review at `defined` in `review-nearest-person`, so the corpus lock and `cdb0`'s three requirements name `df523e4` wherever this design names `1d1cb02`, all 120 entries pass, and `known.yaml` gains no item; A1 and step 4's known item fall away.
7. **The corpus's default junction reads a reviewer.** The conformance runner observes `junctions.default` with the reviewer of the first plain junction no entry supplies, since an agent's default now has one. The alternative leaves 4b4f's observation table as it stands, and `agents-listed` and `review-nearest-person` fail on the field. Binds `4b4f`. Accepted by the owner on 2026-10-20, as stated.
8. **`no trunk`, a new problem of a link.** `LinkError.Problem` gains it for a clone that holds no branch an unpinned requirement can read, beside `no clone`. The alternative reports it as `no clone`, which sends the reader to fetch a clone it already has. Binds tableaud `49ce`, which reads `LinkError`. Accepted by the owner on 2026-10-20, as stated.
