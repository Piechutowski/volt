# The Volt Language — Specification (§V)

**Version:** 0.1 (v0 surface: packages, imports, pipelines, scopes,
routes, resources)
**Status:** Normative for the v0 implementation in this repository.
**Part of:** [Volt](README.md). The schema layer is specified by
[`lang/SPEC.md`](lang/SPEC.md); this document specifies only what the
routing layer adds. Section numbers here are prefixed **§V** to keep the
two documents cross-referencable; diagnostics cite the section they
enforce (`spec/V4`).

Volt is **one language**, grown in layers:

```
schema core (DBML-derived)  ⊂  schema extensions  ⊂  routing (this document)
```

Every construct below is specified by (1) a grammar production in EBNF
(notation of lang/SPEC.md §1), (2) an enumerated list of constraints, and
(3) a minimal example. The collected grammar appears in
[Appendix VA](#appendix-va-collected-grammar). The executable companion
is the conformance corpus in [`lang/conformance/`](lang/conformance/):
files under `valid/` MUST be accepted by a conforming implementation,
files under `invalid/` MUST be rejected.

---

## §V0. Layers, files and the superset rule

1. A Volt source file conventionally uses the extension **`.volt`**.
   The file's *dialect* — plain DBML, EDBML, or full Volt — follows
   from its content, never from its name.
2. **Superset rule.** Every valid EDBML program whose declarations avoid
   the import statements of lang/SPEC.md §7 is a valid Volt file body
   (§V2.5 removes `use`/`reuse`). The lexical grammar is lang/SPEC.md §3
   with one addition:

   ```ebnf
   slash = "/" ;
   ```

   `/` is a token wherever it is not the start of a `//` or `/*`
   comment. Consequence: **`/*` always opens a block comment**, which
   is why the rest-of-path wildcard is spelled `:name...` (§V4.1.4)
   and not `*name` — after a slash, a star-marked wildcard is lexically
   unreachable in a superset of DBML.
3. Declaration order and file assignment carry no semantics (§V1.5).

## §V1. Projects and packages

```ebnf
package clause = "package", name, newline ;
```

1. A **project** is a directory tree rooted at the nearest ancestor
   directory containing a file named **`volt.mod`**. `volt.mod`
   contains comments (`//`), blank lines, and exactly one directive:
   `module <name>`.
2. Every `.volt` file MUST begin with a package clause: its first
   declaration, at most one per file. The name is a plain (unquoted)
   identifier. The name `volt` is reserved: generated files import the
   volt runtime under that qualifier (§V7), so a package of that name
   is an error.
3. A **package** is a directory: all `.volt` files in one directory
   MUST declare the same package name, and share one namespace per
   element kind (tables, enums, partials, pipelines, scopes — each per
   lang/SPEC.md §8.2 extended with §V3.1 and §V4).
4. In every directory other than the project root, the package name
   MUST equal the directory's base name. The root directory's package
   may take any name.
5. **Layout invariance.** Splitting or merging the `.volt` files of a
   package, or reordering declarations, MUST NOT change the meaning of
   the package. Implementations MUST process a package as the
   concatenation of its files in file-name order, and route expansion
   (§V4.7) follows declaration order within that concatenation.
6. Directories whose name begins with `.` or `_`, and directories named
   `node_modules`, are not part of the project. Neither is any
   subdirectory containing its own `volt.mod` (a nested project's
   packages belong to it, not to the enclosing module).

```volt
// db/schema.volt
package db

Table users {
	id integer [pk, increment]
}
```

## §V2. Imports

```ebnf
import decl = "import", "(", { import spec }, ")" ;
import spec = [ alias ], import path, newline ;
alias       = name ;
import path = name, { slash, name } ;
```

1. An import declaration is a parenthesized, newline-separated block of
   import specs; an empty block is an error. Import declarations appear
   after the package clause, before other use of the imported names.
2. An import path names a package by its directory path from the
   project root, `/`-separated, with no `./`, no `..`, and no interior
   whitespace; its segments, and the alias, are plain (unquoted)
   identifiers (§V4.1.6). The path MUST name an existing package of
   the project.
3. The imported package is referenced through its **qualifier**: the
   alias when given, else the last path segment. All cross-package
   references are qualifier-prefixed (`db.User`); nothing is ever
   imported into the local scope.
4. Within one package (across all its files): importing the same path
   twice is idempotent, but with two different qualifiers it is an
   error; two different paths MUST NOT share a qualifier; a package
   MUST NOT import itself; **every import MUST be used** — an import
   whose qualifier is never referenced is an error.
5. **DBML file imports are removed.** The `use` and `reuse` statements
   of lang/SPEC.md §7 are not part of the Volt language at any layer; a
   conforming implementation rejects them with a migration diagnostic.
   (The parser MAY still recognize the syntax so the DBML conformance
   corpus remains checkable; acceptance is what is forbidden.)
6. **Import cycles are errors.** The package import graph MUST be
   acyclic.

```volt
package app

import (
	db
	d2 shared/dicts
)
```

## §V3. Pipelines

```ebnf
pipeline = "Pipeline", name, "{", { plug }, "}" ;
plug     = "use", go ref, newline ;
go ref   = name, [ ".", name ] ;
```

1. A Pipeline is a named, ordered middleware list. Pipeline names form
   one namespace per package and MUST be unique. A pipeline body
   contains only plug lines; it MAY be empty.
2. A plug references Go middleware of type
   `func(http.Handler) http.Handler` by name: `volt.<Name>` (the volt
   runtime), `<Name>` or `<package name>.<Name>` (a function of the
   containing package). Referencing an *imported* package's function is
   not supported in v0 and is an error suggesting a local wrapper.
   Whether the referenced Go function exists is the Go compiler's
   business, not the checker's.
3. Pipelines contribute middleware in declaration order, outermost
   first, composed statically in generated code (§V4.4).

```volt
Pipeline api {
	use volt.RequestID
	use BearerAuth
}
```

## §V4. Scopes and routes

### §V4.1 Route paths

```ebnf
route path = slash,
             [ segment, { slash, segment } ] ;
segment    = name
           | ":", name, [ "(", type name, ")" ]
           | ":", name, "...", "...", "..." ;  (* three '.' tokens *)
type name  = "int" | "int32" | "int64" | "string" ;
```

1. All tokens of a route path are **contiguous**: interior whitespace
   ends the path. A bare `/` is the root path. A non-root path MUST NOT
   end with `/` and MUST NOT contain an empty segment (`//`): routes
   match exactly (§V4.4), so a trailing slash would spell a silently
   different pattern.
2. A `:name` segment captures one path segment as a parameter. The
   parameter name MUST be a valid, non-keyword Go identifier, MUST NOT
   be one of the reserved names `w`, `r`, `opts`, `volt` (they appear
   in generated signatures and would be shadowed), and MUST be unique
   within the route's full path (scope prefixes included).
3. A parameter's type defaults to `string`; the closed type set is
   `int`, `int32`, `int64`, `string`, chosen to coincide with the Go
   types the model generator emits for routable primary keys.
   **Types shape handler signatures, never matching**: a request
   segment that fails to parse as the declared type is that route's
   404, not a fallthrough to a sibling route.
4. `:name...` captures the rest of the path (a **wildcard**), always as
   `string`. It MUST be the last segment of the full path, MUST NOT
   carry a type, and MUST NOT appear in a Scope prefix. (Spelling
   rationale: §V0.2.)
5. Grammar note: a path is assembled from ordinary tokens; the
   contiguity rule of clause 1 is what makes it one lexical island.
6. Every segment name — literal, parameter or wildcard — MUST be a
   plain (unquoted) identifier of ASCII letters, digits and
   underscores. Segment names flow verbatim into registration patterns
   and generated Go; the closed character set is what makes that safe
   (and is why `{`/`}` can serve as the collision-free conflict markers
   of §V4.7.1). The same constraint applies to the `resources` table
   name (§V5.1), to the identifiers of pipeline names and plug
   references (§V3), to route handler names (§V4.3), and to import
   aliases and path segments (§V2.2).

### §V4.2 Routes

```ebnf
route = verb, route path, handler ref, [ settings ], newline ;
verb  = "get" | "post" | "put" | "patch" | "delete"
      | "options" | "head" | "any" ;
```

1. Routes appear only inside a Scope body. The verb is a contextual
   keyword, case-insensitive; `any` registers the route for every
   method (its registration carries no method).
2. A route's **full path** is the enclosing scopes' prefixes followed
   by its own path, in nesting order.
3. Registration semantics are `http.ServeMux` (Go ≥ 1.22) exactly:
   parameters register as `{name}`, wildcards as `{name...}`, and a
   root full path as `/{$}`. Matching is exact — Volt never registers
   subtree patterns.

### §V4.3 Handler references

```ebnf
handler ref = name, ".", name ;
```

1. A handler is `Controller.Action`: exactly two exported Go
   identifiers. The controller MUST NOT be an import qualifier —
   handlers live in the routes package.
2. Every distinct controller becomes one generated interface; every
   distinct action one method with the route's typed parameters
   appended after `(w http.ResponseWriter, r *volt.Request)`, returning
   `error`.
3. Two routes MAY share `Controller.Action` only with identical
   parameter signatures (names, types and wildcard-ness, in order).

### §V4.4 Scopes

```ebnf
scope      = "Scope", route path, [ settings ], "{", { scope item }, "}" ;
scope item = route | resources | scope ;
```

Scope settings (the complete set):

| Setting | Value | Meaning |
|---|---|---|
| `pipe` | Pipeline name | append the pipeline to the inherited chain |
| `name` | identifier | prefix for helper names beneath this scope |
| `error_handler` | function of this package (`Name` or `pkg.Name`) | error spine for routes beneath, nearest wins |

1. Scopes nest arbitrarily. Prefixes concatenate; `pipe` chains
   **append** (ancestors outermost); `name` prefixes concatenate;
   `error_handler` is overridden by the nearest enclosing setting.
2. A route's middleware is the concatenation of its pipeline chain's
   plugs, composed once at generation time — never iterated per
   request.
3. The `error_handler` function has the volt runtime's ErrorHandler
   shape: `func(http.ResponseWriter, *volt.Request, error)`. Routes
   without one use the runtime default.

### §V4.5 (reserved)

### §V4.6 Route names and reverse URLs

1. Every route derives a **helper name**: the scope name prefixes
   followed by the `[name:]` setting (normalized to a Go name) when
   present, else the action name. Resources derive per §V5.4.
2. Each named route yields one generated function
   `Path<Helper>(typed params..., opts ...volt.URLOption) string` that
   produces exactly the paths its route matches. Helper names form one
   namespace per package; a collision is an error. Helper output is
   always a clean path: string parameters are percent-escaped, and the
   values `.` and `..` escape entirely (`%2E` forms) — as literal
   segments they would change the path's shape under cleaning.

### §V4.7 Route conflicts

1. A route's **shape** is its full path with every parameter replaced
   by `{}` and every wildcard by `{...}`; literals compare by spelling.
   (Braces cannot appear in a literal segment, §V4.1.6, so the markers
   cannot collide with any literal.)
2. Two routes **overlap** when some request matches both: their methods
   overlap (equal, or either is `any`) and their paths overlap (some
   request path satisfies both patterns). A route is **more specific**
   than another when every request it matches the other also matches,
   and not conversely. Two overlapping routes where **neither is more
   specific** are ambiguous — an error naming both source positions.
   Identical method-and-shape is the degenerate case (reported as a
   duplicate); routes differing in literal-vs-parameter at some
   position while one is strictly more specific (e.g. `/users/new` vs
   `/users/:id`) are legal, and ServeMux precedence picks the more
   specific one at runtime.
3. This is exactly the rule `http.ServeMux` enforces by panicking at
   registration time. Detection therefore happens at check time, so a
   checked package always registers cleanly; the registration panic
   remains as a backstop a conforming generator never triggers.

```volt
Scope / [pipe: api, error_handler: Errors] {
	get /            Home.Index [name: root]
	Scope /admin [name: admin] {
		get /stats   Admin.Stats
	}
	get /files/:path...           Files.Serve
	get /users/:id(int32)/avatar  Users.Avatar
}
```

## §V5. Resources

```ebnf
resources = "resources", model ref, [ settings ], newline ;
model ref = name, [ ".", name ] ;
```

### §V5.1 Declaration

1. `resources <table>` appears only inside a Scope body and expands to
   the action routes of §V5.2 with the table name as the collection
   segment. The name MUST map to a Go identifier.

### §V5.2 The action table

| Action | Method(s) | Path | Helper |
|---|---|---|---|
| `index` | GET | `/<table>` | plural |
| `new` | GET | `/<table>/new` | `New` + singular |
| `create` | POST | `/<table>` | — |
| `show` | GET | `/<table>/:<param>` | singular |
| `edit` | GET | `/<table>/:<param>/edit` | `Edit` + singular |
| `update` | PATCH **and** PUT | `/<table>/:<param>` | — |
| `delete` | DELETE | `/<table>/:<param>` | — |

`update` expands to two routes sharing one action and signature.

### §V5.3 Settings

| Setting | Value | Meaning |
|---|---|---|
| `api` | flag | restrict to index, create, show, update, delete |
| `only` | action list `(index, show)` | keep only these actions |
| `except` | action list | drop these actions |
| `param` | identifier | key parameter name (default `id`) |
| `singular` | identifier | the singular used for member helper names (§V5.4) |

1. Action names in `only`/`except` are the lowercase names of §V5.2;
   unknown names are errors. `only` and `except` MUST NOT be combined.
   `only`/`except` filter the action set after `api`.
2. When the declaration does not resolve to a model, the key
   parameter's type is `int64`.
3. `singular` overrides the inflector of §V5.4.2. It is required
   whenever singularization leaves the name unchanged, which would
   otherwise make the collection and member helpers collide.

### §V5.4 Model resolution

1. The declaration names a **model** of this package (`User`) or of an
   imported package (`db.User`); a qualified reference marks the import
   used (§V2.4).
2. The name resolves against the target package's tables by the model
   naming of lang/SPEC.md (the `[model:]` table setting when present,
   else the singularized table name). A qualified name that resolves to
   nothing is an error; an unqualified one that resolves to nothing is
   a resource without a schema (clause 6).
3. The resolved table MUST have a single-column primary key whose
   Go type is `int`, `int32`, `int64` or `string`; that type
   becomes the key parameter's type. Composite, missing, or unroutable
   keys are errors.
4. A resolved declaration fixes every derived name from the schema,
   not from the spelling: the URL segment and the controller come from
   the **table** name, the member helper from the **model** name. No
   singularization is guessed, so a non-English table name needs no
   help.
5. An unresolved (schemaless) declaration derives all of it from the
   name as written: plural = its Go name, singular = the `singular:`
   setting when given, else its deterministic singularization (the
   inflector implements English rules only).
6. Plural and singular MUST differ: a name whose singularization is the identity
   (`posty`, `data`, `series`) would give the collection and member
   helpers one name, and is an error naming `singular:` as the fix.
   Neither a route `name:` nor a scope `name:` can resolve it — the
   former is not a resources setting (§V6), the latter prefixes both
   sides equally.

```volt
resources db.User [only: (index, show, create)]
```

## §V6. Settings whitelists

Settings valid on Volt elements, exhaustively (a setting not listed for
an element is an error on that element):

| Element | Settings |
|---|---|
| Scope | `pipe`, `name`, `error_handler` |
| route | `name` |
| resources | `api`, `only`, `except`, `param`, `singular` |

The identifier-list value form `(a, b, c)` (production in Appendix VA)
is valid only where a setting explicitly takes an action list.

## §V7. Generation contract (informative)

The normative output contract is the golden corpus under
`gen/router/testdata/` and the proof suite under `itest/`. In prose:
one `<Controller>Controller` interface per controller and a
`Controllers` struct (§V4.3); `New(Controllers) http.Handler`
registering every route onto a `http.ServeMux` with its pipeline chain
composed statically and its typed shim parsing parameters per §V4.1.3;
`Path*` helpers per §V4.6; a `Table []volt.Route` mirroring the
expanded route list in declaration order. All generated files carry the
standard generated-code header and are gofmt-stable.

## §V8. Reserved words for future layers

`Dataset` is reserved: a conforming v0 implementation rejects it with a
forward-pointing diagnostic. (Design: docs/router.md §12.)

---

## Appendix VA: Collected grammar

Additions to the collected grammar of lang/SPEC.md Appendix A. The
`element` production is extended:

```ebnf
element        = (* lang/SPEC.md Appendix A alternatives *)
               | package clause | import decl | pipeline | scope ;

slash          = "/" ;

package clause = "package", name, newline ;

import decl    = "import", "(", { import spec }, ")" ;
import spec    = [ alias ], import path, newline ;
alias          = name ;
import path    = name, { slash, name } ;

pipeline       = "Pipeline", name, "{", { plug }, "}" ;
plug           = "use", go ref, newline ;
go ref         = name, [ ".", name ] ;

scope          = "Scope", route path, [ settings ],
                 "{", { scope item }, "}" ;
scope item     = route | resources | scope ;

route          = verb, route path, handler ref, [ settings ], newline ;
verb           = "get" | "post" | "put" | "patch" | "delete"
               | "options" | "head" | "any" ;
handler ref    = name, ".", name ;

route path     = slash, [ segment, { slash, segment } ] ;
segment        = name
               | ":", name, [ "(", type name, ")" ]
               | ":", name, "." , ".", "." ;
type name      = "int" | "int32" | "int64" | "string" ;

resources      = "resources", name, [ settings ], newline ;

(* setting value, extended (lang/SPEC.md §4.2): *)
setting value  = (* schema-layer alternatives *) | ident list ;
ident list     = "(", name, { ",", name }, ")" ;
```

All tokens of a `route path` and of an `import path` MUST be contiguous
(§V4.1.1, §V2.2) — the grammar above is subject to that adjacency
constraint, which the token stream expresses via inter-token whitespace
flags.

---

## Conformance and the proof chain

Testing never proves software correct; a specification with an
executable conformance surface narrows the gap deliberately. The v0
chain, each link runnable by `go test ./...`:

1. **Corpus ↔ spec.** Every file in `lang/conformance/snippets/`
   carries a `// spec: §V…` tag; `valid/` MUST check clean, `invalid/`
   MUST be rejected. The corpus is the spec's executable surface.
2. **Schema layer preserved.** The schema conformance corpus continues to run
   unchanged against the same front end: the superset rule (§V0.2) is
   enforced, not assumed.
3. **Generator ↔ contract.** Goldens are byte-compared, gofmt-stable by
   test, and **compiled by the real Go toolchain** against stub
   implementations of the generated interfaces — the typed-signature
   contract (§V4.3) is checked by the Go compiler itself.
4. **Router semantics.** The `itest/` fixture app is generated,
   committed, drift-checked against regeneration, and exercised over
   httptest: match and 404/405 behavior, typed-parameter 404s (§V4.1.3),
   pipeline order (§V4.4), the error spine (§V4.4.3), and
   **reverse-URL round-trip totality** — for every route in the
   generated table, the URL built by its helper is served back to the
   router and MUST dispatch to that same route. The §V4.6 property is
   not sampled; it is enumerated.
