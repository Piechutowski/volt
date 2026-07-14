#!/usr/bin/env bash
# Django documentation — from django/django docs/, converted to markdown.
#
# Django's docs are Sphinx reStructuredText in .txt files (the source of
# https://docs.djangoproject.com). We sparse-clone docs/ at the latest
# stable/X.Y.x branch (override with DJANGO_REF) and convert every file
# with pandoc (rst -> GitHub-flavored markdown). Sphinx-specific roles
# (:setting:, :ref:, ...) degrade to plain text, which is fine for a
# research corpus; files pandoc cannot convert are copied verbatim and
# listed in CONVERSION_FAILURES.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

command -v pandoc >/dev/null || die "pandoc is required (apt-get install pandoc)"

URL=https://github.com/django/django
REF="${DJANGO_REF:-$(latest_branch "$URL" '^stable/[0-9]+\.[0-9]+\.x$')}"
DEST="$CORPUS_DIR/django"
[ -n "$REF" ] || die "could not resolve a Django stable branch"

log "django: $URL @ $REF (docs/)"
clone="$(clone_subdir "$URL" "$REF" docs)"
sha="$(git -C "$clone" rev-parse HEAD)"
src="$clone/docs"

rm -rf "$DEST" && mkdir -p "$DEST"

log "django: converting reStructuredText -> markdown (pandoc)"
fail_list="$DEST/CONVERSION_FAILURES"
export src DEST fail_list
find "$src" -name '*.txt' \
    ! -path "$src/_ext/*" ! -path "$src/_theme/*" ! -path "$src/man/*" \
    -print0 |
  xargs -0 -P "$(nproc)" -I{} bash -c '
    f="{}"
    rel="${f#"$src"/}"
    out="$DEST/${rel%.txt}.md"
    mkdir -p "$(dirname "$out")"
    if ! pandoc --from rst --to gfm --wrap=none "$f" --output "$out" 2>/dev/null; then
      cp "$f" "$out"
      echo "$rel" >> "$fail_list"
    fi
  '

write_manifest "$DEST" "$URL" "$REF" "$sha"
if [ -f "$fail_list" ]; then
  log "django: $(wc -l < "$fail_list" | tr -d ' ') files kept as raw RST (see CONVERSION_FAILURES)"
fi
log "django: $(find "$DEST" -name '*.md' | wc -l | tr -d ' ') markdown files -> $DEST"
