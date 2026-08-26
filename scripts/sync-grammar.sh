#!/usr/bin/env bash
# Publish the grammar/ tree-sitter grammar to a local git mirror and point the
# Zed extension at it.
#
# Zed loads extension grammars from a git repository (repository + commit in
# extension.toml) and supports file:// URLs for local development. This
# script keeps that contract without polluting the main repo with a nested
# git checkout:
#
#   1. (optional) regenerate src/parser.c if a tree-sitter CLI is available
#   2. mirror grammar/ into ~/.cache/volt/tree-sitter-volt-git
#      and commit the current state there
#   3. rewrite [grammars.volt] in zed-extension/extension.toml with the
#      mirror's file:// URL and fresh commit hash
#   4. preflight exactly what Zed's grammar build will do (clear a stale
#      checkout, fetch the pinned commit, verify the exported symbol),
#      so failures surface here with real messages instead of Zed's
#      generic "failed to compile grammar 'volt'"
#
# Run it after cloning and after every grammar change, then use
# `zed: install dev extension` (first time) or Rebuild (afterwards).
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
grammar_src="$repo_root/grammar"
extension_toml="$repo_root/zed-extension/extension.toml"
mirror="${VOLT_GRAMMAR_MIRROR:-${XDG_CACHE_HOME:-$HOME/.cache}/volt/tree-sitter-volt-git}"

# 1. Regenerate the parser when a CLI is around (committed src/ still works
#    when it is not).
if command -v tree-sitter >/dev/null 2>&1; then
    echo "Regenerating parser with $(tree-sitter --version)..."
    (cd "$grammar_src" && tree-sitter generate)
else
    echo "tree-sitter CLI not found; using committed src/parser.c as-is."
fi

# 2. Mirror into a standalone git repo.
mkdir -p "$mirror"
find "$mirror" -mindepth 1 -maxdepth 1 ! -name .git -exec rm -rf {} +
(cd "$grammar_src" && tar cf - --exclude .git --exclude node_modules --exclude build .) |
    (cd "$mirror" && tar xf -)
cd "$mirror"
if [ ! -d .git ]; then
    git init -q
fi
git add -A
if ! git diff --cached --quiet; then
    git -c user.name=volt-sync -c user.email=sync@localhost \
        commit -q -m "sync $(date -u +%Y-%m-%dT%H:%M:%SZ)"
fi
sha="$(git rev-parse HEAD)"

# 3. Point the extension at the mirror. A silent non-match here would
#    leave Zed cloning a placeholder URL, so the rewrite must count.
python3 - "$extension_toml" "file://$mirror" "$sha" <<'PY'
import re, sys
path, url, sha = sys.argv[1:]
text = open(path).read()
text, n = re.subn(r'(?ms)(\[grammars\.volt\]\n)repository = "[^"]*"\ncommit = "[^"]*"',
                  lambda m: f'{m.group(1)}repository = "{url}"\ncommit = "{sha}"', text)
if n != 1:
    sys.exit(f"error: could not rewrite [grammars.volt] in {path} (matched {n} blocks)")
open(path, "w").write(text)
PY

# 4. Preflight: replay Zed's grammar build steps.
#    Zed checks the grammar out into zed-extension/grammars/volt and, if
#    that directory already exists, insists its git remote match the
#    repository URL — a leftover checkout from an earlier mirror path
#    (or a half-failed install) fails the whole build
#    (zed-industries/zed#10569). It is a build artifact, safe to clear.
ext_checkout="$repo_root/zed-extension/grammars/volt"
if [ -d "$ext_checkout" ]; then
    current="$(git --git-dir "$ext_checkout/.git" remote get-url origin 2>/dev/null || echo 'not a git repo')"
    if [ "$current" != "file://$mirror" ]; then
        echo "Clearing stale grammar checkout (pointed at: $current)."
        rm -rf "$repo_root/zed-extension/grammars"
    fi
fi

#    Then prove the exact fetch/checkout Zed performs works against the
#    mirror, and that the parser exports the symbol Zed will demand.
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
git -C "$tmp" init -q
git -C "$tmp" remote add origin "file://$mirror"
if ! git -C "$tmp" fetch -q --depth 1 origin "$sha"; then
    echo "error: Zed-style 'git fetch --depth 1 origin $sha' failed against the mirror." >&2
    echo "       If a git config rewrites URLs (url.*.insteadOf), exempt file:// URLs." >&2
    exit 1
fi
git -C "$tmp" checkout -q "$sha"
[ -f "$tmp/src/parser.c" ] ||
    { echo "error: mirror lacks src/parser.c" >&2; exit 1; }
grep -q 'tree_sitter_volt' "$tmp/src/parser.c" ||
    { echo "error: src/parser.c does not export tree_sitter_volt — regenerate it:" >&2
      echo "       cd grammar && npm install && ./node_modules/.bin/tree-sitter generate" >&2
      exit 1; }

echo "Grammar mirrored at: $mirror"
echo "extension.toml now pins commit ${sha:0:12}"
echo "Preflight OK: Zed-style checkout works; parser.c exports tree_sitter_volt."
echo
echo "Next: in Zed run 'zed: extensions' and click Install Dev Extension"
echo "(first time, pointing at zed-extension/ — remove any old EDBML dev"
echo "extension entry first) or Rebuild on the Volt extension."
echo
echo "If Zed still reports \"failed to compile grammar 'volt'\", run"
echo "'zed: open log' — the usual causes left are Zed failing to download"
echo "its wasi-sdk toolchain (network/proxy; set WASI_SDK_PATH to a local"
echo "install to skip the download) or a git URL rewrite in ~/.gitconfig."
echo "Note: extension.toml's grammar lines are machine-local; avoid"
echo "committing that change."
