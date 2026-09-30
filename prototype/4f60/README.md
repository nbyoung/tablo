# Prototype 4f60: the loader

Gate `function` of task `4f60`.

## Question

Can the loader read a project's `.tableaux` directory at an arbitrary Git ref through `git ls-tree` and `git cat-file` alone, parse each file into a typed model that keeps file, line and column positions, and check `version.yaml` against the versions the module accepts?

## Run

```
L=/home/nbyoung/Projects/Tableaux/tableaux/corpus/build
go run ./prototype/4f60 -C $L/weather-station -labels $L/weather-station.labels.txt W1
go run ./prototype/4f60 -C $L/task-bad-email -labels $L/task-bad-email.labels.txt only
go test ./prototype/4f60/
```

The tests skip when the corpus is absent.

## What it shows

The same repository at two labelled commits, read from Git objects while the worktree holds the tip:

```
$ 4f60 -C weather-station W1          # 3 tasks, no status files yet
commit c4a7fcbe
version.yaml  tableaux 0.2.1 at version.yaml:1:11, trunk main
task 4e2b  "Sensor node"  parent a1c0 assignee ben@example.org at tasks/4e2b.yaml:6:11
...
12 gates, 3 tasks, 0 statuses
$ 4f60 -C weather-station W6          # 6 tasks
12 gates, 6 tasks, 0 statuses
```

An invalid entry reports the position of the offending value:

```
$ 4f60 -C task-bad-email only
error tasks/b2c9.yaml:4:11: T4: assignee "not-an-email" is not an email address
$ 4f60 -C version-minor-ahead only
error version.yaml:1:11: P4: version 0.3.0 not accepted; the module accepts 0.0.0 to 0.2.x
```

The version check calls `tablo.Accepts` and `tablo.ParseVersion` from `version.go`. `version-major-mismatch` reports P4 and `version-bad-pattern` reports P3.

## Layout

- `yaml.go`: a parser for the YAML subset the files use (block and flow maps and sequences, plain and quoted scalars, folded and literal blocks, comments). Every node carries line and column.
- `loader.go`: `Loader.Files` (`git ls-tree -r -z --name-only`), `Loader.Load` (`git cat-file blob`), the typed `Version`, `Task`, `Status` fields (`Str` holds a value with its `Pos`) and a few diagnostics (P3, P4, T4, T6, syntax).
- `main.go`: prints the model. `-labels` resolves a corpus label.

## What it leaves out

- Worktree loading, schema validation, the other rules, the derived facts, subprojects and submodules.
- Full YAML: anchors, tags, multi-line flow values, multiple documents. The implementation needs `gopkg.in/yaml.v3` (its `yaml.Node` keeps line and column) or an equivalent, and the task's go.mod has no require lines yet.
- Speed: one `git cat-file` process per file. The implementation would use `git cat-file --batch`.
- Status files at old refs: the weather station adds them late, so W1 and W6 hold none.

## The design gate decides

- Whether to take `yaml.v3` and lay a typed model on `yaml.Node`, or keep a parser of its own.
- Whether positions are line and column of the value (as here) or of the key.
- Whether the loader keeps `git` as a subprocess or reads objects with a Go library.
- The diagnostic type shared with the validator.
