# Ruby on Rails — Feature Inventory

An exhaustive inventory of every capability documented in the official Ruby on
Rails guides, organized by the problem each feature solves. Compiled as
research input for the Volt Go web framework. Derived from the Rails **v8.1.3**
guides corpus (`rails/rails` @ `fa8f0812160665bff083a089d2bb2fc1817ea03e`,
fetched 2026-07-14, 74 markdown files). Notes cite the source guide file(s);
this documents what exists in Rails, not what Volt should build.

**Tier legend**

| Tier | Meaning |
|---|---|
| `CORE` | Ships enabled in the default `rails new` install (Action Pack, Active Record, Active Job, Action Cable, Active Storage, Action Mailer/Mailbox/Text, Active Support, Solid Queue/Cache/Cable, Propshaft, importmap, Turbo/Stimulus, Kamal, Thruster, …) |
| `OPT` | First-party, but must be generated, installed, or switched on |
| `ECO` | Third-party, but named/blessed in the official guides |
| `DIY` | The guides document a pattern; no shipped mechanism |

---

## P1 — Routing & HTTP dispatch

**Problem.** Map incoming URLs + HTTP verbs to handler code, and generate URLs
back out of the application without hardcoding paths. **Answer.** A Ruby DSL in
`config/routes.rb` centered on RESTful `resources`, which mints seven CRUD
routes plus named `_path`/`_url` helpers per resource; everything else
(constraints, nesting, mounting Rack apps) layers onto that convention.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ROUTE-1 | RESTful `resources` DSL: one line generates index/show/new/edit/create/update/destroy routes mapped to controller actions by convention | CORE | `routing.md` |
| ROUTE-2 | Named route helpers auto-generated per route (`photos_path`, `edit_photo_url(@photo)`); `_path` vs `_url` variants | CORE | `routing.md` |
| ROUTE-3 | Simple verb routes: `get`/`post`/`patch`/`put`/`delete` plus `match ... via:` for multi-verb binding | CORE | `routing.md` |
| ROUTE-4 | Singular resources (`resource :session`) — resourceful routes without `:id` or index | CORE | `routing.md`; used by the auth generator |
| ROUTE-5 | Nested resources with documented ≤1-level nesting convention; `shallow: true` / `shallow do` for shallow nesting | CORE | `routing.md` |
| ROUTE-6 | Extra RESTful routes on a resource via `member do` / `collection do` blocks and `on: :new` | CORE | `routing.md` |
| ROUTE-7 | Controller namespacing: `namespace`, `scope module:`, `scope path:`, `scope as:` to vary URL/module/helper name independently | CORE | `routing.md` |
| ROUTE-8 | Routing concerns (`concern` / `concerns:`) for reusable route fragments across resources | CORE | `routing.md` |
| ROUTE-9 | Dynamic segments bound into `params` (`/photos/:id`), static segments, automatic query-string param merging, optional `(.:format)` segment | CORE | `routing.md` |
| ROUTE-10 | Default parameters (`defaults:` / `defaults do`), `root` route (per-scope roots allowed) | CORE | `routing.md` |
| ROUTE-11 | Segment constraints (regex), `:id` constraints, HTTP-verb constraints | CORE | `routing.md` |
| ROUTE-12 | Request-based constraints (any `Request` method, e.g. `subdomain:`) and advanced constraint objects responding to `matches?`, incl. block form | CORE | `routing.md` |
| ROUTE-13 | Wildcard/glob segments (`*path`) capturing the remainder of a URL | CORE | `routing.md` |
| ROUTE-14 | Redirect routes: `redirect("...")` with `%{param}` interpolation, block form, custom status | CORE | `routing.md` |
| ROUTE-15 | Route directly to any Rack application (`match "/app" => MyRackApp`); `mount` for engines/Rack apps preserving sub-routing | CORE | `routing.md`, `rails_on_rack.md` |
| ROUTE-16 | URL generation from model objects: `url_for(@photo)`, `link_to` with records/arrays, polymorphic routing | CORE | `routing.md` |
| ROUTE-17 | `direct` custom URL helpers and `resolve` to remap polymorphic model→route resolution | CORE | `routing.md` |
| ROUTE-18 | Resource customization: `only:`/`except:`, `as:`, `param:`, `path_names:`, `controller:`, `constraints:`, custom singular via inflections | CORE | `routing.md` |
| ROUTE-19 | Parametric scopes (`scope ":account_id"`) making a segment available to all nested routes | CORE | `routing.md` |
| ROUTE-20 | Translated/localized path segments via `scope(path_names: ...)`; Unicode character routes | CORE | `routing.md` |
| ROUTE-21 | Route splitting across files: `draw(:admin)` loading `config/routes/admin.rb` | CORE | `routing.md` |
| ROUTE-22 | Route introspection: `bin/rails routes` with `-g` grep, `-c` controller filter, `--unused` detection; route helpers usable in console via `app.` | CORE | `routing.md`, `command_line.md` |
| ROUTE-23 | Routing test assertions: `assert_generates`, `assert_recognizes`, `assert_routing` | CORE | `routing.md` |
| ROUTE-24 | Built-in health check endpoint (`Rails::HealthController`, mounted at `/up` in generated apps) for load balancers/uptime monitors | CORE | `action_controller_advanced_topics.md`, `configuring.md` (`silence_healthcheck_path`) |
| ROUTE-25 | Format-based dispatch within an action: `respond_to do \|format\|` (HTML/JSON/XML/MD) | CORE | `routing.md`, `action_controller_overview.md`; see API-4 |

## P2 — Request handling: controllers & middleware

**Problem.** Give each request a place to live: parse input safely, hold
per-request state, run cross-cutting logic, and produce a response. **Answer.**
`ActionController::Base` subclasses with per-request instances, a strong
parameters allowlist at the mass-assignment boundary, declarative
`before/after/around_action` callbacks, and a fully user-editable Rack
middleware stack underneath.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CTRL-1 | Convention-based controllers: `ActionController::Base` subclass per resource, one public method per action, params/session/cookies available implicitly | CORE | `action_controller_overview.md` |
| CTRL-2 | Unified `params` object merging query string, form body, and route segments; hash/array parameter naming conventions; composite-key params | CORE | `action_controller_overview.md` |
| CTRL-3 | Automatic JSON body parsing keyed on `Content-Type`; `wrap_parameters` (ParamsWrapper) nests bare JSON under the resource root | CORE | `action_controller_overview.md`, `api_app.md` |
| CTRL-4 | Strong Parameters: `params.expect` (8.0+, raises 400 on shape mismatch), `require`/`permit`/`permit!`, nested & array permitting, `with_defaults` | CORE | `action_controller_overview.md` |
| CTRL-5 | Session API with pluggable stores: encrypted `CookieStore` (default), `CacheStore`, ActiveRecord/Memcached options; lazy loading; `reset_session` | CORE | `action_controller_overview.md` |
| CTRL-6 | Flash messages with `flash.now` and `flash.keep` semantics | CORE | `action_controller_overview.md` |
| CTRL-7 | Cookies API in three flavors: plain, `cookies.signed`, `cookies.encrypted`; `cookies.permanent`; key-rotation support | CORE | `action_controller_overview.md`, `security.md` |
| CTRL-8 | Controller callbacks: `before_action`/`after_action`/`around_action` with `only/except`, `skip_*`, `prepend_*`, block/class forms | CORE | `action_controller_overview.md` |
| CTRL-9 | `request`/`response` objects: headers, `query_parameters`/`request_parameters`/`path_parameters`, mutable response headers | CORE | `action_controller_overview.md` |
| CTRL-10 | Request variants (`request.variant = :phone`) selecting template variants per device/user-agent | CORE | `action_controller_overview.md` |
| CTRL-11 | Declarative exception handling with `rescue_from`; default 404/422/500 error templates in `public/`, format-aware dev error pages | CORE | `action_controller_advanced_topics.md` |
| CTRL-12 | HTTP Basic, Digest, and Token authentication helpers (`http_basic_authenticate_with`, `authenticate_or_request_with_http_token`) | CORE | `action_controller_advanced_topics.md` |
| CTRL-13 | File responses: `send_data`/`send_file` with disposition/type options; `Rack::Sendfile` X-Sendfile/X-Accel-Redirect offload to front-end server | CORE | `action_controller_advanced_topics.md`, `api_app.md` |
| CTRL-14 | `ActionController::Live` for streaming arbitrary data / Server-Sent Events via `response.stream` | CORE | `action_controller_advanced_topics.md` |
| CTRL-15 | Built-in declarative rate limiting: `rate_limit to:, within:, only:, with:` backed by the cache store | CORE | `sign_up_and_settings.md` (ActionController::RateLimiting) |
| CTRL-16 | Browser version gate: `allow_browser versions: :modern` (or per-browser hash), returning 406 to old browsers; default in generated apps | CORE | `action_controller_advanced_topics.md` |
| CTRL-17 | Fully configurable Rack middleware stack: `config.middleware.use/insert_before/insert_after/swap/move/delete`; `bin/rails middleware` inspection | CORE | `rails_on_rack.md` |
| CTRL-18 | Documented internal middleware stack (~20 middlewares: HostAuthorization, Static, Executor, RequestId, RemoteIp, ShowExceptions, ETag, ConditionalGet, …) | CORE | `rails_on_rack.md`, `api_app.md` |
| CTRL-19 | Executor/Reloader framework wrapping every unit of work (request, job) for code reloading and thread-safety; `Rails.application.executor.wrap` for user threads | CORE | `threading_and_code_execution.md` |
| CTRL-20 | `ActiveSupport::CurrentAttributes`: per-request isolated global state (e.g. `Current.user`), auto-reset between requests | CORE | `sign_up_and_settings.md` |
| CTRL-21 | `default_url_options` per controller/app for URL generation context (locale, host) | CORE | `action_controller_overview.md`, `i18n.md` |
| CTRL-22 | Parameter and redirect log filtering (`config.filter_parameters`, `filter_redirect`) | CORE | `action_controller_advanced_topics.md` |
| CTRL-23 | `ActionController::Metal` / `ActionController::API` slimmed base classes with à-la-carte module opt-in | CORE | `api_app.md` |

## P3 — Views, templating & frontend assets

**Problem.** Produce HTML (and other formats) from data, keep markup DRY, and
ship JS/CSS to browsers with cache-busting. **Answer.** ERB templates compiled
to Ruby methods, a deep helper library, layouts/partials, `form_with` builders
bound to models, Propshaft + importmap for no-build asset delivery, and
Hotwire (Turbo + Stimulus) as the default interactivity layer.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VIEW-1 | ERB template engine with auto-HTML-escaping; templates compiled and cached as Ruby methods | CORE | `action_view_overview.md` |
| VIEW-2 | Alternate template handlers: Builder (XML) and Jbuilder (JSON DSL, gem in default Gemfile) | CORE | `action_view_overview.md` |
| VIEW-3 | Implicit rendering: action renders `views/<controller>/<action>.<format>.erb` by convention; `render` overrides (template, inline, `plain:`, `html:`, `json:`, `xml:`, `body:`, `file:`, `markdown:` in 8.1) | CORE | `layouts_and_rendering.md`, `8_1_release_notes.md` |
| VIEW-4 | Render options: `:status` (symbols or codes), `:layout`, `:content_type`, `:formats`, `:variants`, `:locals` | CORE | `layouts_and_rendering.md` |
| VIEW-5 | `redirect_to` (incl. `redirect_back`, status control) and `head` for header-only responses | CORE | `layouts_and_rendering.md` |
| VIEW-6 | Layouts with `yield` / named `yield :sidebar` + `content_for`/`provide`; per-controller layout resolution, conditional & dynamic layouts, nested layouts | CORE | `layouts_and_rendering.md` |
| VIEW-7 | Partials: underscore naming, `locals:`, shorthand `render "form"`, `object:`/`as:`, partial layouts | CORE | `action_view_overview.md` |
| VIEW-8 | Collection rendering: `render partial: ..., collection:` / `render @products`, `_counter`/`_iteration` variables, `spacer_template:` | CORE | `action_view_overview.md` |
| VIEW-9 | Strict locals: `<%# locals: (message:, count: 0) %>` magic comment turning partial signatures into checked keyword args; `local_assigns` with pattern matching | CORE | `action_view_overview.md` |
| VIEW-10 | Asset tag helpers: `image_tag`, `javascript_include_tag`, `stylesheet_link_tag`, `audio_tag`, `video_tag`, `picture_tag`, `favicon_link_tag`, `preload_link_tag`, `auto_discovery_link_tag` | CORE | `action_view_helpers.md`, `layouts_and_rendering.md` |
| VIEW-11 | URL helpers: `link_to`, `button_to`, `mail_to`, `url_for`, `current_page?` | CORE | `action_view_helpers.md` |
| VIEW-12 | Sanitization helpers backed by rails-html-sanitizer: `sanitize` (allowlist), `sanitize_css`, `strip_tags`, `strip_links` | CORE | `action_view_helpers.md` |
| VIEW-13 | Formatting helper library: date helpers (`distance_of_time_in_words`, `time_ago_in_words`), number helpers (`number_to_currency`, `_human`, `_percentage`, `_with_delimiter`), text helpers (`truncate`, `pluralize`, `excerpt`, `word_wrap`) | CORE | `action_view_helpers.md` |
| VIEW-14 | Block/content helpers: `capture`, `content_for`, `tag` builder, `token_list`, `benchmark`, `debug`, `escape_javascript`, `atom_feed` | CORE | `action_view_helpers.md` |
| VIEW-15 | `form_with` model-bound form builder: field prefilling, record identification (auto URL + method, PATCH spoofing via `_method`), namespace support | CORE | `form_helpers.md` |
| VIEW-16 | Complete field helper suite: text/textarea/hidden/password/email/url/tel/number/range/color/search/date/time/datetime-local, checkbox, radio, `select` with option groups, `collection_select`/`collection_radio_buttons`/`collection_checkboxes`, date/time component selects, `time_zone_select`, `file_field`, `weekday_select` | CORE | `form_helpers.md` |
| VIEW-17 | Custom form builders: subclass `ActionView::Helpers::FormBuilder`, pass `builder:` or set default | CORE | `form_helpers.md` |
| VIEW-18 | Nested forms: `fields_for` + `accepts_nested_attributes_for`, `_destroy` removal, `reject_if`, indexed naming | CORE | `form_helpers.md` |
| VIEW-19 | Propshaft asset pipeline (default since 8.0): digest fingerprinting, logical-path resolution, CSS url() rewriting, `assets:precompile`/`assets:clean` | CORE | `asset_pipeline.md` |
| VIEW-20 | importmap-rails (default): pin npm packages as ESM from CDN or vendored, zero Node build step; `bin/importmap pin/audit` | CORE | `working_with_javascript_in_rails.md`, `asset_pipeline.md` |
| VIEW-21 | Build-tool escape hatches: `jsbundling-rails` (bun/esbuild/rollup/webpack), `cssbundling-rails` (tailwind/postcss/sass), `tailwindcss-rails`; `bin/dev` + Procfile.dev workflow | OPT | `asset_pipeline.md`, `working_with_javascript_in_rails.md` |
| VIEW-22 | Turbo Drive: automatic link/form interception for no-reload page navigation (morphing full-page reloads away), opt-out via `data-turbo="false"` | CORE | `working_with_javascript_in_rails.md` |
| VIEW-23 | Turbo Frames: decomposed page regions that navigate independently, lazy-loaded frames | CORE | `working_with_javascript_in_rails.md` |
| VIEW-24 | Turbo Streams: server-rendered `<turbo-stream>` mutations (append/prepend/replace/update/remove) as form responses or pushed over Action Cable | CORE | `working_with_javascript_in_rails.md` |
| VIEW-25 | Stimulus JS controller framework shipped as part of the Hotwire default | CORE | `getting_started.md` |
| VIEW-26 | HTTP-method + confirm affordances on links/buttons (`data: { turbo_method:, turbo_confirm: }`) | CORE | `working_with_javascript_in_rails.md` |
| VIEW-27 | Action Text rich text: `has_rich_text` attribute, bundled Trix WYSIWYG editor, `rich_textarea` form helper, sanitized rendering | CORE | `action_text_overview.md` |
| VIEW-28 | Action Text attachments: images via Active Storage, any model via signed GlobalID, custom attachable API, per-type render partials, N+1-avoiding scopes (`with_rich_text_*_and_embeds`) | CORE | `action_text_overview.md` |
| VIEW-29 | Template variants and localized templates (`show.html+phone.erb`, `index.es.html.erb`) | CORE | `action_controller_overview.md`, `i18n.md` |

## P4 — Data layer: models, ORM & queries

**Problem.** Persist domain objects and query them without writing SQL by
hand. **Answer.** Active Record — the archetypal active-record ORM: classes
map to tables by naming convention, attributes are reflected from the schema,
a chainable lazy relation algebra builds queries, and associations, callbacks,
and dirty tracking make model objects the center of business logic.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ORM-1 | Convention-over-configuration mapping: `Book` → `books`, `id` PK, `_id` FKs, schema-reflected attributes (no field declarations), namespaced models via `table_name_prefix` | CORE | `active_record_basics.md` |
| ORM-2 | Automatic timestamp columns (`created_at`/`updated_at`) maintained by the framework | CORE | `active_record_basics.md` |
| ORM-3 | CRUD object API: `new`/`save`/`create(!)`, `update(!)`/`update_all`, `destroy(!)`/`destroy_all`/`delete(_all)`, `reload`, `touch` | CORE | `active_record_basics.md` |
| ORM-4 | Single-record finders: `find` (multi-id, raises), `find_by(!)`, `take`, `first`/`last` with counts; 8.1 deprecates order-dependent finders without explicit order | CORE | `active_record_querying.md` |
| ORM-5 | Batch iteration: `find_each`, `find_in_batches`, `in_batches` with `batch_size`/`start`/`finish`/`order` cursor options | CORE | `active_record_querying.md` |
| ORM-6 | Condition forms: raw string, positional/named placeholder arrays, hash conditions (ranges → BETWEEN, arrays → IN, association hashes), `where.not`, `.or`, `.and` | CORE | `active_record_querying.md` |
| ORM-7 | Relation algebra: `order`, `select`/`distinct`, `limit`/`offset`, `group`/`having`; lazy evaluation and full chainability of all query methods | CORE | `active_record_querying.md` |
| ORM-8 | Condition overrides: `unscope`, `only`, `reselect`, `reorder`, `reverse_order`, `rewhere`, `regroup` | CORE | `active_record_querying.md` |
| ORM-9 | `none` null relation and `readonly` marking | CORE | `active_record_querying.md` |
| ORM-10 | Optimistic locking via `lock_version` column; pessimistic locking via `lock`/`with_lock` (SELECT FOR UPDATE) | CORE | `active_record_querying.md` |
| ORM-11 | Joins: `joins` (symbols, nested hashes, raw SQL), `left_outer_joins`, `where.associated` / `where.missing` existence predicates | CORE | `active_record_querying.md` |
| ORM-12 | Eager loading with three strategies: `includes` (adaptive), `preload` (separate queries), `eager_load` (LEFT JOIN); conditions on eager-loaded associations | CORE | `active_record_querying.md` |
| ORM-13 | N+1 guardrails: `strict_loading` per relation/model/association plus app-wide mode raising on lazy loads | CORE | `active_record_querying.md` |
| ORM-14 | Scopes: class-level `scope` with arguments and conditionals, `default_scope`, scope merging semantics, `unscoped` | CORE | `active_record_querying.md` |
| ORM-15 | `enum` attribute macro generating predicates, bang setters, and scopes, with `prefix`/`suffix` options | CORE | `active_record_querying.md` |
| ORM-16 | Find-or-build: `find_or_create_by(!)`, `find_or_initialize_by`, `create_with` | CORE | `active_record_querying.md` |
| ORM-17 | Raw-SQL escape hatches: `find_by_sql`, `select_all`, legacy dynamic finders (`find_by_email`) | CORE | `active_record_querying.md` |
| ORM-18 | Column shortcuts: `pluck` (multi-column, no object instantiation), `pick`, `ids` | CORE | `active_record_querying.md` |
| ORM-19 | Existence & aggregates: `exists?`, `any?`, `many?`, `count`, `sum`, `average`, `minimum`, `maximum`, grouped aggregate hashes | CORE | `active_record_querying.md` |
| ORM-20 | Query diagnostics: `explain` (with `analyze`/`verbose` options), `annotate` SQL comments, `optimizer_hints` | CORE | `active_record_querying.md` |
| ORM-21 | Async queries: `load_async` with configurable global/per-DB thread-pool executor | CORE | `configuring.md` (`async_query_executor`) |
| ORM-22 | Association types: `belongs_to` (required-by-default, `optional:`), `has_one`, `has_many` — each minting a full method family (build/create/ids/reload/reset…) | CORE | `association_basics.md` |
| ORM-23 | Indirect associations: `has_many :through` (join models, nesting), `has_one :through`, `has_and_belongs_to_many` | CORE | `association_basics.md` |
| ORM-24 | Polymorphic associations (`as:`/`polymorphic: true`) with type+id column pairs | CORE | `association_basics.md` |
| ORM-25 | Self-joins, explicit `inverse_of`, automatic bi-directional association awareness | CORE | `association_basics.md` |
| ORM-26 | Association options: `dependent:` (destroy/delete/nullify/restrict), `touch:`, `autosave:`, `validate:`, class/FK overrides, association-level scopes (blocks with full query DSL) | CORE | `association_basics.md` |
| ORM-27 | Counter caches (`counter_cache:`) incl. custom column and default-value backfill workflow | CORE | `association_basics.md`, `wishlists.md` |
| ORM-28 | Association callbacks (`before_add`, `after_remove`, …) and association extensions (module or block-defined methods on the proxy) | CORE | `association_basics.md` |
| ORM-29 | Single Table Inheritance: type column, subclass creation/querying, configurable/disable-able inheritance column | CORE | `association_basics.md` |
| ORM-30 | Delegated Types: class-per-type alternative to STI with a delegating supertype, `delegated_type` macro + `Entryable` concern pattern | CORE | `association_basics.md` |
| ORM-31 | Deprecated associations (8.1): `has_many ..., deprecated: true` reporting usage via `:warn`/`:raise`/`:notify` modes | CORE | `8_1_release_notes.md` |
| ORM-32 | Lifecycle callbacks: `before/after/around_(create\|save\|update\|destroy)`, `after_initialize`, `after_find`, `after_touch`; conditional (`:if`/`:unless`), halt via `throw :abort` | CORE | `active_record_callbacks.md` |
| ORM-33 | Transactional callbacks: `after_commit`/`after_rollback` with `on:` aliases (`after_create_commit`, `after_destroy_commit`, …), per-transaction ordering controls, `ActiveRecord.after_all_transactions_commit` | CORE | `active_record_callbacks.md` |
| ORM-34 | Database transactions with rollback-on-exception; callback suppression/ordering semantics documented around them | CORE | `active_record_callbacks.md` |
| ORM-35 | Dirty tracking (ActiveModel::Dirty): `changed?`, `attr_was`, `changes`, `saved_changes` on every model | CORE | `active_model_basics.md` |
| ORM-36 | Attributes API: `attribute :name, :type` casting/overriding, custom types, `ActiveModel::Attributes` for POROs | CORE | `active_model_basics.md` |
| ORM-37 | Active Model: the model-behavior toolkit (validations, callbacks, naming, conversion, translation, serialization) for non-DB objects, plus lint tests for custom ORMs | CORE | `active_model_basics.md` |
| ORM-38 | `has_secure_password`: bcrypt digest attribute with confirmation/challenge validations (ActiveModel::SecurePassword) | CORE | `active_model_basics.md`, `security.md` |
| ORM-39 | Attribute normalization: `normalizes :email, with: ->` applied on assignment and finders | CORE | `sign_up_and_settings.md` |
| ORM-40 | Token features: `generates_token_for` (purpose-scoped, expiring, state-invalidated tokens), signed IDs, `attr_readonly` | CORE | `getting_started.md`, `sign_up_and_settings.md`, `8_1_release_notes.md` |
| ORM-41 | Active Record Encryption: `encrypts :field` transparent application-level encryption; deterministic mode for querying, `ignore_case`, serialized attributes, compression, param filtering of encrypted attrs | CORE | `active_record_encryption.md`; requires generated keys in credentials |
| ORM-42 | Encryption key management: built-in envelope/derived key providers, custom & per-attribute key providers, key rotation lists, key-reference storage | CORE | `active_record_encryption.md` |
| ORM-43 | Encryption contexts & migration story: `support_unencrypted_data`, previous-scheme support, block-scoped contexts, protected/disabled modes | CORE | `active_record_encryption.md` |
| ORM-44 | Multiple databases: named writer/replica configs in `database.yml`, per-model `connects_to`, three-tier config | CORE | `active_record_multiple_databases.md` |
| ORM-45 | Automatic role switching middleware (write→read routing with configurable delay after writes) plus manual `connected_to(role:)` blocks and granular per-cluster switching | CORE | `active_record_multiple_databases.md` |
| ORM-46 | Horizontal sharding: `connects_to shards:`, `connected_to(shard:)`, automatic shard selection middleware with app-supplied resolver | CORE | `active_record_multiple_databases.md` |
| ORM-47 | Replica load balancing: explicitly out of scope — left to the operator/ecosystem | DIY | `active_record_multiple_databases.md` (Caveats) |
| ORM-48 | Composite primary keys: `query_constraints`/schema-derived, `find([a, b])`, associations across composite keys, composite-key route params | CORE | `active_record_composite_primary_keys.md` |
| ORM-49 | PostgreSQL-native types: bytea, array, hstore, json/jsonb, ranges, composite types, DB enums, UUID (incl. UUID primary keys), bit strings, inet/cidr, geometric, interval, full-text search patterns | CORE | `active_record_postgresql.md` |
| ORM-50 | Schema cache (dump/load) to skip runtime schema queries, per-database cache files | CORE | `active_record_multiple_databases.md`, `configuring.md` |
| ORM-51 | Connection pooling per role/shard with configurable pool size tied to thread count | CORE | `configuring.md`, `tuning_performance_for_deployment.md` |
| ORM-52 | First-class adapters: SQLite3 (production-blessed since 8.0), PostgreSQL, MySQL/Trilogy; adapter registration API for third parties | CORE | `configuring.md`, `getting_started.md` |

## P5 — Schema evolution: migrations & seeding

**Problem.** Change the database schema over time, reproducibly, across
environments and teammates. **Answer.** Timestamped Ruby migration classes
with a reversible DSL, generators that infer operations from names, a dumped
canonical schema (`schema.rb`/`structure.sql`) used to build fresh databases,
and `db/seeds.rb` for baseline data.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MIG-1 | Timestamped migration classes with a single `change` method; framework auto-reverses supported operations | CORE | `active_record_migrations.md` |
| MIG-2 | Name-aware generators: `rails g migration AddPartNumberToProducts part_number:string:index` emits the right ops; `CreateXxx`, `AddXToY`, `RemoveXFromY`, join-table patterns | CORE | `active_record_migrations.md` |
| MIG-3 | Table DSL: `create_table` (options: `id:`, `primary_key:`, composite PKs, `if_not_exists`), `change_table`, `create_join_table`, `rename_table`, `drop_table` | CORE | `active_record_migrations.md` |
| MIG-4 | Column DSL: `add/remove/rename/change_column`, `change_column_null/default`, full type set + modifiers (`limit`, `precision/scale`, `default`, `null`, `comment`, `if_not_exists`) | CORE | `active_record_migrations.md` |
| MIG-5 | `add_reference`/`belongs_to` helper creating FK column + index (+ optional `foreign_key: true`, polymorphic) | CORE | `active_record_migrations.md` |
| MIG-6 | Constraints: `add_foreign_key` (composite, custom PK targets), `add_check_constraint`, indexes (unique, multi-column, named) | CORE | `active_record_migrations.md` |
| MIG-7 | Raw SQL escape hatch: `execute` inside migrations | CORE | `active_record_migrations.md` |
| MIG-8 | Explicit reversibility: `reversible do \|dir\|`, paired `up`/`down`, `ActiveRecord::IrreversibleMigration`, `revert` of prior migrations | CORE | `active_record_migrations.md` |
| MIG-9 | Migration running: `db:migrate` (runs pending; on fresh DB loads schema first since 8.0), `db:rollback`, `db:migrate:redo`, `STEP=`, `VERSION=`, `up`/`down` for single migrations, per-environment runs | CORE | `active_record_migrations.md` |
| MIG-10 | Database lifecycle tasks: `db:create`, `db:drop`, `db:setup`, `db:reset`, `db:prepare` (idempotent create+load+migrate+seed) | CORE | `active_record_migrations.md`, `command_line.md` |
| MIG-11 | Schema dumps: `schema.rb` (Ruby DSL, columns sorted alphabetically since 8.1) or `structure.sql` (`schema_format` option); `db:schema:load` for fresh machines; source-control-friendly | CORE | `active_record_migrations.md`, `8_1_release_notes.md` |
| MIG-12 | `schema_migrations` version tracking table; migration status inspection (`db:migrate:status`, `db:version`) | CORE | `active_record_migrations.md`, `command_line.md` |
| MIG-13 | UUID primary keys via migration options + generator config | CORE | `active_record_migrations.md` |
| MIG-14 | Seed data: `db/seeds.rb` + `db:seed`, `db:seed:replant`, `db:fixtures:load` | CORE | `active_record_migrations.md`, `command_line.md` |
| MIG-15 | Data-migration guidance: keep data changes out of schema migrations (gem ecosystem or rake tasks) | DIY | `active_record_migrations.md` |
| MIG-16 | Engine migration adoption: `railties:install:migrations` copying engine migrations into the host app | CORE | `active_record_migrations.md`, `engines.md` |
| MIG-17 | Multi-database migrations: per-DB `migrations_paths`, scoped tasks (`db:migrate:primary`, `db:migrate:animals`) and per-DB generators (`--database`) | CORE | `active_record_multiple_databases.md` |
| MIG-18 | Old-migration hygiene: squash-to-schema workflow documented; migrations runnable independently of model code | CORE | `active_record_migrations.md` |

## P6 — Validation & data integrity

**Problem.** Keep bad data out of the database while giving humans actionable
error messages. **Answer.** Model-level validations as the primary line of
defense — a rich declarative validator set with an errors object wired into
forms and i18n — backed by optional database constraints in migrations.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VAL-1 | Validation lifecycle: runs on `save`/`create`/`update`; bang variants raise; `valid?`/`invalid?`; skip-validation escape hatches (`update_column`, `save(validate: false)`, …) documented | CORE | `active_record_validations.md` |
| VAL-2 | Presence/absence validators (association-aware presence) | CORE | `active_record_validations.md` |
| VAL-3 | Acceptance and confirmation validators (virtual attributes, e.g. `password_confirmation`) | CORE | `active_record_validations.md` |
| VAL-4 | Format (regex), inclusion, exclusion validators | CORE | `active_record_validations.md` |
| VAL-5 | Length (`minimum/maximum/in/is`), numericality (integer/ranges/`other_than`), comparison (attribute-vs-attribute) validators | CORE | `active_record_validations.md` |
| VAL-6 | Uniqueness validator with `scope:`, `case_sensitive:`; documented race caveat — pair with a unique DB index | CORE | `active_record_validations.md` |
| VAL-7 | Composition validators: `validates_associated`, `validates_each`, `validates_with` (validator classes over the whole record) | CORE | `active_record_validations.md` |
| VAL-8 | Common options: `:allow_nil`, `:allow_blank`, `:message` (string or proc), `:on` (create/update/custom context), `:strict` (raise instead of collect) | CORE | `active_record_validations.md` |
| VAL-9 | Conditional validations: `:if`/`:unless` (symbol/proc), `with_options` grouping, arrays of conditions | CORE | `active_record_validations.md` |
| VAL-10 | Custom validators: `ActiveModel::Validator` / `EachValidator` classes and plain `validate` methods; custom contexts via `save(context:)` | CORE | `active_record_validations.md` |
| VAL-11 | Errors object: `errors.where`, `full_messages`, `errors.add` (symbols + options), `errors[:base]`, `details`, `size`, `clear`; validator listing via `.validators` | CORE | `active_record_validations.md` |
| VAL-12 | View integration: automatic `.field_with_errors` wrapper around invalid fields (customizable via `field_error_proc`); error-list rendering pattern | CORE | `active_record_validations.md` |
| VAL-13 | I18n of error messages with model/attribute-scoped lookup chain | CORE | `active_record_validations.md`, `i18n.md` |
| VAL-14 | Database-side integrity: FKs and check constraints via migrations; framework stance that app-level validation is primary and DB constraints complementary | CORE | `active_record_migrations.md` (Active Record and Referential Integrity) |

## P7 — Authentication & authorization

**Problem.** Identify users, manage their sessions and credentials safely, and
gate what they may do. **Answer.** Since 8.0, a generated (not hidden-in-the-
framework) session-based authentication system built on first-class primitives
— `has_secure_password`, `authenticate_by`, `generates_token_for`,
`rate_limit`, `Current` — with authorization left to app code or gems.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| AUTH-1 | Authentication generator (`bin/rails generate authentication`): User + DB-backed Session models (IP/user-agent metadata), sessions & passwords controllers, `Authentication` concern, reset mailer, routes — all as editable app code | CORE | `security.md`, `8_0_release_notes.md` |
| AUTH-2 | `has_secure_password`: bcrypt hashing, `authenticate`, presence/length/confirmation validations, `password_challenge` re-auth support | CORE | `security.md`, `sign_up_and_settings.md` |
| AUTH-3 | `authenticate_by`: credential lookup hardened against timing-based account enumeration | CORE | `security.md` |
| AUTH-4 | Password reset flow: `find_by_password_reset_token!`, token-param routes, reset mailer generated | CORE | `security.md`, `getting_started.md` |
| AUTH-5 | Purpose-scoped single-use tokens: `generates_token_for :unsubscribe, expires_in:` invalidated by record state changes | CORE | `getting_started.md` |
| AUTH-6 | Login/signup throttling with controller-level `rate_limit` | CORE | `sign_up_and_settings.md` |
| AUTH-7 | Session security guidance & primitives: `reset_session` against fixation, cookie rotation, expiry patterns (DB-backed session model makes server-side revocation possible) | CORE | `security.md` |
| AUTH-8 | `Current.user`/`Current.session` request-scoped identity via CurrentAttributes | CORE | `sign_up_and_settings.md` |
| AUTH-9 | HTTP Basic/Digest/Token authentication for APIs and admin backends | CORE | `action_controller_advanced_topics.md` |
| AUTH-10 | Sign-up, email-change confirmation (unconfirmed_email + token), profile/password settings flows — documented as tutorial patterns, not shipped code | DIY | `sign_up_and_settings.md` |
| AUTH-11 | Authorization: no framework mechanism; guides document role flags (`admin` boolean, `readonly` attrs) + `before_action` guards; privilege-escalation checks by scoping queries to `Current.user` | DIY | `sign_up_and_settings.md`, `security.md` |
| AUTH-12 | Devise named as the ecosystem full-auth alternative | ECO | `security.md` |

## P8 — Security

**Problem.** Web apps face a standard threat catalog — CSRF, XSS, injection,
session attacks, secret leakage. **Answer.** Secure-by-default mechanisms
baked into the stack (auto-escaping, CSRF tokens, parameterized SQL, signed +
encrypted cookies, security headers) plus an encrypted-credentials story and a
guide-length threat manual.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| SEC-1 | CSRF protection on by default: per-session (optionally per-form) authenticity tokens auto-injected into `form_with`, verified on non-GET; `protect_from_forgery` strategies | CORE | `security.md`, `action_controller_advanced_topics.md` |
| SEC-2 | XSS defense: ERB auto-escaping of all interpolated output; `html_safe`/`raw` as explicit opt-outs; allowlist `sanitize` for user HTML | CORE | `security.md`, `action_view_helpers.md` |
| SEC-3 | SQL injection defense: parameterized conditions throughout the query API; `sanitize_sql*` helpers for raw fragments | CORE | `security.md`, `active_record_querying.md` |
| SEC-4 | Mass-assignment defense: Strong Parameters required before model assignment (`expect`/`permit`) | CORE | `action_controller_overview.md`, `security.md` |
| SEC-5 | Encrypted credentials: `config/credentials.yml.enc` + `master.key`/`RAILS_MASTER_KEY`, per-environment credential files, `credentials:edit`, CLI `credentials:fetch` (8.1) for deploy secrets | CORE | `security.md`, `command_line.md`, `8_1_release_notes.md` |
| SEC-6 | Signed & encrypted cookie infrastructure (`secret_key_base`-derived keys) with cookie rotation for config/secret changes | CORE | `action_controller_overview.md`, `security.md` |
| SEC-7 | Default security headers: `X-Frame-Options`, `X-Content-Type-Options`, `X-Permitted-Cross-Domain-Policies`, `Referrer-Policy`, etc., globally configurable | CORE | `security.md` |
| SEC-8 | Content Security Policy DSL: global + per-controller policies, automatic nonce generation, report-only mode & violation reporting | CORE | `security.md` |
| SEC-9 | TLS enforcement: `config.force_ssl` (HSTS, secure cookies, redirect), `assume_ssl` for proxy setups | CORE | `security.md`, `configuring.md` |
| SEC-10 | `ActionDispatch::HostAuthorization` middleware against DNS-rebinding/host-header attacks with allowlist + exclusions | CORE | `configuring.md`, `rails_on_rack.md` |
| SEC-11 | Open-redirect protection: cross-origin `redirect_to` raises unless `allow_other_host:` (`raise_on_open_redirects`) | CORE | `security.md`, `configuring.md` |
| SEC-12 | Session-attack countermeasures documented: fixation (`reset_session`), replay/rotation, hijack mitigation; CookieStore payload guidance | CORE | `security.md` |
| SEC-13 | ReDoS mitigation: global `Regexp.timeout = 1s` default (8.0); `\A...\z` anchoring guidance | CORE | `8_0_release_notes.md`, `security.md` |
| SEC-14 | Security scanning in the default toolchain: Brakeman static analysis, `bundler-audit`, `bin/importmap audit` wired into generated CI | ECO | `getting_started.md`, `8_1_release_notes.md`; third-party gems, shipped in default Gemfile/CI |
| SEC-15 | Sensitive-data log filtering: `filter_parameters` (deep, partial-match; encrypted attrs auto-added) and redirect filtering | CORE | `action_controller_advanced_topics.md`, `active_record_encryption.md` |
| SEC-16 | File upload/download hardening: filename sanitization, executable-upload placement, path-traversal checks — documented patterns | DIY | `security.md` |
| SEC-17 | CAPTCHA/negative-captcha and brute-force countermeasure guidance | DIY/ECO | `security.md` |
| SEC-18 | CORS: `rack-cors` middleware recommended and shown for cross-origin APIs | ECO | `security.md` |
| SEC-19 | Threat-catalog documentation: command injection, header injection/response splitting, CSS/Textile/Ajax injection, account-hijack flows — guide-level education with Rails-specific mitigations | CORE | `security.md` |

## P9 — Background work: jobs, queues & scheduling

**Problem.** Move slow or deferrable work out of the request cycle, reliably,
with retries and scheduling. **Answer.** Active Job as the backend-agnostic
declaration/enqueue API, with Solid Queue — a database-backed queue using
`FOR UPDATE SKIP LOCKED` — as the zero-extra-infrastructure default backend.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| JOB-1 | Active Job abstraction: `ApplicationJob` subclasses with `perform`, `perform_later`/`perform_now`, generator | CORE | `active_job_basics.md` |
| JOB-2 | Scheduling options: `set(wait:, wait_until:, queue:, priority:)` | CORE | `active_job_basics.md` |
| JOB-3 | Solid Queue default backend: DB-backed (PostgreSQL/MySQL/SQLite via `FOR UPDATE SKIP LOCKED`), runs as Puma plugin or dedicated `bin/jobs` process, separate `queue` database | CORE | `active_job_basics.md`, `8_0_release_notes.md` |
| JOB-4 | Worker topology config (`config/queue.yml`): dispatchers/workers, polling intervals, per-worker queue lists & thread counts, process/thread/signal lifecycle documented | CORE | `active_job_basics.md` |
| JOB-5 | Queue routing: `queue_as` (static or block), global/per-job queue-name prefixes & delimiters | CORE | `active_job_basics.md` |
| JOB-6 | Priorities: `queue_with_priority` (lower = sooner) within queues; queue order takes precedence across queues | CORE | `active_job_basics.md` |
| JOB-7 | Bulk enqueue: `perform_all_later` mapped to backend `enqueue_all` (single round-trip), with bulk callbacks semantics | CORE | `active_job_basics.md` |
| JOB-8 | Concurrency controls: `limits_concurrency to:, key:, duration:, group:` limiting concurrent jobs per key across job classes (Solid Queue extension) | CORE | `active_job_basics.md` |
| JOB-9 | Recurring tasks: cron-like `config/recurring.yml` (Fugit schedules) running job classes or commands — built-in scheduler, no OS cron | CORE | `active_job_basics.md` |
| JOB-10 | Job Continuations (8.1): `ActiveJob::Continuable` `step`/cursor API so interrupted long jobs resume from the last completed step after restarts/deploys | CORE | `active_job_basics.md`, `8_1_release_notes.md` |
| JOB-11 | Failure policy: `retry_on` (wait/backoff/jitter, attempts, dead-handler block) and `discard_on` | CORE | `active_job_basics.md` |
| JOB-12 | Job callbacks: `before/around/after_enqueue` and `_perform` | CORE | `active_job_basics.md` |
| JOB-13 | GlobalID argument serialization: pass AR records directly; deserialization errors handled/discardable | CORE | `active_job_basics.md` |
| JOB-14 | Custom argument serializers for arbitrary Ruby objects | CORE | `active_job_basics.md` |
| JOB-15 | Transactional integrity: `enqueue_after_transaction_commit` per-job opt-in; enqueue-failure hooks (`successfully_enqueued?`) | CORE | `active_job_basics.md` |
| JOB-16 | Job-aware i18n/locale and error reporting: executions wrapped in `Rails.error`; locale propagated at enqueue time | CORE | `active_job_basics.md`, `error_reporting.md` |
| JOB-17 | Pluggable backends: adapters for Sidekiq, Resque, GoodJob, Delayed Job, etc.; built-in `:async` (in-process), `:inline`, `:test` adapters | ECO/CORE | `active_job_basics.md`; Sidekiq adapter moving to the sidekiq gem (8.1) |
| JOB-18 | Mission Control — Jobs dashboard for inspecting/retrying/discarding failed jobs | OPT | `active_job_basics.md`; first-party `mission_control-jobs` gem, not installed by default (see ADMIN-2) |

## P10 — Real-time: websockets & server push

**Problem.** Push server-side events to connected browsers without polling.
**Answer.** Action Cable — full-stack WebSockets with Ruby channel classes,
cookie-shared authentication, and pluggable pub/sub, defaulting to the
database-backed Solid Cable so no Redis is required; Turbo Streams rides on
top for HTML-over-the-wire updates.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| LIVE-1 | Connection layer: one WebSocket per consumer, `identified_by` authentication (shares session cookies with the web app), connection-level `reject_unauthorized_connection` | CORE | `action_cable_overview.md` |
| LIVE-2 | Channel abstraction: `ApplicationCable::Channel` subclasses with `subscribed`/`unsubscribed`, params on subscribe, subscription rejection | CORE | `action_cable_overview.md` |
| LIVE-3 | Client library `@rails/actioncable`: `createConsumer`, subscription objects with `received`/`connected` callbacks and `perform` for client→server RPC into channel actions | CORE | `action_cable_overview.md` |
| LIVE-4 | Streams: `stream_from "room_1"` (named broadcastings) and `stream_for @post` (model-scoped) with `broadcast_to` | CORE | `action_cable_overview.md` |
| LIVE-5 | Server broadcast API from anywhere in the app (`ActionCable.server.broadcast`), incl. rebroadcast patterns | CORE | `action_cable_overview.md` |
| LIVE-6 | Pub/sub adapters: Solid Cable default (DB-backed via Active Record, tested on MySQL/SQLite/PG, retains messages ~1 day), Redis (TLS support, channel_prefix), PostgreSQL NOTIFY, async (dev), test | CORE | `action_cable_overview.md`, `8_0_release_notes.md` |
| LIVE-7 | Security: allowed request origins allowlist (regex support), disable-able for dev | CORE | `action_cable_overview.md` |
| LIVE-8 | Deployment modes: mounted in-app at `/cable` or standalone cable server; worker-pool sizing config | CORE | `action_cable_overview.md` |
| LIVE-9 | Turbo Streams delivered over Action Cable for declarative DOM updates (append/replace/remove) without hand-written channel JS | CORE | `working_with_javascript_in_rails.md` |
| LIVE-10 | Connection & channel test cases with broadcast assertions (`assert_broadcast_on`, `assert_has_stream_for`) | CORE | `testing.md`, `action_cable_overview.md` |
| LIVE-11 | Server-Sent Events alternative via `ActionController::Live` for one-way push | CORE | `action_controller_advanced_topics.md` |

## P11 — Mail & notifications

**Problem.** Send transactional email (and receive replies) with the same
conventions as the rest of the app. **Answer.** Action Mailer mirrors
controllers — mailer classes, views, layouts, previews — with Active Job
delivery; Action Mailbox routes inbound email to mailbox classes through
provider webhooks or SMTP relays.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MAIL-1 | Mailer classes mirroring controllers: `ApplicationMailer`, per-action ERB views, instance variables, generator | CORE | `action_mailer_basics.md` |
| MAIL-2 | Implicit multipart: sibling `.html.erb` + `.text.erb` templates assemble `multipart/alternative` automatically | CORE | `action_mailer_basics.md` |
| MAIL-3 | Delivery now/later: `deliver_now` and Active-Job-backed `deliver_later` | CORE | `action_mailer_basics.md`, `active_job_basics.md` |
| MAIL-4 | Parameterized mailers: `Mailer.with(user:).welcome.deliver_later` for shared before_action data | CORE | `action_mailer_basics.md` |
| MAIL-5 | Attachments (incl. inline attachments with cid URLs) | CORE | `action_mailer_basics.md` |
| MAIL-6 | Mailer URL generation via `default_url_options[:host]` (full URLs only); asset host config for images | CORE | `action_mailer_basics.md` |
| MAIL-7 | Mailer layouts, custom view paths, fragment caching in mailer views | CORE | `action_mailer_basics.md` |
| MAIL-8 | Mailer callbacks (`before_action`/`after_action`/`after_deliver`) and `rescue_from` | CORE | `action_mailer_basics.md` |
| MAIL-9 | Delivery methods: SMTP (full config, Gmail example), sendmail, file, test; dynamic per-delivery options (`delivery_method_options`); raw-body sends without templates | CORE | `action_mailer_basics.md` |
| MAIL-10 | Multiple/named recipients, CC/BCC, i18n subject lookup (`default_i18n_subject`) | CORE | `action_mailer_basics.md`, `i18n.md` |
| MAIL-11 | Mailer previews served at `/rails/mailers` in development | CORE | `action_mailer_basics.md` |
| MAIL-12 | Interceptors (mutate/block outgoing mail) and observers (post-delivery hooks) | CORE | `action_mailer_basics.md` |
| MAIL-13 | Action Mailbox: `ApplicationMailbox` routing DSL (regex/address matchers → mailbox classes) with `process` handlers | CORE | `action_mailbox_basics.md` |
| MAIL-14 | Inbound ingresses: Mailgun, Mandrill, Postmark, SendGrid webhooks plus Exim/Postfix/Qmail relay via `action_mailbox:ingress:*` commands | CORE | `action_mailbox_basics.md` |
| MAIL-15 | InboundEmail lifecycle: Active-Storage-backed raw eml, status state machine (pending→processing→delivered/failed/bounced), `bounce_with` replies, scheduled incineration after 30 days | CORE | `action_mailbox_basics.md` |
| MAIL-16 | Local inbound-email testing UI: Rails conductor at `/rails/conductor/action_mailbox/inbound_emails` | CORE | `action_mailbox_basics.md` |
| MAIL-17 | Non-email notification channels (SMS, mobile/web push, in-app feeds): no framework mechanism | DIY | gap; only email + Turbo Streams exist |

## P12 — Caching & performance

**Problem.** Serve repeated work from fast storage and squeeze throughput out
of the runtime. **Answer.** Layered caching — fragment/russian-doll caching in
views over a pluggable `Rails.cache` store — with the database-backed Solid
Cache as the default store, HTTP conditional-GET helpers, per-request SQL
caching, and documented Puma/GVL tuning.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CACHE-1 | Fragment caching: `<% cache @product do %>` keyed on model `cache_key_with_version` (auto-invalidated by `updated_at`) | CORE | `caching_with_rails.md` |
| CACHE-2 | Russian-doll caching: nested fragments + `touch: true` association cascade | CORE | `caching_with_rails.md` |
| CACHE-3 | Collection caching: `render partial: ..., collection: ..., cached: true` multi-fetch | CORE | `caching_with_rails.md` |
| CACHE-4 | Template-digest dependency tracking (view changes bust caches); explicit dependency comments; shared partial caching | CORE | `caching_with_rails.md` |
| CACHE-5 | Low-level cache API: `Rails.cache.fetch/read/write/delete/exist?` with `expires_in`, versioned keys, `race_condition_ttl` | CORE | `caching_with_rails.md` |
| CACHE-6 | Per-request SQL query cache (identical queries hit memory) | CORE | `caching_with_rails.md` |
| CACHE-7 | Solid Cache default store: database-backed FIFO cache (disk-size economics vs RAM), `config/cache.yml` (retention by `max_age`/`max_size`), optional encryption, cluster sharding | CORE | `caching_with_rails.md`, `8_0_release_notes.md` |
| CACHE-8 | Alternate stores: MemoryStore, FileStore, MemCacheStore, RedisCacheStore (with failure handling), NullStore, custom store API | CORE/ECO | `caching_with_rails.md`; Redis/Dalli clients are gems |
| CACHE-9 | Conditional GET: `stale?`/`fresh_when` (ETag + Last-Modified), strong vs weak ETags, `http_cache_forever`; `Rack::ETag`/`ConditionalGet` middleware | CORE | `caching_with_rails.md`, `api_app.md` |
| CACHE-10 | Shared HTTP caching via `Rack::Cache` backed by the Rails cache store | OPT | `api_app.md` |
| CACHE-11 | Development cache toggle: `bin/rails dev:cache` | CORE | `caching_with_rails.md`, `command_line.md` |
| CACHE-12 | Concurrency tuning playbook: Puma workers vs threads, GVL/IO trade-off analysis, latency-vs-throughput presets, `WEB_CONCURRENCY`/`RAILS_MAX_THREADS` | CORE | `tuning_performance_for_deployment.md` |
| CACHE-13 | Thruster front proxy: X-Sendfile acceleration, asset caching, compression in the default Dockerfile | CORE | `8_0_release_notes.md`, `getting_started.md` |
| CACHE-14 | HTTP asset cache headers + far-future fingerprinted assets from Propshaft | CORE | `asset_pipeline.md` |

## P13 — Files & storage

**Problem.** Accept user uploads, store them somewhere durable, transform
them, and serve them safely. **Answer.** Active Storage — attachment macros on
models, pluggable cloud services behind one API, on-the-fly image variants,
previews for non-images, and direct-from-browser uploads.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| FILE-1 | Attachment macros: `has_one_attached` / `has_many_attached` with `attach`/`attached?`, form `file_field` integration | CORE | `active_storage_overview.md` |
| FILE-2 | Three-table design: `active_storage_blobs`/`attachments`(polymorphic join)/`variant_records`; checksums & metadata on blobs | CORE | `active_storage_overview.md` |
| FILE-3 | Storage services: Disk (dev/test), S3 + S3-compatible, Google Cloud Storage, per-attachment `service:` selection (Azure removed in 8.1) | CORE | `active_storage_overview.md` |
| FILE-4 | Mirror service: replicate uploads to multiple services for zero-downtime provider migration (direct-upload compatible) | CORE | `active_storage_overview.md` |
| FILE-5 | Public vs private mode per service (`public: true` for permanent URLs vs signed short-lived URLs) | CORE | `active_storage_overview.md` |
| FILE-6 | Serving strategies: redirect mode (signed short-lived service URL), proxy mode (stream through app, CDN-friendly), and authenticated custom controllers | CORE | `active_storage_overview.md` |
| FILE-7 | Image variants: `variant(resize_to_limit: ...)` lazy transformation via image_processing (libvips default or ImageMagick), named per-attachment variants (`attachable.variant :thumb`), `preprocessed:` eager generation | CORE | `active_storage_overview.md` |
| FILE-8 | Previews for non-image files: video (ffmpeg) and PDF (poppler/muPDF) previewers, custom previewer API | CORE | `active_storage_overview.md` |
| FILE-9 | Metadata analysis by background job (dimensions, duration, etc.); custom analyzers | CORE | `active_storage_overview.md` |
| FILE-10 | Purging: `purge`/`purge_later`, replace-vs-add semantics on reattachment | CORE | `active_storage_overview.md` |
| FILE-11 | Direct uploads: browser→service presigned uploads via `direct_upload: true`, JS event lifecycle, progress tracking, CORS configuration guidance, library integration hooks | CORE | `active_storage_overview.md` |
| FILE-12 | Test support: fixture attachments (`file_fixture_upload`), automatic test-file cleanup guidance | CORE | `active_storage_overview.md`, `testing.md` |
| FILE-13 | Attachment validation (content type/size): no built-in validators; form-level patterns only | DIY | `active_storage_overview.md` (Form Validation) |

## P14 — Building APIs: serialization & content negotiation

**Problem.** Serve machine clients: negotiate formats, render JSON, keep the
middleware honest for a browserless stack. **Answer.** An `--api` mode that
slims Rails to an API core (`ActionController::API`, trimmed middleware),
`render json:` + Jbuilder for serialization, and documented à-la-carte re-add
of anything browser-ish.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| API-1 | API-only application mode: `rails new --api` → `ActionController::API` base, trimmed middleware, no views/assets generation | OPT | `api_app.md` |
| API-2 | Documented API middleware set (~19) and controller module set (~13), each individually addable/removable (`config.middleware.use/delete`, `include` modules with dependency resolution) | CORE | `api_app.md` |
| API-3 | `render json:`/`render xml:` calling `to_json`/`to_xml`; ActiveModel::Serializers::JSON (`as_json`/`serializable_hash`/`from_json`) on all models | CORE | `api_app.md`, `active_model_basics.md` |
| API-4 | Content negotiation: `respond_to`/`format` blocks, `Accept` handling, custom MIME type registration (`Mime::Type.register`) | CORE | `action_controller_overview.md` |
| API-5 | Jbuilder view templates for structured JSON responses | CORE | `action_view_overview.md`; gem in default Gemfile |
| API-6 | Markdown as a response format: `format.md` + `render markdown:` (8.1) | CORE | `8_1_release_notes.md` |
| API-7 | JSON request-body params parsed by Content-Type into `params`; `wrap_parameters` root wrapping | CORE | `api_app.md` |
| API-8 | API-friendly errors: `config.debug_exception_response_format` (`:api`), rescue_responses mapping exceptions→status codes | CORE | `api_app.md`, `configuring.md` |
| API-9 | HTTP caching for APIs: `stale?` + Rack::Cache; ETag middleware retained in API mode | CORE/OPT | `api_app.md` |
| API-10 | Token-based auth for APIs via `ActionController::HttpAuthentication::Token` | CORE | `action_controller_advanced_topics.md`, `api_app.md` |
| API-11 | Sessions/cookies/flash re-addable in API mode for browser clients (explicit middleware + module recipe) | OPT | `api_app.md` |
| API-12 | Dedicated serializer framework (attribute DSL, links, sideloading), API versioning, OpenAPI generation: none built in | DIY | gap; `as_json` overrides & Jbuilder are the documented tools |

## P15 — Internationalization & localization

**Problem.** Serve the same application in many languages with correct plural,
date, and number rules. **Answer.** The `i18n` gem wired through the whole
stack — `t`/`l` helpers with per-request locale switching, YAML locale trees,
lazy lookup, fallbacks, and model/attribute/error-message translation baked
into Active Record and Action View conventions.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| I18N-1 | I18n API throughout the stack: `I18n.t`/`I18n.l`, view/controller/mailer/model integration, `translate`/`localize` helpers | CORE | `i18n.md` |
| I18N-2 | Locale management per request: `around_action` with `I18n.with_locale`, sources documented (params, domain/subdomain, `Accept-Language`, user preference); `default_url_options` locale propagation, `scope ":locale"` routing | CORE | `i18n.md` |
| I18N-3 | Locale files: YAML/Ruby trees under `config/locales`, nested organization conventions, `I18n.load_path`, `available_locales`, `default_locale`, `raise_on_missing_translations` | CORE | `i18n.md` |
| I18N-4 | Lookup features: scoped keys, **lazy lookup** (`t(".title")` resolves per controller/action or per locale-file for mailers/views), `:default` chains, bulk lookup | CORE | `i18n.md` |
| I18N-5 | Interpolation (`%{variable}`) with strict missing/reserved-key errors | CORE | `i18n.md` |
| I18N-6 | Pluralization: CLDR-style plural categories per locale, count-keyed subtrees, custom pluralization backend rules | CORE | `i18n.md` |
| I18N-7 | Date/time/number localization: `l(date, format:)`, per-locale format definitions, localized number/currency helpers | CORE | `i18n.md` |
| I18N-8 | HTML-safe translations via `_html` key suffix (auto-escaped interpolations) | CORE | `i18n.md` |
| I18N-9 | Model translation: `Model.model_name.human`, `human_attribute_name`, error-message lookup cascade (model→attribute→default), `errors.full_messages` formatting | CORE | `i18n.md` |
| I18N-10 | Framework strings translated: helper outputs (`distance_of_time_in_words`, number helpers, date selects) all read locale data | CORE | `i18n.md` |
| I18N-11 | Localized templates (`index.es.html.erb`) resolved by current locale | CORE | `i18n.md` |
| I18N-12 | Per-locale inflection rules and mailer subject lookup | CORE | `i18n.md` |
| I18N-13 | Pluggable backend/exception handling: chained backends, KeyValue/DB backends, custom `I18n.exception_handler` | CORE | `i18n.md` |

## P16 — Testing support

**Problem.** Make automated testing the default path, from unit to full
browser. **Answer.** A complete Minitest-based harness generated with every
app: fixtures with transactional isolation, integration & system tests
(Capybara/Selenium), dedicated test cases for every framework component,
parallel execution by default, and (8.1) a local-CI DSL.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| TEST-1 | Minitest harness generated with the app: `test/` tree mirroring `app/`, `ActiveSupport::TestCase`, `test "..."` DSL, setup/teardown | CORE | `testing.md` |
| TEST-2 | `bin/rails test` runner: file/dir/line-number targeting, `-n` name filter, fail-fast, deferred output, seed control; `test:all`-style suite tasks | CORE | `testing.md` |
| TEST-3 | Rails-specific assertions: `assert_difference`, `assert_changes`, `assert_response`, `assert_redirected_to`, `assert_enqueued_with`, etc., atop full Minitest assertion set | CORE | `testing.md` |
| TEST-4 | Fixtures: YAML per model, association by label, ERB-embedded code, auto-loaded as AR objects, file-attachment fixtures | CORE | `testing.md` |
| TEST-5 | Transactional tests (rollback per test) with opt-out; automatic test-schema maintenance from schema dump | CORE | `testing.md` |
| TEST-6 | Functional tests: `ActionDispatch::IntegrationTest` verb helpers (`get/post/...`), `xhr: true`, `as: :json`, header/CGI injection, `flash`/`session`/`cookies` access | CORE | `testing.md` |
| TEST-7 | Integration tests for multi-request flows: `follow_redirect!`, `open_session` for multi-user scenarios | CORE | `testing.md` |
| TEST-8 | System tests: Capybara + Selenium (headless Chrome/Firefox options), `driven_by` config, screen-size control, failure screenshots | CORE | `testing.md`; Capybara/Selenium are bundled gems |
| TEST-9 | View/HTML testing: `rails-dom-testing` (`assert_dom`/`assert_select`), Nokogiri parsing of `response.parsed_body`, partial & helper test cases | CORE | `testing.md` |
| TEST-10 | Mailer testing: `ActionMailer::TestCase`, `:test` delivery collecting `ActionMailer::Base.deliveries`, `assert_emails`, `assert_enqueued_email_with` | CORE | `testing.md` |
| TEST-11 | Job testing: `ActiveJob::TestCase`, `assert_enqueued_jobs`/`assert_performed_jobs`, `perform_enqueued_jobs` execution control | CORE | `testing.md` |
| TEST-12 | Action Cable testing: connection tests, channel tests, broadcast assertions usable from any test | CORE | `testing.md` |
| TEST-13 | Parallel testing by default: forked processes with per-process databases (or threads), `parallelize` hooks, auto-threshold | CORE | `testing.md` |
| TEST-14 | Time-travel helpers: `travel`, `travel_to`, `freeze_time` | CORE | `testing.md` |
| TEST-15 | Local CI: `config/ci.rb` step DSL run by `bin/ci` (setup, lint, security audits, tests), optional GitHub `gh signoff` gate (8.1); generated GitHub Actions workflow | CORE | `8_1_release_notes.md`, `getting_started.md` |
| TEST-16 | Error-condition testing (`assert_raises`) and screenshot helpers documented as first-class flows | CORE | `testing.md` |

## P17 — CLI, code generation & developer experience

**Problem.** Get from zero to working feature fast, with consistent scaffolded
code and inspectable app state. **Answer.** The `bin/rails` command suite —
app/scaffold/model generators built on Thor, a REPL console loaded with the
app, runners, and app templates — all extensible and configurable.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CLI-1 | `rails new` with deep presets: `-d` database, `--api`, `--devcontainer`, `-j`/`-c` bundler choices, granular `--skip-*` flags (incl. `--skip-solid`), `--minimal` | CORE | `command_line.md`, `getting_started.md` |
| CLI-2 | Generator suite: `controller`, `model` (field:type[:index] syntax, references), `resource`, `scaffold` (full CRUD incl. tests), `migration`, `mailer`, `job`, `channel`, `authentication`; symmetric `bin/rails destroy` | CORE | `command_line.md` |
| CLI-3 | Generator configuration: `config.generators` (ORM, template engine, helpers/assets toggles), fallbacks to alternative generators (e.g. rspec) | CORE | `generators.md`, `configuring.md` |
| CLI-4 | Custom generators: Thor-based `Rails::Generators::NamedBase` classes with `source_root`/templates, hooks (`hook_for :orm`), overridable per-app generator templates in `lib/templates` | CORE | `generators.md` |
| CLI-5 | Application templates: `rails new -m template.rb` / `app:template` with a DSL (`gem`, `gem_group`, `environment`, `route`, `initializer`, `generate`, `git`, `ask`/`yes?`, `after_bundle`) | CORE | `generators.md` |
| CLI-6 | `bin/rails console`: full-app REPL (IRB) with `app` (session + route helpers) and `helper` objects, `--sandbox` rollback mode, environment switch | CORE | `command_line.md` |
| CLI-7 | `bin/rails dbconsole`: database CLI with credentials from `database.yml` (multi-DB aware) | CORE | `command_line.md` |
| CLI-8 | `bin/rails runner` for one-off scripts in app context | CORE | `command_line.md` |
| CLI-9 | `bin/rails server` (Puma), `bin/setup` bootstrap script, `bin/rails boot`, `bin/dev` process runner for bundler workflows | CORE | `command_line.md`, `asset_pipeline.md` |
| CLI-10 | Introspection commands: `routes`, `about`, `stats`, `initializers`, `middleware`, `notes` (TODO/FIXME annotation scanner with custom tags/dirs), `time:zones:all`, `tmp:*`, `secret` | CORE | `command_line.md` |
| CLI-11 | Credentials CLI: `credentials:edit` (`--environment`), `credentials:fetch` (8.1), `db:encryption:init` | CORE | `command_line.md` |
| CLI-12 | Custom rake tasks in `lib/tasks` with app environment access; `Rails::CodeStatistics.register_directory` for stats | CORE | `command_line.md`, `8_0_release_notes.md` |
| CLI-13 | Dev Containers: generated `.devcontainer` (`--devcontainer`) with services, VS Code/Codespaces workflow | OPT | `getting_started_with_devcontainer.md` |
| CLI-14 | RuboCop with `rubocop-rails-omakase` style preset in default Gemfile + CI | ECO | `getting_started.md`; third-party linter, shipped by default |
| CLI-15 | `rails-new` bootstrap tool for creating apps without a local Ruby toolchain | OPT | `getting_started_with_devcontainer.md` |

## P18 — Configuration, environments & deployment

**Problem.** Configure one codebase across dev/test/prod and ship it to
servers. **Answer.** Convention-first config (`config/application.rb`,
per-environment files, versioned `load_defaults`), Zeitwerk autoloading with
dev reload / prod eager-load, and an owned deployment path: production
Dockerfile + Kamal 2 + Thruster out of the box.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CONF-1 | Layered configuration: `config/application.rb` + `config/environments/{development,test,production}.rb`; custom environments creatable | CORE | `configuring.md` |
| CONF-2 | Versioned defaults: `config.load_defaults "8.1"` + generated `new_framework_defaults_*.rb` for incremental upgrade of behavior flags | CORE | `configuring.md`, `upgrading_ruby_on_rails.md` |
| CONF-3 | Hundreds of documented per-framework config keys under `config.<framework>.*` (active_record, action_controller, action_dispatch, active_job, …) | CORE | `configuring.md` |
| CONF-4 | Initializers: `config/initializers/*.rb`, ordered Railtie initializer chain, `before_configuration`/`before_initialize`/`to_prepare`/`after_initialize` hooks | CORE | `configuring.md` |
| CONF-5 | Lazy framework hooks: `ActiveSupport.on_load(:active_record) { ... }` to configure frameworks without forcing load order | CORE | `configuring.md`, `engines.md` |
| CONF-6 | Custom app config: `config.x.*` namespaces and `Rails.application.config_for` (env-keyed YAML with `shared:`) | CORE | `configuring.md` |
| CONF-7 | `database.yml` with ERB, env-var URL override (`DATABASE_URL`), pool sizing, multi-DB/replica/shard layout | CORE | `configuring.md`, `active_record_multiple_databases.md` |
| CONF-8 | Zeitwerk autoloading: `app/*` autoload paths, `lib` opt-in (`autoload_lib`), eager load in production, `to_prepare` reload hooks, custom inflections, `once` autoloaders, `bin/rails zeitwerk:check` | CORE | `autoloading_and_reloading_constants.md` |
| CONF-9 | Development auto-reloading via file watcher (`EventedFileUpdateChecker`) and middleware-driven reload between requests | CORE | `autoloading_and_reloading_constants.md`, `rails_on_rack.md` |
| CONF-10 | Boot process: documented `bin/rails` → boot.rb → application.rb → initializers pipeline; Bootsnap in default Gemfile | CORE | `initialization.md` |
| CONF-11 | Production-ready multi-stage Dockerfile generated with every app (Thruster + Puma entrypoint) | CORE | `getting_started.md`, `8_0_release_notes.md` |
| CONF-12 | Kamal 2 deployment: `config/deploy.yml`, `kamal setup`/`deploy`/`console`, kamal-proxy zero-downtime routing, `.kamal/secrets` (credentials-fetch integration), registry-free local-registry deploys (8.1) | CORE | `getting_started.md`, `8_0_release_notes.md`, `8_1_release_notes.md` |
| CONF-13 | Thruster in front of Puma inside the container (TLS-era static/asset serving, X-Sendfile, compression) | CORE | `8_0_release_notes.md` |
| CONF-14 | Puma as default app server; worker/thread counts via `WEB_CONCURRENCY`/`RAILS_MAX_THREADS` with documented tuning trade-offs | CORE | `tuning_performance_for_deployment.md` |
| CONF-15 | Solid-trifecta production wiring: separate `cache`/`queue`/`cable` databases in `database.yml`, `SOLID_QUEUE_IN_PUMA`, single-server SQLite deployment story | CORE | `getting_started.md`, `caching_with_rails.md`, `active_job_basics.md` |
| CONF-16 | Deploy-to-subdirectory support (`relative_url_root`); public file server toggles (`RAILS_SERVE_STATIC_FILES`) | CORE | `configuring.md` |
| CONF-17 | à-la-carte frameworks: `require "rails/all"` replaceable with individual railties to drop unused components | CORE | `configuring.md` (Avoid Loading Rails Frameworks) |
| CONF-18 | Executor-aware concurrency config: thread pools, `permit_concurrent_loads`, load-interlock semantics for user-spawned threads | CORE | `threading_and_code_execution.md` |

## P19 — Extensibility: DI, events, hooks & packages

**Problem.** Let third parties (and large apps) extend the framework without
forking it. **Answer.** Not DI — Railties. Every framework component is a
Railtie; gems hook boot via initializers and `on_load`; engines are miniature
mountable Rails apps; `ActiveSupport::Notifications` is the generic pub/sub
spine; Active Support's core extensions are the shared standard library.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| EXT-1 | `Rails::Railtie`: the base extension point — contribute initializers, config defaults, rake tasks, generators, console/runner hooks from any gem | CORE | `plugins.md`, `configuring.md` |
| EXT-2 | Engines: mountable mini-applications with their own MVC, routes, migrations, assets, generators; `isolate_namespace` for clean scoping; mounted via `mount Engine => "/path"` | CORE | `engines.md` |
| EXT-3 | Engine↔host integration: migration copying, config points for host classes (`mount_engine`-style hooks, configurable user class), URL helper proxies (`main_app.`/`engine.`) | CORE | `engines.md` |
| EXT-4 | Engine overriding: host app overrides engine views by path shadowing; models/controllers extended via `to_prepare` + class_eval/concern decorators | CORE | `engines.md` |
| EXT-5 | Plugin scaffold: `rails plugin new` (gemified plugin or `--mountable` engine) with dummy app for testing | CORE | `plugins.md` |
| EXT-6 | `ActiveSupport::Notifications.instrument/subscribe`: general-purpose in-process pub/sub usable for app events, not just metrics | CORE | `active_support_instrumentation.md` |
| EXT-7 | `ActiveSupport.on_load` lazy hooks — the blessed way for gems to extend framework classes without forcing load order | CORE | `configuring.md` (Load Hooks table) |
| EXT-8 | `ActiveSupport::Concern`: dependency-resolving module composition (`included do`, `class_methods`) — the framework's mixin idiom (e.g. tutorial `Notifications`/`Authentication` concerns) | CORE | `getting_started.md`, `active_support_core_extensions.md` |
| EXT-9 | Core extensions to Ruby itself: `blank?/present?`, `try`, `presence`, `delegate`/`delegate_missing_to`, `class_attribute`, `attr_internal`, deep dup/merge, `Enumerable#index_by/pluck`, string inflections (`pluralize`, `camelize`, `parameterize`), `squish`/`truncate` | CORE | `active_support_core_extensions.md` |
| EXT-10 | Time/date extensions: `2.days.ago`, durations, `Time.zone` / `TimeWithZone` app-level time zones, `beginning_of_week`, `travel`-friendly `Time.current` | CORE | `active_support_core_extensions.md` |
| EXT-11 | Utility objects: `HashWithIndifferentAccess`, `ActiveSupport::StringInquirer` (`Rails.env.production?`), `ArrayInquirer`, `Duration`, callbacks framework (`ActiveSupport::Callbacks`) | CORE | `active_support_core_extensions.md` |
| EXT-12 | Generator hooks for ecosystem substitution: `hook_for :orm/:template_engine/:test_framework` lets gems (RSpec, alternative ORMs) take over generated stubs | CORE | `generators.md` |
| EXT-13 | ActiveModel lint tests certifying third-party model objects work with Action Pack | CORE | `active_model_basics.md` |
| EXT-14 | Dependency injection container: none — composition via modules, Railties, and conventions | DIY | deliberate; no service container anywhere in guides |

## P20 — Observability: logging, metrics, errors

**Problem.** See what a production app is doing and be told when it breaks.
**Answer.** A logger with tagging and param filtering, a framework-wide
instrumentation bus with ~70 named events, a first-class error-reporter API
that spans requests/jobs/runners, and (8.1) structured event reporting —
with third-party APMs subscribing rather than monkey-patching.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| OBS-1 | `Rails.logger` (BroadcastLogger over `ActiveSupport::Logger`), per-env levels, custom loggers per component | CORE | `debugging_rails_applications.md`, `configuring.md` |
| OBS-2 | Tagged logging: `config.log_tags` (e.g. `:request_id`), `logger.tagged` blocks | CORE | `debugging_rails_applications.md` |
| OBS-3 | `ActiveSupport::Notifications` instrumentation: documented hook catalog across every framework (`process_action.action_controller`, `sql.active_record`, cache/job/mailer/storage events) with timing payloads; `monotonic_subscribe`; custom events | CORE | `active_support_instrumentation.md` |
| OBS-4 | Error Reporter: `Rails.error.handle/record/report/unexpected` with severity/context/source options, subscriber registration (`Rails.error.subscribe`), global context, class filtering, `disable`; all executions auto-wrapped | CORE | `error_reporting.md` |
| OBS-5 | `Rails.error.unexpected`: raises in dev/test, reports-and-continues in production (soft assertions) | CORE | `error_reporting.md` |
| OBS-6 | Structured Event Reporter (8.1): `Rails.event.notify` with tags (`Rails.event.tagged`), context, source location, pluggable `#emit` subscribers | CORE | `8_1_release_notes.md` |
| OBS-7 | SQL observability: verbose query logs (caller line per query), query log tags (marginalia-style SQL comments with controller/action/job context) | CORE | `debugging_rails_applications.md` |
| OBS-8 | Debugging tools: `debug` gem breakpoints (`binding.break`) in default Gemfile, `web-console` in-browser REPL on error pages/views | CORE | `debugging_rails_applications.md` |
| OBS-9 | Actionable errors: dev exception pages that can run remediation (e.g. pending-migration button) via `ActionDispatch::ActionableExceptions` | CORE | `rails_on_rack.md`, `api_app.md` |
| OBS-10 | View-level debug helpers (`debug`, `to_yaml`, `inspect`) and `ServerTiming` middleware exposing instrumentation in DevTools | CORE | `debugging_rails_applications.md`, `rails_on_rack.md` |
| OBS-11 | Verbose redirect logging in development (8.1, `verbose_redirect_logs`) | CORE | `8_1_release_notes.md` |
| OBS-12 | Memory-leak hunting guidance: GC profiling, `derailed_benchmarks`/`memory_profiler` | ECO | `debugging_rails_applications.md` |
| OBS-13 | Error-service integration model: Sentry/Honeybadger register reporter subscribers via Railtie (no middleware/monkey-patching) | ECO | `error_reporting.md` |
| OBS-14 | Metrics/tracing (StatsD, OpenTelemetry) exporters: none built in — instrumentation events are the raw material | DIY | gap; `active_support_instrumentation.md` |

## P21 — Admin & operational UIs

**Problem.** Operators need to inspect and manage the running system.
**Answer.** Thin, targeted dev/ops surfaces (mailer previews, inbound-mail
conductor, a jobs dashboard gem) rather than a general admin framework —
data admin UIs are app code you write.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ADMIN-1 | No general model-admin/CRUD-admin framework; guides teach building admin namespaces by hand (admin flag + namespaced controllers/layout) | DIY | `sign_up_and_settings.md` |
| ADMIN-2 | Mission Control — Jobs: first-party web UI for job queues — inspect failures, arguments, retry/requeue/discard | OPT | `active_job_basics.md` |
| ADMIN-3 | Rails Conductor: dev UI at `/rails/conductor/...` to submit and inspect inbound emails | CORE | `action_mailbox_basics.md` |
| ADMIN-4 | Mailer preview UI at `/rails/mailers` | CORE | `action_mailer_basics.md` |
| ADMIN-5 | Health endpoint `/up` for load balancers (no metrics dashboard attached) | CORE | `action_controller_advanced_topics.md` |

---

## Signature design decisions

The choices that make Rails Rails, as the guides themselves frame them:

1. **Convention over configuration, everywhere.** Naming is the wiring:
   `Book` → `books` table → `BooksController` → `app/views/books/` →
   `books_path`. Schema is reflected, not declared; routes mint helpers;
   templates are found, not registered. Configuration exists (200+ keys in
   `configuring.md`) but is never required to start. (`getting_started.md`
   states this as philosophy #1.)

2. **Omakase full-stack with swap points.** One first-party answer for every
   layer — ORM, jobs, cache, websockets, mail in/out, rich text, file storage,
   assets, deployment — shipped enabled. Every course can be sent back
   (`--skip-*` flags, adapter interfaces, `rails/all` decomposition), but the
   default meal is complete.

3. **The database is the infrastructure (the Solid trifecta).** Rails 8
   deliberately removed Redis from the default stack: Solid Queue
   (`FOR UPDATE SKIP LOCKED`), Solid Cache (disk-priced FIFO cache), and Solid
   Cable (pub/sub table) all run on the app's RDBMS — with SQLite explicitly
   blessed for single-server production. The target is one server, one
   database, no accessory fleet.

4. **The model is fat and alive.** Active Record objects carry validations,
   callbacks, dirty tracking, encryption, tokens, normalization, state
   machines-by-enum — the "active record" pattern maximized rather than
   minimized. Controllers stay thin; there is no service-layer convention.

5. **HTML over the wire.** The default frontend is server-rendered ERB
   upgraded by Hotwire (Turbo Drive/Frames/Streams + Stimulus), with importmaps
   instead of a JS build step. SPA frameworks are the escape hatch
   (jsbundling), not the path.

6. **Generated code over framework magic for auth.** Rails 8's authentication
   is a generator that writes readable app code on top of small sharp
   primitives (`has_secure_password`, `authenticate_by`,
   `generates_token_for`, `rate_limit`) — you own and edit the flow, unlike a
   Devise-style engine.

7. **One integrated testing story.** Fixtures + transactional Minitest +
   system tests + per-component test cases are generated with the app and run
   parallel by default; 8.1 extends the philosophy to CI itself (`bin/ci` — CI
   as an in-repo DSL, not YAML for someone else's cloud).

8. **Deployment is in scope.** A production Dockerfile, Kamal 2 (SSH +
   Docker + kamal-proxy), and Thruster ship with `rails new` — the framework
   claims the path all the way to a Linux box, including secrets
   (`credentials:fetch`) and zero-downtime restarts (job continuations sized
   to Kamal's 30s drain window).

9. **Extension by Railtie, not container.** There is no DI: gems integrate by
   Railtie initializers, `on_load` hooks, engines, and monkey-patch-friendly
   Ruby. Engines being "miniature applications" gives the ecosystem
   full-stack plugins (Action Text and Action Mailbox are themselves engines).

10. **Instrument first, integrate later.** `ActiveSupport::Notifications`,
    the Error Reporter, and the 8.1 Event Reporter define neutral in-process
    buses that APMs and log pipelines subscribe to — observability vendors
    plug in without middleware or patches.

## Non-goals & gaps

What Rails deliberately does not provide, or the guides acknowledge as weak:

- **Authorization.** No policy/permission framework — only tutorial-level
  `before_action` + role-flag patterns; the ecosystem (Pundit/CanCanCan) is
  not even name-checked in the corpus.
- **Dependency injection / interfaces.** No container, no contracts; explicit
  non-goal in favor of conventions and Ruby's open classes.
- **API serializer framework & versioning.** `as_json`/Jbuilder only; no
  attribute-DSL serializers, no API versioning conventions, no OpenAPI/schema
  generation.
- **GraphQL.** Entirely absent from the guides.
- **Admin UI.** No model-admin framework; admin areas are hand-built app code
  (ADMIN-1).
- **Notifications beyond email.** No SMS/push/in-app notification channel
  abstraction; Action Mailer/Mailbox are email-only.
- **Presence & realtime state.** Action Cable has no presence tracking,
  cluster registry, or client-state reconciliation primitives (contrast
  Phoenix); appearance tracking is a hand-rolled example in the guide.
- **Replica load balancing.** Multi-DB switching picks roles, but distributing
  reads across replicas is explicitly left to the operator
  (`active_record_multiple_databases.md`).
- **Attachment validation.** Active Storage ships no content-type/size
  validators (FILE-13).
- **Static typing.** No type system integration; correctness leans on tests
  and runtime checks (strict locals, `params.expect` are runtime shape
  checks).
- **Metrics export.** Instrumentation events exist, but no built-in
  StatsD/Prometheus/OTel exporters (OBS-14).
- **Full-text / search engine integration.** Nothing beyond
  PostgreSQL-specific query patterns.
- **Multitenancy, soft deletes, audit trails, feature flags.** All absent;
  horizontal sharding is the only tenancy-adjacent primitive.
- **Page & action caching.** Historically removed from core; the caching
  guide's ladder starts at fragment caching.
- **Single-language, single-process worldview.** Concurrency is
  process/thread-based behind the GVL with documented throughput/latency
  trade-offs (`tuning_performance_for_deployment.md`); there is no async/event
  runtime in the default stack.
