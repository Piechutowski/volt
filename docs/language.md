# The Volt Language — Modules, Packages & Dialects (draft v0.1)

**Status:** Draft for discussion — companion to [`router.md`](./router.md)
(R-series decisions there; L-series here). This document owns everything
about the language that is *not* a specific element kind: the dialect
layering, the file model, and the package system.

**TLDR.** Volt is **one language with three layers** —
core DBML ⊂ EDBML (nao's extensions) ⊂ Volt (routing/dataset elements) —
written in `.volt` files whose dialect is determined by **content, not
extension**. The unit of compilation is the **package (a directory)**,
Go-style: files are arbitrary slices of their package, and the tool has
no opinion about which declaration lives in which file. Cross-package
references use a Go-style `package` / `import` system with qualified
access. DBML's file-based `use`/`reuse` import syntax is **removed** —
a deliberate compatibility break, documented in
[`dbml-imports.md`](./dbml-imports.md).

---

## L1 — The package is the unit; files are invisible

A **package** is a directory of `.volt` files. All top-level
declarations in a package share one namespace (per element kind, as in
DBML §8.2 — tables, enums, partials, pipelines, scopes, datasets each
their own). File boundaries and declaration order carry **no
semantics** — DBML already promises order-independence (SPEC §5); Volt
extends the same promise to file assignment.

Consequences, by construction:

- The solo builder puts an entire application in one `app.volt` —
  models on top, routes below.
- A team gives every member their own file, or one file per model with
  its routes beside it, or `schema.volt` + `routes.volt` — all
  equivalent to the compiler.
- Reorganizing files is never a semantic change. `volt gen` output is
  identical for every layout of the same declarations.

Discipline is a lint, not a law: teams that want "schema files stay
pure" enforce it with an opt-in `volt vet` rule, never with the
grammar.

## L2 — One language, three dialect layers

```
core DBML  ⊂  EDBML (nao extensions)  ⊂  Volt (routing & dataset elements)
```

- Every file is named `.volt`. What a file *is* — plain DBML, EDBML, or
  full Volt — follows from what it contains.
- `volt vet` can classify each file's effective dialect; a purity lint
  is available for teams that want it.
- **Diagramming** is content-based, not extension-based: a file whose
  content is pure core DBML pastes into dbdiagram.io as-is, and
  `volt export dbml <package>` derives a pure-DBML view of any package
  for diagramming, whatever its layout. The ER diagram is a derived
  artifact of the schema elements, not a property of file bytes.
- The core-DBML layer remains conformance-guarded by the corpus
  (`nao/edbml/conformance/`) — with the one deliberate exception below
  (L4).

## L3 — Go-style packages and imports

Every `.volt` file begins with a `package` clause; all files in a
directory MUST declare the same package (conventionally the directory
name). Cross-package references go through an `import` block:

```volt
// db/schema.volt
package db

Table users {
  id    integer [pk, increment]
  email text    [not null, unique]
}

// app/routes.volt
package app

import (
	db
	shared/dicts
	d2 shared/dicts2      // optional alias, Go-style
)

Pipeline api { use volt.RequestID }

Scope /api [pipe: api] {
  resources users [model: db.User, bind]
}
Ref: app_audit.user_id > db.users.id
```

Rules (all Go's, deliberately):

1. **Paths are project-root-relative.** The project root is marked by a
   `volt.mod` file (one line: the module name). No `./`, no `..` —
   inside the project you name packages by their path from the root,
   exactly as Go names packages within a module.
2. **One import form.** A newline-separated block; each entry is a
   path, optionally preceded by an alias. The default qualifier is the
   last path element.
3. **Qualified access only.** Imported declarations are reached through
   the package qualifier (`db.users`, `db.group(DA)`). Nothing is ever
   dumped into the importing scope.
4. **Unused import = check error.** As in Go: the import list is a
   truthful dependency manifest or it is an error.
5. **Import cycles = check error.** Go's no-cycles rule, adopted:
   packages must layer. (DBML's declarative circular file imports die
   with the file-import system itself; intra-package references need no
   imports at all.)

**Qualifier resolution.** DBML already uses `.` for `schema.table`.
Resolution rule: the first component of a qualified name resolves as a
package qualifier first, then as a local schema; a package
alias/name colliding with a local schema name is a check error (rename
one — the alias makes this always possible). A package's internal DBML
schemas remain meaningful within it; cross-package references address
the package, and the package's exported names, only.

## L4 — The compatibility break: DBML `use`/`reuse` is removed

DBML/EDBML's file-based module system — `use * from './file'`,
selective imports with per-element kinds, per-element `as` aliases, and
`reuse` re-export — is **not part of the Volt language, at any dialect
layer**. Four mechanisms existed to manage the name conflicts created
by dumping declarations into file scope; qualified package access makes
those conflicts impossible, so the mechanisms and their complexity go
too. `reuse` re-export is likewise dropped: consumers name their real
dependencies (neither Go nor Odin re-exports, and both ecosystems layer
better for it).

- This is a deliberate break with upstream DBML, made **before v1**,
  while the cost is lowest. The old system is documented for reference
  in [`dbml-imports.md`](./dbml-imports.md).
- The front end retains the *ability to parse* §7 import statements so
  the conformance corpus can keep cross-checking against upstream
  `@dbml/parse`; the Volt checker rejects them with a targeted
  diagnostic pointing at the migration doc.
- Migration is mechanical: files that imported each other become one
  package (delete the imports), or separate directories with a
  `package` clause and a root-relative `import` (add qualifiers).

## L5 — Migration identity is semantic, never file bytes

The (future) migration engine pins the schema by a **canonical hash of
the package's schema elements** — tables, enums, refs, checks, indexes
— computed from the checked AST, not from file contents. Reformatting,
comments, reordering declarations, and re-slicing files never perturb
the migration ledger; only semantic schema changes do. (This is forced
by L1 — files carry no meaning — and is better regardless: a comment
edit was never a migration.)

## L6 — One repository, one toolchain

not-an-orm's source now lives in this repository under
[`nao/`](../nao/) (module `github.com/Piechutowski/volt/nao`; full test
suite — conformance corpus, generator goldens, SQLite round-trips —
passing post-move). The strategic consequence, stated plainly: nao is
Volt's data layer; "Not an ORM" the product becomes `volt gen` the
capability. A language change touches scanner, parser, checker, vet,
generators, LSP, grammar, and corpus *together* — one repository makes
that one atomic commit with one CI run, which is why this is a copy-in,
not a submodule.

Near-term layout roadmap (deliberately incremental):

1. **Now:** `nao/` as copied, own Go module, everything green.
2. **When Volt elements land in the grammar:** the front end
   (`nao/edbml/{token,scanner,ast,parser,check,vet}`) migrates to a
   shared `lang/` tree that both generators consume; `cmd/volt` grows
   `check | vet | gen | routes | export dbml | lsp`; `cmd/nao` remains
   as a compatibility shim during the alpha, then deprecates.
3. nao's D-series decisions remain law for the data layer; the L- and
   R-series govern the language and router. Register unification is a
   v1-era cleanup.

## Open questions

1. **`volt.mod` contents** — just the module name, or also toolchain
   version pinning (Go's `go 1.22` line has proven its worth)?
2. **Exported vs package-private declarations** — Go exports by
   capitalization; DBML names are user-domain (table names must match
   the database). Does Volt need visibility control at all in v1, or is
   everything exported until proven painful?
3. **Package name vs directory name** — Go allows them to differ and
   regrets it. Enforce equality?
4. **The `Project` element** — with packages and `volt.mod`, does
   `Project` survive (as per-package metadata), move into `volt.mod`,
   or die?
