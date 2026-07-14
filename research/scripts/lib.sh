#!/usr/bin/env bash
# Shared helpers for the docs-fetching scripts.
#
# Every fetch script sources this file. It provides version resolution
# against the upstream git remotes and a sparse/shallow clone helper so
# we never download more than the docs directory of a large repo.
set -euo pipefail

SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPTS_DIR")"
CORPUS_DIR="$ROOT_DIR/corpus"

WORK_DIR="$(mktemp -d "${TMPDIR:-/tmp}/docs-fetch.XXXXXX")"
trap 'rm -rf "$WORK_DIR"' EXIT

log() { printf '>> %s\n' "$*" >&2; }
die() { printf 'error: %s\n' "$*" >&2; exit 1; }

# Highest release tag at $1 whose name matches extended regex $2
# (version-sorted; pass a regex that excludes rc/beta/alpha tags).
latest_tag() {
  git ls-remote --tags --refs "$1" \
    | awk -F'refs/tags/' '{print $2}' \
    | grep -E "$2" | sort -V | tail -1
}

# Highest branch at $1 whose name matches extended regex $2.
latest_branch() {
  git ls-remote --heads "$1" \
    | awk -F'refs/heads/' '{print $2}' \
    | grep -E "$2" | sort -V | tail -1
}

# Shallow-clone $1 at ref $2 into a temp dir; prints the clone path.
clone_repo() {
  local url=$1 ref=$2
  local dest="$WORK_DIR/$(basename "$url" .git)-$$-$RANDOM"
  git clone --quiet --depth 1 --branch "$ref" --single-branch "$url" "$dest"
  printf '%s' "$dest"
}

# Sparse, shallow, blob-less clone of subdir $3 from $1 at ref $2;
# prints the clone path (subdir is at <path>/$3).
clone_subdir() {
  local url=$1 ref=$2 subdir=$3
  local dest="$WORK_DIR/$(basename "$url" .git)-$$-$RANDOM"
  git clone --quiet --depth 1 --branch "$ref" --single-branch \
    --filter=blob:none --sparse "$url" "$dest"
  git -C "$dest" sparse-checkout set --no-cone "/$subdir/*" >/dev/null
  printf '%s' "$dest"
}

# write_manifest DEST URL REF SHA — provenance record next to the corpus.
write_manifest() {
  local dest=$1 url=$2 ref=$3 sha=$4
  {
    echo "source: $url"
    echo "ref: $ref"
    echo "commit: $sha"
    echo "fetched: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo "markdown_files: $(find "$dest" -name '*.md' -o -name '*.cheatmd' | wc -l | tr -d ' ')"
  } > "$dest/MANIFEST"
}

# Remove everything that is not documentation text (images, build
# scaffolding), then delete directories left empty.
prune_non_docs() {
  local dest=$1; shift
  find "$dest" -type f ! -name MANIFEST "$@" -delete
  find "$dest" -type d -empty -delete 2>/dev/null || true
}
