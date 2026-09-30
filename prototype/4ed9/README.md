# Prototype 4ed9: the plumbing command's shape

## Question

Can one small dispatcher give every `tablo` subcommand the same shape: global `-C <dir>` and `--ref`, JSON or YAML out, one versioned envelope, exit codes a script can branch on, and a `trailer` subcommand that composes correct `Authorised:`, `Reviewed:` and `Reaffirmed:` lines per SYNTAX.md#commit-trailers?

## Run

```
go run ./prototype/4ed9 [-C dir] [--ref ref] [--json|--yaml] <command> [args]
go test ./prototype/4ed9
```

Commands: `validate`, `audit`, `view tasks`, `status`, `history`, `trailer`, `version`. The tests read the `weather-station` corpus entry by absolute path and skip when it is absent.

## What it shows

The envelope has five keys (`schema`, `command`, `ref`, `data`, `diagnostics`); the keys sort, so the output is stable. `ref` holds the name given and the commit it resolves to. `trailer` and `version` read no ref and omit it.

```
$ go run ./prototype/4ed9 -C .../weather-station view tasks
{ "command": "view", "data": { "tasks": [ { "assignee": "ada@example.org",
  "id": "9f31", "parent": "4e2b", "title": "Sensor board" }, ...
  "view": "tasks" }, "diagnostics": [], "ref": { "commit": "fbcd6ad...", "name": "HEAD" },
  "schema": "tablo/1" }
$ go run ./prototype/4ed9 -C .../weather-station --yaml status
statuses:
- gate: "function"
  id: "9f31"
  reason: "blocked"
  state: "stalled"
$ go run ./prototype/4ed9 trailer reviewed 9f31 design
Reviewed: 9f31 design
```

- Reading goes through `git ls-tree` and `git cat-file` at the ref, so `--ref` selects content (a test shows the sensor board absent at the first commit).
- Exit codes: 0 success, 1 an error diagnostic, 2 a usage error (usage on stderr), 3 a failure to read (unknown ref, unknown gate).
- `trailer` prints the bare line, ready for `git commit --trailer`, unless `--json` or `--yaml` asks for the envelope. It rejects an id that is not four lowercase hex digits, a missing or extra gate, and, when `-C` names a project, a gate absent from `gates.yaml`.
- Diagnostics carry `severity`, a stable `code`, `path` and `message`.

## Stand-ins and omissions

- The content subcommands are stubs: `view tasks` lists ids, titles, assignees and parents; `validate` checks the version rule and orphan status files; `audit` warns of a trailer naming an unknown task; `history` lists commits with their method trailers.
- YAML is read by a top-level line scan and written by a 40-line emitter. The implementation needs a YAML library (`gopkg.in/yaml.v3`) for both.
- Not done: `Model:` trailers, the check that a `Reviewed:` gate applies to the task, role filters, focusing parameters, the library behind the command.

## For the design gate

- The envelope: is `tablo/1` the right version scheme, and does a change to `data` alone bump it?
- Should the trailer line be plain by default, as here, or always an envelope?
- Exit code 1 for any error diagnostic: keep, or add a `--strict` for warnings?
- Should the command shell out to `git` (as here) or read objects in process?
