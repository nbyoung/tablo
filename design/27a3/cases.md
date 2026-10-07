# Design 27a3: the cases the corpus lacks

The tables of [design 27a3](../27a3.md) that a test builds in memory, with no repository: a project of a few task and status files, composed through `load.Parse` and `load.Compose`. Each row states the files that matter and what the Derivation gives. The throwaway trial of the design gives every row as written. [`cases.sh`](cases.sh) and [`cases.expected.txt`](cases.expected.txt) hold the cases that need a history.

The gates are `undefined`, `defined`, `mockup`, `function`, `design`, `release`. The states are `undefined` 0, `nominal` 1, `stalled` 3, `complete` 0. `r000` is the root; a child's place is its `order`.

## Roll-up

| #   | Children of `r000`, in display order                                                         | `r000` rolls up to            | Shows                                                         |
|-----|-----------------------------------------------------------------------------------------------|-------------------------------|---------------------------------------------------------------|
| R1  | `a000` at function, nominal; `b000`, exempt from mockup and function, at defined, nominal    | defined, nominal, from `b000` | A child exempt from early gates enters by its own gate        |
| R2  | `a000` at mockup, stalled; `b000`, exempt from mockup, at design, nominal                     | mockup, stalled, from `a000`  | A gate that does not apply to a child never enters for it     |
| R3  | `r000` itself exempts mockup; `a000` states `mockup: {}` and stands at mockup, nominal; `b000` at design, nominal | mockup, nominal, from `a000` | The parent's gate is a child's, whatever the parent's own junction there |
| R4  | `a000` at release, complete; `b000`, exempt from release, at design, complete                 | design, complete, from `b000` | All complete: the earliest last gate decides                  |
| R5  | `a000` at release, complete; `b000` with no status file                                       | undefined, undefined, from `b000` | Any undefined child gives an undefined parent             |
| R6  | `a000` at defined, nominal; `b000` at design, stalled                                         | defined, nominal, from `a000` | The earliest gate decides before severity does (F14)          |
| R7  | `b000` (order 1) and `a000` (order 2), both at design, nominal                                | design, nominal, from `b000`  | A tie goes to the first in display order (F21)                |
| R8  | `a000` at defined, state `paused`; `b000` at gate `nowhere`; `c000` at design, nominal        | design, nominal, from `c000`; considered `c000` | A state gates.yaml lacks ranks 0; a gate it lacks leaves the child out |
| R9  | `a000` at defined, state `paused`, alone                                                      | defined, `paused`, from `a000` | With no child of non-zero severity, all children count       |
| R10 | `p000`, a parent of `a000` at mockup, stalled; `b000` at mockup, nominal                      | mockup, stalled, from `p000`; the chain ends at `a000` | `From` is the direct child; `Chain` reaches the leaf |

## Junctions

Each row resolves the junction of the leaf `a000` at design. `a@x` is its assignee unless the row says otherwise.

| #  | Entries at design, nearest last                                                  | Resolves to                                                                                  | Marks      |
|----|----------------------------------------------------------------------------------|----------------------------------------------------------------------------------------------|------------|
| J1 | `r000`: contributor `bot@x`, model `m`; `a000`: contributor `carol@x`            | Plain; contributor `carol@x` by `a000`; no model; no reviewer                                | 🧑         |
| J2 | `r000`: reviewer `rev@x`; `a000`: contributor `bot@x`, model `m`                 | Plain; contributor and model by `a000`; reviewer `rev@x` by `r000`; sources `a000`, `r000`   | 🤖👀       |
| J3 | `r000`: contributor `bot@x`, model `m`; `a000`: none                             | Plain; contributor and model by `r000`; reviewer `a@x` by assignee                           | 🤖👀       |
| J4 | As J3, and the assignee of `a000` is `bot@x`                                     | Plain; reviewer `bot@x` by assignee                                                          | 🤖         |
| J5 | `r000`: reviewer `rev@x`; `p000`: `applies: false`; `a000`: `{}`                 | Plain; contributor `a@x` by assignee; no reviewer: the field above the exemption does not pass | 🧑       |
| J6 | As J5, and `a000` states no entry                                                | Not applicable, by `p000`                                                                    | —          |
| J7 | `r000`: reviewer `rev@x`; `p000`: a recursive entry (J3, an error); `a000`: none | Plain; reviewer `rev@x` by `r000`: a recursive entry does not inherit                        | 🧑👀       |
| J8 | `r000`: reviewer `rev@x`; `a000`: `applies: false` beside a contributor (J4, an error) | Plain; contributor `a@x` by assignee; reviewer `rev@x` by `r000`: the mixed entry is no entry | 🧑👀  |
| J9 | `a000`: reviewer `a@x`                                                           | Plain; reviewer `a@x` by `a000`                                                              | 🧑         |

## Refusals

The built corpus holds 119 repositories. The Derivation refuses these 18, by the part it cannot read, and derives every fact of the other 101 without a fault.

| Part      | Entries                                                                                                                              | Rules that report it      |
|-----------|--------------------------------------------------------------------------------------------------------------------------------------|---------------------------|
| `project` | `no-tableaux-directory`                                                                                                              | P1                        |
| `version` | `version-missing`, `version-bad-pattern`, `version-major-mismatch`, `version-minor-ahead`                                           | P2, P3, P4                |
| `gates`   | `gates-missing`, `gates-only-undefined`, `gates-first-not-undefined`, `gates-duplicate-gate`                                        | G1, G2, G3, G4, G6        |
| `states`  | `gates-no-states`, `gates-negative-severity`, `gates-duplicate-state`, `gates-no-undefined-state`, `gates-complete-nonzero-severity` | G7, G8, G9, G12           |
| `tree`    | `tree-no-root`, `tree-two-roots`, `tree-parent-missing`, `tree-parent-cycle`                                                        | T8, T9, T10               |
