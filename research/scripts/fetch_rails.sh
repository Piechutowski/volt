#!/usr/bin/env bash
# Rails guides — native markdown from rails/rails, guides/source/.
#
# guides/source is the source of https://guides.rubyonrails.org. We pin
# the latest stable release tag (vX.Y.Z, no rc/beta) unless RAILS_REF
# overrides it (e.g. RAILS_REF=main for edge guides).
#
# Note: this is the guides, not the API reference (api.rubyonrails.org
# is generated from source comments — see crawl_html.py for that).
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

URL=https://github.com/rails/rails
REF="${RAILS_REF:-$(latest_tag "$URL" '^v[0-9]+\.[0-9]+\.[0-9]+$')}"
DEST="$CORPUS_DIR/rails"
[ -n "$REF" ] || die "could not resolve a Rails release tag"

log "rails: $URL @ $REF (guides/source)"
clone="$(clone_subdir "$URL" "$REF" guides/source)"
sha="$(git -C "$clone" rev-parse HEAD)"

rm -rf "$DEST" && mkdir -p "$DEST"
cp -R "$clone/guides/source/." "$DEST/"
# Keep the guides (*.md) and their index (documents.yaml); drop ERB
# layout templates, epub scaffolding, and images.
prune_non_docs "$DEST" ! -name '*.md' ! -name 'documents.yaml'
write_manifest "$DEST" "$URL" "$REF" "$sha"
log "rails: $(find "$DEST" -name '*.md' | wc -l | tr -d ' ') markdown files -> $DEST"
