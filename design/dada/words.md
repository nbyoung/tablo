# The words

Every text the Audit writes beyond the rows of [`rules.md`](rules.md). A value stands in angle brackets. The Audit writes an id, a gate key, a state, a path, a hash, a URL, a model and a trailer in backticks, and an email and a date bare (tabloio `e3ed`, R17). A hash is always the full hash. [`corpus.expected.txt`](corpus.expected.txt) shows each text on the corpus.

## The two kinds

| Field    | `proposed`                                               | `stale`                                                  |
|----------|----------------------------------------------------------|----------------------------------------------------------|
| Severity | `warning`                                                | `warning`                                                |
| Kind     | A proposed task                                          | A stale status                                           |
| Task     | The task                                                 | The task                                                 |
| Gate     | None                                                     | The gate the status states                               |
| Files    | `tasks/<id>.yaml`                                        | `status/<id>.yaml`                                       |
| Act      | `authorise`                                              | `reaffirm`                                               |
| Resolver | The first of `Authorities(id)`; for the root the first of `Authorisation.Judges`; else `Owner()` | The recorder, as the design defines the role |
| Commits  | `Authorisation.Commit`, when there is one                | `Status.Commit`                                          |
| Sentence | The task is authorised when the deciding commit's author or committer is one of its authorities, and proposed otherwise. | A review that finds no change reaffirms with an empty commit, so a status is never older than its last confirmation. |
| Source   | README.md, Proposed and authorised tasks: `https://github.com/nbyoung/tableaux/blob/main/README.md#proposed-and-authorised-tasks` | README.md, Status: `https://github.com/nbyoung/tableaux/blob/main/README.md#status` |

The message and the action of a proposed task follow `Authorisation.Why`:

| Case                                    | Message                                                                                             | Action                                         |
|-----------------------------------------|-----------------------------------------------------------------------------------------------------|------------------------------------------------|
| `Determined`, with a deciding commit    | The task stands proposed: &lt;author's email&gt;, the author of its deciding commit, is none of its authorities | Authorise: commit `` `Authorised: <id>` ``     |
| `NoCommit`                              | The task stands proposed: no commit on the trunk holds its file                                     | Authorise: commit `` `tasks/<id>.yaml` `` on the trunk |
| `OffTrunk`, and `Differs`               | The task stands proposed: the source changes its file and is off the trunk `` `<trunk>` ``          | Authorise: merge the source into `` `<trunk>` `` |

A stale status reads: `The status dates from <date>, more than <n> days before <now>`, and its action ``Reaffirm: commit `Reaffirmed: <id>`, or record the next status``. No message ends in a full stop.

## Actions

The texts that take a value or a condition; every other rule takes the text of its row.

| Rule | Condition                                             | Act              | Resolver    | Action                                                                                             |
|------|-------------------------------------------------------|------------------|-------------|----------------------------------------------------------------------------------------------------|
| J8   | The link's `Problem` is `NoClone` or `CommitAbsent`, and its `Form` is `Submodule` | `checkout` | nobody | Check out the submodule `` `<url>` ``                                                 |
| J8   | The same problems, any other form                     | `checkout`       | nobody      | Check out the subproject: give the tool a clone of `` `<url>` `` that holds the commit             |
| J8   | Any other problem, or no link found                   | `revise`         | keeper      | Revise the file                                                                                    |
| H2   | `Accepted(id, gate)` has no commit                    | `review`         | reviewer    | Review: commit `` `Reviewed: <id> <gate>` ``                                                       |
| H2   | `Accepted(id, gate)` has a commit                     | `none`           | author      | Nothing resolves it: the commit stands in the history                                              |
| S11  | Always                                                | `review`         | reviewer    | Review: commit `` `Reviewed: <id> <gate>` ``                                                       |
| H4   | `Handoff(id).Self` is false                           | `record_handoff` | contributor | Record the hand-off: the reason `` `review` `` when the work is done; or work on                   |
| H4   | `Handoff(id).Self` is true                            | `record_handoff` | contributor | Record the gate with `` `Reviewed: <id> <gate>` `` on the commit that records the status; or work on |
| H5   | Always                                                | `clear_handoff`  | contributor | Clear the hand-off: record the status at `` `<gate>` ``                                            |
| R9   | The condition is found                                | `advance`        | origin      | Advance `` `<origin>` `` past `` `<from>` ``                                                       |
| R9   | No condition is found                                 | `advance`        | origin      | Advance the originating task                                                                       |

## Facts

A finding takes each fact whose condition holds, in this order.

| Name          | Condition                                                                 | Value                                                                                      |
|---------------|---------------------------------------------------------------------------|--------------------------------------------------------------------------------------------|
| Junction      | A rule finding with a task and a gate, where `Junction(id, gate)` is plain | &lt;contributor&gt; contributes at `` `<gate>` ``; then `` under the model `<model>` `` where it states one; then `; <reviewer> reviews.`, `; the contributor reviews its own work.` or `; nobody reviews.` |
| Junction      | The same, recursive                                                       | `` `<gate>` reads `<target>` of `<url>` at `<commit>`. `` without `` `<target>` of `` when the snapshot names none, without the commit for a directory, and `` `<gate>` reads a subproject. `` with no link |
| Junction      | The same, not applicable                                                  | `` `<gate>` does not apply. ``                                                             |
| Status        | The one file is `status/<id>.yaml`, the task is a leaf, and its status is `Recorded` or `Snapshotted` | `` `<id>` stands at `<gate>` ``; then `` , `<state>` `` and `` , `<reason>` `` where the status has them; then `, since <date>` and `, recorded by <recorder>` where it has them; then a full stop |
| Authorisation | A proposed task with a deciding commit                                    | &lt;author&gt; authors the deciding commit and &lt;committer&gt; commits it; the authorities are &lt;judges, joined by a comma and a space&gt;. |
| Authorisation | A proposed task, `NoCommit`                                               | No commit on the trunk decides the task.                                                   |
| Authorisation | A proposed task, `OffTrunk`                                               | The source is off the trunk `` `<trunk>` ``.                                               |
| Requirement   | R9 or R13, the condition found                                            | `` `<task>` requires `<origin>` from `<from>` to `<to>`; `<origin>` stands at `<stands>`. `` |
| Model         | H3 or H6, with the event of `Events(id)` that has no `Sub`, the finding's commit and a `Reading` at its gate | `` The junction states `<stated>`; the commit carries `Model: <trailer>`. `` or `` … carries no `Model:` trailer. `` |
| Pin           | J13, J17 or R13, the link found                                           | `` `<url>` reads at `<commit>` ``; then `` ; the tip of its trunk is `<tip>` `` where `Pin(link).Tip` is not empty; then a full stop |

## Commits

A finding's commits are, each once and the oldest first by `Commit.Seq`: the commit its diagnostic names, when the pass holds it; for a finding whose Status fact holds, the status's deciding commit, `Status.Commit` for a recorded status and `Status.Own` for a snapshot; and the commits of the two kinds above.

## Commands

The first command shows the finding, with the comment `shows the finding`; the second resolves it, with the comment `resolves it`, for four acts alone.

| Case                                              | Text                                                                                   |
|---------------------------------------------------|----------------------------------------------------------------------------------------|
| Shows: the finding has a commit                   | `git show --no-patch --format='%as %H %ae %ce%n%(trailers)' <its newest commit>`       |
| Shows: else it has a file                         | `git log -1 --format='%as %H %ae' -- <Where.Dir>/.tableaux/<its first file>`, the path joined and cleaned |
| Shows: else                                       | No command                                                                             |
| Resolves: `authorise`, the action names a trailer | `git commit --allow-empty --trailer 'Authorised: <id>'`                                |
| Resolves: `review`                                | `git commit --allow-empty --trailer 'Reviewed: <id> <gate>'`                           |
| Resolves: `reaffirm`                              | `git commit --allow-empty --trailer 'Reaffirmed: <id>'`                                |
| Resolves: `checkout`, a submodule                 | `git submodule update --init <url>`                                                    |
