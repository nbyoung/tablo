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
tablo audit --at v1.0                # findings at a tag
tablo view tableau --json            # the global tableau as data
tablo view queue --for ada@example.org --json
tablo trailer reviewed 9f31 design   # the trailer line a porcelain commits
```

## Plan

The project's plan is the Tableaux project in [`.tableaux/`](.tableaux/). The
[Tableaux tooling plan](https://github.com/nbyoung/tableaux/blob/main/PLAN.md)
in the `tableaux` repository pins this project as the submodule
`subprojects/tablo` and tracks its root task through a recursive junction, states the review policy every task here inherits, and
proposes Go as the implementation language.

## Licence

[MIT](LICENSE).
