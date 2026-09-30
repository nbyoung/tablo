# Prototype dada: the audit

## The question

Can the audit find the discrepancies between the files and the history, and report each as a finding that names the task, the gate, the commit and the action that resolves it? The risky part is that the answer comes from the history (`git log` with changed files and trailers), not from the files, and that each finding needs a gate and a commit to name.

## Run

```
go run ./prototype/dada [-labels file] <repo> [ref]
go test ./prototype/dada
```

The command reads the repository through `git ls-tree`, `git show` and `git log`, never the working tree. `ref` defaults to `main`. It reads `<repo>.labels.txt` when present and prints the corpus label beside each hash. The test compares the audit with `expected.yaml` for `review-by-non-reviewer`, `model-mismatch`, `unknown-trailer`, `status-unreviewed-gate` and `weather-station` (main and the branch `sensor-board`), and skips when the corpus is absent. A further test builds a repository in a temporary directory to check the D8 reading.

## What it shows

Findings covered, by the rule or fact that fixes them:

| Finding                | Source                              | Reading                                                                                              |
|------------------------|-------------------------------------|------------------------------------------------------------------------------------------------------|
| proposed task          | derived fact (`authorisation`)      | The deciding commit on the first-parent line is by none of the task's authorities, or the ref is off the trunk |
| status past unreviewed junction | S11                        | The status gate passes a reviewed junction with no `Reviewed:` commit by its reviewer                |
| review by non-reviewer | H2                                  | A `Reviewed:` commit by someone who is not the junction's reviewer                                   |
| model mismatch         | H3                                  | A `Model:` trailer not prefixed by the junction's `model`; a task-file edit counts as work at `defined` (D8), a status change as work at the status's gate |
| trailer names nothing  | H1                                  | An `Authorised:`, `Reaffirmed:` or `Reviewed:` trailer for an unknown task or a gate that does not apply |

Real output, from the corpus (labels come from `build/<entry>.labels.txt`):

```
$ go run ./prototype/dada corpus/build/review-by-non-reviewer
| review-by-non-reviewer | H2 | b2c9 | design | R2 051c1b6 | olive@example.org commits `Reviewed: b2c9 design`; pat@example.org's commit has no effect |

$ go run ./prototype/dada corpus/build/model-mismatch
| model-mismatch | H3 | b2c9 | design | M2 dcce353 | Redo the work under claude-fable, or state claude-sonnet-5 as the junction's model |

$ go run ./prototype/dada corpus/build/weather-station
| proposed   |     | 3c5d | defined | W12 b426894 | An authority (ada@example.org) commits `Authorised: 3c5d` |
| unreviewed | S11 | c07d | mockup  | W11 303c805 | ben@example.org commits `Reviewed: c07d mockup`, or the status returns to the last reviewed gate |

$ go run ./prototype/dada corpus/build/unknown-trailer
| unknown-trailer | H1 | | | U2 9c471d7 | `Authorised: zzzz`: Name a task in the project ... |
| unknown-trailer | H1 | | | U2 9c471d7 | `Reviewed: 9f31 nowhere`: Name a task in the project ... |
```

(The output above drops the header row; the command prints `| Finding | Rule | Task | Gate | Commit | Action |`.)

Against `expected.yaml`, every H1, H2, H3 and S11 finding matches by rule, task, gate and commit, and every task whose authorisation the file states matches in state and deciding commit. `weather-station` on the branch `sensor-board` reads every task as proposed at W3.

One difference: the audit reports S11 for `weather-station` task `c07d` at `mockup`. The status stands at `design`, the inherited `mockup` reviewer is `ben`, and the history holds no `Reviewed: c07d mockup`. `expected.yaml` states no S11 finding and `valid: true`. The test asserts this finding by name so the difference stays visible.

## What it leaves out

- Findings for unmet requirements, stale statuses, agent contributions without a human reviewer, and subprojects that do not resolve. They follow the same shape: a derivation, then a finding with task, gate, commit and action.
- YAML: no library is available, so `yaml.go` parses the subset the audit reads: top-level scalars, `parent: {...}`, and one-line flow mappings under `junctions:`. The implementation needs a YAML library (`gopkg.in/yaml.v3`) and line positions from the loader task.
- Stand-ins for the loader and the derivation (`Project`, `Junction`, `Authorities`) that stay in this directory. They resolve junctions field by field from the task and its ancestors, and treat a junction with a `model` as an agent whose missing reviewer is the assignee (D4).
- Trunk resolution beyond `version.yaml` and the caller's ref; `Reaffirmed:` dating; replay of S11 at each commit (F15: the audit applies S11 at the tip).
- Severity, `--at` refs other than branches, and JSON output.

## What the design gate must decide

- Whether `weather-station` `c07d` is a corpus error (add `Reviewed: c07d mockup` to the history, or state the S11 finding) or the method exempts it (for example a recursive next junction).
- How a `Model:` trailer on a commit that changes several tasks or gates attributes its work: this prototype checks each pair the commit touches.
- Whether a commit that carries `Reviewed:` or `Reaffirmed:` also counts as work at that gate for H3 (this prototype says yes).
- Whether the audit keeps H2 for a junction with no reviewer (this prototype says no) and for `defined` (no: authorisation stands).
- Whether an agent is a junction with a `model`, or a contributor with `Co-Authored-By` in history (D4).
- Whether the finding's `commit` for S11 is the status's deciding commit (this prototype) or the newest commit at the unreviewed gate.
