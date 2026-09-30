# Prototype 886d: status views

Task `886d` builds the global tableau, the contextual tableau, the work-blockage tree and the contributor work queue as data. This prototype answers the two risky questions in it: how children roll up to a parent, and how the default gate window around the next gates in view folds the other columns into counts.

It reads the corpus entry `weather-station` at `main` and prints each view as JSON.

## Run

```
go test ./prototype/886d                                  # skips when the corpus is absent
go run ./prototype/886d tableau                           # global tableau (-level glance for depth one)
go run ./prototype/886d contextual -task 4e2b             # contextual tableau, task form
go run ./prototype/886d contextual -person ben@example.org  # person form
go run ./prototype/886d blockage
go run ./prototype/886d queue -person ada@example.org
```

Flags: `-repo` (default: the corpus build path), `-ref`, `-task`, `-person`, `-window N` (default 1), `-cell state-at-gate|state-at-next`, `-order kind|dependents`, `-level glance|detail`.

The full JSON for each view sits in `output/`: `global-tableau.json`, `global-tableau-window0.json`, `global-tableau-state-at-next.json`, `contextual-task-4e2b.json`, `contextual-person-ben.json`, `work-blockage-tree.json`, `queue-ada.json`, `queue-ben.json`, `queue-dan.json`.

## What it shows

Global tableau, `main`, default window. The next gates in view are defined, mockup, performance and implementation, so the window runs from undefined to unit and the columns from integrate to release fold to a count of zero. The note after each row gives its date and source.

```
id    task            ❔    📝    📌    ⚙️   ⚡    ⚓    📐    🛠️   🧩
a1c0  Weather station .    🟢    🧑    🧑    🧑    🧑    🧑    🧑    🧑    rolls up from 3c5d, 2026-09-17
4e2b    Sensor node   .    🧑    🧑    🔴⛔   🧑    🧑    🧑    🧑    🧑    rolls up from 9f31, 2026-09-17
9f31      Sensor board.    🧑    🧑👀   🔴⛔   🧑    🧑    🧑    🧑    🧑    2026-09-28 (reaffirmed)
c07d      Node firmware.   🧑    🧑    🧑    🧑    —    🟢    🪆    🤖👀   2026-09-17 (subproject at F2)
7b2e    Gateway       ⚪    🧑    🧑    🧑    🧑    🧑    🧑    🧑    🧑    2026-09-15
3c5d    Dashboard     .    🟢    🧑    🧑    🧑    🧑    🧑    🧑    🧑    2026-09-22
folded after: integrate..release, count 0
```

With `-window 0` the columns run from defined to implementation, and the column before folds to a count of 1 (`7b2e` stands at undefined). The contextual tableau for `4e2b` has next gates performance and implementation, shows function to unit and folds undefined..mockup and integrate..release to zero. The person form for `ben@example.org` gives `c07d` (corner), `9f31` (sibling), `4e2b` and `a1c0` (spine), the set README.md names.

Roll-up, from the tests: `a1c0` is defined, nominal from `3c5d`, dated 2026-09-17. `4e2b` is function, stalled, blocked from `9f31`, dated 2026-09-17, the oldest of its two children. `c07d` shows its own gate `design` with state, note, date and recorder from `f1a0` at the pin `F2`.

Work-blockage tree, three causes, largest first:

```
9f31 Sensor board is stalled at function       ada    holds 2:  9f31 at performance; c07d at implementation
9f31 Sensor board has not passed design        ada    holds 1:  c07d at implementation
3c5d Dashboard is proposed                     ada    holds 1:  3c5d at mockup
not yet due: 3c5d requires 7b2e (function to integrate)
```

`c07d` appears under two causes, as the view definition says. Queue for `ada@example.org`: authorisation owed `3c5d`; work ready `7b2e` at defined; reaffirmation `9f31` (2026-09-28); work waiting `9f31` at performance (blocked). `ben@example.org` has an empty queue (`c07d`'s next junction is recursive, so it names no contributor). `dan@example.org` has `3c5d` ready at mockup and a reaffirmation.

## Finding: the corpus disagrees with the roll-up rule

README.md#status says the parent takes the earliest gate among its considered children. `9f31` stands at `function` and `c07d` at `design`, and `function` comes first, so `4e2b` rolls up to function, stalled, blocked from `9f31`. The corpus (`expected.yaml`, and corpus/README.md line 159) says design, nominal from `c07d`, and its finding F14 and VIEWS.md describe `9f31` as "at a later gate". The prototype follows the rule. The test asserts the rule's result, and the root agrees with `expected.yaml` either way. The owner must decide whether the rule or the corpus is wrong; the tableau and the parent's tie-break depend on it.

## Points that depend on a VIEWS.md choice still open

- **Q2, the gate window.** `-window N` sets the columns either side of the next gates in view; the default is 1. `output/global-tableau-window0.json` shows the alternative. The fold count counts leaf tasks in view by their current (status) gate; VIEWS.md does not say whether parents count, so the prototype counts leaves only, which avoids counting a task twice. A parent's own next gate joins the window, which adds no column in this project.
- **Q3, the queue order.** `-order kind` is the default (reviews, authorisations, work ready, reaffirmations, work waiting). `-order dependents` sorts work ready by direct dependents. `weather-station` has one work-ready item for ada, so the corpus cannot show a difference.
- **Q5, the cell rule.** `-cell state-at-gate` puts the state in the column of the gate the status names (default). `-cell state-at-next` puts it in the next gate's column (`output/global-tableau-state-at-next.json`). Both leave the other cells as marks. Under the second rule the window and fold counts do not change.
- **Q7, the person form.** `contextual -person` selects the person's leaf tasks (assigned, or contributor at the next junction) as corner, the ancestors as spine and the corner's siblings. It reads leaf tasks only, so `4e2b` (assigned to Ben) is spine, as README.md's example says; VIEWS.md leaves this open.

Other choices the prototype made where VIEWS.md is silent:

- A proposed task with an accepted status still counts as work ready for its contributor (`3c5d` for `dan`).
- The mark for a person contributor is 🧑 (finding F23); 👀 follows when the reviewer differs from the contributor, and an agent with no stated reviewer takes the assignee.
- The resolver of an unmet requirement or a stalled status is the contributor at the task's next junction; of a proposed task, the nearest authority.
- A held task holds only the dependents whose requirement on it is unmet; a not-yet-due requirement is listed apart.
- Roll-up ties take the first child in display order (F21); the corpus has no tie.

## Stand-ins and what it leaves out

The views need other tasks' functions. This directory holds a minimum of each:

- `yaml.go`: a reader for the YAML subset these files use. No third-party module exists in `go.mod`. The implementation needs `gopkg.in/yaml.v3` (with line positions).
- `project.go`: the loader (`git ls-tree` and `git show` at a ref), junction resolution, applicable gates, status with date and recorder from `git log`, roll-up, requirement conditions, authorisation on the first-parent line (authorities read at the tip, not at the deciding commit), and the subproject snapshot (submodule path from `.gitmodules`, pin from `git ls-tree`).
- No validator, no role filter, no `person` filter on the window, no `ref` off the trunk beyond "all proposed", no `columns` list, no `task`+`person` mix.
- The blockage tree derives three of five causes. A review outstanding needs a status with the reason `review` and a pin that has not advanced needs a moving subproject; the weather station has neither, so the code omits both and says so in the output (`not_derived`).
- The queue omits the brief and the level split; it emits every field at once.

## What the design gate must decide

- Whether the roll-up rule or the corpus expectation for `4e2b` stands (the finding above).
- Q2, Q3, Q5 and Q7 above, and whether a fold counts leaves only.
- Whether a proposed task is ready work, and who resolves each blockage cause.
- The JSON shape: the cell list per row, the `folded` blocks and the `tree` of held tasks are this prototype's; the front ends need a fixed schema.
