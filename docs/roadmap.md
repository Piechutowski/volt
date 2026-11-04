# Roadmap

> [!CAUTION]
> **This project is an alpha version, and everything may change in the future.**

Status against Phoenix and Rails, one row per capability. `built` is
built and tested; `partial` is partly built; `missing` is not built,
with the step of `docs/plan.md` that builds it, or no step when it is
deferred; `not planned` is a decision. A status changes in the commit
that lands it.

| Area | Phoenix / Rails | Volt | Step |
|---|---|---|---|
| Routing: scopes, pipelines, resources, reverse URLs | yes | built | |
| Route-level data loading | plugs / before_action | built | |
| Handlers as pure functions | no | built | |
| Strong parameters | yes | built (`Cast` allow lists) | |
| Request bodies bound to changesets on controller routes | yes | built (`body:`, `only:`; a struct body too) | |
| HTML templates | HEEx / ERB | not planned (Elm and guigui are the UIs) | |
| JSON API with negotiated formats | yes | built (JSON, GOB) | |
| Generated Go client | no | built | |
| Generated Elm client | no | missing | |
| Models, CRUD, dynamic queries | Ecto / ActiveRecord | built | |
| Changesets and validation | Ecto changesets / validations | built | |
| Joins in declared queries | yes | built (`join` over relation names) | |
| Aggregates and grouping | yes | built (`count`, `sum`, `avg`, `min`, `max`, arithmetic, `group:`) | |
| Nested results (preload / include) | yes | built (`with` over a reverse relation) | |
| Many-to-many through a junction | yes | missing | |
| Migrations with a ledger and a schema check | yes | built | |
| Migration generation from drift | yes | not planned: a migration is written by hand, and the checker folds the migrations against the schema instead of guessing one from a diff | |
| Seeds | yes | built (`records`) | |
| Transactions | yes | built (as values; `rt.Atomic` over `rt.Then` for writes that use an earlier write's key) | |
| Sessions | yes | missing | |
| Password hashing | yes | missing | |
| CSRF | yes | missing | |
| Current user and authorization hooks | yes | built (scope loads; the user comes from a header until sessions land) | |
| Background jobs | Oban / ActiveJob | missing | |
| Mail | Swoosh / ActionMailer | missing | |
| Realtime push | Channels / ActionCable | partial (SSE broker) | |
| WebSockets | yes | not planned | |
| File uploads and storage | ActiveStorage | not planned | |
| Caching | yes | partial (statement cache) | |
| Internationalization | yes | not planned | |
| Structured logging per request and per statement | yes | built | |
| Metrics and tracing hooks | Telemetry / instrumentation | missing | |
| Secure headers, CORS, rate limiting | yes | missing | |
| Graceful shutdown, health endpoint | releases / Puma | missing | |
| Test helpers (temp database, server, clients) | ConnTest / fixtures | missing | |
| Code formatter | mix format / rubocop | missing | |
| Language server and editor support | yes | built | |
| Generators and scaffolds | yes | built (`volt gen` is the generator) | |
| REPL | iex / rails c | not planned | |
| Admin dashboard | LiveDashboard / gems | not planned | |
| Databases other than SQLite | yes | not planned | |
| Example applications as end-to-end tests | yes | partial (five feature examples with guigui clients: tables, migrations, scopes, pipelines, validation; the Elm example waits for step 5) | |
