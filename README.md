# Volt

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Volt is my codegen-first web framework for Go: schema and routes are
declared in `.volt` files, `volt gen` writes the router, controller
interfaces and reverse-URL helpers onto plain `net/http.ServeMux`, and
nao — its ORM — generates the models and queries. Everything
resolves at compile time.

## Layout

The pipeline: `.volt` → `lang` (scanner, parser, schema and project
semantics) → `gen/router` → my app implements the generated interfaces
and mounts a plain `http.Handler`. `nao/` is Volt's ORM, a
sub-library of the framework the way ActiveRecord sits inside Rails.

Three Go modules, stitched together by `go.work`: the **library**
(root — runtime, `lang`, `gen`, `nao`; stdlib-only, sqlite3 is
test-only), and the two **tools** (`lsp/`, `cmd/volt/`) that carry the
third-party deps. Importing `nao/rt` alone drags in nothing.

| Directory        | What it is |
|------------------|------------|
| `*.go` (root)    | package `volt` — the runtime generated code links against: error spine, param parsing, path builders, minimal middleware |
| `cmd/volt`       | the one binary, one file: `check` `vet` `gen` `routes` `stress` `migration` `lsp` `version` — `gen` emits models, queries and routers; `stress` writes a one-file thousand-table project to feel the limits by hand. **Own module** — its CLI deps stay out of the library |
| `lang/`          | **the language**: `token` `scanner` `parser` `ast` `diag` `check` `vet` front end, plus go.mod root discovery, package/import resolution, route expansion and conflict detection; `lang/conformance/snippets/` = the executable corpus |
| `gen/router/`    | router generator; goldens are gofmt-stable and compiled by the real toolchain |
| `gen/model/`     | the data half: nao's models, queries and DDL, driven by the same project load |
| `nao/`           | **nao — the ORM**: model + query + SQLite generation (`nao/gen`), its runtime (`nao/rt`), inflector and integration tests |
| `lsp/`           | the Volt language server (`volt lsp`); project-aware diagnostics for files under a go.mod, single-file schema pass otherwise. **Own module** — glsp and its tail stay out of the library |
| `grammar/`       | tree-sitter grammar for the whole language |
| `zed-extension/` | Zed glue; install via `scripts/sync-grammar.sh` + Install Dev Extension |
| `scripts/`       | `sync-grammar.sh` — mirrors the grammar into the local git cache Zed loads dev grammars from; `perf.go` — the performance report of this machine: the profiling sweep from 10 to 320 tables, the editor's keystroke at each size, charts and the machine's specification, as Markdown |
| `itest/`         | committed fixture project served over httptest, drift-checked against the generator |
| `docs/`          | `spec.md` (the language and the generated code, normative) · `lint.md` (vet rules) · `editor.md` (grammar, extension, language server) · `hypotheses.md` (open design questions) · `roadmap.md` (what is built) |
| `AGENT.md`       | cross-session working doctrine for AI sessions (CLAUDE.md imports it) |


## Crib sheet

Requires **Go 1.27+** (all three modules and go.work declare it); with the
default `GOTOOLCHAIN=auto` an older `go` fetches it automatically.

```sh
go install ./cmd/volt                # with cgo (a C compiler on the path): the tool carries its own SQLite
volt check ./app && volt gen ./app && volt routes ./app
volt gen --sql ./app                 # models + queries + router + DDL
go test ./... ./lsp/... ./cmd/volt/...   # everything, ORM + tooling modules
go test ./gen/router -update         # refresh goldens after gen changes
go run ./cmd/volt gen ./itest/blog   # refresh the itest fixture
go test ./lang -run '^$' -bench . -benchmem > lang/testdata/bench_baseline.txt   # refresh the (ungated) baseline
VOLT_SWEEP_DIR=/tmp/sweep go test ./lang -run TestProfileSweep -count=1 -timeout 30m -v   # time and profile every phase at 10..160 tables; charts in /tmp/sweep
volt stress ./big && time volt check ./big && time volt gen --sql ./big && time go build -C ./big ./...   # one file, 1000 tables of 150 columns: the limits of check, gen, the LSP and the Go compiler
./scripts/sync-grammar.sh            # mirror grammar + preflight Zed
```
