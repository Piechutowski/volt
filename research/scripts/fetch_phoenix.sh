#!/usr/bin/env bash
# Phoenix documentation — native markdown guides from the framework and
# the ecosystem libraries that make it comparable to batteries-included
# frameworks: Ecto (data layer), LiveView (real-time UI), Plug (HTTP).
#
# Each library's guides/ directory is the source of its hexdocs "pages".
# Latest stable tag per repo unless PHOENIX_REF / LIVEVIEW_REF /
# ECTO_REF / PLUG_REF override. Plug keeps its docs mostly in module
# doc-comments, so for it we take README plus guides if present.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

STABLE_TAG='^v[0-9]+\.[0-9]+\.[0-9]+$'
DEST_ROOT="$CORPUS_DIR/phoenix"
rm -rf "$DEST_ROOT"

fetch_guides() { # name url ref
  local name=$1 url=$2 ref=$3
  local dest="$DEST_ROOT/$name"
  [ -n "$ref" ] || die "could not resolve a release tag for $name"
  log "$name: $url @ $ref"
  local clone; clone="$(clone_repo "$url" "$ref")"
  local sha; sha="$(git -C "$clone" rev-parse HEAD)"
  mkdir -p "$dest"
  if [ -d "$clone/guides" ]; then
    cp -R "$clone/guides/." "$dest/"
  fi
  # Some repos (Plug) document primarily via README + moduledocs.
  [ -f "$clone/README.md" ] && cp "$clone/README.md" "$dest/README.md"
  [ -f "$clone/CHANGELOG.md" ] && cp "$clone/CHANGELOG.md" "$dest/CHANGELOG.md"
  # Markdown and hexdocs cheatsheets only; drop images/config.
  prune_non_docs "$dest" ! -name '*.md' ! -name '*.cheatmd'
  write_manifest "$dest" "$url" "$ref" "$sha"
  log "$name: $(find "$dest" -name '*.md' -o -name '*.cheatmd' | wc -l | tr -d ' ') files -> $dest"
}

PHOENIX_URL=https://github.com/phoenixframework/phoenix
LIVEVIEW_URL=https://github.com/phoenixframework/phoenix_live_view
ECTO_URL=https://github.com/elixir-ecto/ecto
PLUG_URL=https://github.com/elixir-plug/plug

fetch_guides phoenix  "$PHOENIX_URL"  "${PHOENIX_REF:-$(latest_tag "$PHOENIX_URL" "$STABLE_TAG")}"
fetch_guides liveview "$LIVEVIEW_URL" "${LIVEVIEW_REF:-$(latest_tag "$LIVEVIEW_URL" "$STABLE_TAG")}"
fetch_guides ecto     "$ECTO_URL"     "${ECTO_REF:-$(latest_tag "$ECTO_URL" "$STABLE_TAG")}"
fetch_guides plug     "$PLUG_URL"     "${PLUG_REF:-$(latest_tag "$PLUG_URL" "$STABLE_TAG")}"
