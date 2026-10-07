# tablo

The Tableaux backend: a Go library and a plumbing command that load, validate, audit and derive a Tableaux project from a Git repository and serve every abstract view as data.

The name plays on *Tableaux*: it spells the pronunciation.

## Place in the family

| Project                                            | Role                                                     |
|----------------------------------------------------|----------------------------------------------------------|
| [tableaux](https://github.com/nbyoung/tableaux)    | The language: method, syntax, schemas, corpus, mockups   |
| [tablo](https://github.com/nbyoung/tablo)          | The backend: library and plumbing command                |
| [tabloio](https://github.com/nbyoung/tabloio)      | The command line: textual output and Git input           |
| [tablotui](https://github.com/nbyoung/tablotui)    | The terminal user interface                              |
| [tableaud](https://github.com/nbyoung/tableaud)    | The local daemon: HTML views with progressive disclosure |

The split follows Git's own: `tablo` is plumbing that reads a repository and
emits data, and the three front ends are porcelain that presents it. A front end
never reads a task file itself.

## What it does

- **Load** a project's `.tableaux` directory from a worktree or from any Git ref.
- **Validate** every file against the schemas and the rules in the method.
- **Derive** what the files leave to Git and to inheritance: resolved junctions, requirement conditions, authorisation, review, roll-up, history and subproject status.
- **Audit** the discrepancies between files and history as actionable findings.
- **Serve** every abstract view as JSON through the `tablo` command, with a role filter and focusing parameters.

```
tablo validate                       # diagnostics for the working tree
tablo audit --ref v1.0               # findings at a tag
tablo tableau --json                 # the global tableau as data
tablo queue --person ada@example.org --json
tablo trailer reviewed 9f31 design   # the trailer line a porcelain commits
```

## Layout

The module is `github.com/nbyoung/tablo`. Its shape is the template every
front end in the family copies.

```
.
├── go.mod                     # the module; the go directive pins the toolchain
├── doc.go                     # package tablo: the library
├── version.go                 # Version and the tableaux versions the module accepts
├── version_test.go
├── params.go                  # the views, the levels, the roles and the parameters a view takes
├── errors.go                  # the usage, not-found and read errors, which fix the exit code
├── envelope.go                # the envelope every command's data travels in, and its JSON
├── envelope.schema.yaml       # the envelope's schema, embedded
├── yaml.go                    # the same envelope as YAML
├── trailer.go                 # the four commit trailers as Git writes them
├── cmd/tablo/main.go          # command tablo: the plumbing command
├── internal/model/            # the typed model of a project as its files state it, with positions
├── internal/git/              # the one place that runs the git executable
├── internal/load/             # the Loader: one project from the working tree or a revision, and its links
├── internal/validate/         # the Validator: the rules of the method's RULES.md over a loaded project
├── internal/history/          # the one pass over the Git history that the Derivation reads
├── internal/derive/           # the Derivation: the facts the files leave to Git and to inheritance
├── internal/cli/              # the command's argument parser, plain forms and usage text
├── schemas/                   # the tableaux schema files, embedded into the binary
├── .tableaux/                 # this project's plan
├── .github/workflows/
│   ├── ci.yml                 # format, vet, lint and test on every push and pull request
│   └── release.yml            # build and publish on a tag
├── .goreleaser.yaml           # the six targets and the release archives
└── .golangci.yml              # the lint set
```

`schemas/` holds a copy of the schema files that the
[tableaux](https://github.com/nbyoung/tableaux) schema-files task produces;
`schemas/README.md` records the commit copied, and `schemas/embed.go` embeds
the five files into the binary.

A prototype under `prototype/` is the function gate's demonstration. It goes when its task records `implementation`: the design's account of what it kept from the prototype and `git log -- prototype/<id>` keep what it showed, and the trunk builds what it ships.

## Build and test

Go builds the module with no other tool; the version in `go.mod` decides which
toolchain `go` fetches.

```
go build ./...                       # the library and the command
go test ./...                        # the tests
go run ./cmd/tablo version           # prints the version
gofmt -l . && go vet ./...           # what CI checks first
golangci-lint run                    # the lint set in .golangci.yml
```

CI runs those four checks on every push and pull request. Contributors work
from the sibling checkout beside the `tableaux` repository; a personal
`go.work` there builds the family together and stays uncommitted.

## Release

A tag `v<major>.<minor>.<patch>` releases. The release workflow runs
[GoReleaser](https://goreleaser.com) from `.goreleaser.yaml`, which
cross-compiles `cmd/tablo` with `CGO_ENABLED=0` for Linux, macOS and Windows on
amd64 and arm64 and publishes one archive per target and a checksum file to the
GitHub release for the tag. No user installs Go: they fetch one file from the
release page.

```
git tag v0.1.0 && git push origin v0.1.0
```

`Version` in `version.go` names the release the next tag makes; bump it in the
same change that tags.

## Versions accepted

`version.yaml` in a project states the tableaux language version its files
follow. The module accepts a project whose major version equals its own and
whose minor version does not exceed it, as the method's version rule states.
This module accepts major `0` and minor up to `3`, so `0.0.x`, `0.1.x`, `0.2.x` and `0.3.x`
pass and `0.4.0` and `1.0.0` do not. `Accepts` in the root package applies the
rule and the loader task calls it.

## Dependencies

The owner decided at the function gate, after every prototype wrote its own
YAML reader, that the implementation takes two libraries, which make three
modules:

| Module                                    | Version | Purpose                                                        |
|-------------------------------------------|---------|----------------------------------------------------------------|
| `gopkg.in/yaml.v3`                        | v3      | Parse project files; `yaml.Node` keeps the line and column of every value |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6      | Validate files against the embedded JSON Schema 2020-12 schemas |

The schema module brings a third, `golang.org/x/text` v0.14.0 (BSD-3-Clause),
as an indirect requirement that the binary links: it holds the message
catalogue of the schema module, whose wording the Validator never prints.

All three are pure Go, so the six release targets still build with `CGO_ENABLED=0`.
`go.mod` requires all three; `go` fetches the modules from the module proxy,
and no one installs anything by hand. The prototypes under `prototype/` predate the decision and
keep their stand-in readers.

## Plan

The project's plan is the Tableaux project in [`.tableaux/`](.tableaux/). The
[Tableaux tooling plan](https://github.com/nbyoung/tableaux/blob/main/PLAN.md)
in the `tableaux` repository pins this project as the submodule
`subprojects/tablo` and tracks its root task through a recursive junction, states the review policy every task here inherits, and
proposes Go as the implementation language.

## Licence

[MIT](LICENSE).
