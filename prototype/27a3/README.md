# Prototype 27a3: derivations that read Git history

Task `27a3` (Derivation), gate `function`.

## The question

Can a tool derive, from a plain Git repository and the files at one commit, the
three facts the derivation task calls riskiest, and agree with the corpus?

1. Each task's authorisation from the deciding commit on the trunk's first-parent line.
2. Each reviewed junction's completion from `Reviewed:` trailers by the right person.
3. Each junction resolved through the ancestor chain, with the provenance of every field.

The answer is yes: on `weather-station`, `junction-kinds`, `trunk-stated`,
`trunk-inferred` and `trunk-undetermined` the prototype agrees with every
authorisation, authority list, resolved junction, applicable list and review
that `expected.yaml` states (74, 104, 10, 9 and 2 checks, no disagreement).

## Run

```
export PATH=$PATH:/usr/local/go/bin
B=/home/nbyoung/Projects/Tableaux/tableaux/corpus
go run ./prototype/27a3 -repo $B/build/weather-station \
   -labels $B/build/weather-station.labels.txt \
   -expected $B/entries/weather-station/expected.yaml
go test ./prototype/27a3          # skips when the corpus is not built
```

Flags: `-ref` (branch in view, default `HEAD`), `-trunk` (the caller's branch,
used only when the project and `origin/HEAD` name none).

## What it shows

```
task 9f31 parent="4e2b" assignee=ada@example.org
  authorised by deciding commit W4 (ben@example.org); authorities [ben@example.org ada@example.org]
  mockup         plain contributor=ada@example.org<default> reviewer=ben@example.org<4e2b>  source=[4e2b]
  validate       plain contributor=ada@example.org<default> reviewer=ben@example.org<9f31>  source=[9f31]
  status gate function
  review mockup by ben@example.org: W9

task 3c5d ...
  proposed by deciding commit W12 (dan@example.org); authorities [ada@example.org]

task a110 (junction-kinds)
  unit    plain contributor=bot@example.org<a110> model=claude-sonnet<a110> reviewer=olive@example.org<a100>  source=[a110 a100]
task a210
  release plain contributor=pat@example.org<a210> undecided=F7  source=[a210]
```

- Authorisation. `git log --first-parent -1` over the task file and over
  `--grep '^Authorised: <id>$'` gives two candidates; the one nearer the tip is
  the deciding commit. The authorities come from the tree at that commit. The
  `--no-ff` merge W4 decides `9f31` (the branch commit W3 is off the line);
  dan's W12 revision returns `3c5d` to proposed after ada's trailer at W6. A
  view that is not the trunk tip reads every task as proposed, as does an
  undetermined trunk (P5 warning printed).
- Trunk. Order: `version.yaml` `trunk` (`trunk-stated`: `develop`), then
  `refs/remotes/origin/HEAD` (`trunk-inferred`), then the caller, else undetermined.
- Review. For each junction the status gate passes, the reviewer is resolved
  (an agent with no stated reviewer takes the assignee), and the log of the view
  is searched for `Reviewed: <id> <gate>` with the reviewer as author or
  committer. A trailer from anyone else lands in `Ignored`. `defined` is skipped
  when the task is authorised. `a300`'s self-review rides on K8.
- Junctions. The task's own entry, then ancestors nearest first, field by
  field; contributor and model travel together; a not-applicable entry
  exempts, and stops the walk; a recursive entry never inherits. Each field
  carries its supplying task (`<a100>`) or `default`. `a210` restates release
  under `a200`'s exemption, so its reviewer stays open and the prototype marks it `F7`.

## Findings for the owner

- `weather-station` `c07d` stands at `design` and passes a `mockup` junction whose
  reviewer is `ben` (inherited from `4e2b`), yet no `Reviewed: c07d mockup` commit
  exists and `expected.yaml` states no S11 finding. The prototype reports it as
  `UNREVIEWED`. Either the entry needs a finding or the review, or the method needs
  a rule for it.
  *Resolved 2026-09-30: the owner ruled README.md right and the corpus now carries `Reviewed: c07d mockup` at W11 (tableaux `1b0c002`).*
- `trunk-stated` `expected.yaml` lists `b2c9` as proposed on `main`, but `main`
  does not hold that task (only `develop` does). The prototype prints a NOTE
  and does not count it.

## What it leaves out

Requirement conditions, status date and recorder, roll-up, events, recursive
status at the pin, and every validator finding (H1 to H3 appear only as `Ignored`).
The YAML reader is a hand-written subset (block maps and lists, flow collections,
folded scalars, comments) with no anchors or escapes; the implementation needs a
real YAML module (`gopkg.in/yaml.v3`) that keeps line positions. It runs one
`git` process per lookup; the implementation needs `git cat-file --batch` and one
`rev-list --first-parent` pass. The single-file `expected.yaml` comparison covers
authorisation, authorities, junctions, applicable gates and reviews only.

## The design gate must decide

- The deciding commit's judge: author or committer (the prototype accepts either).
- Which `Reviewed:` commit is reported when the reviewer signs twice (the prototype
  takes the oldest).
- Root authority: the prototype takes the owner from the tree before the deciding
  commit, so a hand-over is accepted by the outgoing owner.
- A view is "on the trunk" only when it equals the trunk tip; a tag or an older
  commit of the trunk reads as proposed.
- F7 and the two findings above.
