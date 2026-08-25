# Legacy reference: the DBML file-import system (`use` / `reuse`)

> **Status: not part of the Volt language.** Volt replaces file-based
> imports with a Go-style package system — see
> [`language.md`](./language.md) §L3–L4 for the system and the
> rationale for the break. This document preserves the old design for
> reference, for migration, and because the conformance corpus still
> cross-checks the *parser* against upstream `@dbml/parse`, which
> implements it. The Volt **checker** rejects these statements with a
> diagnostic pointing here.

DBML's module system split a schema across files by importing
*elements* from other *files*. It had four mechanisms:

## 1. Import all

```dbml
use * from './base'        // path relative to the current file;
                           // .dbml extension optional
```

Everything the target file exports is dumped into the current file's
scope.

## 2. Selective import

```dbml
use {
  table auth.users as u
  table auth.roles as r
  schema billing
  tablegroup auth_core
} from './auth'
```

Only the named elements become visible. Supported element kinds:
`table` (records and refs come along), `enum`, `tablepartial`, `note`
(sticky note), `schema` (everything under it), `tablegroup` (the group
and its tables). Kind keywords are case-insensitive.

## 3. Per-element aliases

```dbml
use { table users as auth_users }    from './auth'
use { table users as billing_users } from './billing'
```

`as` renames an import to dodge conflicts; once aliased, only the alias
is visible.

## 4. Re-export with `reuse`

```dbml
// common/index.dbml
reuse * from './users'
reuse * from './orders'
```

`use` is file-private and **not transitive**; `reuse` additionally
re-exports, so files importing `common/index` see `users` and `orders`
without knowing the internal folder structure.

**Other properties:** circular file imports were permitted (DBML being
declarative); name conflicts between imported and local elements were
errors, resolved by selective import and aliases.

## Why Volt removed it

All four mechanisms manage one problem — name conflicts caused by
dumping declarations into the importer's scope. Volt's package system
(qualified access only: `db.users`) makes the conflicts impossible, so
the machinery solving them is deleted rather than carried:
`use *` (the conflict generator), selective import and element aliases
(the conflict manager), and `reuse` (structure-hiding re-export, which
neither Go nor Odin has). Circular imports become a check error at the
package level — layering is enforced, and intra-package references
never needed imports in the first place.

**Migration:** files that imported each other become one package
(delete the import statements); genuinely separate concerns become
separate directories with `package` clauses and root-relative
`import` blocks (add the package qualifier at use sites).
