# Phoenix — Feature Inventory

An exhaustive inventory of what the **Phoenix** web framework and its core ecosystem
(**Ecto**, **LiveView**, **Plug**) offer, treated as one framework the way a real
Phoenix application uses them. Compiled as research input for Volt from the official
guides corpus:

- Phoenix **v1.8.9** — `phoenixframework/phoenix` @ `734c8d1c` (46 guide files)
- Phoenix LiveView **v1.2.7** — `phoenixframework/phoenix_live_view` @ `2570f650` (18 guide files)
- Ecto **v3.14.1** — `elixir-ecto/ecto` @ `e0aa6e1b` (19 guide files incl. cheatsheets)
- Plug **v1.20.3** — `elixir-plug/plug` @ `9fa11c8e` (README + HTTPS guide + changelog)

All fetched 2026-07-14. **The corpus is the guides**, not the per-module API docs on
hexdocs — capabilities that Phoenix documents only in moduledocs (e.g. most of
`Plug.Conn`'s surface, `Phoenix.Component`'s full component API) appear here only to
the extent the guides name them. Gaps relative to batteries-included frameworks are
recorded deliberately: they are data.

**Tier legend**

| Tier | Meaning |
|---|---|
| `CORE` | Ships in a default `mix phx.new` app (Phoenix, Plug, Ecto+ecto_sql, LiveView, LiveDashboard, Bandit, esbuild/Tailwind, Swoosh, Gettext, Telemetry, Heroicons, dns_cluster…) |
| `OPT` | First-party or hex package explicitly documented but not on by default (flags, adapters, extra config) |
| `ECO` | Third-party but blessed — named in the official guides (Oban, Ueberauth, libcluster, CORSPlug…) |
| `DIY` | Guides document a pattern; no shipped mechanism |

---

## P1 — Routing & HTTP dispatch

**Problem.** Map every incoming verb/path pair to handler code, with grouping,
parameterization, and URL generation that don't rot as the app grows.
**Answer.** A compile-time macro router (`Phoenix.Router`) that expands routes into one
pattern-matched function optimized by the BEAM, plus compiler-verified path literals
(`~p`) so broken links fail at build time, not in production.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ROUTE-1 | HTTP-verb macros `get`/`post`/`put`/`patch`/`delete`/`options`/`connect`/`trace`/`head` compile all routes into one pattern-matched case statement. | CORE | phoenix/routing.md; macro-based for speed + metadata |
| ROUTE-2 | `resources "/users", UserController` expands to the 8 conventional actions (index/edit/new/show/create/update/delete, PUT+PATCH both map to update). | CORE | phoenix/routing.md |
| ROUTE-3 | `:only` and `:except` options filter which resource actions are generated. | CORE | phoenix/routing.md |
| ROUTE-4 | Nested resources (`resources ... do resources ... end`) produce `/users/:user_id/posts/:id` style routes. | CORE | phoenix/routing.md |
| ROUTE-5 | `scope "/admin", HelloWeb.Admin` groups routes under a path prefix and module namespace; scopes nest arbitrarily; multiple scopes may share a path. | CORE | phoenix/routing.md |
| ROUTE-6 | Duplicate routes are caught at compile time ("this clause cannot match" warning). | CORE | phoenix/routing.md |
| ROUTE-7 | `pipeline`/`pipe_through` attach named plug stacks to route scopes; default `:browser` (accepts, session, live flash, root layout, CSRF, secure headers) and `:api` (accepts json) pipelines. | CORE | phoenix/routing.md |
| ROUTE-8 | Pipelines are themselves plugs, so pipelines can plug other pipelines (composition), and a request runs all pipelines of all enclosing scopes in order. | CORE | phoenix/routing.md |
| ROUTE-9 | Verified routes: `~p"/users/#{user}"` sigil emits a compile-time warning for any path that doesn't match the router. | CORE | phoenix/routing.md; `Phoenix.VerifiedRoutes` |
| ROUTE-10 | `~p` supports query strings via interpolated keyword lists/maps; `url(~p"...")` builds full URLs from endpoint host/port/SSL config. | CORE | phoenix/routing.md |
| ROUTE-11 | `Phoenix.Param` protocol lets structs interpolate directly into paths (`~p"/users/#{user}"` plucks the id — or a slug via `@derive {Phoenix.Param, key: :slug}`). | CORE | phoenix/routing.md, phoenix/authn_authz/scopes.md |
| ROUTE-12 | Path segments declare params (`get "/hello/:messenger"`) delivered as string-keyed entries in `params`. | CORE | phoenix/request_lifecycle.md |
| ROUTE-13 | `mix phx.routes` prints the full routing table for introspection. | CORE | phoenix/routing.md |
| ROUTE-14 | `forward "/jobs", SomePlug` mounts an entire plug (sub-app, admin UI) under a path prefix, composable with pipelines; forwarding to another Phoenix endpoint is advised against. | CORE | phoenix/routing.md |
| ROUTE-15 | `live "/thermostat", ThermostatLive` routes directly to a LiveView module; only router-mounted LiveViews get live navigation. | CORE | phoenix/live_view.md, liveview/server/live-navigation.md |
| ROUTE-16 | `live_session` groups live routes at the router level with shared `on_mount` hooks and `:root_layout`, allowing websocket reuse across navigation within the group. | CORE | liveview/server/security-model.md |
| ROUTE-17 | Versioned APIs are plain nested scopes (`scope "/api" ... scope "/v1", V1`); no dedicated versioning machinery. | CORE | phoenix/routing.md |
| ROUTE-18 | `Plug.Router` offers a standalone micro-router (`plug :match; plug :dispatch`, `get`/`match`/`forward` macros) compiled to a VM-optimized tree lookup — usable without Phoenix. | CORE | plug/README.md |
| ROUTE-19 | Router cheatsheet documents the canonical `~p` output for every resource/scope shape. | CORE | phoenix/cheatsheets/router.cheatmd |

## P2 — Request handling: controllers & middleware

**Problem.** Turn a raw connection into a response through composable, reorderable
processing steps with clean short-circuiting.
**Answer.** Plug: a single immutable `%Plug.Conn{}` flows through function plugs and
module plugs (`init/1` + `call/2`) at endpoint, router-pipeline, and controller level;
controllers are themselves plugs whose actions are plain 2-arity functions.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CTRL-1 | Plug contract: function plugs (`conn, opts -> conn`) and module plugs (`init/1`, `call/2`); everything (endpoint, router, controller) is a plug. | CORE | phoenix/plug.md, plug/README.md |
| CTRL-2 | Immutable `%Plug.Conn{}` unifies request+response; `assign/3` stores per-request data; `halt/1` stops the pipeline; conn is a direct interface to the server so `send_resp` streams immediately. | CORE | phoenix/plug.md, plug/README.md |
| CTRL-3 | `Phoenix.Endpoint` is the shared entry pipeline for all requests; anything that must run on every request goes there before the router. | CORE | phoenix/request_lifecycle.md |
| CTRL-4 | Default endpoint plugs: `Plug.Static`, `Phoenix.LiveDashboard.RequestLogger`, `Plug.RequestId`, `Plug.Telemetry`, `Plug.Parsers`, `Plug.MethodOverride`, `Plug.Head`, `Plug.Session`, then the router. | CORE | phoenix/plug.md |
| CTRL-5 | Dev-only endpoint block: live reload socket, `Phoenix.CodeReloader`, and `Phoenix.Ecto.CheckRepoStatus` (actionable "run migrations" error page). | CORE | phoenix/plug.md |
| CTRL-6 | Controller actions are functions `action(conn, params)`; params pattern-matched in heads (`%{"messenger" => m}`); string keys always. | CORE | phoenix/controllers.md |
| CTRL-7 | Controller-scoped plugs with action guards: `plug :authenticate when action in [:index]`. | CORE | phoenix/plug.md |
| CTRL-8 | Plug-based auth/authz chains replace nested conditionals: `plug :authenticate; plug :fetch_message; plug :authorize` with `halt()` on failure paths. | CORE | phoenix/plug.md |
| CTRL-9 | Rendering helpers: `render/3` (view dispatch), `text/2`, `json/2`, `html/2`. | CORE | phoenix/controllers.md |
| CTRL-10 | Raw response composition: `send_resp/3`, `put_resp_content_type/2`, `put_status/2` (integer or atom names via `Plug.Conn.Status`). | CORE | phoenix/controllers.md |
| CTRL-11 | `redirect/2` distinguishes `to:` (in-app paths only — open-redirect protection) from `external:` full URLs. | CORE | phoenix/controllers.md |
| CTRL-12 | Flash messages: `put_flash/3`, `clear_flash/1`, `Phoenix.Flash.get/2`, rendered via the `flash_group` core component; survives redirects; keys are free-form (`:info`/`:error` conventional). | CORE | phoenix/controllers.md |
| CTRL-13 | `action_fallback SomeController` centralizes non-`%Plug.Conn{}` action returns (e.g. `{:error, :not_found}`) into one plug — the backbone of API error handling. | CORE | phoenix/json_and_apis.md |
| CTRL-14 | `Plug.Parsers` parses urlencoded, multipart, and JSON bodies with `:length` (8 MB default), `:read_length`, `:read_timeout` limits — documented as a slow-loris/DoS mitigation. | CORE | phoenix/plug.md, phoenix/howto/file_uploads.md |
| CTRL-15 | Params from path, body, and query merge into one map with priority path > body > query; raw sources stay available on `conn.path_params` / `conn.body_params` / `conn.query_params`. | CORE | phoenix/json_and_apis.md |
| CTRL-16 | `Plug.MethodOverride` (form `_method` → PUT/PATCH/DELETE) and `Plug.Head` (HEAD → GET) normalize verbs. | CORE | phoenix/plug.md |
| CTRL-17 | `Plug.Session` sets up cookie-backed session storage; `fetch_session` plug materializes it; `put_session`/`get_session` used throughout auth flows. | CORE | phoenix/plug.md, liveview/server/security-model.md |
| CTRL-18 | Batteries in Plug proper: `Plug.BasicAuth`, `Plug.Logger`, `Plug.RewriteOn` (x-forwarded-* rewriting), `Plug.SSL`, `Plug.Static`, `Plug.CSRFProtection`. | CORE | plug/README.md |
| CTRL-19 | Error views `ErrorHTML`/`ErrorJSON` render exceptions per-format from one place; default returns the status message for the template name ("404.html" → "Not Found"). | CORE | phoenix/controllers.md, phoenix/howto/custom_error_pages.md |
| CTRL-20 | Custom exceptions map to statuses via the `Plug.Exception` protocol or a `plug_status` struct field; exceptions can define "actions" (label + MFA) rendered as buttons on the dev error page. | CORE | phoenix/howto/custom_error_pages.md |
| CTRL-21 | `Plug.Debugger` (rich dev error page) and `Plug.ErrorHandler` (custom production error hook) wrap any plug pipeline. | CORE | plug/README.md |
| CTRL-22 | Plug v1.14+ connection `upgrade` API gives WebSocket support to bare Plug apps via `WebSockAdapter.upgrade/4`. | OPT | plug/README.md; needs `websock_adapter` |
| CTRL-23 | Server adapters are swappable behind Plug: Bandit (default in new apps) or Cowboy via `plug_cowboy`. | CORE | plug/README.md, phoenix/introduction/up_and_running.md |

## P3 — Views, templating & frontend assets

**Problem.** Produce HTML (and other formats) safely and fast, keep markup reusable, and
ship JS/CSS without a fragile external toolchain.
**Answer.** HEEx — an HTML-aware, compile-checked template language where every template
is a *function component* — plus zero-Node asset building through Elixir-wrapped
esbuild and Tailwind.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VIEW-1 | Function components: any function taking `assigns` and returning a `~H` template; declared attributes via `attr/3` with `:required`/`:default` and compile-time warnings on misuse. | CORE | phoenix/components.md |
| VIEW-2 | HEEx (`~H` / `.heex`): HTML-validated templates — unclosed/mistyped tags are compile errors; `{expr}` interpolation in bodies and attributes; `<%= %>` for block constructs. | CORE | phoenix/components.md |
| VIEW-3 | Automatic HTML escaping of all interpolated values (XSS-safe by default); `raw/1` is the explicit, documented-as-dangerous opt-out. | CORE | phoenix/components.md, phoenix/security.md |
| VIEW-4 | Attribute smarts: `class={@class}` handles `false`/nil removal and class lists; `{@many_attributes}` splats dynamic attribute maps. | CORE | phoenix/components.md |
| VIEW-5 | `:if` and `:for` special attributes as shorthand for conditionals and comprehensions on any tag; `:key` optimizes list diffing. | CORE | phoenix/components.md, liveview/server/assigns-eex.md |
| VIEW-6 | Slots: components take inner content (`<Layouts.app>...</Layouts.app>`, `<:col :let={item}>`, `render_slot(@inner_block)`). | CORE | phoenix/data_modelling/more_examples.md, liveview/server/assigns-eex.md |
| VIEW-7 | View modules per format co-located with controllers (`HelloHTML`, `HelloJSON`); `embed_templates "hello_html/*"` compiles template files into function components — file vs. in-module function is equivalent. | CORE | phoenix/request_lifecycle.md |
| VIEW-8 | Two-tier layouts: a root layout (html skeleton, set via `put_root_layout` plug) plus an explicit `<Layouts.app flash={@flash}>` function component invoked inside templates; alternate layouts are just more functions (`Layouts.admin`). | CORE | phoenix/components.md, liveview/server/live-layouts.md |
| VIEW-9 | `CoreComponents` module generated into every app (`.input`, `.form`, `.button`, `.table`, `.header`, `.list`, `.icon`, `.flash_group`) — the substrate the generators build on. | CORE | phoenix/components.md |
| VIEW-10 | Community component systems (Petal, Doggo, SaladUI, Bloom, PrimerLive, Fluxon, Mishka Chelekom) named as blessed richer alternatives. | ECO | liveview/README.md |
| VIEW-11 | `phoenix_html` package provides the safe-HTML building blocks (`Phoenix.HTML`) imported into all templates. | CORE | phoenix/components.md |
| VIEW-12 | esbuild via the Elixir `esbuild` wrapper: bundles `assets/js/app.js` to `priv/static/assets`, dev watcher, no Node.js required. | CORE | phoenix/asset_management.md |
| VIEW-13 | Tailwind CSS via the Elixir `tailwind` wrapper; v1.8 apps add daisyUI with light/dark/system theming. | CORE | phoenix/asset_management.md, phoenix/CHANGELOG.md |
| VIEW-14 | Heroicons shipped as a sparse git dep and embedded as Tailwind CSS classes so only used icons ship; recipe for swapping icon sets. | CORE | phoenix/asset_management.md |
| VIEW-15 | Third-party JS: vendor files, `npm --prefix assets`, or Mix git deps (`app: false, compile: false`) all supported; esbuild resolves `deps/`. | CORE | phoenix/asset_management.md |
| VIEW-16 | `mix assets.deploy` = minified build + `mix phx.digest` fingerprinting with a cache manifest for production asset serving; `phx.digest.clean`. | CORE | phoenix/deployment/deployment.md |
| VIEW-17 | Escape hatches documented: custom esbuild build scripts (plugins/SASS), replacing esbuild or Tailwind entirely, custom `watchers:` config. | CORE | phoenix/asset_management.md |
| VIEW-18 | Static files served from `priv/static` by `Plug.Static`; images/fonts marked `--external` in esbuild but still digested. | CORE | phoenix/asset_management.md |
| VIEW-19 | HEEx debug annotations in dev (caller comments + `data-phx-loc`), opt-out per module via `@debug_heex_annotations false`. | CORE | liveview/CHANGELOG.md |
| VIEW-20 | `mix format` formats HEEx (`Phoenix.LiveView.HTMLFormatter`); `phx-no-format` opt-out; `TagFormatter` behaviour formats embedded script/style via third-party tools (e.g. prettier). | CORE | liveview/cheatsheets/html-attrs.cheatmd, liveview/CHANGELOG.md |
| VIEW-21 | Non-HTML formats via templates too: `home.xml.eex` + `put_format(:xml)` renders XML through plain EEx. | CORE | phoenix/controllers.md |

## P4 — Data layer: models, ORM & queries

**Problem.** Move data between the database and language structs, and express the whole
range of SQL — from CRUD to lateral joins — safely and composably.
**Answer.** Ecto's four-part split: `Repo` (the only thing that talks to the database),
`Schema` (pure struct↔source mapping), `Query` (compile-checked, parameterized,
composable DSL), and `Changeset` (see P6). No lazy loading, no global model objects.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ORM-1 | `Ecto.Repo`: application-owned repository module (`use Ecto.Repo, otp_app:, adapter:`) handling connection pooling, adapter dispatch, and constraint-error translation; started in the supervision tree. | CORE | phoenix/ecto.md, ecto/introduction/Getting Started.md |
| ORM-2 | Adapters: PostgreSQL (postgrex), MySQL (myxql), MSSQL (tds), SQLite3 (ecto_sqlite3), ClickHouse (ecto_ch), ETS (etso); chosen at `phx.new --database` or by swapping deps+adapter. | CORE/OPT | phoenix/ecto.md, ecto/README.md, phoenix/howto/swapping_databases.md |
| ORM-3 | `Ecto.Schema`: `schema "users" do field :email, :string ... end` maps *any* source to a struct; typed fields with defaults; singular module / plural table convention. | CORE | ecto/introduction/Getting Started.md |
| ORM-4 | Schema conventions with escape hatches: autogenerated integer `id` pk (customizable via `@primary_key {:org_id, :id, autogenerate: true}`), `timestamps/1` for inserted_at/updated_at. | CORE | phoenix/ecto.md, ecto/howtos/Multi tenancy with foreign keys.md |
| ORM-5 | Virtual fields (`virtual: true`) and `redact: true` (kept out of inspect/logs) on schema fields. | CORE | phoenix/security.md, ecto/howtos/Data mapping and validation.md |
| ORM-6 | `Ecto.Enum` typed enum fields (`field :visibility, Ecto.Enum, values: [:public, :private]`). | CORE | ecto/howtos/Embedded Schemas.md |
| ORM-7 | Repo read API: `get/2`, `get_by/2` (+ `!` raising variants), `one/2`, `all/1`, `all_by/3`, `exists?/1`, `first`/`last` query helpers. | CORE | ecto/cheatsheets/crud.cheatmd, ecto/introduction/Getting Started.md |
| ORM-8 | Repo write API: `insert/2`, `update/2`, `delete/2` returning `{:ok, struct}` or `{:error, changeset}`, plus raising `!` variants (`Ecto.InvalidChangesetError`). | CORE | ecto/introduction/Getting Started.md |
| ORM-9 | Bulk operations: `insert_all/3` (with `{:placeholder, key}` value reuse), `update_all/3`, `delete_all/2` — all schemaless-capable, returning `{count, returning}`. | CORE | ecto/howtos/Schemaless queries.md, Constraints and Upserts.md |
| ORM-10 | `Repo.stream/1` lazily streams query results. | CORE | ecto/cheatsheets/crud.cheatmd |
| ORM-11 | `Repo.aggregate/4` (`:avg`, `count`, etc.) with automatic subquery wrapping when the query has limit/offset/distinct. | CORE | ecto/howtos/Aggregates and subqueries.md |
| ORM-12 | Query DSL in two syntaxes — keyword (`from p in Post, where: ...`) and pipe-based (`Post`, then `where(...)`, then `order_by(...)`) — pre-compiled for performance, fields checked against the schema at compile time. | CORE | ecto/howtos/Dynamic queries.md |
| ORM-13 | Interpolated values require the pin operator `^var`, which parameterizes the SQL — the SQL-injection guarantee is structural, not sanitization. | CORE | ecto/introduction/Getting Started.md, phoenix/security.md |
| ORM-14 | Full clause coverage: where/order_by/limit/offset/group_by/having/distinct/select (maps, structs, tuples, `%{u.id => u.email}` shapes), `ilike`, `count`, window functions, CTEs referenced in changelog. | CORE | phoenix/ecto.md, ecto/CHANGELOG.md |
| ORM-15 | Joins: `join:` with `on:`, `assoc(p, :authors)` association joins, named bindings (`as: :authors`), `inner_lateral_join`. | CORE | ecto/howtos/Dynamic queries.md, Aggregates and subqueries.md |
| ORM-16 | Subqueries: `subquery/1` over any Queryable, selectable maps whose keys surface in the parent, `parent_as/1` for correlated subqueries. | CORE | ecto/howtos/Aggregates and subqueries.md |
| ORM-17 | Queries are composable values: build a query, pipe more `where` clauses onto it later, extract reusable query functions. | CORE | ecto/introduction/Getting Started.md, Dynamic queries.md |
| ORM-18 | Data-structure queries: keyword lists/maps accepted by most constructs (`where: [author: "José"]`, `order_by: [desc: :published_at]`) and interpolatable wholesale (`where(^filters)`). | CORE | ecto/howtos/Dynamic queries.md |
| ORM-19 | `dynamic/2` macro builds interpolatable query fragments at runtime — the sanctioned way to do user-driven search filters, composed via `Enum.reduce` over params, including against named join bindings. | CORE | ecto/howtos/Dynamic queries.md |
| ORM-20 | `fragment/1..n` embeds raw SQL snippets; interpolating a string as the fragment is a **compile error** — values must go through `?` placeholders. | CORE | phoenix/security.md |
| ORM-21 | Raw SQL escape hatch `Ecto.Adapters.SQL.query(Repo, sql, params)` with positional parameters. | CORE | phoenix/security.md |
| ORM-22 | Schemaless queries: `from "posts", select: [:title, :body]`, schemaless update_all/delete_all/insert_all, `type/2` for cast guarantees without a schema. | CORE | ecto/howtos/Schemaless queries.md |
| ORM-23 | Atomic update operators on `update_all`: `:set`, `:inc`, `:push`, `:pull` — the documented answer to increment race conditions. | CORE | ecto/howtos/Schemaless queries.md, phoenix/data_modelling/your_first_context.md |
| ORM-24 | Associations: `belongs_to`, `has_one`, `has_many`, `has_many :through`, `many_to_many` (join table or join schema, custom `join_keys`, self-referencing with reverse association). | CORE | ecto/cheatsheets/associations.cheatmd, Self-referencing many to many.md |
| ORM-25 | Explicit preloading only: `preload:` in queries, `Repo.preload/2` post-hoc, or join+`preload: [assoc: binding]` for single-query loads — no lazy loading exists. | CORE | ecto/cheatsheets/associations.cheatmd |
| ORM-26 | `Ecto.build_assoc/3` builds a child struct with the FK set; `Ecto.assoc/2` returns a query for a record's association (e.g. to `update_all` over it). | CORE | ecto/cheatsheets/associations.cheatmd |
| ORM-27 | Nested writes: `cast_assoc/3` (external params matched to children by pk, inserting/updating/deleting per `:on_replace`) vs `put_assoc/4` (internal structs/changesets, e.g. tags parsed from a comma string). | CORE | ecto/howtos/Constraints and Upserts.md, cheatsheets/associations.cheatmd |
| ORM-28 | Upserts: `on_conflict:` (`:nothing`, `[set: ...]`, `[inc: ...]`, replace strategies) + `conflict_target:` on `insert` and `insert_all`; `:replace_changed` enables Postgres HOT updates. | CORE | ecto/howtos/Constraints and Upserts.md, phoenix/data_modelling/cross_context_boundaries.md, ecto/CHANGELOG.md |
| ORM-29 | Transactions: `Repo.transact/2` (fn returns `{:ok, _}` or rolls back; supersedes `transaction/2`) used for multi-step invariants like order checkout. | CORE | phoenix/data_modelling/more_examples.md, ecto/CHANGELOG.md |
| ORM-30 | `Ecto.Multi` for named, inspectable multi-operation transactions (referenced as the alternative to ad-hoc transactions). | CORE | ecto/howtos/Constraints and Upserts.md |
| ORM-31 | Embedded schemas: `embeds_one`/`embeds_many` (inline or extracted module), stored as `:map`/JSONB, validated via `cast_embed`, queryable via `u.profile["visibility"]` (jsonpath). | CORE | ecto/howtos/Embedded Schemas.md |
| ORM-32 | `embedded_schema` as a persistence-free struct+cast+validate tool for forms, API payloads, contact forms (the "form object" pattern). | CORE | ecto/howtos/Data mapping and validation.md, Embedded Schemas.md |
| ORM-33 | Multi-tenancy via query prefixes (Postgres schemas / MySQL databases): connection `search_path`, `@schema_prefix`, per-operation `:prefix`, per-from/join prefix, documented precedence rules; prefix metadata travels on structs (`Ecto.get_meta`/`put_meta`). | CORE | ecto/howtos/Multi tenancy with query prefixes.md |
| ORM-34 | Multi-tenancy via foreign keys: `prepare_query/3` repo callback enforces `org_id` on every query, `default_options/1` + process dictionary sets tenant per process, `:skip_org_id` explicit opt-out. | DIY | ecto/howtos/Multi tenancy with foreign keys.md; documented pattern over repo callbacks |
| ORM-35 | Read replicas: multiple repo modules with `read_only: true` and a `replica/0` picker; sandbox-compatible testing via `:default_dynamic_repo`. | CORE | ecto/howtos/Replicas and dynamic repositories.md |
| ORM-36 | Dynamic repositories: `Repo.start_link(name: ..., hostname: ...)` at runtime + `put_dynamic_repo/1` to point calls at any pool — for per-client databases and short-lived connections. | CORE | ecto/howtos/Replicas and dynamic repositories.md |
| ORM-37 | Custom driver types: `Postgrex.Types.define/3` (e.g. decode Postgres `interval` into Elixir `Duration` for the `:duration` field type). | OPT | ecto/howtos/Duration Types with Postgrex.md |
| ORM-38 | UUIDv7 support and helper functions in `Ecto.UUID`; `binary_id` schemas supported by generators. | CORE | ecto/CHANGELOG.md, phoenix/authn_authz/mix_phx_gen_auth.md |
| ORM-39 | Formatter integration: `import_deps: [:ecto, :ecto_sql]` gives query-DSL-aware `mix format`. | CORE | ecto/introduction/Getting Started.md |

## P5 — Schema evolution: migrations & seeding

**Problem.** Evolve the database schema reproducibly across environments and time, and
load baseline data.
**Answer.** Timestamped migration modules with a reversible `change/0` DSL, mix tasks to
drive them, a `schema_migrations` ledger, and a plain-Elixir seeds script.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MIG-1 | `mix ecto.gen.migration name` generates a timestamped module in `priv/repo/migrations`; generators (`phx.gen.*`) emit migrations automatically. | CORE | phoenix/ecto.md |
| MIG-2 | Single `change/0` callback auto-reverses for rollback; migration DSL: `create table`, `alter`, `add`, `timestamps()`, `drop`. | CORE | phoenix/ecto.md, ecto/introduction/Getting Started.md |
| MIG-3 | `references(:posts, on_delete: :delete_all)` foreign keys — guides push DB-level integrity over app-level cleanup. | CORE | phoenix/data_modelling/in_context_relationships.md |
| MIG-4 | `create index/2` and `create unique_index/2`, incl. composite unique indexes and leftmost-prefix guidance against redundant indexes. | CORE | phoenix/data_modelling/in_context_relationships.md |
| MIG-5 | Column options in migrations: `precision`/`scale` for decimals, `null: false`, `default:`; join tables with `primary_key: false`. | CORE | phoenix/data_modelling/your_first_context.md |
| MIG-6 | `mix ecto.create` / `mix ecto.drop` manage the database itself (per-repo with `-r`). | CORE | phoenix/ecto.md |
| MIG-7 | `mix ecto.migrate` runs pending migrations, tracked in the `schema_migrations` table; `-n/--step` and `--to VERSION` control granularity. | CORE | phoenix/ecto.md |
| MIG-8 | `mix ecto.rollback` reverses migrations with the same step/to options. | CORE | phoenix/ecto.md |
| MIG-9 | Multi-tenant migrations: `mix ecto.migrate --prefix p`, or per-table `prefix:` inside a migration with `flush()` between tenants. | CORE | ecto/howtos/Multi tenancy with query prefixes.md |
| MIG-10 | Seeding: `priv/repo/seeds.exs` executed with `mix run`, calling ordinary context functions; also runnable from the dev error page as an exception "action". | CORE | phoenix/data_modelling/in_context_relationships.md, phoenix/howto/custom_error_pages.md |
| MIG-11 | Production migrations without Mix: `mix phx.gen.release` emits `bin/migrate` overlay + `Release.migrate` using `Ecto.Migrator.with_repo/2`; rollback-to-version helper included. | CORE | phoenix/deployment/releases.md |
| MIG-12 | Dev safety net: `Phoenix.Ecto.CheckRepoStatus` raises an actionable error (with a "migrate now" button) when the DB is out of date. | CORE | phoenix/plug.md |

## P6 — Validation & data integrity

**Problem.** Accept untrusted external data, validate it, report errors usably, and keep
invariants that only the database can truly enforce.
**Answer.** `Ecto.Changeset`: an explicit cast-then-validate pipeline that whitelists
fields (killing mass assignment), separates cheap in-app *validations* from DB-backed
*constraints*, and doubles as the error-carrying data structure for forms and APIs.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VAL-1 | `cast(struct, attrs, allowed_fields)` whitelists and type-casts external params; unknown keys silently dropped — mass assignment protection is opt-in-per-field by design. | CORE | phoenix/ecto.md, phoenix/security.md |
| VAL-2 | `validate_required/2` presence validation. | CORE | phoenix/ecto.md |
| VAL-3 | `validate_length/3` with `min:`/`max:`. | CORE | phoenix/ecto.md |
| VAL-4 | `validate_format/3` regex validation. | CORE | phoenix/ecto.md |
| VAL-5 | `validate_number/3` (`greater_than_or_equal_to:`, `less_than:` …). | CORE | phoenix/data_modelling/cross_context_boundaries.md |
| VAL-6 | Changeset introspection: `valid?`, `errors` (message + metadata keyword list with `%{count}`-style interpolation), `changes`, `apply_changes/1`, `action` annotation for UI error display. | CORE | ecto/introduction/Getting Started.md, Data mapping and validation.md |
| VAL-7 | `traverse_errors/2` flattens errors into a rendering-friendly map (used by the generated `ChangesetJSON`). | CORE | ecto/introduction/Getting Started.md, phoenix/json_and_apis.md |
| VAL-8 | Validations vs constraints: `unique_constraint/2` (and friends) convert DB constraint violations into changeset errors instead of raising — the race-condition-proof complement to validations; `changeset.valid?` explicitly does not cover constraints. | CORE | ecto/howtos/Constraints and Upserts.md, Getting Started.md |
| VAL-9 | Nested validation: `cast_assoc/3` and `cast_embed/3` (with `required:`, `with:` custom changeset fn); child errors invalidate the parent. | CORE | ecto/howtos/Embedded Schemas.md |
| VAL-10 | Schemaless changesets: `{data, types}` tuples get the full cast/validate pipeline without any schema — for search forms, API endpoints. | CORE | ecto/howtos/Data mapping and validation.md |
| VAL-11 | Separate write-models: dedicated `embedded_schema` (e.g. `Registration`) validates UI-shaped input, then transforms to persistence structs — UI shape never dictates DB shape. | DIY | ecto/howtos/Data mapping and validation.md |
| VAL-12 | Repos are changeset-aware: invalid changesets never reach the DB; valid ones produce minimal-diff UPDATEs from tracked changes. | CORE | phoenix/ecto.md |
| VAL-13 | Changesets are the contract between contexts and the web layer — controllers/LiveViews receive `{:error, changeset}` and forms render from it (via `Phoenix.HTML.FormData` protocol). | CORE | phoenix/data_modelling/faq.md |
| VAL-14 | Form error UX: LiveView sends `_unused_` params so `used_input?/1` suppresses errors for untouched fields; opt-out `phx-no-unused-field`. | CORE | liveview/client/form-bindings.md |
| VAL-15 | DB-level integrity emphasized throughout: not-null, composite unique indexes, `on_delete: :delete_all` cascades as the source of truth, not app code. | CORE | phoenix/data_modelling guides |

## P7 — Authentication & authorization

**Problem.** Identify users and constrain what each identity can see and do — across both
stateless HTTP and long-lived stateful connections.
**Answer.** `mix phx.gen.auth` generates a complete, owned-by-you auth system (magic
links, confirmation, sudo mode, hashed tokens), and Phoenix v1.8 **Scopes** thread an
authorization context through every generated controller, LiveView, and context function.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| AUTH-1 | `mix phx.gen.auth Accounts User users` scaffolds registration + email confirmation, magic-link login, opt-in password auth, and sudo mode (re-auth before sensitive actions), in LiveView or controller flavor. | CORE (generated) | phoenix/authn_authz/mix_phx_gen_auth.md |
| AUTH-2 | Generated code is application code: no framework upgrades apply to it; security fixes are flagged in CHANGELOG for manual porting — an explicit design tradeoff. | CORE | phoenix/authn_authz/mix_phx_gen_auth.md |
| AUTH-3 | Generated plugs: `fetch_current_scope_for_user`, `require_authenticated_user`, `redirect_if_user_is_authenticated`, `require_sudo_mode`; matching LiveView `on_mount` hooks (`mount_current_scope`, `ensure_authenticated`). | CORE (generated) | mix_phx_gen_auth.md, scopes.md |
| AUTH-4 | Password hashing via Comeonin interface: bcrypt (Unix default), pbkdf2 (Windows default), argon2 recommended — `--hashing-lib` switch. | CORE (generated) | mix_phx_gen_auth.md |
| AUTH-5 | All sessions/tokens stored hashed in a `users_tokens` table: per-device session tracking, global invalidation on password change. | CORE (generated) | mix_phx_gen_auth.md |
| AUTH-6 | Case-insensitive email lookup handled per database (Postgres `citext` extension, SQLite `COLLATE NOCASE`). | CORE (generated) | mix_phx_gen_auth.md |
| AUTH-7 | Scopes: a `%Scope{}` struct (current user, org, permissions, request metadata) assigned as `:current_scope` and passed as the first argument to every generated context function. | CORE | phoenix/authn_authz/scopes.md |
| AUTH-8 | Scope config (`config :my_app, :scopes`) drives generators: `access_path`, `schema_key`/`schema_type`/`schema_table` produce FK columns and `where: post.user_id == ^scope.user.id` filters automatically. | CORE | scopes.md |
| AUTH-9 | Multiple/augmented scopes: org membership added to the scope via plug + `on_mount` hook, `route_prefix: "/organizations/:org"` nests generated routes, `--scope` picks the scope per generator run. | CORE | scopes.md |
| AUTH-10 | Scoped PubSub: generated LiveViews subscribe to scope-filtered topics so real-time updates respect ownership. | CORE (generated) | scopes.md |
| AUTH-11 | API authentication recipe: extend the generated `UserToken` with an "api-token" context, hashed storage, 365-day validity query, and a `Bearer` header plug for the `:api` pipeline. | DIY | phoenix/authn_authz/api_authentication.md |
| AUTH-12 | `Phoenix.Token.sign/verify` — salted, expiring tokens for socket/channel authentication (`max_age:`), transport-agnostic via the socket `auth_token: true` option. | CORE | phoenix/real_time/channels.md |
| AUTH-13 | Stateful-session revocation: `live_socket_id` in the session lets `Endpoint.broadcast("users_socket:#{id}", "disconnect", %{})` kill every LiveView/channel of a logged-out user. | CORE | liveview/server/security-model.md |
| AUTH-14 | LiveView security model documented: auth in plugs *and* in `mount`/`on_mount` (both entry paths), authorization re-checked in `handle_event`, never trust params/payloads. | CORE | liveview/server/security-model.md |
| AUTH-15 | `conn.assigns.current_user` (not request params) as the authorization subject — broken-access-control guidance with a documented anti-pattern. | CORE | phoenix/security.md |
| AUTH-16 | Ueberauth named as the blessed third-party/OAuth complement or replacement. | ECO | phoenix/authn_authz/authn_authz.md |
| AUTH-17 | User-enumeration protection is explicitly **not** attempted by the generated code (documented tradeoff incl. timing-attack caveats). | — | mix_phx_gen_auth.md; gap by design |

## P8 — Security

**Problem.** Resist the standard web attack classes without every developer becoming a
security expert.
**Answer.** Safe-by-default primitives (escaped templates, parameterized queries, CSRF
plug, secure headers) plus an unusually frank official guide cataloguing the ways to
defeat them.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| SEC-1 | CSRF protection by default: `:protect_from_forgery` in the `:browser` pipeline (`Plug.CSRFProtection`), hidden `_csrf_token` in forms, token handed to the LiveSocket. | CORE | phoenix/security.md, routing.md |
| SEC-2 | Action-reuse CSRF documented: state-changing actions must never be reachable via GET (GET+POST on one action = vulnerability). | CORE | phoenix/security.md; guidance |
| SEC-3 | XSS: HEEx auto-escaping; `raw/1`, `html/2`-from-input, `put_resp_content_type` from user input, and unvalidated upload content types all documented as the bypass vectors. | CORE | phoenix/security.md |
| SEC-4 | `put_secure_browser_headers` sets modern defaults incl. `content-security-policy: base-uri 'self'; frame-ancestors 'self'` when unset. | CORE | phoenix/security.md, phoenix/CHANGELOG.md |
| SEC-5 | SQL injection: pin-operator parameterization, fragment interpolation is a compile error, raw-SQL guidance to always use placeholders. | CORE | phoenix/security.md |
| SEC-6 | RCE guidance: never eval user input; `Plug.Crypto.non_executable_binary_to_term/2` as the safe `binary_to_term` (the `[:safe]` option alone is insufficient). | CORE | phoenix/security.md |
| SEC-7 | SSRF guidance: base URLs are not security barriers (Req/Tesla examples); avoid user-controlled outbound URLs. | DIY | phoenix/security.md |
| SEC-8 | CORS: not in core; `CORSPlug` shown with correct origin allow-listing and the regex-wildcard anti-pattern. | ECO | phoenix/security.md |
| SEC-9 | Mass assignment: changeset `cast` whitelists; the `is_admin`-in-cast-list vulnerability worked as the canonical example. | CORE | phoenix/security.md |
| SEC-10 | TLS: `https:` endpoint config passed to `Plug.SSL`; `cipher_suite: :strong` (TLS 1.3 only) and `:compatible` (1.3+1.2) AEAD-only profiles per OWASP. | CORE | phoenix/howto/using_ssl.md, plug/https.md |
| SEC-11 | `force_ssl:` endpoint option: HTTP→HTTPS redirects, `rewrite_on: [:x_forwarded_proto]` proxy awareness, HSTS with localhost caveats; compile-time config. | CORE | phoenix/howto/using_ssl.md |
| SEC-12 | `mix phx.gen.cert` generates self-signed dev certificates. | CORE | phoenix/howto/using_ssl.md |
| SEC-13 | Session cookies signed (or encrypted) — LiveView reads identity from the signed session on mount. | CORE | liveview/introduction/welcome.md |
| SEC-14 | Upload/body-size limits in `Plug.Parsers` framed as DoS protection (slow-client connection exhaustion). | CORE | phoenix/howto/file_uploads.md |
| SEC-15 | Transport hardening in point releases: `max_channels_per_transport` (default 100) against channel-spawning memory exhaustion; longpoll batch limits. | CORE | phoenix/CHANGELOG.md |
| SEC-16 | Log hygiene: `password` and `token` params masked by default in Phoenix.Logger. | CORE | phoenix/CHANGELOG.md |
| SEC-17 | Pointers to EEF "Web App Security Best Practices for BEAM" and "Secure Coding and Deployment Hardening" as canonical deep dives. | — | phoenix/security.md |

## P9 — Background work: jobs, queues & scheduling

**Problem.** Run work outside the request cycle: queues, retries, scheduled tasks.
**Answer.** Phoenix ships **no job framework**. The BEAM's processes/OTP are the
low-level answer (spawn a process, supervise it); Oban is the blessed ecosystem answer.
This is the largest deliberate gap versus Rails/Laravel/Django.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| JOB-1 | No built-in job queue, worker pool, retry, or scheduling framework anywhere in the corpus. | — | gap; no guide exists |
| JOB-2 | Oban named in official docs (telemetry libraries list) as the instrumented ecosystem job system. | ECO | phoenix/telemetry.md |
| JOB-3 | OTP primitives as native background work: guides model long-running work as supervised GenServers added to the application supervision tree. | DIY | phoenix/telemetry.md (MyServer example), directory_structure.md |
| JOB-4 | Periodic execution exists only as `:telemetry_poller` periodic measurements (MFA invoked on an interval) — measurement-oriented, not a cron replacement. | CORE | phoenix/telemetry.md |
| JOB-5 | Router `forward "/jobs", BackgroundJob.Plug` example assumes a separate job system exposing its own web UI. | DIY | phoenix/routing.md |
| JOB-6 | One-off operational tasks in production via release `eval` / custom commands (`bin/my_app eval "MyApp.Release.migrate"`), with a documented minimal-boot pattern. | CORE | phoenix/deployment/releases.md |

## P10 — Real-time: websockets & server push

**Problem.** Bidirectional, low-latency communication with many clients — and ideally a
programming model where real-time UI doesn't require a client-side framework.
**Answer.** Three layers: **PubSub** (cluster-wide topic messaging with zero brokers),
**Channels** (topic-routed socket processes with a documented wire protocol and clients
in many languages), and **LiveView** (server-rendered reactive UI sending minimal diffs
over the channel transport). Presence adds CRDT-replicated "who's online".

| ID | Feature | Tier | Notes |
|---|---|---|---|
| LIVE-1 | `socket "/socket", UserSocket, websocket: true, longpoll: false` endpoint declaration; WebSocket and long-polling transports built in, per-socket configurable. | CORE | phoenix/real_time/channels.md |
| LIVE-2 | `UserSocket.connect/2..3` authenticates/identifies the connection and sets base assigns; `channel "room:*", RoomChannel` wildcard topic routing. | CORE | channels.md |
| LIVE-3 | `Phoenix.Channel` callbacks: `join/3` (authorize per topic), `handle_in/3` (client events), `handle_out/3`, `terminate/2`; one lightweight process per client per topic. | CORE | channels.md |
| LIVE-4 | Server push: `push/3`, synchronous `{:reply, {:ok, payload}, socket}`, `broadcast!/3` to all topic subscribers cluster-wide. | CORE | channels.md |
| LIVE-5 | `intercept ["event"]` + `handle_out/3` filters/customizes broadcasts per recipient (with a documented cost warning). | CORE | channels.md |
| LIVE-6 | Transport-agnostic token auth: `auth_token: true` socket option + `Phoenix.Token` verify in `connect/3` (preferred over cookies for long-lived connections). | CORE | channels.md |
| LIVE-7 | Reliability semantics documented: client auto-reconnect with exponential backoff and topic rejoin; client-side `PushBuffer` outbox; server delivery is at-most-once — catch-up (`last_seen_id`) is your code. | CORE | channels.md |
| LIVE-8 | Official JS client (`phoenix` package) plus third-party Swift/Java/Kotlin/C#/Elixir/GDScript clients; the V2 JSON wire protocol (`[join_ref, ref, topic, event, payload]`, `phx_join`/`phx_leave`/`heartbeat`) is documented for writing new clients. | CORE | channels.md, phoenix/howto/writing_a_channels_client.md |
| LIVE-9 | `mix phx.gen.socket` and `mix phx.gen.channel` generate socket/channel server + JS client + tests. | CORE | channels.md, testing_channels.md |
| LIVE-10 | `Phoenix.PubSub`: subscribe/broadcast on topics across all cluster nodes via PG2 (native distribution) by default; Redis adapter when nodes can't cluster; `local_broadcast` for node-local fanout. | CORE | channels.md, real_time/presence.md |
| LIVE-11 | `Phoenix.Presence`: `track/3` + `list/1` replicate per-topic process metadata across the cluster — CRDT-based, no single source of truth, no external dependency, self-healing. | CORE | presence.md |
| LIVE-12 | Presence customization callbacks: `init/1`, `fetch/2` (shape/enrich presence data, e.g. DB lookups), `handle_metas/4` (react to joins/leaves, e.g. re-broadcast to LiveViews); `mix phx.gen.presence` generator. | CORE | presence.md |
| LIVE-13 | JS `Presence` class syncs `presence_state`/`presence_diff` events with an `onSync` callback and `list` formatter (multi-device counts). | CORE | presence.md |
| LIVE-14 | LiveView model: process-per-view; first render is plain HTTP (SEO + fast first paint), then the client connects over `/live` socket and `mount/3` re-runs stateful. | CORE | phoenix/live_view.md, liveview/introduction/welcome.md |
| LIVE-15 | Lifecycle callbacks: `mount/3` (params, session, socket), `handle_params/3` (URL changes, pre-render), `handle_event/3` (client events), `handle_info/2` (any process message — PubSub-driven UI), `render/1`. | CORE | welcome.md, live-navigation.md |
| LIVE-16 | Socket assigns API with change tracking: `assign/2,3`, `update/3`, `assign_new/3` (dedupes work across parent/child mounts); templates read `@name`. | CORE | assigns-eex.md |
| LIVE-17 | Diffs over the wire: statics sent once; only changed dynamics re-executed and resent, including map/struct-field granularity (`@user.name` vs `@user.id`) and across function-component boundaries. | CORE | assigns-eex.md, liveview/README.md |
| LIVE-18 | Comprehension diffing with optional `:key` for identity-based tracking; documented pitfalls (no template-local variables, never `Map.put` on assigns, don't pass whole `assigns`). | CORE | assigns-eex.md |
| LIVE-19 | Streams: `stream/3,4` (`at:`, `limit:`, `reset:`), `stream_insert/3`, `stream_delete/3` + `phx-update="stream"` manage large collections without keeping them in server memory. | CORE | presence.md, bindings.md |
| LIVE-20 | Bidirectional infinite scroll: `phx-viewport-top`/`phx-viewport-bottom` + streams = virtualized lists; `_overran` param for scrollbar jumps. | CORE | bindings.md |
| LIVE-21 | `Phoenix.LiveComponent`: stateful in-process components with own `mount`/`update`/`update_many`/`handle_event`, addressed via `phx-target={@myself}`, rendered by `live_component/1`. | CORE | welcome.md |
| LIVE-22 | Nested LiveViews via `live_render/3`: separate process, error isolation, required stable `:id`; documented as the heavyweight isolation option. | CORE | welcome.md |
| LIVE-23 | Event bindings: `phx-click`/`phx-click-away`, `phx-blur`/`phx-focus` (+window variants), `phx-keydown`/`phx-keyup` (+`phx-key` filter, window variants), with `phx-value-*` params and client-configurable event `metadata`. | CORE | bindings.md |
| LIVE-24 | Client-side rate limiting: `phx-debounce` (ms or `"blur"`) and `phx-throttle`, with special form/keydown reset semantics. | CORE | bindings.md |
| LIVE-25 | `Phoenix.LiveView.JS` commands: declarative client-side ops (`show`/`hide`/`toggle`, `add_class`/`remove_class`/`toggle_class`, `transition`, attribute ops, `dispatch`, `push` with target/loading options, `navigate`/`patch`) that compose with pipes and survive DOM patches; JSON-encodable for `push_event`. | CORE | bindings.md, js-interop.md, liveview/CHANGELOG.md |
| LIVE-26 | DOM patching controls: `phx-update` (`replace`, `stream`, `ignore` for JS-library-owned subtrees with data-attr passthrough), `phx-mounted`, `phx-remove`. | CORE | bindings.md |
| LIVE-27 | Connection-state bindings `phx-connected`/`phx-disconnected`, and container CSS classes `phx-connected`/`phx-loading`/`phx-error`. | CORE | bindings.md, syncing-changes.md |
| LIVE-28 | Client hooks (`phx-hook`): `mounted`/`beforeUpdate`/`updated`/`destroyed`/`disconnected`/`reconnected` lifecycle with `pushEvent`(+reply)/`pushEventTo`/`handleEvent`/`upload`/`js()` APIs; hooks also run on non-LiveView pages (mounted only). | CORE | js-interop.md |
| LIVE-29 | Colocated assets: `ColocatedHook` and `ColocatedJS` extract `<script>` from HEEx at compile time into an importable bundle; v1.2 adds ColocatedCSS behaviour. | CORE | js-interop.md, liveview/CHANGELOG.md |
| LIVE-30 | Server→client events: `push_event/3` dispatches `phx:*` window events or hook `handleEvent` callbacks (charts, highlights); namespacing pattern for component-scoped events. | CORE | js-interop.md |
| LIVE-31 | Forms over the socket: `<.form>` + `to_form`, `phx-change` validation + `phx-submit` save against changesets; per-input `phx-change` targeting; `_target` param identifies the changed field. | CORE | form-bindings.md |
| LIVE-32 | Form sync guarantees: client is source of truth for focused inputs; in-flight event counting prevents stale-update rollbacks; submit locks inputs readonly + disables buttons until acknowledged. | CORE | form-bindings.md, syncing-changes.md |
| LIVE-33 | Automatic form recovery after crash/reconnect (re-triggers `phx-change`); `phx-auto-recover` custom recovery event for stateful wizards; `"ignore"` opt-out. | CORE | form-bindings.md, deployments.md |
| LIVE-34 | `phx-trigger-action` hands a validated LiveView form off to a plain HTTP controller POST (for session mutation like login). | CORE | form-bindings.md |
| LIVE-35 | Loading/optimistic UI: per-event `-loading` CSS classes, `phx-disable-with` button text swap, `JS.push(loading: selector)` custom loading targets, Tailwind variant recipes. | CORE | syncing-changes.md |
| LIVE-36 | Uploads: `allow_upload/3` (accept types, max_entries, max_file_size, `auto_upload`), `<.live_file_input>`, `@uploads` reactive entries with progress, drag-and-drop `phx-drop-target` (+active class), `cancel_upload`, `upload_errors`, `consume_uploaded_entries/3`; size limits enforced server-side per chunk. | CORE | liveview/server/uploads.md |
| LIVE-37 | External (direct-to-cloud) uploads: `external: &presign/2` returns client metadata; JS `uploaders` namespace (S3 multipart-POST, signed-PUT for R2, chunked UpChunk) reports progress/errors back to entries. | CORE | liveview/client/external-uploads.md |
| LIVE-38 | Live navigation: `<.link patch>`/`push_patch` (same LiveView, `handle_params`, scroll kept) vs `<.link navigate>`/`push_navigate` (new LiveView, same session/layout) vs `href`/`redirect` (full reload); graceful fallback to full reloads; `replace` history option. | CORE | live-navigation.md |
| LIVE-39 | Navigation JS events: `phx:page-loading-start/stop` (with kind metadata) for topbar-style indicators; low-level `phx:navigate` on URL changes. | CORE | syncing-changes.md |
| LIVE-40 | Live `<title>` updates via the special `@page_title` assign + `live_title` component (prefix/suffix/default) — the one root-layout element LiveView can touch. | CORE | live-layouts.md |
| LIVE-41 | Error semantics per phase: HTTP-mount errors → normal error pages; connected-mount errors → page reload; post-mount crashes → process restart and live remount, often self-healing stale UI. | CORE | error-handling.md |
| LIVE-42 | Deploy/recovery posture: reconnect with backoff; guidance to keep state in URL params, DB, and forms so rolling deploys lose nothing. | CORE | deployments.md |
| LIVE-43 | `phx-track-static` + `static_changed?/1` detect stale cached assets after deploys. | CORE | html-attrs.cheatmd |
| LIVE-44 | Client debugging: `liveSocket.enableDebug()`, and a built-in latency simulator (`enableLatencySim(ms)`) persisted per browser session. | CORE | js-interop.md |
| LIVE-45 | `lv:clear-flash` built-in client event; flash works uniformly in LiveView via `put_flash`. | CORE | bindings.md, error-handling.md |
| LIVE-46 | LiveDebugger: process/assign/lifecycle inspection tool for LiveView apps. | ECO | liveview/README.md |
| LIVE-47 | Longpoll fallback (`longPollFallbackMs`) with documented clustering requirements: distributed Erlang, Redis PubSub, or sticky sessions. | CORE | phoenix/deployment/deployment.md |

## P11 — Mail & notifications

**Problem.** Compose and deliver transactional email (and other user notifications).
**Answer.** Swoosh is wired into every new app (`Hello.Mailer`) with a dev mailbox
preview; everything beyond email is out of scope.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MAIL-1 | Swoosh mailer generated by default: `lib/hello/mailer.ex` (`use Swoosh.Mailer, otp_app: :hello`) — the app's email delivery interface; glossary describes Swoosh as composing, delivering, and testing email. | CORE | phoenix/directory_structure.md, introduction/packages_glossary.md |
| MAIL-2 | Dev mailbox: emails viewable at `/dev/mailbox` during development (used by the auth confirmation flow). | CORE | phoenix/authn_authz/mix_phx_gen_auth.md |
| MAIL-3 | `mix phx.gen.auth` generates notifier modules for confirmation/magic-link/reset emails; they log to terminal until you wire a real delivery adapter — production adapters are your integration work. | CORE (generated) | mix_phx_gen_auth.md |
| MAIL-4 | `--no-mailer` flag excludes Swoosh for API-only apps. | CORE | phoenix/json_and_apis.md |
| MAIL-5 | No SMS/push/webhook/notification-channel framework; the guides only note "it is your responsibility to integrate with the proper system". | — | gap; mix_phx_gen_auth.md |

## P12 — Caching & performance

**Problem.** Avoid recomputation and re-transfer: caches, precompilation, efficient
serving.
**Answer.** No cache framework. Phoenix's performance story is structural — precompiled
templates, compile-time routing, LiveView diffs, BEAM concurrency — with ETS as the
in-memory store when you need one.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CACHE-1 | No framework cache store, fragment caching, or HTTP caching layer documented anywhere in the corpus. | — | gap |
| CACHE-2 | Templates precompiled to functions ("pre-compiled templates for blazing speed"); routes compiled to a single VM-optimized match function. | CORE | phoenix/introduction/overview.md, routing.md |
| CACHE-3 | LiveView diff engine minimizes wire traffic (statics once, changed dynamics only) — claimed 5–10x faster client updates vs HTML-fragment replacement. | CORE | liveview/README.md |
| CACHE-4 | Static assets served with digests + cache manifest (`mix phx.digest`) for far-future caching; default responses carry `cache-control: max-age=0, private, must-revalidate`. | CORE | deployment.md, json_and_apis.md |
| CACHE-5 | Ecto queries are pre-compiled; a `:query_cache` option can selectively bypass the query cache. | CORE | ecto/howtos/Dynamic queries.md, ecto/CHANGELOG.md |
| CACHE-6 | ETS: in-memory Erlang term storage available to every app (used by telemetry internally; etso exposes it as an Ecto adapter); DETS for disk — noted as the BEAM-native key-value answer. | CORE (runtime) | phoenix/howto/swapping_databases.md, telemetry.md |

## P13 — Files & storage

**Problem.** Accept uploads, store files, and serve them back.
**Answer.** `Plug.Upload` handles multipart plumbing into temp files; LiveView adds
reactive/direct-to-cloud uploads; durable storage is explicitly left to you.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| FILE-1 | `Plug.Upload` struct (content_type, filename, temp path) appears automatically in params for multipart forms; temp files auto-deleted when the request ends; `give_away/3` transfers ownership. | CORE | phoenix/howto/file_uploads.md |
| FILE-2 | Multipart forms via the `multipart` attr on `<.form>` + `type="file"` input; absent file input means no param key at all. | CORE | file_uploads.md |
| FILE-3 | Upload size/rate limits via `Plug.Parsers` `:length`/`:read_length`/`:read_timeout`. | CORE | file_uploads.md |
| FILE-4 | Serving stored files: additional `Plug.Static` mounts (`at: "/uploads", from: "/media"`) or `Plug.Conn.send_file/5`; LiveView uploads served by adding the dir to `static_paths/0`. | CORE | file_uploads.md, liveview/server/uploads.md |
| FILE-5 | LiveView reactive uploads with progress/preview/cancel (see LIVE-36) and direct-to-cloud presigned flows (see LIVE-37). | CORE | liveview upload guides |
| FILE-6 | Local-disk storage caveats documented: multi-instance deployments break it; store in DB or object storage instead. | — | liveview/server/uploads.md |
| FILE-7 | No storage abstraction layer (no ActiveStorage/Flysystem equivalent): unique naming, extension validation, S3 clients are "use a library" advice; ExAws/ExAws.S3 referenced for presigning. | — | file_uploads.md, external-uploads.md; gap |

## P14 — Building APIs: serialization & content negotiation

**Problem.** Serve machine clients: negotiate formats, serialize data, and shape errors
consistently.
**Answer.** JSON views are plain functions returning maps (no serializer DSL), the
`:accepts` plug negotiates formats per pipeline, and `action_fallback` +
`ChangesetJSON` standardize error responses.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| API-1 | `mix phx.gen.json Urls Url urls link:string` scaffolds a complete CRUD JSON API: controller, JSON view, changeset error view, fallback controller, context, migration, tests. | CORE | phoenix/json_and_apis.md |
| API-2 | JSON rendering = functions returning maps (`UrlJSON.index/show/data`), encoded by Jason (`Phoenix.json_library()`); no serializer framework. | CORE | json_and_apis.md |
| API-3 | Content negotiation: `plug :accepts, ["html", "json"]` per pipeline; `_format` query param switches formats on the fly; `MIME` library backs content types. | CORE | phoenix/controllers.md |
| API-4 | Per-format view resolution: `use Phoenix.Controller, formats: [:html, :json]` (option now required) or explicit `plug :put_view, html: ..., json: ...`; `put_format` for others. | CORE | controllers.md, phoenix/CHANGELOG.md |
| API-5 | `action_fallback` + generated `FallbackController` map `{:error, %Ecto.Changeset{}}` → 422 with `ChangesetJSON` (`traverse_errors` output) and `{:error, :not_found}` → 404. | CORE | json_and_apis.md |
| API-6 | REST conventions for APIs: `resources ..., except: [:new, :edit]`, 201 + `location` header on create, 204 no-content on delete. | CORE | json_and_apis.md, testing_controllers.md |
| API-7 | `ErrorJSON` renders exception statuses as `%{errors: %{detail: ...}}` for API consumers. | CORE | custom_error_pages.md |
| API-8 | API-only app generation: `mix phx.new --no-html --no-assets --no-gettext --no-mailer --no-dashboard --no-ecto` combinations documented. | CORE | json_and_apis.md |
| API-9 | Bearer-token API authentication recipe on top of gen.auth (see AUTH-11). | DIY | api_authentication.md |
| API-10 | HTML and API can coexist in one app via separate pipelines/scopes; versioning via nested scopes only. | CORE | json_and_apis.md, routing.md |
| API-11 | Changeset JSON encoding: changesets encode their errors as JSON objects directly. | CORE | json_and_apis.md |
| API-12 | GraphQL: not in core; Absinthe named in the official telemetry libraries list. | ECO | phoenix/telemetry.md |
| API-13 | No rate limiting, API pagination, or OpenAPI tooling in the corpus. | — | gap |

## P15 — Internationalization & localization

**Problem.** Serve users in multiple languages, including validation errors.
**Answer.** Gettext ships in every app; locale *selection* is your plumbing, documented
as plugs (HTTP) and `on_mount` hooks (LiveView).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| I18N-1 | Gettext generated by default (`lib/hello_web/gettext.ex`), backed by GNU gettext PO workflows; `--no-gettext` opt-out. | CORE | phoenix/directory_structure.md, packages_glossary.md |
| I18N-2 | Locale-from-params plug pattern: validate `?locale=` against a whitelist, `assign(conn, :locale, ...)`, combine with Gettext — the canonical module-plug example. | DIY | phoenix/plug.md |
| I18N-3 | LiveView locale restoration: `Gettext.put_locale/2` in `mount` from URL param, session, or DB; reusable `on_mount RestoreLocale` hook; navigation-type caveats per storage choice. | DIY | liveview/server/gettext.md |
| I18N-4 | Changeset error translation: generated `translate_error/1` interpolates `%{count}`-style bindings and routes messages through Gettext (used by CoreComponents and ChangesetJSON). | CORE (generated) | json_and_apis.md, form-bindings.md |
| I18N-5 | No automatic locale negotiation (Accept-Language), locale-aware routing, or date/number localization framework in the corpus. | — | gap |

## P16 — Testing support

**Problem.** Make fast, isolated, full-stack tests the default, including for databases
and stateful connections.
**Answer.** ExUnit plus generated case templates — `ConnCase`, `DataCase`,
`ChannelCase` — with the SQL Sandbox giving every test its own rolled-back transaction,
enabling concurrent DB tests; every generator ships tests and fixtures.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| TEST-1 | ExUnit is the test framework; new apps come with a passing suite, `test_helper.exs`, and support files. | CORE | phoenix/testing/testing.md |
| TEST-2 | `ConnCase`: case template wiring `@endpoint`, verified `~p` routes, `Plug.Conn` + `Phoenix.ConnTest` imports, and a fresh `build_conn()` per test. | CORE | testing.md |
| TEST-3 | Request helpers and assertions: `get`/`post`/`put`/`delete`, `html_response/2`, `json_response/2`, `response/2`, `redirected_to/1`, `redirected_params/1`. | CORE | testing_controllers.md |
| TEST-4 | `assert_error_sent 404, fn -> ... end` verifies exception→HTTP-status translation as the browser would see it. | CORE | testing_controllers.md |
| TEST-5 | `DataCase` + `Ecto.Adapters.SQL.Sandbox`: per-test transactions rolled back automatically; `async: true` concurrent DB tests on PostgreSQL; shared mode for non-async. | CORE | testing_contexts.md, ecto/testing/Testing with Ecto.md |
| TEST-6 | `errors_on/1` helper flattens changeset errors for validation assertions; guidance splits schema tests (pure) from context tests (side effects) from controller tests (integration broad strokes). | CORE | testing_contexts.md |
| TEST-7 | `ChannelCase` + `Phoenix.ChannelTest`: `socket/3`, `subscribe_and_join/3`, `push/3`, `assert_reply/3`, `assert_broadcast/3`, `assert_push/3`, `broadcast_from!/3`. | CORE | testing_channels.md |
| TEST-8 | `Phoenix.LiveViewTest`: element selection + `render_hook/3` shown for viewport events; built-in DOM checks during tests (duplicate ID detection, missing-form-id warnings) with per-check `:test_warnings` config (`:warn`/`:raise`/`:ignore`). | CORE | bindings.md, liveview/CHANGELOG.md |
| TEST-9 | Every generator emits tests + `test/support/fixtures/*_fixtures.ex` fixture modules; scope-aware generators add `test_data_fixture` and `test_setup_helper` (e.g. `register_and_log_in_user`). | CORE | testing.md, scopes.md |
| TEST-10 | Test selection: per-directory/file/line runs, `@tag`/`@moduletag` with `--only`/`--exclude`/`--include`, default exclusions in `test_helper.exs`. | CORE | testing.md |
| TEST-11 | Deterministic randomized ordering with `--seed`; CI partitioning via `MIX_TEST_PARTITION` + `--partitions N` (auto per-partition databases). | CORE | testing.md |
| TEST-12 | `Plug.Test`: `conn(:get, "/hello")` + direct `MyPlug.call/2` for testing any plug without a server. | CORE | plug/README.md |
| TEST-13 | View tests via `Phoenix.Template.render_to_string/4` (error view examples). | CORE | testing.md |
| TEST-14 | Hand-rolled test factories on plain functions + `Repo.insert!` documented as sufficient — no factory library needed. | DIY | ecto/howtos/Test factories.md |
| TEST-15 | `test` mix alias chains `ecto.create --quiet, ecto.migrate, test` for zero-setup DB tests. | CORE | ecto/testing/Testing with Ecto.md |
| TEST-16 | Dynamic-query unit testing pattern: assert on `inspect(dynamic)` output. | DIY | ecto/howtos/Dynamic queries.md |

## P17 — CLI, code generation & developer experience

**Problem.** Get from zero to a working app fast, and keep scaffolding repetitive layers
without hiding them.
**Answer.** `mix phx.new` plus a family of `phx.gen.*` generators that write code *into
your app* (contexts, schemas, LiveViews, auth) as an explicitly-stated learning tool and
starting point — plus hot code reloading and an IEx-first workflow.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CLI-1 | `mix phx.new` app generator (installed via `mix archive.install hex phx_new`); Phoenix Express one-liner (curl-piped installer from new.phoenixframework.org) installs Erlang/Elixir/Phoenix and picks a database automatically. | CORE | phoenix/introduction/up_and_running.md |
| CLI-2 | `phx.new` flags compose the stack: `--database pg/mysql/mssql/sqlite3`, `--no-ecto`, `--no-html`, `--no-assets`/`--no-esbuild`/`--no-tailwind`, `--no-live`, `--no-dashboard`, `--no-gettext`, `--no-mailer`, `--interactive`. | CORE | json_and_apis.md, up_and_running.md, CHANGELOG |
| CLI-3 | `mix phx.server` runs the app; `iex -S mix phx.server` runs it inside the REPL; `recompile()` in IEx. | CORE | up_and_running.md, ecto.md |
| CLI-4 | Hot code reloading + browser live reload in dev (CSS updates without refresh; inotify-based watching). | CORE | request_lifecycle.md, installation.md |
| CLI-5 | `mix phx.gen.html` / `phx.gen.json` / `phx.gen.live`: full vertical slices — context, schema, migration, web layer, tests, fixtures; context name optional (defaults to plural resource). | CORE | data_modelling guides |
| CLI-6 | `mix phx.gen.context` (no web files) and `mix phx.gen.schema` (schema+migration only); generating into an existing context *injects* functions and warns with a function count. | CORE | in_context_relationships.md, ecto.md |
| CLI-7 | Generator field syntax: `title:string`, `body:text`, `price:decimal`, `title:string:unique`, `post_id:references:posts`; `--no-scope`/`--scope` control scope wiring. | CORE | data_modelling guides |
| CLI-8 | `mix phx.gen.auth` (see P7) — the largest generator, with LiveView/controller variants and hashing-lib/table-name/binary-id options. | CORE | mix_phx_gen_auth.md |
| CLI-9 | `mix phx.gen.channel`, `phx.gen.socket`, `phx.gen.presence` for real-time scaffolding. | CORE | channels.md, presence.md, testing_channels.md |
| CLI-10 | `mix phx.gen.secret` (secret generation), `mix phx.gen.cert` (dev TLS), `mix phx.gen.release [--docker]` (release scripts + Dockerfile). | CORE | deployment guides |
| CLI-11 | `mix phx.routes` route table; `mix help phx.new` self-documenting tasks. | CORE | routing.md |
| CLI-12 | Ecto tasks: `ecto.create`, `ecto.drop`, `ecto.gen.migration`, `ecto.migrate`, `ecto.rollback`, `ecto.gen.repo` — with friendly permission-error diagnostics. | CORE | phoenix/ecto.md, ecto/introduction/Getting Started.md |
| CLI-13 | Mix aliases as workflow glue: `mix setup`, `assets.build`, `assets.deploy`, `ecto.setup`, custom `test` alias — all user-editable in `mix.exs`. | CORE | asset_management.md, Testing with Ecto.md |
| CLI-14 | Conventional directory layout: `lib/app` (domain) vs `lib/app_web` (interface), `priv/` for runtime resources, mirrored `test/` — documented as an architectural statement. | CORE | directory_structure.md |
| CLI-15 | Generators positioned as learning tools/starting points; docs repeatedly instruct renaming/refactoring generated code rather than treating it as framework-owned. | CORE | your_first_context.md, faq.md |
| CLI-16 | `.iex.exs` project REPL config (e.g. aliasing Scope for console ergonomics). | CORE | scopes.md |
| CLI-17 | Release console: `bin/my_app remote` attaches a live IEx to production; `eval` runs one-off expressions. | CORE | deployment/releases.md |
| CLI-18 | Dev error pages with actionable buttons (run migrations, run seeds) via `Plug.Exception` actions; `debug_errors` toggle. | CORE | custom_error_pages.md |

## P18 — Configuration, environments & deployment

**Problem.** Configure per environment, keep secrets out of the build, and ship a
runnable artifact.
**Answer.** Layered compile-time config (`config.exs` + per-env files) with runtime
config (`runtime.exs`) reading env vars at boot; `mix release` produces a
self-contained artifact (VM included) with generated server/migrate entry points and
first-class Docker and clustering support.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CONF-1 | Config layering: `config/config.exs` → `dev/test/prod.exs` (compile time) → `config/runtime.exs` (boot time, the sanctioned home for secrets/env vars). | CORE | directory_structure.md, deployment.md |
| CONF-2 | Secrets convention: `SECRET_KEY_BASE` (via `mix phx.gen.secret`) signs/encrypts framework data; `DATABASE_URL` (`ecto://user:pass@host/db`) configures the repo. | CORE | deployment.md |
| CONF-3 | Endpoint config: `http`/`https` listeners, `url: [host:, port:]` driving `url(~p)` generation, `cache_static_manifest`, `debug_errors`, `code_reloader`, `watchers`. | CORE | using_ssl.md, custom_error_pages.md, asset_management.md |
| CONF-4 | `MIX_ENV` environments; test DB names parameterized by `MIX_TEST_PARTITION`. | CORE | ecto.md, testing.md |
| CONF-5 | Bare-metal prod flow documented: `deps.get --only prod`, `MIX_ENV=prod compile`, `assets.deploy`, `ecto.migrate`, `PORT=... mix phx.server` (incl. detached mode). | CORE | deployment.md |
| CONF-6 | `mix release`: self-contained directory with ERTS + code + deps; `phx.gen.release` adds `bin/server` + `bin/migrate` overlays and `Release.ex`; commands: start/stop/remote/eval. | CORE | releases.md |
| CONF-7 | `mix phx.gen.release --docker`: production multi-stage Dockerfile (hexpm builder image, slim runner, non-root, runtime.exs-driven config). | CORE | releases.md |
| CONF-8 | Runtime-config-first container guidance: secrets provided at container start, libraries never read env vars themselves. | CORE | releases.md |
| CONF-9 | Clustering: `dns_cluster` DNS-based node discovery in default apps (`DNS_CLUSTER_QUERY`); `rel/env.sh.eex` for distribution ports/names; epmd-less fixed-port setup documented. | CORE | releases.md |
| CONF-10 | libcluster / libcluster_postgres named for gossip/k8s/EC2/Postgres node discovery. | ECO | releases.md |
| CONF-11 | Platform guides: Fly.io (`fly launch` auto-detects Phoenix and runs gen.release --docker), Gigalixir, Heroku (with a frank limitations list), plus Render/Railway community guides. | CORE docs | deployment/*.md |
| CONF-12 | Long-poll + multi-node caveat: requires clustering, Redis PubSub, or sticky sessions — documented decision matrix. | CORE | deployment.md |
| CONF-13 | Repo config surface: pool_size, credentials, `socket_options: [:inet6]`, `parameters: [search_path: ...]`, custom `types:`. | CORE | ecto.md, ecto/README.md, multi-tenancy guides |
| CONF-14 | Application supervision tree as boot config: `Telemetry`, `Repo`, `{Phoenix.PubSub, name: ...}`, `Endpoint` children started in order, stopped in reverse. | CORE | directory_structure.md |
| CONF-15 | `force_ssl` is compile-time (prod.exs) — the compile-vs-runtime config distinction is called out explicitly. | CORE | using_ssl.md |

## P19 — Extensibility: DI, events, hooks & packages

**Problem.** Let applications and libraries extend the framework without forking it.
**Answer.** No DI container and no global event bus for app logic. Extension happens
through *contracts*: the Plug behaviour, Elixir protocols (`Phoenix.Param`,
`Phoenix.HTML.FormData`, `Plug.Exception`), Ecto adapter/repo callbacks, LiveView
hooks, and telemetry events — all wired explicitly.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| EXT-1 | Plug is the universal middleware contract — any package exposing `init/1`+`call/2` slots into endpoint, pipeline, or controller unchanged. | CORE | phoenix/plug.md |
| EXT-2 | Protocols as extension points: `Phoenix.Param` (URL params from any struct), `Phoenix.HTML.FormData` (forms over any data source — `phoenix_ecto` implements it for changesets), `Plug.Exception` (status/actions for any error). | CORE | data_modelling/faq.md, custom_error_pages.md |
| EXT-3 | Ecto extension points: adapter behaviour (six adapters listed), repo callbacks `prepare_query/3`, `default_options/1`, `prepare_transaction/2`, custom Postgrex type modules. | CORE | ecto/README.md, Multi tenancy with foreign keys.md, CHANGELOG |
| EXT-4 | LiveView lifecycle hooks: `on_mount/1` (per-LiveView, per-`live_session`, or app-wide via the `live_view` macro in `MyAppWeb`) and `attach_hook/4` for shared event handling across LiveViews. | CORE | security-model.md, welcome.md |
| EXT-5 | `MyAppWeb` macro module (`use HelloWeb, :controller / :html / :live_view / :router / :verified_routes`) centralizes app-wide imports/behaviour — the app's own extension seam. | CORE | request_lifecycle.md, controllers.md |
| EXT-6 | Telemetry events as the universal instrumentation hook — libraries (Absinthe, Ash, Broadway, Oban, Tesla…) all publish events into the same system. | CORE | phoenix/telemetry.md |
| EXT-7 | Behaviours for asset tooling: `ColocatedCSS`, `ColocatedJS`/`ColocatedHook`, `HTMLFormatter.TagFormatter`. | CORE | liveview/CHANGELOG.md, js-interop.md |
| EXT-8 | Contexts as the sanctioned internal-API boundary: public context module, private schemas/queries behind it; internal organization deliberately unstandardized. | CORE (pattern) | data_modelling/contexts.md, faq.md |
| EXT-9 | Scope config system extends *the generators themselves* — apps declare their own authorization shapes and future scaffolding conforms. | CORE | scopes.md |
| EXT-10 | Package management via Hex/`mix deps` incl. git deps with `sparse:`/`app: false` options (used for icon sets). | CORE | asset_management.md |
| EXT-11 | Pipelines + `forward` let whole third-party plug apps mount inside a Phoenix router. | CORE | routing.md |
| EXT-12 | No DI container: composition is function arguments, module attributes, and OTP app config; libraries are told *not* to read application env directly. | — | releases.md; design stance |
| EXT-13 | No framework event/listener bus for domain events; `Phoenix.PubSub` + `handle_info` is the idiom (used by generated scoped broadcasts). | CORE (idiom) | scopes.md, channels.md |

## P20 — Observability: logging, metrics, errors

**Problem.** See what the running system is doing: request logs, metrics, traces of
failures.
**Answer.** `:telemetry` events everywhere (Phoenix, Plug, Ecto, LiveView all emit
them), `Telemetry.Metrics` to declare aggregations, pluggable reporters — with
LiveDashboard as the built-in real-time visualizer.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| OBS-1 | `:telemetry` core: named events with measurements + metadata, handlers stored in ETS, emitted at key lifecycle moments across the whole stack. | CORE | phoenix/telemetry.md |
| OBS-2 | Generated Telemetry supervisor (`MyAppWeb.Telemetry`) with a `metrics/0` declaration list and `:telemetry_poller` for periodic VM measurements (memory, run queues). | CORE | telemetry.md |
| OBS-3 | `Telemetry.Metrics` five metric types (counter, summary, distribution, last_value, …) with `:tags` and `:tag_values` transformations (e.g. group request duration by route+method from `conn`). | CORE | telemetry.md |
| OBS-4 | Reporters are pluggable: `ConsoleReporter` bundled; StatsD, Prometheus, etc. via hex ("telemetry_metrics_*" packages). | CORE/ECO | telemetry.md |
| OBS-5 | Phoenix HTTP events: `[:phoenix, :endpoint, :stop]` (Plug.Telemetry) and `[:phoenix, :router_dispatch, :stop]` with route/plug metadata; full catalog in `Phoenix.Logger`. | CORE | telemetry.md |
| OBS-6 | Ecto events: `[:ecto, :repo, :init]` plus adapter query events with query/queue/decode timings (`my_app.repo.query.*`). | CORE | telemetry.md |
| OBS-7 | LiveView/LiveComponent telemetry: start/stop/exception spans for mount, handle_params, handle_event, render, component update/handle_event, plus `:destroyed`. | CORE | liveview/server/telemetry.md |
| OBS-8 | Custom instrumentation: `:telemetry.execute/3` + documented span pattern (start/stop/exception) and periodic custom measurements via the poller. | CORE | telemetry.md |
| OBS-9 | Request logging with `Plug.Logger`/Phoenix logger incl. processing module, params (with password/token filtering), pipelines, timing; Ecto debug query logs in dev. | CORE | plug/README.md, data_modelling guides |
| OBS-10 | `Plug.RequestId` request correlation IDs surfaced in the `x-request-id` header. | CORE | phoenix/plug.md, json_and_apis.md |
| OBS-11 | Phoenix.LiveDashboard: real-time Telemetry.Metrics charts and performance/debugging pages, shipped in default apps; its RequestLogger plug streams request logs into the dashboard. | CORE | telemetry.md, plug.md |
| OBS-12 | Dev error experience: `Plug.Debugger`-style rich pages with actionable buttons; production error rendering via ErrorHTML/ErrorJSON. | CORE | custom_error_pages.md |
| OBS-13 | No built-in error-tracker/APM service integration (Sentry-style) in the corpus; telemetry is the integration surface. | — | gap |

## P21 — Admin & operational UIs

**Problem.** Operate and inspect the running app through a UI.
**Answer.** LiveDashboard for runtime introspection; no data-admin framework — the
generators scaffold your own admin CRUD instead.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ADMIN-1 | Phoenix.LiveDashboard in every default app: real-time performance monitoring/debugging (metrics charts, request logger) mounted in dev; removable via `--no-dashboard`. | CORE | packages_glossary.md, telemetry.md, plug.md |
| ADMIN-2 | Dev mailbox UI at `/dev/mailbox` for previewing Swoosh emails. | CORE | mix_phx_gen_auth.md |
| ADMIN-3 | Erlang `:observer` for live-system process inspection (noted in deployment limitations discussion). | CORE (runtime) | deployment/heroku.md |
| ADMIN-4 | No admin CRUD framework (no Django-admin equivalent): admin areas are ordinary scoped routes + generated resources under `/admin` namespaces. | — | routing.md; gap by design |

## P22 — BEAM runtime: processes, supervision & distribution

**Problem.** Concurrency, fault tolerance, and multi-node operation usually require
external infrastructure (process managers, message brokers, sticky routers).
**Answer.** Phoenix inherits them from the Erlang VM: every request/channel/LiveView is
an isolated lightweight process under a supervision tree, and nodes cluster natively —
so PubSub, Presence, and "restart on crash" need zero external services.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| BEAM-1 | Application supervision tree (`application.ex`): Telemetry, Repo, PubSub, Endpoint (and Presence, custom GenServers) as supervised children with restart strategies. | CORE | directory_structure.md, presence.md |
| BEAM-2 | Process-per-unit isolation: each channel client/topic and each LiveView is its own lightweight process with its own state; crashes are contained and logged, clients recover automatically. | CORE | channels.md, error-handling.md |
| BEAM-3 | Native distribution: connected nodes pass messages seamlessly; PubSub broadcasts cross nodes at ~one message per node cost; the same code scales by adding nodes ("2 million websocket connections" reference). | CORE | channels.md, releases.md |
| BEAM-4 | Presence's CRDT replication and Channels' cluster broadcasts require no Redis/queue/database — repeatedly called out as a differentiator. | CORE | presence.md, liveview/README.md |
| BEAM-5 | GenServer/Task/ETS as in-app state and concurrency primitives (guides use GenServers for instrumentation, ETS for storage; Heroku guide warns their state dies on dyno restarts). | CORE | telemetry.md, heroku.md |
| BEAM-6 | Hot code reloading in dev; live remote shells (`iex`, `bin/app remote`) into running systems. | CORE | request_lifecycle.md, releases.md |
| BEAM-7 | Functional/immutable core: `conn` and `socket` are immutable values transformed and returned — the property LiveView's change tracking and Plug's composability are built on. | CORE | plug/README.md, liveview/README.md |

---

## Signature design decisions

**Everything is an explicit pipeline of one immutable value.** Plug reduces the entire
HTTP layer to `conn -> conn` functions; the endpoint, router pipelines, controllers, and
even authentication are just plugs listed in source order. There is no hidden middleware
registry: reading `endpoint.ex` and `router.ex` *is* reading the request path. Ecto
repeats the shape for data (`changeset` pipelines) and LiveView for UI state
(`socket` transformations). Immutability is what makes this composition safe — and
what makes LiveView's diff engine and Ecto's change tracking possible at all.

**"Phoenix is not your application."** The generated project splits `lib/app` (domain,
Ecto, contexts) from `lib/app_web` (the web *interface* to that domain). Contexts —
plain modules like `Catalog` and `Orders` with intention-revealing functions
(`complete_order/2`, not `create_order/1`) — are the public API; controllers and
LiveViews are thin callers. The framework's own generators enforce this: every
`phx.gen.*` writes a context first and a web layer second.

**Generators scaffold *into* your app instead of hiding features in the framework.**
The auth system, core components, release scripts, even the Dockerfile are generated
source code you own and modify. The documented tradeoff is explicit: total freedom to
adapt, in exchange for porting future improvements yourself. Phoenix v1.8's Scopes
push this further — a config-declared authorization shape that all *future* generator
runs automatically thread through routes, contexts, queries, PubSub topics, and tests,
making per-tenant data scoping the default rather than a discipline.

**LiveView: the server is the application, diffs are the wire format.** Rather than a
client framework plus an API, LiveView keeps state in a server process, re-renders only
changed template dynamics, and ships minimal diffs over a channel — with progressive
degradation (first paint is plain HTTP), automatic reconnect/remount recovery, form
recovery, optimistic-UI loading classes, and a JS command escape hatch. HEEx's
compile-time change tracking (statics vs dynamics, per-assign granularity) is the
enabling technology, and it does double duty as the ordinary template engine for
non-live pages.

**Compile time is a feature.** Routes compile to one pattern-matched function; `~p`
paths are verified against the router at build time; Ecto queries are checked against
schemas and pre-compiled; fragment string interpolation is a *compile error*; HEEx
validates HTML structure during compilation. A whole class of broken-link,
typo'd-column, and injection bugs is moved from production to `mix compile`.

**Explicit data access, no magic objects.** Ecto refuses lazy loading, identity maps,
and active-record `save`: the Repo is the only thing that touches the database,
preloads are always explicit, validations (in-app) are distinguished from constraints
(database, race-proof), and the guides consistently push integrity into the database
(`references on_delete`, unique indexes, upserts) rather than application callbacks.

**The BEAM replaces infrastructure.** Process-per-connection concurrency, supervision
for fault tolerance, and native clustering mean PubSub, Presence, "who's online" CRDTs,
and cross-node broadcasts ship with zero brokers, queues, or Redis — the corpus
repeatedly contrasts this with stacks that need external services for the same
features.

## Non-goals & gaps

Relative to batteries-included frameworks (Rails/Laravel/Django), the corpus shows
Phoenix deliberately does **not** provide:

- **Background jobs** — no queue, retries, workers, or cron anywhere in core. The
  answer is OTP processes for the simple cases and Oban (third-party) for real job
  systems. The biggest single omission versus peers.
- **Caching** — no cache store abstraction, fragment caching, or HTTP cache helpers;
  ETS and the structural performance model (precompiled templates, diffs) stand in.
- **File storage abstraction** — `Plug.Upload` and LiveView uploads handle transport;
  durable storage, naming, and cloud clients are your problem (guides say so).
- **Admin framework** — LiveDashboard is ops introspection, not data administration;
  admin CRUD is scaffolded like any other resource.
- **First-party OAuth/social login** — `phx.gen.auth` covers first-party identity;
  Ueberauth is the named ecosystem answer.
- **Notification channels beyond email** — Swoosh only; SMS/push are explicitly
  "integrate it yourself".
- **API conveniences** — no serializer DSL, pagination, rate limiting, API versioning
  helpers, or OpenAPI tooling; JSON views are plain functions and versioning is nested
  scopes.
- **CORS** — not in core (CORSPlug, third-party, appears only in the security guide).
- **Locale negotiation** — Gettext ships, but Accept-Language detection, locale
  routing, and date/number localization are hand-rolled patterns.
- **ORM conveniences by design** — no lazy loading, callbacks/observers, default
  scopes, dirty-tracking magic, STI, or soft deletes; Ecto's explicitness is presented
  as the feature.
- **Generated-code maintenance** — auth (and all scaffolding) is frozen at generation
  time; upstream security improvements must be ported by hand.
- **User-enumeration resistance** — the generated auth intentionally does not attempt
  it (documented, with timing-attack caveats).
- **Error-tracking/APM integrations** — telemetry events are the only surface; no
  bundled Sentry-style reporter.
- **Delivery guarantees in real-time** — channel messages are at-most-once; message
  persistence and catch-up are documented as application code.
