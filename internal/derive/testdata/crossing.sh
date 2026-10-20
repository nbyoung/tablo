#!/bin/sh
# crossing.sh <directory>: builds the repository of design cdb0's crossing,
# one history whose version.yaml rises from 0.3.2 to 0.4.0, and writes
# <directory>.labels.txt. It fixes identities, dates and messages, as design
# 27a3's cases.sh does, so the hashes repeat. crossing.expected.txt states
# what a tool derives and reports at each label.
set -eu
REPO=$1
rm -rf "$REPO"
git init -q -b main "$REPO"
git -C "$REPO" config commit.gpgsign false
: > "$REPO.labels.txt"

who() { export GIT_AUTHOR_NAME="$1" GIT_AUTHOR_EMAIL="$1@example.org" GIT_COMMITTER_NAME="$1" GIT_COMMITTER_EMAIL="$1@example.org"; }
on() { export GIT_AUTHOR_DATE="${1}T12:00:00+00:00" GIT_COMMITTER_DATE="${1}T12:00:00+00:00"; }
put() { mkdir -p "$REPO/$(dirname "$1")"; cat > "$REPO/$1"; }
label() { printf '%s %s\n' "$1" "$(git -C "$REPO" rev-parse HEAD)" >> "$REPO.labels.txt"; }
commit() { _l=$1; _s=$2; shift 2; git -C "$REPO" add -A; git -C "$REPO" commit -q --allow-empty -m "$_s" "$@"; label "$_l"; }
version() { printf 'tableaux: %s\ntrunk: main\n' "$1" | put .tableaux/version.yaml; }
status() { printf 'gate: %s\nstate: %s\n' "$2" "$3" | put ".tableaux/status/$1.yaml"; }
agent='Co-Authored-By: Corpus Agent <bot@example.org>'

version 0.3.2
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
put .tableaux/tasks/e4a1.yaml <<'Y'
title: Crossing
description: The root; olive is a person.
assignee: olive@example.org
Y
put .tableaux/tasks/c5f0.yaml <<'Y'
title: Agent subtree
description: A parent an agent is assigned.
assignee: bot@example.org
parent: { id: "e4a1", order: 1 }
Y
put .tableaux/tasks/b2c9.yaml <<'Y'
title: Agent leaf
description: An agent's leaf under an agent's parent.
assignee: bot@example.org
junctions:
  design: { contributor: bot@example.org, model: claude-fable }
parent: { id: "c5f0", order: 1 }
Y
put .tableaux/tasks/d3a6.yaml <<'Y'
title: Person leaf
description: A person's leaf whose design an agent does.
assignee: pat@example.org
junctions:
  design: { contributor: bot@example.org, model: claude-fable }
parent: { id: "c5f0", order: 2 }
Y
status b2c9 defined nominal
status d3a6 defined nominal
who olive; on 2026-09-01; commit X1 'Plan the crossing'

# X2: at 0.3.2 bot is an agent at design by its model, and the assignee, bot, reviews it.
status b2c9 design nominal
who bot; on 2026-09-02; commit X2 'Record the agent leaf at design' --trailer 'Reviewed: b2c9 design' --trailer 'Model: claude-fable-5-1'

# X3: at 0.3.2 pat records bot's work under pat's identity: exempt from H7; pat, the assignee, reviews.
status d3a6 design nominal
who pat; on 2026-09-03; commit X3 'Record the person leaf at design' --trailer 'Reviewed: d3a6 design' --trailer 'Model: claude-fable-5-1' --trailer "$agent"

# X4: the language rises.
version 0.4.0
who olive; on 2026-09-04; commit X4 'Raise the language to 0.4.0'

# X5: at 0.4.0 bot, paired with a model, is an agent at every junction; pat records bot's
# release work under pat's identity: H7. The nearest person, olive, has not reviewed it.
status b2c9 release complete
who pat; on 2026-09-05; commit X5 'Record the agent leaf at release' --trailer 'Model: claude-fable-5-1' --trailer "$agent"

# X6: olive, the nearest person, accepts the release; X2's review of design stays accepted.
who olive; on 2026-09-06; commit X6 'Accept the agent release' --trailer 'Reviewed: b2c9 release'
