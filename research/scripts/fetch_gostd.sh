#!/usr/bin/env bash
# Go standard library documentation — dumped from the installed toolchain.
#
# Go's docs are doc comments in the source, rendered by pkg.go.dev; the
# authoritative offline source is `go doc -all <pkg>`. This dumps the
# packages relevant to web-framework problems into corpus/gostd/,
# mirroring package paths (net/http -> net/http.txt). The corpus is
# versioned by whatever `go` is on PATH — pin by installing the Go
# release you want first.
set -euo pipefail
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

command -v go >/dev/null || die "go toolchain required"

DEST="$CORPUS_DIR/gostd"
GO_VERSION="$(go version | awk '{print $3}')"

# Curated for web-framework relevance; trivially extendable.
PACKAGES=(
  # HTTP & networking
  net/http net/http/httptest net/http/httputil net/http/pprof
  net/http/cookiejar net/http/httptrace net/url net net/netip
  net/mail net/smtp net/textproto
  mime mime/multipart mime/quotedprintable
  # encoding & serialization
  encoding/json encoding/json/v2 encoding/json/jsontext encoding/xml encoding/csv
  encoding/base32 encoding/base64 encoding/gob encoding/hex
  encoding/pem encoding/asn1
  # templating & HTML
  html html/template text/template
  # data layer
  database/sql database/sql/driver
  # core plumbing every framework builds on
  context errors fmt io io/fs bufio bytes strings strconv
  regexp path path/filepath sort slices maps iter cmp unicode unicode/utf8
  # crypto & security
  crypto crypto/aes crypto/cipher crypto/ecdsa crypto/ecdh crypto/ed25519
  crypto/hmac crypto/hkdf crypto/pbkdf2 crypto/sha3 crypto/mlkem
  crypto/md5 crypto/rand crypto/rsa crypto/sha1 crypto/sha256 crypto/sha512
  crypto/subtle crypto/tls crypto/x509
  hash hash/crc32 hash/fnv hash/maphash
  # observability
  log log/slog log/syslog expvar
  runtime runtime/metrics runtime/pprof runtime/trace runtime/debug
  # process, config & runtime
  os os/exec os/signal os/user flag embed time
  math/rand math/rand/v2 math/big
  sync sync/atomic
  # archives & compression
  compress/gzip compress/flate compress/zlib archive/zip archive/tar
  # testing
  testing testing/quick testing/fstest testing/iotest testing/synctest
)

rm -rf "$DEST" && mkdir -p "$DEST"
log "gostd: dumping $(printf '%s\n' "${PACKAGES[@]}" | wc -l | tr -d ' ') packages via 'go doc -all' ($GO_VERSION)"

fail=0
for pkg in "${PACKAGES[@]}"; do
  out="$DEST/$pkg.txt"
  mkdir -p "$(dirname "$out")"
  # Retry behind GOEXPERIMENT for experiment-gated packages (json/v2).
  if ! go doc -all "$pkg" > "$out" 2>/dev/null &&
     ! GOEXPERIMENT=jsonv2 go doc -all "$pkg" > "$out" 2>/dev/null; then
    rm -f "$out"
    log "gostd: WARN no docs for $pkg (not in $GO_VERSION?)"
    fail=$((fail + 1))
  fi
done

{
  echo "source: go doc -all (installed toolchain)"
  echo "ref: $GO_VERSION"
  echo "commit: n/a (toolchain release)"
  echo "fetched: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "files: $(find "$DEST" -name '*.txt' | wc -l | tr -d ' ')"
} > "$DEST/MANIFEST"

log "gostd: $(find "$DEST" -name '*.txt' | wc -l | tr -d ' ') packages -> $DEST ($fail skipped)"
