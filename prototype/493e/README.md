# Prototype 493e: structural views as data

Task `493e` Structural views, gate `function`.

## The question

Can a tool derive the gate definition, task definition, authority delegation and task assignment views as JSON from a project and its Git history alone, following the data and parameters of the draft VIEWS.md, and agree with the conformance corpus on the facts the views show?

The riskiest parts are the facts the views draw on that the files leave to Git: authorisation from the trunk's first-parent history, junction fields resolved through the ancestors, a status overlaid from a submodule at its pin, and requirement conditions. The prototype derives all of them for `weather-station` and renders the four views over them.

## Run

```
export PATH=$PATH:/usr/local/go/bin
go test ./prototype/493e
go run ./prototype/493e -repo <corpus>/weather-station -ref main -view authority -level glance
```

Flags: `-view gate|task|authority|assignment`, `-task`, `-person`, `-level glance|detail|provenance` (default provenance, which includes the other two), `-columns a,b`, `-window n`, `-proposed`, `-ref`. The output is one JSON object: `view`, `ref`, `on_trunk`, `params`, then `glance`, `detail` and `provenance`, each holding what its level adds. Levels nest by inclusion.

## What it shows

Authority delegation at `main`, glance (the dot is the parent's assignee):

```
a1c0 Weather station  ·
  4e2b Sensor node      ben@example.org   authorised
    9f31 Sensor board     ada@example.org   authorised
    c07d Node firmware    ·                 authorised
  7b2e Gateway          ·                 authorised
  3c5d Dashboard        dan@example.org   proposed
```

Task assignment at glance (default window: the next gates in view):

```
ada@example.org   assigned 3  contributes next 2  reviews next 0
ben@example.org   assigned 2  contributes next 0  reviews next 0    (c07d's next junction is recursive)
dan@example.org   assigned 1  contributes next 1  reviews next 0
opus@example.org  assigned 0  contributes next 0  reviews next 0  models claude-opus-5-5
```

Task `c07d` at glance carries the status of `f1a0` at the pinned firmware commit: gate `design` from its own file, `nominal`, the note and 2026-09-17 from the submodule, and a `snapshot` with the url, pin and task. Its requirement on `9f31` reads `met: false, due: true, condition: unmet`. At provenance, `9f31` lists the events task, authorised, status, reviewed, status, reaffirmed, on the commits `expected.yaml` names (W3, W4, W7, W9, W10, W13), and its authorisation names the merge `931c43d` by ben@example.org. On the branch commit W3 every task reads proposed and the tasks whose file differs from the trunk's are marked.

The tests (`main_test.go`) compare the derived facts with `expected.yaml` for authorisation, status and date, the snapshot, requirement conditions, reviews, events, junction sources, the branch view and the view parameters. They skip when the corpus is absent.

## Stand-ins

Inside this directory, since the loader and derivation (tasks `27a3` and its neighbours) are not built:

- `yaml.go`: a parser for the YAML subset the files use (flow maps and lists, folded blocks, block lists and maps). The implementation needs a YAML library, which `go.mod` does not yet require.
- `project.go`, `history.go`: the loader through `git ls-tree` and `git show`, the tree, junction resolution, authorisation, status with roll-up and subproject overlay, requirements, reviews and events. The code is larger than a prototype should be (about 1,400 lines beside the tests) because the four views need these facts.

## What it leaves out

The other views. Validation and findings. Models (the `Model:` trailer check). Events of a subproject and the change of a pin. The `role` parameter and its per-role level. Ranges. Text output. Speed: it runs `git` once or more per task and per fact.

## Findings

- **Roll-up conflicts with the corpus.** README.md takes the earliest gate among the considered children. `4e2b` has children `9f31` (function, stalled) and `c07d` (design), so the rule gives function, stalled, from `9f31`. `expected.yaml` and corpus/README.md give design, nominal, from `c07d`. The prototype follows the rule and the test records the difference. This is not the F14 finding, which assumes the corpus is right.
- **The `pending` condition.** `expected.yaml` states requirement conditions as met, unmet or pending; VIEWS.md lists "met, due, unmet". The prototype emits `met` and `due` as booleans and `condition` as the corpus has it.
- **Off the trunk.** The corpus treats the commit W3, which is reachable from the trunk but off its first-parent line, as off the trunk. The prototype does the same: a ref is on the trunk when it lies on the trunk's first-parent line.

## Where the prototype depends on a VIEWS.md choice still open

- **Q1 Three levels.** Every view has `glance`, `detail` and `provenance`, nested; the names are the keys of the JSON. A rename changes the keys only.
- **Q2 The window.** The default window differs by view in the prototype because VIEWS.md is unclear: the gate definition and task definition show every gate by default (their examples do), and the assignment view spans the next gates in view with no column either side (its Parameters). The general rule says one either side for every view. `-window n` adds n either side. A folded column is a count (`folded`) in the gate view only.
- **Q4 and the marks.** The gate view lists five marks, including the 🧑 mark that VIEWS.md adds beyond README.md (finding F23). If the method drops 🧑, the marks list and the plain-contributor mark change.
- **Requirement condition.** Whether "met" means the originating task has reached the `from` gate (the prototype) or stands at exactly it, and the name of the third condition.
- **Assignment counts.** Assigned counts every task including parents; junction counts cover leaves only, since a parent's junctions are defaults. The VIEWS.md example (10 and 27 assigned; 32 next junctions) fits this, but the text does not say it.
- **A person's models.** Taken over every gate whatever the window; the text does not say.
- **Authority of a person.** With `person`, the prototype keeps the tasks the person has authority over plus the ancestors of the tasks assigned to the person, and lists per email the top-most assigned parents with their descendant counts as "subtrees it has authority over".
- **Off-trunk marking.** "Tasks whose file differs from the trunk's" is a blob comparison with the trunk's tip; a task new on the branch differs.
- **The way a commit accepts.** The prototype reports `merge` before `trailer` before `commit`, since the corpus calls W4 a merge though it also carries `Authorised:`.
- **Q3, Q5, Q6, Q7 and the brief.** None of the four views depends on them. The queue, tableaux and audit views do.

## What the design gate must decide

- Whether the JSON keys above become the data contract between `tablo` and the front ends, or the views return typed structures that the schema files describe.
- The roll-up rule and the corpus's expected value for `4e2b`.
- The default window per view, and the meaning of "condition".
- Whether a view carries its own text (the mark meanings) or only keys and symbols.
- The YAML library, and whether one pass over `git log` replaces the per-fact `git` calls.
