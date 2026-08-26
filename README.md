# Volt

Volt is a codegen-first web framework for Go. You describe your
application — database schema and HTTP routes — in one small language,
and `volt gen` writes the code you would otherwise write by hand:
typed controller interfaces, a router on `net/http.ServeMux`, reverse-URL
helpers, models and queries. Rails-grade ergonomics, but everything is
resolved at compile time: no reflection, no runtime DSL, and the
generated code is plain Go you can read.

One language, three layers, one file extension:

```
DBML  ⊂  EDBML  ⊂  Volt          (SPEC.md §V0)
tables    partials,    packages, imports,
enums     records,     Pipelines, Scopes,
refs      checks…      routes, resources
```

Every `.volt` file is written in this language; which layer it uses is
determined by its content. A project is a directory tree rooted at a
`volt.mod` file; a package is a directory (files carry no semantics —
split or merge them freely, §V1.5).

## Architecture

The pipeline, front to back:

```
 .volt files
     │  scanner → parser → AST        nao/edbml/…   (shared front end)
     ▼
 lang/        project semantics: volt.mod, packages, imports,
     │        pipelines, scope/route expansion, conflict detection
     ▼
 gen/router/  volt_handlers.go, volt_router.go, volt_paths.go,
     │        volt_routes.go — generated onto http.ServeMux (Go 1.22+)
     ▼
 your app     implements the generated controller interfaces;
              app.New(Controllers{…}) returns a plain http.Handler
```

| Directory        | What it is |
|------------------|------------|
| `*.go` (root)    | **package `volt`** — the small runtime generated code links against: the error spine (`volt.Request`, `HTTPError`, error handlers), parameter parsing, path builders, middleware. |
| `cmd/volt`       | The one binary: `check`, `vet`, `gen`, `routes`, `lsp`, `version`. |
| `lang/`          | Project loading and the Volt-layer checker (SPEC.md §V), plus the executable conformance corpus (`lang/conformance/`). |
| `gen/router/`    | The router generator, with golden files compiled by the real Go toolchain. |
| `nao/`           | The embedded data layer, [not-an-orm](https://github.com/Piechutowski/not-an-orm): the DBML/EDBML front end (`nao/edbml/…`) every layer shares, model/query/SQLite generation, and the `nao` CLI. |
| `lsp/`           | The Volt language server (`volt lsp`): project-aware diagnostics, completion, hover, navigation, rename. |
| `grammar/`       | The tree-sitter grammar for the whole language — syntax highlighting in any tree-sitter editor. |
| `zed-extension/` | The Zed extension wiring grammar + server together (`scripts/sync-grammar.sh` to install as a dev extension). |
| `itest/`         | The proof suite: a committed fixture project served over `httptest`, drift-checked against the current generator. |
| `docs/`          | The design documents — `router.md`, `language.md`, the editor architecture, and `docs/example/`, a worked end-to-end application. |
| `research/`      | Groundwork: local documentation corpus and feature inventories of Laravel, Rails, Phoenix and Django. |
| `SPEC.md`        | The normative language specification (§V). Every rule is backed by a conformance snippet or test — see its "Conformance and the proof chain" section. |

## Quick start

```sh
go install ./cmd/volt

volt check  ./myapp      # semantic analysis against SPEC.md §V
volt gen    ./myapp      # write volt_*.go into every routing package
volt routes ./myapp      # print the expanded route table
volt lsp                 # language server over stdio
```

A minimal project:

```
myapp/
├── volt.mod             module myapp
├── db/schema.volt       package db  +  Table users { … }
└── app/routes.volt      package app +  import ( db ) + Scope / { … }
```

## License

Apache-2.0 — see [LICENSE](LICENSE).
