#!/usr/bin/env bash
# Laravel documentation — native markdown from github.com/laravel/docs.
#
# The laravel/docs repo IS the source of https://laravel.com/docs: one
# flat directory of markdown files, one branch per release line. We take
# the highest "N.x" branch unless LARAVEL_REF overrides it
# (e.g. LARAVEL_REF=12.x).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

URL=https://github.com/laravel/docs
REF="${LARAVEL_REF:-$(latest_branch "$URL" '^[0-9]+\.x$')}"
DEST="$CORPUS_DIR/laravel"
[ -n "$REF" ] || die "could not resolve a Laravel docs branch"

log "laravel: $URL @ $REF"
clone="$(clone_repo "$URL" "$REF")"
sha="$(git -C "$clone" rev-parse HEAD)"

rm -rf "$DEST" && mkdir -p "$DEST"
cp "$clone"/*.md "$DEST/"
write_manifest "$DEST" "$URL" "$REF" "$sha"
log "laravel: $(find "$DEST" -name '*.md' | wc -l | tr -d ' ') markdown files -> $DEST"
