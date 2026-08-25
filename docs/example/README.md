# Worked example: the FADN survey app

A complete, minimal Volt application over two of the real `da_*` survey
tables, showing exactly where hand-written code meets generated code.

> **Status: illustrative.** `volt gen` does not exist yet. Files marked
> *generated* below are hand-written to show what the generator **will
> emit**, per [`router.md`](../router.md) and [`language.md`](../language.md).
> The authored `.volt` files and the "yours" `.go` files are the real
> proposed authoring experience. `Select` blocks are nao v1 syntax
> (features.md), shown speculatively.

```
example/
├── volt.mod                 ─ project root marker
├── main.go                  ← YOURS      wire everything, once
├── db/
│   ├── schema.volt          ← YOURS      tables, group, one Select block
│   └── nao_models.go        ← generated  structs (excerpt)
├── app/
│   ├── routes.volt          ← YOURS      pipelines, Dataset, routes
│   ├── controllers.go       ← YOURS      implements generated interfaces
│   ├── middleware.go        ← YOURS      std-contract middleware
│   ├── volt_handlers.go     ← generated  interfaces + Controllers + New()
│   ├── volt_dataset_da.go   ← generated  registrations + columns + defaults
│   └── volt_paths.go        ← generated  typed reverse URLs
└── client/
    └── grid.go              ← YOURS      guigui side: gob into the same structs
```

Go module (not committed, to keep tooling quiet):
`module example.com/fadn` requiring `github.com/Piechutowski/volt`
(runtime) and importing `example.com/fadn/db`, `example.com/fadn/app`.

**The four seams**, findable in the code:

1. *You implement what gen declares* — `controllers.go` satisfies
   `AppController` (volt_handlers.go) implicitly; no import needed.
2. *Gen calls what the DSL names* — `use app.BearerAuth` (pipeline) and
   `[list: App.DaRRList]` (override) are the only places generated code
   calls yours; both are declared in `routes.volt`.
3. *You call what gen exports* — `db.Queries`, `paths.DaRR()`,
   `next(...)` (the generated default handed to your override).
4. *You wire once* — `main.go`.
