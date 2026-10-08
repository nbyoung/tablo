# The words and the typed facts

What a finding holds beyond the rows of [`rules.md`](rules.md): the three kinds no rule states, with their messages, the only texts the Audit words; the typed facts each rule and kind carries; the commits; the commands. The Audit words no action and no fact (decision 8 of [design dada](../dada.md#decisions-at-review)): a front end words the action from the act and renders each fact as it renders it in every other view. A value stands in angle brackets. A hash is always the full hash. [`corpus.expected.txt`](corpus.expected.txt) shows each on the corpus.

## The three kinds

| Field    | `proposed`                                | `stale`                                   | `sole_review`                                      |
|----------|-------------------------------------------|-------------------------------------------|----------------------------------------------------|
| Severity | `warning`                                 | `warning`                                 | `information`                                      |
| Kind     | A proposed task                           | A stale status                            | A junction nobody else reviews                     |
| Task     | The task                                  | The leaf                                  | The leaf                                           |
| Gate     | None                                      | The gate the status states                | The junction's gate, never one the authorisation reviews |
| File     | `.tableaux/tasks/<id>.yaml`               | `.tableaux/status/<id>.yaml`              | `.tableaux/tasks/<id>.yaml`                        |
| Act      | `authorise`                               | `reaffirm`                                | `none`                                             |
| Resolver | The first of `Authorities(id)`; for the root the first of `Authorisation.Judges`; else `Owner()` | The recorder, as the design defines the role | The first of `Authorities(id)`; `Owner()` for the root |
| Commits  | `Authorisation.Commit`, when there is one | `Status.Commit`                           | None                                               |
| Sentence | The task is authorised when the deciding commit's author or committer is one of its authorities, and proposed otherwise. | A review that finds no change reaffirms with an empty commit, so a status is never older than its last confirmation. | An agent contributor with no stated reviewer takes the assignee as reviewer; a person's plain junction with no stated reviewer has none, and its status completes the gate on the contributor's word. |
| Source   | README.md, Proposed and authorised tasks: `https://github.com/nbyoung/tableaux/blob/main/README.md#proposed-and-authorised-tasks` | README.md, Status: `https://github.com/nbyoung/tableaux/blob/main/README.md#status` | README.md, Junctions: `https://github.com/nbyoung/tableaux/blob/main/README.md#junctions` |

The messages. No message ends in a full stop, and none names an act: the act and the typed facts say what resolves the finding.

| Kind          | Case                                                    | Message                                                                                                         |
|---------------|---------------------------------------------------------|-----------------------------------------------------------------------------------------------------------------|
| `proposed`    | `Authorisation.Why` is `Determined`, with a deciding commit | The task stands proposed: &lt;author's email&gt;, the author of its deciding commit, is none of its authorities |
| `proposed`    | `NoCommit`                                              | The task stands proposed: no commit on the trunk holds its file                                                 |
| `proposed`    | `OffTrunk`, and `Differs`                               | The task stands proposed: the source changes its file and is off the trunk `` `<trunk>` ``                      |
| `stale`       | Always                                                  | The status dates from &lt;date&gt;, more than &lt;n&gt; days before &lt;now&gt;                                 |
| `sole_review` | Always, in both cases                                   | Nobody but the contributor reviews the work at the gate                                                         |

The two cases of a sole review read from the Junction fact and from no second kind: `reviewer` equals `contributor` where the contributor reviews its own work, an agent's where `model` is not empty; `reviewer` is `""` where a person's plain junction states none.

## The typed facts

`Facts` of a finding holds values of the Derivation; the view writes each through the builder of `internal/views` that every other view uses ([`model.md`](model.md#from-a-finding-to-the-data)). `id` and `gate` are the finding's task and gate. A fact is nil when the finding names no task of the project, when its condition fails, or when the method returns nil.

| Fact            | `view` type, key                | From `derive.Facts`                                                             | A finding carries it when                                                                                  |
|-----------------|---------------------------------|---------------------------------------------------------------------------------|------------------------------------------------------------------------------------------------------------|
| `Junction`      | `Junction`, `junction`          | `Junction(id, gate)`                                                            | It names a task and a gate after `undefined` that `gates.yaml` holds: every rule finding that does, and `sole_review` |
| `Status`        | `Status`, `status`              | `Status(id)`                                                                    | Its file is `.tableaux/status/<id>.yaml` and the task is a leaf whose status is `Recorded` or `Snapshotted`; and `stale`. A finding demoted by its junction or by its task carries it too, since the status says so |
| `Requirement`   | `Requirement`, `requirement`    | The entry of `Requires(id)` the diagnostic stands at, by its position           | R9, R12 or R13, the condition found                                                                        |
| `Authorisation` | `Authorisation`, `authorisation` | `Authorisation(id)`                                                            | `proposed`                                                                                                 |
| `Snapshot`      | `Linkage`, `linkage`            | `Junction(id, gate).Snapshot`; the view reads `Pin(Snapshot.Link)` for the pin   | J8, J9 and J13 to J17, where the junction is recursive                                                     |
| `Reading`       | `ModelCheck`, `model`           | The `Reading` of the event of `Events(id)` that has no `Sub`, the finding's commit and the finding's gate | H3 or H6, the event found                                                    |

By rule and by kind, what a finding holds. ● always, where the task and the gate are of the project; ○ by the condition of the table above; a blank never.

| Key                          | Act                        | junction | status | requirement | authorisation | linkage | model | Commits                               |
|------------------------------|----------------------------|----------|--------|-------------|---------------|---------|-------|---------------------------------------|
| `proposed`                   | `authorise`                |          |        |             | ●             |         |       | The deciding commit, when there is one |
| `stale`                      | `reaffirm`                 |          | ●      |             |               |         |       | `Status.Commit`                        |
| `sole_review`                | `none`                     | ●        |        |             |               |         |       | None                                   |
| S11                          | `review`                   | ●        | ●      |             |               |         |       | The status's deciding commit           |
| R9                           | `await`                    |          |        | ○           |               |         |       | None                                   |
| R12                          | `revise`                   |          |        | ○           |               |         |       | None                                   |
| R13                          | `move_pin`                 |          |        | ○           |               |         |       | None                                   |
| J8, J9                       | `checkout` or `revise`     | ●        |        |             |               | ○       |       | None                                   |
| J13 to J17                   | `move_pin` or `revise`     | ●        |        |             |               | ○       |       | None                                   |
| H1                           | `none`                     | ○        | ○      |             |               |         |       | The commit the diagnostic names        |
| H2                           | `review` or `none`         | ●        | ○      |             |               |         |       | The commit; with `none`, also `Accepted(id, gate).Commit` |
| H3, H6                       | `none`                     | ●        | ○      |             |               |         | ○     | The commit                             |
| H4, H5                       | `record_handoff`, `clear_handoff` | ●  | ●      |             |               |         |       | `Handoff(id).Commit`, and the status's deciding commit |
| P5                           | `revise`                   |          |        |             |               |         |       | None                                   |
| Every other rule             | `revise`                   | ○        | ○      |             |               |         |       | The status's deciding commit where the Status fact holds |

A requirement on a subproject carries its link in the `Requirement` itself, as `subproject`, `commit` and `at_trunk`, so R13 needs no linkage. The hand-off of H4 and H5 is `Facts.Handoff(id)`, which the Validator reads to raise them (886d A7); the finding carries its junction and its status and no fact of its own.

## Commits

A finding's commits are, each once and the oldest first by `Commit.Seq`: the commit its diagnostic names, when the pass holds it; for a finding whose Status fact holds by its file, the status's deciding commit, `Status.Commit` for a recorded status and `Status.Own` for a snapshot; for an H2 with the act `none`, the commit that accepts the junction; and the commits of the kinds above.

## Commands

The first command shows the finding, with the comment `shows the finding`; the second resolves it, with the comment `resolves it`, for four acts alone.

| Case                                              | Text                                                                                   |
|---------------------------------------------------|----------------------------------------------------------------------------------------|
| Shows: the finding has a commit                   | `git show --no-patch --format='%as %H %ae %ce%n%(trailers)' <its newest commit>`       |
| Shows: else it has a file                         | `git log -1 --format='%as %H %ae' -- <Where.Dir>/<its file>`, the path joined and cleaned |
| Shows: else                                       | No command                                                                             |
| Resolves: `authorise`, `Authorisation.Why` is `Determined` | `git commit --allow-empty --trailer 'Authorised: <id>'`                       |
| Resolves: `review`                                | `git commit --allow-empty --trailer 'Reviewed: <id> <gate>'`                           |
| Resolves: `reaffirm`                              | `git commit --allow-empty --trailer 'Reaffirmed: <id>'`                                |
| Resolves: `checkout`, the link's `Form` is `Submodule` | `git submodule update --init <url>`                                               |
