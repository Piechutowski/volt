# Volt

**Status: alpha.** Personal project. The language, the generated code
and every API in here can change without ceremony. Nothing is promised
to anyone; SPEC.md describes what the implementation does today, not a
stability contract.

Volt is my codegen-first web framework for Go: schema and routes are
declared in `.volt` files, `volt gen` writes the router, controller
interfaces and reverse-URL helpers onto plain `net/http.ServeMux`, and
nao (embedded here) generates the models and queries. Everything
resolves at compile time.

One language, three layers, one extension. A `.volt` file is DBML, or
EDBML, or full Volt — decided by its content, not its name (SPEC.md
§V0):

```
DBML   — tables, enums, refs, notes
EDBML  — + partials, records, checks, diagram views
Volt   — + package / import, Pipeline, Scope, routes, resources
```

Project = tree rooted at `volt.mod`. Package = directory. File
boundaries carry no meaning (§V1.5).

## Layout

The pipeline: `.volt` → `nao/edbml` front end (shared scanner/parser)
→ `lang` (project semantics) → `gen/router` → my app implements the
generated interfaces and mounts a plain `http.Handler`.

| Directory        | What it is |
|------------------|------------|
| `*.go` (root)    | package `volt` — the runtime generated code links against: error spine, param parsing, path builders, minimal middleware |
| `cmd/volt`       | the binary: `check` `vet` `gen` `routes` `lsp` `version` |
| `lang/`          | volt.mod/package/import resolution, route expansion, conflict detection; `lang/conformance/` = executable corpus of SPEC §V |
| `gen/router/`    | router generator; goldens are gofmt-stable and compiled by the real toolchain |
| `nao/`           | not-an-orm, merged in as the data layer; owns the shared front end (`nao/edbml/…`) and model/SQLite generation |
| `lsp/`           | the Volt language server (`volt lsp`); project-aware diagnostics for files under a volt.mod, single-file DBML pass otherwise |
| `grammar/`       | tree-sitter grammar for the whole language |
| `zed-extension/` | Zed glue; install via `scripts/sync-grammar.sh` + Install Dev Extension |
| `itest/`         | committed fixture project served over httptest, drift-checked against the generator |
| `docs/`          | design docs (`router.md`, `language.md`, editor architecture) + `docs/example/` — the FADN case study; **partly illustrative, see below** |
| `research/`      | Laravel/Rails/Phoenix/Django docs corpus + feature inventories |
| `SPEC.md`        | the §V language spec; every rule backed by a corpus snippet or test |

## What is real vs. not yet

Implemented and tested: the three-layer front end, project checking,
router generation (typed params, wildcards, resources, pipelines,
per-scope error handlers, reverse URLs), the runtime, the CLI, the
LSP, the grammar, the Zed extension, and the whole proof chain.

Designed but **not implemented** — `docs/example/` shows these as
hand-written illustration only:

- **Datasets** (`Dataset` is just reserved, §V8) — the auto-CRUD grid
  the FADN case wants
- the renderer / content negotiation (HTML, JSON, GOB) from
  `docs/router.md` §12; runtime has only `volt.JSON`
- `volt.WithQueries` / `WithTemplates` / `BuildDeps` options
- plugs from imported packages (a v0 error on purpose)
- LSP navigation for Volt-layer symbols (diagnostics yes, go-to-def
  still DBML-only)

## Crib sheet

```sh
go install ./cmd/volt
volt check ./app && volt gen ./app && volt routes ./app
go test ./...                        # everything, incl. nao's suites
go test ./gen/router -update         # refresh goldens after gen changes
go run ./cmd/volt gen ./itest/blog   # refresh the itest fixture
./scripts/sync-grammar.sh            # mirror grammar + preflight Zed
```

Apache-2.0.
