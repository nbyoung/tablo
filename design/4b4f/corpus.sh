#!/bin/sh
# corpus.sh: the copy of the tableaux conformance corpus that tablo tests against.
#
#   sh internal/corpustest/corpus.sh refresh <tableaux checkout> [<commit>]
#   sh internal/corpustest/corpus.sh fetch <directory>
#   sh internal/corpustest/corpus.sh digest [<corpus directory>]
#
# refresh  replaces testdata/corpus with corpus/ of the tableaux repository at
#          <commit> (HEAD by default) and rewrites corpus.lock.yaml. It reads a
#          local checkout and never the network. A person runs it, reads the
#          diff and commits both paths in one commit.
# fetch    clones the repository the lock names into <directory>, checks out the
#          locked commit and the submodules, points the clone's own tag
#          corpus/tooling at the locked commit of that tag, and fails unless the
#          clone's corpus/ equals testdata/corpus. CI runs it; it needs the network.
# digest   prints the digest of a corpus directory, testdata/corpus by default.
#
# The digest is the SHA-256 of the lines "<sha256 of the file>  ./<path>", one
# per regular file outside build/, sorted bytewise by path. The Go test
# TestVendoredCopy computes the same value.

set -eu
HERE=$(cd "$(dirname "$0")" && pwd)
LOCK="$HERE/corpus.lock.yaml"
COPY="$HERE/testdata/corpus"

# field <key>: the value of a top-level scalar of the lock.
field() { sed -n "s/^$1: *\([^ #]*\).*/\1/p" "$LOCK"; }

# sum: SHA-256 of standard input or of the files named, in sha256sum's format.
sum() {
  if command -v sha256sum > /dev/null 2>&1; then sha256sum "$@"; else shasum -a 256 "$@"; fi
}

digest() {
  ( cd "${1:-$COPY}" &&
    find . -type f ! -path './build/*' -print | LC_ALL=C sort |
    while IFS= read -r f; do sum "$f"; done | sum | cut -d' ' -f1 )
}

refresh() {
  src=$1
  commit=$(git -C "$src" rev-parse --verify "${2:-HEAD}^{commit}")
  tooling=$(git -C "$src" rev-parse --verify 'refs/tags/corpus/tooling^{commit}')
  repository=$(field repository)
  rm -rf "$COPY"
  mkdir -p "$COPY"
  git -C "$src" archive "$commit" corpus | tar -x -C "$COPY" --strip-components=1
  cat > "$LOCK" <<EOF
# corpus.lock.yaml: the tableaux commit that testdata/corpus copies.
# corpus.sh refresh writes this file; do not edit it by hand.
repository: $repository
commit: $commit
tooling: $tooling
digest: $(digest "$COPY")
EOF
  echo "testdata/corpus now copies $repository at $commit"
}

fetch() {
  dir=$1
  git clone -q --no-checkout "$(field repository)" "$dir"
  git -C "$dir" checkout -q --detach "$(field commit)"
  git -C "$dir" submodule -q update --init
  git -C "$dir" tag -f corpus/tooling "$(field tooling)" > /dev/null
  if ! diff -r -x build "$dir/corpus" "$COPY"; then
    echo "corpus.sh: testdata/corpus differs from $(field repository) at $(field commit)" >&2
    exit 1
  fi
  if [ "$(digest "$COPY")" != "$(field digest)" ]; then
    echo "corpus.sh: testdata/corpus does not match the digest in corpus.lock.yaml" >&2
    exit 1
  fi
}

case "${1:-}" in
  refresh) [ $# -ge 2 ] || { echo "usage: corpus.sh refresh <tableaux checkout> [<commit>]" >&2; exit 2; }
           shift; refresh "$@" ;;
  fetch)   [ $# -eq 2 ] || { echo "usage: corpus.sh fetch <directory>" >&2; exit 2; }
           fetch "$2" ;;
  digest)  digest "${2:-}" ;;
  *)       echo "usage: corpus.sh refresh|fetch|digest" >&2; exit 2 ;;
esac
