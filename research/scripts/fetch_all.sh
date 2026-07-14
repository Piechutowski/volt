#!/usr/bin/env bash
# Fetch and convert the documentation of all four reference frameworks.
# See the individual fetch_*.sh scripts for version pinning via env vars.
set -euo pipefail
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

"$here/fetch_laravel.sh"
"$here/fetch_rails.sh"
"$here/fetch_phoenix.sh"
"$here/fetch_django.sh"

echo
echo "corpus summary:"
find "$here/../corpus" -name MANIFEST -print0 | sort -z | xargs -0 grep -H . | sed 's|.*/corpus/||'
