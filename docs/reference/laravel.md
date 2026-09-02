# Laravel — Feature Inventory

An exhaustive inventory of every capability documented in the official **Laravel 13.x**
documentation corpus, organized by problem solved. Derived from
a local corpus (source: `github.com/laravel/docs`, ref `13.x`,
commit `6d8246ff751a299421520660979cc34a2b255bc9`, fetched 2026-07-14, 103 markdown files).
This is an inventory of what exists — research input for the Volt framework design,
not a build plan. Section skeleton (P1–P21 + extras) is shared with the Rails/Phoenix/Django
inventories so rows can be aligned in a comparison matrix.

Tier legend:

- `CORE` — ships with the default `laravel/laravel` install, no extra package.
- `OPT` — first-party, separate install (composer/npm package: Horizon, Sanctum, Reverb, Octane, Cashier, AI SDK…).
- `ECO` — third-party but blessed and documented in the official docs (Pusher, Inertia, Livewire, WorkOS…).
- `DIY` — the docs document a pattern/recipe; no shipped mechanism.

---

## P1 — Routing & HTTP dispatch

**Problem.** Map incoming URLs + HTTP verbs to handler code, extract typed parameters, generate URLs back out, and control cross-cutting dispatch concerns (throttling, CORS, caching). **Answer.** A fluent `Route` facade over closure or controller handlers in convention-based route files (`routes/web.php`, `routes/api.php`, `routes/console.php`, `routes/channels.php`), with implicit model binding as the signature convenience, plus an optional file-based routing package (Folio).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ROUTE-1 | Verb-based route registration: `Route::get/post/put/patch/delete/options`, plus `match` (multiple verbs) and `any`, to closures or `[Controller::class, 'method']` | CORE | routing.md |
| ROUTE-2 | Convention route files: `routes/web.php` (session/CSRF middleware group) and opt-in `routes/api.php` via `install:api` (stateless, `/api` prefix) | CORE | routing.md |
| ROUTE-3 | Shorthand routes: `Route::redirect` / `permanentRedirect` and `Route::view` (render view without controller) | CORE | routing.md |
| ROUTE-4 | Required and optional (`{param?}`) route parameters injected by name into handler signature | CORE | routing.md |
| ROUTE-5 | Parameter constraints: `where` regex, helpers `whereNumber/whereAlpha/whereAlphaNumeric/whereUuid/whereUlid/whereIn`, and global patterns via `Route::pattern` | CORE | routing.md |
| ROUTE-6 | Named routes (`->name()`) with `route()` URL generation, parameter/query-string filling, and `named()`/`routeIs()` inspection | CORE | routing.md, urls.md |
| ROUTE-7 | Route groups sharing middleware, controller, URI prefix, and name prefix | CORE | routing.md |
| ROUTE-8 | Subdomain routing: `Route::domain('{account}.example.com')` with subdomain parameters | CORE | routing.md |
| ROUTE-9 | Route model binding: implicit resolution of Eloquent models from `{param}`, customizable key (`{post:slug}` or `getRouteKeyName`), scoped child bindings, `withTrashed`, `missing()` fallback | CORE | routing.md |
| ROUTE-10 | Implicit enum binding: string-backed PHP enum route params 404 unless the value is a valid case | CORE | routing.md |
| ROUTE-11 | Explicit binding via `Route::model` / `Route::bind` and model-side `resolveRouteBinding` / `resolveChildRouteBinding` overrides | CORE | routing.md |
| ROUTE-12 | Fallback route `Route::fallback` for unmatched requests | CORE | routing.md |
| ROUTE-13 | Route rate limiting: named limiters via `RateLimiter::for` (per minute/hour/day, `by()` segmentation, multiple limits, custom 429 responses), attached with `throttle:` middleware; Redis-optimized variant | CORE | routing.md |
| ROUTE-14 | Form method spoofing via hidden `_method` field / `@method` Blade directive for PUT/PATCH/DELETE from HTML forms | CORE | routing.md |
| ROUTE-15 | Current route introspection: `$request->route()`, `Route::current/currentRouteName/currentRouteAction` | CORE | routing.md |
| ROUTE-16 | CORS: `HandleCors` middleware auto-responds to OPTIONS preflight, configured via published `config/cors.php` | CORE | routing.md |
| ROUTE-17 | Route caching (`route:cache`/`route:clear`) compiling the route table for production dispatch speed | CORE | routing.md, deployment.md |
| ROUTE-18 | `route:list` CLI with middleware expansion, path/vendor filtering | CORE | routing.md |
| ROUTE-19 | Routing customization in `bootstrap/app.php`: extra route files, custom prefixes/middleware per file, fully custom registration via `using()` | CORE | routing.md |
| ROUTE-20 | Signed URLs: `URL::signedRoute` / `temporarySignedRoute` with HMAC signature, `signed` middleware validation, expired-signature error page hook | CORE | urls.md |
| ROUTE-21 | URL generation toolkit: `url()`, `action()`, `URL::defaults` for default route params, fluent immutable `Uri` object (`Uri::of()->withQuery()...`) | CORE | urls.md, helpers.md |
| ROUTE-22 | Folio: page-based routing — Blade files under `resources/views/pages` become routes, with `[id]` params, `[...ids]` wildcards, `[Model]` binding (incl. soft-deleted opt-in), per-page middleware, named routes, render hooks, subdomains, route caching | OPT | folio.md (`laravel/folio`) |

## P2 — Request handling: controllers & middleware

**Problem.** Organize handler logic, give it typed access to request data (input, headers, files, cookies), compose cross-cutting behavior around it, and build responses. **Answer.** Plain controller classes with container-powered dependency injection, an onion of middleware configured centrally in `bootstrap/app.php`, a rich `Request` object, and a fluent response/redirect builder.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CTRL-1 | Controllers as plain classes (no required base class); `make:controller` generator; single-action controllers via `__invoke` | CORE | controllers.md |
| CTRL-2 | Controller middleware via `HasMiddleware` interface or `#[Middleware]` PHP attribute on class/method (new in 13); `only/except` scoping; closure middleware | CORE | controllers.md, releases.md |
| CTRL-3 | `#[Authorize]` controller attribute declaring policy/gate checks per action (new in 13) | CORE | controllers.md, releases.md |
| CTRL-4 | Resource controllers: `Route::resource`/`resources` mapping 7 CRUD actions, `apiResource` (5 actions), `only/except` partial routes | CORE | controllers.md |
| CTRL-5 | Resource routing depth: nested resources with `shallow()`, scoped bindings (`scoped(['comment' => 'slug'])`), singleton resources (`Route::singleton`, creatable/destroyable), custom names/parameters, `withTrashed`, `missing()`, localized verbs via `Route::resourceVerbs` | CORE | controllers.md |
| CTRL-6 | Constructor and method dependency injection in controllers (container resolves type-hints alongside route params) | CORE | controllers.md, container.md |
| CTRL-7 | Middleware contract: `handle($request, Closure $next)` before/after logic; generator `make:middleware` | CORE | middleware.md |
| CTRL-8 | Central middleware configuration in `bootstrap/app.php`: global stack append/prepend/remove/replace, route aliases, group membership editing (`web`/`api` groups), explicit priority ordering | CORE | middleware.md |
| CTRL-9 | Middleware parameters (`throttle:60,1`-style `:`-delimited args) | CORE | middleware.md |
| CTRL-10 | Terminable middleware: `terminate()` runs after the response is sent (FastCGI) | CORE | middleware.md |
| CTRL-11 | Request introspection: `path/is/routeIs/url/fullUrl/fullUrlWithQuery/host/method/isMethod`, header access, `bearerToken`, `ip/ips`, content negotiation (`accepts/prefers/expectsJson`) | CORE | requests.md |
| CTRL-12 | PSR-7 request/response bridge (Symfony PSR bridge; return PSR-7, framework converts back) | CORE | requests.md |
| CTRL-13 | Input retrieval: `input` (dot notation, defaults), `query`, `json`, typed helpers `string/integer/float/boolean/array/date/enum/enums`, `collect`, `only/except`, dynamic properties | CORE | requests.md |
| CTRL-14 | Input presence predicates: `has/hasAny/whenHas/filled/isNotFilled/anyFilled/whenFilled/missing/whenMissing`; input merging `merge/mergeIfMissing` | CORE | requests.md |
| CTRL-15 | Old input: `flash/flashOnly/flashExcept`, `withInput()` on redirects, `old()` helper for form repopulation | CORE | requests.md |
| CTRL-16 | Global input normalization: `TrimStrings` and `ConvertEmptyStringsToNull` middleware, disable-able per-route/conditionally | CORE | requests.md |
| CTRL-17 | Uploaded files: `$request->file()`, validity/extension/mime helpers, `store/storeAs/storePublicly` straight to filesystem disks | CORE | requests.md |
| CTRL-18 | Response building: strings/arrays auto-converted, `response()` with status/`header()`/`withHeaders`, cookie attach/expire (queued cookies), automatic cookie encryption | CORE | responses.md |
| CTRL-19 | Redirects: `redirect()`, `back()->withInput()`, `route()`, `action()`, `away()` (external), `with()` flashed session data | CORE | responses.md |
| CTRL-20 | Response types: view responses, `response()->json()`/`jsonp`, `download()`, `file()` (inline display) | CORE | responses.md |
| CTRL-21 | Streaming: `response()->stream/streamJson/streamDownload`, SSE `eventStream` (with `StreamedEvent`, completion sentinel), client consumption via first-party `@laravel/stream-react|vue|svelte` npm packages and `useStream`/`useEventStream` hooks | CORE | responses.md |
| CTRL-22 | Response macros: `Response::macro` custom response builders | CORE | responses.md |
| CTRL-23 | Session: file/cookie/database/memcached/redis/dynamodb/mongodb/array drivers; `get/put/push/pull/increment`, `only/except`, flash (`flash/reflash/keep/now`), `forget/flush`, ID `regenerate/invalidate` | CORE | session.md |
| CTRL-24 | Session cache: per-session scoped cache store (`session()->cache()`) | CORE | session.md |
| CTRL-25 | Session blocking: per-session request serialization via atomic locks (`->block($lock, $wait)`) | CORE | session.md |
| CTRL-26 | Custom session drivers via `SessionHandlerInterface` + `Session::extend` | CORE | session.md |
| CTRL-27 | Request lifecycle: single `public/index.php` entry, HTTP/console kernels, provider boot, router dispatch — documented architecture | CORE | lifecycle.md |

## P3 — Views, templating & frontend assets

**Problem.** Render HTML server-side with layouts, components and safe interpolation, and get modern JS/CSS through a bundler into pages. **Answer.** Blade — a compiled, zero-overhead template language with a full component system — plus a first-party Vite plugin (`@vite` directive, HMR, SSR), with Livewire and Inertia as the blessed paths to rich interactivity.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VIEW-1 | View layer: `view()` helper / `View` facade, nested directories (dot notation), `View::first`, `View::exists` | CORE | views.md |
| VIEW-2 | View data: per-view `with`, global `View::share`, view composers (class or closure, multi-view, wildcard) and creators | CORE | views.md |
| VIEW-3 | View compilation cache + `view:cache`/`view:clear` precompilation for deploys | CORE | views.md, deployment.md |
| VIEW-4 | Blade echo `{{ }}` with automatic `htmlspecialchars` XSS escaping; raw `{!! !!}`; `@verbatim`; `Js::from`/`@js` for safe JS injection | CORE | blade.md |
| VIEW-5 | Control-flow directives: `@if/@unless/@isset/@empty`, `@auth/@guest`, `@production/@env`, `@switch`, `@session`, `@hasSection/@sectionMissing` | CORE | blade.md |
| VIEW-6 | Loops: `@foreach/@forelse/@for/@while` with `@continue/@break` and the `$loop` metadata object (index, first/last, depth, parent) | CORE | blade.md |
| VIEW-7 | Conditional attribute directives: `@class`, `@style`, `@checked/@selected/@disabled/@readonly/@required` | CORE | blade.md |
| VIEW-8 | Subview inclusion: `@include/@includeIf/@includeWhen/@includeUnless/@includeFirst`, `@each` collection rendering, `@once`, `@php`, `@use`, Blade comments | CORE | blade.md |
| VIEW-9 | Class-based components: `make:component`, constructor props, methods/computed access, attribute bag (`$attributes->merge/class/prepends/filter`), reserved data, slots (default + named + scoped `$slot`, slot attributes), inline views, index components | CORE | blade.md |
| VIEW-10 | Anonymous components (Blade-file-only) with `@props`, `@aware` parent data access, custom component paths/namespaces, index components | CORE | blade.md |
| VIEW-11 | Dynamic components (`<x-dynamic-component>`) and manual registration/namespacing for packages | CORE | blade.md, packages.md |
| VIEW-12 | Layouts two ways: component-based layouts with slots, or classic template inheritance (`@extends/@section/@yield/@parent`) | CORE | blade.md |
| VIEW-13 | Form helpers: `@csrf`, `@method`, `@error` (with named bag support) | CORE | blade.md |
| VIEW-14 | Stacks: `@push/@pushOnce/@pushIf/@prepend` rendered by `@stack` for per-page scripts/styles | CORE | blade.md |
| VIEW-15 | `@inject` service-container injection into templates | CORE | blade.md |
| VIEW-16 | Inline rendering (`Blade::render` from string) and Blade fragments (`@fragment` + `->fragment()` responses) for turbo/htmx-style partial responses | CORE | blade.md |
| VIEW-17 | Blade extension: `Blade::directive` custom directives, custom echo handlers (`Blade::stringable`), `Blade::if` custom conditionals | CORE | blade.md |
| VIEW-18 | Vite integration: `laravel-vite-plugin`, `@vite` directive (entry points, dev-server HMR, build manifest), `Vite::asset` for static assets, blade `refresh`-on-save watching | CORE | vite.md |
| VIEW-19 | Vite framework presets and options: Vue/React/Inertia plugin recipes, aliases, URL processing, environment variables (`VITE_` prefix), custom base URLs (CDN), dev-server CORS/URL correction | CORE | vite.md |
| VIEW-20 | Vite production hardening: SSR builds (`ssr` entry, `ssr:serve`), CSP nonce (`useCspNonce`), Subresource Integrity via manifest, arbitrary script/style tag attributes, asset prefetching strategies (`Vite::prefetch`) | CORE | vite.md |
| VIEW-21 | Vite font optimization: font providers, local font handling, rendered via Blade `@fonts` directive | CORE | vite.md, blade.md |
| VIEW-22 | Pagination rendering: `links()` Blade output (Tailwind default, Bootstrap 4/5 opt-in), link-window control (`onEachSide`), fully customizable views | CORE | pagination.md |
| VIEW-23 | Livewire: full-stack reactive components in PHP/Blade (`wire:click` etc.) — the blessed no-JS-framework interactivity path | ECO | blade.md, frontend.md |
| VIEW-24 | Inertia: SPA glue for React/Vue/Svelte views with server-side routing, incl. SSR support — the blessed JS-framework path (used by starter kits) | ECO | frontend.md, starter-kits.md |
| VIEW-25 | Laravel Mix: legacy webpack-based asset pipeline (superseded by Vite) | OPT | mix.md |

## P4 — Data layer: models, ORM & queries

**Problem.** Talk to relational (and document) databases: connections, query composition, row↔object mapping, relationships, mutation lifecycles, and collection ergonomics. **Answer.** A two-layer stack — a fluent SQL query builder over PDO (MariaDB/MySQL/PostgreSQL/SQLite/SQL Server) and Eloquent, an ActiveRecord ORM with relationships, eager loading, casts, events and factories — plus first-party Redis and MongoDB integrations.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ORM-1 | Multi-connection database config (MariaDB, MySQL, PostgreSQL, SQLite, SQL Server), URL-style DSNs, per-query connection selection | CORE | database.md |
| ORM-2 | Read/write connection splitting with `sticky` option (read-your-writes within request) | CORE | database.md |
| ORM-3 | Pooled PostgreSQL connections (PgBouncer-style pooling support) | CORE | database.md (new in 13.x) |
| ORM-4 | Raw SQL escape hatch: `DB::select/insert/update/delete/statement/unprepared` with named bindings | CORE | database.md |
| ORM-5 | Query event hooks: `DB::listen`, cumulative time budget alerts via `whenQueryingForLongerThan` | CORE | database.md |
| ORM-6 | Transactions: `DB::transaction` closure with deadlock retry count, manual `beginTransaction/commit/rollBack`, `afterCommit` hooks | CORE | database.md |
| ORM-7 | DB tooling: `php artisan db` CLI session, `db:show`, `db:table`, `db:monitor` (connection-count alerts + event) | CORE | database.md |
| ORM-8 | Query builder retrieval: `table()->get/first/firstOrFail/find/value/pluck`, `exists/doesntExist` | CORE | queries.md |
| ORM-9 | Constant-memory iteration: `chunk/chunkById/orderedLazyById`, `lazy/lazyById/lazyByIdDesc` (LazyCollection streams) | CORE | queries.md, eloquent.md |
| ORM-10 | Aggregates: `count/max/min/avg/sum` composable with constraints | CORE | queries.md |
| ORM-11 | Projections: `select/addSelect/distinct`, subquery selects (`selectSub`), `fromSub` | CORE | queries.md |
| ORM-12 | Raw expressions: `DB::raw` plus `selectRaw/whereRaw/orWhereRaw/havingRaw/orderByRaw/groupByRaw` with bindings | CORE | queries.md |
| ORM-13 | Joins: inner/left/right/cross, multi-condition closure joins (`on/orOn` + wheres), subquery joins, lateral joins (`joinLateral`) | CORE | queries.md |
| ORM-14 | Unions: `union/unionAll` | CORE | queries.md |
| ORM-15 | Where vocabulary: basic ops, `orWhere`, `whereNot`, `whereAny/whereAll/whereNone` (column sets), `whereBetween(Columns)`, `whereIn` (+`whereIntegerInRaw`), `whereNull`, `whereLike`, date/time family (`whereDate/Month/Day/Year/Time/Past/Future/Today/Before/After`), `whereColumn`, closure-based logical grouping | CORE | queries.md |
| ORM-16 | JSON queries: `->` path syntax, `whereJsonContains(Key)`, `whereJsonLength`, `whereJsonOverlaps` on JSON-capable databases | CORE | queries.md |
| ORM-17 | Advanced wheres: `whereExists`, subquery comparisons (`where(fn, ...)`), full-text `whereFullText/orWhereFullText` (MATCH…AGAINST / tsvector per driver) | CORE | queries.md |
| ORM-18 | Vector similarity queries: `whereVectorSimilarTo` (cosine similarity, `minSimilarity` threshold, auto-embedding of plain strings), `whereVectorDistanceLessThan`, `selectVectorDistance`, `orderByVectorDistance` on pgvector/MongoDB | CORE | queries.md, search.md (new in 13; string auto-embedding needs AI SDK) |
| ORM-19 | Ordering/grouping/limits: `orderBy/latest/oldest/inRandomOrder/reorder`, `groupBy/having/havingBetween`, `limit/offset` | CORE | queries.md |
| ORM-20 | Conditional composition: `when($value, $callback, $default)` | CORE | queries.md |
| ORM-21 | Writes: `insert/insertOrIgnore/insertGetId/insertUsing`, `upsert` (conflict target + update columns), `update/updateOrInsert`, JSON column updates, `increment/decrement/incrementEach`, `delete/truncate` | CORE | queries.md |
| ORM-22 | Pessimistic locking: `sharedLock`, `lockForUpdate` | CORE | queries.md |
| ORM-23 | Reusable query components: `tap()`/`pipe()` with invokable query-object classes for shared filter/pagination logic | CORE | queries.md |
| ORM-24 | Query debugging: `dd/dump/dumpRawSql/ddRawSql` | CORE | queries.md |
| ORM-25 | Eloquent conventions: snake-case plural table inference, `$table/$primaryKey/$incrementing/$keyType` overrides, `created_at/updated_at` timestamps (disable/rename/touch), per-model `$connection`, `$attributes` defaults | CORE | eloquent.md |
| ORM-26 | UUID/ULID primary keys via `HasUuids`/`HasUlids` traits (ordered UUIDv7, custom generation, `HasVersion4Uuids`) | CORE | eloquent.md |
| ORM-27 | Model strictness: `Model::shouldBeStrict()` — prevent lazy loading, silently-discarded fills, and missing-attribute access (env-conditional) | CORE | eloquent.md, eloquent-relationships.md |
| ORM-28 | Retrieval API: `all`, chainable builder, `fresh/refresh`, `cursor()` (single-query streaming), advanced subquery selects/ordering | CORE | eloquent.md |
| ORM-29 | Single-model retrieval: `find/first/firstWhere/findOr/firstOr/sole/findOrFail/firstOrFail` (404-throwing variants) | CORE | eloquent.md |
| ORM-30 | Retrieve-or-create family: `firstOrCreate/firstOrNew/updateOrCreate/incrementOrCreate` | CORE | eloquent.md |
| ORM-31 | Mass-assignment guard: `$fillable`/`$guarded`, `forceFill`, `preventSilentlyDiscardingAttributes` | CORE | eloquent.md |
| ORM-32 | Change tracking: `isDirty/isClean/wasChanged/getOriginal/getChanges/getPrevious` | CORE | eloquent.md |
| ORM-33 | Deletes & soft deletes: `delete/destroy/truncate`; `SoftDeletes` trait with `deleted_at`, `withTrashed/onlyTrashed/restore/forceDelete`, restore/trashed model events | CORE | eloquent.md |
| ORM-34 | Model pruning: `Prunable`/`MassPrunable` traits + scheduled `model:prune` with `pruning()` hook | CORE | eloquent.md |
| ORM-35 | Replication: `replicate()` with attribute exclusion | CORE | eloquent.md |
| ORM-36 | Global scopes: scope classes/closures applied to all queries, `#[ScopedBy]` attribute, `withoutGlobalScope(s)` | CORE | eloquent.md |
| ORM-37 | Local & dynamic scopes: `#[Scope]`-attributed methods, chainable, parameterized; pending attributes (`withAttributes`) so scope-created models inherit scope constraints | CORE | eloquent.md |
| ORM-38 | Model comparison `is()/isNot()` (same key/table/connection) | CORE | eloquent.md |
| ORM-39 | Model lifecycle events (retrieved, creating/created, updating/updated, saving/saved, deleting/deleted, trashed, restoring/restored, replicating) via `$dispatchesEvents`, closures, or Observer classes (`#[ObservedBy]`); `afterCommit` observers; muting via `withoutEvents/saveQuietly/deleteQuietly` | CORE | eloquent.md |
| ORM-40 | Relationship types: `hasOne`, `hasMany`, `belongsTo`, has-one-of-many (`latestOfMany/oldestOfMany/ofMany` with aggregate + closure constraints), `hasOneThrough`, `hasManyThrough` (+ fluent `through()->has()` string syntax) | CORE | eloquent-relationships.md |
| ORM-41 | Many-to-many: `belongsToMany` with pivot access (`withPivot/withTimestamps/as`), pivot filtering/ordering (`wherePivot*`, `orderByPivot`), custom pivot models | CORE | eloquent-relationships.md |
| ORM-42 | Polymorphic relationships: one-to-one, one-to-many, one-of-many, many-to-many (`morphTo/morphOne/morphMany/morphToMany/morphedByMany`), custom morph type column, morph maps with `enforceMorphMap` | CORE | eloquent-relationships.md |
| ORM-43 | Scoped & dynamic relationships: relationship-level `where`/`withAttributes` scoping, runtime `resolveRelationUsing` | CORE | eloquent-relationships.md |
| ORM-44 | Relationship existence queries: `has/orHas/whereHas/whereDoesntHave/doesntHave`, inline `whereRelation/whereMorphRelation`, morph-aware `whereHasMorph` | CORE | eloquent-relationships.md |
| ORM-45 | Relationship aggregates: `withCount` (aliased, constrained), deferred `loadCount`, `withSum/withMin/withMax/withAvg/withExists`, morph-to variants | CORE | eloquent-relationships.md |
| ORM-46 | Eager loading: `with` (nested, specific columns, multiple), default `$with` (+`without/withOnly`), constrained eager loads (`withWhereHas`), lazy eager loading `load/loadMissing/loadMorph` | CORE | eloquent-relationships.md |
| ORM-47 | Automatic eager loading (`withRelationshipAutoloading` / global `Model::automaticallyEagerLoadRelationships`) and N+1 prevention `preventLazyLoading` with violation handler | CORE | eloquent-relationships.md |
| ORM-48 | Persisting through relations: `save/saveMany/create/createMany` (+quietly), recursive `push`, `associate/dissociate`, m2m `attach/detach/sync/syncWithoutDetaching/syncWithPivotValues/toggle/updateExistingPivot` | CORE | eloquent-relationships.md |
| ORM-49 | Parent timestamp touching via `$touches` | CORE | eloquent-relationships.md |
| ORM-50 | Eloquent collections: model-aware methods (`find/fresh/load/modelKeys/diff/intersect/unique/only/except/toQuery/append/setVisible`), custom collection classes via `#[CollectedBy]` or `newCollection` | CORE | eloquent-collections.md |
| ORM-51 | Accessors/mutators: unified `Attribute::make(get:, set:)`, multi-attribute mutation, value-object caching control (`shouldCache/withoutObjectCaching`) | CORE | eloquent-mutators.md |
| ORM-52 | Attribute casting: primitives, `array/json/object/collection`, `AsArrayObject/AsCollection/AsStringable/AsEnumCollection/AsEnumArrayObject`, date/immutable-date with formats, enum casts, `encrypted:*` casts, `hashed`, binary `AsBinary` for uuid/ulid binary columns (13), query-time `withCasts` | CORE | eloquent-mutators.md |
| ORM-53 | Custom casts: `CastsAttributes` classes, value-object casts with serialization control, inbound-only casts, cast parameters, `equals` comparison, self-casting `Castable` value objects (incl. anonymous-class casters) | CORE | eloquent-mutators.md |
| ORM-54 | Serialization: `toArray/attributesToArray/toJson`, `$hidden/$visible` (+`makeVisible/makeHidden` runtime), computed `$appends/append`, `serializeDate` date format override | CORE | eloquent-serialization.md |
| ORM-55 | Model factories: `definition()` with Faker, states (incl. built-in `trashed`), `afterMaking/afterCreating` callbacks, `make/create/createMany/raw`, `Sequence` cycling, relationship builders (`has/for/hasAttached` + magic methods), `recycle()` for shared parents, custom model resolution | CORE | eloquent-factories.md |
| ORM-56 | Pagination: `paginate` (length-aware), `simplePaginate`, `cursorPaginate` (keyset, ordered-column encoding), relationship pagination, `withQueryString/appends/fragment`, manual paginator construction, JSON structure, full paginator instance-method API | CORE | pagination.md |
| ORM-57 | Redis integration: phpredis/predis clients, clusters (native + client-side sharding), TLS `scheme`, `Redis` facade magic commands + `command()`, `transaction()` (MULTI/EXEC), `pipeline()`, pub/sub incl. `psubscribe` wildcard | CORE | redis.md (driver: phpredis extension or predis package) |
| ORM-58 | MongoDB: official `mongodb/laravel-mongodb` package — Eloquent models on collections, embedded relationships, query builder, `vectorSearch`, plus mongodb cache (TTL-index), queue, session, Scout drivers | OPT | mongodb.md |

## P5 — Schema evolution: migrations & seeding

**Problem.** Version the database schema across environments and time, and populate development/test data reproducibly. **Answer.** Class-based migration files with a fluent schema builder abstracting five SQL dialects, batch-tracked rollback, schema squashing, and seeder classes that compose model factories.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MIG-1 | Migration generation: `make:migration` with name-based table/create inference, timestamp ordering, anonymous class files, custom path | CORE | migrations.md |
| MIG-2 | `up`/`down` migration pairs; `--pretend` SQL preview; `withoutForeignKeyConstraints` sections | CORE | migrations.md |
| MIG-3 | Run/rollback lifecycle: `migrate` (batch-tracked), `migrate:status`, `rollback --step/--batch`, `reset`, `refresh`, `fresh --seed` | CORE | migrations.md |
| MIG-4 | Production safety: `--force`, `--isolated` (cache-lock so one server runs migrations during multi-server deploy) | CORE | migrations.md |
| MIG-5 | Schema squashing: `schema:dump --prune` writes one SQL schema file; fresh installs load dump then remaining migrations | CORE | migrations.md |
| MIG-6 | Table ops: `Schema::create/table/rename/drop/dropIfExists`, `hasTable/hasColumn/hasIndex`, engine/charset/collation/comments, temporary tables, non-default connections | CORE | migrations.md |
| MIG-7 | ~100 column types incl. `id`, numeric/decimal families, string/text sizes, boolean, `enum`/`set`, date/time(+Tz), json/jsonb, `uuid/ulid`, binary, ip/mac, geometry/geography (SRID), `morphs`/`uuidMorphs`/`ulidMorphs`/`nullableMorphs`, `foreignId(For)`, `rememberToken`, `softDeletes`, `timestamps`, `vector(dimensions:)` | CORE | migrations.md (`vector` new in 13) |
| MIG-8 | Column modifiers: `nullable/default/useCurrent(OnUpdate)/unsigned/autoIncrement(from)/comment/charset/collation/invisible/after/first/storedAs/virtualAs` | CORE | migrations.md |
| MIG-9 | Column changes: `change()` (must restate modifiers), `renameColumn`, `dropColumn` (multi), convenience drops (`dropTimestamps/dropSoftDeletes/dropMorphs...`) | CORE | migrations.md |
| MIG-10 | Indexes: `primary/unique/index/fullText(->language())/spatialIndex/vector-index (HNSW)`, composite, named, renaming, dropping; created inline via chained `->unique()->index()` | CORE | migrations.md, search.md |
| MIG-11 | Foreign keys: `foreign()->references()->on()`, terse `foreignId()->constrained()` with conventions, `cascadeOnDelete/nullOnDelete/restrictOnUpdate...`, enable/disable constraint enforcement | CORE | migrations.md |
| MIG-12 | `Schema::ensureVectorExtensionExists()` enabling pgvector before vector tables | CORE | search.md (new in 13) |
| MIG-13 | Migration events (`MigrationsStarted/Ended`, `MigrationStarted/Ended`, `NoPendingMigrations`, `SchemaDumped/Loaded`) | CORE | migrations.md |
| MIG-14 | Seeders: `make:seeder`, `DatabaseSeeder::run` with DI, factory usage, `call([...])` composition, `WithoutModelEvents`, `db:seed --class`, `migrate:fresh --seed(er)`, prod `--force` | CORE | seeding.md |

## P6 — Validation & data integrity

**Problem.** Reject malformed input at the boundary with reusable rules, useful error messages, and a good authoring story for both HTML forms and JSON APIs. **Answer.** One rule vocabulary (~100 built-in rules) usable three ways — inline `$request->validate()`, self-contained FormRequest classes, or manual `Validator::make` — with automatic redirect-or-422 behavior and deep array validation.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VAL-1 | Inline validation: `$request->validate()` — auto redirect back with errors + old input for web, auto 422 JSON (`message` + `errors` map) for XHR | CORE | validation.md |
| VAL-2 | Rule syntax: pipe strings or arrays, `bail` stop-on-first-failure per field, dot-notation nested attributes | CORE | validation.md |
| VAL-3 | Error display: `$errors` MessageBag shared to all views, `@error` directive, named error bags for multi-form pages | CORE | validation.md |
| VAL-4 | FormRequest classes: `make:request`, `rules()` with DI, `authorize()` (403 on failure, route-binding access), `after()` validators, `messages()`/`attributes()` overrides, `prepareForValidation/passedValidation` hooks, `stopOnFirstFailure`, fail-on-unknown-fields option | CORE | validation.md |
| VAL-5 | Manual validators: `Validator::make`, `->validate()/validateWithBag` auto-throw, `fails/errors`, custom redirect location/error bag | CORE | validation.md |
| VAL-6 | Validated data access: `validated()` and `safe()->only/except/merge/collect` (ValidatedInput) | CORE | validation.md |
| VAL-7 | MessageBag API: `first/get/all/has`, wildcard key retrieval | CORE | validation.md |
| VAL-8 | Language-file customization: per-rule, per-attribute-rule messages, friendly attribute names, value display names (`lang/xx/validation.php`) | CORE | validation.md |
| VAL-9 | ~100 built-in rules: types/formats (string, integer, decimal, boolean, date families, email w/ dns/spoof checks, url, uuid/ulid, ip/mac, json, hex_color, regex), comparisons (gt/gte/lt/lte, same, different, confirmed), presence matrix (required_if/unless/with/without/array_keys, present, filled, prohibited/prohibits, missing, exclude_*), sets (in/not_in, `Rule::in`, in_array, distinct), misc (accepted_if, declined, timezone, active_url, ascii, doesnt_start_with…) | CORE | validation.md |
| VAL-10 | Database rules: `exists`/`unique` with column/connection overrides, fluent `Rule::unique()->ignore()->where()` | CORE | validation.md |
| VAL-11 | Conditional rules: `sometimes` (only-if-present), `Validator::sometimes()` with fluent `$item` context for arrays, `Rule::when/unless`, `exclude` family | CORE | validation.md |
| VAL-12 | Array validation: `*` wildcards for nested arrays, `array:keys` whitelisting, nested messages, `{index}`/`{position}` placeholders in messages | CORE | validation.md |
| VAL-13 | File validation: fluent `File::types()->min()->max()`, `File::image(allowSvg:)`, `dimensions()` constraints, `extensions` | CORE | validation.md |
| VAL-14 | Password rules: fluent `Password::min()->letters()->mixedCase()->numbers()->symbols()->uncompromised()` (haveibeenpwned check), app-wide `Password::defaults()` | CORE | validation.md |
| VAL-15 | Custom rules: `make:rule` `ValidationRule` objects (+`DataAwareRule`/`ValidatorAwareRule`), inline closures, implicit rules (run on empty input) | CORE | validation.md |
| VAL-16 | Precognition: live/predictive validation — `HandlePrecognitiveRequests` middleware runs FormRequest rules without side effects; first-party `laravel-precognition-{vue,react,alpine}` npm form helpers with per-field validation, file-upload opt-in, side-effect hooks, testing headers | CORE | precognition.md (server core; npm client packages) |

## P7 — Authentication & authorization

**Problem.** Identify users across sessions, tokens and OAuth, and decide what each identity may do. **Answer.** A guard/provider architecture with session auth in core, an ecosystem of first-party packages layered on top (starter kits for UI, Fortify for headless backends, Sanctum for tokens/SPAs, Passport for OAuth2, Socialite for social login), and a gate/policy system for authorization.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| AUTH-1 | Guard + user-provider architecture (`config/auth.php`): session guard, eloquent/database providers, multiple guards per app | CORE | authentication.md |
| AUTH-2 | Current-user access: `Auth::user()/id()`, `$request->user()`, per-guard access | CORE | authentication.md |
| AUTH-3 | Route protection: `auth` middleware (guard-parameterized), unauthenticated redirect customization | CORE | authentication.md |
| AUTH-4 | Manual auth: `Auth::attempt` (extra conditions, closures), `attemptWhen`, `login/loginUsingId/once/onceUsingId`, remember-me cookies + `viaRemember` | CORE | authentication.md |
| AUTH-5 | HTTP Basic auth: `auth.basic` middleware and stateless `onceBasic` | CORE | authentication.md |
| AUTH-6 | Logout + session hygiene: `Auth::logout` with invalidate/regenerate recipe; `logoutOtherDevices` (with `AuthenticateSession` middleware) | CORE | authentication.md |
| AUTH-7 | Password re-confirmation flow: `password.confirm` middleware, confirm timeout config | CORE | authentication.md |
| AUTH-8 | Extensibility: custom guards (`Auth::extend`), closure request guards (`Auth::viaRequest`), custom user providers (`Auth::provider`, `UserProvider`/`Authenticatable` contracts) | CORE | authentication.md |
| AUTH-9 | Automatic password rehashing when the hashing work factor changes | CORE | authentication.md |
| AUTH-10 | Auth events (Registered, Attempting, Login, Logout, Failed, Lockout, PasswordResetLinkSent, Verified, etc.) | CORE | authentication.md, verification.md |
| AUTH-11 | Login throttling (username+IP based) via starter kits/Fortify; DIY via rate limiter | CORE | authentication.md |
| AUTH-12 | Gates: `Gate::define`, `allows/denies/check/any/none/authorize`, user-argument variants, `Response` objects with messages + custom HTTP status (`denyWithStatus/denyAsNotFound`), `before/after` interceptors, inline `Gate::allowIf/denyIf` | CORE | authorization.md |
| AUTH-13 | Policies: `make:policy`, auto-discovery by model naming (+`Gate::policy`/`UsePolicy` attribute manual registration), per-action methods, model-less methods, optional-user guests, `before` filters | CORE | authorization.md |
| AUTH-14 | Authorizing everywhere: `$user->can/cannot`, `Gate::authorize` in controllers, `#[Authorize]` attribute, `can:` middleware (`->can()` helper), Blade `@can/@cannot/@canany`, extra-context array arguments | CORE | authorization.md |
| AUTH-15 | Email verification: `MustVerifyEmail` interface auto-sends link, `verified` middleware, signed verification routes, resend flow, notification/customization hooks | CORE | verification.md |
| AUTH-16 | Password reset: token broker (`Password::sendResetLink/reset`), DB or cache token storage, expiry + throttle config, `auth:clear-resets`, notification customization | CORE | passwords.md |
| AUTH-17 | Starter kits: React/Vue/Svelte (Inertia) and Livewire variants scaffolding full auth UI (login, registration, reset, verification, profile), feature toggles in `config/fortify.php`, 2FA opt-in, customization guidance | OPT | starter-kits.md |
| AUTH-18 | WorkOS AuthKit starter-kit variant: SSO, social auth, passkeys, email-magic-auth via WorkOS service | ECO | starter-kits.md |
| AUTH-19 | Fortify: headless (frontend-agnostic) auth backend — registration, login, reset, verification, password confirmation, customizable auth pipeline (custom actions/rate limiting), redirect customization, view-layer disable for APIs | OPT | fortify.md |
| AUTH-20 | Fortify two-factor auth: TOTP with QR provisioning, recovery codes, optional password confirmation before enabling | OPT | fortify.md |
| AUTH-21 | Fortify passkeys: WebAuthn registration/login/confirmation with first-party `@laravel/passkeys` JS client | OPT | fortify.md (new in 13.x docs) |
| AUTH-22 | Sanctum API tokens: hashed personal access tokens with abilities (scopes), `auth:sanctum` guard, `tokenCan`, ability middleware (`abilities`/`ability`), expiration + `sanctum:prune-expired`, revocation | OPT | sanctum.md |
| AUTH-23 | Sanctum SPA auth: cookie/session-based, stateful-domain list, CSRF cookie handshake, private broadcast channel authorization; mobile-app token flow | OPT | sanctum.md |
| AUTH-24 | Passport: full OAuth2 server — authorization-code (+PKCE), device authorization, password, client-credentials, implicit grants; personal access tokens; client management API | OPT | passport.md |
| AUTH-25 | Passport token management: scopes (define, default, check middleware `scopes/scope`), token lifetimes, refresh/revoke, `passport:purge`, JSON API for first-party SPA token dashboards, testing helpers | OPT | passport.md |
| AUTH-26 | Socialite: OAuth1/2 social login (GitHub, Google, Facebook, X, LinkedIn, Slack…), scope control, stateless mode, optional params, normalized user object + token refresh fields | OPT | socialite.md |
| AUTH-27 | Testing: `actingAs($user, $guard)`, `Sanctum::actingAs` / `Passport::actingAs(Client)` fakes, auth assertions (`assertAuthenticated/assertGuest`) | CORE/OPT | http-tests.md, sanctum.md, passport.md |

## P8 — Security

**Problem.** Defend against the standard web attack classes — CSRF, XSS, session fixation, credential theft, tampered payloads — with safe defaults. **Answer.** Security is mostly ambient: forgery protection, cookie encryption, auto-escaping and hashing are on by default; encryption and signing are one-liner services keyed from `APP_KEY`.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| SEC-1 | Request forgery protection: `PreventRequestForgery` middleware — `Sec-Fetch-Site` origin verification first, CSRF-token fallback; strict `originOnly` mode | CORE | csrf.md (formalized in 13) |
| SEC-2 | CSRF tokens: per-session token, `@csrf` field, `X-CSRF-TOKEN`/`X-XSRF-TOKEN` header support (encrypted XSRF cookie for JS frameworks), URI exclusion list | CORE | csrf.md |
| SEC-3 | Symmetric encryption service: `Crypt::encrypt(String)/decrypt(String)` — AES with MAC, keyed by `APP_KEY` | CORE | encryption.md |
| SEC-4 | Graceful key rotation: `APP_PREVIOUS_KEYS` decrypt fallback + `key:generate` | CORE | encryption.md |
| SEC-5 | Password hashing: bcrypt default, argon2/argon2id drivers, per-call options (rounds/memory/threads), `Hash::check/needsRehash`, unknown-hash rejection (`verify` algorithm check) | CORE | hashing.md |
| SEC-6 | Cookie encryption + signing by default (`encryptCookies` exemptions configurable) | CORE | responses.md |
| SEC-7 | XSS: Blade `{{ }}` auto-escaping (see VIEW-4); `Js::from` safe JSON-to-JS | CORE | blade.md |
| SEC-8 | Trusted proxies (`trustProxies`, header selection, `*` wildcard) and trusted hosts (`trustHosts`) middleware for correct scheme/host behind LBs | CORE | requests.md |
| SEC-9 | Signed/tamper-proof URLs (see ROUTE-20); hash-verified email verification links | CORE | urls.md, verification.md |
| SEC-10 | Sensitive-data hygiene: encrypted casts (ORM-52), encrypted queued jobs (JOB-6), hidden model attributes (ORM-54), `#[SensitiveParameter]`-style env encryption (CONF) | CORE | cross-refs |
| SEC-11 | Compromised-password rule `uncompromised()` (k-anonymity HIBP lookup) | CORE | validation.md |
| SEC-12 | Security-adjacent gaps documented per-page (e.g. rate limiting for brute force, CSP nonce via Vite VIEW-20) | CORE | routing.md, vite.md |

## P9 — Background work: jobs, queues & scheduling

**Problem.** Move slow work off the request path, retry it safely, fan it out, and run recurring tasks — with operational visibility. **Answer.** A driver-abstracted queue (Redis/database/SQS/Beanstalkd/…) with a very deep job feature set (uniqueness, debouncing, middleware, chains, batches), a code-defined cron scheduler, and Horizon as the first-party Redis queue dashboard/supervisor.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| JOB-1 | Queue drivers behind one API: database, redis (+blocking pop), beanstalkd, SQS, mongodb (pkg), sync, null; per-connection multiple named queues | CORE | queues.md |
| JOB-2 | Queue failover driver: ordered connection list, automatic fallback on push failure | CORE | queues.md (new in 13) |
| JOB-3 | Job classes: `make:job`, `ShouldQueue` + `Queueable` trait; `SerializesModels` re-fetches Eloquent models on dequeue (`WithoutRelations` attribute) | CORE | queues.md |
| JOB-4 | Unique jobs: `ShouldBeUnique` (+`uniqueId/uniqueFor`, custom lock store), `ShouldBeUniqueUntilProcessing` | CORE | queues.md |
| JOB-5 | Debounced jobs: `#[DebounceFor]` — only the last dispatch in a window executes | CORE | queues.md (new in 13) |
| JOB-6 | Encrypted job payloads via `ShouldBeEncrypted` | CORE | queues.md |
| JOB-7 | Job middleware: custom classes plus shipped `WithoutOverlapping`, `RateLimited(WithRedis)`, `ThrottlesExceptions(WithRedis)` (circuit-breaker), `Skip` | CORE | queues.md |
| JOB-8 | Dispatching: `dispatch/dispatchIf/dispatchUnless`, `delay()` (+`withoutDelay`), `afterResponse()`, `dispatchSync`, dispatch-time `onQueue/onConnection` | CORE | queues.md |
| JOB-9 | Bulk dispatch `Bus::bulk` (grouped pushes, no batch tracking) | CORE | queues.md |
| JOB-10 | Transaction safety: `after_commit` connection option, per-dispatch `afterCommit/beforeCommit` | CORE | queues.md |
| JOB-11 | Job chaining: `Bus::chain` sequential execution with `catch`, in-job `prependToChain/appendToChain`, chain-wide connection/queue | CORE | queues.md |
| JOB-12 | Queue routing: `Queue::route(Job::class, connection:, queue:)` central default routing per job class | CORE | queues.md (new in 13) |
| JOB-13 | Retry/timeout controls: `$tries`/`#[Tries]`, `retryUntil`, `$maxExceptions`, `$timeout`/`#[Timeout]`, `#[FailOnTimeout]`, `backoff` (fixed/array/`#[Backoff]`), worker-level flags | CORE | queues.md |
| JOB-14 | SQS FIFO & fair queues: `onGroup` message-group ID, deduplication (`withDeduplicator`, custom deduplicators) | CORE | queues.md |
| JOB-15 | Manual control inside jobs: `release($delay)`, `fail($e)`, `delete()` | CORE | queues.md |
| JOB-16 | Job batching: `Bus::batch` with `before/progress/then/catch/finally`, named batches, `allowFailures`, cancellation checks, add-jobs-from-within, chains inside batches & batches inside chains, `queue:prune-batches`, DynamoDB batch storage | CORE | queues.md |
| JOB-17 | Queueable closures with `->name()->catch()` | CORE | queues.md |
| JOB-18 | Workers: `queue:work` daemon (`--queue` priority lists, `--once`, `--max-jobs`, `--max-time`, `--stop-when-empty`, `--sleep`, `--rest`, `--memory`, `--timeout`, `--tries`, `--backoff`), `queue:listen` auto-reload dev mode | CORE | queues.md |
| JOB-19 | Worker operations: `queue:restart` graceful deploy restarts, signal reactions (custom SIGUSR1/SIGTERM handlers), `queue:pause`/`queue:resume` per connection+queue | CORE | queues.md |
| JOB-20 | Supervisor process-manager configuration recipe | DIY | queues.md |
| JOB-21 | Failed jobs: `failed_jobs` table, `failed()` cleanup hook, `queue:failed/retry/forget/flush/prune-failed`, retry-all or by-queue/UUID, `ignoreMissingModels`, DynamoDB storage, disable storage, `JobFailed` event | CORE | queues.md |
| JOB-22 | `queue:clear` and `queue:monitor` (size thresholds → `QueueBusy` event for alerting) | CORE | queues.md |
| JOB-23 | Queue testing: `Queue::fake` (+subset fakes), `Bus::fake`, chain/batch assertions (`assertChained/assertBatched`), `withFakeQueueInteractions` unit-style job tests | CORE | queues.md |
| JOB-24 | Job lifecycle events: `Queue::before/after/looping`, `JobQueued/JobProcessing/JobProcessed/JobReleasedAfterException/JobAttempted` | CORE | queues.md |
| JOB-25 | Horizon: Redis queue dashboard + supervisor config-as-code (`config/horizon.php` environments/supervisors), auto/simple/no balancing with scaling cooldowns, wildcard supervisors | OPT | horizon.md |
| JOB-26 | Horizon operations: job tags (auto-tagged by Eloquent models), metrics snapshots (`horizon:snapshot`), long-wait notifications (mail/Slack/SMS), silenced jobs, `horizon:pause/continue/terminate`, dark mode dashboard, failed-job retention | OPT | horizon.md |
| JOB-27 | Scheduler: code-defined cron in `routes/console.php` — schedule Artisan commands, queued jobs (queue/connection), shell commands, invokables | CORE | scheduling.md |
| JOB-28 | Schedule frequency DSL: `everySecond`…`yearly`, `cron()`, day/time constraints (`between/unlessBetween/days/weekdays/environments/when/skip`), timezones (+app-wide schedule timezone) | CORE | scheduling.md |
| JOB-29 | Scheduler coordination: `withoutOverlapping` (lock), `onOneServer` (+named one-server jobs) for multi-server, `runInBackground` parallelism, maintenance-mode opt-in, schedule groups | CORE | scheduling.md |
| JOB-30 | Running: single cron entry → `schedule:run`, sub-minute tasks + `schedule:interrupt`, local `schedule:work`, task output capture (`sendOutputTo/emailOutputOnFailure`), hooks (`before/after/onSuccess/onFailure`), HTTP pings (`pingOnSuccess` etc.), scheduler events | CORE | scheduling.md |
| JOB-31 | Concurrency facade: `Concurrency::run/defer` closures in parallel child processes (process/fork/sync drivers, named results, timeouts) | CORE | concurrency.md |

## P10 — Real-time: websockets & server push

**Problem.** Push server events to connected browsers over websockets, with per-user authorization and presence. **Answer.** Event broadcasting: server events implement `ShouldBroadcast` and flow through a driver (first-party Reverb websocket server, or Pusher/Ably) to the Echo JS client; SSE streaming is the core-only fallback.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| LIVE-1 | Broadcasting architecture: `ShouldBroadcast` events on public/private/presence channels, drivers reverb/pusher/ably/log/null, `install:broadcasting` scaffold | CORE | broadcasting.md (driver server/service is separate) |
| LIVE-2 | Reverb: first-party Pusher-protocol websocket server — app credentials, allowed origins, multi-app, SSL termination, `reverb:start`, debug/restart commands | OPT | reverb.md |
| LIVE-3 | Reverb production scaling: event-loop selection (ext-uv), open-file/port tuning, nginx proxy recipe, horizontal scaling via Redis pub/sub, Pulse monitoring integration | OPT | reverb.md |
| LIVE-4 | Echo JS client: subscribe/listen (`channel/private/presence`, `listen/stopListening/leave`), namespace handling, error hooks | CORE (npm `laravel-echo`) | broadcasting.md |
| LIVE-5 | First-party React/Vue hooks: `useEcho/useEchoPresence/useEchoModel` with auto channel cleanup | CORE (npm) | broadcasting.md |
| LIVE-6 | Channel authorization: closures or channel classes in `routes/channels.php`, guard selection, model binding in channel names | CORE | broadcasting.md |
| LIVE-7 | Broadcast event shaping: `broadcastOn/broadcastAs/broadcastWith`, queue selection (`broadcastQueue`/`ShouldBroadcastNow`), conditional `broadcastWhen` | CORE | broadcasting.md |
| LIVE-8 | `toOthers()` (exclude current socket via `X-Socket-ID`), `via()` connection selection, `Broadcast::on` anonymous events (no event class), `rescue` failure tolerance | CORE | broadcasting.md |
| LIVE-9 | Presence channels: membership payloads, `here/joining/leaving` client callbacks, broadcast-to-presence | CORE | broadcasting.md |
| LIVE-10 | Model broadcasting: `BroadcastsEvents` trait auto-broadcasts create/update/delete on convention channels, `broadcastOn($event)` control, Echo `.listen('.UserUpdated')` conventions | CORE | broadcasting.md |
| LIVE-11 | Client-to-client whisper events (no server round-trip) | CORE | broadcasting.md |
| LIVE-12 | Broadcast notifications channel (notification → websocket, `Echo.notification()`) | CORE | broadcasting.md, notifications.md |
| LIVE-13 | Pusher Channels / Ably as hosted alternatives (config recipes, Ably-native vs compat mode) | ECO | broadcasting.md |
| LIVE-14 | Server-sent events without websockets: `eventStream` SSE responses + `useEventStream` (see CTRL-21) | CORE | responses.md |

## P11 — Mail & notifications

**Problem.** Send templated transactional email across providers, and fan out user-facing messages to many channels (email, SMS, chat, in-app). **Answer.** Mailable classes with a fluent envelope/content/attachment API over Symfony Mailer transports, and a parallel Notification abstraction that delivers one message class via mail/database/broadcast/SMS/Slack channels.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MAIL-1 | Mail transports: SMTP, SES, Postmark, Resend, MailerSend, Mailgun, sendmail, log, array; per-mailer config | CORE | mail.md (API transports need vendor SDK composer packages) |
| MAIL-2 | Transport resilience: `failover` mailer and `roundrobin` load distribution | CORE | mail.md |
| MAIL-3 | Mailables: `make:mail` with `envelope()` (from/replyTo/subject/tags/metadata/`using` Symfony hook), `content()` (Blade view/text/`htmlString`), public-prop or `with` view data | CORE | mail.md |
| MAIL-4 | Attachments: path/storage-disk/raw-data with name+mime, inline embeds (`$message->embed`), `Attachable` objects on domain models | CORE | mail.md |
| MAIL-5 | Markdown mailables: pre-styled responsive components (`<x-mail::message/button/panel/table>`), publishable theme/CSS customization | CORE | mail.md |
| MAIL-6 | Sending: `Mail::to()->cc()->bcc()->send()`, collection recipients, per-mailer send, `queue()`/`later()` (+`ShouldQueue` mailables, afterCommit), `Mail::send` closure API | CORE | mail.md |
| MAIL-7 | Rendering/previews: `render()` to string, return mailable from route for browser preview | CORE | mail.md |
| MAIL-8 | Localized sending (`locale()`, user `HasLocalePreference`) | CORE | mail.md |
| MAIL-9 | Mail testing: content assertions (`assertSeeInHtml`, `assertHasAttachment`…), `Mail::fake` send/queue assertions with closures | CORE | mail.md |
| MAIL-10 | Local dev: log mailer, Mailpit recipe, universal `to` override | CORE | mail.md |
| MAIL-11 | Extensibility: `Mail::extend` custom transports, extra Symfony transports (e.g. Brevo), MessageSending/Sent events | CORE | mail.md |
| MAIL-12 | Notifications: one class, many channels — `via()` returns mail/database/broadcast/vonage/slack per notifiable; `Notifiable` trait or `Notification::send/sendNow` facade | CORE | notifications.md |
| MAIL-13 | On-demand notifications to non-users: `Notification::route('mail', ...)->notify()` | CORE | notifications.md |
| MAIL-14 | Queued notifications: per-channel queue/connection/delay maps, afterCommit, `shouldSend` last-second cancellation | CORE | notifications.md |
| MAIL-15 | Mail channel: fluent `MailMessage` (lines/actions/greeting), error styling, sender/recipient/subject/mailer overrides, template publish, attachments, tags/metadata, full Mailable substitution, browser preview | CORE | notifications.md |
| MAIL-16 | Database channel: `notifications` table (`make:notifications-table`), `toDatabase/toArray`, `notifications/unreadNotifications` relations, mark-as-read APIs | CORE | notifications.md |
| MAIL-17 | SMS via Vonage (`laravel/vonage-notification-channel`) with unicode, from override, client-ref, `routeNotificationForVonage` | OPT | notifications.md |
| MAIL-18 | Slack channel: Block Kit builder API, interactivity/confirm dialogs, block-builder template dumps, external-workspace routing via Slack routes | OPT | notifications.md (`laravel/slack-notification-channel`) |
| MAIL-19 | Notification localization, `Notification::fake` testing assertions, sending/sent events, custom channel classes, community channel matrix | CORE/ECO | notifications.md (community channels site) |

## P12 — Caching & performance

**Problem.** Avoid recomputing/refetching expensive data, coordinate concurrent work, and squeeze more requests out of a PHP runtime. **Answer.** A unified multi-store cache API with atomic locks and stampede protection, cache-backed rate limiting, and Octane for resident-process application serving.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CACHE-1 | Cache stores behind one API: redis, memcached, database, file, dynamodb, mongodb, array, null; default + `Cache::store()` selection | CORE | cache.md |
| CACHE-2 | Failover cache store: ordered store list with automatic fallback | CORE | cache.md (new in 13) |
| CACHE-3 | Read/write API: `get` (closure defaults), `many`, `pull`, `put/add/forever`, TTL as seconds/DateTime, `increment/decrement`, `forget/flush` | CORE | cache.md |
| CACHE-4 | Compute-through helpers: `remember/rememberForever` and `flexible([$fresh, $ttl])` stale-while-revalidate (deferred background refresh) | CORE | cache.md |
| CACHE-5 | `Cache::touch` TTL extension without value round-trip | CORE | cache.md (new in 13) |
| CACHE-6 | Cache memoization: `Cache::memo()` in-request memory layer over any store, invalidated by writes | CORE | cache.md |
| CACHE-7 | Cache tags: tag-scoped write/read/flush on redis/memcached/dynamodb/array | CORE | cache.md |
| CACHE-8 | Atomic locks: `Cache::lock($name, $ttl)->get/block/release`, owner-token passing across processes (`restoreLock`), forceRelease | CORE | cache.md |
| CACHE-9 | Concurrency limiting on locks: `Cache::withoutOverlapping` and funnel-style concurrent-slot limiting | CORE | cache.md |
| CACHE-10 | `cache()` helper, custom driver registration (`Cache::extend`), cache events (hit/miss/write/forget, disable per store) | CORE | cache.md |
| CACHE-11 | General-purpose `RateLimiter` facade: `attempt/tooManyAttempts/hit/increment/remaining/availableIn/clear`, cache-store selection | CORE | rate-limiting.md |
| CACHE-12 | Octane: serve the booted app from resident workers on FrankenPHP / Swoole / RoadRunner — `octane:start`, HTTPS, nginx recipe, `--watch`, worker/max-request tuning, graceful `octane:reload` | OPT | octane.md |
| CACHE-13 | Octane long-lived-state guidance: container/request/config injection pitfalls, memory-leak management | OPT | octane.md |
| CACHE-14 | Octane (Swoole) extras: `Octane::concurrently` task parallelism, `tick/interval` timers, ultra-fast octane cache with interval caches, shared-memory `Octane::table` | OPT | octane.md |
| CACHE-15 | Deploy-time caches: config/event/route/view caching via `optimize` (see CONF-9) | CORE | deployment.md |

## P13 — Files & storage

**Problem.** Store and serve user files across local disk and cloud object stores with one API. **Answer.** The `Storage` facade over Flysystem: named disks (local/public/S3/FTP/SFTP), uniform read/write/URL/metadata operations, streamed uploads, and fakeable tests.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| FILE-1 | Disk abstraction: local, public (symlinked `storage:link`), s3 (+any S3-compatible: MinIO, R2, DO Spaces), config-driven | CORE | filesystem.md (S3 needs flysystem-aws adapter package) |
| FILE-2 | FTP and SFTP drivers (flysystem adapter packages), incl. SFTP key auth + permission maps | CORE | filesystem.md |
| FILE-3 | Scoped (path-prefixed) and read-only disks via adapter packages | CORE | filesystem.md |
| FILE-4 | On-demand disks: `Storage::build([...])` from inline config | CORE | filesystem.md |
| FILE-5 | Reads: `get/json/exists/missing`, streamed `download` responses, `url()` (customizable host), `temporaryUrl` (S3/local, request params, URL customization), S3 `temporaryUploadUrl` for client-direct uploads | CORE | filesystem.md |
| FILE-6 | Metadata: `size/lastModified/mimeType/checksum/path` | CORE | filesystem.md |
| FILE-7 | Writes: `put`, automatic stream detection, `putFile(As)` managed uploads with unique hashing, `prepend/append`, `copy/move`, failure-as-boolean (+throw option) | CORE | filesystem.md |
| FILE-8 | Upload ergonomics: `$file->store(As)/storePublicly`, extension/hashName helpers | CORE | filesystem.md, requests.md |
| FILE-9 | Visibility (public/private) at write or via `setVisibility`, per-driver permission mapping | CORE | filesystem.md |
| FILE-10 | Delete + directory ops: `delete`, `files/allFiles/directories/allDirectories`, `makeDirectory/deleteDirectory` | CORE | filesystem.md |
| FILE-11 | Testing: `Storage::fake` with existence/count assertions, `UploadedFile::fake()->image()/create()` | CORE | filesystem.md |
| FILE-12 | Custom filesystems: `Storage::extend` wrapping any Flysystem adapter (e.g. Dropbox) | CORE | filesystem.md |

## P14 — Building APIs: serialization & content negotiation

**Problem.** Serve JSON APIs: transform models into stable payloads, paginate, version envelope shape, and authenticate token clients. **Answer.** Eloquent API Resources for controlled transformation (plus first-party JSON:API resources as of 13), automatic 422/JSON error behavior, and Sanctum/Passport for auth (see P7).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| API-1 | API route scaffold: `install:api` — stateless `routes/api.php`, `/api` prefix, throttle group, Sanctum wiring | CORE | routing.md, sanctum.md |
| API-2 | JsonResource classes: `make:resource`, `toArray` transformation with request access, `$this` model proxying | CORE | eloquent-resources.md |
| API-3 | Resource collections: `Resource::collection()` and dedicated `ResourceCollection` classes with meta, `preserveKeys`, custom underlying resource (`$collects`) | CORE | eloquent-resources.md |
| API-4 | Data envelope control: `data` wrapping (disable via `withoutWrapping`), nested-wrapping rules, pagination-aware wrapping with `links`/`meta` blocks (customizable via `paginationInformation`) | CORE | eloquent-resources.md |
| API-5 | Conditional payload shaping: `when/whenHas/whenNotNull/whenAppended/mergeWhen`, relationship-aware `whenLoaded/whenCounted/whenAggregated/whenPivotLoaded` | CORE | eloquent-resources.md |
| API-6 | Top-level meta (`with()`) and per-response `additional()`; response tuning via `->response()->header()` and `withResponse` | CORE | eloquent-resources.md |
| API-7 | JSON:API resources: `JsonApiResource` — spec-compliant `type/id/attributes/relationships`, compound documents with `?include=`, sparse fieldsets `?fields[type]=`, links & meta at all levels, JSON:API content-type headers | CORE | eloquent-resources.md (new in 13) |
| API-8 | JSON error conventions: validation 422 `{message, errors}`, `expectsJson`-aware exception rendering, `abort()` HTTP exceptions | CORE | validation.md, errors.md |
| API-9 | Fast model serialization fallback: return models/collections directly (`toJson`, hidden/appends control) — see ORM-54 | CORE | eloquent-serialization.md |
| API-10 | Token auth for APIs: Sanctum (AUTH-22/23) and Passport OAuth2 (AUTH-24/25) | OPT | sanctum.md, passport.md |

## P15 — Internationalization & localization

**Problem.** Serve the app in multiple human languages with parameterized, pluralized strings. **Answer.** File-based translations (PHP key arrays + JSON source-string files) resolved by `__()`, with locale/fallback config; comparatively thin versus other subsystems.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| I18N-1 | Translation files: `lang/{locale}/*.php` short-key arrays and `lang/{locale}.json` source-string maps; scaffolded by `lang:publish` | CORE | localization.md |
| I18N-2 | Locale config: `APP_LOCALE` + fallback locale, runtime `App::setLocale`, `currentLocale/isLocale` | CORE | localization.md |
| I18N-3 | Retrieval: `__()` helper (dot-notation or literal source string), Blade `{{ __(...) }}` | CORE | localization.md |
| I18N-4 | Parameter replacement `:name` with case-mirroring capitalization; object-parameter formatting hooks (`Lang::stringable`) | CORE | localization.md |
| I18N-5 | Pluralization: `trans_choice` with `|` alternatives, exact `{1}` / range `[2,*]` rules, count placeholders; Eloquent pluralizer language override for non-English model naming | CORE | localization.md |
| I18N-6 | Override package translations via `lang/vendor/{package}` | CORE | localization.md |
| I18N-7 | Localized delivery: mailable/notification `locale()` + `HasLocalePreference` (MAIL-8/19); localized resource-controller verbs (CTRL-5) | CORE | mail.md, controllers.md |
| I18N-8 | Locale-aware formatting utilities: `Number::useLocale/withLocale`, currency/file-size/human-readable formatting; Carbon date localization | CORE | helpers.md |

## P16 — Testing support

**Problem.** Test the full stack — HTTP flows, console, DB state, browser UI — quickly and without hitting real external services. **Answer.** Pest/PHPUnit scaffolding with an unusually rich fake ecosystem: every facade subsystem ships `::fake()` + assertions, databases reset per test, HTTP tests run in-process, and Dusk drives a real browser.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| TEST-1 | Test scaffolding: Pest (default) or PHPUnit, `tests/Feature` + `tests/Unit`, `make:test`, `artisan test` runner (filters, `--compact`) | CORE | testing.md |
| TEST-2 | Test environment: automatic `testing` env, array/sync drivers, `.env.testing`, `phpunit.xml` vars, config-cache warning | CORE | testing.md |
| TEST-3 | Parallel testing (`--processes`) with per-process test databases + `ParallelTesting` setup hooks; coverage (`--coverage`, `--min`) and profiling (`--profile`) | CORE | testing.md |
| TEST-4 | In-process HTTP testing: `get/post/put/patch/delete/json`, header/cookie customization, session seeding, `actingAs` (+guard), `flushHeaders` | CORE | http-tests.md |
| TEST-5 | Response debugging & exception control: `dd/dump/dumpHeaders/dumpSession`, `Exceptions::fake` + `assertReported/assertNotReported`, `withoutExceptionHandling/withoutDeprecationHandling` | CORE | http-tests.md |
| TEST-6 | Huge response assertion vocabulary (100+): status family, redirects (`assertRedirect(ToRoute/ToSignedRoute)`), content (`assertSee(InOrder)/assertDontSee`), JSON (`assertJson/assertExactJson/assertJsonPath/assertJsonValidationErrors/assertJsonStructure/assertJsonApi`…), views (`assertViewIs/assertViewHas`), session/cookies/streaming | CORE | http-tests.md |
| TEST-7 | Fluent JSON assertions: `AssertableJson` `has/hasAll/where/whereType/missing/etc(fn)`, scoped collections (`first/each`) | CORE | http-tests.md |
| TEST-8 | File-upload testing: `UploadedFile::fake()->image/create(size, mime)` with `Storage::fake` | CORE | http-tests.md |
| TEST-9 | View testing without HTTP: `$this->view()`, `blade()`, `component()` render + `assertSee` | CORE | http-tests.md |
| TEST-10 | Console testing: `artisan()->expectsQuestion/expectsChoice/expectsSearch/expectsOutput/expectsTable/doesntExpectOutput/assertExitCode/assertSuccessful`, Prompts-aware | CORE | console-tests.md |
| TEST-11 | Database testing: `RefreshDatabase` (transaction rollback), `DatabaseMigrations`, `DatabaseTruncation`, factory usage, `seed()` per test/class | CORE | database-testing.md |
| TEST-12 | Database assertions: `assertDatabaseHas/Missing/Count/Empty`, `assertSoftDeleted/NotSoftDeleted`, `assertModelExists/Missing`, `expectsDatabaseQueryCount` | CORE | database-testing.md |
| TEST-13 | Mockery integration: `mock/partialMock/spy` container-bound, facade mocking (`Facade::shouldReceive`), facade spies | CORE | mocking.md |
| TEST-14 | Time control: `travel()->to/back`, `freezeTime/freezeSecond` for clock-dependent code | CORE | mocking.md |
| TEST-15 | Subsystem fakes across the framework: `Event/Mail/Notification/Queue/Bus/Storage/Http/Process/Sleep/Pennant/AI` fakes with dedicated assertions (rows in their sections) | CORE | mocking.md + per-subsystem docs |
| TEST-16 | Dusk browser tests: ChromeDriver-managed real-browser testing — element interaction DSL (forms, keyboard, mouse, iframes, JS dialogs), `dusk` selectors, waiting API (`waitFor/whenAvailable/waitForRoute…`), screenshots/console/source capture, auth helpers, cookie control | OPT | dusk.md |
| TEST-17 | Dusk structure & CI: Page objects (shorthand selectors, methods), reusable Components, database truncation guidance, CI recipes (GitHub Actions, Heroku, Travis, Chipper) | OPT | dusk.md |
| TEST-18 | HTTP client testing: `Http::fake` (URL maps, sequences, callables), `preventStrayRequests`, `Http::record` + `assertSent/assertSentCount` inspection | CORE | http-client.md |

## P17 — CLI, code generation & developer experience

**Problem.** Give developers a productive command-line surface: scaffolding, introspection, custom commands, and beautiful prompts. **Answer.** Artisan — one binary exposing ~all framework operations plus a `make:*` generator family — Tinker REPL, the Prompts TUI library, and starter kits for whole-app scaffolding.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CLI-1 | Artisan console: `list`, `help`, `--env`, global verbosity; `php artisan about` environment overview (extensible by packages) | CORE | artisan.md, configuration.md |
| CLI-2 | Tinker REPL (psysh): interactive Eloquent/jobs/events, allow-list config for commands/casting | CORE | artisan.md (`laravel/tinker`, included by default) |
| CLI-3 | `make:*` generator family: model (`-mfsc`, `--all`, `--pivot`), controller (resource/api/singleton/invokable), migration, seeder, factory, request, middleware, policy, provider, event, listener, job, mail, notification, resource, rule, cast, channel, command, component, observer, scope, test, view… | CORE | throughout corpus |
| CLI-4 | Custom commands: `make:command`, `$signature` DSL (arguments/options/arrays, optional/default/shortcut, descriptions), `handle()` with DI | CORE | artisan.md |
| CLI-5 | Closure commands in `routes/console.php` with `purpose()` | CORE | artisan.md |
| CLI-6 | Isolatable commands: `--isolated` cache-lock single execution with custom lock id/TTL/exit code | CORE | artisan.md |
| CLI-7 | `PromptsForMissingInput` interface auto-prompts required args; customizable questions and follow-up | CORE | artisan.md |
| CLI-8 | Command I/O: `argument/option`, interactive `ask/secret/confirm/anticipate/choice` (validation, multiselect), output `info/error/warn/line/newLine/table/withProgressBar`, verbosity levels | CORE | artisan.md |
| CLI-9 | Laravel Prompts TUI: browser-quality prompts — text/textarea/number/password/confirm/select/multiselect/suggest/search/multisearch/pause/autocomplete with placeholder, default, required, validation (incl. validator-rule syntax), transform; graceful Windows/unsupported fallbacks | CORE | prompts.md (bundled; also standalone package) |
| CLI-10 | Prompts display toolkit: `form()` multi-step (revertable), info/warning/error/alert/table/spin(ner)/progress/task/stream, terminal title/clear; testable via `expects*` | CORE | prompts.md |
| CLI-11 | Programmatic execution: `Artisan::call/queue` (array or string args), exit codes + output capture, `$this->call/callSilently` between commands | CORE | artisan.md |
| CLI-12 | Signal handling (`trap`) and stub customization (`stub:publish` to override generator templates) | CORE | artisan.md |
| CLI-13 | Console events (CommandStarting/Finished, ArtisanStarting; `ScheduledTask*` events) | CORE | artisan.md, scheduling.md |
| CLI-14 | Pint: zero-config opinionated code style fixer (PHP-CS-Fixer based) — presets (laravel/psr12/symfony/empty), per-rule config, path exclusion, `--test`/`--diff` CI modes, `--parallel` | OPT | pint.md (bundled with new apps) |
| CLI-15 | App scaffolding: `laravel new` installer with starter-kit picker (React/Vue/Svelte/Livewire/WorkOS/none), php.new one-liner PHP bootstrap, Laravel Herd local env, community starter kits via Composer/repo | CORE/ECO | installation.md, starter-kits.md |
| CLI-16 | IDE/DX support docs: Boost-powered editor/agent setup, PhpStorm/VS Code extension pointers | CORE | installation.md, ai.md |

## P18 — Configuration, environments & deployment

**Problem.** Configure per-environment behavior safely, and run the framework in production (and comfortable local dev). **Answer.** dotenv + plain PHP config files with aggressive production caching, `bootstrap/app.php` as the single wiring point, maintenance mode, and a suite of first-party environment tools (Sail, Valet, Homestead, Envoy) plus hosted platforms (Forge/Cloud).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CONF-1 | Environment config: `.env` + `.env.example`, `APP_ENV`-specific files, typed env parsing (bool/null/quoted), `env()` only inside config files | CORE | configuration.md |
| CONF-2 | Environment detection: `App::environment(...)`, `APP_ENV` override | CORE | configuration.md |
| CONF-3 | Encrypted env files: `env:encrypt` / `env:decrypt` (custom key/cipher, per-env files) for committing secrets safely | CORE | configuration.md |
| CONF-4 | Config access: `config()` / `Config` facade with dot notation, typed getters (`string/integer/array`…), runtime set | CORE | configuration.md |
| CONF-5 | Slim-by-default config: unpublished package configs with `config:publish`; cascading defaults | CORE | configuration.md |
| CONF-6 | Config caching `config:cache` (env() cut-off caveat), `optimize`/`optimize:clear` umbrella commands (config+events+routes+views) | CORE | configuration.md, deployment.md |
| CONF-7 | Debug mode `APP_DEBUG` with prod warning; error detail control | CORE | configuration.md |
| CONF-8 | Maintenance mode: `down` (custom refresh/retry/redirect, prerendered view, `--secret` bypass cookie, custom status), 503 handling, `up`; multi-server note (cache-based), queue interplay | CORE | configuration.md |
| CONF-9 | Deployment guidance: server requirements, nginx + FrankenPHP configs, directory permissions, optimization checklist | DIY | deployment.md |
| CONF-10 | Health endpoint: `/up` route dispatching `DiagnosingHealth` event for custom checks | CORE | deployment.md |
| CONF-11 | Single wiring point `bootstrap/app.php`: chained `withRouting/withMiddleware/withExceptions/withCommands/withProviders` app builder | CORE | structure.md, lifecycle.md |
| CONF-12 | Convention directory structure (documented app/ layout, generated-on-demand directories) | CORE | structure.md |
| CONF-13 | Sail: Docker-compose local env — PHP 8.0–8.5 images, MySQL, MongoDB, Redis, Valkey, Meilisearch, Typesense, MinIO, Mailpit, Selenium services; `sail` CLI passthrough (artisan/composer/node/tests), site sharing, Xdebug, customizable Dockerfiles | OPT | sail.md |
| CONF-14 | Valet: macOS minimal dev env — `park/link` site serving, `secure` TLS, per-site PHP versions/isolation, sharing, service proxying, custom drivers | OPT | valet.md |
| CONF-15 | Homestead: legacy Vagrant VM with pre-installed service menagerie, per-project or global, features/aliases/ports, Blackfire/Xdebug | OPT | homestead.md (legacy) |
| CONF-16 | Envoy: Blade-syntax SSH task runner — tasks, multi-server (+parallel), `@setup/@import`, stories, success/error/finished hooks, confirmations, Slack/Discord/Telegram/Teams notifications | OPT | envoy.md |
| CONF-17 | First-party hosting: Laravel Cloud (managed platform) and Forge (server management) as the blessed deployment story | ECO | deployment.md (paid services) |
| CONF-18 | Upgrade path docs + Shift/AI-assisted upgrades (`boost:upgrade` guidance) | DIY | upgrade.md |

## P19 — Extensibility: DI, events, hooks & packages

**Problem.** Let applications and third parties rewire, extend and package framework behavior without forking it. **Answer.** The service container is the universal seam (everything is resolvable/replaceable), service providers are the composition root, events decouple domains, facades/macros give ergonomic static surfaces, and a documented package protocol (discovery + publishable resources) powers a huge ecosystem.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| EXT-1 | Service container: zero-config reflection autowiring for concrete types | CORE | container.md |
| EXT-2 | Binding vocabulary: `bind/bindIf/singleton/scoped` (per request/job lifecycle), `instance`, interface→implementation | CORE | container.md |
| EXT-3 | Contextual binding (`when->needs->give(Tagged/Config)`) and primitive/variadic injection | CORE | container.md |
| EXT-4 | Contextual attributes: `#[Storage]/#[Auth]/#[Cache]/#[Config]/#[Context]/#[CurrentUser]/#[Database]/#[Give]/#[Log]/#[RouteParameter]/#[Tag]` + custom `ContextualAttribute` classes | CORE | container.md |
| EXT-5 | Tagging + `tagged()` resolution for plugin-style collections; `extend()` decoration of resolved services | CORE | container.md |
| EXT-6 | Resolution API: `make/makeWith`, `bound`, `App::call` method invocation with injection, resolving callbacks + `rebinding`, PSR-11 compliance | CORE | container.md |
| EXT-7 | Service providers: `register` (bindings only) / `boot` (post-registration, DI-capable), `booting/booted` callbacks, registration via `bootstrap/providers.php` | CORE | providers.md |
| EXT-8 | Deferred providers (`DeferrableProvider` + `provides()`) for lazy service registration | CORE | providers.md |
| EXT-9 | Facades: static proxies resolving container services (`getFacadeAccessor`), documented tradeoffs, full facade→service reference table | CORE | facades.md |
| EXT-10 | Real-time facades: `Facades\` namespace prefix turns any class into a facade on the fly | CORE | facades.md |
| EXT-11 | Contracts: interface package (`illuminate/contracts`) for every major component, enabling loose coupling in packages | CORE | contracts.md |
| EXT-12 | Events & listeners: `make:event/listener`, automatic listener discovery by type-hint (with caching), manual `Event::listen`, closure + wildcard listeners, event subscribers (many listeners, one class) | CORE | events.md |
| EXT-13 | Queued listeners: `ShouldQueue` with `viaConnection/viaQueue/withDelay`, conditional `shouldQueue`, queue interaction (release/delete), listener middleware, unique listeners, `failed()` + max attempts/backoff, `ShouldHandleEventsAfterCommit` | CORE | events.md |
| EXT-14 | Dispatching: `Event::dispatch(If/Unless)`, `ShouldDispatchAfterCommit`, `Event::defer()` block-scoped deferral of (model) events | CORE | events.md (`defer` new in 13) |
| EXT-15 | Event testing: `Event::fake(For)` scoped/subset fakes with dispatch assertions | CORE | events.md |
| EXT-16 | Macroable surfaces: `macro()` on Response, HTTP client, collections, strings, etc. — user-extendable core APIs | CORE | responses.md, http-client.md, collections.md, strings.md |
| EXT-17 | Package protocol: composer `extra.laravel.providers/aliases` auto-discovery (opt-out `dont-discover`), publishable groups (`vendor:publish --tag`) for config/migrations/views/lang/assets/routes | CORE | packages.md |
| EXT-18 | Package integration points: `mergeConfigFrom`, `loadRoutesFrom/loadMigrationsFrom/publishesMigrations/loadTranslationsFrom/loadViewsFrom` (+override cascade), Blade component/namespace registration, `AboutCommand::add`, `optimizes()` hooks, `reloads()` Octane hooks | CORE | packages.md |

## P20 — Observability: logging, metrics, errors

**Problem.** See what a production app is doing: structured logs, exception capture/reporting, request-scoped metadata, and performance/debug dashboards. **Answer.** Monolog-based channel logging plus a centralized exception handler in core; Telescope (local debugging), Pulse (production perf dashboard) and Pail (log tailing) as first-party add-ons. No built-in metrics/tracing exporters.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| OBS-1 | Log channels: single, daily (retention), slack (webhook, level-gated), syslog, errorlog, papertrail, monolog (any handler), custom via factory | CORE | logging.md |
| OBS-2 | Log stacks: one channel fanning out to many, per-channel levels | CORE | logging.md |
| OBS-3 | Writing: PSR-3 eight levels on `Log`, context arrays, request-scoped `withContext` / global `shareContext` (jobs inherit), `Log::channel/stack`, on-demand `Log::build` | CORE | logging.md |
| OBS-4 | Monolog customization: `tap` classes, handler `with` params, formatters, processors; deprecation-warning channel | CORE | logging.md |
| OBS-5 | Pail: real-time CLI log tailing across any driver, with filters (`--filter/--message/--level/--user`) | OPT | logging.md (`laravel/pail`) |
| OBS-6 | Exception pipeline: `report()` hooks by type, global context + per-exception `context()`, per-type log-levels map (`->level()`), ignore lists (`dontReport`, `ShouldntReport` interface, internal ignore removal), deduplication `dontReportDuplicates` | CORE | errors.md |
| OBS-7 | Exception rendering: `render()` per type (incl. overriding built-ins), JSON-vs-HTML `shouldRenderJsonWhen`, full `respond()` customization, self-contained `Reportable/Renderable` exceptions | CORE | errors.md |
| OBS-8 | Report throttling: `Lottery` sampling and `Limit` rate caps per exception type | CORE | errors.md |
| OBS-9 | HTTP errors: `abort()`, per-status custom error views (`errors/404.blade.php`, 4xx/5xx fallbacks, `vendor:publish` defaults) | CORE | errors.md |
| OBS-10 | Context: request/job-scoped metadata (`Context::add/get/pull/remember`, stacks, scope), hidden context, auto-propagation into every log entry and across queue job boundaries (dehydrate/hydrate events) | CORE | context.md |
| OBS-11 | Telescope: local-dev debug console — 19 watchers (requests, queries w/ slow flag, jobs, exceptions, cache, events, gates, HTTP client, logs, mail, models, notifications, redis, schedule, views, dumps…), tagging, filtering, pruning, gate-protected dashboard | OPT | telescope.md |
| OBS-12 | Pulse: production performance dashboard — usage top-users, slow queries/requests/jobs/outgoing-requests, cache stats, exceptions, queue throughput, server resource cards; layout/authorization customization | OPT | pulse.md |
| OBS-13 | Pulse scale controls: recorder config, sampling, trimming, dedicated DB, Redis-buffered ingest; custom cards (Livewire) with aggregation API | OPT | pulse.md |
| OBS-14 | DB/queue health probes: `db:monitor`, `queue:monitor` + events for alerting (see ORM-7, JOB-22) | CORE | database.md, queues.md |
| OBS-15 | Metrics/APM export (OpenTelemetry, StatsD, etc.) | — | Not provided; gap (see Non-goals) |

## P21 — Admin & operational UIs

**Problem.** Give operators visual surfaces for the running system, and admins CRUD over domain data. **Answer.** Laravel ships operational dashboards for its own subsystems (Horizon, Telescope, Pulse) but no domain admin/CRUD builder in the open-source line.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ADMIN-1 | Horizon dashboard: queue workload, throughput/runtime metrics, per-job/tag drill-down, failed-job inspection + retry — gate-protected at `/horizon` | OPT | horizon.md |
| ADMIN-2 | Telescope dashboard: entry browser across all watchers at `/telescope` (local-only install pattern) | OPT | telescope.md |
| ADMIN-3 | Pulse dashboard: `/pulse` production health cards, user-resolving, layout editing | OPT | pulse.md |
| ADMIN-4 | Dashboard authorization pattern: `viewHorizon/viewTelescope/viewPulse` gates for non-local access | OPT | horizon.md, telescope.md, pulse.md |
| ADMIN-5 | Domain CRUD admin generator | — | Not provided in OSS docs; commercial (Nova) / community (Filament) fill this — outside corpus |

## P22 — Search & retrieval

**Problem.** Let users find content by keyword or by meaning, without necessarily standing up external infrastructure. **Answer.** A layered story new in 13: database-native full-text search, pgvector semantic search + AI reranking in the query builder, and Scout for index-synced model search across database or hosted engines.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| SRCH-1 | Database full-text search: `fullText` indexes (+Postgres `language()`), `whereFullText/orWhereFullText` compiling to MATCH…AGAINST / tsvector per driver, composite-column search | CORE | search.md, queries.md, migrations.md |
| SRCH-2 | Semantic vector search: `vector()` columns + HNSW indexes, `Schema::ensureVectorExtensionExists`, `whereVectorSimilarTo` (cosine, threshold, auto-embeds plain-string queries), distance-level methods | CORE | search.md (needs pgvector or MongoDB + AI SDK) |
| SRCH-3 | Embedding generation: `Str::of(...)->toEmbeddings()`, batch `Embeddings::for([...])` | OPT | search.md, ai-sdk.md (`laravel/ai`) |
| SRCH-4 | AI reranking: `Reranking::of([...])->rerank($query)` and collection `->rerank(field, query)` macro; retrieve-then-rerank pattern | OPT | search.md, ai-sdk.md |
| SRCH-5 | Scout: `Searchable` trait auto-syncing indexes on model create/update/delete, queueable sync (+`afterCommit`), `toSearchableArray` shaping, `makeSearchableUsing` eager loads | OPT | scout.md |
| SRCH-6 | Scout engines: database (LIKE/full-text with `SearchUsingFullText`/`SearchUsingPrefix` attributes), collection (driver-agnostic brute force), Algolia, Meilisearch (index-settings sync incl. filterable/sortable), Typesense (explicit schema), null; custom engines | OPT/ECO | scout.md (hosted engines are third-party services) |
| SRCH-7 | Scout operations: batch `scout:import/flush/index/delete-index`, chunked import tuning, `withoutSyncingToSearch`, `shouldBeSearchable` conditions | OPT | scout.md |
| SRCH-8 | Scout querying: `Model::search()->where/whereIn/whereNotIn`, engine-native option passthrough + raw callback, `paginate`, soft-delete-aware indexing, `query()` Eloquent-constraint hook | OPT | scout.md |
| SRCH-9 | Technique-combining guidance (full-text → rerank, vector + keyword hybrid) | DIY | search.md |

## P23 — AI & agents

**Problem.** Build LLM-powered features (agents, embeddings, media generation) and make the app itself legible to coding agents. **Answer.** New first-party pillar in 13: the AI SDK (provider-agnostic agents/embeddings/media), Laravel MCP (serve and consume Model Context Protocol), and Boost (MCP server + guidelines that teach coding agents the app).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| AI-1 | AI SDK provider layer: one API over OpenAI, Anthropic, Gemini, Groq, xAI, Mistral, Ollama, OpenRouter, DeepSeek, Cohere, ElevenLabs, VoyageAI, Jina, Azure + any OpenAI-compatible endpoint; custom base URLs; per-capability default models | OPT | ai-sdk.md (`laravel/ai`, new in 13) |
| AI-2 | Agent classes: instructions + `prompt()`, provider/model selection, structured output schemas, attachments (images/documents), provider options | OPT | ai-sdk.md |
| AI-3 | Agent conversation persistence: `agent_conversations`/`agent_conversation_messages` tables, continue-conversation API | OPT | ai-sdk.md |
| AI-4 | Agent tools: PHP tool classes with DI + validation, file-storage tools, MCP-server tools, provider-hosted tools, sub-agent delegation (agents as tools), middleware, anonymous agents | OPT | ai-sdk.md |
| AI-5 | Agent delivery modes: streaming (SSE), broadcasting over websockets, queued agent runs | OPT | ai-sdk.md |
| AI-6 | Media generation: `Image::of()->generate()`, TTS `Audio::of()->generate()`, STT transcription | OPT | ai-sdk.md |
| AI-7 | Embeddings: generation (single/batch), caching, querying integration with vector columns (SRCH-2/3) | OPT | ai-sdk.md |
| AI-8 | Provider files & vector stores: upload files, attach to provider-hosted vector stores, use in conversations | OPT | ai-sdk.md |
| AI-9 | Provider failover chains for resilience; full fake/testing suite per capability; lifecycle events | OPT | ai-sdk.md |
| AI-10 | Laravel MCP servers: `Mcp::web` (HTTP routes) and `Mcp::local` (stdio) servers; tools with input/output JSON schemas, validation, annotations, conditional registration | OPT | mcp.md (`laravel/mcp`) |
| AI-11 | MCP prompts and resources: argument validation, DI, templates, URIs/MIME, annotations, response shaping | OPT | mcp.md |
| AI-12 | MCP Apps: tools rendering sandboxed interactive HTML via `#[RendersApp]` app resources | OPT | mcp.md |
| AI-13 | MCP auth & testing: OAuth 2.1 via Passport or Sanctum tokens, per-primitive authorization, MCP Inspector + unit-test helpers | OPT | mcp.md |
| AI-14 | MCP client: connect to external MCP servers (named clients, auth), call their tools/prompts/resources — also pluggable into AI-SDK agents | OPT | mcp.md |
| AI-15 | Boost: dev-time MCP server (app info, DB schema/queries, log + browser-log readers, URL helper, hosted docs search), version-aware AI guidelines, composable agent skills, docs API, IDE/agent onboarding (`boost:install`), extension points | OPT | boost.md, ai.md |

## P24 — Payments & billing

**Problem.** Subscription/payment billing has brutal edge cases (webhooks, dunning, SCA, proration). **Answer.** Cashier: first-party expressive wrappers over Stripe and Paddle billing.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| BILL-1 | Cashier Stripe: `Billable` trait — customer sync, balances, tax IDs, billing portal redirect | OPT | billing.md |
| BILL-2 | Payment methods: SetupIntents, store/list/default/delete cards & other types | OPT | billing.md |
| BILL-3 | Subscriptions: create with trials/quantities/coupons, rich status checks (+query scopes), price swaps with proration control, multi-product subscriptions, multiple concurrent subscriptions, usage-based metered billing, anchor dates, cancel (at period end / now) and resume | OPT | billing.md |
| BILL-4 | Stripe Checkout: product/price/subscription sessions, promo codes, tax-ID collection, guest checkout | OPT | billing.md |
| BILL-5 | One-off charges: `charge/pay` with payment intents, invoiced charges, refunds; invoice listing/preview/PDF generation (customizable renderer) | OPT | billing.md |
| BILL-6 | Webhook handling: built-in controller for lifecycle events (cancellations, updates), custom handlers via events, signature verification, CSRF exclusion | OPT | billing.md |
| BILL-7 | SCA/3DS: incomplete/past_due states, payment-confirmation page, off-session confirmation notifications; Stripe SDK escape hatch; automatic tax via Stripe Tax | OPT | billing.md |
| BILL-8 | Cashier Paddle: overlay + inline checkout sessions, price previews, subscriptions (pause, plan changes, multi-product), trials, transactions/refunds/credits, webhooks — for Paddle's merchant-of-record model | OPT | cashier-paddle.md |

## P25 — Utility toolkit: collections, strings, HTTP client & helpers

**Problem.** Everyday data-manipulation and integration chores shouldn't require ad-hoc code or third-party libs. **Answer.** A large standard library: Collections, lazy collections, string fluency, an outbound HTTP client, process execution, and dozens of focused helpers — all macroable and test-fakeable.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| UTIL-1 | Collections: `collect()` with ~200 chainable methods (map/filter/reduce, groupBy, pluck, sort family, sole, partition, sliding, zip…), macro-extendable | CORE | collections.md |
| UTIL-2 | Higher-order messages: `$collection->map->name` proxy shorthand | CORE | collections.md |
| UTIL-3 | LazyCollection: generator-backed, constant-memory pipelines (`takeUntilTimeout/remember/tapEach`), Enumerable contract shared with Collection | CORE | collections.md |
| UTIL-4 | String toolkit: 100+ `Str::` helpers (slug, uuid/ulid/uuid7, markdown→HTML, mask, squish, password, plural/singular, case conversions, excerpt, inline-markdown) and chainable `Str::of()` fluent strings | CORE | strings.md |
| UTIL-5 | Array/object helpers: `Arr::` suite + dot-notation `data_get/data_set/data_forget/data_fill` | CORE | helpers.md |
| UTIL-6 | `Number` class: locale-aware format/spell/ordinal, currency, file size, human-readable abbreviation, clamp, pairs | CORE | helpers.md |
| UTIL-7 | Misc helpers: `abort(_if/_unless)`, `dd/dump`, `retry` (backoff, when), `rescue` (report + default), `throw_if/unless`, `tap`, `once` (memoize), `value/transform/blank/filled`, `optional`, `cache/config/session/cookie/old/request/response` accessors | CORE | helpers.md |
| UTIL-8 | Focused utilities: `Benchmark` (timing + value), Carbon date library integration (`now()`, `travel` in tests), `defer()` post-response execution (`always()`, named cancel), `Lottery` probabilistic callbacks, `Pipeline` middleware-style value passing, `Sleep` fluent + `Sleep::fake` with assertions, `Timebox` fixed-duration execution (timing-attack defense), `Uri` fluent builder | CORE | helpers.md |
| UTIL-9 | HTTP client (Guzzle wrapper): `Http::get/post/...` with query/form/multipart/JSON bodies, fluent response object (json/object/collect, status predicates), URI templates, `dump/dd` | CORE | http-client.md |
| UTIL-10 | HTTP client resilience & control: basic/digest/bearer auth, timeout/connectTimeout, `retry` (backoff array, `when`, per-request throw control), rich exception hierarchy + `throw(If)`, Guzzle middleware (request/response mapping) and raw options, global middleware/options | CORE | http-client.md |
| UTIL-11 | Concurrent outbound HTTP: `Http::pool` and `Http::batch` (completion callbacks: before/progress/then/catch/finally), client macros, request/response events | CORE | http-client.md (`batch` new in 13) |
| UTIL-12 | Process execution: `Process::run/start` (Symfony Process wrapper) — path/timeout/idle-timeout/env/tty/input options, output capture + real-time callbacks, quiet mode, `pipe` chains, async with PID/signals/waitUntil, `Process::pool/concurrently` pools | CORE | processes.md |
| UTIL-13 | Process testing: `Process::fake` (per-command results, `describe()` sequenced/async fakes with output/exit/iterations), `assertRan/assertDidntRun/assertRanTimes`, `preventStrayProcesses` | CORE | processes.md |

---

## Signature design decisions

The choices that make Laravel *Laravel*, as evidenced across the corpus:

1. **The service container as the universal seam.** Every subsystem is a container-bound service resolvable by type-hint anywhere the framework invokes user code (controllers, listeners, jobs, commands, even Blade via `@inject`). Extensibility is uniform: rebind or decorate the service. Zero-config reflection autowiring keeps the common case invisible (container.md).

2. **Facades + helpers as the ergonomic front door.** Rather than forcing constructor injection, Laravel exposes every service through static-looking facades (`Cache::`, `Gate::`, `Bus::`) and global helpers (`cache()`, `view()`), each still container-resolved, swappable and — crucially — *mockable* (`Cache::shouldReceive`, real-time facades). The docs openly discuss the tradeoff (facades.md). This is Laravel's most distinctive (and most polarizing) surface.

3. **Driver/Manager abstraction everywhere.** Cache, session, queue, mail, filesystem, broadcasting, hashing, auth guards, Scout engines, Pennant stores, logging channels — all follow the same pattern: a config-selected named driver behind a uniform API, plus an `extend()` hook for custom drivers. Laravel 13 doubled down by adding *failover* drivers (cache, queue, mail) that chain drivers for availability.

4. **Convention density with escape hatches.** Table names, foreign keys, pivot tables, route keys, policy discovery, event-listener discovery, channel names — all inferred from class names, all individually overridable. The framework optimizes for "the convention is the config" while never locking the door.

5. **One expressive fluent API per problem.** Query builder, validation rules, mailable envelopes, schedule frequencies, Prompt builders, Slack Block Kit — Laravel consistently answers a domain with a chainable DSL rather than config arrays, and lets applications extend those DSLs via `Macroable`.

6. **First-party everything.** Laravel deliberately covers the "second ring" — websocket server (Reverb), queue dashboard (Horizon), OAuth2 server (Passport), billing (Cashier), feature flags (Pennant), browser testing (Dusk), app server (Octane), search sync (Scout), and now AI (AI SDK/MCP/Boost) — as versioned, optional first-party packages rather than leaving them to the ecosystem. The docs read as one coherent product.

7. **Testability as a framework feature.** Every I/O subsystem ships a `::fake()` with purpose-built assertions (mail, notifications, events, queue/bus, storage, HTTP client, processes, sleep, AI). Time itself is fakeable. In-process HTTP testing plus `RefreshDatabase` makes full-stack tests the default workflow, with Pest as the blessed runner.

8. **Aggressive production compilation of a dynamic language.** Config, routes, events, views and Blade all compile to cached PHP artifacts (`optimize`); schema history squashes to a SQL dump; Octane keeps the booted framework resident. The dev-time dynamism is paid for once, at deploy.

9. **PHP attributes as the modern configuration channel (13.x direction).** `#[Middleware]`, `#[Authorize]`, `#[Tries]`, `#[Backoff]`, `#[Scope]`, `#[ObservedBy]`, `#[ScopedBy]`, `#[DebounceFor]`, `#[SearchUsingFullText]`, contextual DI attributes — colocating declarations with code, replacing registration boilerplate.

10. **Batteries for the request's whole lifetime.** The request path is instrumented end-to-end by first-party tooling: Precognition validates before submission, Context flows metadata into logs and queued jobs, `defer()` runs work after the response, terminable middleware after send, and Telescope/Pulse observe it all.

## Non-goals & gaps

What the corpus shows Laravel deliberately does not provide, or is comparatively weak at:

- **No domain admin/CRUD UI in OSS.** Operational dashboards exist (Horizon/Telescope/Pulse) but model admin is left to commercial Nova / community Filament — neither in the docs corpus.
- **No built-in metrics/tracing export.** Logging is rich, but there is no OpenTelemetry/StatsD/Prometheus story in core docs; Pulse is a self-contained dashboard, not an APM or metrics pipeline.
- **No API versioning, OpenAPI/schema generation, GraphQL.** API resources shape JSON (now incl. JSON:API), but spec generation, version negotiation and GraphQL are ecosystem territory.
- **Concurrency is process-based, not async.** PHP's model shows: "real-time" means queue workers, `Concurrency::run` child processes, or Octane workers — there is no in-process async/await, actor, or channel primitive; websockets require a separate Reverb process.
- **Frontend is delegated.** Beyond Blade + Vite glue, interactivity is explicitly outsourced to Livewire or Inertia+JS frameworks (frontend.md says so directly); no first-party reactive component model in core.
- **i18n is thin.** String tables with pluralization and parameter replacement — no message extraction tooling, ICU MessageFormat, locale-aware routing, or translation management workflow.
- **Migrations trust the author.** No migration linting, drift detection against live schema, or data-migration framework; irreversible migrations simply omit `down()`. Concurrency-safe deploys get only `--isolated`.
- **The database is not the integrity layer of last resort.** Validation is request-centric; docs lean on application-level rules (`unique` rule vs DB constraint races are acknowledged) rather than DB-enforced invariants.
- **No first-party full-text engine.** Scout's quality features (typo tolerance, facets, geo) require third-party services (Algolia/Meilisearch/Typesense); the database engine is explicitly "not scale-ready" for large tables.
- **Multi-tenancy, soft real-time presence scaling, and horizontal session affinity** are left to patterns/ecosystem — mentioned only implicitly (e.g. Reverb scaling via Redis, `onOneServer`).
- **Backwards-compat boundaries are explicit:** named arguments are not covered by BC; upgrade guides assume yearly majors with small deltas as the compatibility strategy.
