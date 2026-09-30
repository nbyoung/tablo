# Prototype 8ed1: the temporal views

## The question

Can the history and audit views, each bounded by a ref range, come from `git log` cheaply, in the shape of `schemas/history.schema.yaml` and SYNTAX.md#history?

Yes. One `git log --root --reverse --raw --no-abbrev` over `FROM..TO` yields every commit with its author, author time, message and changed paths with blob hashes. One `git cat-file --batch` reads the status blobs the range changes. The prototype makes those two calls whatever the range holds, plus one `git grep` for each pin (to find the task whose recursive junction names the submodule path). It never checks a commit out.

## How to run

```
export PATH=$PATH:/usr/local/go/bin
go run ./prototype/8ed1 -repo <repo> -view history|audit -range FROM..TO [-task ID] [-person EMAIL] [-stale DAYS]
go test ./prototype/8ed1
```

`FROM` and `TO` are refs or corpus labels (`W3`, resolved through `<repo>.labels.txt`). `FROM` may be empty for the whole history up to `TO`. The test skips when the corpus at `/home/nbyoung/Projects/Tableaux/tableaux/corpus/build/` is absent.

## What it shows

The history of `weather-station` from W2 to W6 (the sensor board proposal, its merge, the dashboard proposal and acceptance):

```
{"date": "2026-09-18", "commit": "0effab7", "by": "ada@example.org", "task": "9f31", "event": "task"}
{"date": "2026-09-19", "commit": "931c43d", "by": "ben@example.org", "task": "9f31", "event": "authorised"}
{"date": "2026-09-20", "commit": "1939209", "by": "dan@example.org", "task": "3c5d", "event": "task"}
{"date": "2026-09-21", "commit": "61b6558", "by": "ada@example.org", "task": "3c5d", "event": "authorised"}
```

The history from W9 to W13 (a status with its reason and note, a status with no reason, a pin, a task revision and a reaffirmation):

```
{"date": "2026-09-25", "commit": "efa5c1e", "by": "ada@example.org", "task": "9f31", "event": "status", "gate": "function", "state": "stalled", "reason": "blocked", "note": "Barometer ICs on 14-week backorder"}
{"date": "2026-09-26", "commit": "303c805", "by": "ben@example.org", "task": "c07d", "event": "status", "gate": "design"}
{"date": "2026-09-26", "commit": "303c805", "by": "ben@example.org", "task": "c07d", "event": "pin", "note": "pin firmware at 3dc465e"}
{"date": "2026-09-27", "commit": "b426894", "by": "dan@example.org", "task": "3c5d", "event": "task"}
{"date": "2026-09-28", "commit": "fbcd6ad", "by": "ada@example.org", "task": "9f31", "event": "reaffirmed"}
```

The audit from W4 to W5, and from W3 to W13 with a stale age of three days:

```
{"rule": "PROPOSED", "severity": "warning", "task": "3c5d", "message": "The task stands proposed: no trailer or owner commit on the trunk authorises it", "since": "1939209", "range": "introduced"}
{"rule": "PROPOSED", "severity": "warning", "task": "9f31", "message": "The task stands proposed: no trailer or owner commit on the trunk authorises it", "since": "0effab7", "range": "resolved"}
{"rule": "STALE", "severity": "warning", "task": "3c5d", "message": "The status dates from 2026-09-22, more than 3 days before 2026-09-28, with no later reaffirmation", "since": "4ebe52f", "range": "introduced"}
```

Each finding carries `range`: `introduced` (absent at FROM, present at TO), `standing` (both) or `resolved` (present at FROM, absent at TO). One log pass to TO serves both ends of the audit: the state at FROM is the same events cut to the commits reachable from FROM.

Decisions the prototype makes, all visible in the output:

- Events come from the commit that made them. A merge shows no diff, so it yields only its trailers; the branch commit `0effab7` carries the `task` event and the merge `931c43d` the `authorised` event. A `--no-ff` merge keeps both, as README.md says, and no event counts twice.
- Order is author time, ties broken by log order (topological, oldest first).
- The `pin` event uses the value `pin`, which the history schema's enum lacks (finding F12). Its `task` is the task whose junction names the submodule path (`c07d` for `firmware`), and its `note` gives the pinned commit. Every other event validates against the schema as it stands.
- A status event reads the blob the commit records, so it carries the values as recorded, not as derived.

## What it leaves out

- Stand-ins: `standIn` in `main.go` holds three rules (PROPOSED, H1, STALE) computed from the events alone, and `authorities` reads assignees and parents from the task files at TO. Task dada builds the audit and task 27a3 the authorisation; both replace these. The stand-in misses the corpus's `W12` case, where Dan's revision returns `3c5d` to proposed, because it does not compare a task file with the accepted one.
- YAML: the status and task files are read as flat `key: value` lines (`gate`, `state`, `reason`, `note`, `assignee`, `parent: { id: ... }`). The implementation needs a real YAML parser (`gopkg.in/yaml.v3`), which keeps line positions.
- The `person`, `task` and `window` parameters: only `task` and `person` are filtered, and after the log pass. A subtree, the replayed status after each event, the committer, the `Model:` trailer and the subproject's events up to the pin are not built.
- Trailer validity: a trailer counts wherever it appears in the message; the implementation reads the trailer block and checks the gate against `gates.yaml`.
- The `Reviewed:` and `Authorised:` checks against the committer, and a file's removal (a deleted status file yields a `status` event with no values).
- Cost measurement: the range is linear in the commits it holds; a very long range with a single-task filter still reads every commit. The implementation can pass the task's paths to `git log -- <paths>` for the file events and `--grep` for the trailers, as README.md#history gives, and merge the two passes.

## What depends on a choice still open in VIEWS.md (draft on `worktree-agent-a6b84238bbacc4fcd`)

- **The change of a pin is an event named `pin`** (VIEWS.md, Events row; finding F12 waits on task 9f3f). If 9f3f names it otherwise, or gives it a `pin` field, one string in `History` changes.
- **The range as a history bound.** VIEWS.md gives `v1.0..v1.1` and `main..task/e9c6`. The prototype uses Git's `A..B` (commits reachable from B, not from A). For a branch that merged trunk, that differs from "the events the branch adds beyond the trunk, marked as proposals"; the marking is not built.
- **Which findings a range lists.** VIEWS.md's audit says "ref" and leaves open whether a range lists findings standing at its end, findings that arose in it, or both. The prototype lists all three with a `range` tag; the choice decides whether `standing` and `resolved` stay.
- **The stale age (Q6).** Fourteen days by default; the prototype takes the date of TO, not the wall clock, so a past range reports as it stood. Whether the age counts from the wall clock or the range end is open.
- **The audit's data list** (proposed, H1, H2, H3, F24, R9, stale, J13, J8/J9, P5, F14). Only three rules are stood in; F24 changes what H3 needs (a missing `Model:` trailer) and F14 what the roll-up gives, so those wait on `dada`.
- **The events' detail level.** The committer, model and status after each event are detail-level data in VIEWS.md and the schema has no field for them (`additionalProperties: false`). The prototype emits the schema's fields only; adding them is a schema change or a second structure.
- **The level names (Q1)** do not touch this data; the prototype emits one level, the widest the schema allows.
- **F21 ties** and **F22 (the `review` reason)** do not affect these views, since the replayed parent status is not built.

## What the design gate must decide

- The `pin` event: its name, its schema entry and its `task` (the task holding the recursive junction, as here, or the parent).
- The range semantics for a branch (`A..B`, or "beyond the trunk" with proposals marked).
- Whether an audit range reports introduced findings only, or all three tags.
- Whether the implementation reads the log in one pass (as here, simple, linear in the range) or in two path-limited passes (as README.md#history gives, faster for a single task).
- The YAML dependency and its line-position support.
