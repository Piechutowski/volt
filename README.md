# Volt

**Status: alpha.** Personal project. The language, the generated code
and every API in here can change without ceremony. Nothing is promised
to anyone; SPEC.md describes what the implementation does today, not a
stability contract.

Volt is my codegen-first web framework for Go: schema and routes are
declared in `.volt` files, `volt gen` writes the router, controller
interfaces and reverse-URL helpers onto plain `net/http.ServeMux`, and
nao — its ORM — generates the models and queries. Everything
resolves at compile time.

**One language, one file extension.** A `.volt` file is Volt — the
markup began as DBML and grew outward, but it is a single language, not
three that share a suffix. Which constructs a file happens to use is up
to the file. Its layers, with their honest build state:

- **Schema** (the DBML-derived core) — *built.* `lang/SPEC.md`:
  Project, Table with columns/indexes/checks, TablePartial, Records,
  Enum, Ref, TableGroup, Notes, DiagramView.
- **Schema extensions** — *designed, not built* (`nao/docs/decisions.md`
  D05–D07): declared queries (`Select` / `View` / `Trigger`), `[was:]`,
  `[repr:]`. Only the `[model:]` naming override has landed.
- **Routing** — *alpha:* `package` / `import`, `Pipeline`, `Scope`,
  routes, `resources` (SPEC.md §V).

Project = tree rooted at `volt.mod`. Package = directory. File
boundaries carry no meaning (§V1.5).

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
| `cmd/volt`       | the one binary: `check` `vet` `gen` `routes` `lsp` `version` — `gen` emits models, queries and routers. **Own module** — its CLI deps stay out of the library |
| `lang/`          | **the language**: `token` `scanner` `parser` `ast` `diag` `check` `vet` front end, plus volt.mod/package/import resolution, route expansion and conflict detection; `lang/SPEC.md` = schema-layer spec, `lang/conformance/{volt,dbml}` = executable corpora |
| `gen/router/`    | router generator; goldens are gofmt-stable and compiled by the real toolchain |
| `gen/model/`     | the data half: nao's models, queries and DDL, driven by the same project load |
| `nao/`           | **nao — the ORM**: model + query + SQLite generation (`nao/gen`), its runtime (`nao/rt`), inflector and docs |
| `lsp/`           | the Volt language server (`volt lsp`); project-aware diagnostics for files under a volt.mod, single-file DBML pass otherwise. **Own module** — glsp and its tail stay out of the library |
| `grammar/`       | tree-sitter grammar for the whole language |
| `zed-extension/` | Zed glue; install via `scripts/sync-grammar.sh` + Install Dev Extension |
| `itest/`         | committed fixture project served over httptest, drift-checked against the generator |
| `examples/crud/` | **runnable**: two `resources` lines → 14 routes; `go run ./examples/crud`, `go test ./examples/crud` |
| `docs/`          | design docs (`router.md`, `language.md`, editor architecture) |
| `research/`      | Laravel/Rails/Phoenix/Django docs corpus + feature inventories |
| `SPEC.md`        | the §V language spec; every rule backed by a corpus snippet or test |

## What is real vs. not yet

Implemented and tested: the three-layer front end, project checking,
router generation (typed params, wildcards, resources, pipelines,
per-scope error handlers, reverse URLs), the runtime, the CLI, the
LSP, the grammar, the Zed extension, and the whole proof chain.

Designed but **not implemented** — `docs/router.md` describes these;
no code backs them yet:

- **the schema extensions** — the query layer (`Select`/`View`/
  `Trigger`), `[was:]`, `[repr:]`; only `[model:]` exists today
- **Datasets** (`Dataset` is just reserved, §V8) — the auto-CRUD grid
  over a TableGroup (`docs/router.md` §12)
- the renderer / content negotiation (HTML, JSON, GOB) from
  `docs/router.md` §12; runtime has only `volt.JSON`
- `volt.WithQueries` / `WithTemplates` / `BuildDeps` options
- plugs from imported packages (a v0 error on purpose)
- LSP navigation beyond Volt symbols: go-to-def, hover and rename work
  across the project for tables, pipelines and imports, but handler
  refs (`Home.Index`) and `error_handler:` point at Go, which this
  server does not index

## Crib sheet

Requires **Go 1.27+** (all four modules declare `go 1.27`); with the
default `GOTOOLCHAIN=auto` an older `go` fetches it automatically.

```sh
go install ./cmd/volt
volt check ./app && volt gen ./app && volt routes ./app
volt gen --sql ./app                 # models + queries + router + DDL
go run ./examples/crud                # the runnable CRUD example
go test ./... ./lsp/... ./cmd/volt/...   # everything, ORM + tooling modules
go test ./gen/router -update         # refresh goldens after gen changes
go run ./cmd/volt gen ./itest/blog   # refresh the itest fixture
./scripts/sync-grammar.sh            # mirror grammar + preflight Zed
```

Apache-2.0.
