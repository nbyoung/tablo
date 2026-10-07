#!/bin/sh
# cases.sh <directory>: builds the repository of design 27a3's cases, the
# history the corpus has no entry for, and writes <directory>.labels.txt.
# Identities, dates and messages are fixed, as in the corpus's lib.sh, so the
# hashes repeat. cases.expected.txt states what a tool derives at each label.
set -eu
REPO=$1
rm -rf "$REPO"
git init -q -b main "$REPO"
git -C "$REPO" config commit.gpgsign false
: > "$REPO.labels.txt"

who() { export GIT_AUTHOR_NAME="$1" GIT_AUTHOR_EMAIL="$1@example.org" GIT_COMMITTER_NAME="${2:-$1}" GIT_COMMITTER_EMAIL="${2:-$1}@example.org"; }
on() { export GIT_AUTHOR_DATE="${1}T12:00:00+00:00" GIT_COMMITTER_DATE="${1}T12:00:00+00:00"; }
put() { mkdir -p "$REPO/$(dirname "$1")"; cat > "$REPO/$1"; }
label() { printf '%s %s\n' "$1" "$(git -C "$REPO" rev-parse HEAD)" >> "$REPO.labels.txt"; }
commit() { _l=$1; _s=$2; shift 2; git -C "$REPO" add -A; git -C "$REPO" commit -q --allow-empty -m "$_s" "$@"; label "$_l"; }
status() { put .tableaux/status/b2c9.yaml; }
leaf() { # leaf <title> <reviewer at design, or ''>
  { printf 'title: %s\ndescription: A leaf.\nassignee: pat@example.org\n' "$1"
    [ -z "$2" ] || printf 'junctions:\n  design: { reviewer: %s@example.org }\n' "$2"
    printf 'parent: { id: "e4a1", order: 1 }\n'; } | put .tableaux/tasks/b2c9.yaml
}
root() { printf 'title: Cases\ndescription: The cases of design 27a3.\nassignee: %s@example.org\n' "$1" | put .tableaux/tasks/e4a1.yaml; }

put .tableaux/version.yaml <<'Y'
tableaux: 0.3.1
trunk: main
Y
put .tableaux/gates.yaml <<'Y'
gates:
  - { key: undefined, symbol: ❔, name: Undefined, criteria: No one has started work on the definition }
  - { key: defined,   symbol: 📝, name: Defined,   criteria: "Title, description, assignee and references exist" }
  - { key: design,    symbol: 📐, name: Design,    criteria: A model and sufficient tests exist }
  - { key: release,   symbol: 🚀, name: Release,   criteria: All variants documented and approved }
states:
  - { key: undefined, symbol: ⚪, severity: 0, synopsis: The work has not yet been defined }
  - { key: nominal,   symbol: 🟢, severity: 1, synopsis: The work is proceeding as expected }
  - { key: complete,  symbol: ✅, severity: 0, synopsis: All deliverables satisfy their requirements }
reasons:
  - { key: review, symbol: 👓, synopsis: The work waits for its reviewer }
Y
root olive
leaf Leaf ''
status <<'Y'
gate: defined
state: nominal
Y
who olive; on 2026-09-01; commit C1 'Plan the cases'

# C2, C3: a branch proposes a task, and the owner merges it with no trailer.
git -C "$REPO" checkout -q -b proposal
put .tableaux/tasks/c3d7.yaml <<'Y'
title: Second leaf
description: A leaf a contributor proposes.
assignee: pat@example.org
parent: { id: "e4a1", order: 2 }
Y
who pat; on 2026-09-02; commit C2 'Propose the second leaf'
git -C "$REPO" checkout -q main
who olive; on 2026-09-03
git -C "$REPO" merge -q --no-ff --no-edit -m 'Merge the second leaf' proposal; label C3

# C4: the author is no authority and the committer is.
leaf 'Leaf, revised' ''
who pat olive; on 2026-09-04; commit C4 'Revise the first leaf'

# C5: an Authorised: trailer from no authority decides, and the task reads proposed.
who pat; on 2026-09-05; commit C5 'Accept my own task' --trailer 'Authorised: c3d7'

# C6, C7: the owner hands the root over; the new owner accepts.
root pat
who olive; on 2026-09-06; commit C6 'Hand the project over'
who pat; on 2026-09-07; commit C7 'Accept the second leaf' --trailer 'Authorised: c3d7'

# C8 to C12: a reviewer, the hand-off, the review, the gate, and a second review.
leaf 'Leaf, revised' olive
on 2026-09-08; commit C8 'Name a reviewer at design'
status <<'Y'
gate: defined
state: nominal
reason: review
Y
on 2026-09-09; commit C9 'Hand the design to its reviewer'
who olive; on 2026-09-10; commit C10 'Accept the design' --trailer 'Reviewed: b2c9 design'
status <<'Y'
gate: design
state: nominal
Y
who pat; on 2026-09-11; commit C11 'Record the leaf at design'
who olive; on 2026-09-12; commit C12 'Accept the design again' --trailer 'Reviewed: b2c9 design'

# C13: the reviewer changes after the review; the review stands.
leaf 'Leaf, revised' dan
who pat; on 2026-09-13; commit C13 'Name another reviewer'

# S1, C14, C15: two branches change the status, and the merge writes a third reading.
git -C "$REPO" checkout -q -b side
status <<'Y'
gate: design
state: nominal
note: From the side branch
Y
on 2026-09-14; commit S1 'Note the status on a branch'
git -C "$REPO" checkout -q main
status <<'Y'
gate: design
state: nominal
note: From the trunk
Y
on 2026-09-15; commit C14 'Note the status on the trunk'
on 2026-09-16
git -C "$REPO" merge -q --no-ff --no-commit side >/dev/null 2>&1 || true
status <<'Y'
gate: design
state: nominal
note: From both
Y
git -C "$REPO" add -A; git -C "$REPO" commit -q -m 'Merge the two notes'; label C15

# C16: a reaffirmation from another hand.
who olive; on 2026-09-17; commit C16 'Weekly review: no change' --trailer 'Reaffirmed: b2c9'
