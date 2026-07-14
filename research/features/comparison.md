# Cross-framework feature comparison — Laravel · Rails · Phoenix · Django

This document is the synthesis layer of the Volt framework research. Volt is a planned Go web framework; before designing it, we inventoried four mature full-stack frameworks — **Laravel**, **Ruby on Rails**, **Phoenix**, and **Django** — against a shared skeleton of 21 problem areas (P1–P21, from routing through admin UIs). This file compares the four answers problem by problem, then closes with a synthesis: the table-stakes baseline, each framework's differentiators, the gaps none of them fill, and the design tensions Volt must resolve.

**How to read it.** Each `## PN` section carries a one-line portrait of each framework's approach, a capability matrix, the notable divergences, and Volt-specific notes. Matrix cells follow the format *mechanism · TIER · feature ID(s)*. Tiers are as recorded in the inventories: **CORE** (in the framework/default app), **OPT** (first-party or contrib, opt-in), **ECO** (blessed ecosystem), **DIY** (documented pattern you build yourself); `—` means that corpus has no answer. Feature IDs (ROUTE-9, ORM-25, AUTH-2, …) refer into the per-framework inventory documents, which hold the full detail:

- [laravel.md](./laravel.md)
- [rails.md](./rails.md)
- [phoenix.md](./phoenix.md)
- [django.md](./django.md)

**Corpus provenance.** Laravel 13.x, Rails 8.1.3, Phoenix 1.8.9 (+ Ecto 3.14.1, LiveView 1.2.7, Plug 1.20.3), Django 6.1.x — all from official documentation sources; exact commits are recorded in `../corpus/*/MANIFEST`. Framework-specific extra sections beyond P21 (Phoenix's BEAM runtime, Django's GeoDjango/PostgreSQL, Laravel's search/AI/billing) are folded into the nearest section or the closing synthesis where relevant.

## The four philosophies at a glance

| Framework | One-line philosophy | Runtime model | Data-layer stance | Frontend stance | Batteries stance |
|---|---|---|---|---|---|
| **Laravel** | A fluent DSL and a config-selected driver for every problem; convention density with escape hatches | PHP process-per-request; Octane resident workers as the opt-in escape | Maximal Active Record (Eloquent) plus a fluent SQL builder | Blade components + first-party Vite plugin; interactivity delegated to Livewire/Inertia | Maximal: first-party packages for the whole second ring (websockets, OAuth2, billing, search, AI), shading into paid products |
| **Rails** | Convention over configuration; omakase full stack — the database is the infrastructure | Threaded Rack behind the GVL; Executor-wrapped units of work; documented tuning playbook | Archetypal fat Active Record, schema-reflected, callbacks as architecture | HTML over the wire: Hotwire (Turbo + Stimulus) + importmaps, no Node build | Complete default meal including deployment (Solid trifecta, Kamal); every course can be sent back |
| **Phoenix** | Explicit pipelines of one immutable value; compile time is a feature; the BEAM replaces infrastructure | BEAM process per connection; supervision trees; native clustering | Ecto data mapper: Repo/Schema/Changeset, no lazy loading, database as the integrity layer | Compile-checked HEEx function components; LiveView server-rendered reactive UI | Deliberately thin: no jobs, cache, or storage in core; generators write owned code into your app |
| **Django** | Loose coupling + DRY: the model is the single source of truth; explicit over implicit, "no magic" | WSGI/ASGI dual stack; sync and async views both first-class | Declarative Active Record with lazy QuerySets; database-agnostic by charter | Deliberately restricted templates; staticfiles collect/hash only; JS toolchains out of scope | Encyclopedic contrib (admin, auth, GIS, i18n) but abstains from REST, real-time, and workers |

**Laravel** optimizes for a single coherent product experience. One pattern — a config-selected named driver behind a uniform fluent API, with an `extend()` hook — repeats across cache, queue, mail, storage, auth, broadcasting, and logging, and Laravel 13 extends it with failover drivers for availability. The service container is the universal seam, facades the ergonomic front door, and every I/O subsystem ships a `::fake()` for testing. Where the other three stop at the framework's edge, Laravel keeps shipping first-party packages outward — Reverb, Horizon, Passport, Cashier, Scout, Octane, now an AI/MCP pillar — trading ecosystem pluralism for a docs corpus that reads as one product. Its acknowledged weak flanks: thin i18n, no metrics story, request-centric validation that treats the database as secondary, and a process-per-request runtime it must sell Octane to escape.

**Rails** bets that naming conventions can replace wiring: `Book` → `books` → `BooksController` → `app/views/books/`, schema reflected rather than declared. The model object is deliberately fat — validations, callbacks, dirty tracking, encryption live on it — and controllers stay thin. Rails 8's sharpest position is infrastructural: the Solid trifecta (Queue/Cache/Cable) runs jobs, caching, and websockets on the application's own database, removing Redis from the default stack, while a generated Dockerfile, Kamal, and Thruster claim the deploy path all the way to a Linux box. Auth flipped in 8.0 from engine-magic to generated, owned code atop small primitives. What Rails refuses: authorization frameworks, DI containers, API serializers/versioning, presence, and metrics export — instrumentation events are emitted for others to consume.

**Phoenix** makes explicitness and the compiler its product. Everything HTTP is a plug over one immutable conn — reading the endpoint and router *is* reading the request path — and the same pipeline shape repeats in Ecto changesets and LiveView sockets. Routes, `~p` URLs, queries, and HEEx templates are all verified at compile time; unsafe SQL interpolation is a compile *error*. Generators scaffold whole vertical slices into your application and declare the output yours, with v1.8 Scopes threading an authorization context through everything future generators emit. The BEAM replaces a broker fleet: PubSub, CRDT presence, clustering, and supervision ship with zero external services, and LiveView makes server-rendered reactive UI the default paradigm. The trade: Phoenix deliberately ships no job queue, no cache framework, no storage abstraction, no admin — and generated code receives no automatic security patches.

**Django** is the oldest philosophy and the most explicitly argued: loose coupling (templates know nothing of requests, the ORM nothing of display), DRY (the model declaration is the single source from which forms, admin, migrations, and serialization derive), and "no magic" (`save()` never validates, fields never infer from names). Its two flagship consequences are the model admin — a production CRUD UI derived entirely from model metadata, unmatched by any peer — and derived migrations (`makemigrations` diffs model state). Contrib depth is encyclopedic (auth, gettext-based i18n with JS catalogs, a full timezone framework, GeoDjango) and API stability is a published promise. But the charter is equally explicit about refusals: database-agnostic core, no NoSQL, no websockets in core, a tasks framework that deliberately ships no worker, no REST framework, and total frontend-toolchain agnosticism.

---

## P1 — Routing & HTTP dispatch

- **Laravel** — fluent `Route` facade in convention route files (`routes/web.php`/`api.php`); implicit model binding is the signature convenience; throttling, CORS, signing hang directly off the router.
- **Rails** — Ruby DSL in `config/routes.rb` centered on RESTful `resources` minting 7 CRUD routes + `_path`/`_url` helpers; everything else layers onto that convention.
- **Phoenix** — compile-time macro router expanding to one pattern-matched function; verified `~p` path literals fail at build time; pipelines attach plug stacks to scopes.
- **Django** — URLconf: plain Python `urlpatterns` lists matched top-down on path only (never the verb); typed converters, `include()` nesting with namespaces, `reverse()` for generation.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Verb+path route declaration | `Route::get/post/…`, `match`/`any` · CORE · ROUTE-1 | `get/post/…` + `match … via:` · CORE · ROUTE-3 | verb macros compiled to one pattern-match · CORE · ROUTE-1 | `path()`/`re_path()`; **verb deliberately ignored** by router (dispatch in view) · CORE · ROUTE-1/3/14 |
| RESTful resource routing | `Route::resource/apiResource` 7/5 actions, only/except · CORE · CTRL-4 (P2) | `resources` DSL + singular `resource`, member/collection, deep customization (`only/as/param/path_names`) · CORE · ROUTE-1/4/6/18 | `resources` macro 8 actions, `:only/:except` · CORE · ROUTE-2/3 | — (per-URL CBVs instead) |
| Nested / shallow resources | nested + `shallow()`, scoped child bindings · CORE · CTRL-5 (P2) | ≤1-level nesting convention, `shallow:` · CORE · ROUTE-5 | nested `resources do … end` · CORE · ROUTE-4 | — (prefix nesting via `include()` only · ROUTE-9/10) |
| Route parameters | `{p}`, optional `{p?}`, injected by name · CORE · ROUTE-4 | `:id` segments, optional `(.:format)` · CORE · ROUTE-9 | `:param` → string-keyed params · CORE · ROUTE-12 | `<conv:name>` captures → view kwargs · CORE · ROUTE-3/4 |
| Param constraints / typed converters | `where` regex + `whereNumber/Uuid/In…`, global `Route::pattern` · CORE · ROUTE-5 | segment regex + verb constraints · CORE · ROUTE-11 | — (pattern-match in action heads instead) | typed converters `str/int/slug/uuid/path` returning typed values; custom `register_converter` · CORE/DIY · ROUTE-5/6 |
| Wildcard / regex catch-all | only via Folio `[...ids]` · OPT · ROUTE-22 | `*path` glob · CORE · ROUTE-13 | — | `path` converter (crosses `/`); full regex incl. unnamed/nested groups (discouraged) · CORE/OPT · ROUTE-5/4/7/8 |
| Grouping, prefixing & namespacing | route groups sharing middleware/prefix/name/controller · CORE · ROUTE-7 | `namespace`/`scope` decoupling URL, module, helper name; parametric scopes · CORE · ROUTE-7/19 | `scope` path + module namespace, arbitrarily nested · CORE · ROUTE-5 | `include()` nesting + app/instance namespaces (`'ns:name'`) · CORE · ROUTE-9/16 |
| Reusable route fragments | — (groups suffice) | routing `concern`s · CORE · ROUTE-8 | pipelines compose (pipelines plug pipelines) · CORE · ROUTE-8 | `include(pattern_list)` · CORE · ROUTE-9 |
| Route file organization | convention files web/api/console/channels; custom files & full override in `bootstrap/app.php` · CORE · ROUTE-2/19 | one `routes.rb` + `draw(:admin)` split files · CORE · ROUTE-21 | one compiled router module; `forward` for sub-routers · CORE · ROUTE-14 | per-app URLconf modules composed by `include()` · CORE · ROUTE-1/9 |
| Model binding from route params | implicit Eloquent binding (custom key `{post:slug}`, scoped, soft-deletes, `missing()`), enum binding, explicit `Route::model/bind` · CORE · ROUTE-9/10/11 | — (fetch in action; models used generation-side only · ROUTE-16) | — (generation-side only: `Phoenix.Param` · ROUTE-11) | — (view-side `get_object_or_404` · CTRL-50 (P2)) |
| Named routes & URL generation | `->name()` + `route()/url()/action()`, `URL::defaults`, fluent `Uri` · CORE · ROUTE-6/21 | auto `_path`/`_url` helpers, `url_for(@model)`, polymorphic, `direct`/`resolve` remaps, `default_url_options` · CORE · ROUTE-2/16/17, CTRL-21 (P2) | `~p` sigil + `url()`; structs interpolate via `Phoenix.Param` · CORE · ROUTE-9/10/11 | `name=` + `reverse()/reverse_lazy()`, namespaced resolution order, `query=`/`fragment=`; `get_script_prefix` · CORE · ROUTE-15/16/17/18/21 |
| Compile/build-time route safety | — (runtime) | — (runtime; `--unused` detection · ROUTE-22) | `~p` warns on unknown paths; duplicate routes warn at compile · CORE · ROUTE-9/6 | — (runtime `NoReverseMatch` · ROUTE-17) |
| Redirect & shorthand routes | `Route::redirect/permanentRedirect`, `Route::view` · CORE · ROUTE-3 | `redirect()` with `%{param}` interpolation, block form, status · CORE · ROUTE-14 | — | `RedirectView`/`TemplateView` in URLconf · CORE · CTRL-92/93 (P2); DB-driven contrib.redirects fallback · OPT · ROUTE-24 |
| Subdomain / host routing | `Route::domain('{account}.…')` with params · CORE · ROUTE-8 | request-based constraints (`subdomain:`), `matches?` objects · CORE · ROUTE-12 | — | — (per-request URLconf swap can emulate · OPT · ROUTE-2) |
| Mounting sub-apps / engines | — | `mount`/`match` any Rack app · CORE · ROUTE-15 | `forward` any plug; standalone `Plug.Router` micro-router · CORE · ROUTE-14/18 | `include()` third-party app URLconfs · CORE · ROUTE-9 |
| Fallback / unmatched handling | `Route::fallback` · CORE · ROUTE-12 | — (error templates · CTRL-11 (P2)) | — (no clause → ErrorHTML/JSON · CTRL-19 (P2)) | `handler400/403/404/500` hooks · DIY · ROUTE-22; contrib.redirects last-resort · OPT · ROUTE-24 |
| Rate limiting / throttling | named `RateLimiter::for` + `throttle:` middleware, Redis variant · CORE · ROUTE-13 | controller `rate_limit to:, within:` (cache-backed) · CORE · CTRL-15 (P2) | — (not inventoried; ecosystem) | — (not inventoried) |
| CORS | `HandleCors` + `config/cors.php`, auto preflight · CORE · ROUTE-16 | — (rack-cors, not inventoried) | — (cors_plug, not inventoried) | — (django-cors-headers, not inventoried) |
| Form method spoofing / verb normalization | `_method` + `@method` · CORE · ROUTE-14 | Rack::MethodOverride in default stack · CORE · CTRL-18 (P2, stack) | `Plug.MethodOverride` + `Plug.Head` · CORE · CTRL-16 (P2) | — (no override mechanism) |
| Signed URLs | `signedRoute`/`temporarySignedRoute` + `signed` middleware · CORE · ROUTE-20 | — (in P1; signing lives elsewhere) | — (Phoenix.Token elsewhere) | — (signing framework elsewhere; signed cookies · CTRL-16 (P2)) |
| Route-table performance | `route:cache` compiled table · CORE · ROUTE-17 | — | compile-time pattern-match; VM-optimized tree in Plug.Router · CORE · ROUTE-1/18 | first-access compile + resolver cache · CORE · ROUTE-23 |
| Route introspection | `route:list` CLI; current-route API · CORE · ROUTE-18/15 | `bin/rails routes` (`-g`/`-c`/`--unused`); console helpers · CORE · ROUTE-22 | `mix phx.routes`; router cheatsheet · CORE · ROUTE-13/19 | programmatic `resolve()`/`ResolverMatch` (no CLI) · CORE · ROUTE-19/20 |
| Routing test assertions | — | `assert_generates/recognizes/routing` · CORE · ROUTE-23 | — | — (documented `@override_settings(ROOT_URLCONF=…)` pattern · ROUTE-22) |
| Localized route paths | localized resource verbs · CORE · CTRL-5 (P2) | `scope(path_names:)`, Unicode routes · CORE · ROUTE-20 | — | `gettext_lazy` translated routes · CORE · ROUTE-3 |
| Default/extra params to handlers | — (generation-side `URL::defaults` · ROUTE-21) | `defaults:`/`defaults do`, `root` · CORE · ROUTE-10 | — | extra-options dicts (per route or per `include`), Python default args · OPT · ROUTE-11/12/13 |
| API versioning | opt-in `/api` prefix file · CORE · ROUTE-2 | plain `namespace` scopes · CORE · ROUTE-7 | plain nested scopes; explicitly no dedicated machinery · CORE · ROUTE-17 | plain `include()` prefixes · CORE · ROUTE-9 |
| File-based (page) routing | Folio: Blade files under `pages/` become routes, `[id]`/`[Model]` params · OPT · ROUTE-22 | — | — | — |
| Stateful / live routes | — | — | `live`/`live_session` router-mounted LiveViews with shared `on_mount` · CORE · ROUTE-15/16 | — |
| Health-check endpoint | — | `/up` `Rails::HealthController` in generated apps · CORE · ROUTE-24 | — | — |
| Per-request routing swap | — | — | — | middleware sets `request.urlconf` · OPT · ROUTE-2 |
| Format-suffix dispatch | — (Accept-header negotiation instead · CTRL-11 (P2)) | `(.:format)` + `respond_to do \|format\|` · CORE · ROUTE-9/25 | — (`accepts` pipeline plug · ROUTE-7) | — (Accept-header negotiation · CTRL-18/19 (P2)) |

**Notable divergences.** Django is the structural outlier: the router matches path only and never the HTTP verb (verb dispatch happens in the view), and it has no resource-routing concept at all, while Rails makes RESTful `resources` the organizing center and Laravel/Phoenix ship it as a first-class macro. Route-resolution cost is handled three different ways: Phoenix eliminates it at compile time (and uniquely verifies URL literals at build time), Laravel and Django paper over interpreted-language cost with route caches, Rails does neither. Laravel is alone in resolving domain objects inside the router (implicit model binding, enum binding, 404-on-miss); the other three treat that as controller/view business. Cross-cutting dispatch concerns also land in different layers: Laravel hangs rate limiting, CORS, and signed-URL validation directly off the router, Rails puts rate limiting in the controller and CORS/signing nowhere, Phoenix and Django route them to pipelines/middleware or the ecosystem. Finally, mountability differs by substrate: Rails and Phoenix can mount any Rack app/plug because they sit on an ecosystem-wide interface, Django composes at the URLconf level, Laravel has no mounting story.

**Volt notes.**
- Go's `http.ServeMux` (1.22+) already gives verb+`{param}` matching; three of four frameworks say verb-aware routing is table stakes, and Django's verb-agnostic design is the historical outlier, not a model.
- Phoenix's compile-time verified `~p` paths are the only static-safety story among the four and the most natural fit for Go: typed route constants or generated URL helpers could deliver what Laravel/Rails/Django only offer as runtime string lookup (`route('name')`, `reverse()`), which fights Go's type system.
- Route caching (Laravel, Django) is a scripting-language workaround Go gets for free — but every framework ships a route-introspection CLI (`route:list`, `bin/rails routes`, `mix phx.routes`), which a compiled router still needs to provide.
- Laravel's implicit model binding is the biggest DX gap between these frameworks and idiomatic Go; a Go equivalent needs generics/codegen and a defined DB-coupling boundary, since it welds the router to the ORM (Rails and Phoenix deliberately only couple in the URL-generation direction).

## P2 — Request handling: controllers & middleware

- **Laravel** — plain controller classes with container DI, a middleware onion configured centrally in `bootstrap/app.php`, rich `Request` object, fluent response/redirect builders.
- **Rails** — `ActionController::Base` subclasses, strong-parameters allowlist at the mass-assignment boundary, `before/after/around_action` callbacks, fully editable Rack stack underneath.
- **Phoenix** — Plug all the way down: one immutable `%Plug.Conn{}` flows through function/module plugs at endpoint, pipeline, and controller level; actions are plain 2-arity functions.
- **Django** — views are plain callables (`HttpRequest → HttpResponse`); middleware onion via the `MIDDLEWARE` setting; optional generic class-based views layered on top; sync and async both first-class.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Handler unit | plain classes (no base class), `__invoke` single-action · CORE · CTRL-1 | `ActionController::Base` subclass, method per action, implicit params/session · CORE · CTRL-1 | controllers are plugs; actions `fn(conn, params)`, params pattern-matched · CORE · CTRL-1/6 | plain callables (FBV) + optional `View` class w/ verb-name dispatch · CORE · CTRL-42/90 |
| Slim / API-only base variants | — (same classes for both) | `ActionController::Metal`/`::API` à-la-carte modules · CORE · CTRL-23 | — (already minimal) | — |
| DI into handlers | container resolves ctor + method type-hints alongside route params · CORE · CTRL-6 | — | — (explicit args) | — |
| Middleware contract | `handle($req, Closure $next)` classes · CORE · CTRL-7 | Rack middleware · CORE · CTRL-17 | function plugs + module plugs (`init/1`, `call/2`); everything is a plug · CORE · CTRL-1 | factory `(get_response) → callable`; optional `process_view/exception/template_response` hooks · DIY · CTRL-62/67/68/69 |
| Stack configuration & ordering | central `bootstrap/app.php`: append/prepend/replace, aliases, group editing, explicit priority · CORE · CTRL-8 | `config.middleware use/insert_before/after/swap/move/delete`; `bin/rails middleware` · CORE · CTRL-17 | literal code order in endpoint + `pipeline`/`pipe_through` per scope · CORE · CTRL-3, ROUTE-7 (P1) | ordered `MIDDLEWARE` setting; 15-point ordering doc; `MiddlewareNotUsed` startup opt-out · CORE/OPT · CTRL-63/76/66 |
| Middleware parameters | `:`-delimited args (`throttle:60,1`) · CORE · CTRL-9 | constructor args via `use` · CORE · CTRL-17 | `init/1` opts, compile-time · CORE · CTRL-1 | — (`__init__` takes only `get_response` · CTRL-65) |
| Short-circuiting | return response before `$next` (onion contract) · CORE · CTRL-7 | Rack contract (respond without calling app) · CORE · CTRL-17 | explicit `halt/1` · CORE · CTRL-2 | return without calling `get_response` · CORE · CTRL-64 |
| Per-controller/action hooks | `HasMiddleware`/`#[Middleware]` attr w/ only/except; `#[Authorize]` attr · CORE · CTRL-2/3 | `before/after/around_action` + `skip_*`/`prepend_*` · CORE · CTRL-8 | controller plugs w/ `when action in […]` guards; plug chains + halt for auth · CORE · CTRL-7/8 | per-view decorators (`require_*`, `condition`, csrf…) + `method_decorator` for CBVs · CORE · CTRL-52–61/103 |
| Post-response hooks | terminable middleware `terminate()` (FastCGI) · CORE · CTRL-10 | — (Executor lifecycle is the closest · CTRL-19) | — | — |
| Default middleware battery | `web`/`api` groups; `TrimStrings`, `ConvertEmptyStringsToNull` · CORE · CTRL-8/16 | ~20 documented middlewares (HostAuthorization, RequestId, RemoteIp, ETag, ConditionalGet…) · CORE · CTRL-18 | endpoint defaults (Static, RequestId, Telemetry, Parsers, MethodOverride, Head, Session) + Plug batteries (BasicAuth, Logger, SSL, CSRF, Static) · CORE · CTRL-4/18 | `SessionMiddleware`, `CommonMiddleware` + ordering guidance · CORE · CTRL-76/77/78 |
| Server-interface standard | PSR-7 bridge (return PSR-7, converted back) · CORE · CTRL-12 | Rack throughout; mountable Rack apps · CORE · CTRL-17, ROUTE-15 (P1) | Plug IS the standard; swappable Bandit/Cowboy adapters · CORE · CTRL-1/23 | WSGI + ASGI dual stack · CORE · CTRL-79 |
| Request introspection | `path/is/routeIs/fullUrl/host/method`, headers, `bearerToken`, `ip/ips` · CORE · CTRL-11 | `request`/`response` objects; `query_/request_/path_parameters` · CORE · CTRL-9 | one `%Plug.Conn{}` struct unifying req+resp · CORE · CTRL-2 | `HttpRequest` attrs: scheme/method/path/META/headers/encoding, streamed body, `get_host` (proxy-aware), `build_absolute_uri` · CORE · CTRL-1–8/13–15/17/20 |
| Params access & merging | `input()` dot-notation + typed getters (`integer/date/enum…`), presence predicates, `merge` · CORE · CTRL-13/14 | one `params` bag (query+body+route), hash/array conventions · CORE · CTRL-2 | one merged map, priority path>body>query; raw sources preserved · CORE · CTRL-15 | deliberately unmerged `GET`/`POST` QueryDicts (multi-value API) · CORE · CTRL-3/21/22 |
| Input allowlisting at the boundary | `only/except` selection (validation → P6 FormRequest) · CORE · CTRL-13 | Strong Parameters: `expect` (400 on shape mismatch)/`require`/`permit` · CORE · CTRL-4 | — (Ecto changeset `cast`, filed P6) | — (forms layer, filed P6) |
| Body parsing & limits | JSON via `json()`/`input()` (parsing implicit) · CORE · CTRL-13 | auto JSON parse keyed on Content-Type; `wrap_parameters` · CORE · CTRL-3 | `Plug.Parsers` (urlencoded/multipart/JSON) w/ `:length`/`:read_timeout` DoS limits · CORE · CTRL-14 | no auto JSON body parse; raw `body` + swappable multipart parser class · CORE/OPT · CTRL-2/9 |
| File uploads | `$request->file()`, mime helpers, `store/storeAs` to disks · CORE · CTRL-17 | — here (Active Storage, filed P13) | multipart via `Plug.Parsers` · CORE · CTRL-14 | `FILES` → `UploadedFile` · CORE · CTRL-4 |
| Content negotiation & variants | `accepts/prefers/expectsJson` · CORE · CTRL-11 | `respond_to` format blocks + `request.variant` per device · CORE · ROUTE-25 (P1)/CTRL-10 | `accepts` pipeline plug; per-format render/error views · CORE · ROUTE-7 (P1), CTRL-19 | `accepts()` + RFC 9110 `get_preferred_type()` · CORE · CTRL-18/19 |
| Conditional GET / ETag | — (filed P12) | ETag/ConditionalGet middleware in stack · CORE · CTRL-18 | — | `condition/etag/last_modified` decorators + `conditional_page` · CORE · CTRL-54/55/56 |
| Response construction | `response()` w/ status/headers/queued cookies; strings/arrays auto-convert · CORE · CTRL-18 | mutable `response` headers · CORE · CTRL-9 | `send_resp/3`, `put_status`, `put_resp_content_type` · CORE · CTRL-10 | `HttpResponse` family, dict-style headers, status subclasses (400/404/…), file-like write API · CORE · CTRL-23–26/29/35/43 |
| Response types (JSON/file/template) | view, `json/jsonp`, `download`, `file` · CORE · CTRL-20 | `send_data/send_file` + X-Sendfile offload · CORE · CTRL-13 | `render/3`, `text/json/html` helpers · CORE · CTRL-9 | `render()` shortcut, `JsonResponse`, `FileResponse` (`wsgi.file_wrapper`) · CORE · CTRL-37/39/47 |
| Redirect helpers | `redirect/back/route/action/away` + flashed input · CORE · CTRL-19 | (controller `redirect_to` not inventoried; route-level · ROUTE-14 (P1)) | `redirect/2` splits `to:` (in-app only, open-redirect safe) vs `external:` · CORE · CTRL-11 | `redirect()` (model/view-name/URL), 301/302/307/308 matrix, `RedirectView` · CORE · CTRL-32/33/48/49/93 |
| Streaming & SSE | `stream/streamJson/streamDownload`, SSE `eventStream` + first-party npm client hooks · CORE · CTRL-21 | `ActionController::Live` + `response.stream` SSE · CORE · CTRL-14 | conn streams natively via `send_resp`; WebSocket `upgrade` for bare Plug · CORE/OPT · CTRL-2/22 | `StreamingHttpResponse` (async iterator under ASGI); async file streaming via `aiofiles` · OPT/ECO · CTRL-38/40 |
| Error → response mapping | — in P1–P2 (central exception handler filed elsewhere) | `rescue_from` + `public/` error templates, format-aware dev pages · CORE · CTRL-11 | `action_fallback` normalizes non-conn returns; `ErrorHTML/JSON`; `Plug.Exception` status protocol; `Plug.Debugger`/`ErrorHandler` · CORE · CTRL-13/19/20/21 | `Http404`, default error views, `handler40x` hooks, auto exception→response between layers · CORE/DIY · CTRL-44/45/71, ROUTE-22 (P1) |
| Flash messages | session `flash/reflash/keep/now` · CORE · CTRL-23 | `flash.now`/`flash.keep` · CORE · CTRL-6 | `put_flash/clear_flash` + `flash_group` component · CORE · CTRL-12 | contrib.messages: levels, tags, 3 storage backends, CBV mixin · OPT · CTRL-114–117 |
| Old-input repopulation | `flash*/withInput()/old()` · CORE · CTRL-15 | — (form builders rebind model) | — (changesets re-render) | — (bound forms re-render) |
| Sessions | file/cookie/db/redis/dynamodb/mongo drivers; per-session cache; per-session request serialization via locks; custom drivers · CORE · CTRL-23/24/25/26 | encrypted `CookieStore` default + Cache/AR/Memcached; lazy; `reset_session` · CORE · CTRL-5 | cookie-backed `Plug.Session` + `fetch/put/get_session` · CORE · CTRL-17 | contrib.sessions: 5 backends, async API, `cycle_key`, `clearsessions`, cookie security settings, custom engines · OPT · CTRL-106–113 |
| Cookies (signed/encrypted) | queued cookies, automatic encryption · CORE · CTRL-18 | plain/`signed`/`encrypted`/`permanent` + key rotation · CORE · CTRL-7 | — (only the session cookie inventoried) | `set_cookie` (samesite etc.) + salted signed cookies w/ expiry · CORE/OPT · CTRL-27/28/16 |
| Per-request global state | — in P1–P2 (Context filed elsewhere) | `CurrentAttributes`, auto-reset per request · CORE · CTRL-20 | `conn.assigns` · CORE · CTRL-2 | middleware-set request attrs (`user/session/site`) + app hooks · CORE/OPT · CTRL-10/11 |
| Sync/async execution model | — (process-per-request; Octane filed P18) | threaded Rack; Executor/Reloader wrap every unit of work · CORE · CTRL-19 | — n/a (BEAM process per connection is the runtime) | first-class async views/middleware/CBVs, `sync_to_async`/`async_to_sync` adapters, capability flags, `SynchronousOnlyOperation` guard, cost model · CORE · CTRL-46/72–74/79–89/91 |
| Code reload / dev-loop safety | — | Executor/Reloader framework · CORE · CTRL-19 | dev endpoint block: live reload, CodeReloader, CheckRepoStatus ("run migrations" page) · CORE · CTRL-5 | — (runserver autoreload, not inventoried here) |
| Generic CRUD handler abstractions | resource controllers (see P1) · CORE · CTRL-4/5 | — (scaffold generators, P17) | — (phx.gen, P17) | full generic-CBV suite: Template/Redirect/List/Detail/Form/Create/Update/Delete + 7 date-archive views, mixin lattice, hook methods · CORE · CTRL-92–105 |
| Pagination at the request layer | — (ORM paginator, filed P4) | — (ecosystem) | — (ecosystem) | `Paginator` + `AsyncPaginator`, `ListView paginate_by` · CORE · CTRL-118/119/120 |
| HTTP auth helpers | — (guards, filed P7) | Basic/Digest/Token helpers · CORE · CTRL-12 | `Plug.BasicAuth` · CORE · CTRL-18 | — (contrib.auth, filed P7) |
| Browser/client gating | — | `allow_browser versions:` → 406 for old browsers, on by default · CORE · CTRL-16 | — | — |
| Sensitive-data log filtering | — (filed P20) | `filter_parameters`/`filter_redirect` · CORE · CTRL-22 | — (filed P20) | `sensitive_variables`/`sensitive_post_parameters` decorators · CORE · CTRL-61 |
| Request normalization | `TrimStrings`, `ConvertEmptyStringsToNull` (disable-able) · CORE · CTRL-16 | — | `MethodOverride`/`Head` · CORE · CTRL-16 | `APPEND_SLASH`/`PREPEND_WWW` + `no_append_slash` opt-out · CORE/OPT · CTRL-78/58 |
| Proxy / forwarded-header handling | — in P1–P2 (TrustProxies filed elsewhere) | `RemoteIp` in default stack · CORE · CTRL-18 | `Plug.RewriteOn` (x-forwarded-*) · CORE · CTRL-18 | `USE_X_FORWARDED_HOST/PORT`, `ALLOWED_HOSTS` check, documented multi-proxy rewrite pattern · CORE · CTRL-13 |
| Response extensibility | `Response::macro` builders · CORE · CTRL-22 | — (reopen classes) | — (plain functions) | custom `HttpResponse` subclasses, `render()`-emulation · DIY · CTRL-31/36 |
| Documented request lifecycle | single `index.php` entry, HTTP/console kernels · CORE · CTRL-27 | Rack stack + Executor docs · CORE · CTRL-17/18/19 | Endpoint as shared entry pipeline · CORE · CTRL-3 | — (implied by WSGI/ASGI handlers · CTRL-79) |

**Notable divergences.** The deepest split is the handler model: Phoenix and Django treat a handler as a plain function over an explicit request value, Rails demands a framework base class with implicitly available state (`params`, `session`, `flash`), and Laravel sits between — plain classes but with container DI injecting anything type-hinted. Phoenix is alone in making the pipeline element and the application the same abstraction (everything is a plug over one immutable Conn); the others separate "middleware" from "controller" as different species, and only Rack/Plug are ecosystem-wide standards — Laravel and Django middleware are framework-private contracts (Laravel only bridges out via PSR-7). On input, three frameworks merge route/query/body params into one bag while Django refuses on principle (separate multi-value `GET`/`POST`), and only Rails enforces an allowlist (strong params) at the controller boundary — Laravel, Phoenix, and Django all push input shaping down to the validation layer (P6). Concurrency is answered by runtime in three cases (PHP process-per-request, BEAM processes, Ruby threads + Executor) but is a massive explicit API surface in Django (~20 inventory rows of sync/async adaptation rules). Finally, Django uniquely ships generic CRUD as request-layer class machinery (CBVs + mixins) where Rails/Phoenix reach for code generators and Laravel for resource controllers.

**Volt notes.**
- Plug's model — one immutable conn, handlers and middleware sharing a single `fn(conn) → conn` shape, halt to short-circuit — is the closest analog to Go's `http.Handler` and the strongest evidence that a unified handler/middleware abstraction works at framework scale; Rails' implicit-state base class is the least Go-translatable.
- All four ship a curated default middleware battery (request ID, static, parsers, session, CSRF, proxy-header rewriting) and a documented ordering; Go's ecosystem norm is à-la-carte, so choosing and ordering a default stack is itself a headline feature for Volt.
- The params question has three positions (merged bag / merged-with-allowlist / deliberately separate); in Go the real decision is where typed decoding happens — Laravel's typed getters (`$request->integer()`, `date()`, `enum()`) and Django's typed route converters both target exactly the `strconv` boilerplate Go handlers drown in.
- Sessions with pluggable stores plus signed/encrypted cookies are CORE in every framework (only tier varies — Django marks them OPT-contrib); net/http offers none of this, and Rails' key-rotation and Laravel's session-locking rows show the long tail beyond "a cookie store".

## P3 — Views, templating & frontend assets

- **Laravel**: Blade — a compiled, zero-overhead template language with a full class/anonymous component system — plus a first-party Vite plugin (HMR, SSR, CSP/SRI); Livewire and Inertia are the blessed interactivity paths.
- **Rails**: ERB compiled to Ruby methods, a very deep helper/form-builder library, layouts + partials, Propshaft + importmap for a no-Node asset story, Hotwire (Turbo + Stimulus) as default interactivity.
- **Phoenix**: HEEx — HTML-aware, compile-checked templates where everything is a *function component* with declared attrs and slots — plus zero-Node assets via Elixir-wrapped esbuild and Tailwind.
- **Django**: DTL — a deliberately restricted, autoescaping text language with `{% block %}` inheritance and ~60 filters — behind a multi-engine abstraction (Jinja2 blessed); staticfiles collects/hashes assets; bundlers and JS toolchains are explicitly out of scope.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Template language & compilation model | Blade compiled to PHP, cached · CORE · VIEW-1, VIEW-3 | ERB compiled/cached as Ruby methods · CORE · VIEW-1 | HEEx `~H` compiled into module functions · CORE · VIEW-2, VIEW-7 | DTL interpreted, restricted-by-design · CORE · VIEW-1, VIEW-2 |
| Auto-escaping + raw opt-out | `{{ }}` escape, `{!! !!}` raw, `Js::from` · CORE · VIEW-4 | ERB auto-escapes · CORE · VIEW-1 | Auto-escape all interpolation; `raw/1` documented-dangerous · CORE · VIEW-3 | Autoescape on; `safe`, `{% autoescape %}` · CORE · VIEW-17, VIEW-4 |
| Compile-time template validation | — (runtime) | Strict locals: checked partial signatures · CORE · VIEW-9 | HTML validity = compile errors; `attr/3` misuse warnings · CORE · VIEW-1, VIEW-2 | — |
| Control flow + loop metadata | `@if/@switch`, `@foreach` + `$loop` object · CORE · VIEW-5, VIEW-6 | Plain Ruby; collection `_counter/_iteration` · CORE · VIEW-1, VIEW-8 | `:if`/`:for` special attrs, `:key` diffing · CORE · VIEW-5 | `{% if %}`, `{% for %}` + `forloop.*` · CORE · VIEW-7 |
| Layouts / inheritance | Component layouts *or* `@extends/@section/@yield` · CORE · VIEW-12 | `yield`/named yield + `content_for`, nested/conditional layouts · CORE · VIEW-6 | Root layout plug + explicit `<Layouts.app>` function component · CORE · VIEW-8 | `{% extends %}/{% block %}` + `block.super` · CORE · VIEW-8 |
| Partials / includes | `@include*`, `@each`, `@once` · CORE · VIEW-8 | Partials w/ locals, collection render, spacers · CORE · VIEW-7, VIEW-8 | Function components; `embed_templates` (file ≡ function) · CORE · VIEW-1, VIEW-7 | `{% include with ... only %}`; `{% partialdef %}` (6.0) · CORE · VIEW-9, VIEW-16 |
| Components w/ props, slots, attribute bags | Class + anonymous components, `@props`/`@aware`, slots, `$attributes` bag, dynamic components · CORE · VIEW-9, VIEW-10, VIEW-11 | — in core (partials + strict locals are the tool; ViewComponent not inventoried) | `attr/3` declared props, named/scoped slots, attr splat · CORE · VIEW-1, VIEW-4, VIEW-6 | — (closest: `inclusion_tag` · VIEW-30) |
| Generated/blessed UI component library | — | — | `CoreComponents` generated into app · CORE · VIEW-9; community systems · ECO · VIEW-10 | — |
| Conditional attribute/class helpers | `@class/@style/@checked/...` · CORE · VIEW-7 | `tag` builder, `token_list` · CORE · VIEW-14 | `class={}` nil/false handling, `{@attrs}` splat · CORE · VIEW-4 | — |
| Form builders (model-bound) | Form directives only: `@csrf/@method/@error` · CORE · VIEW-13 | `form_with` + full field suite, custom builders, nested `fields_for` · CORE · VIEW-15–18 | `.form`/`.input` in CoreComponents; `phoenix_html` substrate · CORE · VIEW-9, VIEW-11 | — in P3 (forms framework under P6; widget assets via Media, VIEW-43) |
| URL / asset / link tag helpers | `Vite::asset` · CORE · VIEW-18 | `image_tag`, `link_to`, `button_to`, preload etc. · CORE · VIEW-10, VIEW-11 | `Plug.Static` paths (verified routes in P1) · CORE · VIEW-18 | `{% url %}`, `{% static %}` · CORE · VIEW-10, VIEW-34 |
| Text/number/date formatting helpers | — in P3 (Str/Number in P25) | Deep helper library (`time_ago_in_words`, `number_to_currency`, `truncate`...) · CORE · VIEW-13 | — | ~60 filters (string/list/number/date families) · CORE · VIEW-3–6; humanize · OPT · VIEW-49 |
| Untrusted-HTML sanitization | — (escaping only) | `sanitize`/`strip_tags`/`sanitize_css` (rails-html-sanitizer) · CORE · VIEW-12 | — (`raw` is the only door) | `striptags`, escaping filter family · CORE · VIEW-4 |
| Shared/global template data injection | `View::share`, composers/creators, `@inject` DI · CORE · VIEW-2, VIEW-15 | — (controller ivars, not inventoried) | — (explicit assigns only, by design) | Context processors, built-in + custom · CORE/DIY · VIEW-26, VIEW-27 |
| Template-language extension API | `Blade::directive/if`, echo handlers · CORE · VIEW-17 | Alternate template handlers · CORE · VIEW-2 | Any function is a component — no special API · CORE · VIEW-1 | Custom filters, `simple_tag`/`inclusion_tag`/`simple_block_tag`, Node API · CORE/DIY · VIEW-29–31 |
| Multi-engine abstraction | — (Blade only) | Handlers per format (Builder XML, Jbuilder) · CORE · VIEW-2 | — (HEEx + plain EEx) | `TEMPLATES` multi-engine, Jinja2 backend, custom backend · CORE/ECO/DIY · VIEW-20–23 |
| Template resolution, loaders & overriding | Dot notation, `View::first/exists` · CORE · VIEW-1 | Implicit `views/<controller>/<action>.<format>` convention · CORE · VIEW-3 | Per-format view modules co-located with controllers · CORE · VIEW-7 | Loaders (filesystem/app_dirs/cached/locmem), app-template overriding · CORE/DIY · VIEW-24, VIEW-25, VIEW-28, VIEW-32 |
| Template precompilation/caching for deploys | `view:cache`/`view:clear` · CORE · VIEW-3 | Compiled at first render · CORE · VIEW-1 | Compiled into bytecode at build · CORE · VIEW-2, VIEW-7 | Cached loader (auto-on) · CORE · VIEW-24 |
| Inline/string rendering | `Blade::render` from string · CORE · VIEW-16 | `render inline:` · CORE · VIEW-3 | `~H` sigil in any module · CORE · VIEW-1 | `render_to_string`, Engine API · CORE · VIEW-28, VIEW-21 |
| Fragment responses (htmx/turbo-style partial pages) | `@fragment` + `->fragment()` · CORE · VIEW-16 | Turbo Frames/Streams · CORE · VIEW-23, VIEW-24 | — in P3 (LiveView owns this, P10) | `{% partialdef %}` + `template.html#name` loading (6.0) · CORE · VIEW-16, VIEW-28 |
| Blessed interactivity layer | Livewire (no-JS) / Inertia (SPA glue, SSR) · ECO · VIEW-23, VIEW-24 | Hotwire: Turbo Drive/Frames/Streams + Stimulus, default · CORE · VIEW-22–26 | LiveView (core, inventoried under P10); component ecosystems · ECO · VIEW-10 | — (explicitly out of scope) |
| Bundler/build-tool integration | First-party Vite plugin: `@vite`, HMR, presets, env vars · CORE · VIEW-18, VIEW-19 (legacy Mix · OPT · VIEW-25) | importmap no-build default; `jsbundling/cssbundling` escape hatches · CORE/OPT · VIEW-20, VIEW-21 | Elixir-wrapped esbuild + Tailwind, no Node; documented replacements · CORE · VIEW-12, VIEW-13, VIEW-17 | — (non-goal: frontend tooling out of scope) |
| Asset fingerprinting / cache-busting | Vite build manifest · CORE · VIEW-18 | Propshaft digests, url() rewriting · CORE · VIEW-19 | `mix assets.deploy` + `phx.digest` manifest · CORE · VIEW-16 | `ManifestStaticFilesStorage` hashed names · OPT · VIEW-40 |
| Static file collection & serving | public/ convention (not inventoried) · — | Propshaft logical paths, precompile/clean · CORE · VIEW-19 | `Plug.Static` over `priv/static` · CORE · VIEW-18 | staticfiles app: `collectstatic`/`findstatic`, dev serving, storage/finders, deployment recipes · OPT/DIY · VIEW-33–42 |
| Third-party JS package handling | npm via Vite · CORE · VIEW-19 | `bin/importmap pin` (CDN or vendored ESM) · CORE · VIEW-20 | vendor / `npm --prefix assets` / Mix git deps · CORE · VIEW-15 | — |
| Production asset hardening (SSR, CSP nonce, SRI, prefetch) | Vite SSR builds, `useCspNonce`, SRI, prefetch strategies · CORE · VIEW-20 | — | — | `{% csp_nonce_attr %}` (6.1) · CORE · VIEW-18 |
| Font/icon optimization | Vite font providers + `@fonts` · CORE · VIEW-21 | — | Heroicons as Tailwind classes (only used icons ship) · CORE · VIEW-14 | — |
| Per-page asset injection (stacks / content_for / Media) | `@push/@stack/@prepend` · CORE · VIEW-14 | `content_for`/`provide`, `capture` · CORE · VIEW-6, VIEW-14 | — (slots can serve) | Form/widget `Media` class, `Script`/`Stylesheet` objects (6.1) · CORE · VIEW-43, VIEW-44 |
| Rich text editing | — | Action Text: `has_rich_text`, Trix, Active Storage attachments · CORE · VIEW-27, VIEW-28 | — | — |
| Template variants & localized templates | — | `+phone` variants, `index.es.html.erb` · CORE · VIEW-29, VIEW-4 | Per-format modules (`HelloHTML`/`HelloJSON`) · CORE · VIEW-7 | — (i18n tag libraries · OPT · VIEW-19) |
| Pagination rendering in views | `links()` w/ Tailwind/Bootstrap views, customizable · CORE · VIEW-22 | — | — | — |
| Template dev ergonomics (formatter, debug annotations) | — | — | HEEx debug annotations, `mix format` for HEEx · CORE · VIEW-19, VIEW-20 | `{% debug %}`, `string_if_invalid` · CORE · VIEW-13, VIEW-21 |
| Non-HTML template output (XML/CSV/PDF) | — | Builder XML, Jbuilder JSON handlers · CORE · VIEW-2 | `home.xml.eex` + `put_format` · CORE · VIEW-21 | CSV via csv module/streaming · DIY · VIEW-45; PDF via ReportLab · ECO · VIEW-46 |
| CMS-lite flat pages | — | — | — | contrib.flatpages + fallback middleware · OPT · VIEW-47, VIEW-48 |

**Notable divergences.** The deepest split is *when errors surface*: Phoenix makes malformed HTML and undeclared component attributes compile errors, Rails type-checks partial signatures with strict locals, while Blade and DTL fail (or silently render) at runtime. The component model splits the field: Laravel and Phoenix are component-first (declared props, named/scoped slots, attribute merging), Rails deliberately stays with partials + a huge helper library, and Django restricts template logic so hard that "components" only exist as custom tags. On assets the four take four philosophies: Laravel embraces the Node ecosystem via a first-party Vite plugin (HMR, SSR, CSP/SRI); Rails and Phoenix both engineer Node *away* (importmap CDN/vendored pins vs Elixir-wrapped esbuild binaries) but keep a build; Django declares the whole problem out of scope and only handles collection + fingerprinting of finished files. Django alone abstracts over multiple template engines; the other three bet on one language. Interactivity tiers differ too: Hotwire is CORE for Rails and LiveView is core Phoenix, while Laravel's equivalents (Livewire/Inertia) are ecosystem packages and Django offers nothing.

**Volt notes.**
- Go can plausibly match Phoenix's headline feature — compile-checked, HTML-aware templates (cf. templ) — which none of the interpreted-template frameworks offer; alternatively `html/template` gives contextual auto-escaping for free but no component/slot model, which is now the 2/4-and-rising baseline (props + slots + attribute merging).
- All four converged on fingerprinted assets with a manifest; the Phoenix pattern (framework-managed standalone esbuild/tailwind binaries, no Node) maps naturally onto Go's single-binary + `embed` deployment story and avoids Laravel's Node dependency without Django's abdication.
- Fragment/partial responses (Blade fragments, Turbo Streams, Django 6.0 `partialdef`, LiveView) appear in all four — first-class named-fragment rendering is now table stakes for htmx-style frontends.
- Rails and Django invest heavily in form/field helper suites tied to their model layers; Laravel and Phoenix push forms into components. Whichever Volt picks, form rendering is coupled to the validation/ORM design (P6/P4), not a standalone view concern.

## P4 — Data layer: models, ORM & queries

- **Laravel:** two layers — a fluent SQL query builder over PDO plus Eloquent, a maximal ActiveRecord (relationships, casts, events, factories), with first-party Redis/MongoDB.
- **Rails:** the archetypal Active Record — schema-reflected classes, lazy chainable relation algebra, associations/callbacks/dirty-tracking making the model object the center of business logic.
- **Phoenix:** Ecto's data-mapper split — Repo (only thing that touches the DB), Schema (pure struct mapping), compile-checked composable Query DSL, Changeset; no lazy loading, no global model objects.
- **Django:** declarative active-record models carrying fields + Meta (constraints, indexes, managers); lazy QuerySets with lookups/F/Q/expressions/window functions; explicit `save()`; growing async surface.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| ORM paradigm | Eloquent ActiveRecord · CORE · ORM-25 | Active Record, convention mapping · CORE · ORM-1 | Repo+Schema data mapper · CORE · ORM-1,3 | Declarative AR `models.Model` · CORE · ORM-1 |
| Field/attribute declaration | Schema-reflected + declared casts · CORE · ORM-25,52 | Schema-reflected, no declarations; Attributes API override · CORE · ORM-1,36 | Explicit typed `field` decls · CORE · ORM-3 | Explicit field classes (~30 types) · CORE · ORM-19–36 |
| Supported databases | MariaDB/MySQL/PG/SQLite/SQL Server · CORE · ORM-1 | SQLite (prod-blessed)/PG/MySQL/Trilogy + adapter API · CORE · ORM-52 | PG/MySQL/MSSQL/SQLite/ClickHouse/ETS · CORE/OPT · ORM-2 | PG/MySQL/MariaDB/SQLite/Oracle (agnostic charter; capability tiers) · CORE · MIG-4, P23 preamble |
| Connection pooling | PgBouncer-style pooled PG · CORE · ORM-3 | Per-role/shard pools sized to threads · CORE · ORM-51 | Repo-owned pools in supervision tree · CORE · ORM-1 | — (per-request connections; no pool row) |
| Read/write splitting & replicas | `sticky` read-your-writes split · CORE · ORM-2 | Auto role-switching middleware + `connected_to` · CORE · ORM-45; LB explicitly DIY · ORM-47 | Replica repo modules + `replica/0` picker · CORE · ORM-35 | `using()` + DB routers · CORE/DIY · ORM-198,199 |
| Multiple DBs / sharding | Multi-connection config, per-query selection · CORE · ORM-1 | `connects_to`, horizontal sharding + shard-selection middleware · CORE · ORM-44,46 | Dynamic repos (`start_link` + `put_dynamic_repo`) · CORE · ORM-36 | Router `db_for_read/write` · DIY · ORM-199 |
| Multi-tenancy | — | Via sharding · CORE · ORM-46 | Query prefixes (PG schemas) · CORE · ORM-33; FK-scoping via `prepare_query` · DIY · ORM-34 | — |
| PK strategies (UUID/ULID) | `HasUuids`/`HasUlids` (UUIDv7) · CORE · ORM-26 | UUID PKs via migration/generator config · CORE · MIG-13 | Custom `@primary_key`, `binary_id`, UUIDv7 helpers · CORE · ORM-4,38 | Auto BigAutoField; native UUIDField · CORE · ORM-2,22; `UUID4/UUID7` funcs · OPT · ORM-185 |
| Composite primary keys | — | Full support incl. assoc + routing · CORE · ORM-48 | — (custom `@primary_key` only · ORM-4) | `CompositePrimaryKey` with real holes (no FK to CPK, no admin/forms) · OPT · ORM-31,32 |
| Auto timestamps | created/updated_at, disable/rename/touch · CORE · ORM-25 | Framework-maintained · CORE · ORM-2 | `timestamps/1` macro · CORE · ORM-4 | `auto_now`/`auto_now_add` field opts · CORE · ORM-23 |
| Enum attributes | Enum casts + `AsEnumCollection` · CORE · ORM-52 | `enum` macro (predicates, scopes, prefix/suffix) · CORE · ORM-15 | `Ecto.Enum` typed field · CORE · ORM-6 | `TextChoices`/`IntegerChoices` · CORE · ORM-4,5 |
| Query builder / composition model | Fluent builder, `when()` conditional, `tap/pipe` query objects · CORE · ORM-8–20,23 | Lazy chainable relation algebra + overrides (`unscope/rewhere/reorder`) · CORE · ORM-6–8 | Compile-checked keyword+pipe DSL; queries are composable values · CORE · ORM-12,17 | Lazy chainable QuerySets, result caching, slicing · CORE · ORM-130–132 |
| Where/lookup vocabulary | Huge `where*` family incl. date/JSON/full-text · CORE · ORM-15–17 | String/placeholder/hash conditions, `where.not/.or/.and` · CORE · ORM-6 | Full clause coverage; keyword-list/map shortcuts · CORE · ORM-14,18 | `__lookup` families (text/date-part/regex/JSON) + `Q` · CORE · ORM-157–165 |
| Runtime/dynamic query building | `when($value, cb)` conditional · CORE · ORM-20 | Scope merging, `merge` · CORE · ORM-14 | `dynamic/2` macro composed via `Enum.reduce` (sanctioned filter pattern) · CORE · ORM-19 | `Q()` composition with `&`/`\|`/`~` · CORE · ORM-165 |
| Joins | inner/left/right/cross, closure, subquery, lateral · CORE · ORM-13 | `joins`/`left_outer_joins`, `where.associated/missing` · CORE · ORM-11 | `join on:`, assoc joins, named bindings, `inner_lateral_join` · CORE · ORM-15 | Implicit via `__` traversal; `FilteredRelation` for ON-clause control · CORE/OPT · ORM-162,154 |
| Subqueries / CTEs / window fns | Subquery selects, `fromSub`, `whereExists` · CORE · ORM-11,17 | — (not in corpus; Arel undocumented) | `subquery/1`, `parent_as` correlated; windows/CTEs · CORE · ORM-16,14 | `Subquery`/`OuterRef`/`Exists`, `Window` + frames · OPT · ORM-166–168,186 |
| Union / set operations | `union/unionAll` · CORE · ORM-14 | — | — (not in corpus) | `union/intersection/difference`, `&`/`\|`/`^` · OPT/CORE · ORM-149,155 |
| Aggregates | count/max/min/avg/sum · CORE · ORM-10 | Grouped aggregate hashes · CORE · ORM-19 | `Repo.aggregate` w/ auto subquery wrap · CORE · ORM-11 | `annotate`/`aggregate`, rich fn set, conditional `filter=Q` · CORE/OPT · ORM-174–186 |
| Column shortcuts (no hydration) | `pluck/value` · CORE · ORM-8 | `pluck/pick/ids` · CORE · ORM-18 | `select:` maps/tuples/`%{id => email}` shapes · CORE · ORM-14 | `values`/`values_list` · CORE · ORM-141,142 |
| Single-record finders | `find/sole/firstOrFail` (404-throwing) · CORE · ORM-29 | `find` (raises), `find_by(!)`, `take` · CORE · ORM-4 | `get/get_by/one` + `!` variants · CORE · ORM-7 | `get()` raising DoesNotExist; `first/last/latest` · CORE · ORM-130,140 |
| Find-or-create / upsert-one | `firstOrCreate/updateOrCreate/incrementOrCreate` · CORE · ORM-30 | `find_or_create_by(!)/find_or_initialize_by` · CORE · ORM-16 | — (use `on_conflict` upsert · ORM-28) | `get_or_create/update_or_create` (atomic only w/ DB uniqueness) · CORE · ORM-134 |
| Bulk writes | `insert/upsert/update/delete`, `insertUsing` · CORE · ORM-21 | `update_all/delete_all` (no bulk insert in corpus) · CORE · ORM-3 | `insert_all/update_all/delete_all` w/ placeholders · CORE · ORM-9 | `bulk_create/bulk_update/update()/delete()` · CORE · ORM-135–138 |
| Upsert / conflict targets | `upsert(conflict, update)` · CORE · ORM-21 | — (not in corpus) | `on_conflict:`+`conflict_target:` incl. `:replace_changed` HOT · CORE · ORM-28 | `bulk_create(update_conflicts/unique_fields)` · CORE · ORM-135 |
| Race-free atomic updates | `increment/decrement/incrementEach` · CORE · ORM-21 | — (not in corpus) | `update_all` `:set/:inc/:push/:pull` (documented answer) · CORE · ORM-23 | `F()` expressions · CORE · ORM-117,164 |
| Pessimistic locking | `sharedLock/lockForUpdate` · CORE · ORM-22 | `lock/with_lock` · CORE · ORM-10 | — (not in corpus) | `select_for_update(nowait, skip_locked, of, no_key)` · OPT · ORM-152 |
| Optimistic locking | — | `lock_version` column · CORE · ORM-10 | — (not in corpus; Ecto has it upstream) | — |
| Transactions | Closure w/ deadlock retry, manual API, `afterCommit` · CORE · ORM-6 | Rollback-on-exception; `after_commit/rollback` callbacks · CORE · ORM-33,34 | `Repo.transact` + named `Ecto.Multi` · CORE · ORM-29,30 | `atomic` (nest/savepoint/durable), `on_commit`, autocommit control · CORE/OPT · ORM-192–196 |
| Associations: core types | hasOne/hasMany/belongsTo, one-of-many, through · CORE · ORM-40 | belongs_to (required by default)/has_one/has_many + full method family · CORE · ORM-22 | belongs_to/has_one/has_many/:through · CORE · ORM-24 | ForeignKey/OneToOne; reverse managers; recursive/lazy refs · CORE · ORM-39,46–48 |
| Many-to-many + pivot data | `belongsToMany`, pivot models, wherePivot · CORE · ORM-41 | HABTM + has_many :through join models · CORE · ORM-23 | `many_to_many` join table/schema, custom keys · CORE · ORM-24 | `ManyToManyField` + `through` model · CORE · ORM-43,45 |
| Polymorphic / generic relations | Full morph family + morph maps · CORE · ORM-42 | `polymorphic: true` type+id pairs · CORE · ORM-24 | — (by design; embedded/abstract patterns instead) | Generic FK via contenttypes (outside P4; `GenericPrefetch` ref) · OPT · ORM-147 note |
| Association config (dependent, touch, autosave) | `$touches`, cascade via FK or events · CORE · ORM-49 | `dependent:/touch:/autosave:/counter_cache:` + assoc callbacks/extensions · CORE · ORM-26–28 | DB-level `on_delete` in migrations instead · CORE · MIG-3 | `on_delete` Python-emulated + DB-level DB_CASCADE (6.1) · CORE/OPT · ORM-40,41 |
| Eager loading | `with` (nested/constrained), `load/loadMissing` · CORE · ORM-46 | `includes`/`preload`/`eager_load` 3 strategies · CORE · ORM-12 | Explicit-only: `preload:`/`Repo.preload`/join+preload · CORE · ORM-25 | `select_related` (JOIN) / `prefetch_related` + `Prefetch` · CORE · ORM-146,147 |
| Lazy-loading & N+1 guardrails | Auto eager-load mode; `preventLazyLoading`; `shouldBeStrict` · CORE · ORM-27,47 | `strict_loading` per relation/model/app · CORE · ORM-13 | No lazy loading exists, structurally · CORE · ORM-25 | `FETCH_PEERS` fetch mode auto-solves N+1; `RAISE` mode · OPT · ORM-151,201 (6.1) |
| Relationship aggregates / counters | `withCount/withSum...`, deferred `loadCount` · CORE · ORM-45 | Counter caches w/ backfill workflow · CORE · ORM-27 | Via aggregate queries · CORE · ORM-11 | `annotate(Count(...))` · CORE · ORM-174 |
| Persisting through relations / nested writes | `save/createMany`, `associate`, m2m `attach/sync/toggle` · CORE · ORM-48 | Autosave + association build/create methods · CORE · ORM-22,26 | `cast_assoc` (external params) vs `put_assoc` (internal), `build_assoc` · CORE · ORM-26,27 | Related managers `add/set/clear`; no nested-write API (formsets in P6, VAL-65) · CORE · ORM-45 |
| Scopes / reusable query logic | Global scopes + `#[Scope]` locals, pending attributes · CORE · ORM-36,37 | `scope`/`default_scope`, merging, `unscoped` · CORE · ORM-14 | Plain query functions; `Ecto.Query` composition · CORE · ORM-17 | Custom managers + `QuerySet.as_manager`/`from_queryset` · CORE · ORM-118–126 |
| Lifecycle hooks / model events | Full event set, observers, `saveQuietly` muting · CORE · ORM-39 | before/after/around callbacks, `throw :abort`, transactional callbacks · CORE · ORM-32,33 | — (by design: no callbacks; explicit pipelines) | Signal pipeline pre/post save/delete; skipped by bulk ops · CORE · ORM-102,37 |
| Dirty / change tracking | `isDirty/wasChanged/getOriginal` · CORE · ORM-32 | ActiveModel::Dirty on every model · CORE · ORM-35 | Changesets carry `changes`; minimal-diff UPDATEs · CORE · P6 VAL-6,12 | Manual `update_fields`; `from_db` recipe · CORE/DIY · ORM-107,98 |
| Attribute casting / custom types | Rich casts + `CastsAttributes` value objects · CORE · ORM-52,53 | `attribute :name, :type`, custom types · CORE · ORM-36 | Custom driver types (`Postgrex.Types.define`) · OPT · ORM-37 | Custom `Field` subclass API · DIY · ORM-34 |
| Accessors / normalization | `Attribute::make(get:, set:)` · CORE · ORM-51 | `normalizes :email` on assignment+finders · CORE · ORM-39 | — (transform inside changeset pipeline) | `@property`, save() override · CORE · ORM-37 |
| Attribute-level encryption | `encrypted:*` casts, `hashed` · CORE · ORM-52 | AR Encryption: deterministic mode, key rotation, contexts · CORE · ORM-41–43 | `redact: true` (log-hiding only, not encryption) · CORE · ORM-5 | — |
| Soft deletes & pruning | `SoftDeletes` trait; `Prunable` scheduled pruning · CORE · ORM-33,34 | — (ecosystem) | — (by design) | — (ecosystem) |
| Inheritance strategies | — | STI + Delegated Types alternative · CORE · ORM-29,30 | — (structs, no inheritance) | Abstract bases, multi-table, proxy models · CORE · ORM-87,90,93 |
| Embedded documents / JSON columns | JSON `->` queries, `whereJsonContains`; array/object casts · CORE · ORM-16,52 | jsonb via PG-native types · CORE · ORM-49 | `embeds_one/many` structs in JSONB, jsonpath queryable · CORE · ORM-31 | `JSONField` + full lookup family · CORE/OPT · ORM-29,163 |
| Schemaless / table-only queries | Query builder is table-first by nature · CORE · ORM-8 | — | `from "posts"`, schemaless CRUD, `type/2` · CORE · ORM-22 | `values()`/raw; no schemaless model layer · ORM-141,188 |
| Generated/computed columns | `storedAs/virtualAs` migration modifiers · CORE · MIG-8 | — | — | `GeneratedField` (stored/virtual) · CORE · ORM-30 |
| PostgreSQL-native types | — (vector only, ORM-18) | bytea/array/hstore/ranges/composite/enums/inet... · CORE · ORM-49 | Via custom postgrex types · OPT · ORM-37 | contrib.postgres: ArrayField/HStore/Ranges + PG indexes, exclusion constraints · OPT · PG-1–3,7,8 |
| Full-text search (query-level) | `whereFullText` (per-driver) · CORE · ORM-17, SRCH-1 | PG full-text patterns · CORE · ORM-49 | — (`ilike` only, ORM-14) | `__search`/trigram/unaccent via contrib.postgres · ECO/OPT · ORM-173, PG-4–6 |
| Vector similarity search | `whereVectorSimilarTo`, distance select/order, auto-embedding · CORE · ORM-18, SRCH-2 | — | — | — (not in corpus) |
| Batch/streaming reads | `chunk/lazy/cursor` LazyCollections · CORE · ORM-9,28 | `find_each/in_batches` cursors · CORE · ORM-5 | `Repo.stream` · CORE · ORM-10 | `iterator(chunk_size)` server-side cursors · OPT · ORM-203 |
| Pagination | Length-aware/simple/cursor paginators · CORE · ORM-56 | — (ecosystem; not in corpus) | — (explicit gap: API-13) | Core `Paginator`/`AsyncPaginator` (inventoried at P2 CTRL-118–120) · CORE |
| Async query surface | — | `load_async` thread-pool executor · CORE · ORM-21 | Whole runtime is concurrent (BEAM) — no special API | `a`-prefixed methods, `async for`; async transactions unsupported · CORE · ORM-200,197 |
| Raw SQL escape hatch | `DB::select/statement`, raw expressions · CORE · ORM-4,12 | `find_by_sql/select_all` · CORE · ORM-17 | `fragment` + `Ecto.Adapters.SQL.query` · CORE · ORM-20,21 | `raw()`, cursors, `RawSQL`, legacy `extra()` · DIY · ORM-188–191,156 |
| Injection-safety mechanism | Bound parameters in builder/raw · CORE · ORM-12 | Placeholder arrays (string interpolation warned) · CORE · ORM-6 | Structural: `^var` pin required; string fragment = compile error · CORE · ORM-13,20 | Params required; `RawSQL` warned · CORE/DIY · ORM-191 |
| Query debugging / instrumentation | `DB::listen`, slow-budget alerts, `dd/dumpRawSql` · CORE · ORM-5,24 | `explain(analyze)`, `annotate` comments, optimizer hints · CORE · ORM-20 | — (telemetry in P20) | `explain()`, `execute_wrapper` · OPT/DIY · ORM-202,205 |
| Strictness modes | `shouldBeStrict` (lazy-load/discard/missing-attr) · CORE · ORM-27 | `strict_loading` + deprecated-association reporting · CORE · ORM-13,31 | Compile-time field checks (inherent) · CORE · ORM-12 | `RAISE` fetch mode · OPT · ORM-201 |
| Model factories (test data) | First-party factories: states, sequences, relationship builders, `recycle` · CORE · ORM-55 | — (fixtures only, P5 MIG-14; factory_bot is ecosystem) | — (seeds; ExMachina is ecosystem) | — (fixtures, P5 MIG-49) |
| Redis / NoSQL integration | Redis facade (clusters, pipelines, pub/sub) · CORE · ORM-57; MongoDB pkg · OPT · ORM-58 | — (Redis appears only as infra elsewhere) | ETS adapter · OPT · ORM-2 | Explicitly none ("no NoSQL" charter) · non-goals |
| Schema cache | — | Dump/load schema cache per DB · CORE · ORM-50 | — (compile-time schemas make it moot) | — |

**Notable divergences.** The deepest split is object model: Laravel/Rails/Django put persistence, query state, and business behavior on the model object (ActiveRecord), while Ecto refuses — structs are inert, only `Repo` does I/O, and mutation intent lives in changesets, which eliminates whole categories (lazy loading, lifecycle callbacks, global scopes) rather than guarding them. N+1 answers span the full spectrum accordingly: structural impossibility (Phoenix), opt-in strictness/auto-eager modes (Laravel, Rails), and a brand-new fetch-mode system retrofitted onto lazy loading (Django 6.1). Rails and Laravel treat callbacks/events as core architecture; Django keeps them at arm's length (signals, skipped by bulk ops); Phoenix has none on principle. Portability philosophy diverges: Django is DB-agnostic by charter with a sanctioned Postgres contrib exception, Rails embraces PG-native types in core, Laravel ships pgvector/full-text in the core builder, Ecto pushes DB-specific work to adapters/fragments. Only Rails ships optimistic locking and attribute encryption in core; only Laravel ships soft deletes, pagination (incl. cursor), and model factories in core.

**Volt notes.**
- Ecto is the natural comparison point for Go (explicit, no runtime magic, compile-checked queries) — but note how much convention-driven productivity Rails/Laravel extract from schema reflection and how Django derives forms/admin/migrations from one model declaration; Go's static types could play the "single source of truth" role that Django's model class plays.
- Every framework converged on the same guardrail set from different directions: N+1 prevention, parameterization-by-construction, race-free atomic updates, and keyset/cursor iteration for large tables — table stakes for Volt's query layer.
- Absence patterns are informative: nobody ships replica load-balancing (Rails marks it explicitly out of scope), and pagination/optimistic-locking/factories are each core in exactly one framework — cheap differentiation opportunities.
- Laravel 13's pgvector-in-the-query-builder (`whereVectorSimilarTo`, auto-embedding) is the only first-party vector story among the four; Django's answer is still "contrib.postgres has no vector module."

## P5 — Schema evolution: migrations & seeding

- **Laravel:** class-based up/down migrations with a fluent 5-dialect schema builder, batch-tracked rollback, schema squashing, seeder classes composing factories.
- **Rails:** timestamped `change` migrations that auto-reverse, name-aware generators, a dumped canonical `schema.rb`/`structure.sql` used to build fresh DBs, `db/seeds.rb`.
- **Phoenix:** timestamped modules with reversible `change/0`, mix tasks, a `schema_migrations` ledger, plain-Elixir `seeds.exs`; release-friendly migration story without Mix.
- **Django:** autodetected per-app migration files as "schema version control" — `makemigrations` diffs model state into operation graphs; dependencies, reversal, squashing, fake-apply, `RunPython` data migrations with historical models; fixtures for seed data.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Migration authoring model | Hand-written `up`/`down` classes · CORE · MIG-1,2 | Hand-written `change` (auto-reversed) · CORE · MIG-1 | Hand-written `change/0` (auto-reversed) · CORE · MIG-2 | **Autogenerated** operation lists diffed from models · CORE · MIG-1,9 |
| Generation / codegen | `make:migration` with name-based inference · CORE · MIG-1 | Name-aware generators (`AddXToY x:string:index`) · CORE · MIG-2 | `ecto.gen.migration`; `phx.gen.*` emit migrations · CORE · MIG-1 | `makemigrations` autodetector (`--empty/--dry-run`) · CORE · MIG-1 |
| Ordering & dependencies | Timestamp order · CORE · MIG-1 | Timestamp order · CORE · MIG-1 | Timestamp order · CORE · MIG-1 | Explicit per-app dependency graph, `run_before`, swappable deps · CORE · MIG-6–8 |
| Version ledger | Batch-tracked table · CORE · MIG-3 | `schema_migrations` + `db:migrate:status` · CORE · MIG-12 | `schema_migrations` · CORE · MIG-7 | Recorded graph + history-consistency check, merge-conflict linearization prompt · CORE · MIG-10 |
| Rollback / reversal | `rollback --step/--batch`, reset/refresh · CORE · MIG-3 | `db:rollback`, `redo`, STEP/VERSION, `revert`, IrreversibleMigration · CORE · MIG-8,9 | `ecto.rollback` (`--step/--to`) · CORE · MIG-8 | `migrate app 0002` / `zero`; IrreversibleError · CORE · MIG-12 |
| Table/column DSL | ~100 column types, rich modifiers, `change()` · CORE · MIG-6–9 | Full type set + modifiers, `change_table` · CORE · MIG-3,4 | `create/alter/add/drop`, precision/null/default · CORE · MIG-2,5 | Operation classes (`CreateModel/AddField/AlterField/Rename*`) · CORE · MIG-29–39 |
| Index DSL | primary/unique/fulltext/spatial/vector(HNSW) · CORE · MIG-10 | unique/multi-column/named · CORE · MIG-6 | `create index/unique_index`, composite + leftmost-prefix guidance · CORE · MIG-4 | AddIndex/RemoveIndex/RenameIndex + `Meta.indexes` (P4) · CORE · MIG-40, ORM-64 |
| FK / constraint DSL | `foreignId()->constrained()`, cascade helpers · CORE · MIG-11 | `add_foreign_key`, `add_check_constraint`, `add_reference` · CORE · MIG-5,6 | `references(on_delete:)` — DB integrity favored over app cleanup · CORE · MIG-3 | AddConstraint/AlterConstraint + model-declared Check/Unique constraints (P4 ORM-66) · CORE · MIG-41 |
| Raw SQL in migrations | — (no dedicated row; `DB` facade usable) | `execute` · CORE · MIG-7 | — (not in corpus) | `RunSQL` (reverse_sql, state_operations, elidable) · CORE · MIG-42 |
| Data migrations | — (seeders are separate) | Explicitly out: keep data out of schema migrations (gems/rake) · DIY · MIG-15 | — (seeds/contexts instead) | First-class: `RunPython` + historical models, batched non-atomic recipes · CORE · MIG-13,15,16,43 |
| Atomic / transactional DDL | — (only FK-constraint toggling, MIG-2) | — (not in corpus) | — (not in corpus) | Atomic per-migration on capable backends, opt-out · CORE · MIG-5 |
| SQL preview | `--pretend` · CORE · MIG-2 | — | — | `sqlmigrate` · CORE · MIG-1 |
| Squashing / schema snapshot | `schema:dump --prune` → SQL file + remaining migrations · CORE · MIG-5 | `schema.rb`/`structure.sql` dump is *canonical*; squash-to-schema workflow · CORE · MIG-11,18 | — (not in corpus) | `squashmigrations` w/ operation optimizer, `replaces`, elidable · CORE · MIG-18 |
| Fresh-environment bootstrap | `migrate:fresh --seed`; dump-then-migrate · CORE · MIG-3,5 | `db:prepare` idempotent create+load+migrate+seed; fresh DB loads schema first (8.0) · CORE · MIG-9,10 | `ecto.create` + `ecto.migrate` · CORE · MIG-6,7 | Run full migration graph; `--fake-initial` for existing DBs · CORE · MIG-11 |
| DB create/drop tasks | — (via `db` CLI only, ORM-7) | `db:create/drop/setup/reset` · CORE · MIG-10 | `ecto.create` / `ecto.drop` · CORE · MIG-6 | — (test DBs only) |
| Production deploy safety | `--force`, `--isolated` cache-lock (one server migrates) · CORE · MIG-4 | — | `phx.gen.release` `bin/migrate` overlay (no Mix at runtime); dev out-of-date checker w/ migrate button · CORE · MIG-11,12 | — |
| Multi-DB / tenant migrations | — | Per-DB `migrations_paths`, scoped tasks · CORE · MIG-17 | `--prefix` + per-table prefix + `flush()` · CORE · MIG-9 | Router `allow_migrate` + `schema_editor.connection.alias` · OPT · MIG-17 |
| Migration hooks / events | Full event set (Started/Ended/SchemaDumped...) · CORE · MIG-13 | — | — | — |
| Fake / out-of-band application | — | — | — | `--fake/--fake-initial/--prune`; `SeparateDatabaseAndState` · CORE/OPT · MIG-1,44 |
| Custom operations / DDL abstraction | — | — | — | `SchemaEditor` interface + custom `Operation` subclasses · OPT/DIY · MIG-28,47 |
| Serialization of model state | n/a (no autodetection) | n/a | n/a | Value serializer rules, custom serializers, `deconstruct()` · CORE/DIY · MIG-19–21 |
| Extension enablement (pgvector etc.) | `ensureVectorExtensionExists` + `vector()` columns · CORE · MIG-12,7 | — | — | `CreateExtension` + per-extension ops, `CreateCollation`, concurrent index ops · OPT · PG-10 |
| Zero-downtime recipes | `--isolated` only · MIG-4 | — | — | Documented: add-unique-non-null 3-step, concurrent indexes, NOT VALID constraints · OPT · MIG-24, PG-10 |
| Seeding | Seeder classes + factories, `db:seed --class` · CORE · MIG-14 | `db/seeds.rb`, `db:seed:replant` · CORE · MIG-14 | `priv/repo/seeds.exs` via `mix run` calling context fns · CORE · MIG-10 | Data migrations (auto-applied incl. tests) · CORE · MIG-48 |
| Fixtures (serialized data files) | — | `db:fixtures:load` · CORE · MIG-14 | — | JSON/XML/YAML fixtures, discovery, compression, per-DB, raw-save semantics · CORE/OPT · MIG-49–55 |
| Third-party/engine migrations | — | `railties:install:migrations` copies engine migrations · CORE · MIG-16 | — | Third-party app compat rules · OPT · MIG-23 |

**Notable divergences.** The core split is authored vs derived: Laravel/Rails/Phoenix have developers write migrations (with generators helping), while Django *derives* them by diffing model state — which then forces a large secondary apparatus (dependency graphs, historical models, value serialization, `deconstruct()`, fake-apply) that the other three simply don't need. Source-of-truth differs too: Rails treats the dumped schema file as canonical (fresh DBs load it, not the migration chain), Laravel makes squashing an optional optimization, Django replays the graph, Phoenix's corpus doesn't mention squashing at all. Data migrations are first-class in Django (`RunPython` runs even during test-DB setup), explicitly rejected by Rails ("keep it out, use gems"), and absent from Laravel/Phoenix, which route data through seeders. On production deployment, only Laravel (`--isolated` lock) and Phoenix (Mix-free release binaries, dev drift checker) address the multi-server/runtime story; Django alone addresses transactional DDL and zero-downtime patterns (concurrent indexes, NOT VALID).

**Volt notes.**
- Autodetected migrations (Django-style) require serializing model state into files — heavy in Go without reflection-friendly metaprogramming; the authored-with-smart-generators model (Rails/Laravel/Phoenix) plus a canonical schema snapshot (Rails) looks like the lower-complexity path.
- Phoenix's release-binary migration story (`bin/migrate` without the build toolchain) is the closest analogue to Go's deploy reality (single static binary, no interpreter at runtime) — worth first-class treatment, as is Laravel's `--isolated` multi-server lock.
- Auto-reversal of a single `change` function (Rails/Phoenix) vs explicit up/down (Laravel) is a real DX divide; auto-reverse depends on a closed DSL vocabulary, which suits a typed builder API.
- Seeding splits into code-that-calls-your-app (Rails/Phoenix/Laravel) vs serialized data files (Django fixtures, whose raw-save signal semantics generate their own caveat list); the code path ages better with schema drift.

## P6 — Validation & data integrity

- **Laravel:** one ~100-rule vocabulary used three ways (inline `validate()`, FormRequest classes, manual Validator) at the HTTP boundary, with automatic redirect-or-422 behavior and deep array validation.
- **Rails:** model-level validations as the primary line of defense — declarative validator macros, an errors object wired into forms and i18n, DB constraints framed as complementary.
- **Phoenix:** `Ecto.Changeset` — explicit cast-then-validate pipelines that whitelist fields, split cheap in-app validations from race-proof DB-backed constraints, and double as the error-carrying contract between layers.
- **Django:** the forms system *is* the validation layer — Form/ModelForm field pipelines, `clean_<field>`/`clean()` hooks, widgets and formsets; shared validator callables; model `full_clean()` + DB constraints as the back-stop (but `save()` never validates).

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Primary validation locus | HTTP boundary (request) · CORE · VAL-1 | Model object, runs on save · CORE · VAL-1 | Changeset pipeline (context boundary) · CORE · VAL-1 | Form classes; model `full_clean` as second layer · CORE · VAL-1,54 |
| Coupling to persistence | None — validation precedes model | Automatic on `save/create/update`; bang variants raise; skip hatches · CORE · VAL-1 | Repos accept changesets; invalid never reaches DB · CORE · VAL-12 | Decoupled: `save()` does NOT validate; `full_clean` explicit · CORE · VAL-54 |
| Built-in rule vocabulary | ~100 rules (types, comparisons, presence matrix, sets) · CORE · VAL-9 | Presence/absence, acceptance/confirmation, format/inclusion, length/numericality/comparison · CORE · VAL-2–5 | Small core: required/length/format/number · CORE · VAL-2–5 | Validator callables (regex/email/URL/range/length/step/file) shared by models+forms · OPT · VAL-45–48 |
| Type coercion at the boundary | — (validates, doesn't cast; casts live on the model, ORM-52) | Attributes API casting (P4 ORM-36) | `cast/3` casts params against schema types · CORE · VAL-1 | Per-field `to_python()` → typed `cleaned_data` · CORE · VAL-40,3 |
| Mass-assignment defense | `$fillable/$guarded` on model · CORE · ORM-31 | Strong params (P2, controller layer) | `cast` whitelist — unknown keys dropped by design · CORE · VAL-1 | Explicit `Meta.fields` on ModelForm ("strongly recommended") · CORE · VAL-50 |
| Form-object / write-model layer | FormRequest classes (rules+authorize+hooks) · CORE · VAL-4 | ActiveModel for non-DB objects (P4 ORM-37) · CORE | `embedded_schema` write-models; schemaless `{data, types}` changesets · CORE/DIY · VAL-10,11, ORM-32 | Form/ModelForm classes; `modelform_factory` · CORE/OPT · VAL-1,50,55 |
| Cross-field validation | `confirmed/same/different/gt-lt` rules, `after()` hooks · CORE · VAL-9,4 | Comparison validator; `validates_with` whole-record classes · CORE · VAL-5,7 | Custom changeset functions in pipeline (composition) · CORE · VAL-1 | `Form.clean()` + `add_error` · CORE · VAL-42 |
| Conditional validation | `sometimes`, `Rule::when/unless`, `exclude` family · CORE · VAL-11 | `:if/:unless`, `:on` contexts, `with_options` · CORE · VAL-8,9 | Per-action changeset functions (idiomatic; not a DSL) | — (partial: `full_clean(exclude=)`) |
| Custom validators | Rule objects + closures + implicit rules · CORE · VAL-15 | Validator/EachValidator classes, `validate` methods · CORE · VAL-10 | Any `changeset -> changeset` function · CORE · VAL-1 | Callables raising ValidationError (+deconstruct for migrations) · DIY · VAL-49,30 |
| Uniqueness handling | `unique`/`exists` DB query rules (`Rule::unique()->ignore()`) · CORE · VAL-10 | Uniqueness validator + documented race caveat → pair with unique index · CORE · VAL-6 | `unique_constraint` converts DB violation into changeset error — race-proof by construction · CORE · VAL-8 | `validate_unique` in full_clean + `UniqueConstraint` validated too · CORE · VAL-53,54, ORM-70,72 |
| DB-constraint philosophy | Constraints in migrations; no error translation | "App validation primary, DB complementary" · CORE · VAL-14 | DB as source of truth (not-null, unique, cascades); constraints ≠ validations, explicit split · CORE · VAL-8,15 | Constraints declared on model, checked during validation AND enforced in DB · CORE · ORM-66,70–78 |
| Error data structure | MessageBag (wildcards, named bags) · CORE · VAL-3,7 | `errors` object (`where/full_messages/details/add`) · CORE · VAL-11 | Keyword errors w/ metadata + `traverse_errors` flattening · CORE · VAL-6,7 | `errors` dict, `as_data/as_json`, `add_error/non_field_errors` · CORE/OPT · VAL-3–5 |
| Message customization & i18n | Lang-file per-rule/attribute overrides · CORE · VAL-8 | Model/attribute-scoped i18n lookup chain · CORE · VAL-13 | `%{count}`-style interpolation metadata (gettext in P15) · CORE · VAL-6 | `ValidationError(code, params)` + gettext · OPT · VAL-43 |
| HTML form error integration | `$errors` shared to views, `@error` directive · CORE · VAL-3 | Auto `.field_with_errors` wrapper, error-list pattern · CORE · VAL-12 | Changeset implements FormData protocol; forms render from it · CORE · VAL-13 | Full rendering system: BoundField, CSS/aria hooks, templates · CORE/OPT · VAL-9–14 |
| Untouched-field UX | Precognition per-field client helpers · CORE · VAL-16 | — | `_unused_` params + `used_input?` suppress premature errors · CORE · VAL-14 | — |
| JSON API error shape | Auto-422 `{message, errors}` for XHR · CORE · VAL-1 | — (not in corpus) | Generated `ChangesetJSON` via traverse_errors · CORE · VAL-7 | `errors.as_json()` · OPT · VAL-4 |
| Nested / collection validation | `*` wildcards, `array:keys`, index placeholders · CORE · VAL-12 | `validates_associated` · CORE · VAL-7 | `cast_assoc/cast_embed` (child errors invalidate parent) · CORE · VAL-9 | Formsets (management form, min/max, ordering/deletion, inline FK editing) · CORE/OPT · VAL-56–65 |
| File upload validation | Fluent `File::types()->max()`, image dimensions · CORE · VAL-13 | — (not in corpus) | — (not in corpus; uploads in P13) | `FileExtensionValidator`, image validation, multi-file recipe · OPT/DIY · VAL-48,66–68 |
| Password strength rules | `Password::min()->uncompromised()` (pwned check) app defaults · CORE · VAL-14 | `has_secure_password` confirmation/challenge (P4 ORM-38) | — (in phx.gen.auth, P7) | — (password validators live in auth, P7) |
| Live / predictive validation | Precognition middleware + npm form helpers · CORE · VAL-16 | — | LiveView `phx-change` per-field validation · CORE · VAL-14 | — |
| Validated-data access | `validated()`/`safe()->only/except` · CORE · VAL-6 | — (model attrs are the data) | `changes`, `apply_changes/1` · CORE · VAL-6 | `cleaned_data` (declared fields only) · CORE · VAL-3 |
| Pre-validation normalization | `prepareForValidation` hook · CORE · VAL-4 | `normalizes` on assignment (P4 ORM-39) · CORE | Transform steps in pipeline (plain functions) | `to_python` coercion; `clean_<field>` returns replacement · CORE · VAL-40,41 |
| Widget / input rendering layer | — (Blade/frontend; not part of validation) | — (form helpers in P3) | — (function components in P3) | Full widget system (30+ widgets, Media assets, templates) bundled *inside* the validation layer · CORE/OPT · VAL-31–39 |
| Stop-on-first-failure control | `bail` per field, `stopOnFirstFailure` · CORE · VAL-2,4 | `:strict` raise option · CORE · VAL-8 | Pipeline order is explicit code | — (all errors collected) |
| DB rules querying the DB | `exists`/`unique` rules · CORE · VAL-10 | Uniqueness/associated validators · CORE · VAL-6,7 | Rejected — constraints handle DB truth · VAL-8 | `ModelChoiceField` queryset membership · CORE · VAL-29 |

**Notable divergences.** Four genuinely different answers to "where does validation live": the request boundary (Laravel), the model object (Rails), an explicit pipeline value between boundary and repo (Phoenix), and a form class that also owns HTML rendering (Django — uniquely, widgets and template machinery are inside the validation layer, and `save()` never validates, so skipping `full_clean` silently skips validation). The uniqueness race is the sharpest philosophical marker: Rails documents the TOCTOU caveat and says add an index; Laravel queries the DB as a rule; Phoenix refuses to pretend — `unique_constraint` translates the actual DB violation into a user-facing error; Django does both (validator query + constraint validation + DB enforcement). Mass assignment is likewise structural in Phoenix (cast whitelist per call site) but configuration in the others (fillable/guarded, strong params, Meta.fields). Only Laravel and Phoenix have a first-party predictive/live validation story (Precognition; LiveView change events).

**Volt notes.**
- The changeset insight most portable to Go: constraint errors are *data*, not exceptions — translating driver unique/FK violations into the same error structure as field validations gives race-proof uniqueness with one UX; Go's `(T, error)` returns fit `{:ok, _}/{:error, changeset}` naturally.
- All four converge on one error-value contract consumed by both HTML forms and JSON APIs (MessageBag / errors object / changeset / errors dict) — designing that struct first, independent of transport, appears to be the load-bearing decision.
- Cast-then-validate (Phoenix/Django) doubles as mass-assignment defense and type coercion in one step; in Go, decoding `map[string]any` → typed struct with a field whitelist is the same move and avoids a separate "rules language."
- Laravel's ~100-rule string DSL trades compile-time safety for terseness — the exact opposite tradeoff a Go framework would make; Rails' `:on` contexts and Phoenix's per-action changeset functions are two ways to express "different rules for create vs update," a problem Volt must answer either way.

### Aside — database-native extras (Django P23, Laravel P22)

Already folded into P4/P5 above: vector search (Laravel SRCH-2/ORM-18 vs nothing elsewhere), PG-native types/indexes/exclusion constraints (Django PG-1–8 vs Rails ORM-49 in core), extension/concurrent-index migration ops (Django PG-10 vs Laravel MIG-12), and PG full-text/trigram (Django PG-4–6, Laravel SRCH-1). Not carried in: Laravel Scout (index-synced external search engines, SRCH-5–8) — an application-search product layer rather than a data-layer capability; Django PG form fields (PG-13) — belongs to the forms layer.

## P7 — Authentication & authorization

- **Laravel**: guard/provider architecture with session auth in core, plus a layered first-party package stack — starter kits (UI), Fortify (headless + 2FA + passkeys), Sanctum (tokens/SPA), Passport (OAuth2 server), Socialite (social login) — and a full gate/policy authorization system in core.
- **Rails**: since 8.0, a *generated* session auth system (User + DB Session models, controllers, mailer) built on core primitives (`has_secure_password`, `authenticate_by`, `generates_token_for`, `rate_limit`, `Current`); authorization is deliberately left to app code or gems.
- **Phoenix**: `mix phx.gen.auth` generates a complete owned-by-you auth system (magic links by default, confirmation, sudo mode, hashed token table), and v1.8 **Scopes** thread an authorization context through every generated controller, LiveView, and context function.
- **Django**: `django.contrib.auth` — a swappable User model, pluggable backends behind `authenticate()`, model-level permissions + groups, packaged views/forms for login/password flows, and a tunable auto-upgrading hasher stack; contrib (OPT) but enabled by `startproject`.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Identity architecture | guard + user-provider config, multiple guards · CORE · AUTH-1 | generated User + DB Session models (app code, IP/UA metadata) · CORE · AUTH-1 | phx.gen.auth generated, explicitly owned code · CORE(gen) · AUTH-1/2 | contrib.auth User model · OPT · AUTH-1 |
| Full-flow scaffolding (UI + flows) | starter kits (Inertia/Livewire) · OPT · AUTH-17; headless Fortify backend · OPT · AUTH-19 | generator: controllers, concern, mailer, routes · CORE · AUTH-1; signup/settings flows tutorial-only · DIY · AUTH-10 | LiveView or controller flavor generated · CORE(gen) · AUTH-1 | login/logout/password views + forms (no signup view) · OPT · AUTH-9/10/11 |
| Current-user access | `Auth::user()`, `$request->user()`, per-guard · CORE · AUTH-2 | `Current.user`/`Current.session` (CurrentAttributes) · CORE · AUTH-8 | `:current_scope` assign; `current_user` as authz subject · CORE · AUTH-3/7/15 | `request.user` / async `auser()`; `AnonymousUser` null-object · OPT · AUTH-6/7 |
| Route/view login guards | `auth` middleware (guard-param) · CORE · AUTH-3 | generated `Authentication` concern `before_action` · CORE · AUTH-1 | `require_authenticated_user` plug + `on_mount` LiveView hooks · CORE(gen) · AUTH-3 | `login_required` decorator, site-wide `LoginRequiredMiddleware`, CBV mixins · OPT · AUTH-24/25/27 |
| Manual login/credential API | `Auth::attempt`/`login`/`once` + remember-me · CORE · AUTH-4 | `authenticate_by` (enumeration-hardened), `authenticate` · CORE · AUTH-2/3 | generated context functions · CORE(gen) · AUTH-1 | `authenticate()`/`login()`/`logout()` + async variants · OPT · AUTH-4/5 |
| Password hashing | bcrypt default, argon2/argon2id, tunable · CORE · SEC-5 (P8) | `has_secure_password` (bcrypt) · CORE · AUTH-2 | Comeonin: bcrypt/pbkdf2/argon2 via `--hashing-lib` · CORE(gen) · AUTH-4 | PBKDF2 default; scrypt in-box; argon2/bcrypt via libs; standalone utils · OPT/ECO · AUTH-30/31/32/35 |
| Hash upgrade / auto-rehash | rehash on work-factor change · CORE · AUTH-9 | — (bcrypt only) | — | upgrade-on-login, work-factor tuning, wrapped-hasher offline upgrade · OPT/DIY · AUTH-33/34 |
| Password strength/compromise rules | `uncompromised()` HIBP k-anonymity rule · CORE · SEC-11 (P8) | presence/length/confirmation validations · CORE · AUTH-2 | — (changeset validations implicit in AUTH-1) | `AUTH_PASSWORD_VALIDATORS` incl. 20k common-password list · OPT · AUTH-36/37 |
| Password reset | token broker, DB/cache storage, expiry + throttle · CORE · AUTH-16 | generated flow, `find_by_password_reset_token!`, mailer · CORE · AUTH-4 | — (magic-link login fills the role · AUTH-1) | `PasswordReset*` view suite + token generator · OPT · AUTH-10 |
| Email verification / confirmation | `MustVerifyEmail` + signed links + `verified` middleware · CORE · AUTH-15 | email-change confirmation pattern (tutorial) · DIY · AUTH-10 | registration + email confirmation generated · CORE(gen) · AUTH-1 | — |
| Magic-link / passwordless login | — (WorkOS variant only · ECO · AUTH-18) | — | default login mode; password auth is the opt-in · CORE(gen) · AUTH-1 | — |
| Re-auth for sensitive actions (sudo) | `password.confirm` middleware + timeout · CORE · AUTH-7 | `password_challenge` · CORE · AUTH-2 | sudo mode + `require_sudo_mode` plug · CORE(gen) · AUTH-1/3 | — |
| Two-factor auth (TOTP) | Fortify TOTP + QR + recovery codes · OPT · AUTH-20 | — | — | — |
| Passkeys / WebAuthn | Fortify passkeys + `@laravel/passkeys` JS · OPT · AUTH-21 | — | — | — |
| Social login (OAuth client) | Socialite, normalized user object · OPT · AUTH-26 | — | Ueberauth (blessed) · ECO · AUTH-16 | — (explicitly not provided · SEC-16 (P8)) |
| OAuth2 authorization server | Passport: all grants + PKCE + device flow, scopes · OPT · AUTH-24/25 | — | — | — |
| API token auth | Sanctum hashed tokens, abilities, expiry, prune · OPT · AUTH-22 | HTTP Token auth · CORE · AUTH-9 | hashed api-token recipe on generated `UserToken` · DIY · AUTH-11 | — (DRF ECO) |
| SPA cookie auth for APIs | Sanctum stateful domains + CSRF-cookie handshake · OPT · AUTH-23 | — | — | — |
| HTTP Basic/Digest auth | `auth.basic`, stateless `onceBasic` · CORE · AUTH-5 | Basic/Digest/Token · CORE · AUTH-9 | — | — |
| External/SSO delegation | WorkOS AuthKit (SSO, social, magic) · ECO · AUTH-18 | — | — | `REMOTE_USER` middleware + backend (web-server auth) · OPT · AUTH-16 |
| Server-side session store, per-device revocation | `logoutOtherDevices` + `AuthenticateSession` · CORE · AUTH-6 | DB-backed Session model enables revocation · CORE · AUTH-1/7 | hashed `users_tokens` table, per-device tracking, global invalidation on password change · CORE(gen) · AUTH-5 | logout flushes session; invalidation on password change via session-auth hash · OPT · AUTH-5/12 |
| Long-lived connection revocation | — | — | `live_socket_id` broadcast disconnect kills all LiveViews/channels · CORE · AUTH-13 | — |
| WebSocket/channel auth | Sanctum private broadcast channel authz · OPT · AUTH-23 | — | `Phoenix.Token` sign/verify; dual-entry (plug + mount) security model, re-check in `handle_event` · CORE · AUTH-12/14 | — |
| Purpose-scoped expiring tokens | signed URLs · CORE · SEC-9 (P8) | `generates_token_for :purpose, expires_in:`, state-invalidated · CORE · AUTH-5 | `Phoenix.Token` with `max_age:` · CORE · AUTH-12 | `Signer`/`TimestampSigner` · CORE · SEC-10/11 (P8) |
| Login throttling | username+IP via kits/Fortify; DIY via rate limiter · CORE · AUTH-11 | controller-level `rate_limit` · CORE · AUTH-6 | — | — (explicit non-goal · SEC-16 (P8)) |
| Enumeration/timing hardening | — | `authenticate_by` hardened by default · CORE · AUTH-3 | explicitly **not** attempted, tradeoff documented · — · AUTH-17 | hasher `harden_runtime` hook · DIY · AUTH-34 |
| Declarative authorization framework | Gates + Policies (auto-discovery, before/after, Response objects w/ status) · CORE · AUTH-12/13 | none: role flags + `before_action` guards · DIY · AUTH-11 | `%Scope{}` struct threaded as first arg to every context fn · CORE · AUTH-7 | model-level Permission + Group models, custom perms · OPT · AUTH-18/20/22/23 |
| Object-level authorization | policy methods receive the model instance · CORE · AUTH-13 | scope queries to `Current.user` (privilege-escalation pattern) · DIY · AUTH-11 | generated `where: post.user_id == ^scope.user.id` filters · CORE · AUTH-8 | `obj` param hooks exist but core returns empty; third-party · ECO · AUTH-21 + SEC-16 (P8) |
| Enforcement surfaces (controller/route/template) | `can:` middleware, `#[Authorize]`, `Gate::authorize`, Blade `@can` · CORE · AUTH-14 | `before_action` guards · DIY · AUTH-11 | plugs + `on_mount` hooks + `handle_event` re-checks · CORE · AUTH-3/14 | `permission_required`, `user_passes_test`, CBV mixins, `{{ perms }}` context · OPT · AUTH-15/26/27 |
| Multi-tenancy scoping of generated code | — | — | scope config drives generators: FK columns, `route_prefix`, org scopes, scoped PubSub · CORE · AUTH-8/9/10 | — |
| Pluggable backends / custom guards | `Auth::extend`, `viaRequest` closures, custom `UserProvider` · CORE · AUTH-8 | all app code — edit directly · CORE · AUTH-1/2 | generated code is application code · CORE · AUTH-2 | `AUTHENTICATION_BACKENDS` chain, async backend iface · OPT/DIY · AUTH-17/28 |
| Swappable/custom user model | `Authenticatable` contract · CORE · AUTH-8 | User model is app code · CORE · AUTH-1 | generated schema is app code · CORE(gen) · AUTH-1 | `AUTH_USER_MODEL` + `AbstractBaseUser`/`AbstractUser` bases · DIY · AUTH-29 |
| Auth lifecycle events/signals | full event set (Login, Failed, Lockout, Verified…) · CORE · AUTH-10 | — | — | `user_logged_in/out/login_failed` signals · OPT · AUTH-13 |
| Admin/user management CLI | — | — | — | `createsuperuser`/`changepassword` · OPT · AUTH-3 |
| Auth testing helpers | `actingAs`, `Sanctum::actingAs`/`Passport::actingAs`, assertions · CORE/OPT · AUTH-27 | — | — | — |
| Full-auth ecosystem alternative | — (first-party stack is the answer) | Devise named · ECO · AUTH-12 | Ueberauth named · ECO · AUTH-16 | — |

**Notable divergences.** The deepest split is *where the auth code lives*: Rails 8 and Phoenix generate editable application code (Phoenix even documents that security fixes will NOT auto-apply, AUTH-2), Laravel installs framework packages you configure but don't own, and Django ships a contrib app with a swappable-model escape hatch. Authorization gets four unrelated answers: Laravel has a complete gate/policy system in core; Django has a model-level permission/group framework whose object-level hooks are deliberately stubbed (ECO); Phoenix invented Scopes — an authz context threaded through generated queries, effectively compile-time row-level security; Rails ships nothing and documents role-flag patterns as DIY. Modern credential mechanisms diverge sharply: Phoenix defaults to magic links, Laravel is the only one with first-party 2FA, passkeys, an OAuth2 server, and a social-login client; Rails and Django remain password-first with ecosystem pointers. On account enumeration, Rails hardens by default (`authenticate_by`) while Phoenix explicitly declines and documents why — opposite defaults from the two generator-based frameworks.

**Volt notes.**
- The 2024–25 convergence among the newest designs (Rails 8, Phoenix 1.8) is *generated, owned auth code* over installed libraries. This fits Go unusually well (explicit code beats reflection), but the Phoenix AUTH-2 tradeoff — no automatic security patches for generated code — must be confronted, e.g. versioned generators + advisory tooling.
- Phoenix Scopes are the only structural answer to "authorization context flows through every query": a struct passed as the first argument everywhere. That is idiomatically identical to Go's `ctx`/first-param convention and is the most Go-transplantable idea in either section.
- The table-stakes union beyond login is large and consistent: email verification, password reset (or magic link), sudo re-auth, per-device server-side session revocation, login throttling, purpose-scoped expiring tokens, API tokens, auth test helpers. All four cover most of it; 2FA/passkeys/OAuth-server are where they stop and only Laravel continues (as OPT packages).
- Everyone hashes passwords with bcrypt/argon2-class algorithms and three of four handle work-factor upgrades transparently on login; hasher chains + auto-rehash are a framework concern, not something `x/crypto` gives you.

## P8 — Security

- **Laravel**: security is ambient — forgery protection (now `Sec-Fetch-Site`-first), cookie encryption, auto-escaping, and hashing are on by default; encryption/signing are one-liner services keyed from `APP_KEY`.
- **Rails**: secure-by-default stack (auto-escaping, CSRF, parameterized SQL, signed+encrypted cookies, headers, CSP DSL) plus a unique encrypted-credentials story and a guide-length threat manual; Brakeman scanning in default CI.
- **Phoenix**: safe-by-default primitives (HEEx escaping, pin-operator SQL params, CSRF plug, secure headers with default CSP) plus an unusually frank guide cataloguing how to defeat them; distinctive DoS focus for stateful connections.
- **Django**: secure-by-default behavior plus a stack of dedicated middleware all present in `startproject` — CSRF with Origin checking, `SecurityMiddleware` header suite, host validation, CSP (new in 6.0), and a signing API with key-fallback rotation.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| CSRF protection | `Sec-Fetch-Site` check first, session token fallback, strict `originOnly`, XSRF cookie for JS · CORE · SEC-1/2 | per-session/per-form tokens auto-injected in `form_with`, strategies · CORE · SEC-1 | `Plug.CSRFProtection` in `:browser`, hidden `_csrf_token`, token handed to LiveSocket; GET-action-reuse pitfall documented · CORE · SEC-1/2 | `CsrfViewMiddleware`: secret cookie, BREACH masking, Origin/Referer + trusted origins, rotation on login; AJAX header, decorators suite · CORE · SEC-5/6/7 |
| XSS / auto-escaping | Blade `{{ }}` escaping, `Js::from` · CORE · SEC-7 | ERB auto-escaping, `html_safe`/`raw` opt-outs, allowlist `sanitize` · CORE · SEC-2 | HEEx auto-escaping; bypass vectors (`raw/1`, content-type from input) documented · CORE · SEC-3 | template autoescaping + `mark_safe` caveats · CORE · SEC-1 |
| SQL injection defense | — (parameterized ORM; no P8 row) | parameterized query API + `sanitize_sql*` helpers · CORE · SEC-3 | pin-operator params; fragment interpolation is a *compile error* · CORE · SEC-5 | queryset parameterization; `raw()`/`extra()` cautions · CORE · SEC-2 |
| Mass-assignment defense | — (guarded attributes live in P4) | Strong Parameters required (`expect`/`permit`) · CORE · SEC-4 | changeset `cast` whitelist; `is_admin` canonical example · CORE · SEC-9 | — (form `fields` whitelists; no P8 row) |
| Password hashing service | bcrypt/argon2 drivers, `needsRehash`, unknown-hash rejection · CORE · SEC-5 | — (AUTH-2, P7) | — (AUTH-4, P7) | — (AUTH-30, P7) |
| Symmetric encryption API | `Crypt` AES+MAC keyed from `APP_KEY` · CORE · SEC-3 | — (encrypted cookies/credentials only, SEC-5/6) | — (`Plug.Crypto` adjacent, SEC-6) | — (signing only, SEC-10) |
| Signing / tamper-proof data & URLs | signed URLs, hash-verified verification links · CORE · SEC-9 | signed-cookie infrastructure · CORE · SEC-6 | signed session; `Phoenix.Token` · CORE · SEC-13 + AUTH-12 (P7) | `Signer`/`TimestampSigner`, `dumps()`/`loads()`, swappable backend · CORE · SEC-10/11 |
| Key rotation | `APP_PREVIOUS_KEYS` decrypt fallback + `key:generate` · CORE · SEC-4 | cookie rotation for secret/config changes · CORE · SEC-6 | — | `SECRET_KEY_FALLBACKS` (validate-only) · CORE · SEC-12 |
| Secrets management | env encryption (cross-ref CONF) · CORE · SEC-10 | `credentials.yml.enc` + master key, per-env files, `credentials:fetch` CLI · CORE · SEC-5 | — (runtime env config, P18) | — |
| Cookie encryption/signing defaults | encrypted + signed by default, exemptions configurable · CORE · SEC-6 | signed & encrypted jars from `secret_key_base` · CORE · SEC-6 | session cookie signed (optionally encrypted) · CORE · SEC-13 | — (secure/HttpOnly flags via settings · SEC-8 note) |
| Default security headers | — | `X-Frame-Options`, nosniff, referrer-policy, etc. by default · CORE · SEC-7 | `put_secure_browser_headers` incl. default CSP when unset · CORE · SEC-4 | `SecurityMiddleware`: HSTS, nosniff, referrer policy, COOP · CORE · SEC-8 |
| Clickjacking | — | `X-Frame-Options` default · CORE · SEC-7 | `frame-ancestors 'self'` in default CSP · CORE · SEC-4 | `XFrameOptionsMiddleware` (DENY) + per-view decorators · CORE · SEC-3/4 |
| Content Security Policy | — (nonce via Vite; documented gap) · SEC-12 | DSL: global + per-controller, auto nonces, report-only mode · CORE · SEC-8 | default baseline CSP via secure headers · CORE · SEC-4 | middleware + nonce sentinel/context/tag + per-view overrides (new 6.0/6.1) · CORE · SEC-13/14/15 |
| TLS enforcement / HSTS | — | `force_ssl` (HSTS, secure cookies, redirect), `assume_ssl` · CORE · SEC-9 | `force_ssl` + `Plug.SSL`, OWASP cipher-suite profiles, `phx.gen.cert` dev certs · CORE · SEC-10/11/12 | `SECURE_SSL_REDIRECT` + HSTS settings suite · CORE · SEC-8 |
| Host-header / DNS-rebinding defense | `trustHosts` middleware · CORE · SEC-8 | `HostAuthorization` allowlist middleware · CORE · SEC-10 | — (socket `check_origin` lives in P10) | `ALLOWED_HOSTS` validation in `get_host()` · CORE · SEC-9 |
| Reverse-proxy trust | `trustProxies` (header selection, wildcard) · CORE · SEC-8 | `assume_ssl` · CORE · SEC-9 | `rewrite_on: [:x_forwarded_proto]` · CORE · SEC-11 | `SECURE_PROXY_SSL_HEADER`, `USE_X_FORWARDED_HOST` · CORE · SEC-8/9 |
| Open-redirect protection | — | cross-origin `redirect_to` raises unless allowed · CORE · SEC-11 | — | — (login views' `success_url_allowed_hosts` only · AUTH-9 (P7)) |
| CORS | — | `rack-cors` recommended · ECO · SEC-18 | `CORSPlug` shown, wildcard anti-pattern warned · ECO · SEC-8 | — (django-cors-headers, not in inventory) |
| Sensitive-data log/param filtering | hidden attrs, encrypted casts, encrypted jobs (cross-refs) · CORE · SEC-10 | `filter_parameters` deep filtering + redirect filtering, encrypted attrs auto-added · CORE · SEC-15 | `password`/`token` params masked by default · CORE · SEC-16 | — |
| Session-attack countermeasures | invalidate/regenerate on logout · CORE · AUTH-6 (P7) | fixation/replay/hijack documented, `reset_session`, CookieStore guidance · CORE · SEC-12 | token-table invalidation · CORE · AUTH-5 (P7) | session-key rotation via auth hash · OPT · AUTH-12 (P7) |
| ReDoS mitigation | — | global `Regexp.timeout = 1s` default + anchoring guidance · CORE · SEC-13 | — | — |
| Request/connection DoS limits | — (rate-limiting pointer · SEC-12) | — | `Plug.Parsers` body/upload limits; `max_channels_per_transport`, longpoll batch caps · CORE · SEC-14/15 | — |
| Safe deserialization / RCE defense | — | command-injection guidance · CORE · SEC-19 | `non_executable_binary_to_term` (safe `binary_to_term`), never-eval guidance · CORE · SEC-6 | — |
| SSRF guidance | — | — | base-URLs-not-barriers patterns · DIY · SEC-7 | — |
| File-upload hardening | — | filename sanitization, traversal, executable placement patterns · DIY · SEC-16 | content-type validation warnings; size limits · CORE · SEC-3/14 | — |
| Compromised-password check | `uncompromised()` HIBP rule · CORE · SEC-11 | — | — | — (CommonPasswordValidator · AUTH-36 (P7)) |
| Security scanning toolchain | — | Brakeman + `bundler-audit` + `importmap audit` wired into generated CI · ECO · SEC-14 | — | — |
| Brute-force / CAPTCHA guidance | rate limiter cross-ref · CORE · SEC-12 | CAPTCHA/negative-captcha guidance · DIY/ECO · SEC-17 | — | — (explicit non-goal · SEC-16) |
| Threat-catalog documentation | per-page gap documentation · CORE · SEC-12 | guide-length threat manual (injection, header splitting, hijack flows) · CORE · SEC-19 | frank security guide + EEF hardening pointers · CORE · SEC-17 | topics/security.md overview (section preamble; no dedicated row) |

**Notable divergences.** All four agree on the safe-by-default core (auto-escaped templates, parameterized queries, CSRF for browser routes), but Laravel 13 is alone in flipping CSRF to `Sec-Fetch-Site` origin verification first with tokens as fallback — the other three remain token-first. Default security headers and CSP split the field: Rails, Phoenix, and Django all ship header middleware with CSP support in core (Phoenix even applies a baseline CSP when none is set), while Laravel ships no header middleware and documents CSP as a gap. Secrets are philosophical: Rails uniquely ships encrypted-credentials-in-repo; Laravel encrypts env files; Django and Phoenix defer entirely to the environment. Rails is alone in treating static security analysis (Brakeman in generated CI) and ReDoS (global regex timeout) as framework concerns, while Phoenix is alone in treating connection-level DoS (channel caps, slow-client body limits) as first-class — a direct consequence of its stateful-connection architecture. Django and Phoenix both publish explicit non-goals (throttling/object-perms/OAuth for Django; enumeration protection for Phoenix), which the other two do not.

**Volt notes.**
- Go starts ahead on two universal rows: `html/template` does contextual auto-escaping and `database/sql` placeholders parameterize queries — but Phoenix shows the bar is higher (making unsafe interpolation a *compile error*, not a runtime footgun), which Go's `vet`-style static tooling could match.
- Every framework derives cookies/signing/encryption from one app secret and three of four support graceful rotation via fallback key lists (`APP_PREVIOUS_KEYS`, `SECRET_KEY_FALLBACKS`, Rails rotations). Multi-key verify/single-key sign should be in Volt's crypto core from day one, not retrofitted.
- Laravel's `Sec-Fetch-Site`-first CSRF is the modern, mostly-stateless design a new framework can adopt outright, keeping tokens only as a fallback for old clients.
- Security-header + CSP middleware is now table stakes (3 of 4 in core, on by default via project template); Rails' Brakeman-in-CI precedent maps naturally to wiring `govulncheck`/`gosec` into Volt's generated CI rather than building a scanner.

## P9 — Background work: jobs, queues & scheduling

- **Laravel**: the maximal answer — driver-abstracted queue (Redis/DB/SQS/…) with the deepest job feature set of the four (uniqueness, debounce, middleware, chains, batches), a code-defined cron scheduler, and Horizon (OPT) as Redis dashboard/supervisor.
- **Rails**: Active Job as backend-agnostic declaration/enqueue API; Solid Queue (DB-backed, `FOR UPDATE SKIP LOCKED`) as the zero-extra-infrastructure CORE default, with a built-in recurring-task scheduler and Mission Control dashboard (OPT).
- **Phoenix**: ships **no job framework** — its largest deliberate gap. OTP supervised processes (P22) are the low-level answer; Oban is the blessed ECO answer.
- **Django**: Tasks framework (new 6.0) — `@task` + `.enqueue()` + results over pluggable backends, but deliberately **no worker process, no durable queue, no scheduler** in core; those are ECO.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Job/task declaration API | Job classes `ShouldQueue` + `make:job` · CORE · JOB-3 | Active Job subclasses + generator · CORE · JOB-1 | — (supervised GenServer · DIY · JOB-3; Oban · ECO · JOB-2) | `@task` decorator → `Task` · CORE · JOB-1 |
| Backend/driver abstraction | One API over database/redis/SQS/beanstalkd/sync/null · CORE · JOB-1 | Pluggable adapters (Sidekiq, Resque, GoodJob…) + async/inline/test · CORE/ECO · JOB-17 | — | Pluggable backends + `supports_*` feature flags · CORE · JOB-9 |
| Default durable backend shipped | database/redis drivers · CORE · JOB-1 | Solid Queue (DB, SKIP LOCKED, separate queue DB) · CORE · JOB-3 | — (Oban ECO) | — (only dev Immediate/Dummy · CORE · JOB-7/8; durable = ECO · JOB-10) |
| Worker process shipped | `queue:work` daemon + `queue:listen` dev reload · CORE · JOB-18 | `bin/jobs` or Puma plugin; topology in `queue.yml` · CORE · JOB-3/4 | — (OTP processes stand in · DIY · JOB-3) | — by design; execution is external infra · ECO · JOB-10 |
| Backend failover | Ordered connection list, auto fallback on push failure · CORE · JOB-2 | — | — | — |
| Delayed/deferred execution | `delay()` / `withoutDelay` · CORE · JOB-8 | `set(wait:, wait_until:)` · CORE · JOB-2 | — | `run_after` via `Task.using` · CORE · JOB-3 (needs `supports_defer`) |
| Priorities | Worker `--queue` priority lists · CORE · JOB-18 | `queue_with_priority` within queues · CORE · JOB-6 | — | `priority` kwarg · CORE · JOB-1/3 |
| Named queues & routing | `onQueue/onConnection` + central `Queue::route` per class · CORE · JOB-8/12 | `queue_as` (static/block), name prefixes · CORE · JOB-5 | — | `queue_name` · CORE · JOB-1 |
| Retry/backoff policy | `$tries`/`retryUntil`/`maxExceptions`/backoff arrays/timeouts · CORE · JOB-13 | `retry_on` (wait/backoff/jitter) + `discard_on` · CORE · JOB-11 | — (supervisor restarts are process-level · CORE · BEAM-1/2) | — (attempt counter exposed via `TaskContext` · JOB-4; retry loop is the ECO worker's job) |
| Failed-job store & ops | `failed_jobs` table, retry/forget/flush/prune CLI, `failed()` hook · CORE · JOB-21 | `retry_on` dead-handler block · CORE · JOB-11; retry/discard UI · OPT · JOB-18 | — | `TaskResult` FAILED + `TaskError` traceback · CORE · JOB-5/6 |
| Job result retrieval | — (fire-and-forget; batches expose progress only) | — | — (`Task.await` on BEAM · DIY · BEAM-5) | `TaskResult`/`get_result`, statuses enum · CORE · JOB-5/6 |
| Unique jobs / dedup | `ShouldBeUnique(UntilProcessing)`, SQS dedup · CORE · JOB-4/14 | — | — | — |
| Debounced jobs | `#[DebounceFor]` last-dispatch-wins · CORE · JOB-5 | — | — | — |
| Concurrency limiting / overlap control | `WithoutOverlapping`, `RateLimited`, `ThrottlesExceptions` middleware · CORE · JOB-7 | `limits_concurrency to:, key:, group:` · CORE · JOB-8 | — (process primitives · DIY · BEAM-5) | — |
| Job middleware / callbacks | Custom + shipped middleware · CORE · JOB-7 | `before/around/after_enqueue`/`_perform` · CORE · JOB-12 | — | `TaskContext` only · CORE · JOB-4 |
| Bulk enqueue | `Bus::bulk` · CORE · JOB-9 | `perform_all_later` → `enqueue_all` single round-trip · CORE · JOB-7 | — | — |
| Chains (sequential workflows) | `Bus::chain` + `catch`, in-job append/prepend · CORE · JOB-11 | — | — | — |
| Batches (fan-out with completion callbacks) | `Bus::batch` (then/catch/finally, cancellation, nesting) · CORE · JOB-16 | — | — | — |
| Long-job resumability | — | Continuations: `step`/cursor resume after restarts · CORE · JOB-10 | — | — |
| Transaction-safe enqueue | `after_commit` option + per-dispatch afterCommit · CORE · JOB-10 | `enqueue_after_transaction_commit` opt-in · CORE · JOB-15 | — | `on_commit` pattern · CORE · JOB-2 |
| Argument/model serialization | `SerializesModels` re-fetch on dequeue · CORE · JOB-3 | GlobalID + custom serializers · CORE · JOB-13/14 | — | JSON-serializable args/returns · CORE · JOB-2 |
| Encrypted payloads | `ShouldBeEncrypted` · CORE · JOB-6 | — | — | — |
| Manual in-job control | `release($delay)`/`fail`/`delete` · CORE · JOB-15 | — | — | — |
| Recurring tasks / scheduler | Code-defined cron DSL (`routes/console.php`, `everySecond`…`cron()`) · CORE · JOB-27/28 | `config/recurring.yml` (Fugit), no OS cron · CORE · JOB-9 | — (`:telemetry_poller` interval MFA, measurement-oriented · CORE · JOB-4) | — (no scheduler in core) |
| Multi-server schedule coordination | `withoutOverlapping`/`onOneServer` locks · CORE · JOB-29 | — | — | — |
| Scheduler execution & hooks | Single cron → `schedule:run`, sub-minute, output capture, HTTP pings · CORE · JOB-30 | — | — | — |
| Ops dashboard | Horizon (Redis-only): supervisors-as-code, balancing, tags, metrics, alerts · OPT · JOB-25/26 | Mission Control — Jobs (inspect/retry/discard) · OPT · JOB-18 | — (`forward "/jobs"` to external job UI · DIY · JOB-5) | — |
| Worker lifecycle ops | `queue:restart/pause/resume`, signal handlers · CORE · JOB-19; Supervisor recipe · DIY · JOB-20 | Process/thread/signal lifecycle documented · CORE · JOB-4 | Supervision tree restarts, no external PM needed · CORE · BEAM-1/2 | — |
| Queue monitoring/alerts | `queue:monitor` → `QueueBusy` event; Horizon long-wait alerts · CORE/OPT · JOB-22/26 | — | — | — |
| Lifecycle events / error reporting | `JobQueued/Processing/Processed/Failed` events · CORE · JOB-24 | Wrapped in `Rails.error`, locale propagation · CORE · JOB-16 | — (Oban telemetry · ECO · JOB-2) | Errors on `TaskResult` · CORE · JOB-5/6 |
| Testing support | `Queue::fake`/`Bus::fake`, chain/batch assertions · CORE · JOB-23 | `:test`/`:inline` adapters · CORE · JOB-17 | — | Immediate/Dummy backends · CORE · JOB-7/8 |
| In-process parallel execution helper | `Concurrency::run/defer` (child processes) · CORE · JOB-31 | — | GenServer/Task/OTP native · CORE · BEAM-5 | — |
| One-off prod tasks | — (CLI covered elsewhere) | — | Release `eval` / custom commands, minimal-boot pattern · CORE · JOB-6 | — |

**Notable divergences.** This is the section where the four disagree most on *what belongs in a framework at all*: Laravel says everything (31 CORE-heavy rows, from debouncing to circuit-breaker middleware), Rails says the essential 80% with zero extra infrastructure (Solid Queue over the DB you already have), Django says only the *interface* (define/enqueue/result — the worker is explicitly someone else's product), and Phoenix says nothing (the runtime's processes are the primitive; a queue is an ecosystem concern, Oban). Durability defaults split the same way: Rails is durable out of the box, Laravel needs you to pick a driver, Django and Phoenix are not durable at all without third parties. Scheduling repeats the pattern — Laravel a rich code-defined cron DSL with multi-server locks, Rails a declarative YAML scheduler, Django and Phoenix nothing. Workflow orchestration (chains, batches, unique/debounced jobs) is Laravel-only; Rails's unique contribution is instead Continuations (resumable long jobs), which nobody else has.

**Volt notes.**
- Goroutines dissolve the "move work off the request path" motivation (Phoenix proves a concurrent runtime does), but they do not provide the *durability* half: retries across restarts, dead-letter stores, and schedulers still need a queue. Rails's DB-backed Solid Queue (`SKIP LOCKED`) shows a zero-new-infrastructure durable default is possible and popular — a natural fit for Go.
- Django's split is instructive: a small, stable define/enqueue/result API with backend feature flags, letting the ecosystem compete on workers. A Go framework could ship the interface + a DB backend and still swap in River/Asynq-style engines.
- The union feature list is dominated by Laravel one-offs (debounce, encryption, batches-with-callbacks, failover drivers); Rails demonstrates which subset (retry/backoff, priorities, bulk enqueue, concurrency limits, transactional enqueue, recurring tasks, dashboard) is enough for a defaults-first framework.
- Transactional enqueue (enqueue-after-commit) appears in all three frameworks that have jobs — table stakes when the queue and the app DB can diverge, and free when the queue *is* the app DB.
- Phoenix's supervision substrate (BEAM-1/2) has no Go equivalent out of the box: goroutines aren't supervised or isolated, so "just spawn a goroutine" loses crash containment and restart that OTP gives Elixir. A Volt answer needs explicit lifecycle/panic-recovery machinery.

## P10 — Real-time: websockets & server push

- **Laravel**: event broadcasting — server events flow through a driver (first-party Reverb websocket server · OPT, or Pusher/Ably · ECO) to the Echo JS client; SSE is the core-only fallback.
- **Rails**: Action Cable — full-stack in-process WebSockets with Ruby channel classes and cookie-shared auth, defaulting to DB-backed Solid Cable pub/sub (no Redis); Turbo Streams rides on top.
- **Phoenix**: the deepest answer, in three layers on the BEAM: PubSub (zero-broker cluster messaging), Channels (process-per-client topic sockets, documented wire protocol), LiveView (server-rendered reactive UI over the channel transport), plus CRDT Presence.
- **Django**: nothing in core — ASGI + async streaming responses give SSE/long-poll; WebSockets are ecosystem (Channels, outside the corpus).

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| WebSocket server | Reverb standalone (Pusher protocol, multi-app, SSL) · OPT · LIVE-2 | Action Cable mounted at `/cable` or standalone · CORE · LIVE-1/8 | `socket` endpoint declaration · CORE · LIVE-1 | — · ECO · LIVE-4 (ASGI servers speak it; core has no API) |
| Long-poll transport fallback | — | — | Built-in per-socket, `longPollFallbackMs` + clustering guidance · CORE · LIVE-1/47 | — |
| SSE / one-way HTTP streaming | `eventStream` + `useEventStream` · CORE · LIVE-14 | `ActionController::Live` · CORE · LIVE-11 | — (LiveView occupies this niche) | `StreamingHttpResponse` async iterator on ASGI · OPT/CORE · LIVE-1/3 |
| Connection authentication | Auth via channel routes/guards · CORE · LIVE-6 | `identified_by`, shares session cookies · CORE · LIVE-1 | `connect/2..3` + `Phoenix.Token` `auth_token` (tokens over cookies) · CORE · LIVE-2/6 | — |
| Channel/topic abstraction | Public/private/presence channels · CORE · LIVE-1 | Channel classes + `stream_from`/`stream_for` · CORE · LIVE-2/4 | `channel "room:*"` wildcard routing, process per client-topic · CORE · LIVE-2/3 | — |
| Per-channel authorization | Closures/classes in `routes/channels.php`, model binding · CORE · LIVE-6 | Subscription rejection · CORE · LIVE-2 | `join/3` per topic · CORE · LIVE-3 | — |
| Server broadcast API | `ShouldBroadcast` events, `Broadcast::on` anonymous · CORE · LIVE-1/7/8 | `ActionCable.server.broadcast` / `broadcast_to` from anywhere · CORE · LIVE-4/5 | `broadcast!/3` + `Phoenix.PubSub` cluster-wide · CORE · LIVE-4/10 | — |
| Client→server RPC | — (Echo is subscribe/listen) | `perform` → channel actions · CORE · LIVE-3 | `handle_in/3` + sync replies · CORE · LIVE-3/4 | — |
| Client-to-client events (no server hop) | Whisper events · CORE · LIVE-11 | — | — | — |
| Per-recipient broadcast filtering | `toOthers()` exclude-sender, `via()` · CORE · LIVE-8 | — | `intercept` + `handle_out/3` customize per recipient · CORE · LIVE-5 | — |
| Pub/sub backplane | Redis pub/sub for Reverb horizontal scale · OPT · LIVE-3 | Solid Cable (DB) default; Redis, PG NOTIFY, async/test adapters · CORE · LIVE-6 | PG2 native distribution default; Redis adapter; `local_broadcast` · CORE · LIVE-10 | — |
| Zero-extra-infra default | — (needs Reverb process or hosted service) | Solid Cable over existing DB · CORE · LIVE-6 | BEAM clustering, no broker at all · CORE · LIVE-10, BEAM-3/4 | — |
| Presence ("who's online") | Presence channels, here/joining/leaving · CORE · LIVE-9 | — | `Phoenix.Presence` CRDT-replicated, self-healing, no external dep; fetch/handle_metas hooks; JS sync class · CORE · LIVE-11/12/13 | — |
| Official JS client | Echo (`laravel-echo`) · CORE · LIVE-4 | `@rails/actioncable` · CORE · LIVE-3 | `phoenix` npm package · CORE · LIVE-8 | — |
| React/Vue integration hooks | `useEcho/useEchoPresence/useEchoModel` · CORE · LIVE-5 | — | — (LiveView is the alternative) | — |
| Documented wire protocol / polyglot clients | Pusher protocol compat (any Pusher client) · OPT · LIVE-2 | — | V2 JSON protocol documented; Swift/Java/Kotlin/C#/Elixir clients · CORE · LIVE-8 | — |
| Model-driven auto-broadcast | `BroadcastsEvents` trait on CRUD · CORE · LIVE-10 | `stream_for @post` / `broadcast_to` model-scoped · CORE · LIVE-4 | — | — |
| HTML-over-the-wire DOM updates | — (Livewire is ecosystem, outside corpus) | Turbo Streams over Action Cable · CORE · LIVE-9 | LiveView minimal template diffs · CORE · LIVE-14/17 | — |
| Server-rendered reactive UI framework | — | — | LiveView: process-per-view, HTTP first paint, `mount`/`handle_event`/`handle_info`, change-tracked assigns · CORE · LIVE-14–18 | — |
| Stateful server components / nesting | — | — | LiveComponent (in-process) + nested LiveViews (isolated process) · CORE · LIVE-21/22 | — |
| Declarative client bindings & rate limiting | — | — | `phx-click/keydown/…` + `phx-debounce`/`phx-throttle` · CORE · LIVE-23/24 | — |
| Server↔client JS command channel | — | — | `Phoenix.LiveView.JS` commands, `push_event`, hooks lifecycle, colocated JS/hooks · CORE · LIVE-25/28/29/30 | — |
| Forms over socket + crash recovery | — | — | `<.form>` change/submit, in-flight sync guarantees, auto form recovery on reconnect · CORE · LIVE-31–34 | — |
| Large-collection streaming / infinite scroll | — | — | `stream/3` + `phx-update="stream"` + viewport bindings · CORE · LIVE-19/20 | — |
| File uploads over the socket | — | — | `allow_upload`, progress, drag-drop, direct-to-cloud presign · CORE · LIVE-36/37 | — |
| Live navigation (patch/navigate) | — | — | `push_patch`/`push_navigate`, loading events, `@page_title` · CORE · LIVE-38–40 | — |
| Reliability semantics documented | — | — | At-most-once delivery stated; client backoff/rejoin, PushBuffer outbox; catch-up is app code · CORE · LIVE-7 | Disconnect via `asyncio.CancelledError` · OPT · LIVE-2 |
| Crash/deploy recovery posture | — | — | Post-mount crash → process restart + live remount; state-in-URL/DB guidance; `static_changed?` · CORE · LIVE-41–43 | — |
| Origin/security controls | Reverb allowed origins · OPT · LIVE-2 | Allowed request origins allowlist (regex) · CORE · LIVE-7 | — (not in corpus) | — |
| Hosted-service alternative | Pusher Channels / Ably recipes · ECO · LIVE-13 | — | — | — |
| Horizontal scaling story | Reverb + Redis pub/sub, ext-uv tuning, nginx recipe · OPT · LIVE-3 | — (adapter-dependent) | Add nodes; native distribution, "2M websocket connections" · CORE · BEAM-3 | — |
| Notifications over websocket | Broadcast notification channel + `Echo.notification()` · CORE · LIVE-12 | — | — | — |
| Testing support | — (not in corpus for broadcasting) | Connection/channel test cases, `assert_broadcast_on` · CORE · LIVE-10 | Generators emit channel tests · CORE · LIVE-9 | — |
| Dev/debug tooling | Reverb debug/restart commands · OPT · LIVE-2 | — | `enableDebug()`, built-in latency simulator; LiveDebugger · CORE/ECO · LIVE-44/46 | — |
| Scaffolding/generators | `install:broadcasting` · CORE · LIVE-1 | — | `mix phx.gen.socket/channel/presence` · CORE · LIVE-9/12 | — |

**Notable divergences.** Four entirely different architectures: Rails runs the socket *inside* the app process with channels as Ruby classes; Laravel externalizes it — the framework only *broadcasts events at* a separate driver process (Reverb) or a paid service, and the deep logic lives in the JS client; Phoenix makes the socket the center of the programming model (a process per client, and LiveView turning real-time into the default UI paradigm — 47 rows, more than the other three combined); Django simply refuses, offering ASGI/SSE plumbing and delegating WebSockets wholesale to the ecosystem. The backplane question splits identically to P9: Phoenix needs nothing (native clustering + CRDT presence), Rails defaults to the database (Solid Cable), Laravel reaches for Redis, Django abstains. Presence exists only in Laravel (server-assisted channel type) and Phoenix (masterless CRDT); Rails and Django have no answer. Phoenix is also alone in documenting delivery semantics (at-most-once, catch-up is your code) — the others don't state guarantees at all.

**Volt notes.**
- Go is unusually strong here: goroutine-per-connection is the native idiom (gorilla/nhooyr websockets scale like BEAM processes for this purpose), so a CORE in-process websocket layer à la Action Cable/Channels is cheap — the hard part Go lacks is Phoenix's *cluster* story (PG2, CRDT presence); any multi-node Volt needs an explicit backplane (Redis/NATS/Postgres) like Rails/Laravel.
- Channels-style topic routing + per-topic auth (join callback) is the convergent abstraction across all three implementers — a stable design target independent of transport.
- LiveView is the outlier bet: 30+ of Phoenix's rows only exist because server-rendered reactive UI is in scope. That's a product decision, not a websocket-layer decision — templ/datastar-style HTML-over-the-wire could be layered later if the pub/sub + socket substrate exposes the right hooks (cf. Turbo Streams riding on Action Cable).
- Explicitly documenting delivery guarantees (Phoenix's at-most-once + client outbox + rejoin) is rare and valuable; silence elsewhere pushes users to discover semantics in production.

## P11 — Mail & notifications

- **Laravel**: Mailable classes (envelope/content/attachments) over Symfony Mailer transports, plus a full parallel Notification abstraction — one class fanned out to mail/database/broadcast/SMS/Slack channels.
- **Rails**: Action Mailer mirrors controllers (classes, views, layouts, previews) with Active Job delivery; Action Mailbox handles *inbound* email; non-email channels are an acknowledged gap (DIY).
- **Phoenix**: Swoosh wired into every new app with a dev mailbox; production adapters and everything beyond email are explicitly your integration work.
- **Django**: `django.core.mail` — `EmailMessage` over pluggable backends, 6.1's named `MAILERS` config; console/file/locmem backends for dev/test; no notification framework (flash messages only).

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Transport/backend abstraction | Symfony transports: SMTP/SES/Postmark/Resend/Mailgun/sendmail/log/array · CORE · MAIL-1 | SMTP/sendmail/file/test delivery methods · CORE · MAIL-9 | Swoosh adapters (prod adapter wiring is your work) · CORE · MAIL-1/3 | SMTP/console/file/locmem/dummy backends · CORE · MAIL-7–11; ESP APIs · ECO · MAIL-12 |
| Named multi-mailer config | Per-mailer config, per-mailer send · CORE · MAIL-1/6 | Per-delivery `delivery_method_options` · CORE · MAIL-9 | — | `MAILERS` setting + `using=` (new 6.1) · CORE · MAIL-14/17 |
| Transport resilience | `failover` + `roundrobin` mailers · CORE · MAIL-2 | — | — | — |
| Message composition class | Mailables: `envelope()`/`content()` fluent API · CORE · MAIL-3 | Mailer classes mirroring controllers · CORE · MAIL-1 | Swoosh compose/deliver interface · CORE · MAIL-1 | `EmailMessage` (cc/bcc/reply-to/headers) · CORE · MAIL-4 |
| Template-rendered bodies | Blade views + view data · CORE · MAIL-3 | Per-action ERB views, layouts, fragment caching · CORE · MAIL-1/7 | — (notifier modules compose in code) · MAIL-3 | — (string bodies; template rendering is your call) |
| HTML+text multipart | View/text/`htmlString` in `content()` · CORE · MAIL-3 | Implicit multipart from sibling `.html/.text` templates · CORE · MAIL-2 | — | `EmailMultiAlternatives`/`content_subtype` · CORE · MAIL-5/6 |
| Pre-styled component templates | Markdown mailables (`<x-mail::button>` etc.), themeable · CORE · MAIL-5 | — | — | — |
| Attachments & inline embeds | Path/disk/raw + inline embeds + `Attachable` models · CORE · MAIL-4 | Attachments incl. inline cid URLs · CORE · MAIL-5 | — | `attach()`/`attach_file()`, MIMEPart · CORE · MAIL-4 |
| Queued/async delivery | `queue()`/`later()`, `ShouldQueue`, afterCommit · CORE · MAIL-6 | `deliver_later` via Active Job · CORE · MAIL-3 | — | — (via task-queue backends · ECO · MAIL-12) |
| Bulk sending / connection reuse | Collection recipients · CORE · MAIL-6 | — | — | `send_mass_mail`, `open()`/`close()` reuse · CORE · MAIL-2/15 |
| Dev preview / capture | Route-return browser preview, log mailer, Mailpit, universal `to` · CORE · MAIL-7/10 | `/rails/mailers` preview UI · CORE · MAIL-11 | `/dev/mailbox` viewer · CORE · MAIL-2 | Console/file backends · CORE · MAIL-8/9 |
| Test doubles & assertions | `Mail::fake`, content/attachment assertions · CORE · MAIL-9 | `test` delivery method · CORE · MAIL-9 | — (Swoosh "testing" per glossary · MAIL-1) | locmem → `mail.outbox` · CORE · MAIL-10 |
| Localized sending | `locale()`, `HasLocalePreference` · CORE · MAIL-8 | i18n subject lookup · CORE · MAIL-10 | — | — |
| Hooks: intercept/observe/events | `MessageSending/Sent` events · CORE · MAIL-11 | Interceptors + observers; `before/after_action`, `after_deliver`, `rescue_from` · CORE · MAIL-8/12 | — | — |
| Custom transport extension | `Mail::extend`, extra Symfony transports · CORE · MAIL-11 | — | Swoosh adapter integration · CORE · MAIL-3 | Subclass `BaseEmailBackend` · DIY · MAIL-13 |
| Header-injection protection | — (not in corpus) | — (not in corpus) | — | CRLF `ValueError`, safe `Address` building · CORE · MAIL-16 |
| URL generation in mail | — | `default_url_options[:host]`, asset host · CORE · MAIL-6 | — | — |
| Admin/ops mail shortcuts | — | — | — | `mail_admins`/`mail_managers` · CORE · MAIL-3 |
| Auth-flow email scaffolding | — (covered by starter kits, P7) | — | `phx.gen.auth` notifier modules (confirm/magic-link/reset) · CORE · MAIL-3 | — |
| **Inbound** email processing | — | Action Mailbox: routing DSL, Mailgun/Postmark/SendGrid/relay ingresses, lifecycle state machine, bounce replies, incineration, conductor test UI · CORE · MAIL-13–16 | — | — |
| Multi-channel notification abstraction | Notifications: one class, `via()` → mail/database/broadcast/SMS/Slack · CORE · MAIL-12 | — · DIY · MAIL-17 (named gap) | — · MAIL-5 (named gap: "integrate it yourself") | — (contrib.messages flash only) |
| In-app/database notifications | Database channel, unread tracking, mark-as-read · CORE · MAIL-16 | — · DIY · MAIL-17 | — | — |
| WebSocket notification delivery | Broadcast channel + `Echo.notification()` · CORE · MAIL-12 (LIVE-12) | — | — | — |
| SMS channel | Vonage channel pkg · OPT · MAIL-17 | — · DIY · MAIL-17 | — · MAIL-5 | — |
| Chat (Slack) channel | Block Kit builder, interactivity · OPT · MAIL-18 | — | — | — |
| On-demand notify (non-users) | `Notification::route(...)->notify()` · CORE · MAIL-13 | — | — | — |
| Queued notifications, per-channel tuning | Per-channel queue/delay maps, `shouldSend` cancel · CORE · MAIL-14 | — | — | — |
| Rich per-channel message builders | `MailMessage` fluent lines/actions, full Mailable substitution · CORE · MAIL-15 | — | — | — |
| Notification testing/localization/custom channels | `Notification::fake`, locale, custom channel classes; community channel matrix · CORE/ECO · MAIL-19 | — | — | — |

**Notable divergences.** Outbound email is the one place all four agree on shape (message class over pluggable transports with dev/test backends) and differ only in depth — Laravel adds transport failover and styled markdown components, Django adds multi-mailer config and header-injection hardening, Phoenix ships only the thinnest wiring. The real fault line is everything *around* email: Laravel is alone with a first-class multi-channel notification framework (database, websocket, SMS, Slack, on-demand routing) — Rails and Phoenix both explicitly name this a gap, and Django doesn't even frame the problem. Rails is alone in the opposite direction: Action Mailbox makes *inbound* email a framework concern (ingress webhooks, lifecycle state machine, test conductor), which no other corpus touches. Template philosophy also splits: Rails/Laravel render mail through the same view layer as pages; Django and Phoenix treat bodies as strings you assemble.

**Volt notes.**
- The convergent core is small and clear: message struct + pluggable transports + console/memory dev-test backends + a browser preview (all four have a preview/capture story — the single most universal DX feature in this section).
- Laravel's notification layer is really a *routing* abstraction (notifiable → channel → per-channel renderer) sitting on top of mail/broadcast/queue subsystems; it's only possible because those substrates exist first — sequencing matters for Volt.
- Queued delivery is the standard bridge between P11 and P9 (Laravel `ShouldQueue`, Rails `deliver_later`); if Volt has any job story, mail should ride it rather than growing its own async path. In Go the tempting shortcut — fire a goroutine — loses retries and observability, the exact things the queue bridge buys.
- Inbound email (Action Mailbox) is a genuine differentiator nobody copied; its ingress-webhook + normalize + route design is transport-agnostic and would port cleanly to Go, but it drags in storage (raw eml) and lifecycle-state dependencies.

## P12 — Caching & performance

- **Laravel**: unified multi-store cache API (redis/memcached/db/file/dynamodb/…) with stampede protection, atomic locks and a general rate limiter built on it, plus Octane resident-process serving as the throughput answer.
- **Rails**: layered *view* caching (fragment/russian-doll/collection) over a pluggable `Rails.cache`, with database-backed Solid Cache as the default store, HTTP conditional GET as core, and a documented Puma/GVL tuning playbook.
- **Phoenix**: deliberately no cache framework — performance is structural (precompiled templates/routes, LiveView diffs, BEAM concurrency); ETS is the raw in-memory store when needed.
- **Django**: one multi-backend cache API used at four granularities (per-site, per-view, template fragment, low-level) plus the richest HTTP cache-header/middleware toolkit (Vary, Cache-Control, conditional GET, gzip).

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Cache backends behind one API | redis/memcached/db/file/dynamodb/mongodb/array/null · CORE · CACHE-1 | Solid Cache (DB, default) + Memory/File/MemCache/Redis/Null stores · CORE/ECO · CACHE-7/8 | — (no cache framework; explicit gap) · CACHE-1 | locmem (default)/db/file/redis/memcached/dummy · CORE/OPT · CACHE-1–6 |
| Default-store philosophy | config-selected, `Cache::store()` per call · CORE · CACHE-1 | DB-backed Solid Cache: disk-size economics over RAM · CORE · CACHE-7 | ETS in-memory terms as BEAM-native answer · CORE · CACHE-6 | per-process LRU LocMemCache · CORE · CACHE-5 |
| Failover / fallback store | ordered store list, auto fallback (new in 13) · CORE · CACHE-2 | Redis store failure handling only · CORE · CACHE-8 | — | — |
| Low-level read/write API | get/put/pull/many/incr/forget/flush, TTL types · CORE · CACHE-3 | fetch/read/write/delete/exist? + expires_in · CORE · CACHE-5 | — (raw ETS ops) · CACHE-6 | set/get/add/get_or_set/*_many/touch/incr/clear · CORE · CACHE-12 |
| Compute-through + stampede protection | remember/rememberForever, `flexible` stale-while-revalidate w/ bg refresh · CORE · CACHE-4 | `fetch` with `race_condition_ttl` · CORE · CACHE-5 | — | `get_or_set` (no SWR/stampede story) · CORE · CACHE-12 |
| TTL extension without re-write | `Cache::touch` (new in 13) · CORE · CACHE-5 | — | — | `touch` · CORE · CACHE-12 |
| In-request memoization | `Cache::memo()` memory layer over any store · CORE · CACHE-6 | per-request SQL query cache · CORE · CACHE-6 | — (assigns/process state idiomatic) | `cached_property` · CORE · CACHE-19 |
| Grouped invalidation: tags | tag-scoped write/flush (redis/memcached/dynamodb) · CORE · CACHE-7 | — (key versioning instead) | — | — |
| Key versioning / prefixing | — (tags instead) | versioned keys, model `cache_key_with_version` · CORE · CACHE-1/5 | — | `version` arg + incr/decr_version, KEY_PREFIX/KEY_FUNCTION · CORE · CACHE-13 |
| Atomic/distributed locks | `Cache::lock` w/ owner tokens, block, forceRelease · CORE · CACHE-8 | — | — (BEAM processes serialize state instead) | — |
| Concurrency limiting / funnels | `withoutOverlapping` + funnel slots on locks · CORE · CACHE-9 | — | — | — |
| Rate limiting (server-side) | general `RateLimiter` facade on cache · CORE · CACHE-11 | controller `rate_limit` backed by cache store · CORE · (P2) CTRL-15 | — (explicit gap; client-side debounce only) · API-13, LIVE-24 | — (explicitly not provided) · (P8) SEC-16 |
| Fragment caching in views | — | fragment + russian-doll + collection multi-fetch, template-digest busting · CORE · CACHE-1–4 | — (LiveView diff engine replaces it) · CACHE-3 | `{% cache %}` tag + fragment key helper · CORE · CACHE-11 |
| Per-view / per-page caching | — | — (page/action caching removed from core; non-goal) | — | `cache_page` decorator + per-site middleware pair · CORE · CACHE-9/10 |
| HTTP conditional GET (ETag/Last-Modified) | — | `stale?`/`fresh_when`, strong/weak ETags, Rack::ETag · CORE · CACHE-9 | — (only default `max-age=0, private` header) · CACHE-4 | ConditionalGetMiddleware + per-view `condition` · OPT · CACHE-20 |
| Cache-Control / Vary header helpers | — | `http_cache_forever`, ConditionalGet middleware · CORE · CACHE-9 | — | `cache_control`/`never_cache`/`vary_on_*` decorators + patch helpers · CORE · CACHE-15–17 |
| Shared/proxy HTTP caching | — | Rack::Cache backed by Rails cache · OPT · CACHE-10 | — | downstream Expires/Cache-Control auto-set · CORE · CACHE-16 |
| Response compression | — | Thruster front proxy (compression + X-Sendfile) · CORE · CACHE-13 | — | GZipMiddleware w/ BREACH mitigation · OPT · CACHE-21 |
| Static-asset cache headers / fingerprinting | — (Vite; outside P12) | Propshaft far-future fingerprints · CORE · CACHE-14 | `mix phx.digest` manifest + digests · CORE · CACHE-4 | ManifestStaticFilesStorage content hashes · CORE · CACHE-22 |
| Compile/deploy-time caches | config/route/view/event caching via `optimize` · CORE · CACHE-15 | — | templates & routes precompiled by design · CORE · CACHE-2/5 | cached template loader (auto) · CORE · CACHE-23 |
| Runtime/app-server acceleration | Octane resident workers (FrankenPHP/Swoole/RoadRunner) + state pitfalls + Swoole extras · OPT · CACHE-12–14 | Puma workers-vs-threads/GVL tuning playbook · CORE · CACHE-12 | — (BEAM is natively resident/concurrent) · CACHE-3 | — (WSGI/ASGI left to deployment) |
| Async cache API | — | — | — (everything is async on BEAM) | a-prefixed variants (aget/aset/…) · CORE · CACHE-14 |
| Cached sessions backend | — (not in P12) | — (not in P12) | — | cached-sessions backend pointer · CORE · CACHE-18 |
| Retention/culling/encryption of store | — | Solid Cache max_age/max_size, optional encryption, sharding · CORE · CACHE-7 | — | MAX_ENTRIES/CULL_FREQUENCY tuning · CORE · CACHE-8 |
| Custom store + cache events | `Cache::extend`, hit/miss/write events · CORE · CACHE-10 | custom store API · CORE · CACHE-8 | — | dotted-path custom backend · DIY · CACHE-7 |
| Dev-mode cache toggle | array/null stores · CORE · CACHE-1 | `bin/rails dev:cache` · CORE · CACHE-11 | — | DummyCache · CORE · CACHE-6 |
| **Full-text search (cluster; Laravel P22 / Django P23)** | | | | |
| Database full-text search | `whereFullText` → MATCH/tsvector, fullText indexes · CORE · SRCH-1 | PG full-text query *patterns* only (non-goal beyond) · CORE · ORM-49 | — (no search rows in corpus) | SearchVector/Query/Rank/Headline, Lexeme; PG-only · OPT · PG-4 |
| Search-engine index sync framework | Scout: Searchable trait, engines (db/collection/Algolia/Meilisearch/Typesense), ops + querying · OPT/ECO · SRCH-5–8 | — (explicit non-goal) | — | — (ecosystem; absent from corpus) |
| Vector / semantic search | `vector()` cols + HNSW, `whereVectorSimilarTo`, embeddings + AI rerank · CORE/OPT · SRCH-2–4, SRCH-9 | — | — | — |
| Fuzzy match: trigram / unaccent | — | — | — | Trigram similarity lookups + `unaccent` · OPT · PG-5/6 |

**Notable divergences.** Laravel and Django converge on "one KV cache API over many backends," but disagree on invalidation (Laravel tags, Django versioned/prefixed keys) and on what to build atop it (Laravel: locks, funnels, rate limiting; Django: HTTP middleware layers). Rails' center of gravity is entirely different — the view layer (russian-doll fragment caching with automatic digest busting) plus a contrarian default store (Solid Cache betting on cheap disk over RAM). Phoenix flatly refuses the whole category: no cache store, no fragment cache, no HTTP caching — its inventory records this as deliberate, arguing precompiled templates, LiveView diffs, and BEAM concurrency remove the need. HTTP conditional GET is core in Rails, opt-in middleware in Django, and absent in Laravel and Phoenix. Runtime acceleration is a tier tell: Laravel must sell Octane (OPT) to escape per-request PHP, Rails documents tuning around the GVL, Phoenix gets it free, Django doesn't engage. On search, Laravel is far ahead (DB FTS + pgvector + Scout as a layered story); Django offers PG-native FTS/trigram in contrib; Rails names FTS a non-goal; Phoenix has nothing.

**Volt notes.**
- Go is already the "Octane runtime" — resident process, real concurrency — so the entire acceleration tier (Octane, Puma tuning) is moot; what remains contested and relevant is the store API + stampede protection (Laravel `flexible` SWR, Rails `race_condition_ttl`), which maps naturally onto `singleflight`.
- Tags vs versioned-keys is a genuine fork with backend implications: tags need backend support (Laravel limits them to 3 stores), versioning works on any KV store.
- HTTP-layer caching (ETag/conditional GET, Cache-Control helpers) is cheap, backend-free, and shipped by the two batteries-included frameworks — natural middleware territory in Go.
- Cache-backed locks/rate-limiting (Laravel-only combo) matters more for Go's multi-instance deployments than it did for PHP; Phoenix's "the runtime is the cache" stance is the counter-argument that a framework store can be skipped entirely.

## P13 — Files & storage

- **Laravel**: `Storage` facade over Flysystem — named disks (local/public/S3/FTP/SFTP), one read/write/URL/metadata API, temporary URLs, fakeable tests; a filesystem abstraction, not an attachment framework.
- **Rails**: Active Storage — attachment macros on models, three-table blob design, pluggable cloud services, on-the-fly image variants and non-image previews, direct-from-browser uploads.
- **Phoenix**: `Plug.Upload` multipart plumbing into temp files + LiveView reactive/direct-to-cloud uploads; durable storage abstraction explicitly left to libraries (recorded gap).
- **Django**: `File` wrapper hierarchy + chunked streaming upload handlers (2.5 MB memory→tempfile crossover) + pluggable Storage API via `STORAGES`; cloud backends are ecosystem (django-storages).

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Storage abstraction layer | named disks over Flysystem · CORE · FILE-1 | Active Storage services API · CORE · FILE-1/3 | — (explicit gap: "use a library") · FILE-7 | Storage API + `STORAGES` aliases · CORE · FILE-8/11 |
| Local-disk backend | local + symlinked public disk · CORE · FILE-1 | Disk service (dev/test) · CORE · FILE-3 | Plug.Static extra mounts (serving, not storage) · CORE · FILE-4 | FileSystemStorage · CORE · FILE-9 |
| Cloud object storage | S3 + any S3-compatible (MinIO/R2/Spaces) · CORE (adapter pkg) · FILE-1 | S3 + S3-compatible, GCS (Azure removed 8.1) · CORE · FILE-3 | — (ExAws/S3 referenced for presigning only) · FILE-7 | django-storages et al. · ECO · FILE-13 |
| FTP / SFTP | drivers incl. key auth · CORE · FILE-2 | — | — | — |
| Scoped / read-only / on-demand disks | path-prefixed + read-only disks, `Storage::build` inline config · CORE · FILE-3/4 | per-attachment `service:` selection · CORE · FILE-3 | — | per-alias storage instances, callable/lazy · CORE · FILE-11 |
| Mirror / multi-service replication | — | mirror service for zero-downtime provider migration · CORE · FILE-4 | — | — |
| ORM attachment integration | — (store paths yourself; upload helpers only) | `has_one/has_many_attached`, 3-table blob/attachment/variant design · CORE · FILE-1/2 | — | `FileField`/`ImageField` with `.url/.path`, save/delete · CORE · FILE-4 |
| Multipart upload handling | `$request->file()`, validity/mime helpers · CORE · FILE-8, (P2) CTRL-17 | form `file_field` → attach · CORE · FILE-1 | `Plug.Upload` temp-file structs auto in params, `give_away/3` · CORE · FILE-1/2 | `request.FILES` UploadedFile w/ `.chunks()` · CORE · FILE-5 |
| Streaming / memory-safety discipline | automatic stream detection on writes · CORE · FILE-7 | — (direct upload sidesteps the app) | `Plug.Parsers` length/read_length/read_timeout limits · CORE · FILE-3 | pluggable upload handlers, 2.5 MB crossover, per-request override · CORE/DIY · FILE-6/7 |
| Direct-to-cloud browser uploads | S3 `temporaryUploadUrl` presigning · CORE · FILE-5 | `direct_upload: true` w/ JS lifecycle, CORS guidance · CORE · FILE-11 | LiveView presigned external uploads · CORE · FILE-5 (LIVE-37) | — |
| Reactive upload UX (progress/preview/cancel) | — (Livewire territory, not in P13) | JS events + progress tracking · CORE · FILE-11 | LiveView uploads: progress, preview, cancel · CORE · FILE-5 (LIVE-36) | — |
| URL generation: public vs signed temporary | `url()` + `temporaryUrl` (S3/local) · CORE · FILE-5 | public permanent vs signed short-lived per service · CORE · FILE-5 | — | `.url` via storage; no signed-URL story in core · CORE · FILE-8 |
| Serving strategies | streamed `download` responses · CORE · FILE-5 | redirect vs proxy modes + auth controllers; Thruster X-Sendfile · CORE · FILE-6, (P12) CACHE-13 | `send_file/5` or Plug.Static mounts · CORE · FILE-4 | — (serving left to web server) |
| Image variants / transformation | — | lazy `variant()` via libvips/ImageMagick, named + preprocessed variants · CORE · FILE-7 | — | — (ImageField reads dimensions only · FILE-3) |
| Previews for non-images (video/PDF) | — | ffmpeg/poppler previewers, custom API · CORE · FILE-8 | — | — |
| Metadata & checksums | size/lastModified/mime/checksum · CORE · FILE-6 | blob checksums + background analyzers (dimensions, duration) · CORE · FILE-2/9 | content_type/filename only on struct · CORE · FILE-1 | size, image width/height · CORE · FILE-1/3 |
| Visibility / permissions | public/private at write, per-driver maps · CORE · FILE-9 | public vs private mode per service · CORE · FILE-5 | — | `permissions` on FileSystemStorage · CORE · FILE-9 |
| Directory operations / listing | files/directories listing, make/deleteDirectory · CORE · FILE-10 | — (flat blob keys) | — | `listdir` on Storage · CORE · FILE-8 |
| Write ops: copy/move/append | put/prepend/append/copy/move, throw-or-bool · CORE · FILE-7 | purge/purge_later, replace-vs-add semantics · CORE · FILE-10 | — (stdlib `File`) | save/open/delete, overwrite control · CORE · FILE-8/9 |
| Unique/sanitized filenames | hashName, `putFile` unique hashing · CORE · FILE-7/8 | auto-generated blob keys · CORE · FILE-2 | — ("use a library" advice) · FILE-7 | get_valid_name/get_available_name/generate_filename · CORE · FILE-8 |
| Upload validation (type/size/dimensions) | fluent `File::types()->min()->max()`, image/dimensions · CORE · (P6) VAL-13 | — no built-in validators; form patterns only · DIY · FILE-13 | parser-level size limits only · CORE · FILE-3 | form FileField/ImageField (Pillow) · CORE · FILE-5 |
| Temp-file lifecycle | — (implicit) | — | auto-delete at request end, ownership transfer · CORE · FILE-1 | TemporaryUploadedFile w/ configurable dir · CORE · FILE-5/6 |
| In-memory / fake storage for tests | `Storage::fake` + `UploadedFile::fake()` assertions · CORE · FILE-11 | fixture attachments, cleanup guidance · CORE · FILE-12 | — | InMemoryStorage + ContentFile · CORE · FILE-10/2 |
| Custom storage backend | `Storage::extend` any Flysystem adapter · CORE · FILE-12 | — (service list is what's documented) | — | subclass `Storage` (`_open`/`_save`) · DIY · FILE-12 |
| Multi-instance local-disk caveats | — | — (cloud assumed) | documented: local disk breaks multi-node; use DB/object store · FILE-6 | — |

**Notable divergences.** The four pick different layers to own. Rails owns the most: attachments are an ORM concept with a schema, transformation pipeline (variants, previews), and provider-migration story (mirror) — yet ships zero upload validators (FILE-13, DIY), which Laravel covers in its validator instead. Laravel owns the filesystem: richest path/disk API (FTP/SFTP, scoped disks, directory ops) but no attachment model and no image processing. Django owns the ingress: it is the only one that treats memory-safe chunked upload handling as a first-class, pluggable pipeline, while pushing all cloud storage to the ecosystem. Phoenix owns almost nothing durable on purpose — its recorded gap says unique naming and S3 are "use a library" — yet its LiveView upload UX (progress/preview/cancel + presigned direct upload) is the most interactive of the four. Direct-to-cloud presigned upload appears in three of four (all but Django).

**Volt notes.**
- Three distinct scopes are on the table: filesystem-API (Laravel), attachment-lifecycle-on-the-ORM (Rails), upload-streaming-discipline (Django). They are separable; Rails is the only one that stacks all three.
- Django's chunked-handler design maps most naturally to Go (`io.Reader`/`multipart.Reader` streaming is idiomatic); Laravel/Rails-style whole-file convenience APIs would sit above that.
- S3-compatible object storage is the de facto floor (3 of 4 treat local disk as dev-only or caveated); FTP/SFTP is Laravel-only legacy surface.
- Image variants are Rails-only and imply a native-dependency choice (libvips/ImageMagick) — in Go that means cgo bindings or shelling out, a real cost worth flagging before copying the feature.

## P14 — Building APIs: serialization & content negotiation

- **Laravel**: a real serializer framework — JsonResource/ResourceCollection with envelope + conditional-field control, plus spec-compliant JSON:API resources (new in 13), automatic 422 JSON errors, Sanctum/Passport for tokens.
- **Rails**: an `--api` subtractive mode with documented à-la-carte middleware; serialization is `render json:` + `as_json` + Jbuilder templates — a dedicated serializer framework is an explicit gap.
- **Phoenix**: no serializer DSL on principle — JSON views are plain functions returning maps; `:accepts` pipelines negotiate formats; `action_fallback` + `ChangesetJSON` standardize errors.
- **Django**: core stops short of a REST framework — its serializers target fixtures/interchange (JSON/JSONL/XML/YAML, natural keys); REST/OpenAPI/GraphQL are explicitly ecosystem (DRF, django-ninja); contrib covers sitemaps and feeds.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| API app mode / scaffold | Additive `install:api` (routes, throttle, Sanctum) · CORE · API-1 | Subtractive `rails new --api` (trimmed middleware, no views) · OPT · API-1 | `phx.new --no-html --no-assets ...` · CORE · API-8; `phx.gen.json` full CRUD scaffold · CORE · API-1 | — (no API mode) |
| Serializer/transformer framework | JsonResource + ResourceCollection (`$collects`, meta) · CORE · API-2, API-3 | None built in — explicit gap · DIY · API-12 | None by design — functions returning maps · CORE · API-2 | `serializers.serialize()` — fixture/interchange-oriented, not API payloads · CORE · API-1 |
| Direct model → JSON fallback | Return models/collections, `toJson`, hidden/appends · CORE · API-9 | `as_json`/`serializable_hash`/`to_json` on all models · CORE · API-3 | — (maps are hand-built; changeset errors self-encode · API-11) | JSON serializer + `DjangoJSONEncoder` · CORE · API-2 |
| Templated JSON views | — (resources instead) | Jbuilder DSL templates · CORE · API-5 | JSON view modules (`UrlJSON.index/show`) · CORE · API-2 | — |
| Envelope/wrapping control | `data` wrap, `withoutWrapping`, nested rules · CORE · API-4 | `wrap_parameters` (request side only) · CORE · API-7 | — (you build the map) | — |
| Conditional/sparse field shaping | `when*`/`mergeWhen`/`whenLoaded/whenCounted` · CORE · API-5 | `as_json(only:/except:)` options · CORE · API-3 | — (hand-written maps) | `fields=[...]` subset (pk always emitted) · CORE · API-6 |
| Spec-compliant JSON:API | `JsonApiResource`: `?include=`, sparse fieldsets, links/meta · CORE · API-7 (new in 13) | — | — | — |
| Content negotiation | `expectsJson`-aware rendering · CORE · API-8 | `respond_to`/format blocks, `Mime::Type.register` · CORE · API-4 | `:accepts` plug per pipeline, `_format` param, per-format views · CORE · API-3, API-4 | Request-object negotiation (cross-ref CTRL-18/19/37) · CORE |
| Multiple wire formats | JSON (+ JSON:API) · API-2, API-7 | `render json:/xml:`, markdown format (8.1) · CORE · API-3, API-6 | JSON; XML via EEx templates (P3 VIEW-21) · CORE · API-2 | JSON/JSONL/XML/YAML serializers · CORE/OPT · API-2–5 |
| API error shape conventions | 422 `{message, errors}`, JSON-aware exceptions, `abort()` · CORE · API-8 | `debug_exception_response_format :api`, rescue_responses status mapping · CORE · API-8 | `action_fallback` + FallbackController → 422 `ChangesetJSON` / 404; `ErrorJSON` · CORE · API-5, API-7, API-11 | — (manual `JsonResponse`) |
| JSON request-body parsing | — here (P2) | Content-Type-driven params + `wrap_parameters` · CORE · API-7 | — here (Plug.Parsers, P2) | — |
| Token auth for APIs | Sanctum + Passport OAuth2 · OPT · API-10 | `HttpAuthentication::Token` · CORE · API-10 | Bearer-token recipe atop gen.auth · DIY · API-9 | — (DRF territory) |
| Middleware slimming / à-la-carte re-add | Stateless `routes/api.php` group · CORE · API-1 | ~19 middleware + ~13 controller modules individually add/removable; sessions/cookies re-addable · CORE/OPT · API-2, API-11 | Pipelines per scope · CORE · API-10 | — |
| HTML + API coexistence in one app | web + api route files · CORE · API-1 | Default app has both; `--api` drops HTML · API-1 | Separate pipelines/scopes · CORE · API-10 | Implicit (any view can return JsonResponse) |
| REST conventions (201/location, 204, except new/edit) | `apiResource` routing (P1 cross-ref) · API-1 | — (resources routing in P1) | 201 + `location` on create, 204 on delete, `resources except:` · CORE · API-6 | — |
| API pagination | Pagination-aware wrapping with `links`/`meta`, customizable · CORE · API-4 | — | — · gap · API-13 | — (Paginator exists but not API-wired) |
| API versioning | — | None built in · DIY · API-12 | Nested router scopes only · CORE · API-10 | — |
| OpenAPI / schema generation | — | None built in · DIY · API-12 | — · gap · API-13 | — (ecosystem: DRF/ninja, per non-goals) |
| Rate limiting for APIs | Throttle group wired by `install:api` · CORE · API-1 | — here | — · gap · API-13 | — |
| HTTP caching for APIs | — here | `stale?` + Rack::Cache, ETag middleware kept in API mode · CORE/OPT · API-9 | — | — (ConditionalGetMiddleware noted re sitemaps · API-10) |
| Custom formats/encoders | — | `Mime::Type.register` · CORE · API-4 | Swappable `Phoenix.json_library()` (Jason) · CORE · API-2 | Custom encoder `cls=`, `SERIALIZATION_MODULES` · DIY · API-2, API-9 |
| Deserialization / load controls | — | `from_json` · CORE · API-3 | — (changesets, P6) | `DeserializedObject.save()`, `ignorenonexistent`, forward refs · CORE · API-7 |
| Natural keys / stable identity | — | — | — | `natural_key()`/`get_by_natural_key()`, dependencies · CORE · API-8 |
| Sitemaps & syndication feeds | — | `atom_feed` helper (P3 VIEW-14 cross-ref) | — | contrib.sitemaps (i18n, index) + contrib.syndication + feedgenerator · OPT/DIY · API-10–14 |
| GraphQL | — | — | Absinthe · ECO · API-12 | — (ecosystem, per non-goals) |

**Notable divergences.** The core disagreement is whether a framework should own the model→payload transformation: Laravel says emphatically yes (a resource layer with envelope, conditional fields, and now full JSON:API compliance), Phoenix says no on principle (plain functions returning maps beat a serializer DSL), Rails says no by omission (documents `as_json`/Jbuilder and labels a serializer framework a gap), and Django's "serializers" solve a different problem entirely (fixtures/interchange with natural keys) while delegating real APIs to DRF. API modes also diverge: Rails and Phoenix are subtractive (strip a full app down), Laravel is additive (`install:api` bolts API wiring onto a web app), Django has no mode at all. Error shaping is standardized by Laravel and Phoenix (automatic 422 validation/changeset JSON), only status-mapped by Rails, and absent in Django. Notably, *none* of the four ship API versioning, OpenAPI generation, or a dedicated serializer with schema in core — the richest problem-space the inventories collectively mark as gap/DIY/ecosystem.

**Volt notes.**
- Phoenix's "no serializer DSL, just functions returning maps" is the position most congruent with Go idiom (structs + `encoding/json` tags), but Laravel's popularity of conditional-field/envelope/pagination-meta tooling shows real demand the bare-struct approach leaves on the table.
- OpenAPI generation is absent from all four cores despite universal demand — a statically-typed Go framework could derive schemas from handler/struct types, a differentiator none of these dynamic-language frameworks could offer cheaply.
- A standardized validation-error JSON shape (Laravel's 422 `{message, errors}`, Phoenix's `ChangesetJSON`) is a proven cross-framework convention; wiring it to the validation layer by default costs little and removes a universal bikeshed.
- Content negotiation is uniformly thin (Accept header + format param + per-format render paths); Rails' à-la-carte middleware documentation and Phoenix's pipelines both suggest API-vs-HTML should be a composition/route-group concern, not a separate framework mode.

## P15 — Internationalization & localization

- **Laravel**: file-based translations (PHP key arrays + JSON source-string maps) via `__()`, locale/fallback config; the inventory itself calls the subsystem comparatively thin.
- **Rails**: the `i18n` gem wired through the whole stack — YAML trees, lazy scoped lookup, CLDR plurals, model/attribute/error translation, pluggable backends.
- **Phoenix**: Gettext ships in every app, but locale *selection* is documented DIY plumbing (plugs, `on_mount` hooks); no negotiation or formatting framework (recorded gap).
- **Django**: GNU gettext end-to-end — extraction/compilation commands, `LocaleMiddleware` negotiation, JS catalogs, locale-aware formatting, and a full timezone framework under the same umbrella.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Message storage format | PHP key arrays + JSON source-string maps · CORE · I18N-1 | YAML/Ruby trees under config/locales · CORE · I18N-3 | GNU gettext PO/MO · CORE · I18N-1 | GNU gettext PO/MO in locale/ dirs · CORE · I18N-20/21/23 |
| Marking/retrieval API | `__()` (dot-key or literal source string) · CORE · I18N-3 | `I18n.t`/`l` + view/controller/mailer helpers · CORE · I18N-1 | gettext macros via generated backend module · CORE · I18N-1 | `gettext`/`gettext_lazy`/`gettext_noop` (`_` alias) · CORE · I18N-1 |
| Extraction/compilation tooling | — (keys authored by hand; `lang:publish` scaffold) · I18N-1 | — (YAML maintained by hand) | PO workflows via gettext tooling · CORE · I18N-1 | `makemessages`/`compilemessages` (+ customization) · CORE/DIY · I18N-20–22 |
| Locale selection / negotiation | config + runtime `App::setLocale` (manual) · CORE · I18N-2 | `around_action` + `I18n.with_locale`; sources documented (params/domain/header/user) · CORE · I18N-2 | locale-from-params plug pattern; whitelist validation · DIY · I18N-2 | `LocaleMiddleware`: URL prefix → cookie → Accept-Language → default · CORE · I18N-19 |
| Locale-prefixed routing / translated URLs | — | `scope ":locale"` + `default_url_options` propagation · CORE · I18N-2 | — (gap) · I18N-5 | `i18n_patterns` + `gettext_lazy` translatable routes · OPT · I18N-16/17 |
| Locale-switch endpoint + cookie persistence | — | — (DIY within I18N-2 patterns) | — | `set_language` view + LANGUAGE_COOKIE_* settings · OPT · I18N-18/26 |
| Fallbacks / missing-key handling | fallback locale config · CORE · I18N-2 | `:default` chains, `raise_on_missing_translations`, custom exception handler · CORE · I18N-3/4/13 | — (gettext default: msgid passthrough; undocumented) | territorial→generic fallback, LOCALE_PATHS precedence · CORE · I18N-23 |
| Interpolation | `:name` w/ case-mirroring, stringable object hooks · CORE · I18N-4 | `%{var}` w/ strict missing/reserved-key errors · CORE · I18N-5 | `%{count}`-style bindings in changeset errors · CORE · I18N-4 | `%(name)s` named (f-strings unsupported by xgettext) · CORE · I18N-5 |
| Pluralization model | pipe alternatives + exact `{1}`/range `[2,*]` rules (non-CLDR) · CORE · I18N-5 | CLDR plural categories, count-keyed subtrees, custom rules · CORE · I18N-6 | — (gettext plural-forms implied, not documented) | `ngettext`/`npgettext`, `{% plural %}` blocks · CORE · I18N-3/8 |
| Context disambiguation (msgctxt) | — | — (scoped keys serve instead) | — | `pgettext`/`pgettext_lazy` · CORE · I18N-2 |
| Scoped / inferred key lookup | dot-notation keys · CORE · I18N-3 | lazy lookup: `t(".title")` resolves per controller/action/view · CORE · I18N-4 | — | — (flat msgid model) |
| Deferred/lazy translation objects | — | — | — | `gettext_lazy` + `format_lazy` for module-load-time strings · CORE · I18N-4 |
| Template integration | Blade `{{ __(...) }}` · CORE · I18N-3 | `translate`/`localize` helpers · CORE · I18N-1 | — (call gettext in HEEx; no dedicated constructs) | `{% translate %}`/`{% blocktranslate %}` + language tags · CORE · I18N-7–9 |
| HTML safety of translations | — (general Blade escaping) | `_html` key suffix auto-escapes interpolations · CORE · I18N-8 | — | noted: `{% translate %}` output NOT auto-escaped · CORE · I18N-7 |
| Per-locale template files | — | `index.es.html.erb` resolution · CORE · I18N-11 | — | — |
| Model/attribute-name translation | Eloquent pluralizer language override only · CORE · I18N-5 | `model_name.human`, `human_attribute_name`, error cascade · CORE · I18N-9 | — | — (lazy `verbose_name` convention via I18N-4) |
| Validation-error translation | — (lang files exist; not inventoried as a row) | error-message lookup cascade model→attribute→default · CORE · I18N-9 | generated `translate_error/1` through Gettext · CORE · I18N-4 | — in P15 (form errors translated via framework catalogs) |
| Framework strings pre-translated | — (scaffold ships English) | helper outputs read locale data (time distances, numbers) · CORE · I18N-10 | — | ships `django/conf/locale` catalogs · CORE · I18N-23 |
| Date/time/number locale formatting | `Number::useLocale`, currency/filesize; Carbon dates · CORE · I18N-8 | `l(date, format:)`, per-locale formats, number/currency helpers · CORE · I18N-7 | — (explicit gap) · I18N-5 | locale-aware render + localized form parsing, `{% localize %}`, filters, FORMAT_MODULE_PATH, separators · CORE/OPT · I18N-29–34 |
| Time-zone framework (as i18n concern) | — (schedule tz only · P9 JOB-28) | — in P15 (`Time.zone` lives in Active Support · EXT-10) | — | USE_TZ UTC storage, helpers, override, template tags/filters, aware forms, per-DB tz · CORE/OPT/DIY · I18N-36–48 |
| Client-side (JS) translation catalogs | — | — | — | JavaScriptCatalog + JSONCatalog views (+ statici18n ECO) · OPT · I18N-12–15 |
| LiveView/SPA locale restoration | — | — | `on_mount RestoreLocale` hook pattern · DIY · I18N-3 | — |
| Localized mail/notifications | mailable `locale()` + `HasLocalePreference` · CORE · I18N-7 | mailer subject lookup + mailer t/l integration · CORE · I18N-1/12 | — | — |
| Bidi / RTL support | — | — | — | `LANGUAGE_BIDI` tags + language info API · CORE · I18N-9/10 |
| Translator comments in source | — | — | — | `# Translators:` surfaced into .po · CORE · I18N-6 |
| Vendor/package translation override | `lang/vendor/{package}` · CORE · I18N-6 | `I18n.load_path` ordering · CORE · I18N-3 | — | LOCALE_PATHS > app > framework precedence · CORE · I18N-23 |
| Pluggable translation backend | — | chained/KeyValue/DB backends · CORE · I18N-13 | — | — (gettext fixed) |
| Country-specific form fields | — | — | — | django-localflavor · ECO · I18N-35 |

**Notable divergences.** Message format splits the field in two camps: gettext PO/MO with real extraction tooling (Django, Phoenix) versus framework-native hand-maintained key files (Laravel PHP/JSON, Rails YAML) — which cascades into everything else (msgctxt, translator comments, and `makemessages` exist only on the gettext side; lazy scoped keys and pluggable backends only on the key-file side). Locale negotiation is shipped middleware only in Django; Rails documents the pattern, Phoenix explicitly makes it your plumbing, Laravel stops at a config setting. Depth is wildly uneven — Django 48 rows (including an entire timezone framework filed under i18n), Rails 13, Laravel 8 (self-described "comparatively thin"), Phoenix 5 (two of which are gap rows). Pluralization sophistication follows the same gradient: CLDR categories (Rails) vs gettext plural-forms (Django) vs a bespoke pipe/range syntax (Laravel) vs undocumented (Phoenix).

**Volt notes.**
- Go's `golang.org/x/text` gives CLDR plural rules and message catalogs natively; the gettext-vs-native fork here is exactly the Django/Phoenix-vs-Rails/Laravel split — gettext buys a mature translator-tool ecosystem, a native format buys type-safe keys and no C-heritage tooling.
- Extraction is the hidden cost: only the gettext camp gets it free (`xgettext`); a native-format Go framework would need its own static extractor (feasible — Go parses cleanly), or translations rot.
- Accept-Language → cookie → URL-prefix negotiation middleware is small, has no dependencies, and only one of four frameworks ships it — a cheap differentiator.
- Django is the only one to treat timezone correctness (UTC-in-DB, aware datetimes, per-request activation) as part of i18n; in Go, `time.Time` carries location natively, but the *conventions* (store UTC, activate per request/user) still need a framework opinion.

## P16 — Testing support

- **Laravel**: Pest/PHPUnit scaffolding plus an unusually rich fake ecosystem — every facade subsystem ships `::fake()` + assertions, per-test DB reset, in-process HTTP tests, Dusk for real browsers.
- **Rails**: A complete Minitest harness generated with every app — YAML fixtures + transactional rollback, integration & system tests (Capybara/Selenium bundled), a dedicated TestCase per framework component, parallel by default, 8.1 local-CI DSL.
- **Phoenix**: ExUnit + generated case templates (`ConnCase`/`DataCase`/`ChannelCase`); the SQL Sandbox gives every test its own rolled-back transaction, enabling `async: true` concurrent DB tests; every generator ships tests and fixtures.
- **Django**: A unittest test-case hierarchy (`SimpleTestCase` → `TransactionTestCase` → `TestCase`) with an in-process sync/async `Client`, framework-aware assertions, settings overrides, automatic email doubles, managed test-DB lifecycle, and a swappable runner.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Test framework & app scaffolding | Pest (default)/PHPUnit, `tests/Feature`+`Unit`, `make:test` · CORE · TEST-1 | Minitest tree generated with app, `test "..."` DSL · CORE · TEST-1 | ExUnit + generated case templates, passing suite on `phx.new` · CORE · TEST-1 | unittest hierarchy (Simple/Transaction/TestCase) · CORE · TEST-1..3 |
| Test runner: filtering & selection | `artisan test` (filters, `--compact`) · CORE · TEST-1 | `bin/rails test` (file/line/name, fail-fast) · CORE · TEST-2 | per-file/line runs, `@tag` + `--only/--exclude` · CORE · TEST-10 | `test` cmd `-k`, `@tag`/`--tag` · CORE/OPT · TEST-19 (+P17 CLI-21) |
| Test environment isolation | auto `testing` env, array/sync drivers, `.env.testing` · CORE · TEST-2 | auto test-schema maintenance from schema dump · CORE · TEST-5 | `test` mix alias chains `ecto.create`+`migrate`+`test` · CORE · TEST-15 | auto test-DB create/destroy, `--keepdb`, `TEST` dict, `DEBUG=False` · CORE · TEST-22 |
| Parallel execution | opt-in `--processes`, per-process DBs, `ParallelTesting` hooks · CORE · TEST-3 | **default-on** forked processes w/ per-process DBs · CORE · TEST-13 | per-test concurrency via async sandbox; CI `--partitions` w/ auto per-partition DBs · CORE · TEST-5, TEST-11 | opt-in `--parallel` per-process DB (needs `tblib`) · OPT · TEST-23 |
| Randomized ordering / seed control | — (not in inventory; PHPUnit-level) | seed control in runner · CORE · TEST-2 | `--seed` deterministic randomization · CORE · TEST-11 | `--shuffle`/`--reverse` · OPT · TEST-23 |
| In-process HTTP/feature tests | `get/post/json`, headers/cookies/session seeding · CORE · TEST-4 | `IntegrationTest` verb helpers, `xhr:`, `as: :json` · CORE · TEST-6 | `ConnCase` + `build_conn()`, verified `~p` routes · CORE · TEST-2, TEST-3 | test `Client` (all verbs, JSON, file upload, CSRF toggle) · CORE · TEST-5 |
| Response assertions (HTML/JSON) | 100+ assertion vocabulary + fluent `AssertableJson` · CORE · TEST-6, TEST-7 | `assert_response/redirected_to` + `assert_dom` (Nokogiri) · CORE · TEST-3, TEST-9 | `html_response/json_response/redirected_to` · CORE · TEST-3 | `assertContains/Redirects/HTMLEqual/JSONEqual/FormError`… · CORE · TEST-13 |
| Multi-request / multi-session flows | session seeding on requests · CORE · TEST-4 | `follow_redirect!`, `open_session` multi-user · CORE · TEST-7 | — | `follow=True` + `redirect_chain` · CORE · TEST-5 |
| Auth helpers in tests | `actingAs` (+guard) · CORE · TEST-4 | `session`/`cookies` access · CORE · TEST-6 | generated `register_and_log_in_user` setup helpers · CORE · TEST-9 | `login/force_login` (+async variants) · CORE · TEST-6 |
| DB isolation strategy | `RefreshDatabase` (rollback) / `DatabaseMigrations` / `Truncation` · CORE · TEST-11 | transactional rollback per test, opt-out · CORE · TEST-5 | SQL Sandbox per-test transaction, **concurrent-safe** · CORE · TEST-5 | `TestCase` atomic rollback; `TransactionTestCase` truncation; `serialized_rollback` · CORE/OPT · TEST-2, TEST-3, TEST-24 |
| DB state assertions | `assertDatabaseHas/Count`, `assertModelExists`, `expectsDatabaseQueryCount` · CORE · TEST-12 | `assert_difference`/`assert_changes` · CORE · TEST-3 | `errors_on/1` changeset helper; layered test guidance · CORE · TEST-6 | `assertNumQueries`, `assertQuerySetEqual` · CORE · TEST-14 |
| Test data: fixtures vs factories | model factories + `seed()` per test · CORE · TEST-11 | YAML fixtures per model (labels, ERB, attachments) · CORE · TEST-4 | generated fixture modules; hand-rolled factory functions "sufficient" · CORE/DIY · TEST-9, TEST-14 | `fixtures` attr (dumpdata format) + `setUpTestData` per-class · CORE/OPT · TEST-11, TEST-12 |
| Tests generated with scaffolds | `make:test` (manual) · CORE · TEST-1 | scaffold generates tests (see P17 CLI-2) · CORE · TEST-1 | **every** generator emits tests + fixtures · CORE · TEST-9 | — |
| Browser / system testing | Dusk: ChromeDriver DSL, waits, screenshots, page objects · OPT · TEST-16, TEST-17 | Capybara + Selenium bundled, `driven_by`, failure screenshots · CORE · TEST-8 | — (LiveViewTest covers most UI needs w/o browser) | `LiveServerTestCase` + Selenium (third-party) · OPT/ECO · TEST-4, TEST-21 |
| Stateful-frontend tests w/o browser | — | — | `Phoenix.LiveViewTest`: element selection, `render_hook`, DOM checks · CORE · TEST-8 | — |
| View/template unit tests (no HTTP) | `view()/blade()/component()` render + assert · CORE · TEST-9 | partial & helper test cases, `assert_dom` · CORE · TEST-9 | `render_to_string/4` · CORE · TEST-13 | response `templates`/`context` inspection, `assertTemplateUsed` · CORE · TEST-10, TEST-13 |
| Console-command testing | `artisan()->expectsQuestion/Output/Table`, exit codes · CORE · TEST-10 | — | — | `call_command` + `StringIO` capture · CORE · TEST-18 |
| Mail testing | `Mail::fake` + assertions · CORE · TEST-15 | `:test` delivery + `assert_emails` · CORE · TEST-10 | — (Swoosh test adapter lives in P11) | `mail.outbox` capture, per-test reset · CORE · TEST-17 |
| Job/queue testing | `Queue/Bus::fake` · CORE · TEST-15 | `assert_enqueued/performed_jobs`, `perform_enqueued_jobs` · CORE · TEST-11 | — (Oban is OPT, outside P16) | — (task doubles mentioned only in preamble) |
| WebSocket/channel testing | `Event`/broadcast fakes · CORE · TEST-15 | connection/channel tests, broadcast assertions · CORE · TEST-12 | `ChannelCase`: `subscribe_and_join`, `assert_push/broadcast/reply` · CORE · TEST-7 | — |
| Subsystem fakes (breadth) | Event/Mail/Notification/Queue/Bus/Storage/Http/Process/Sleep/AI fakes · CORE · TEST-15 | per-component TestCases instead of fakes · CORE · TEST-10..12 | — (real processes + sandbox instead) | email/storage doubles + settings override · CORE · TEST-15, TEST-17 |
| Outbound HTTP-client faking | `Http::fake` (URL maps, sequences), `preventStrayRequests` · CORE · TEST-18 | — (WebMock/VCR are ecosystem, not in inventory) | — | — |
| File upload & storage testing | `UploadedFile::fake` + `Storage::fake` · CORE · TEST-8 | file-attachment fixtures · CORE · TEST-4 | — | Client file upload; `InMemoryStorage` · CORE/OPT · TEST-5, TEST-12 |
| Time control | `travel()`, `freezeTime` · CORE · TEST-14 | `travel/travel_to/freeze_time` · CORE · TEST-14 | — | — |
| Mocking/stubbing | Mockery: `mock/spy`, facade mocking · CORE · TEST-13 | — (not in inventory) | — (hand-rolled patterns; DIY stance) · TEST-14, TEST-16 | `mock.patch` guidance only · DIY · TEST-30 |
| Exception / error-path testing | `Exceptions::fake`, `withoutExceptionHandling` · CORE · TEST-5 | `assert_raises` documented flow · CORE · TEST-16 | `assert_error_sent 404` (exception→status) · CORE · TEST-4 | `assertRaisesMessage/WarnsMessage` · CORE · TEST-13 |
| Settings/config override per test | `.env.testing` + `phpunit.xml` vars · CORE · TEST-2 | — | — | `override_settings/modify_settings` + `setting_changed` signal · CORE · TEST-15 |
| Coverage & profiling | `--coverage`, `--min`, `--profile` built-in · CORE · TEST-3 | — | — | `coverage.py` (named third-party); `--durations` · ECO/OPT · TEST-29, TEST-23 |
| Local CI orchestration | Dusk CI recipes (GH Actions etc.) · OPT · TEST-17 | `config/ci.rb` DSL + `bin/ci` + `gh signoff` (8.1) · CORE · TEST-15 | `MIX_TEST_PARTITION` CI partitioning · CORE · TEST-11 | — |
| Async client / async test support | — | — | (concurrency is execution-level, TEST-5) | `AsyncClient`, `AsyncRequestFactory`, auto-wrapped `async def` tests · OPT · TEST-7, TEST-9 |
| Runner extensibility & multi-DB tests | — | — | — | swappable `TEST_RUNNER`/`DiscoverRunner`; `databases` attr, `MIRROR`/`DEPENDENCIES` · OPT · TEST-27, TEST-25 |
| Plug/middleware unit testing | — | — | `Plug.Test`: `conn/2` + direct `call/2`, no server · CORE · TEST-12 | `RequestFactory` (view w/o middleware) · CORE · TEST-8 |

**Notable divergences.** The four disagree most on *what substitutes for the real thing in a test*: Laravel fakes every subsystem behind its facades (TEST-15) and even the HTTP client (TEST-18); Rails prefers real objects with per-component TestCases and `:test` adapters; Phoenix rejects both fakes and factory libraries (DIY stance, TEST-14) and instead makes real DB access safe and concurrent via the SQL Sandbox; Django sits in between with a few automatic doubles (mail outbox) plus `override_settings`. Parallelism philosophy differs by grain: Rails parallelizes by default at the process level, Laravel and Django make it opt-in, and Phoenix is the only one with *per-test* concurrency against a real database. Browser testing tiers diverge fully — CORE and bundled in Rails, a separate OPT package in Laravel (Dusk), OPT/ECO in Django, and absent in Phoenix, which instead offers the unique LiveViewTest (browser-fidelity UI tests without a browser). Test data splits factories (Laravel) vs YAML fixtures (Rails, Django) vs generated fixture modules + plain functions (Phoenix).

**Volt notes.**
- `go test` already covers ~6 rows the frameworks had to build (runner, filtering, parallelism via `t.Parallel`, seed/shuffle, coverage, benchmarks) — Volt's leverage is in what `go test` lacks: DB isolation, an HTTP test client with framework-aware assertions, and subsystem doubles.
- Phoenix's SQL Sandbox is the only DB-isolation design compatible with `t.Parallel`; Rails/Laravel/Django-style single-transaction rollback breaks under Go's default concurrent tests. This is arguably the highest-value P16 feature to port.
- Laravel's fake ecosystem depends on global facades/container swapping — un-Go-like; the Go-idiomatic equivalent is interfaces + provided fake implementations (mail, queue, storage, clock), which Rails' adapter approach (`:test` delivery) resembles more than Laravel's.
- Time control (Laravel/Rails CORE) needs deliberate design in Go: there is no monkey-patching, so a framework-injected clock interface must exist from day one or clock-dependent code is untestable.

## P17 — CLI, code generation & developer experience

- **Laravel**: Artisan — one binary exposing ~all framework ops plus the `make:*` generator family — Tinker REPL, the Prompts TUI library, starter kits for whole-app scaffolding.
- **Rails**: The `bin/rails` suite — app/scaffold/model generators on Thor, an app-loaded REPL, runners, application templates — all extensible and configurable.
- **Phoenix**: `mix phx.new` + `phx.gen.*` generators that write code *into your app* (contexts, LiveViews, auth) as an explicit learning tool/starting point; hot code reload; IEx-first workflow.
- **Django**: One command framework, three entry points, ~40 built-in ops commands (migrations, data dump/load, i18n, inspection); minimal codegen (project/app skeletons only); custom commands as a first-class extension point.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| CLI entry point & command framework | Artisan: `list/help`, `--env`, extensible `about` · CORE · CLI-1 | `bin/rails` command suite (Thor) · CORE · CLI-1, CLI-10 | Mix tasks, `mix help` self-documenting · CORE · CLI-11 | `django-admin`/`manage.py`/`python -m django` · CORE · CLI-1 |
| New-app generator & presets | `laravel new` + starter-kit picker (React/Vue/Svelte/Livewire) · CORE/ECO · CLI-15 | `rails new`: `-d`, `--api`, `--devcontainer`, `--skip-*`, `--minimal` · CORE · CLI-1 | `phx.new` + composable `--no-*` flags (ecto/html/assets/live/mailer) · CORE · CLI-1, CLI-2 | `startproject`/`startapp` with `--template` dir/archive/URL · CORE · CLI-2, CLI-3 |
| Zero-toolchain bootstrap | php.new one-liner, Laravel Herd · CORE/ECO · CLI-15 | `rails-new` tool (no local Ruby) · OPT · CLI-15 | Phoenix Express curl installer (installs Erlang/Elixir too) · CORE · CLI-1 | — |
| Resource/CRUD scaffolding | `make:*` family (model `-mfsc --all`, controller variants, 20+ types) · CORE · CLI-3 | `scaffold` full CRUD incl. tests; `model` field:type syntax · CORE · CLI-2 | `phx.gen.html/json/live`: full vertical slice (context+schema+web+tests) · CORE · CLI-5 | — (only skeletons; no CRUD codegen) |
| Domain-layer-only generators | make:model/migration/seeder standalone · CORE · CLI-3 | `model`, `migration`, `resource` · CORE · CLI-2 | `phx.gen.context`/`phx.gen.schema`; re-gen *injects* into existing context · CORE · CLI-6 | — (`inspectdb` reverse-engineers models from legacy DB · CORE · CLI-20) |
| Generator field syntax | via make:model options · CORE · CLI-3 | `field:type[:index]`, `references` · CORE · CLI-2 | `title:string:unique`, `references`, `--scope` wiring · CORE · CLI-7 | — |
| Auth scaffolding | via starter kits · CORE/ECO · CLI-15 | `authentication` generator · CORE · CLI-2 | `phx.gen.auth` (largest generator; LiveView/controller variants) · CORE · CLI-8 | — (runtime contrib + `createsuperuser` · OPT · CLI-24) |
| Real-time scaffolding | — | `channel` generator · CORE · CLI-2 | `phx.gen.channel/socket/presence` · CORE · CLI-9 | — |
| Generator customization | `stub:publish` overrides templates · CORE · CLI-12 | `config.generators`, custom Thor generators, `lib/templates` overrides · CORE · CLI-3, CLI-4 | — (not in inventory) | `--template` mechanism on start* · CORE · CLI-2, CLI-3 |
| App templates / recipe DSL | community starter kits (Composer/repo) · ECO · CLI-15 | `rails new -m template.rb` DSL (`gem`, `route`, `ask`, `after_bundle`) · CORE · CLI-5 | — | — |
| Symmetric un-generate | — | `bin/rails destroy` · CORE · CLI-2 | — | — |
| Generated-code ownership stance | framework-owned stubs (customizable) · CLI-12 | conventional, config-swappable (e.g. rspec fallback) · CLI-3 | explicit: generators are learning tools; rename/refactor freely · CORE · CLI-15, CLI-6 | codegen avoided by design |
| REPL / console | Tinker (psysh): Eloquent/jobs/events, allow-list · CORE · CLI-2 | `rails console`: `app`+`helper`, `--sandbox` rollback · CORE · CLI-6 | `iex -S mix phx.server`, `recompile()`, `.iex.exs` config · CORE · CLI-3, CLI-16 | `shell`: auto-imports models (**6.0**), ipython/bpython, customizable imports · CORE/DIY · CLI-6, CLI-7 |
| Production remote console | — | — | `bin/my_app remote` live IEx; `eval` one-offs · CORE · CLI-17 | — |
| DB console | — (not in inventory) | `dbconsole` multi-DB aware · CORE · CLI-7 | — | `dbshell` (psql/mysql/sqlite/sqlplus passthrough) · CORE · CLI-8 |
| One-off script runner | `Artisan::call` from code · CORE · CLI-11 | `bin/rails runner` in app context · CORE · CLI-8 | release `eval` · CORE · CLI-17 | `shell -c` / stdin exec · CORE · CLI-6 |
| Dev server & process runner | Herd local env · CORE/ECO · CLI-15 | `server` (Puma), `bin/dev`, `bin/setup` · CORE · CLI-9 | `mix phx.server` · CORE · CLI-3 | `runserver` w/ autoreload (Watchman opt.); ASGI dev via ECO `daphne-runserver` · CORE/ECO · CLI-4, CLI-5 |
| Hot reload / live reload | — (Vite HMR lives in P3) | — (bundler watch via `bin/dev`) · CLI-9 | hot code reload + browser live-reload, CSS w/o refresh · CORE · CLI-4 | Python autoreload per request · CORE · CLI-4 |
| Custom commands | `make:command`, `$signature` DSL, DI `handle()`, closure cmds · CORE · CLI-4, CLI-5 | rake tasks in `lib/tasks` w/ app env · CORE · CLI-12 | Mix aliases as workflow glue (user-editable) · CORE · CLI-13 | `management/commands/` `BaseCommand` (+App/LabelCommand) · DIY · CLI-32, CLI-33 |
| Interactive prompts / TUI toolkit | Prompts: text/select/search/form + spinners/progress, testable, fallbacks · CORE · CLI-8, CLI-9, CLI-10 | `ask`/`yes?` in template DSL only · CORE · CLI-5 | — | colored output + `DJANGO_COLORS` only · CORE · CLI-28 |
| Auto-prompt for missing args | `PromptsForMissingInput` · CORE · CLI-7 | — | — | — |
| Programmatic command invocation | `Artisan::call/queue`, `$this->call` between cmds · CORE · CLI-11 | — | — | `call_command()` w/ stream redirect · CORE · CLI-31 |
| Command locking / isolation | `--isolated` cache-lock single execution · CORE · CLI-6 | — | — | — |
| Signal handling & console events | `trap`; CommandStarting/Finished events · CORE · CLI-12, CLI-13 | — | — | — |
| Introspection commands | `about` env overview · CORE · CLI-1 | `routes/about/stats/initializers/middleware/notes` (TODO scanner) · CORE · CLI-10 | `mix phx.routes` · CORE · CLI-11 | `check` (system checks), `diffsettings`, `showmigrations` · CORE · CLI-9, CLI-10, CLI-15 |
| Migration/DB task CLI | `make:migration` (rest in P5) · CORE · CLI-3 | (db:* tasks live in P5) | `ecto.create/migrate/rollback/gen.migration` w/ friendly diagnostics · CORE · CLI-12 | full suite: `makemigrations/migrate/sqlmigrate/squash/optimize/flush` · CORE · CLI-13..19 |
| Data dump/load CLI | — | — | — | `dumpdata`/`loaddata` (compression, natural keys) · CORE · CLI-11, CLI-12 |
| Secrets/credentials CLI | — (key:generate not in inventory) | `credentials:edit/fetch`, `db:encryption:init` · CORE · CLI-11 | `phx.gen.secret` · CORE · CLI-10 | — |
| Deployment/release codegen | — | — | `phx.gen.release [--docker]`; `phx.gen.cert` dev TLS · CORE · CLI-10 | — |
| Code style / formatting tooling | Pint: zero-config fixer, presets, CI modes · OPT · CLI-14 | RuboCop `rails-omakase` in default Gemfile · ECO · CLI-14 | — (mix format not in inventory) | auto-`black` of generated files when on PATH · OPT · CLI-30 |
| i18n / cache infra commands | — | — | — | `makemessages/compilemessages`, `createcachetable` · OPT · CLI-23 |
| Uniform global CLI flags & UX | verbosity levels, `--env` · CORE · CLI-1 | — | — | `--settings/--pythonpath/--verbosity/--no-color/--skip-checks` on every command; bash completion · CORE/OPT · CLI-27, CLI-29 |
| Dev containers / IDE support | Boost editor/agent setup docs · CORE · CLI-16 | generated `.devcontainer`, Codespaces flow · OPT · CLI-13 | — | — |
| Dev error pages w/ actions | — | — | actionable buttons (run migrations/seeds) via `Plug.Exception`; `debug_errors` · CORE · CLI-18 | — (debug page lives outside P17) |
| Directory-layout convention as DX | — | test/ mirrors app/ (P16 TEST-1) | `lib/app` vs `lib/app_web` split documented as architectural statement · CORE · CLI-14 | — |

**Notable divergences.** Codegen philosophy is the sharpest split: Django essentially refuses CRUD scaffolding (skeletons + `inspectdb` only), Rails and Laravel generate heavily but treat output as convention-shaped framework code, and Phoenix generates the most per invocation (full vertical slices, the giant `phx.gen.auth`) while explicitly declaring the output *yours* — a learning tool to rename and refactor. Where Django invests instead is operational breadth: ~40 built-in commands (migrations, dumpdata/loaddata, i18n, checks) with uniform global flags, versus Laravel investing in CLI *ergonomics* (Prompts TUI, isolatable commands, auto-prompting, console testing) that no other framework matches. REPL centrality varies: Rails and Django load the whole app into a console, Laravel sandboxes Tinker with an allow-list, and Phoenix goes furthest — the REPL *is* the runtime (`iex -S mix`) and extends into production (`bin/app remote`), something only the BEAM can offer. Custom-command extension is CORE and DSL-driven in Laravel/Rails, but DIY-tier plumbing in Django.

**Volt notes.**
- Go has no REPL: Tinker/console/`iex`/`shell` — CORE in all four — is simply unavailable. The realistic substitutes are a `runner`-style command executing snippets in app context, `dbshell` passthrough, and rich introspection commands (`routes`, `about`, `check`); Volt should over-invest there to compensate.
- Codegen is *more* natural in Go than in these languages, since Go lacks the runtime metaprogramming Rails/Laravel use to avoid writing code; Phoenix's model (generate explicit code into the user's app, declare it user-owned, inject into existing files on re-run) fits Go's explicitness culture best, and `gofmt` gives the Django-style auto-format for free.
- Single static binary changes the CLI shape: the app binary itself can carry ops commands (migrate, routes, eval-ish tasks) à la Phoenix releases, separating dev-time codegen (a `volt` tool) from run-time ops (the compiled app) — none of the four has to make that split.
- Hot reload is table stakes in all four (autoreload/live-reload CORE in Phoenix/Django, watch flows in Rails); Go needs rebuild-and-restart tooling (air-style watcher) plus browser live-reload to match, which the framework must ship or bless since the compiler makes DIY setups fiddly.

## P18 — Configuration, environments & deployment
(Django titles this "Configuration & deployment")

- **Laravel**: dotenv + plain PHP config files with aggressive production caching, `bootstrap/app.php` as single wiring point, maintenance mode, and a fleet of first-party env tools (Sail/Valet/Homestead/Envoy) plus paid hosting (Forge/Cloud).
- **Rails**: convention-first layered Ruby config with versioned `load_defaults`, Zeitwerk autoloading, and a fully owned deploy path: generated production Dockerfile + Kamal 2 + Thruster + Puma out of the box.
- **Phoenix**: compile-time config layering plus `runtime.exs` for boot-time env vars; `mix release` produces a self-contained artifact (VM included) with generated Docker and clustering support.
- **Django**: settings are a plain Python module (`DJANGO_SETTINGS_MODULE`) with dict-of-alias config families; deployment is a WSGI/ASGI callable handed to ecosystem servers, gated by `check --deploy`.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Config format & source | dotenv + PHP config files · CORE · CONF-1, CONF-4 | Ruby `application.rb` + per-env files · CORE · CONF-1, CONF-3 | `config.exs` → per-env `.exs` layering · CORE · CONF-1 | Python settings module · CORE · CONF-1, CONF-2 |
| Environment selection/detection | `APP_ENV` + `App::environment()` · CORE · CONF-2 | env files + custom envs · CORE · CONF-1 | `MIX_ENV` · CORE · CONF-4 | `DJANGO_SETTINGS_MODULE` / `--settings` · CORE · CONF-1 |
| Runtime vs build-time config split | `env()` only in config files; cache cuts env off · CORE · CONF-1, CONF-6 | boot-time initializers (no split) · CORE · CONF-4 | `runtime.exs` vs compile-time; `force_ssl` compile-time called out · CORE · CONF-1, CONF-15 | all boot-time; no mutation after load · CORE · CONF-2 |
| Secrets handling | encrypted committable env files · CORE · CONF-3 | credentials + `.kamal/secrets` fetch · CORE · CONF-12 | `SECRET_KEY_BASE` + env vars in runtime.exs · CORE · CONF-2, CONF-8 | `SECRET_KEY` + `SECRET_KEY_FALLBACKS`, checklist · CORE · CONF-6 |
| Typed/structured config access | `config()` dot-notation typed getters · CORE · CONF-4 | `config.x.*` + `config_for` env-keyed YAML · CORE · CONF-6 | OTP app config (keyword lists) · CORE · CONF-1 | `settings` object; roll-your-own uppercase keys · CORE/DIY · CONF-2, CONF-5 |
| Backing-service config families | per-service config files, slim defaults · CORE · CONF-5 | `database.yml` + Solid trifecta DB wiring · CORE · CONF-7, CONF-15 | Repo config surface (pool, socket opts, types) · CORE · CONF-13 | dict-of-alias `DATABASES/CACHES/STORAGES/TASKS/MAILERS/TEMPLATES` · CORE · CONF-13, CONF-14 |
| Config caching/boot optimization | `config:cache`, `optimize` umbrella · CORE · CONF-6 | eager load + Bootsnap · CORE · CONF-8, CONF-10 | compile-time config is the cache · CORE · CONF-1 | — |
| Versioned defaults / upgrade path | upgrade docs + Shift/AI-assisted · DIY · CONF-18 | `load_defaults` + `new_framework_defaults_*` · CORE · CONF-2 | — | release notes + `-Wa` deprecation testing · CORE · CONF-17 |
| Debug-mode toggle | `APP_DEBUG` with prod warning · CORE · CONF-7 | per-env config · CORE · CONF-1 | `debug_errors` endpoint flag · CORE · CONF-3 | `DEBUG` critical setting · CORE · CONF-6 |
| App wiring / boot process | `bootstrap/app.php` chained builder · CORE · CONF-11 | documented boot pipeline + initializer chain · CORE · CONF-4, CONF-10 | supervision tree as boot config · CORE · CONF-14 | `django.setup()` + app registry · OPT · CONF-4 |
| Autoload / dev code reload | — (Composer, outside section) | Zeitwerk + file-watcher reload · CORE · CONF-8, CONF-9 | `code_reloader` + watchers · CORE · CONF-3 | — (runserver autoreload, outside section) |
| Production HTTP server story | nginx + FrankenPHP guidance · DIY · CONF-9 | Puma + Thruster in-box · CORE · CONF-13, CONF-14 | built-in endpoint, `mix phx.server` · CORE · CONF-3, CONF-5 | hand off WSGI/ASGI callable to Gunicorn/Uvicorn/etc. · CORE+ECO · CONF-8–11 |
| Deployable build artifact | — (guidance only) | multi-stage Dockerfile generated per app · CORE · CONF-11 | `mix release` self-contained dir + `bin/server`/`bin/migrate` · CORE · CONF-6 | — (checklist only) |
| Docker support | Sail (dev only) · OPT · CONF-13 | generated production Dockerfile · CORE · CONF-11 | `phx.gen.release --docker` prod Dockerfile · CORE · CONF-7 | — |
| First-party deploy tool | Envoy SSH runner · OPT · CONF-16 | Kamal 2 zero-downtime deploys · CORE · CONF-12 | — (platform guides: Fly/Gigalixir/Heroku · CORE docs · CONF-11) | — (checklist · CORE · CONF-6) |
| First-party hosting/platform | Forge + Laravel Cloud (paid) · ECO · CONF-17 | — | — (Fly.io auto-detect documented · CONF-11) | — |
| Local dev environments | Sail / Valet / Homestead · OPT · CONF-13, CONF-14, CONF-15 | — | watchers in dev endpoint · CORE · CONF-3 | — |
| Deploy config linting | optimization checklist · DIY · CONF-9 | — | — | `check --deploy` + per-env guidance · CORE · CONF-6, CONF-7 |
| Health endpoint | `/up` + `DiagnosingHealth` event · CORE · CONF-10 | `/up` · CORE · (P21 ADMIN-5) | — | — |
| Maintenance mode | `down`/`up`, bypass secret, multi-server · CORE · CONF-8 | — | — | — |
| Clustering / multi-node | — | — | `dns_cluster` default + libcluster; multi-node decision matrix · CORE/ECO · CONF-9, CONF-10, CONF-12 | — |
| Concurrency/runtime tuning config | — (Octane outside section) | executor/load-interlock semantics, `WEB_CONCURRENCY` · CORE · CONF-14, CONF-18 | supervision tree · CORE · CONF-14 | ASGI async-safety caveats · CORE · CONF-9 |
| Drop unused framework parts | slim unpublished configs · CORE · CONF-5 | à-la-carte railties vs `rails/all` · CORE · CONF-17 | — (deps-based) | `INSTALLED_APPS` (see P19 EXT-10) |
| Multi-site/domain awareness | — | — | — | contrib.sites + CurrentSiteMiddleware · OPT · CONF-20, CONF-21 |
| Subdirectory deploy / static toggles | — | `relative_url_root`, `RAILS_SERVE_STATIC_FILES` · CORE · CONF-16 | `cache_static_manifest`, url host config · CORE · CONF-3 | `STATIC_ROOT` + `collectstatic` pointer · OPT · CONF-15 |

**Notable divergences.** Rails and Phoenix own the path to production (generated Dockerfile + Kamal; `mix release` + generated Docker/clustering), while Laravel sells it (Forge/Cloud are the blessed story, OSS docs are DIY guidance) and Django explicitly stops at the WSGI/ASGI callable boundary, delegating servers to the ecosystem. Phoenix is alone in formalizing the compile-time vs boot-time config distinction (`runtime.exs`) and in treating clustering as a default-on concern; the other three are single-node-first. Laravel invests uniquely in local-dev environments (Sail/Valet/Homestead) and maintenance mode; Django invests uniquely in config linting (`check --deploy`) and multi-site config. Rails is the only one with versioned behavior defaults (`load_defaults`) as an upgrade mechanism.

**Volt notes.**
- Go compiles to a static binary, so Volt gets Phoenix's "self-contained release artifact" for free — the open questions are the Rails-style parts around it: generated Dockerfile, deploy tool, health endpoint, zero-downtime story.
- The compile-time/runtime config split Phoenix formalizes maps naturally onto Go (build-time vs env-at-boot); the Laravel/Rails "cache your config" machinery is a scripting-language workaround Volt doesn't need.
- Nobody except Django ships config linting (`check --deploy`); a typed-config language could do this at compile time — a potential differentiator.
- All four converge on: layered per-env config, env-var secrets at boot, a debug flag with prod warnings, and typed access to config — that's the table-stakes core.

## P19 — Extensibility: DI, events, hooks & packages

- **Laravel**: the service container is the universal seam — everything resolvable/replaceable; providers are the composition root, events decouple domains, macros/facades extend surfaces, and a documented package protocol powers the ecosystem.
- **Rails**: not DI — Railties. Every component is a Railtie; gems hook boot via initializers and `on_load`; engines are mountable mini-apps; `ActiveSupport::Notifications` is the pub/sub spine.
- **Phoenix**: no DI container, no global event bus. Extension is contracts: the Plug behaviour, Elixir protocols, Ecto callbacks, LiveView hooks, and telemetry — all wired explicitly.
- **Django**: no DI container — the composition unit is the *app* (`AppConfig.ready()` in a process-wide registry), wired by signals, checked by the system-check framework, with dotted-path settings as the override seam.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Service container / DI | reflection autowiring, full binding vocabulary, contextual bindings/attributes, tagging, PSR-11 · CORE · EXT-1–6 | none, deliberate · DIY · EXT-14 | none; function args + explicit wiring · — · EXT-12 | none; app registry instead · — · (P19 intro) |
| Composition root / boot hook | service providers `register`/`boot`, deferred providers · CORE · EXT-7, EXT-8 | Railtie initializers · CORE · EXT-1 | `MyAppWeb` macro module as app-wide seam · CORE · EXT-5 | `AppConfig.ready()` · CORE · EXT-10 |
| Reusable package/app unit | package protocol: auto-discovery + publishable groups · CORE · EXT-17 | engines (mountable mini-apps) + plugin scaffold · CORE · EXT-2, EXT-5 | Hex/`mix deps` (incl. sparse git deps) · CORE · EXT-10 | apps framework + reusable-app conventions · CORE/OPT · EXT-10, EXT-12 |
| Package ↔ host integration points | `mergeConfigFrom`, `loadRoutesFrom/Migrations/Views/Translations`, override cascade · CORE · EXT-18 | migration copying, host-class config, `main_app`/engine URL proxies · CORE · EXT-3 | pipelines + `forward` mount whole plug apps · CORE · EXT-11 | `INSTALLED_APPS` + app registry APIs · CORE · EXT-11 |
| Host overrides package behavior | publish + override cascade · CORE · EXT-17, EXT-18 | view path shadowing, `to_prepare` + class_eval decorators · CORE · EXT-4 | — | settings dotted-paths + subclassing · CORE · (P19 intro) |
| Domain event system | events/listeners, discovery, wildcards, subscribers, dispatch control, `Event::fake` · CORE · EXT-12, EXT-14, EXT-15 | `ActiveSupport::Notifications` as general pub/sub · CORE · EXT-6 | no bus; `Phoenix.PubSub` + `handle_info` idiom · CORE (idiom) · EXT-13 | `Signal` w/ weak refs, `dispatch_uid`; custom signals discouraged · CORE/DIY · EXT-1, EXT-9 |
| Framework lifecycle events | framework events via same bus · CORE · EXT-12 | `on_load` lazy hooks + Notifications catalog · CORE · EXT-7, EXT-6 | telemetry events across the stack · CORE · EXT-6 | model/request/migrate/db/test/task signals · CORE/OPT · EXT-3–8 |
| Async/queued event handling | queued listeners: `ShouldQueue`, backoff, after-commit · CORE · EXT-13 | — (Active Job separately) | processes/PubSub natively async · CORE · EXT-13 | `asend`, async receivers via TaskGroup · CORE · EXT-2 |
| Middleware contract as extension seam | — (see P2) | — (Rack, see P2) | Plug behaviour: `init/1`+`call/2` slots anywhere · CORE · EXT-1 | — (see P2) |
| Interface/protocol extension points | contracts package for every component · CORE · EXT-11 | ActiveModel lint tests · CORE · EXT-13 | protocols (`Phoenix.Param`, `FormData`, `Plug.Exception`) + Ecto adapter/repo callbacks · CORE · EXT-2, EXT-3 | subclass-based protocols · CORE · (P19 intro) |
| Extend core API surfaces in place | macroable Response/HTTP/collections/strings; container `extend()` · CORE · EXT-16, EXT-5 | `Concern` mixins + core extensions to Ruby itself · CORE · EXT-8, EXT-9 | — (closed modules; use behaviours) | — |
| Static-ergonomics layer | facades + real-time facades · CORE · EXT-9, EXT-10 | — | — | — |
| Generator/scaffold extensibility | — (stubs, see P17) | `hook_for :orm/:template_engine/:test_framework` · CORE · EXT-12 | scope config reshapes future generators · CORE · EXT-9 | — |
| UI-framework lifecycle hooks | — | — | `on_mount` / `attach_hook` LiveView hooks · CORE · EXT-4 | — |
| Config linting / system checks | — | — | — | check framework: tags, custom checks, deploy checks, silencing · CORE/DIY/OPT · EXT-13–19 |
| Generic model references | — (morphs, see P4) | — | — | contenttypes: GFK, GenericRelation, generic inlines, GenericPrefetch · OPT · EXT-20–25 |
| Stdlib-augmentation toolkit | — (see P25) | core extensions, time/date, `HashWithIndifferentAccess`, inquirers, callbacks · CORE · EXT-9–11 | — (Elixir stdlib suffices) | `django.utils.*`: functional/html/text/timezone/module_loading/encoding · OPT · EXT-26–31 |
| Sanctioned internal-boundary pattern | — | — | contexts: public module, private schemas · CORE (pattern) · EXT-8 | apps as boundaries · CORE · EXT-10 |
| Load-order-safe lazy extension | deferred providers · CORE · EXT-8 | `ActiveSupport.on_load` · CORE · EXT-7 | — (OTP app deps order) | three-stage `django.setup()` loading · CORE · EXT-12 |

**Notable divergences.** The DI split is the sharpest disagreement in the whole comparison: Laravel makes a service container the universal extension seam; Rails, Phoenix, and Django all explicitly reject one, substituting Railties/conventions, explicit functional wiring, and the app registry respectively — three different "no"s to the same question. Event systems diverge just as much: Laravel has a rich domain-event bus with queued listeners, Django has signals (whose own docs discourage them vs. explicit calls), Rails repurposes its instrumentation bus, and Phoenix deliberately has no bus at all (PubSub + message passing is the idiom). Rails and Laravel embrace open-class/macro modification of core surfaces; Phoenix and Django cannot or do not, extending via protocols/subclassing instead. Only Rails (engines) has a package unit that carries its own routes/MVC/migrations as a mountable sub-application; Django's apps are close but flatter; Laravel packages integrate via a publish protocol; Phoenix packages are plain libraries plus `forward`.

**Volt notes.**
- Go can't do Laravel-style reflection autowiring ergonomically or Rails-style monkey-patching at all; the realistic seams are Phoenix's — interfaces (behaviours/protocols), explicit middleware contracts, and compile-time codegen — which is also the answer 3 of 4 frameworks chose anyway.
- Every framework has *some* blessed "package registers itself at boot" hook (provider / Railtie / AppConfig.ready / OTP app). Go's `init()` is too weak and implicit; Volt needs an explicit registration contract.
- The event-bus question is genuinely open: strong bus (Laravel) vs instrumentation-bus-doubling-as-events (Rails) vs none (Phoenix). Go channels ≠ an app-level event API; Volt must decide tier, not just mechanism.
- Django's system-check framework (config linting with tags, extensible by packages) has no equivalent in the other three and fits a static language well.

## P20 — Observability: logging, metrics, errors

- **Laravel**: Monolog channel logging + a centralized exception pipeline in core; Telescope (local debug), Pulse (prod perf dashboard), Pail (log tailing) as first-party add-ons; no metrics/tracing exporters.
- **Rails**: tagged logger + a ~70-event instrumentation bus, a first-class error-reporter API spanning requests/jobs, and (8.1) structured event reporting; APMs subscribe rather than monkey-patch.
- **Phoenix**: `:telemetry` events emitted by every layer, `Telemetry.Metrics` declarations, pluggable reporters, LiveDashboard as built-in visualizer — metrics-first, error-tracking absent.
- **Django**: stdlib `logging` via `LOGGING` dictConfig with framework-named loggers, error reports emailed to `ADMINS` with sensitive-data scrubbing, rich debug page; no metrics or tracing story.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Logging core | Monolog channels (daily/slack/syslog/custom) · CORE · OBS-1 | `BroadcastLogger` over AS::Logger, per-env levels · CORE · OBS-1 | Plug/Phoenix logger · CORE · OBS-9 | stdlib `LOGGING` dictConfig + defaults · CORE · OBS-1, OBS-2 |
| Multi-destination fan-out | log stacks, per-channel levels · CORE · OBS-2 | BroadcastLogger · CORE · OBS-1 | — (Logger backends, outside corpus) | handlers/filters per logger · CORE · OBS-1 |
| Structured/contextual log metadata | `Context` request/job-scoped, queue-propagating; `withContext` · CORE · OBS-3, OBS-10 | tagged logging (`log_tags`, `tagged` blocks) · CORE · OBS-2 | request-id metadata · CORE · OBS-10 | — (filters only · OBS-5) |
| Framework-named logger taxonomy | — | — | `Phoenix.Logger` event catalog · CORE · OBS-5 | `django.request/db/security.*/server…` loggers · CORE · OBS-3 |
| Instrumentation bus / event spans | — (gap; domain events only) | Notifications: ~70 documented timed events · CORE · OBS-3 | `:telemetry` events everywhere + span pattern · CORE · OBS-1, OBS-5–8 | — (signals are not timed instrumentation) |
| Structured app-event reporting | — | `Rails.event.notify` (8.1), tags, pluggable emit · CORE · OBS-6 | `:telemetry.execute` custom events · CORE · OBS-8 | — |
| Metrics declaration/aggregation | — · OBS-15 (named gap) | — (events are raw material) · DIY · OBS-14 | `Telemetry.Metrics` 5 types, tags, poller · CORE · OBS-2, OBS-3 | — (no story, per P20 intro) |
| Metrics exporters (StatsD/Prometheus/OTel) | — · OBS-15 | — · DIY · OBS-14 | pluggable reporters; Console bundled, rest hex · CORE/ECO · OBS-4 | — |
| Error/exception reporting API | `report()` hooks, per-type levels, ignore lists, dedup · CORE · OBS-6 | `Rails.error.handle/record/report` + subscribers, auto-wrapped executions · CORE · OBS-4 | — (telemetry is the surface) · — · OBS-13 | ADMINS email on 500, MANAGERS on 404 · OPT · OBS-4, OBS-8 |
| Error-service integration model | report hooks · CORE · OBS-6 | Sentry et al. register reporter subscribers · ECO · OBS-13 | telemetry handlers · — · OBS-13 | logging handler swap · OPT · OBS-4, OBS-6 |
| Soft assertions | — | `Rails.error.unexpected`: raise in dev, report in prod · CORE · OBS-5 | — | — |
| Error report throttling/sampling | `Lottery` sampling, `Limit` caps · CORE · OBS-8 | — | — | `IGNORABLE_404_URLS` only · OPT · OBS-8 |
| Sensitive-data scrubbing in reports | — (hidden context · OBS-10) | param filtering (see P8) | password/token param filtering in logs · CORE · OBS-9 | `sensitive_variables`/`sensitive_post_parameters`, reporter filters · OPT/DIY · OBS-9, OBS-10 |
| Custom error rendering (prod) | per-type `render()`, JSON-vs-HTML, per-status views · CORE · OBS-7, OBS-9 | — (public/*.html, outside section) | ErrorHTML/ErrorJSON modules · CORE · OBS-12 | overridable 400/403/404/500 templates + views · CORE · OBS-11 |
| Dev error pages / debuggers | — (Telescope dumps · OBS-11) | web-console REPL, actionable errors, `debug` gem · CORE · OBS-8, OBS-9 | Plug.Debugger-style pages with actions · CORE · OBS-12 | debug 500 page w/ locals · CORE · OBS-11 |
| Request correlation IDs | via `Context` · CORE · OBS-10 | `log_tags :request_id` · CORE · OBS-2 | `Plug.RequestId` → `x-request-id` · CORE · OBS-10 | — |
| SQL observability | Telescope query watcher w/ slow flag · OPT · OBS-11 | verbose query logs (caller line), SQL comment tags · CORE · OBS-7 | Ecto telemetry timings + dev debug logs · CORE · OBS-6, OBS-9 | `django.db.backends` logger (DEBUG only), `test --debug-sql` · OPT · OBS-7 |
| Live observability dashboard | Telescope (19 watchers, local) + Pulse (prod, custom cards) · OPT · OBS-11, OBS-12, OBS-13 | — | LiveDashboard: real-time metrics charts + request logger · CORE · OBS-11 | — |
| Log tailing tooling | Pail CLI (`--filter/--level/--user`) · OPT · OBS-5 | — | LiveDashboard RequestLogger · CORE · OBS-11 | — |
| Subsystem health monitors | `db:monitor`, `queue:monitor` + alert events · CORE · OBS-14 | — | telemetry_poller VM measurements · CORE · OBS-2 | — |
| In-browser perf surfacing | — | `ServerTiming` middleware → DevTools · CORE · OBS-10 | — | — |
| Logger internals customization | Monolog tap/formatters/processors · CORE · OBS-4 | custom loggers per component · CORE · OBS-1 | — | `LOGGING_CONFIG` swap/disable · OPT · OBS-6 |

**Notable divergences.** No framework ships a metrics *exporter*, but they differ on how close they get: Phoenix has a full metrics pipeline (declare + aggregate + pluggable reporters, dashboard) stopping only at the wire format; Rails ships the instrumentation events and says "subscribe"; Laravel and Django name it a flat gap. Error reporting inverts that ranking: Laravel and Rails have rich first-class error-reporter pipelines (hooks, dedup, sampling, subscriber APIs), Django's is a 1.0-era email-the-admins design with excellent scrubbing, and Phoenix has nothing — telemetry is presumed sufficient. Dashboards split three ways: Phoenix bundles LiveDashboard in every app (CORE), Laravel offers Telescope/Pulse as opt-in packages, Rails and Django offer none. Rails 8.1's structured event reporter and `Rails.error.unexpected` soft assertions are novel and have no counterpart elsewhere.

**Volt notes.**
- Go's ecosystem standard is OpenTelemetry + `log/slog`; every one of these four predates or ignores OTel — shipping OTel-native instrumentation (Phoenix's telemetry model with an exporter, closing the gap all four share) is the clearest open lane.
- The Phoenix/Rails pattern — framework emits named timed events, everything else (logs, metrics, APM) subscribes — decouples core from vendors and is directly portable to Go.
- An error-reporter API à la Rails (`handle/record/report` + subscribers, spanning HTTP and background work) is cheap in core and prevents every APM from reinventing panic-recovery middleware.
- Sensitive-data scrubbing appears in three of four inventories (params, locals, headers); it needs to be designed into logging/error paths from day one, not bolted on.

## P21 — Admin & operational UIs

- **Laravel**: operational dashboards for its own subsystems (Horizon, Telescope, Pulse) behind gates; no domain CRUD admin in OSS (Nova is commercial, Filament community).
- **Rails**: thin, targeted surfaces (mailer previews, inbound-mail conductor, Mission Control jobs UI); data admin is app code you write by hand.
- **Phoenix**: LiveDashboard for runtime introspection in every default app; no data-admin framework — generators scaffold your own `/admin` CRUD.
- **Django**: `django.contrib.admin`, the flagship — register a model and get list/search/filter/edit/history/inlines/actions/permissions/theming; 39 inventory rows on its own.

| Capability | Laravel | Rails | Phoenix | Django |
|---|---|---|---|---|
| Domain CRUD admin framework | — (commercial Nova / community Filament, outside corpus) · ADMIN-5 | — (guides teach hand-built admin namespaces) · DIY · ADMIN-1 | — (scaffold your own `/admin` resources; gap by design) · ADMIN-4 | ModelAdmin registration + autodiscovery · OPT · ADMIN-1, ADMIN-2 |
| Changelist: columns/filters/search/facets | — | — | — | `list_display`, `list_filter` + custom filters, `search_fields`, facet counts, date hierarchy · OPT/DIY · ADMIN-3–11 |
| Changelist perf/pagination controls | — | — | — | `list_select_related`, pagination, ordering, `list_editable` · OPT · ADMIN-9, ADMIN-12–15 |
| Edit-form customization & widgets | — | — | — | fieldsets, readonly, autocomplete/raw-id/filter_horizontal, form overrides, formfield hooks · OPT/DIY · ADMIN-16–23 |
| Inline/nested editing | — | — | — | Tabular/Stacked inlines, M2M-through, generic inlines · OPT · ADMIN-24, ADMIN-25 |
| Bulk actions | — | — | — | `@admin.action`, site-wide actions, intermediate pages · OPT · ADMIN-29, ADMIN-30 |
| Admin permissions/row scoping | gate-protected dashboards · OPT · ADMIN-4 | hand-rolled admin flag · DIY · ADMIN-1 | scoped routes/live_session auth (idiom) · — · ADMIN-4 | per-action permission hooks, per-user `get_queryset()`, `staff_member_required` · OPT · ADMIN-28, ADMIN-39 |
| Admin extensibility (custom views/URLs/templates/theming) | — | — | — | `get_urls`, AdminSite subclassing, template overrides, CSS-var theming + dark mode, JS events · OPT/DIY · ADMIN-31–36 |
| Audit trail of admin edits | — | — | — | `LogEntry` history model + history view · OPT · ADMIN-37 |
| Auto-generated internal docs | — | — | — | admindocs (docstrings, reST roles, bookmarklet) · OPT · ADMIN-38 |
| Jobs/queue operations UI | Horizon: workload, metrics, failed-job retry · OPT · ADMIN-1 | Mission Control — Jobs: inspect/retry/discard · OPT · ADMIN-2 | — (Oban Web outside corpus) | — |
| Runtime/perf dashboard | Telescope `/telescope` + Pulse `/pulse` · OPT · ADMIN-2, ADMIN-3 | — | LiveDashboard in every default app · CORE · ADMIN-1 | — |
| Live process/VM inspection | — | — | Erlang `:observer` · CORE (runtime) · ADMIN-3 | — |
| Mail dev/ops UIs | — (Mailpit via Sail, P18 CONF-13) | mailer previews `/rails/mailers` + inbound Conductor · CORE · ADMIN-3, ADMIN-4 | `/dev/mailbox` Swoosh preview · CORE · ADMIN-2 | — |
| Health endpoint | `/up` (P18 CONF-10) | `/up` for load balancers · CORE · ADMIN-5 | — | — |
| Dashboard access-control pattern | `viewHorizon/viewTelescope/viewPulse` gates · OPT · ADMIN-4 | — (DIY) | dev-only mount by default · CORE · ADMIN-1 | admin login + staff flag · OPT · ADMIN-39 |

**Notable divergences.** This is the most lopsided section: Django's model admin is a 39-row flagship with no counterpart — the other three all explicitly declare domain CRUD admin a non-goal (Laravel routes it to a commercial product, Rails and Phoenix to hand-written app code). Conversely, Django has *zero* operational/runtime UI, while Laravel (Horizon/Telescope/Pulse) and Phoenix (LiveDashboard, `:observer`) are strongest exactly there; Rails sits in between with narrow single-purpose surfaces. Tier philosophy differs too: Phoenix mounts its dashboard in every generated app (CORE), Laravel's are opt-in packages, and Rails keeps its ops UIs minimal and mostly dev-oriented.

**Volt notes.**
- "Ops dashboards for the framework's own subsystems: yes; domain CRUD admin: no" is the 3-of-4 consensus — and Django's admin is widely cited as its killer feature. A Go framework must consciously pick a side; Go's static typing + struct tags could make a reflection/codegen-driven admin cheaper than in Rails/Phoenix.
- Every dashboard here needed an auth answer (gates, staff flags, dev-only mounts); design the protected-mount primitive once and reuse it for all operational UIs.
- Phoenix shows a real-time dashboard is viable as a default-on component when the framework already owns a live-update transport; that couples P21 to Volt's websocket/SSE story.
- Small dev surfaces (mailer preview, health endpoint, jobs UI) appear repeatedly across frameworks and are cheap wins independent of the big-admin decision.

---

**First-party breadth outliers.** Laravel's extra sections show it expanding first-party into *product* domains: P23 ships a provider-agnostic AI SDK, agent framework, MCP server/client, and agent-onboarding tooling (Boost) as a new pillar, and P24 ships Cashier's full Stripe/Paddle subscription-billing wrappers — territory no other framework's docs touch. Django's P22 (GeoDjango) is the opposite kind of breadth: deep specialized *infrastructure* — a complete GIS stack (spatial fields, lookups, DB functions, GEOS/GDAL bindings) maintained in contrib for decades. The signal: Laravel grows toward monetizable application features, Django toward encyclopedic built-in capability, while Rails and Phoenix keep their breadth inside the web-runtime envelope (Rails' extra depth goes to deployment, Phoenix's to the BEAM runtime itself).

---

## Table stakes

Capabilities all four frameworks provide (tier and mechanism vary; qualifications noted). This is the must-have baseline any full-stack framework — Volt included — is measured against.

**Routing & HTTP.** Parameterized route declaration with named routes and URL generation; grouping/prefixing/namespacing; composable route files or sub-routers; a route-resolution performance answer (compile or cache); route introspection (a CLI in three, programmatic in Django).

**Request layer.** A middleware pipeline with explicit ordering control and short-circuiting; a curated default middleware battery (request ID, parsers, session, CSRF, proxy-header rewriting) with a documented order; a rich request-introspection object; body parsing and multipart uploads; content negotiation; sessions over pluggable stores; signed cookies (Phoenix: session cookie only); flash messages; streaming responses/SSE; reverse-proxy header trust; exception-to-response mapping with dev error pages and overridable production error rendering.

**Views & assets.** Auto-escaping templates with an explicit raw opt-out; layouts/inheritance plus partials or components; template compilation or caching; inline/string rendering; named-fragment/partial responses (the htmx/Turbo pattern — now in all four); fingerprinted assets with a build manifest.

**Data layer.** Parameterized-by-construction queries; a composable/lazy query-building model; associations including many-to-many with join-table data; explicit eager loading plus an N+1 guardrail; transactions with after-commit hooks; single-record finders with raise-on-missing variants; bulk writes; race-free atomic updates; enum attributes; auto timestamps; batch/streaming reads for large tables; a raw-SQL escape hatch. Migrations: generated files, deterministic ordering, an applied-versions ledger, rollback, and a seeding story.

**Validation.** One error-value contract (message bag / errors object / changeset / errors dict) consumed by both HTML forms and JSON responses; typed coercion of inbound params somewhere in the stack; custom and cross-field validators; a documented uniqueness answer; error-message customization and i18n.

**Auth & security.** bcrypt/argon2-class password hashing; session login/logout with server-side revocation; route/controller guards; purpose-scoped expiring tokens; CSRF protection for browser routes; XSS-safe templating; secret-keyed signing with (in three of four) fallback-key rotation; host/proxy trust configuration.

**Mail.** A message-composition abstraction over pluggable transports; dev preview/capture backends; test capture with assertions.

**Testing.** A test harness scaffolded with the app; an in-process HTTP client with framework-aware response assertions; a DB isolation strategy; auth helpers; view/template render tests; exception-path assertions.

**CLI & DX.** A project generator; a dev server with code reload and live reload; custom app-defined commands; introspection commands; a REPL/console — CORE in all four, and the one table-stake Go cannot replicate (see P17 notes).

**Config & ops.** Layered per-environment config with typed access; env-var secrets at boot; a debug flag with production warnings; a package/app unit with a blessed boot-registration hook; multi-destination logging.

**i18n.** Translation catalogs with an in-template retrieval API and variable interpolation (pluralization documented in three; negotiation middleware in only one).

Just as telling is what is *not* table stakes: background jobs, websockets, caching, rate limiting, CORS, and file-storage abstraction each have at least one refusenik among the four. The "obvious" batteries are contested territory, not baseline.

## Differentiators

Capabilities that exist in only one or two of the four — where each framework's identity actually lives.

**Phoenix-only.**
- **LiveView** — server-rendered reactive UI over the channel transport: 30+ inventory rows (streams, uploads, live navigation, form recovery, crash remount) with no counterpart anywhere (P10).
- **Compile-time verification suite** — verified `~p` route literals, HEEx HTML/attribute validation, compile-checked Ecto queries, SQL-interpolation-as-compile-error (P1, P3, P4, P8).
- **BEAM primitives** — zero-broker PubSub and clustering, masterless CRDT Presence, supervision trees, remote production IEx, `:observer` (P10, P17, P18); the only framework with a documented delivery-semantics story (at-most-once + client outbox).
- **SQL Sandbox** — the only DB-test isolation compatible with concurrent tests (P16); **Scopes** — generated row-level authorization threaded through every context/query (P7); Mix-free release binaries with `bin/migrate` (P5, P18); the formalized compile-time/runtime config split (P18).

**Django-only.**
- **The model admin** — 39 rows of list/filter/search/inlines/actions/permissions/audit derived from model metadata; the other three explicitly declare domain CRUD admin a non-goal (P21).
- **Derived migrations** — autodetection by diffing model state, with the squash/fake/`RunPython`/historical-models apparatus around it (P5); the system-check framework (`check --deploy`, extensible tags) (P18, P19).
- **Full gettext pipeline** — extraction/compilation commands, `LocaleMiddleware` negotiation, JS catalogs, plus an entire timezone framework filed under i18n (P15).
- Generic CBVs + formsets as request-layer CRUD machinery (P2, P6); the pluggable chunked upload-handler pipeline (P13); multi-engine template abstraction (P3); a published API-stability promise; GeoDjango.

**Laravel-only.**
- **The first-party package ring** — Fortify (2FA, passkeys), Passport (OAuth2 server), Sanctum (SPA/API tokens), Socialite, Reverb, Horizon, Cashier, Scout, Octane, Pennant, plus the new AI/MCP pillar: no other framework's docs go past email login or touch these product domains (P7, P10, breadth outliers).
- **Multi-channel notifications** — one class fanned out to mail/database/broadcast/SMS/Slack; Rails and Phoenix both name this a gap (P11).
- **Job orchestration depth** — chains, batches with completion callbacks, unique/debounced jobs, encrypted payloads, driver failover, plus a code-defined scheduler with multi-server locks (P9).
- Cache tags, atomic locks/funnels, and a general rate limiter built on the cache (P12); implicit route-model binding (P1); Precognition live validation (P6); maintenance mode (P18); pgvector in the query builder (P4); the Prompts TUI + console testing (P17).

**Rails-only.**
- **The Solid trifecta** — Queue, Cache, and Cable all running on the app's own RDBMS, making the database the default infrastructure (P9, P10, P12); **Kamal + Thruster + generated Dockerfile** owning the deploy path (P18).
- **Action Mailbox** — inbound email as a framework concern, uncopied by anyone (P11); Action Text rich-text (P3); job Continuations (resumable long jobs) (P9).
- Encrypted credentials in-repo (P8); strong parameters at the controller boundary (P2); russian-doll fragment caching (P12); versioned `load_defaults` upgrades (P18); `bin/ci` local CI DSL + Brakeman in generated CI (P16, P8); global ReDoS timeout (P8); engines as mountable full-stack mini-apps (P19); core optimistic locking and attribute encryption, and full composite-PK support (P4).

**Two-of-four capabilities worth noting.** Generated, owned auth code (Rails 8 + Phoenix — the newest designs converge here, P7); presence tracking (Laravel + Phoenix, P10); predictive/live validation (Laravel Precognition + Phoenix LiveView, P6); ops dashboards for the framework's own subsystems (Laravel's Horizon/Telescope/Pulse + Phoenix's LiveDashboard, P21); HTML-over-the-wire as default interactivity (Rails Hotwire + Phoenix LiveView, P3); no-Node asset pipelines (Rails importmaps + Phoenix-wrapped esbuild, P3); direct-to-cloud presigned uploads (all but Django, P13).

## Universal gaps

What none of the four ships in core — the open opportunities for a new framework.

1. **OpenAPI/schema generation, API versioning, GraphQL.** The single richest collectively-marked gap (P14): all four route versioning through plain prefixes/scopes, none generates a schema, and none ships a serializer with a typed contract. A statically-typed framework could derive OpenAPI from handler/struct types — something none of these dynamic-language cores could offer cheaply.
2. **Metrics and tracing exporters.** No Prometheus/StatsD/OpenTelemetry wire-up anywhere (P20). Phoenix gets closest (declare + aggregate + pluggable reporters) but stops at the wire format; Rails emits events and says "subscribe"; Laravel and Django name it a flat gap. All four predate or ignore OTel — OTel-native instrumentation is the clearest open lane.
3. **Replica read load-balancing.** Role/replica *switching* exists in all four data layers, but distributing reads is nobody's business — Rails marks it explicitly out of scope (P4).
4. **Real-time delivery guarantees.** At-most-once is the best documented semantics (Phoenix); message persistence and reconnect catch-up are application code everywhere; the other three don't state guarantees at all (P10).
5. **Application search.** No first-party search engine or typo-tolerant/faceted search: database FTS at best, with Laravel's Scout syncing to third-party services and Rails naming FTS a non-goal (P12).
6. **Migration safety tooling.** No drift detection, no migration linting, no zero-downtime automation — only documented recipes (Django's concurrent-index/NOT VALID patterns, Laravel's `--isolated` lock) (P5).
7. **Multi-tenancy as a product.** Fragments exist (Phoenix Scopes and query prefixes, Rails horizontal sharding, Laravel/Django nothing), but no framework ships an end-to-end tenancy story (P4, P7).
8. **Secrets beyond the environment.** Rails' encrypted-credentials file is the only in-repo answer; no framework integrates external secret managers (P8, P18).
9. **Config validation.** Only Django lints configuration at all (`check --deploy`, runtime); typed compile-time config checking is unclaimed territory (P18).

**Near-universal gaps (one framework has it).** CORS in core (Laravel only — Rails/Phoenix/Django all point at ecosystem middleware, P1/P8); server-side rate limiting (Laravel and Rails only; Phoenix marks it a gap, Django an explicit non-goal, P12); object-level authorization as a shipped framework (Laravel policies only — Django deliberately stubs its object hooks, Rails ships nothing, Phoenix's Scopes are generated per-app code, P7); locale-negotiation middleware (Django only, P15); WebSockets in core (absent from Django, externalized by Laravel, P10); a durable job queue in the default install (Rails' Solid Queue only, P9); inbound email (Rails only, P11); a domain admin (Django only, P21). Each of these is a proven-demand capability where three of four frameworks leave users to the ecosystem.

## Design tensions for Volt

Ten decisions where the four frameworks split — with the trade-offs as evidence, not verdicts.

1. **Runtime model vs goroutines.** The four answer concurrency at the runtime layer: PHP process-per-request (escaped via Octane), Ruby threads behind the GVL (escaped via tuning playbooks), BEAM processes (no escape needed), Python's dual sync/async stack (~20 inventory rows of adaptation rules — the cautionary tale of bolting async on). Go gets Octane/Puma-class concerns for free, but goroutines are not BEAM processes: no supervision, no isolation, no crash containment (P9 notes). Volt must decide how much explicit lifecycle/panic-recovery machinery replaces what Elixir gets from OTP and what the others get from process boundaries.

2. **Reflection magic vs codegen.** Rails and Laravel buy their convention density with runtime metaprogramming Go does not have; Phoenix shows the alternative — compile-time macros, verified routes/queries/templates, and generators that write owned code. Phoenix's compile-time verification is the only static-safety story among the four and the most natural fit for Go; Volt's sibling project **not-an-orm** has already staked the codegen position for the data layer. The cost side is real: generated code freezes at generation time (Phoenix documents that security fixes will not auto-apply), so a codegen-first Volt needs a regeneration/upgrade story from day one.

3. **Batteries vs ecosystem.** Laravel ships everything first-party; Rails ships a complete-but-swappable default meal; Django ships interfaces and lets the ecosystem compete (its tasks framework deliberately has no worker); Phoenix ships almost nothing outside the runtime. Go's ecosystem norm is à-la-carte — which is precisely why a curated default (P2's middleware battery, P9's durable queue) is itself a headline feature. The tension: every battery Volt ships is a maintenance commitment and an ecosystem it partially forecloses; every battery it skips re-creates the fragmentation users come to a framework to escape.

4. **Active record vs data mapper vs schema-as-truth.** Three frameworks put persistence and behavior on the model object; Ecto refuses and eliminates whole bug classes (lazy loading, callbacks, global scopes) structurally. Orthogonally, the single source of truth differs: Rails reflects from the live schema, Django derives everything (forms, admin, migrations) from the model declaration, Ecto declares schemas that queries are checked against. For Volt the question is what Go structs are: hand-authored truth, codegen output from SQL (the not-an-orm stance), or reflection targets. Each choice fixes where the Django-style "derive everything from one declaration" leverage can and cannot come from.

5. **Where validation lives.** Four genuinely different answers: the HTTP boundary (Laravel), the model on save (Rails), an explicit pipeline value between boundary and repo (Phoenix changesets), a form class that also owns rendering (Django). The convergent artifact is a single error-value contract serving both HTML and JSON — designing that struct first is the load-bearing decision (P6 notes). Phoenix's constraint-errors-as-data (DB violations translated into the same error shape, race-proof uniqueness) maps cleanly onto Go's `(T, error)` idiom; Laravel's ~100-rule string DSL is the exact trade Go would not make.

6. **Real-time as core vs bolt-on.** Four architectures: socket-in-process with channel classes (Rails), externalized broadcast driver (Laravel), socket-as-programming-model (Phoenix), refusal (Django). Goroutine-per-connection makes an in-process CORE websocket layer cheap for Volt, and topic routing + per-topic join auth is the convergent abstraction across all three implementers. But single-node is the easy 90%: Phoenix's clustering/presence has no Go equivalent, so multi-node Volt needs an explicit backplane (Redis/NATS/Postgres) — and the LiveView-scale bet (real-time as the *UI paradigm*) is a separable product decision, not a transport decision.

7. **Derived vs authored migrations.** Django derives migrations by diffing model state — and pays for it with dependency graphs, historical models, and a value-serialization apparatus the other three simply don't need; Laravel/Rails/Phoenix have developers author migrations with smart generators, and Rails adds a canonical schema snapshot so fresh DBs skip the replay. Autodetection in Go would require serializing model state without Python's introspection — heavy. Authored-with-generators plus a canonical snapshot, plus Phoenix's Mix-free `bin/migrate` release pattern (the closest analogue to Go's static-binary deploys), is the low-complexity path; derived migrations are the higher-magic, higher-DX bet.

8. **Convention density.** Rails demonstrates the ceiling (naming is the wiring, zero registration) and Django the counter-position (explicit registration, "no magic" as doctrine). Go's culture sides with Django, but the chunks show conventions are where Rails/Laravel productivity actually comes from. Codegen offers Volt a third way — *generated* convention, where the wiring is explicit in code but written by the tool — at the price of more generated surface to own and regenerate.

9. **DI container vs explicit wiring.** The sharpest disagreement in the whole comparison (P19): Laravel makes a reflection-autowired container the universal extension seam; Rails, Phoenix, and Django all reject one, with three different substitutes (Railties, plain function arguments, the app registry). Go cannot do reflection autowiring ergonomically, and three of four frameworks chose against it anyway — but every framework has *some* blessed "package registers itself at boot" contract, and Go's `init()` is too weak and implicit to be it. Volt needs an explicit registration/composition contract even if it never needs a container.

10. **Generated-owned auth — and how far batteries extend past login.** The 2024–25 convergence (Rails 8, Phoenix 1.8) is generated, editable auth code over installed libraries, which suits Go's explicitness — but it re-raises tension #2's patch problem for the most security-critical code in the app. And the table-stakes union beyond login is large (email verification, reset or magic links, sudo mode, per-device revocation, throttling, API tokens); only Laravel keeps going into 2FA, passkeys, OAuth2-server, and social login as first-party packages. Volt must pick both the delivery mechanism (generated vs packaged) and where on the Laravel–Phoenix breadth spectrum its auth story stops.
