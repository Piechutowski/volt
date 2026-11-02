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
| Aggregates and grouping | yes | missing | 10 |
| Nested results (preload / include) | yes | missing | 11 |
| Many-to-many through a junction | yes | missing | |
| Migrations with a ledger and a schema check | yes | built | |
| Migration generation from drift | yes | missing | |
| Seeds | yes | built (`Records`) | |
| Transactions | yes | built (as values) | |
| Sessions | yes | missing | |
| Password hashing | yes | missing | |
| CSRF | yes | missing | |
| Current user and authorization hooks | yes | missing | 12 |
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
| Example applications as end-to-end tests | yes | missing | |
