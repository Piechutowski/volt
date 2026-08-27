# Go Standard Library — Feature Inventory (the Volt baseline)

An exhaustive inventory of what the Go standard library, the `go` toolchain, and the
quasi-first-party `golang.org/x` modules already provide for each problem a full-stack
web framework must solve — the answer to "what does Go give you before Volt adds
anything." Derived from a local corpus (source: `go doc -all` dumps of the
installed **go1.26.5** toolchain, fetched 2026-07-14, 105 files, see `MANIFEST`).
This is an inventory of what exists — research input for the Volt framework design,
not a build plan. Section skeleton (P1–P21) is shared with the Laravel/Rails/Phoenix/Django
inventories so rows can be aligned in a comparison matrix — but unlike those documents,
here the `NO` rows are the point: each one marks a place where the standard library
stops and Volt's potential surface begins.

Tier legend:

- `STD` — importable standard-library API (`import "net/http"`), covered by the Go 1
  compatibility promise (exceptions noted inline, e.g. `GOEXPERIMENT=jsonv2`).
- `TOOL` — a language or toolchain feature rather than an importable package: goroutines,
  struct tags, `//go:embed`, `go generate`, `go test -race`.
- `X` — lives in `golang.org/x/*`: Go-team-maintained but a separate module, outside this
  corpus and outside the compatibility promise; cited from knowledge where relevant.
- `NO` — absent from std, x/ and the toolchain: third-party or DIY today, and therefore
  candidate Volt surface.

---

## P1 — Routing & HTTP dispatch

**Problem.** Map incoming URLs to handler code, extract typed parameters, and generate URLs back out. **Answer.** `http.ServeMux` — since Go 1.22 a real pattern router: `[METHOD ][HOST]/[PATH]` patterns with `{name}`, `{name...}` and `{$}` wildcards, specificity-based precedence (registration order is irrelevant; conflicts panic at registration), automatic 404/405, trailing-slash redirects, and path sanitization. Parameters come out as untyped strings via `Request.PathValue`; there are no converters, no named routes, and no reverse URL generation — the round trip back from handler to URL is entirely manual.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ROUTE-1 | `ServeMux` pattern router (`[METHOD ][HOST]/[PATH]`) | STD | net/http.txt — pattern syntax since 1.22 (rollback via `GODEBUG=httpmuxgo121=1`); literal parts match case-sensitively; per-segment unescaping (`%2F` preserved, not a separator) |
| ROUTE-2 | Method-scoped routes (`"GET /path"`) | STD | net/http.txt — since 1.22; method must match exactly, except `GET` also matches `HEAD`; no-method pattern matches every verb; `CONNECT` uses path/host unchanged |
| ROUTE-3 | Single-segment wildcards `{name}` | STD | net/http.txt — since 1.22; must be full path segments (`/b_{bucket}` invalid); name must be a Go identifier |
| ROUTE-4 | Rest-of-path wildcards `{name...}` | STD | net/http.txt — matches remainder incl. slashes, end-of-pattern only; a bare trailing `/` acts as an anonymous `...` wildcard (subtree match) |
| ROUTE-5 | Exact-end marker `{$}` | STD | net/http.txt — `"/{$}"` matches only `/`, disambiguating from the match-everything `"/"` subtree pattern |
| ROUTE-6 | Path-param extraction: `Request.PathValue`/`SetPathValue` | STD | net/http.txt — since 1.22; returns `string` only |
| ROUTE-7 | Typed path converters (int/uuid/slug) & per-param constraints | NO | PathValue is always a string; convert/validate by hand with strconv/regexp; no equivalent of Django's `<int:pk>` or converter registration |
| ROUTE-8 | Regex route patterns | NO | regexp.txt exists but is not integrated into ServeMux; no `re_path` analog |
| ROUTE-9 | Host-based routing | STD | net/http.txt — `"example.com/"` patterns; exact host only (port stripped); NO wildcard subdomains (`*.example.com` unmatched) |
| ROUTE-10 | Precedence by specificity, not registration order | STD | net/http.txt — most-specific pattern wins; ambiguous overlaps are conflicts and `Handle` panics at registration (fail-fast, unlike first-match-wins routers); host-ful beats host-less as compat exception |
| ROUTE-11 | Automatic 404 and 405 responses | STD | net/http.txt — unmatched path → "page not found"; matched path with wrong method → "method not supported" (405) handler, both synthesized by ServeMux since 1.22; `NotFoundHandler()` reusable |
| ROUTE-12 | Trailing-slash redirect | STD | net/http.txt — subtree registration `/images/` redirects `/images` → `/images/` unless the bare path is registered separately |
| ROUTE-13 | Request sanitizing / canonical redirects | STD | net/http.txt — strips port from Host, redirects `.`/`..`/`//` paths to the clean URL; escaped `%2e`/`%2f` preserved and not treated as separators |
| ROUTE-14 | `DefaultServeMux` + package-level `Handle`/`HandleFunc` | STD | net/http.txt — global mutable mux used when `Server.Handler` is nil; convenient, but shared global state (expvar/pprof self-register onto it) |
| ROUTE-15 | Mounting sub-muxes (nested routers) | STD | net/http.txt — idiom: `mux.Handle("/api/", http.StripPrefix("/api", apiMux))`; works but prefix is repeated by hand |
| ROUTE-16 | Route groups / namespaces / per-group middleware | NO | no `include()`, no app/instance namespaces, no group-scoped middleware or shared prefixes as first-class objects — every framework router's core sugar is absent |
| ROUTE-17 | Named routes + reverse URL generation | NO | nothing like `reverse()`/`url_for`; URLs are rebuilt by string concatenation or url.URL — the single biggest routing gap (breaks silently when a route moves) |
| ROUTE-18 | Route table introspection / enumeration | NO | ServeMux has no way to list registered patterns (unexported fields); no route-listing CLI possible without wrapping registration |
| ROUTE-19 | Matched-pattern introspection: `Request.Pattern` | STD | net/http.txt — since 1.23; the ServeMux pattern that matched, ideal for metrics/logging cardinality control |
| ROUTE-20 | Match-without-dispatch: `ServeMux.Handler(r)` | STD | net/http.txt — returns handler + pattern for a request without serving it (does not populate path wildcards) |
| ROUTE-21 | URL parsing & query handling | STD | net/url.txt — `Parse`/`ParseRequestURI`, `URL` struct, `Values` (multi-valued map) with `ParseQuery`/`Encode` |
| ROUTE-22 | URL construction helpers | STD | net/url.txt — `JoinPath` (1.19, safe segment joining), `ResolveReference`, `PathEscape`/`QueryEscape`, `Redacted()` (password-stripped), `String()` round-trip |
| ROUTE-23 | IP address / CIDR types for dispatch decisions | STD | net/netip.txt — since 1.18; comparable value types `Addr`/`AddrPort`/`Prefix`, `ParseAddrPort(r.RemoteAddr)`, `Prefix.Contains` for trusted-proxy / allowlist checks |
| ROUTE-24 | Redirect handlers | STD | net/http.txt — `http.Redirect(w,r,url,code)` (HTML body + Location) and `RedirectHandler` as a routable value |

## P2 — Request handling: controllers & middleware

**Problem.** Give application code a structured request/response abstraction, run cross-cutting concerns (auth, logging, limits, CSRF) around every handler, and manage the server lifecycle. **Answer.** The `http.Handler` interface is the entire contract — controllers, middleware, routers, and reverse proxies are all the same type, and middleware is nothing but a function returning a wrapped Handler. The server itself is production-grade (timeouts, HTTP/2, graceful shutdown, context plumbing, panic isolation), and 1.25 even added CSRF protection; but there is no middleware framework, no sessions, no signed cookies, and no error-page layer — every convention above the Handler interface is on you.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CTRL-1 | `Handler` interface / `HandlerFunc` adapter | STD | net/http.txt — `ServeHTTP(ResponseWriter, *Request)` is the universal controller primitive; plain funcs adapt via `HandlerFunc` |
| CTRL-2 | Middleware as Handler wrapping | STD | net/http.txt — `func(next Handler) Handler` is an idiom on the interface, not an API; composes by function nesting; no stack, ordering config, or registry |
| CTRL-3 | Built-in middleware wrappers | STD | net/http.txt — `TimeoutHandler` (503 after dt), `StripPrefix`, `MaxBytesHandler` (1.18), `AllowQuerySemicolons` (1.17), `CrossOriginProtection.Handler` (1.25) — the complete stdlib "middleware collection" |
| CTRL-4 | Middleware framework (named stack, config-driven ordering, before/after phases) | NO | nothing like Django's `MIDDLEWARE` setting or Rails' rack stack; ordering is source-code nesting order |
| CTRL-5 | Class/struct-based controllers, per-verb method dispatch | NO | no `as_view()` analog; verb dispatch is ServeMux method patterns (P1) or a manual `switch r.Method` |
| CTRL-6 | Request object | STD | net/http.txt — `Request` struct: Method, URL, Header (`Header` multi-map with canonical keys), Body, ContentLength, Host, RemoteAddr, TLS state, Trailer, ProtoAtLeast; helpers `Referer()`, `UserAgent()`, `BasicAuth()` |
| CTRL-7 | Streaming request body | STD | net/http.txt — `Body io.ReadCloser`, server auto-sends 100-continue on first read when `Expect: 100-continue`; server closes body for you |
| CTRL-8 | Request body size limits | STD | net/http.txt — `MaxBytesReader` (per-handler), `MaxBytesHandler` wrapper, typed `MaxBytesError` (1.19) for 413 mapping; header cap via `Server.MaxHeaderBytes` |
| CTRL-9 | Form parsing (urlencoded + query) | STD | net/http.txt — `ParseForm` populates `Form`/`PostForm` (url.Values); `FormValue`/`PostFormValue` shortcuts; POST/PUT/PATCH body merged with query |
| CTRL-10 | File uploads (buffered) | STD | net/http.txt, mime/multipart.txt — `ParseMultipartForm(maxMemory)` spills to temp files beyond maxMemory; `FormFile` → `multipart.File` + `FileHeader.Open`; `Form.RemoveAll` cleanup |
| CTRL-11 | File uploads (streaming) | STD | net/http.txt, mime/multipart.txt — `MultipartReader()`/`NextPart` processes parts sequentially without buffering the whole body |
| CTRL-12 | Request-scoped values | STD | context.txt, net/http.txt — `r.Context()` + `context.WithValue` with unexported key types; `WithContext`/`Clone` to derive; this is the idiom middleware uses to pass auth/user data down |
| CTRL-13 | Client-disconnect cancellation | STD | net/http.txt, context.txt — request context is canceled when the client goes away or ServeHTTP returns; propagates to DB/API calls that accept ctx |
| CTRL-14 | Per-request deadlines & control | STD | net/http.txt — `NewResponseController` (1.20): `SetReadDeadline`/`SetWriteDeadline` per request, `Flush`, `Hijack`, `EnableFullDuplex` (1.21); works through wrapped ResponseWriters via `Unwrap` |
| CTRL-15 | ResponseWriter semantics | STD | net/http.txt — implicit 200 on first Write, Content-Type sniffing via `DetectContentType`, automatic Content-Length for small bodies, HTTP trailers (Trailer header or `TrailerPrefix`), multiple 1xx informational responses |
| CTRL-16 | Response helpers | STD | net/http.txt — `Error` (sets nosniff, text/plain), `NotFound`, `Redirect`, `StatusText`, `ServeContent` (Range, If-None-Match/If-Modified-Since/If-Range; caller-set ETag honored) |
| CTRL-17 | Streaming responses / SSE | STD | net/http.txt — `Flusher`/`ResponseController.Flush`; SSE is a hand-rolled loop (set Content-Type, write `data:` frames, flush) — no helper type |
| CTRL-18 | Connection hijacking | STD | net/http.txt — `Hijacker` (HTTP/1 only) hands over the raw `net.Conn` for custom protocols |
| CTRL-19 | WebSockets | NO | not in the stdlib; `golang.org/x/net/websocket` exists but is legacy/feature-frozen (x/, not in corpus) — practically a third-party dependency (gorilla/coder) |
| CTRL-20 | Cookies: read/write | STD | net/http.txt — `Request.Cookie`/`Cookies`/`CookiesNamed` (1.23), `SetCookie`; `Cookie` struct with SameSite (1.11), Secure, HttpOnly, Partitioned/CHIPS + Quoted (1.23), `Valid()`; standalone `ParseCookie`/`ParseSetCookie` (1.23) |
| CTRL-21 | Signed / encrypted cookies | NO | no secret-key infrastructure; crypto/hmac + base64 primitives exist but the cookie-codec convention (and key rotation) is entirely DIY |
| CTRL-22 | Sessions | NO | no session middleware, no store abstraction (cookie/db/redis), no session ID lifecycle — flagship framework gap |
| CTRL-23 | Flash messages | NO | depends on sessions; absent |
| CTRL-24 | CSRF protection | STD | net/http.txt — `CrossOriginProtection` (1.25): Sec-Fetch-Site/Origin-header based, safe methods pass, trusted origins + ServeMux-pattern bypasses, `Handler` wrapper or manual `Check`; note: no token/hidden-field fallback for pre-2023 clients, no template integration |
| CTRL-25 | Server timeouts & limits | STD | net/http.txt — `Server.ReadTimeout`/`ReadHeaderTimeout` (slowloris)/`WriteTimeout`/`IdleTimeout`/`MaxHeaderBytes`; zero value Server is valid but has NO timeouts — safe defaults are on you |
| CTRL-26 | Graceful shutdown | STD | net/http.txt — `Server.Shutdown(ctx)` (1.8) stops listeners, waits for idle; `RegisterOnShutdown` hooks; `Close` for immediate teardown; hijacked conns excluded |
| CTRL-27 | Server context plumbing | STD | net/http.txt — `BaseContext`/`ConnContext` (1.13) inject per-listener/per-conn contexts; `ConnState` callback for connection lifecycle metrics |
| CTRL-28 | Panic isolation | STD | net/http.txt — server recovers a handler panic, logs stack, kills only that connection; `ErrAbortHandler` panics silently; note: NO automatic 500 error page — recovery middleware is DIY |
| CTRL-29 | Centralized error handling / error pages | NO | no error-handler registration, no debug page, no exception→status mapping; `http.Error` is a one-liner, conventions are yours |
| CTRL-30 | Request logging middleware | NO | no access-log middleware; log/slog exists (P20) but capturing status/bytes means wrapping ResponseWriter yourself |
| CTRL-31 | HTTP/2 | STD | net/http.txt — automatic over TLS (server and Transport); `HTTP2Config` tuning + `Protocols` incl. unencrypted h2c (both 1.24); `Pusher` server push (HTTP/2; of historical value — browsers dropped push) |
| CTRL-32 | Rate limiting / throttling | NO | nothing in net/http; `golang.org/x/time/rate` token bucket is the quasi-first-party answer (x/, not in corpus) |
| CTRL-33 | Response compression | NO | no gzip middleware; compress/gzip.txt provides the codec only — Accept-Encoding negotiation, Vary, and Writer-wrapping are DIY |
| CTRL-34 | Content negotiation (Accept parsing) | NO | mime.txt `ParseMediaType` parses a single media-type value; no Accept header q-value negotiation anywhere in the stdlib |
| CTRL-35 | HTTP client | STD | net/http.txt — `Client` with `Do`/`Get`/`Post`, redirect policy `CheckRedirect` (default 10 hops, sensitive headers stripped cross-domain), `Timeout` end-to-end; goroutine-safe, connection-reusing |
| CTRL-36 | Client cookie jar | STD | net/http/cookiejar.txt — in-memory RFC 6265 `Jar`; `PublicSuffixList` is an interface with NO bundled list — real use wants `golang.org/x/net/publicsuffix` (x/, not in corpus) |
| CTRL-37 | Transport (connection pool) | STD | net/http.txt — pooling/keep-alive, `ProxyFromEnvironment`, `RegisterProtocol`, TLS config, dial hooks; per-connection control via `Transport.NewClientConn`/`ClientConn` `Reserve`/`Release` (new in 1.26) |
| CTRL-38 | Reverse proxy | STD | net/http/httputil.txt — `ReverseProxy` with modern `Rewrite(*ProxyRequest)` hook (1.20; `SetURL`, `SetXForwarded`) or legacy `Director`; FlushInterval, ErrorHandler, ModifyResponse, 1xx passthrough |
| CTRL-39 | Wire-level request/response dumping | STD | net/http/httputil.txt — `DumpRequest`/`DumpRequestOut`/`DumpResponse` for debugging middleware |

## P3 — Views, templating & frontend assets

**Problem.** Render dynamic HTML without XSS, compose pages from layouts and partials, and get CSS/JS/images to the browser. **Answer.** `html/template` is the stdlib's crown jewel: parse-time *contextual* auto-escaping that understands HTML, attributes, CSS, JS, and URL contexts and rewrites every `{{.}}` pipeline with the right escaper chain — a security property most frameworks approximate with blunt output encoding. Composition exists (define/template/block, FuncMap), and `embed` + `FileServer` ship static files inside the binary; but there is no component system, no form rendering, and no asset pipeline of any kind.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VIEW-1 | `html/template` — injection-safe superset of text/template | STD | html/template.txt — same API as text/template, swaps in escaping; "template authors trusted, `Execute` data untrusted" security model |
| VIEW-2 | Contextual auto-escaping | STD | html/template.txt — parse-time rewriting: `<a href="/s?q={{.}}">{{.}}</a>` becomes `{{. \| urlescaper \| attrescaper}}` / `{{. \| htmlescaper}}`; understands HTML, CSS, JS, and URI contexts; nil pipeline → empty string |
| VIEW-3 | Attribute / namespaced / `data-*` context handling | STD | html/template.txt — `my:href` and `data-href` escape as `href`; `xmlns:*` always treated as URL context |
| VIEW-4 | `srcset` context | STD | html/template.txt — dedicated `Srcset` type and escaping context for responsive-image attributes |
| VIEW-5 | Typed trusted values (escape opt-out) | STD | html/template.txt — `HTML`, `HTMLAttr`, `CSS`, `JS`, `JSStr`, `URL`, `Srcset` mark values as pre-trusted per-context; "use of this type presents a security risk: the encapsulated content should come from a trusted source" |
| VIEW-6 | Escaping-error taxonomy | STD | html/template.txt — `Error`/`ErrorCode` classify unsafe constructs at parse/escape time (e.g. branches ending in different contexts) instead of emitting exploitable output; HTML comments stripped from output |
| VIEW-7 | Control-flow actions: `if`/`else if`/`else`, `with`/`else`, `range`/`else` | STD | text/template.txt — dot rebinding in `with`/`range`; empty-value semantics documented |
| VIEW-8 | `break` / `continue` in `range` | STD | text/template.txt — since 1.18 |
| VIEW-9 | Named templates: `define` + `template` invocation | STD | text/template.txt — `{{template "name" pipeline}}` passes explicit data (no implicit inheritance of scope); the partial mechanism |
| VIEW-10 | `block` — layout/override pattern | STD | text/template.txt — `{{block "name" .}}default{{end}}` = define+invoke; base-layout "inheritance" achieved by re-defining blocks in a later parse; a pattern, not a first-class `{% extends %}` |
| VIEW-11 | Comments + whitespace trimming | STD | text/template.txt — `{{/* */}}`; `{{-`/`-}}` trim adjacent whitespace |
| VIEW-12 | Pipelines (`\|` chaining) | STD | text/template.txt — commands chained, previous value appended as final arg; functions may return `(T, error)` and abort execution on non-nil |
| VIEW-13 | Variables | STD | text/template.txt — `$x := ...`, two-variable `range $i, $v`, `$` is the root data value |
| VIEW-14 | Argument evaluation | STD | text/template.txt — field/map/method chains `.A.B.C`, methods on dot, `call` for func-valued fields; nil-safety rules documented |
| VIEW-15 | Built-in function set | STD | text/template.txt — `and or not len index slice call print printf println urlquery html js` + comparisons `eq ne lt le gt ge` (relaxed cross-width integer compare, multi-arg `eq`) |
| VIEW-16 | Custom helpers: `FuncMap` | STD | text/template.txt, html/template.txt — `Funcs()` before parse; the extension point for date-formatting, humanize, asset-URL helpers you must write yourself |
| VIEW-17 | Parsing API | STD | text/template.txt — `New`/`Parse`/`ParseFiles`/`ParseGlob`/`Must` panic-wrapper; `Delims` for `{{`-conflicting frontends (Vue etc.) |
| VIEW-18 | `ParseFS` + embedded templates | STD | text/template.txt, html/template.txt, embed.txt — since 1.16; templates compiled into the binary via `//go:embed templates/*` — single-file deploys |
| VIEW-19 | `Option("missingkey=zero\|error")` | STD | text/template.txt — map-miss behavior control (default silently continues) |
| VIEW-20 | Template-set management | STD | text/template.txt — `Clone` (per-page layout sets), `AddParseTree`, `Lookup`, `Templates`, `DefinedTemplates`, `ExecuteTemplate(w, name, data)` |
| VIEW-21 | Streaming render to any `io.Writer` | STD | text/template.txt — `Execute(wr, data)` writes straight to the ResponseWriter; a parsed template is safe for concurrent execution |
| VIEW-22 | Manual escape helpers | STD | html/template.txt, html.txt — `HTMLEscapeString`, `JSEscapeString`, `URLQueryEscaper`; `html.EscapeString`/`UnescapeString` for non-template code |
| VIEW-23 | `text/template` for non-HTML output | STD | text/template.txt — identical engine without escaping: plaintext emails, config, code generation |
| VIEW-24 | Static assets embedded in the binary | TOOL | embed.txt — `//go:embed` directive (1.16) into `string`/`[]byte`/`embed.FS`; `all:` prefix for dot/underscore files; read-only fs.FS |
| VIEW-25 | Static file serving | STD | net/http.txt — `FileServer`/`FileServerFS` (1.22) + `http.FS`/`Dir`; serves index.html, generates directory listings (NO built-in switch to disable listings — wrap the FS yourself) |
| VIEW-26 | Single-file/conditional serving | STD | net/http.txt — `ServeFile`/`ServeFileFS` (`..` rejected), `ServeContent`: Range requests, If-Match/If-None-Match/If-Modified-Since/If-Range, caller-provided ETag, Last-Modified |
| VIEW-27 | MIME type mapping | STD | mime.txt — `TypeByExtension` (consults system mime tables + built-ins), `AddExtensionType`; net/http `DetectContentType` sniffing fallback |
| VIEW-28 | Component system (props/slots, scoped styles) | NO | define/template with explicit data is all there is — no components, no slots, no per-component CSS/JS; the modern-frontend-shaped hole Volt could fill |
| VIEW-29 | Form generation & widget rendering | NO | nothing like Django forms/Phoenix form helpers — no struct→form rendering, no error re-display, no CSRF tag (pairs with the P6 validation gap) |
| VIEW-30 | Asset pipeline (bundle/minify/fingerprint/transpile) | NO | no Sass/TS compilation, no cache-busting hashed filenames, no manifest, no `{% static %}` helper — the largest P3 gap; everyone bolts on esbuild/Vite or ships raw files |
| VIEW-31 | Precompressed / negotiated static assets | NO | FileServer ignores Accept-Encoding; compress/gzip.txt is codec-only — serving `.gz`/`.br` variants is DIY |
| VIEW-32 | Template hot-reload in development | NO | templates parse once at startup (typically embedded); no watch/reparse mode, no dev-mode distinction — restart or hand-roll re-parsing |
| VIEW-33 | View conventions (template directory layout, auto-lookup, default context) | NO | no `templates/<app>/` convention, no controller→template name inference, no context processors — every project reinvents wiring |
| VIEW-34 | Humanize/date/number template filter library | NO | built-in funcs are logic-only; time.txt formatting exists in Go code but no filter set like Django's `\|date`/`\|filesizeformat` — FuncMap DIY |

## P4 — Data layer: models, ORM & queries

**Problem.** Define domain models, map them to relational tables, and query/persist them safely and efficiently. **Answer.** Go answers only the bottom half: `database/sql` is a driver-agnostic connection-pool-and-query layer — pooling, prepared statements, transactions with isolation levels, context cancellation, NULL handling, and a `Scanner`/`Valuer` custom-type bridge — over pluggable third-party drivers (none ship in the stdlib). Everything above raw SQL strings — models, struct mapping, query building, relations, callbacks — does not exist; `database/sql/driver` defines the SPI that makes drivers swappable and wrappable, and struct tags exist as a language feature for an ORM to hang metadata on.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ORM-1 | `sql.DB` connection pool: goroutine-safe handle over zero-or-more conns, auto create/free/reuse, open once and share process-wide | STD | database/sql.txt — "safe for concurrent use by multiple goroutines"; per-conn state only observable inside a `Tx` or `Conn` |
| ORM-2 | `sql.Open(driverName, dsn)` / `sql.OpenDB(driver.Connector)` (structured config bypassing DSN strings); lazy — validates args without dialing, verify with `Ping` | STD | database/sql.txt |
| ORM-3 | Pool tuning: `SetMaxOpenConns` (default 0 = unlimited), `SetMaxIdleConns` (default 2), `SetConnMaxLifetime`, `SetConnMaxIdleTime` | STD | database/sql.txt — expired conns closed lazily before reuse |
| ORM-4 | Pool observability: `DB.Stats()` → `DBStats` (open/in-use/idle, `WaitCount`, `WaitDuration`, closed-by-cause counters) | STD | database/sql.txt |
| ORM-5 | Uniform query surface on `DB`/`Conn`/`Tx`/`Stmt`: `Exec`, `Query`, `QueryRow` + `*Context` variants; `Result.LastInsertId`/`RowsAffected` (driver-dependent) | STD | database/sql.txt |
| ORM-6 | Context cancellation: canceled ctx rolls back an open `Tx`, aborts queries mid-flight (drivers without ctx support run to completion) | STD | database/sql.txt — package doc caveat on non-ctx drivers |
| ORM-7 | Sentinel errors for control flow: `ErrNoRows` (deferred to `Row.Scan`), `ErrTxDone`, `ErrConnDone`; `Row.Err()` for wrappers | STD | database/sql.txt — `errors.Is(err, sql.ErrNoRows)` is the idiomatic "not found" |
| ORM-8 | Row iteration: `Rows.Next`/`Scan`/`Err`/`Close`; auto-close at iterator exhaustion; `Close` idempotent | STD | database/sql.txt — no `iter.Seq` adapter as of go1.26.5; manual loop |
| ORM-9 | Multiple result sets: `Rows.NextResultSet` (stored procs, batched statements) | STD | database/sql.txt; driver.txt `RowsNextResultSet` |
| ORM-10 | `Scan` conversion rules: assigns through pointers, string↔numeric with overflow checks, `time.Time` → string/[]byte as RFC3339Nano, `strconv.ParseBool` semantics for bools, cursor-valued columns scan into nested `*Rows` | STD | database/sql.txt — Rows.Scan doc |
| ORM-11 | NULL handling: `NullString`/`NullInt64`/`NullInt32`/`NullInt16`/`NullByte`/`NullFloat64`/`NullBool`/`NullTime` + generic `Null[T]` | STD | database/sql.txt — `Null[T]` since 1.22; all implement both `Scanner` and `Valuer` |
| ORM-12 | Custom column↔Go-type mapping: `sql.Scanner` (scan side) + `driver.Valuer` (write side) on user types | STD | database/sql.txt, database/sql/driver.txt — the extension point ORMs and types like UUID/decimal implement |
| ORM-13 | `RawBytes` zero-copy scan (memory owned by driver, valid until next `Next`/`Scan`/`Close`) | STD | database/sql.txt |
| ORM-14 | Prepared statements: `Prepare(Context)` on DB/Conn/Tx; DB-level `Stmt` is concurrency-safe and transparently re-prepares on new pool conns; driver may sanity-check arg counts (`NumInput`) | STD | database/sql.txt, driver.txt |
| ORM-15 | Transactions: `Begin`/`BeginTx`, `Commit`/`Rollback`; tx-prepared stmts auto-closed on commit/rollback | STD | database/sql.txt |
| ORM-16 | Isolation levels: `TxOptions{Isolation, ReadOnly}` — 8 levels (`LevelDefault` … `LevelSerializable`, `LevelLinearizable`); error if driver doesn't support requested level | STD | database/sql.txt |
| ORM-17 | `Tx.Stmt`/`Tx.StmtContext`: rebind a DB-prepared statement into a transaction | STD | database/sql.txt |
| ORM-18 | Named parameters: `sql.Named(name, value)` / `NamedArg` (driver support varies; no `?` vs `$1` unification) | STD | database/sql.txt |
| ORM-19 | Stored-procedure OUTPUT params: `sql.Out{Dest, In}` | STD | database/sql.txt — "not all drivers and databases support" |
| ORM-20 | Session pinning: `DB.Conn(ctx)` for per-connection state (temp tables, `SET` vars); `Conn.Raw(f)` exposes the raw driver conn as an escape hatch | STD | database/sql.txt |
| ORM-21 | Health checks: `Ping`/`PingContext` (establishes a conn if needed; bad conns evicted from pool) | STD | database/sql.txt, driver.txt `Pinger` |
| ORM-22 | Result-set metadata: `Rows.Columns()`, `Rows.ColumnTypes()` → `DatabaseTypeName`, `Nullable`, `ScanType`, `DecimalSize`, `Length` (all driver-optional) | STD | database/sql.txt; driver.txt `RowsColumnType*` interfaces |
| ORM-23 | Driver registry: `sql.Register` / `sql.Drivers()` — drivers self-register via blank import | STD | database/sql.txt |
| ORM-24 | Complete driver SPI: `Driver`/`DriverContext`/`Connector`, `ExecerContext`/`QueryerContext`, `ConnBeginTx`, `StmtExecContext`/`StmtQueryContext`, `NamedValueChecker`, `ValueConverter` — small interfaces, so drivers are wrappable (the standard interception point for logging/tracing/metrics wrappers) | STD | database/sql/driver.txt |
| ORM-25 | Pool lifecycle hooks for drivers: `SessionResetter` (reset session state on reuse), `Validator` (discard bad conns), `ErrBadConn` retry-on-new-conn protocol | STD | database/sql/driver.txt |
| ORM-26 | Value conversion infra: `driver.Value` (6 canonical types), `DefaultParameterConverter`, `Bool`/`Int32`/`String` converters, `Null{Converter}`/`NotNull{Converter}` | STD | database/sql/driver.txt |
| ORM-27 | Struct tags — the language substrate every Go data mapper builds `db:"col"` metadata on; stdlib assigns them no database semantics | TOOL | Language feature + `reflect.StructTag`; used by encoding/*, never by database/sql |
| ORM-28 | Database drivers | NO | Zero drivers in the stdlib — `sql.Open` is useless without a third-party import (pgx, go-sql-driver/mysql, mattn/go-sqlite3); database/sql.txt says so explicitly |
| ORM-29 | ORM / model layer: declarative models, field metadata, managers, identity map, change tracking, `save()` | NO | Nothing. Ecosystem: GORM, ent, sqlc, Bun — the core Volt P4 gap |
| ORM-30 | Struct ↔ row mapping: `Scan` is strictly positional pointer lists; no scan-by-column-name into struct fields | NO | This single absence is what sqlx/scany exist for |
| ORM-31 | Query builder / composable query DSL; queries are opaque SQL strings concatenated by hand | NO | No `Q`/`F` objects, no lazy composition, no dialect-aware SQL generation |
| ORM-32 | Relations: has-many/belongs-to/many-to-many, eager loading, N+1 prevention, cascades | NO | — |
| ORM-33 | Placeholder-dialect abstraction (`?` vs `$1` vs `@p1` vs `:name` per driver) | NO | database/sql passes the query string through verbatim; portability is the caller's problem |
| ORM-34 | Query conveniences: pagination, scopes, soft delete, optimistic locking, auto `created_at`/`updated_at` | NO | — |
| ORM-35 | Multi-DB routing / read-replica splitting / sharding | NO | One `*DB` per database; routing logic is user code |
| ORM-36 | Query logging / slow-query instrumentation | NO | No hook on `DB`; achievable only by wrapping the driver (ORM-24) or the calls |
| ORM-37 | Schema introspection (catalog queries, table listing) beyond per-result `ColumnTypes` | NO | — |

## P5 — Schema evolution: migrations & seeding

**Problem.** Evolve the database schema alongside the code, repeatably across environments, and seed data. **Answer.** It doesn't — the Go stdlib has no concept of migrations, schema versioning, or seeding; this section is nearly all NO rows. What it does supply are the raw materials every third-party migrator (goose, golang-migrate, atlas, tern) is built from: `//go:embed` to compile `.sql` files into the binary, `io/fs` for deterministic lexical ordering of version-numbered files, and `database/sql` transactions to apply DDL.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MIG-1 | Embed migration `.sql` files into the binary: `//go:embed migrations/*.sql` → `embed.FS` (single-binary deploys carry their own migrations) | TOOL | embed.txt — directive is a toolchain feature; `embed.FS` implements `fs.FS`, since 1.16 |
| MIG-2 | Deterministic migration ordering: `fs.ReadDir`/`fs.Glob` return entries sorted by filename — `0001_...sql`, `0002_...sql` order for free | STD | io/fs.txt — ReadDir "sorted by filename"; embed.txt |
| MIG-3 | Applying DDL: `DB.ExecContext`/`Tx` for transactional migrations where the database supports DDL transactions (Postgres yes, MySQL no); `driver.ResultNoRows` for DDL results | STD | database/sql.txt, database/sql/driver.txt — transactional-DDL capability is per-database, not surfaced by the API |
| MIG-4 | Migration-file checksums (dirty-migration detection): `crypto/sha256` over embedded file bytes | STD | crypto/sha256.txt — building block only; no tool consumes it |
| MIG-5 | Migration engine: version-tracking table, up/down pairs, apply/rollback, dirty-state detection | NO | Nothing — the largest single gap in this problem set; ecosystem: goose, golang-migrate |
| MIG-6 | Migration autogeneration / schema diffing from model state | NO | Nothing to diff — no model layer exists (ORM-29); ecosystem: atlas, ent |
| MIG-7 | Migration CLI (`makemigrations`/`migrate`/`showmigrations` equivalents); no `go` subcommand touches databases | NO | — |
| MIG-8 | Seeding / fixtures: no fixture loader, no seed-data format, no test-data factories | NO | Closest primitive: embed a seed `.sql` and `Exec` it yourself |
| MIG-9 | Schema dump/load (`structure.sql` / `schema.rb` equivalent) | NO | — |
| MIG-10 | Migration hygiene tooling: squashing, fake-apply, linear-history enforcement, cross-app dependency graphs | NO | — |

## P6 — Validation & data integrity

**Problem.** Validate untrusted input, surface errors back to users, and keep bad data out of the database. **Answer.** Primitives only: the stdlib is rich in strict parsers (`strconv`, `time`, `net/mail`, `net/url`, `net/netip`) and character-level checks (`regexp`, `unicode`, `utf8`) that each validate one value at a time, but there is no validation framework — no declarative rules, no field-keyed error aggregation, no request-to-struct binding, no message rendering. `errors.Join` flattens multiple errors and struct tags provide the substrate, which is exactly how the ecosystem's go-playground/validator fills the gap.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VAL-1 | Strict numeric/bool parsing: `strconv.ParseInt/ParseUint/ParseFloat/ParseBool/Atoi` with bit-size enforcement; failures return `*NumError` (op, input, `ErrSyntax`/`ErrRange`) supporting `errors.Is` | STD | strconv.txt — range checking built in; the type-coercion half of a form field pipeline |
| VAL-2 | Date/time validation: `time.Parse`/`ParseInLocation` (layout-based), `time.ParseDuration` | STD | time.txt |
| VAL-3 | Pattern validation: `regexp` — RE2 syntax, guaranteed linear-time, so safe to run on untrusted input (no ReDoS class); `MustCompile` for package-init patterns | STD | regexp.txt |
| VAL-4 | Character-class checks: `unicode.IsLetter/IsDigit/IsSpace/IsControl/IsPunct/…`, `In`/`Is` against `RangeTable`s | STD | unicode.txt |
| VAL-5 | Encoding validity & length semantics: `utf8.ValidString`/`Valid`, `utf8.RuneCountInString` (character count vs `len()` byte count — the correct "max length" check) | STD | unicode/utf8.txt |
| VAL-6 | Email validation: `mail.ParseAddress`/`ParseAddressList` (RFC 5322/6532) | STD | net/mail.txt — the honest stdlib answer to "email validator"; RFC 5322 accepts more than typical web forms want (display names, local quirks), so frameworks still layer policy on top |
| VAL-7 | URL validation: `url.Parse` (lenient — most strings parse) vs `url.ParseRequestURI` (stricter, absolute-URL request form); `url.ParseQuery` reports malformed query strings | STD | net/url.txt — lenience of `Parse` is a load-bearing caveat: parsing ≠ "is a sane http(s) URL" |
| VAL-8 | IP/CIDR validation: `netip.ParseAddr`/`ParseAddrPort`/`ParsePrefix` (comparable value types) | STD | net/netip.txt |
| VAL-9 | String normalization primitives: `strings.TrimSpace`, `Cut`, `EqualFold`, `ContainsFunc`, `Map` | STD | strings.txt — sanitize-before-validate building blocks |
| VAL-10 | Decode-time type enforcement: `encoding/json` `Unmarshal` rejects type mismatches (`*UnmarshalTypeError` with field/offset), `Decoder.DisallowUnknownFields` rejects unexpected keys | STD | encoding/json.txt — schema-shaped validation for free at the JSON boundary |
| VAL-11 | Stricter JSON v2: `RejectUnknownMembers` option, structured `SemanticError`, case-sensitive member matching by default | STD | encoding/json/v2.txt — GOEXPERIMENT=jsonv2 only, not under Go 1 compatibility promise |
| VAL-12 | Error aggregation & inspection: `errors.Join` (flat multi-error, since 1.20), `errors.Is`/`As`, generic `errors.AsType[E]` (new in 1.26) | STD | errors.txt — Join concatenates messages; it is NOT a field→errors map |
| VAL-13 | Struct tags — the substrate tag-driven validators (`validate:"required,email"`) build on; stdlib assigns them no validation semantics | TOOL | Language feature; ecosystem: go-playground/validator |
| VAL-14 | Unicode normalization (NFC/NFKC before comparing/validating user text), BCP-47 language tags | X | golang.org/x/text — x/, not in corpus |
| VAL-15 | Internationalized domain validation (IDNA/punycode) | X | golang.org/x/net/idna — x/, not in corpus |
| VAL-16 | Validation framework: declarative per-field rules (`required`, `min`, `max`, `email`, …), reusable validator objects, a `Form`/schema abstraction | NO | Nothing — Volt's P6 gap; every check above is a one-value function you compose by hand |
| VAL-17 | Field-keyed error aggregation & rendering: no `field → []message` structure, no `errors.as_json` equivalent, no non-field-errors bucket | NO | `errors.Join` (VAL-12) flattens; shaping errors for a form/API response is entirely user code |
| VAL-18 | Request binding: no request→struct binding with type coercion — `r.FormValue` returns raw strings; conversion (VAL-1/2) and assignment are manual per field | NO | net/http.txt gives `ParseForm`/`FormValue` only |
| VAL-19 | Cross-field / conditional validation pipeline (`clean()`-style hooks, ordered per-field pipelines) | NO | — |
| VAL-20 | DB-integrity validation: no uniqueness check helper, no translation of constraint violations into user-facing field errors (SQLSTATE inspection is driver-specific) | NO | database/sql surfaces the raw driver error only |
| VAL-21 | Sanitization framework & validation-message i18n/humanization | NO | html/template escapes on output (P3/P8 concern); no input-sanitizer or message-catalog layer |

## P7 — Authentication & authorization

**Problem.** Identify users and control what they may do. **Answer.** The standard library ships the entire cryptographic bedrock — CSPRNG, HMAC, constant-time comparison, modern KDFs, AEAD, Ed25519/ECDSA signatures — plus HTTP Basic auth and complete cookie plumbing. Everything above the primitives is absent: no user model, no session store, no login flow, no memory-hard password hasher (bcrypt/argon2 live in x/crypto), no permissions. Auth is the gap where Go gives you a vault full of parts and no lock.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| AUTH-1 | Cryptographically secure randomness | STD | crypto/rand.txt: `Read` fills buffer, never returns an error (crashes irrecoverably on OS failure); `Reader` for streaming |
| AUTH-2 | Secret token/session-ID generation | STD | crypto/rand.txt `Text()`: ≥128-bit base32 string, doc-blessed "when a secret string, token, password... is needed"; since 1.24 |
| AUTH-3 | HMAC (the signing primitive for cookies/tokens) | STD | crypto/hmac.txt: `New(h, key)` over any hash; `Equal` compares MACs in constant time |
| AUTH-4 | Constant-time comparison & DIT | STD | crypto/subtle.txt: `ConstantTimeCompare`, `ConstantTimeSelect`, `XORBytes`, `WithDataIndependentTiming` (since 1.24) |
| AUTH-5 | PBKDF2 password KDF | STD | crypto/pbkdf2.txt: generic `Key[Hash]`; promoted from x/crypto in 1.24. FIPS-friendly but not memory-hard |
| AUTH-6 | Memory-hard password hashing (bcrypt/scrypt/argon2) | X | x/crypto/bcrypt, x/crypto/scrypt, x/crypto/argon2 — x/, not in corpus. Nothing memory-hard in std |
| AUTH-7 | Auto-upgrading hasher framework (rehash-on-login, algorithm registry) | NO | Django's tunable hasher chain has no equivalent; Volt must own the `$argon2id$...` PHC-string lifecycle |
| AUTH-8 | HKDF (per-purpose subkey derivation) | STD | crypto/hkdf.txt: `Key`/`Extract`/`Expand` generics; promoted to std in 1.24 — right tool for deriving cookie-signing vs encryption keys from one master secret |
| AUTH-9 | SHA-2 / SHA-3 / SHAKE digests | STD | crypto/sha256.txt, sha512.txt, sha3.txt (SHA-3 + SHAKE in std since 1.24, with `hash.Cloner`); md5/sha1 present but legacy-only |
| AUTH-10 | Asymmetric signatures (API keys, SSO assertions) | STD | crypto/ed25519.txt (`Sign`/`Verify`), ecdsa.txt (`SignASN1`, `ParseRawPrivateKey` since 1.25), rsa.txt (PSS/PKCS1v15); crypto.txt `Signer`/`MessageSigner` interfaces |
| AUTH-11 | AEAD encryption (encrypted cookie/token payloads) | STD | crypto/cipher.txt: `AEAD` interface, `NewGCM`, `NewGCMWithRandomNonce` (misuse-resistant, since 1.24) over crypto/aes.txt blocks |
| AUTH-12 | HTTP Basic authentication | STD | net/http.txt: `Request.BasicAuth()` parses, `SetBasicAuth` sends; constant-time credential check is the caller's job (subtle.txt) |
| AUTH-13 | Cookie read/write | STD | net/http.txt: `SetCookie`, `Request.Cookie/Cookies/CookiesNamed`; standalone `ParseCookie`/`ParseSetCookie` since 1.23 |
| AUTH-14 | Cookie security attributes | STD | net/http.txt `Cookie` struct: `Secure`, `HttpOnly`, `SameSite` (Lax/Strict/None), `Partitioned` (CHIPS), `MaxAge`; `Cookie.Valid()` |
| AUTH-15 | Signed/encrypted cookie API (tamper-proof round-trips, key rotation, expiry) | NO | hmac + AEAD + base64 are the bricks; no Django-signing/Rails-MessageVerifier equivalent anywhere |
| AUTH-16 | Session framework (server store, ID rotation, expiry, flash) | NO | Nothing. The single largest auth gap; every Go web app hand-rolls or imports gorilla/scs-alikes |
| AUTH-17 | User model & credential storage | NO | database/sql is generic; no account schema, uniqueness, or lockout logic |
| AUTH-18 | Login/logout/password-reset flows, packaged views & forms | NO | No equivalent of `django.contrib.auth` views or one-time reset tokens |
| AUTH-19 | Pluggable authentication backends (`authenticate()` chain) | NO | Middleware-as-`http.Handler`-wrapping is the extension seam, but no contract exists |
| AUTH-20 | Permissions, groups, roles, policy objects | NO | No authorization layer of any kind |
| AUTH-21 | OAuth2 / OIDC | X | Client: golang.org/x/oauth2 — x/, not in corpus. OIDC discovery/verification and provider side: NO |
| AUTH-22 | JWT / PASETO / JOSE | NO | Not in std or x/; primitives (hmac, ed25519, base64.RawURLEncoding) only. Third-party territory |
| AUTH-23 | MFA: TOTP/HOTP, WebAuthn/passkeys | NO | hmac + sha1/sha256 suffice to implement RFC 6238, but nothing packaged |
| AUTH-24 | mTLS / client-certificate auth (service-to-service) | STD | crypto/tls.txt: `ClientAuthType` (RequireAndVerifyClientCert...), `VerifyPeerCertificate`, `VerifyConnection` hooks; chains via x509.txt |
| AUTH-25 | Token wire encoding | STD | encoding/base64.txt `URLEncoding`/`RawURLEncoding`, encoding/hex.txt — cookie- and URL-safe token serialization |

## P8 — Security

**Problem.** Blunt the standard web attack classes (XSS, SQLi, CSRF, clickjacking, host poisoning, transport downgrade). **Answer.** Unusually strong for a standard library: contextual autoescaping in html/template, parameterized queries in database/sql, header-based CSRF rejection since 1.25 (`CrossOriginProtection`), request-size/timeout hardening, and a production TLS 1.3 stack with post-quantum key exchange on by default. What's missing is the *policy* layer — security-header middleware, token CSRF for old browsers, host validation, ACME, rate limiting, secrets management — i.e. Django's `SecurityMiddleware` stack has no counterpart.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| SEC-1 | XSS: contextual output autoescaping | STD | html/template.txt: "safe against code injection"; escaping is context-aware across HTML, JS, CSS and URI positions; same API as text/template |
| SEC-2 | Manual HTML escaping | STD | html.txt `EscapeString`/`UnescapeString` for non-template paths |
| SEC-3 | SQLi: parameterized queries | STD | database/sql.txt: placeholder args on `Query`/`Exec` end-to-end; no identifier-quoting or safe-fragment helper, though |
| SEC-4 | CSRF: cross-origin request rejection | STD | net/http.txt `CrossOriginProtection` (since 1.25): rejects non-safe cross-origin requests via Sec-Fetch-Site/Origin headers; `Handler` wrapper, `AddTrustedOrigin`, `AddInsecureBypassPattern`, `SetDenyHandler`. Zero value is valid |
| SEC-5 | CSRF: token-based (per-form tokens, legacy browsers without Sec-Fetch-Site) | NO | hmac + rand are the primitives; no token mint/verify/rotate machinery |
| SEC-6 | Security-headers middleware (HSTS, X-Frame-Options, nosniff, Referrer-Policy, COOP) | NO | Each is one `w.Header().Set` — but there are no defaults and no middleware; nothing is on unless the app writes it |
| SEC-7 | CSP support (policy builder, nonce generation/injection) | NO | Django 6.0 ships it; Go has nothing beyond string headers |
| SEC-8 | Host-header validation (`ALLOWED_HOSTS`) | NO | `r.Host` is handed to the app unchecked; cache/password-reset poisoning defense is DIY |
| SEC-9 | TLS server & client | STD | crypto/tls.txt: TLS 1.0–1.3, `GetCertificate`/`GetConfigForClient` for SNI, ALPN via `NextProtos`, session tickets + `SetSessionTicketKeys` rotation, `MinVersion` |
| SEC-10 | Post-quantum key exchange | STD | crypto/tls.txt: `X25519MLKEM768` default since 1.24, `SecP256r1MLKEM768` added in 1.26; crypto/mlkem.txt implements FIPS 203 directly |
| SEC-11 | ACME / automatic certificates (Let's Encrypt) | X | Nothing in std; x/crypto/acme/autocert — x/, not in corpus. Caddy-style auto-HTTPS is a genuine framework opportunity |
| SEC-12 | Certificate parsing, chain verification, cert pools | STD | crypto/x509.txt: `ParseCertificate`, `Certificate.Verify(VerifyOptions)`, `VerifyHostname`, `CertPool`/`SystemCertPool`, name constraints |
| SEC-13 | Certificate issuance (internal CA, dev certs) | STD | crypto/x509.txt: `CreateCertificate`, `CreateCertificateRequest`, `CreateRevocationList` — a `mkcert`-style dev-TLS story is buildable in-std |
| SEC-14 | Request-body size limits | STD | net/http.txt `MaxBytesReader` / `MaxBytesHandler` (typed `*MaxBytesError`) — DoS guard for uploads and JSON bodies |
| SEC-15 | Slowloris / resource-exhaustion hardening | STD | net/http.txt `Server`: `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes` (1 MB default), `TimeoutHandler` — all zero (off) by default |
| SEC-16 | Path-traversal-safe file access | STD | net/http FileServer rejects `..` in paths (net/http.txt); `os.Root` (os.txt, since 1.24) gives traversal-resistant, symlink-safe FS operations for user-named paths |
| SEC-17 | Open-redirect protection | NO | `http.Redirect` follows whatever it's given; net/url parsing is the primitive, validation is the app's |
| SEC-18 | Rate limiting / throttling | X | x/time/rate token bucket — x/, not in corpus. No std limiter, no per-IP/per-key middleware |
| SEC-19 | CORS response handling | NO | `CrossOriginProtection` *rejects* cross-origin writes; nothing emits Access-Control-* headers for intentional sharing |
| SEC-20 | Secrets management (encrypted credentials, .env) | NO | `os.Getenv` is the entire story; no encrypted-credentials file, no masking |
| SEC-21 | Timing-attack-safe token verification | STD | crypto/subtle.txt (see AUTH-4) — present but opt-in; frameworks must remember to use it |
| SEC-22 | Vulnerability scanning of dependencies | X | govulncheck (golang.org/x/vuln) — x/, not in corpus; call-graph-aware, low-noise |
| SEC-23 | Fuzzing (parser/input hardening) | TOOL | testing.txt: native fuzzing via `testing.F`/`F.Fuzz`, seed corpus in testdata/fuzz; `go test -fuzz`; since 1.18 |
| SEC-24 | Memory safety & data-race detection | TOOL | GC'd memory-safe language + `go test -race` / `go build -race` detector — removes whole vuln classes C frameworks carry |
| SEC-25 | FIPS 140-3 compliance mode | TOOL | GOFIPS140 toolchain env + crypto/fips140 module (since 1.24) — from knowledge, not in corpus |

## P9 — Background work

**Problem.** Run work outside the request/response cycle. **Answer.** Go's answer is the language itself: goroutines, channels, `select`, `sync` and `context` form an in-process concurrency substrate richer than any framework's worker abstraction, and `time.Timer`/`time.Ticker` cover in-process timing. But it is all volatile and all in-process — there is no durable queue, no retry/backoff, no scheduler, no result store, and no worker framework: a crashed process loses every job in flight, which is exactly the layer Volt has to add.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| JOB-1 | Goroutines (`go f()`) — cheap concurrent tasks, spawn-per-job | TOOL | language feature; runtime.txt documents the scheduler; no API needed to "enqueue" in-process work |
| JOB-2 | Channels + `select` as typed in-memory work queues | TOOL | language feature; buffered channel = bounded queue with backpressure; `select` + `ctx.Done()` is the canonical worker loop (context.txt shows the pattern) |
| JOB-3 | `sync.WaitGroup` incl. `wg.Go(f)` — track/join a set of tasks | STD | sync.txt — `WaitGroup.Go` **since 1.25** (spawns + counts in one call); `Add`/`Done`/`Wait` the older form |
| JOB-4 | `context.Context` cancellation/deadline plumbing for jobs | STD | context.txt — `WithCancel`/`WithTimeout`/`WithDeadline` + `Cause` variants (`WithCancelCause` etc.); `WithoutCancel` to detach background work from a request (1.21) |
| JOB-5 | `context.AfterFunc(ctx, f)` — run callback when a context ends | STD | context.txt — since 1.21; cancellation-triggered cleanup hooks |
| JOB-6 | One-shot deferral: `time.Timer`, `time.After`, `time.AfterFunc` | STD | time.txt — `AfterFunc` runs f in its own goroutine, `Stop`/`Reset` to cancel/reschedule; since 1.23 timers are GC-recoverable and channels synchronous (no stale ticks; `GODEBUG=asynctimerchan=1` rollback) |
| JOB-7 | Recurring in-process timing: `time.Ticker` / `time.Tick` | STD | time.txt — `NewTicker`/`Reset`/`Stop`; drops ticks for slow receivers; this is the *entire* std "scheduler" |
| JOB-8 | Mutual exclusion & coordination: `Mutex`/`RWMutex`/`Once`/`OnceFunc`/`OnceValue(s)`/`Cond` | STD | sync.txt — `OnceFunc`/`OnceValue`/`OnceValues` since 1.21; docs steer higher-level sync toward channels |
| JOB-9 | Concurrent shared state: `sync.Map`; scratch reuse: `sync.Pool` | STD | sync.txt — Map for grow-only caches / disjoint key sets; Pool for GC-pressure amortization |
| JOB-10 | Lock-free counters/flags: `sync/atomic` typed values | STD | sync/atomic.txt — `Bool`/`Int64`/`Uint64`/`Pointer[T]` types + `Add`/`CompareAndSwap`/`Swap`/`And`/`Or` (And/Or since 1.23) |
| JOB-11 | `errgroup.Group` — goroutine group with first-error propagation, ctx cancel, `SetLimit` concurrency cap | X | golang.org/x/sync/errgroup (x/, not in corpus) — the de-facto structured-concurrency idiom Go teams reach for first |
| JOB-12 | Weighted semaphores, request-coalescing | X | golang.org/x/sync/{semaphore,singleflight} (x/, not in corpus) |
| JOB-13 | Subprocess jobs: `os/exec` `Command`/`CommandContext`, `Cancel`/`WaitDelay` graceful kill | STD | os/exec.txt — CommandContext kills on ctx done; `Cancel` hook + `WaitDelay` bound (1.20) give SIGTERM-then-SIGKILL semantics |
| JOB-14 | Graceful worker shutdown: `signal.NotifyContext` + WaitGroup drain | STD | os/signal.txt — pattern, not framework; cross-ref CONF-12; `context.Cause` names the signal |
| JOB-15 | Durable/persistent job queue (survive restarts, at-least-once) | NO | nothing; the gap Volt fills — ecosystem answers are river/asynq/machinery over Postgres/Redis |
| JOB-16 | Retry with backoff, max-attempts, dead-lettering | NO | no retry API anywhere in std; every team hand-rolls the `for`+`time.Sleep` loop |
| JOB-17 | Cron/calendar scheduling (expressions, catch-up, per-job persistence) | NO | `time.Ticker` is the only primitive; no cron parser, no missed-run semantics, no distributed locking of schedules |
| JOB-18 | Job results/status store & introspection (Django's `TaskResult`) | NO | no id/status/attempts/return-value model; in-process futures are DIY channels |
| JOB-19 | Worker-pool framework (named queues, priorities, concurrency as config) | NO | DIY: N goroutines ranging over a channel; `errgroup.SetLimit` (X) is the closest packaged form |
| JOB-20 | Transactional enqueue (`on_commit` pattern) | NO | database/sql has no hooks; nothing ties job dispatch to tx commit |

## P10 — Real-time

**Problem.** Push server events to connected browsers (websockets, presence, server push). **Answer.** No websockets in the standard library — full stop. What *is* there falls out of the Handler model: SSE and long-polling work today via `http.Flusher` + disconnect-canceled request contexts, `Hijacker` hands over the raw conn for any custom protocol, and goroutine-per-connection happily holds tens of thousands of open streams. Everything above the transport — SSE encoding, hubs/broadcast, presence, channels — does not exist.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| LIVE-1 | SSE via `http.Flusher` — write `data:` frames, `Flush()` per event | STD | net/http.txt — default HTTP/1.x and HTTP/2 ResponseWriters implement Flusher; wrappers may not (test at runtime); proxies may buffer; **no SSE event encoder/helper type** — wire format is hand-written |
| LIVE-2 | Per-stream deadline control: `NewResponseController` `Flush`/`SetWriteDeadline`/`SetReadDeadline` | STD | net/http.txt — since 1.20; lets a streaming handler clear the server-wide `WriteTimeout` for one long-lived response — load-bearing for SSE under production timeouts |
| LIVE-3 | Long-polling: block in the handler; `r.Context()` canceled on client disconnect | STD | net/http.txt — request context is canceled when the client's connection closes (and per-request with HTTP/2), replacing deprecated `CloseNotifier` |
| LIVE-4 | Goroutine-per-connection concurrency for many open streams | TOOL | language/runtime — no async/ASGI split; a blocked streaming handler costs one cheap goroutine |
| LIVE-5 | Full-duplex HTTP/1: `ResponseController.EnableFullDuplex` | STD | net/http.txt — since 1.21; interleave reads/writes on HTTP/1 (HTTP/2 always permits concurrent read/write) |
| LIVE-6 | Connection takeover: `http.Hijacker` → raw `net.Conn` | STD | net/http.txt — HTTP/1 only (HTTP/2 intentionally excluded); the escape hatch a DIY websocket implementation builds on |
| LIVE-7 | WebSockets (RFC 6455 server/client) | NO | absent from std; `golang.org/x/net/websocket` exists but is legacy/frozen and officially points at third-party (x/, not in corpus) — in practice gorilla/websocket or coder/websocket; the single loudest real-time gap |
| LIVE-8 | HTTP/2 server push: `http.Pusher` + `PushOptions` | STD | net/http.txt — interface still documented in 1.26, but moribund: major browsers removed HTTP/2 push handling, so it is not a viable real-time channel; `Push` returns `ErrNotSupported` when unavailable |
| LIVE-9 | HTTP/2 stream multiplexing; `Server.Protocols` incl. unencrypted h2c | STD | net/http.txt — automatic HTTP/2 on TLS; `Protocols`/`HTTP2Config` (since 1.24) enable `UnencryptedHTTP2` on the same port as HTTP/1 |
| LIVE-10 | `CloseNotifier` disconnect channel | STD | net/http.txt — **Deprecated**; predates context; kept for old code only |
| LIVE-11 | Shutdown of long-lived conns: `Server.RegisterOnShutdown` | STD | net/http.txt — `Shutdown` neither closes nor waits for hijacked conns (e.g. websockets); notifying/draining them is the app's job via the registered hook |
| LIVE-12 | Connection lifecycle observation: `Server.ConnState` hook | STD | net/http.txt — StateNew/Active/Idle/Hijacked/Closed transitions; per-connection, not per-request (HTTP/2 caveat noted in docs) |
| LIVE-13 | Broadcast hub / pub-sub / channel layer (rooms, groups, Redis fan-out) | NO | DIY map[chan]+mutex; no Phoenix-Channels/ActionCable analog, no cross-process fan-out |
| LIVE-14 | Presence tracking (who's connected, joins/leaves) | NO | nothing |
| LIVE-15 | SSE reconnection semantics (`Last-Event-ID`, `retry:` handling) | NO | wire protocol details left to the handler author |
| LIVE-16 | In-memory conn for stream tests: `net.Pipe` (+ synctest bubbles) | STD | net.txt — synchronous in-memory full-duplex `Conn` pair; testing/synctest.txt's HTTP example is built on it (cross-ref TEST-21) |

## P11 — Mail & notifications

**Problem.** Send templated transactional email reliably across environments and providers. **Answer.** Go parses RFC 5322 (net/mail) and speaks bare SMTP with STARTTLS and PLAIN/CRAM-MD5 (net/smtp — officially frozen: "not accepting new features"), and every MIME encoding brick is present (multipart, quoted-printable, RFC 2047 words). But there is no message builder, no attachment API, no HTML-mail layer, no delivery backends, no queue, no notification abstraction — the std docs themselves punt: "Higher-level packages exist outside of the standard library."

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MAIL-1 | Address parsing (RFC 5322) | STD | net/mail.txt: `ParseAddress`/`ParseAddressList`, `AddressParser` with pluggable `WordDecoder`; `Address.String()` RFC-2047-encodes display names |
| MAIL-2 | Message reading & header access | STD | net/mail.txt: `ReadMessage`, `Header.Get/Date/AddressList` — inbound parsing half is real |
| MAIL-3 | Mail date handling | STD | net/mail.txt `ParseDate`, `Header.Date()` |
| MAIL-4 | SMTP client protocol | STD | net/smtp.txt `Client`: Hello/Mail/Rcpt/Data, `Extension` discovery, `StartTLS`, `TLSConnectionState`. Package explicitly frozen |
| MAIL-5 | One-shot send | STD | net/smtp.txt `SendMail`: opportunistic STARTTLS; caller supplies a fully formatted RFC 822 byte blob — headers, CRLF and all |
| MAIL-6 | SMTP AUTH mechanisms | STD | net/smtp.txt `PlainAuth` (refuses unencrypted except localhost), `CRAMMD5Auth`. No LOGIN, no XOAUTH2 — Gmail/O365 need third-party |
| MAIL-7 | Implicit TLS (SMTPS :465) | STD | Assembly required: `tls.Dial` + `smtp.NewClient` (net/smtp.txt, crypto/tls.txt); no helper |
| MAIL-8 | MIME multipart construction (attachments, multipart/alternative) | STD | mime/multipart.txt `Writer`: `CreatePart(header)`, `SetBoundary`, `FormDataContentType`; you hand-assemble the tree — no attachment abstraction |
| MAIL-9 | Quoted-printable encoding | STD | mime/quotedprintable.txt `Reader`/`Writer` (RFC 2045) |
| MAIL-10 | Base64 body/attachment encoding | STD | encoding/base64.txt `StdEncoding` + `NewEncoder` streaming |
| MAIL-11 | Encoded-word headers (UTF-8 subjects) | STD | mime.txt `WordEncoder` (B/Q), `WordDecoder` with `CharsetReader` hook (std decodes only utf-8/iso-8859-1/us-ascii) |
| MAIL-12 | HTML email body rendering | STD | html/template.txt works for the body; no email-specific layer (CSS inlining, text alternative generation) |
| MAIL-13 | Message-builder API (`EmailMessage` equivalent: To/Cc/Bcc, attach, headers) | NO | The core gap; every Go shop imports go-mail or wraps smtp by hand |
| MAIL-14 | Pluggable delivery backends (SMTP/API providers; console/file/locmem/dummy for dev & test) | NO | No transport interface, no dev-mode capture, nothing to assert against in tests |
| MAIL-15 | Queued/async delivery with retries | NO | Goroutines + channels are the primitives; no outbox, retry, or backoff |
| MAIL-16 | DKIM signing, SPF/DMARC tooling | NO | net/smtp.txt docs explicitly disclaim DKIM support |
| MAIL-17 | Mailbox retrieval (IMAP/POP3) for inbound flows | NO | net/mail parses messages you already have; no fetch protocols in std |
| MAIL-18 | Notification abstraction (email/SMS/push/webhook channels) | NO | No equivalent of Laravel Notifications or ActionMailer callbacks |
| MAIL-19 | Error-report mails to admins | NO | log/slog.txt is the primitive; wiring alerts is app code |

## P12 — Caching & performance

**Problem.** Avoid recomputing/refetching expensive data and cut response cost. **Answer.** There is no cache framework at any granularity — no unified backend API, no per-view or fragment caching, no Redis/Memcached client, no TTL store. What Go has instead is the best raw material in the business: production-grade in-process concurrency primitives to build caches *from* (sync.Map/Pool/Once, atomics, maphash), conditional-GET handling built into ServeContent, and a profiling/observability stack (pprof, runtime/metrics, GC knobs, flight recorder) that frameworks on other runtimes can only envy.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CACHE-1 | Unified cache API over backends (Redis/Memcached/DB/file/local-memory) | NO | The headline gap. No `Cache` interface, no clients, no serialization convention |
| CACHE-2 | Per-site / per-view response caching | NO | No middleware, no key derivation from URL+headers |
| CACHE-3 | Template fragment caching | NO | html/template.txt has no cache action; recomputed every render |
| CACHE-4 | Concurrent map (low-level cache substrate) | STD | sync.txt `Map`: `Load`/`Store`/`LoadOrStore`/`CompareAndSwap`/`LoadAndDelete`/`Clear`; doc-targeted at append-only and disjoint-keyset cache cases |
| CACHE-5 | Object pooling (allocation amortization) | STD | sync.txt `Pool`: "caches allocated but unused items for later reuse"; GC-cleared, per-P sharded |
| CACHE-6 | Memoization / lazy initialization | STD | sync.txt `Once`, `OnceFunc`/`OnceValue`/`OnceValues` (since 1.21) |
| CACHE-7 | Lock-free counters & hot-path state | STD | sync/atomic.txt typed atomics: `Int64`, `Pointer[T]`, `Value` (load/store of config snapshots) |
| CACHE-8 | Cache-stampede protection (request coalescing) | X | x/sync/singleflight — x/, not in corpus. Nothing in std |
| CACHE-9 | TTL / LRU / size-bounded eviction | NO | No expiring store anywhere in std; sync.Map never evicts |
| CACHE-10 | Hashing for shard/key routing | STD | hash/maphash.txt: `Bytes`/`String`/`Comparable[T]` (since 1.24), per-process random seed; hash/fnv.txt & crc32.txt when a stable hash is needed |
| CACHE-11 | Value interning & weak references (memory-efficient caches) | STD | `unique.Make` (since 1.23) canonicalizes comparables; `weak.Pointer` (since 1.24) enables non-retaining caches — from knowledge, neither package in corpus |
| CACHE-12 | Conditional GET (ETag, If-Modified-Since, If-Range, ranges) | STD | net/http.txt `ServeContent`: evaluates If-Match/If-None-Match/If-Modified-Since/If-Unmodified-Since/If-Range against caller-set ETag + modtime, emits Last-Modified and 304s; `FileServer`/`ServeFileFS` inherit it |
| CACHE-13 | Cache-Control / Vary / Expires helpers | STD | Manual only: `w.Header().Set(...)` — headers pass through untouched, but no `patch_cache_control`-style helpers or defaults |
| CACHE-14 | Response compression | NO | compress/gzip.txt (+ flate, zlib) supply the codec and the http.Client transparently *de*compresses, but there is no server-side Accept-Encoding negotiation middleware |
| CACHE-15 | Client-side HTTP cache (RFC 9111) | NO | http.Client caches nothing; no Transport cache layer |
| CACHE-16 | Caching reverse proxy / CDN-ish edge | NO | net/http/httputil.txt `ReverseProxy` forwards, never stores |
| CACHE-17 | CPU & heap profiling | STD | runtime/pprof.txt: `StartCPUProfile`, `WriteHeapProfile`, `Lookup` (goroutine/block/mutex/allocs); pprof `Labels`+`Do` attribute samples to requests |
| CACHE-18 | Live profiling endpoints | STD | net/http/pprof.txt: /debug/pprof handlers, importable into any mux — always-on production profiling |
| CACHE-19 | Runtime metrics export | STD | runtime/metrics.txt stable metric registry (GC, sched, mem); expvar.txt publishes JSON counters at /debug/vars |
| CACHE-20 | GC & memory tuning | STD | runtime/debug.txt: `SetGCPercent`, `SetMemoryLimit` (soft limit, since 1.19), `FreeOSMemory`, `ReadGCStats` |
| CACHE-21 | Execution tracing | STD | runtime/trace.txt: full tracer plus in-production `FlightRecorder` ring buffer (since 1.25) — capture the trace *after* the latency spike |
| CACHE-22 | Benchmarking | TOOL | testing.txt `testing.B` with `b.Loop` (since 1.24), `go test -bench -benchmem`; benchstat comparison is x/perf |
| CACHE-23 | Concurrency for latency hiding | TOOL | goroutines/channels/`sync.WaitGroup.Go` (since 1.25); GOMAXPROCS container-CPU-aware since 1.25 |
| CACHE-24 | Data-race safety for hand-built caches | TOOL | `go test -race` / `-race` builds — makes DIY cache code auditable |
| CACHE-25 | Profile-guided optimization | TOOL | PGO via default.pgo since 1.21; feed CACHE-17 profiles back into the compiler |
| CACHE-26 | Allocation diagnostics | TOOL | `go build -gcflags=-m` escape analysis; `go vet` — perf-note tier, from knowledge |

## P13 — Files & storage

**Problem.** Accept file uploads without exhausting memory, and abstract file persistence behind a swappable API. **Answer.** Half of each, done well: `mime/multipart` streams upload parts and spills large files to temp files at a caller-set memory ceiling with DoS limits built in, and `fs.FS` is the stdlib's genuinely universal storage abstraction — implemented by the OS (`os.DirFS`), compiled-in assets (`embed.FS`), archives (`zip.Reader`), and in-memory maps (`fstest.MapFS`), and consumed by `net/http`, templates, and the archive writers. The catch: `fs.FS` is read-only — there is no writable storage interface and no cloud backend, so a swappable write-side Storage API is the Volt gap.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| FILE-1 | `fs.FS` — one-method (`Open`) file-system abstraction; THE stdlib storage interface, decoupling "where files live" from all consumers | STD | io/fs.txt — since 1.16; slash-separated, unrooted, UTF-8 path discipline with `ValidPath` |
| FILE-2 | File-object model: `fs.File` (Stat/Read/Close), `FileInfo` (name/size/mode/modtime), `DirEntry`, `FileMode` bits, `PathError` + sentinel errors (`ErrNotExist` etc. via `errors.Is`) | STD | io/fs.txt |
| FILE-3 | Capability-upgrade interfaces: `ReadDirFS`, `ReadFileFS`, `StatFS`, `SubFS`, `GlobFS`, `ReadLinkFS` — helpers use fast paths when implemented | STD | io/fs.txt — `ReadLinkFS` since 1.25 |
| FILE-4 | FS-generic helpers: `fs.WalkDir` (SkipDir/SkipAll), `fs.Glob`, `fs.ReadFile`, `fs.ReadDir`, `fs.Stat`, `fs.Sub` (re-rooting), `fs.ValidPath` | STD | io/fs.txt — Sub explicitly not a chroot security boundary (symlinks escape) |
| FILE-5 | Local-disk backend: `os.DirFS(dir)` exposes a directory as `fs.FS` | STD | os.txt |
| FILE-6 | Compiled-in backend: `//go:embed` → `embed.FS` (read-only, goroutine-safe; files open as `io.Seeker`+`io.ReaderAt`) — static assets/templates shipped in the binary | TOOL | embed.txt — directive is a toolchain feature; `.`/`_`-prefixed files excluded unless `all:` prefix |
| FILE-7 | In-memory backend + conformance test: `fstest.MapFS`, `fstest.TestFS(fsys, expected...)` for verifying custom FS implementations | STD | testing/fstest.txt — the "in-memory storage for tests" analog (read side) |
| FILE-8 | OS file CRUD: `Open`/`Create`/`OpenFile` (flags+perm), `ReadFile`/`WriteFile`, `Mkdir`/`MkdirAll`, `Rename`, `Remove`/`RemoveAll`, `Chmod`/`Chown`/`Chtimes`, `Truncate`, `Link`/`Symlink`; `File` is an `io.Reader/Writer/Seeker/ReaderAt/WriterAt` with `Sync` | STD | os.txt |
| FILE-9 | Temp files: `os.CreateTemp(dir, pattern)` (race-free unique names), `os.MkdirTemp`, `os.TempDir` | STD | os.txt |
| FILE-10 | Traversal-resistant rooted access: `os.OpenRoot` → `os.Root` — full op set (`Open/Create/OpenFile/Mkdir(All)/Remove(All)/Rename/ReadFile/WriteFile/Stat/Lstat/Chmod/Chown/Chtimes/Link/Symlink/OpenRoot` nesting) that cannot escape the root via `..` or symlinks; `Root.FS()` adapts to `fs.FS` | STD | os.txt — since 1.24, op set expanded 1.25; the safe primitive for "save upload under this directory" |
| FILE-11 | `os.OpenInRoot(dir, name)` one-shot traversal-safe open | STD | os.txt |
| FILE-12 | `os.CopyFS(dir, fsys)` — materialize any `fs.FS` tree onto disk | STD | os.txt — since 1.23 |
| FILE-13 | Path hygiene: `filepath.Clean/Base/Ext/Join/Rel/Abs`, `filepath.IsLocal` (no escape via `..`/absolute, since 1.20), `filepath.Localize` (slash-path → safe OS path, since 1.23), `EvalSymlinks`, `Glob`, `WalkDir` | STD | path/filepath.txt — `Part.FileName` sanitization (FILE-19) leans on `Base` |
| FILE-14 | Streaming plumbing: `io.Copy` (uses `os.File.ReadFrom` → kernel-side copy where available), `io.CopyN`, `io.LimitReader`, `io.TeeReader` (hash while storing), `io.Pipe`, `io.ReadAll` | STD | io.txt, os.txt |
| FILE-15 | Streaming multipart parsing: `multipart.NewReader(r, boundary).NextPart()` — pull parser, consumes input as needed, constant memory; transparent quoted-printable decoding (`NextRawPart` to opt out) | STD | mime/multipart.txt — boundary comes from `mime.ParseMediaType` on Content-Type |
| FILE-16 | Memory→disk spill: `Reader.ReadForm(maxMemory)` keeps ≤ maxMemory (+10MB non-file reserve) in RAM, overflows file parts to `*os.File` temp files; `Form.RemoveAll()` cleanup; `ErrMessageTooLarge` | STD | mime/multipart.txt — the Django "2.5 MB crossover" equivalent, threshold caller-chosen |
| FILE-17 | Multipart DoS limits baked in: 10,000 headers per part/form, 1,000 parts per form; tunable via `GODEBUG=multipartmaxheaders=`/`multipartmaxparts=` | STD | mime/multipart.txt |
| FILE-18 | Uploaded-file handle: `multipart.File` (Reader+ReaderAt+Seeker+Closer, memory- or disk-backed), `FileHeader{Filename, Size, Header}.Open()`; `Part.FileName()` passed through `filepath.Base` (path-injection guard) | STD | mime/multipart.txt |
| FILE-19 | HTTP integration: `r.ParseMultipartForm(maxMemory)`, `r.FormFile(key)` (+`ErrMissingFile`), `r.MultipartReader()` for fully streaming handling | STD | net/http.txt |
| FILE-20 | Upload size cap: `http.MaxBytesReader` → typed `*MaxBytesError`, closes underlying reader, stops wasted server resources | STD | net/http.txt — `MaxBytesError` since 1.19 |
| FILE-21 | Multipart generation: `multipart.Writer` (`CreateFormFile`/`CreateFormField`/`CreatePart`, `FormDataContentType`) — client uploads and tests | STD | mime/multipart.txt |
| FILE-22 | Static file serving: `http.FileServer(http.Dir(...))` / `http.FileServerFS(fsys)` — directory listing, index.html, redirects | STD | net/http.txt — `FileServerFS` since 1.22; pairs with embed.FS (embed.txt shows the exact pattern) |
| FILE-23 | Single-file/content serving: `http.ServeFile`/`ServeFileFS` (reject `..` in path), `http.ServeContent` — Range requests, If-Modified-Since, If-Match/If-None-Match via ETag, Content-Type by extension or sniffing | STD | net/http.txt — the "X-Sendfile-quality" download responder |
| FILE-24 | Content-type sniffing: `http.DetectContentType` (WHATWG algorithm, first 512 bytes, always returns a valid MIME type) — validate uploads by content, not extension | STD | net/http.txt |
| FILE-25 | MIME registry: `mime.TypeByExtension`, `ExtensionsByType`, `AddExtensionType`, `ParseMediaType`/`FormatMediaType` (Content-Disposition parsing incl. RFC 2231) | STD | mime.txt |
| FILE-26 | ZIP read/write: `zip.Reader`/`Writer`, ZIP64, Store/Deflate + `RegisterCompressor` custom methods, per-file `fs.FileInfo` interop (`FileInfoHeader`) | STD | archive/zip.txt |
| FILE-27 | Zip-slip guard: `zip.ErrInsecurePath`/`tar.ErrInsecurePath` — non-local names (per `filepath.IsLocal`) rejected under `GODEBUG=zipinsecurepath=0`/`tarinsecurepath=0` (opt-in as of go1.26.5) | STD | archive/zip.txt, archive/tar.txt |
| FILE-28 | Archive ↔ FS bridges: `zip.Reader.Open` implements `fs.FS` (serve straight from a zip); `zip.Writer.AddFS`/`tar.Writer.AddFS` archive any `fs.FS` tree | STD | archive/zip.txt, archive/tar.txt — AddFS since 1.22 |
| FILE-29 | TAR streaming read/write: USTAR/PAX/GNU formats, PAXRecords/xattrs, sparse-file reads, `FileInfoHeader` | STD | archive/tar.txt — Writer has no sparse support |
| FILE-30 | Compression codecs: `compress/gzip`, `flate`, `zlib` (streaming reader/writer) | STD | compress/gzip.txt et al. |
| FILE-31 | Content hashing for checksums/ETags/dedup: `crypto/sha256`, `crypto/md5` over `io.TeeReader` while storing | STD | crypto/sha256.txt, crypto/md5.txt |
| FILE-32 | Image dimension probing: `image.DecodeConfig` + `image/png`, `image/jpeg`, `image/gif` decoders (width/height without full decode) | STD | std `image` packages — in std but not in this corpus dump; cited from knowledge |
| FILE-33 | Image scaling/thumbnailing: `x/image/draw` (Catmull-Rom etc. interpolators); std `image` decodes but has no quality resampler | X | golang.org/x/image — x/, not in corpus |
| FILE-34 | Writable storage abstraction: `fs.FS` is read-only — no `WriteFS`/`CreateFS` interface in std as of go1.26.5, so "swap local disk for X" only works for reads | NO | The central Volt P13 gap; write-side portability requires a bespoke interface (`os.Root` is the best local backend for it) |
| FILE-35 | Cloud/object storage backends (S3/GCS/Azure), public-URL and signed-URL generation | NO | Ecosystem: aws-sdk-go, gocloud.dev blob |
| FILE-36 | Upload management layer: storage-key naming strategies, overwrite/dedup policy, DB association (FileField equivalent), upload validation (allowed types/size policy objects) | NO | Primitives exist (FILE-9/10/18/24), policy layer doesn't |
| FILE-37 | Image processing pipeline (resize/crop/rotate/EXIF handling) | NO | Decode-only in std (FILE-32); ecosystem: imaging, bimg |
| FILE-38 | Resumable/chunked upload protocol (tus-style), multipart-merge helpers | NO | — |

## P14 — Building APIs

**Problem.** Serialize domain objects to JSON/XML/CSV, validate and decode what comes in, negotiate formats, and version/document the surface. **Answer.** Struct tags are the serialization mechanism — declarative field mapping compiled into the type, with `encoding/json` (and the experimental, much stricter `encoding/json/v2` + its `jsontext` streaming layer) doing the reflection work, and streaming Encoder/Decoder pairs for pipelines. That's where it stops: no serializer/resource layer, no content negotiation, no OpenAPI, no pagination/versioning/auth conventions — a JSON codec is not an API framework.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| API-1 | JSON marshal/unmarshal (v1) | STD | encoding/json.txt — reflection-based `Marshal`/`Unmarshal`; maps, slices, pointers, embedding; unknown input fields ignored by default; unmarshal name-matching is case-insensitive (footgun v2 fixes) |
| API-2 | Struct tags as the serialization schema | TOOL | language feature — `json:"name,omitempty"` / `xml:"a>b,attr"`: field naming, omission, flattening declared on the type; the stdlib's answer to serializer classes |
| API-3 | v1 tag options | STD | encoding/json.txt — rename, `-` (skip), `omitempty` (empty values), `omitzero` (zero values / `IsZero()`, added 1.24), `,string` quoting for 64-bit-safe numbers |
| API-4 | Custom serialization hooks | STD | encoding/json.txt — `json.Marshaler`/`Unmarshaler`, falls back to `encoding.TextMarshaler`; error wrapping via `MarshalerError` |
| API-5 | Raw/lazy JSON | STD | encoding/json.txt — `RawMessage` for pass-through and delayed decoding (the poor man's polymorphism: sniff a type field, then decode) |
| API-6 | Number fidelity | STD | encoding/json.txt — `json.Number` + `Decoder.UseNumber()` to avoid float64 truncation of int64/uint64 |
| API-7 | Streaming encode/decode (v1) | STD | encoding/json.txt — `NewEncoder(w).Encode` straight to ResponseWriter; `NewDecoder(r).Decode` from Body; `Decoder.More`/`Token` for incremental array/NDJSON processing without whole-doc buffering |
| API-8 | Strict input mode (v1) | STD | encoding/json.txt — `Decoder.DisallowUnknownFields`; combine with `http.MaxBytesReader` for the canonical safe-decode helper every project copy-pastes |
| API-9 | Output shaping | STD | encoding/json.txt — `MarshalIndent`, `Encoder.SetIndent`/`SetEscapeHTML(false)`, `Compact`/`Indent`/`Valid`, `HTMLEscape` for `<script>`-embeddable JSON |
| API-10 | JSON error taxonomy (v1) | STD | encoding/json.txt — `SyntaxError` (offset), `UnmarshalTypeError` (field/type/offset), `UnsupportedTypeError`/`UnsupportedValueError` — enough to build 400-response mappers |
| API-11 | `encoding/json/v2` | STD | encoding/json/v2.txt — experimental, `GOEXPERIMENT=jsonv2` only (shipped 1.25, still gated in 1.26), not under the Go 1 compat promise; v1 is reimplemented on top of it |
| API-12 | v2 options system | STD | encoding/json/v2.txt — per-call `Options`: `RejectUnknownMembers`, `MatchCaseInsensitiveNames`, `Deterministic`, `StringifyNumbers`, `OmitZeroStructFields`, `FormatNilMapAsNull`/`FormatNilSliceAsNull`, `JoinOptions`; field tags take precedence |
| API-13 | v2 tag extensions | STD | encoding/json/v2.txt — `case:ignore\|strict`, `format:RFC3339` / `format:'2006-01-02'`, `inline` (flattening), `unknown` (capture unmatched members into a `jsontext.Value`/map fallback), single-quoted JSON names |
| API-14 | v2 caller-side custom marshalers | STD | encoding/json/v2.txt — `WithMarshalers` + `MarshalFunc`/`MarshalToFunc` (and unmarshal duals): override serialization of third-party types per call, without wrapper types — genuinely new capability vs v1 |
| API-15 | v2 io/token streaming | STD | encoding/json/v2.txt — `MarshalWrite`/`UnmarshalRead` (io.Writer/Reader), `MarshalEncode`/`UnmarshalDecode` against jsontext Encoder/Decoder |
| API-16 | v2 semantic hardening vs v1 | STD | encoding/json/v2.txt — case-sensitive name matching, duplicate object names rejected, invalid UTF-8 rejected, nil slice/map marshal as `[]`/`{}` by default; `SemanticError` carries a JSON `Pointer` to the offending member |
| API-17 | `jsontext` syntactic layer | STD | encoding/json/jsontext.txt — token/value Encoder+Decoder (`ReadToken`/`ReadValue`/`SkipValue`/`PeekKind`, `WriteToken`/`WriteValue`), `Kind`, RFC 6901 `Pointer`, stack introspection — true streaming transforms without materializing documents |
| API-18 | jsontext formatting/laxness options | STD | encoding/json/jsontext.txt — `Multiline`/`WithIndent`, `SpaceAfterColon/Comma`, `EscapeForHTML`/`EscapeForJS`, `AllowDuplicateNames`, `AllowInvalidUTF8`, canonicalization (`CanonicalizeRawInts/Floats`, `ReorderRawObjects`) |
| API-19 | XML | STD | encoding/xml.txt — Marshal/Unmarshal with `xml:` tags (`attr`, `chardata`, `innerxml`, `a>b>c` paths, `omitempty`), `Marshaler`/`MarshalerAttr`, streaming `Decoder.Token`/`Encoder.EncodeToken`; namespace support is notoriously partial |
| API-20 | CSV | STD | encoding/csv.txt — RFC 4180 `Reader` (`FieldPos`, `ReuseRecord`, LazyQuotes, per-record streaming) and buffered `Writer`; no struct mapping (tags don't apply) |
| API-21 | Go-native binary encoding | STD | encoding/gob.txt — self-describing stream codec for Go↔Go RPC/caching; useless cross-language |
| API-22 | Compression codec | STD | compress/gzip.txt — `gzip.Writer`/`Reader` (+ flate/zlib); response-compression wiring is manual (see CTRL-33) |
| API-23 | Content negotiation | NO | no Accept/Accept-Encoding q-value parsing, no format routing (`.json` suffix, `?format=`), no per-type render registry; mime.txt parses a single media type and stops |
| API-24 | Serializer / resource layer (DRF-style) | NO | struct tags declare shape only — no declarative validation, computed/relation fields, hyperlinked identities, or per-view serializer selection; the central gap between "JSON codec" and "API framework" |
| API-25 | Request validation | NO | no constraint tags, no validator (see P6); a decoded struct is syntactically checked only — every field-level rule is hand-written `if` statements |
| API-26 | OpenAPI / JSON Schema generation | NO | no schema emission or route metadata to derive it from (compounds ROUTE-18's no-introspection) |
| API-27 | API versioning conventions | NO | nothing beyond registering `/v1/...` path patterns by hand |
| API-28 | Pagination / filtering / sorting conventions | NO | no page/cursor helpers, no query-param filter binding |
| API-29 | Token auth / API keys / OAuth | NO | only `Request.BasicAuth`/`SetBasicAuth` (net/http.txt); bearer-token parsing, key storage, OAuth flows all absent (see P7) |
| API-30 | CORS | NO | `CrossOriginProtection` (CTRL-24) is CSRF *rejection*, not CORS header negotiation — no Access-Control-Allow-* handling, no preflight helper |
| API-31 | Rate limiting / quotas | NO | see CTRL-32; `golang.org/x/time/rate` (x/, not in corpus) is the standard building block |
| API-32 | gRPC / protobuf | NO | not in the stdlib and not even under x/ — google.golang.org/grpc is fully third-party |
| API-33 | Streaming JSON responses (NDJSON/SSE) | STD | encoding/json.txt + net/http.txt — idiom: `Encoder.Encode` per line + `Flusher.Flush`; no framing helper, no client reconnect/backpressure support |
| API-34 | Browsable API / auto docs UI | NO | nothing like DRF's browsable API or Swagger UI hosting |

## P15 — i18n & l10n

**Problem.** Translate the UI, format data per locale, and handle time zones correctly. **Answer.** It doesn't — this is the emptiest section in the standard library. Go stops at the Unicode/UTF-8 substrate and the IANA time-zone database. No message catalogs, no plural rules, no locale-aware number/date/currency formatting, no collation, no Accept-Language negotiation: all of it lives in golang.org/x/text (quasi-first-party, CLDR-backed, but with a far thinner extraction/workflow story than gettext) or in the framework.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| I18N-1 | UTF-8 native text | STD | Source and strings are UTF-8; `range` over string yields runes; unicode/utf8.txt `DecodeRuneInString`, `RuneCountInString`, `Valid`/`ValidString` |
| I18N-2 | Unicode character database | STD | unicode.txt: category/script tables, `IsLetter`/`IsSpace`, rune-level `ToUpper`/`ToLower`/`ToTitle` |
| I18N-3 | Locale-specific case mapping | STD | The *only* locale-aware behavior in std: `unicode.SpecialCase` (`TurkishCase`, `AzeriCase`) + strings.txt `ToUpperSpecial`/`ToLowerSpecial`/`ToTitleSpecial` |
| I18N-4 | Case-insensitive comparison | STD | strings.txt `EqualFold` — simple Unicode folding, not full/locale-tailored folding |
| I18N-5 | Time zones | STD | time.txt: `LoadLocation` (IANA tzdb from system, GOROOT, or embedded time/tzdata import), `LoadLocationFromTZData`, `Time.In` — the tz story is genuinely complete |
| I18N-6 | Date/time *formatting* | STD | English-only: time.txt reference-layout formatting hard-codes English month/day names; no locale-aware rendering — the l10n half is missing |
| I18N-7 | Message catalogs / translation | X | x/text/message + catalog + `gotext` extraction CLI — x/, not in corpus; markedly less mature than gettext (no fuzzy matching, thin tooling) |
| I18N-8 | Plural rules | X | x/text/feature/plural (CLDR plural categories) — x/, not in corpus. Nothing in std |
| I18N-9 | BCP 47 tags & language negotiation | X | x/text/language `Matcher` — x/, not in corpus. net/http.txt does not even parse Accept-Language |
| I18N-10 | Locale-aware number formatting | X | x/text/number, x/text/message printers — x/, not in corpus; strconv.txt/fmt.txt are locale-blind (always `1234567.89`) |
| I18N-11 | Currency formatting | X | x/text/currency — x/, not in corpus |
| I18N-12 | Collation (locale-correct sorting) | X | x/text/collate — x/, not in corpus; sort.txt/slices.txt compare code points/bytes only |
| I18N-13 | Unicode normalization (NFC/NFD) | X | x/text/unicode/norm — x/, not in corpus; std will happily treat é and e+◌́ as different users |
| I18N-14 | Charset transcoding (legacy encodings) | X | x/text/encoding + charmap — x/, not in corpus; std mime.txt `WordDecoder` handles only utf-8/us-ascii/iso-8859-1 without a `CharsetReader` you supply |
| I18N-15 | Bidirectional text | X | x/text/unicode/bidi — x/, not in corpus |
| I18N-16 | Per-request locale selection (URL prefix → cookie → Accept-Language) | NO | Django's `LocaleMiddleware` pipeline has no counterpart; framework territory end-to-end |
| I18N-17 | Template translation tags & lazy strings | NO | text/template.txt / html/template.txt have no i18n hooks; no `{% translate %}` equivalent |
| I18N-18 | Localized input parsing (decimal comma, local date formats) | NO | strconv and time.Parse accept one canonical format each |
| I18N-19 | Translated/localized URLs | NO | ServeMux patterns are literal; no per-language route variants |
| I18N-20 | Non-Gregorian calendars | NO | time.txt is Gregorian-only; not in x/text either |

## P16 — Testing

**Problem.** Test application behavior across the HTTP, handler and I/O layers with fast, isolated runs. **Answer.** `go test` + the `testing` package are a complete runner: subtests, parallelism, benchmarks (`b.Loop`), coverage-guided fuzzing, example verification, coverage, and — since 1.25 — `testing/synctest`'s fake-clock bubbles for deterministic concurrent code; `httptest` covers both in-process handler tests (Recorder) and real-socket E2E (Server), with `fstest`/`iotest` faking the filesystem and misbehaving I/O. What's missing is everything framework-shaped: no assertion/matcher library, no mocking, no fixtures/factories, and no database test-isolation story.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| TEST-1 | `go test` runner: per-package binaries, result caching, `-run`/`-v`/`-count`/`-timeout`/`-short`/`-shuffle`, parallel across packages | TOOL | toolchain; testing.txt refers to `go help testflag` |
| TEST-2 | `testing.T`: `Error(f)`/`Fatal(f)`/`Log(f)`/`Fail`/`FailNow`/`Skip*`/`Helper`/`Name`/`Output` | STD | testing.txt — `_test.go` files, `TestXxx(*testing.T)` discovery by name |
| TEST-3 | Subtests: `T.Run` (hierarchical, `-run Test/sub` filtering, table-driven idiom) | STD | testing.txt — # Subtests and Sub-benchmarks; also enables shared setup/teardown |
| TEST-4 | `t.Parallel()` — intra-package parallel tests | STD | testing.txt — parallel tests never overlap across `-count` repeats |
| TEST-5 | `TestMain(m *testing.M)` global setup/teardown | STD | testing.txt — # Main; `m.Run()` returns the exit code |
| TEST-6 | `T.Cleanup(f)` LIFO teardown (incl. subtests) | STD | testing.txt |
| TEST-7 | Process-state isolation: `T.Setenv`, `T.Chdir`, `T.TempDir` | STD | testing.txt — Chdir **since 1.24**; Setenv/Chdir incompatible with `t.Parallel` |
| TEST-8 | `T.Context()` — test-scoped context, canceled before Cleanup | STD | testing.txt — since 1.24 |
| TEST-9 | `T.Deadline()` + `-timeout` | STD | testing.txt |
| TEST-10 | CI metadata: `T.Attr(key,value)` + `T.ArtifactDir()`/`-artifacts` | STD | testing.txt — recent additions (Attr **1.25**, ArtifactDir **1.26**); attribute meaning left to CI systems |
| TEST-11 | Black-box testing via the `_test` package convention | STD | testing.txt — same dir, `pkg_test` package, exported identifiers only |
| TEST-12 | Benchmarks: `BenchmarkXxx(*testing.B)` with `for b.Loop()` | STD | testing.txt — `b.Loop` **since 1.24** (auto timer management, keeps loop body alive against the optimizer); legacy `b.N` style still documented |
| TEST-13 | Benchmark metrics: `ReportAllocs`, `SetBytes`, `ReportMetric`, `Elapsed`, `AllocsPerRun`, `-benchmem` | STD | testing.txt — stable benchmark output format (go.dev/design/14313) |
| TEST-14 | Parallel benchmarks: `b.RunParallel`/`PB.Next`, `SetParallelism`, `-cpu` | STD | testing.txt |
| TEST-15 | Statistical A/B comparison: benchstat | X | golang.org/x/perf/cmd/benchstat (x/, not in corpus) — named by testing.txt itself as the standard tool |
| TEST-16 | Example tests: `ExampleXxx` with `// Output:` / `// Unordered output:` verification | STD | testing.txt — doubles as documentation on pkgsite |
| TEST-17 | Native coverage-guided fuzzing: `FuzzXxx(*testing.F)`, `F.Add` seeds, `testdata/fuzz/<Name>` corpus, `-fuzz` | STD | testing.txt — since 1.18; failing inputs auto-saved as regression seeds; runs seeds as normal tests when not fuzzing |
| TEST-18 | Coverage: `go test -cover`/`-coverprofile`, `go tool cover`; `testing.Coverage()`/`CoverMode()` | TOOL | toolchain + testing.txt — line coverage built in; no third-party coverage.py analog needed |
| TEST-19 | Race detector: `go test -race` | TOOL | toolchain — dynamic data-race detection; the concurrency-testing workhorse |
| TEST-20 | `go vet` subset auto-runs during `go test` | TOOL | toolchain — catches printf mistakes etc. before tests run |
| TEST-21 | `testing/synctest`: `Test(t, f)` bubbles — fake clock (starts 2000-01-01), instant `time.Sleep`, `Wait()` for quiescence, deadlock panic | STD | testing/synctest.txt — **stable since 1.25** (GOEXPERIMENT in 1.24); deterministic tests of timeouts/tickers/concurrency; ships worked examples for context and HTTP 100-continue |
| TEST-22 | Handler unit tests: `httptest.NewRequest(WithContext)` + `ResponseRecorder`/`Result()` | STD | net/http/httptest.txt — no network, no server; Result() gives a real `*http.Response` |
| TEST-23 | E2E tests: `httptest.Server`/`NewTLSServer` (loopback socket, `EnableHTTP2`, `.Client()` trusts the test cert) | STD | net/http/httptest.txt — `NewUnstartedServer` for config; `Close` blocks until requests finish |
| TEST-24 | In-memory network: `net.Pipe()` | STD | net.txt — synchronous `Conn` pair; combine with `Transport.DialContext` override (synctest example) |
| TEST-25 | Filesystem fakes: `fstest.MapFS` + `fstest.TestFS` conformance checker | STD | testing/fstest.txt — in-memory fs.FS; TestFS walks and verifies any FS implementation |
| TEST-26 | Misbehaving I/O: `iotest` (ErrReader, TimeoutReader, HalfReader, OneByteReader, TestReader…) | STD | testing/iotest.txt |
| TEST-27 | Property-based testing: `quick.Check`/`CheckEqual`/`Generator` | STD | testing/quick.txt — **frozen**, no new features; fuzzing is the modern successor |
| TEST-28 | `slog` handler conformance: testing/slogtest | STD | std since 1.21/1.22 (not in corpus dumps) — verifies custom slog.Handler behavior |
| TEST-29 | Assertion/matcher library (`assertEqual`, `assertContains`, HTML/JSON assertions) | NO | the idiom is `if got != want { t.Errorf(...) }`; testify/go-cmp are ecosystem — std deliberately refuses an assert API |
| TEST-30 | Mocking/stubbing framework | NO | idiom: accept interfaces, hand-write fakes; gomock/moq are ecosystem |
| TEST-31 | Fixtures/factories/seed-data loading | NO | nothing like Django `fixtures`; `testdata/` dir convention + go:embed is the raw material; golden-file `-update` flags are hand-rolled |
| TEST-32 | Database test isolation (transactional tests, test-DB lifecycle, parallel per-process DBs) | NO | database/sql has no test story at all — no auto-created test DB, no rollback-per-test, no `assertNumQueries` |
| TEST-33 | In-browser / E2E driver (Selenium/Playwright analog) | NO | nothing |

## P17 — CLI, codegen & developer experience

**Problem.** Scaffold, run, inspect and administer the project from the command line. **Answer.** The `go` tool *is* the developer experience — build/run/test/vet/fmt/generate/mod are uniform, zero-config and fast, and that speed substitutes for a dev-server story. For the app's own CLI there is exactly one package: `flag` (typed flags, FlagSet-per-subcommand). There is no scaffolding, no REPL, no hot reload, no management-command framework — a Volt CLI must be built up from `flag`/`os.Args`, and codegen from `go generate` + `text/template`.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CLI-1 | `go build` / `go install` — one-command compile to a self-contained binary | TOOL | toolchain — no Procfile/venv/asset pipeline; output is the deploy artifact (cross-ref CONF-9) |
| CLI-2 | `go run` — compile-and-execute for dev iteration | TOOL | toolchain — fast compiles are the substitute for a reloading dev server |
| CLI-3 | `go test` | TOOL | toolchain — cross-ref P16 |
| CLI-4 | `go vet` — static analysis (printf, struct tags, context misuse, lostcancel…) | TOOL | toolchain — e.g. checks CancelFuncs are called on all paths (context.txt names it) |
| CLI-5 | `gofmt` / `go fmt` — canonical, argument-free formatting | TOOL | toolchain — ends style debates by fiat; frameworks elsewhere ship linters to get this |
| CLI-6 | `go generate` + `//go:generate` directives | TOOL | toolchain — the codegen driver: runs annotated commands on demand (not at build time); how stringer/mocks/sqlc-style generators are wired |
| CLI-7 | `go mod` / modules — dependency management, `go get`, reproducible builds | TOOL | toolchain — cross-ref EXT-17 |
| CLI-8 | `go doc` / pkgsite — API docs from source | TOOL | toolchain — `go doc -all` is literally this corpus's source (MANIFEST) |
| CLI-9 | `flag`: typed flags (`Bool/Int/String/Duration/Float64/Uint…`), defaults, auto `-h`/usage, `PrintDefaults` | STD | flag.txt — `-flag=x` syntax; no short/long aliasing, no GNU-style grouping |
| CLI-10 | Custom flag parsing: `flag.Func`/`BoolFunc`/`TextVar` | STD | flag.txt — TextVar (1.19) binds any `encoding.TextUnmarshaler` (e.g. slog.Level, netip.Addr) |
| CLI-11 | `flag.Value` interface + `flag.Var` — user-defined flag types | STD | flag.txt — the extension seam (cross-ref EXT-12) |
| CLI-12 | Subcommands via `flag.NewFlagSet` per verb + `switch os.Args[1]` | STD | flag.txt — FlagSet is the primitive; no help tree, no nested command routing — that's why cobra exists |
| CLI-13 | Process plumbing: `os.Args`, `os.Exit`, `os.Stdin/Stdout/Stderr` | STD | os.txt |
| CLI-14 | Shelling out: `os/exec` `Command`/`CommandContext`, `Output`/`CombinedOutput`, stdin/out pipes | STD | os/exec.txt — no shell expansion by design; `Cancel`/`WaitDelay` for graceful termination (1.20) |
| CLI-15 | Codegen substrate: `text/template` | STD | text/template.txt — what a `volt new`/generator would render scaffolds with |
| CLI-16 | Subcommand CLI framework (nested commands, help trees, shell completion) | NO | cobra/urfave-cli are ecosystem; std stops at FlagSet |
| CLI-17 | Project/app scaffolding (`startproject`, `rails new`) | NO | nothing in std or toolchain; `gonew` is an experimental x/tools command (x/, not in corpus) |
| CLI-18 | REPL / interactive shell | NO | no REPL for a compiled language; nothing like `manage.py shell` |
| CLI-19 | Hot reload / watch-mode dev server | NO | no file watcher, no autoreload; ecosystem (air, wgo) or `go run` re-invocation; fast compiles soften but don't close the gap |
| CLI-20 | Terminal UX helpers: colors, progress bars, prompts | NO | fmt + raw ANSI by hand; x/term handles raw mode only (x/, not in corpus) |
| CLI-21 | Management-command registry (apps contribute commands, `call_command`) | NO | nothing like Django's `management/commands/` discovery |

## P18 — Configuration & deployment

**Problem.** Configure the app per environment and put it into production. **Answer.** Configuration is spartan: `os.Getenv` + `flag` are the entire story — no env-file loader, no config-file framework, no layering, no secrets. Deployment is the opposite — Go's crown jewel: one `go build` yields a self-contained static binary with assets `go:embed`-ed in, cross-compiled via two env vars, serving production TLS itself (`crypto/tls`, HTTP/2 automatic) and shutting down gracefully via `os/signal` + `Server.Shutdown`. No app server, no reverse-proxy requirement, no runtime on the host.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CONF-1 | Env vars: `os.Getenv`/`LookupEnv` (set-vs-empty), `Environ`, `Setenv`/`Unsetenv` | STD | os.txt — the idiomatic 12-factor config source |
| CONF-2 | `os.Expand`/`ExpandEnv` — `${var}` interpolation | STD | os.txt |
| CONF-3 | Flag-based config (+ FlagSet, TextVar for typed values) | STD | flag.txt — cross-ref CLI-9…11; no env-var fallback wiring — flag↔env bridging is DIY |
| CONF-4 | `.env` file loading | NO | godotenv et al. are ecosystem; std reads real environment only |
| CONF-5 | Config-file framework (YAML/TOML, per-env layering, validation, defaults) | NO | no YAML/TOML parser in std at all; DIY floor is `os.ReadFile` + encoding/json (encoding/json.txt); no `settings.py` analog |
| CONF-6 | Secrets management / encrypted credentials | NO | nothing like Rails credentials; env vars or external stores |
| CONF-7 | `//go:embed` — files/trees compiled into the binary (string, []byte, `embed.FS`) | TOOL | embed.txt — directive is a language/toolchain feature; patterns exclude `.`/`_` files; THE deploy story for templates/assets/migrations |
| CONF-8 | `embed.FS` implements `fs.FS` → `http.FileServer(http.FS(...))`, `template.ParseFS` | STD | embed.txt, io/fs.txt, net/http.txt — embedded assets served/parsed with zero glue |
| CONF-9 | Single static binary deployment | TOOL | toolchain — `CGO_ENABLED=0` yields a no-libc binary; scp/FROM-scratch container is the whole pipeline; no interpreter, vendor dir, or app server on the host |
| CONF-10 | Cross-compilation: `GOOS`/`GOARCH` env vars | TOOL | toolchain — build linux/arm64 from a mac laptop with no extra setup |
| CONF-11 | Build metadata: `debug.ReadBuildInfo` (module versions, VCS revision/time, build settings); `-ldflags -X` var injection | STD | runtime/debug.txt — version endpoints for free; TOOL for -ldflags |
| CONF-12 | Graceful shutdown trigger: `signal.Notify` / `signal.NotifyContext` (SIGINT/SIGTERM → ctx cancel) | STD | os/signal.txt — NotifyContext + `context.Cause` names the signal; Windows CTRL_CLOSE/LOGOFF/SHUTDOWN map to SIGTERM |
| CONF-13 | `http.Server.Shutdown(ctx)` — stop listeners, drain in-flight, deadline via ctx; `RegisterOnShutdown`; `Close()` hard stop | STD | net/http.txt — hijacked conns excluded (cross-ref LIVE-11); server unusable after Shutdown |
| CONF-14 | Production-grade server in-process: `ReadTimeout`/`ReadHeaderTimeout`/`WriteTimeout`/`IdleTimeout`, `MaxHeaderBytes`, `BaseContext`/`ConnContext`, `ErrorLog` | STD | net/http.txt — no gunicorn/puma/uvicorn analog needed; zero-value `Server` is valid (but timeout-less — safe defaults are on you) |
| CONF-15 | TLS serving: `ListenAndServeTLS`, `tls.Config` (`MinVersion`, `NextProtos`/ALPN, `ClientAuth`, `GetCertificate` per-SNI hook), `LoadX509KeyPair` | STD | crypto/tls.txt, net/http.txt — HTTP/2 enabled automatically over TLS |
| CONF-16 | Automatic certificates (Let's Encrypt/ACME) | X | golang.org/x/crypto/acme/autocert (x/, not in corpus) — plugs into `tls.Config.GetCertificate`; std has the hook, not the ACME client |
| CONF-17 | Protocol config: `Server.Protocols` (HTTP1/HTTP2/UnencryptedHTTP2), `HTTP2Config` | STD | net/http.txt — since 1.24; h2c without x/net/http2 for the first time |
| CONF-18 | Container-aware runtime defaults: GOMAXPROCS honors cgroup CPU quota + live updates; `GOMEMLIMIT`/`SetMemoryLimit` | STD | runtime.txt, runtime/debug.txt — cgroup-aware GOMAXPROCS **since 1.25** (`GODEBUG=containermaxprocs=0` for ≤1.24 behavior) |
| CONF-19 | Time zone handling without OS tzdata: `time.LoadLocation` + `time/tzdata` embed import | STD | time.txt — `LoadLocation` in corpus; the `time/tzdata` blank import (embeds the zone DB in the binary) from knowledge, not in corpus |
| CONF-20 | Well-known dirs: `os.UserConfigDir`/`UserHomeDir`/`UserCacheDir`, `Hostname` | STD | os.txt |
| CONF-21 | Process manager / systemd integration / socket activation | NO | no supervisor story; `Server.Serve(l)` accepts any `net.Listener` so fd-passing is possible, but the wiring is DIY |
| CONF-22 | Zero-downtime restart / listener handoff | NO | nothing; ecosystem (tableflip) or orchestrator-level rolling deploys |
| CONF-23 | Deployment checklist / prod-mode switch (`DEBUG=False`, `check --deploy`) | NO | no framework settings object, no environment notion, no config linting |

## P19 — Extensibility: DI, events, hooks & packages

**Problem.** Let apps hook into framework and each other's lifecycles, ship reusable components, and verify configuration. **Answer.** Go has no DI container, no signals, no app registry — the universal extension mechanism is the small, implicitly-satisfied interface, and the stdlib is a catalog of them: `http.Handler`, `fs.FS`, `sql/driver`, `slog.Handler`, `io.Reader/Writer`, `flag.Value`. Composition is import + constructor arguments by hand; distribution is go modules; registration is the `init()`+blank-import idiom; and the one runtime plugin mechanism (`plugin`) is so caveat-ridden it is effectively unused.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| EXT-1 | Implicit interface satisfaction — the extension seam itself | TOOL | language feature: any type matching the method set plugs in, no `implements` declaration, no registration |
| EXT-2 | `http.Handler`/`HandlerFunc` — the web composition contract | STD | net/http.txt — mux, middleware, reverse proxy, app: all one interface; cross-ref CTRL-1/2 |
| EXT-3 | `http.RoundTripper` — client-side middleware seam (retry, auth, instrumentation transports) | STD | net/http.txt — wrap `Transport` the way handlers wrap handlers |
| EXT-4 | `fs.FS` capability interfaces: `FS`, `ReadDirFS`, `ReadFileFS`, `StatFS`, `GlobFS`, `SubFS`, `ReadLinkFS` | STD | io/fs.txt — one Open method mandatory, everything else optional-with-fallback; embed.FS, os.DirFS, zip, fstest.MapFS all interchange |
| EXT-5 | `io.Reader`/`Writer`/`Closer` (+ combinators `LimitReader`, `TeeReader`, `MultiWriter`, `Pipe`) | STD | io.txt — the streaming lingua franca; wrapping-as-composition |
| EXT-6 | Database driver seam: `database/sql/driver` interfaces + `sql.Register` | STD | database/sql/driver.txt, database/sql.txt — Connector/DriverContext, ExecerContext/QueryerContext, NamedValueChecker for custom types; drivers self-register in `init()` |
| EXT-7 | `init()` + blank-import registration idiom | TOOL | language feature — how sql drivers, image codecs, expvar and net/http/pprof plug in; the closest std thing to a plugin registry, with global-state tradeoffs |
| EXT-8 | Logging backend seam: `slog.Handler` (Enabled/Handle/WithAttrs/WithGroup); `DiscardHandler`; `NewMultiHandler` fan-out | STD | log/slog.txt — DiscardHandler since 1.24, MultiHandler new in the 1.26 corpus; cross-ref OBS-3 |
| EXT-9 | `slog.LogValuer` — types define their own log representation (redaction, grouping) | STD | log/slog.txt — `Value.Resolve` handles nesting safely |
| EXT-10 | Serialization seams: `json.Marshaler`/`Unmarshaler`, `encoding.TextMarshaler`/`BinaryMarshaler` (+ Append variants) | STD | encoding/json.txt, encoding.txt-family — how domain types opt into JSON/text/binary formats framework-wide (flag.TextVar, slog.Level, time.Time all ride these) |
| EXT-11 | Error protocol: `error` interface + `errors.Is/As/Join`, `%w` wrapping | STD | errors.txt, fmt.txt — extensible error taxonomies without a base-class hierarchy |
| EXT-12 | `flag.Value`/`Getter` — custom CLI value types | STD | flag.txt |
| EXT-13 | Struct tags as declarative metadata (`json:"..."`, `db:"..."`) | TOOL | language feature + reflect.StructTag — the substrate every Go ORM/validator/config-binder builds its "declarative" layer on; no std validation/binding consumes them beyond encoding |
| EXT-14 | `context.WithValue` — request-scoped injection channel | STD | context.txt — unexported-key + typed-accessor pattern; the de-facto middleware→handler DI mechanism (and its scope limit) |
| EXT-15 | `httputil.ReverseProxy` hooks: `Rewrite`/`Director`, `ModifyResponse`, `ErrorHandler`, `Transport` | STD | net/http/httputil.txt — a gateway/edge extension surface in std |
| EXT-16 | Go modules — packaging, versioning (semver), distribution (`go get`), vendoring | TOOL | toolchain — the "reusable app" story is just a module exposing constructors; no install hooks, no auto-discovery |
| EXT-17 | `plugin` package — load `.so` at runtime | STD | std but **not in corpus**; Linux/macOS only, requires exact toolchain+dependency version match, no unload — docs themselves steer users to other patterns; effectively a dead end |
| EXT-18 | DI container / service locator / auto-wiring | NO | constructor injection by hand; wire (codegen) and fx (runtime) are ecosystem |
| EXT-19 | Event bus / signals / lifecycle hooks (`request_started`, `pre_save`…) | NO | channels are the primitive; no framework events, no receiver registry, no dispatch_uid |
| EXT-20 | App/component registry with `ready()` lifecycle (Django AppConfig analog) | NO | package `init()` ordering is the only lifecycle; no introspectable registry of installed components |
| EXT-21 | Config/system-check framework (lint app wiring at boot) | NO | go vet checks code, nothing checks runtime configuration |

## P20 — Observability

**Problem.** Know what the running application is doing and hear about failures. **Answer.** Unusually strong for a standard library: `log/slog` for structured leveled logging with pluggable handlers, `expvar` + `runtime/metrics` for metrics, always-on production profiling over HTTP via pprof, and `runtime/trace` with a flight recorder for capturing the moments before an incident. The gap is at the boundary of the process: nothing exports any of it — no Prometheus/OTLP format, no distributed tracing, no error-reporting service, not even a request-log middleware.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| OBS-1 | `log/slog` structured logging: `Logger`, message+level+key/value attrs; levels Debug/Info/Warn/Error (any int works) | STD | log/slog.txt — since 1.21; default logger + top-level funcs |
| OBS-2 | Built-in handlers: `TextHandler` (key=value), `JSONHandler` (NDJSON) + `HandlerOptions` (`Level`, `AddSource`, `ReplaceAttr`) | STD | log/slog.txt — ReplaceAttr enables field renaming/redaction/time-format policy |
| OBS-3 | `slog.Handler` interface (custom sinks); `DiscardHandler`; `NewMultiHandler` fan-out to several handlers | STD | log/slog.txt — Handler is the OTel/file/syslog bridge point; DiscardHandler 1.24; MultiHandler new in the 1.26 corpus |
| OBS-4 | Dynamic log level: `LevelVar` (goroutine-safe, flip at runtime) | STD | log/slog.txt — SIGHUP-to-debug patterns without restart |
| OBS-5 | Contextual loggers: `Logger.With` (pre-bound attrs), `WithGroup`, `slog.Group`/`GroupAttrs` | STD | log/slog.txt — per-request logger with request-id via With is the idiom |
| OBS-6 | Context-aware calls: `InfoContext`/`Log(ctx,…)` — ctx passed to Handler (trace-ID extraction point) | STD | log/slog.txt — note: std handlers ignore ctx; correlating logs to traces is your Handler's job |
| OBS-7 | `LogValuer` — lazy/redacted value rendering | STD | log/slog.txt — secrets redaction at the type level |
| OBS-8 | Performance path: `LogAttrs` (zero-alloc), `Record`/`NewRecord`/`Clone` for middleware handlers, wrapping guidance (pc/source) | STD | log/slog.txt |
| OBS-9 | Old↔new bridge: `slog.SetDefault` retargets `log.Printf`; `NewLogLogger` makes a `*log.Logger` backed by a Handler; `SetLogLoggerLevel` | STD | log/slog.txt — lets `http.Server.ErrorLog` feed structured logs |
| OBS-10 | Legacy `log` package (+ `log/syslog`) | STD | log.txt, log/syslog.txt — syslog is frozen, unavailable on Windows; slog supersedes both |
| OBS-11 | Request/access logging middleware (combined log format, latency, status) | NO | hand-rolled ResponseWriter-wrapping middleware; every Go shop rewrites it |
| OBS-12 | `expvar` — process metrics as JSON at `/debug/vars`: `Int`/`Float`/`Map`/`String`/`Func`, `Publish` | STD | expvar.txt — auto-publishes `cmdline` + `memstats`; self-registers on DefaultServeMux (GET-only since 1.22); bespoke format, nothing scrapes it natively |
| OBS-13 | `runtime/metrics` — stable, self-describing runtime metric registry (GC, heap, `/sched/latencies`, pause histograms) | STD | runtime/metrics.txt — `All()` descriptions + `Read(samples)`; supersedes ReadMemStats for new code |
| OBS-14 | Runtime introspection: `runtime.NumGoroutine`, `ReadMemStats`, `GOMAXPROCS` | STD | runtime.txt |
| OBS-15 | `runtime/pprof` — CPU/heap/allocs/goroutine/block/mutex/threadcreate profiles, custom `Profile` types, `WriteHeapProfile` | STD | runtime/pprof.txt — block/mutex need `runtime.SetBlockProfileRate`/`SetMutexProfileFraction` (runtime.txt) |
| OBS-16 | Profiler labels: `pprof.Do`/`WithLabels`/`Labels` — tag samples with request-scoped keys | STD | runtime/pprof.txt — per-route/per-tenant CPU attribution via context |
| OBS-17 | `net/http/pprof` — `/debug/pprof/` endpoints (index, profile?seconds=, heap?gc=, delta profiles, trace) for `go tool pprof` | STD | net/http/pprof.txt — production-safe live profiling by blank import; GET-only since 1.22; **must be access-gated by you** |
| OBS-18 | `runtime/trace` — execution tracer (`Start`/`Stop`) + user annotations `NewTask`/`StartRegion`/`Log` viewed in `go tool trace` | STD | runtime/trace.txt — request-scoped tasks/regions ride context |
| OBS-19 | `trace.FlightRecorder` — always-on in-memory trace ring; `WriteTo` snapshots the last seconds on an incident | STD | runtime/trace.txt — **new 1.25**; `FlightRecorderConfig{MinAge,MaxBytes}`; one recorder per process |
| OBS-20 | `runtime/debug`: `ReadBuildInfo` (version endpoint), `ReadGCStats`, `Stack`/`PrintStack`, `SetCrashOutput` (crash-dump side channel), `SetMemoryLimit`/`SetGCPercent` | STD | runtime/debug.txt — SetCrashOutput enables DIY crash reporters |
| OBS-21 | `httptrace.ClientTrace` — hook DNS/connect/TLS/first-byte events of *outgoing* requests | STD | net/http/httptrace.txt — client-side only; no server-side twin |
| OBS-22 | Wire debugging: `httputil.DumpRequest`/`DumpRequestOut`/`DumpResponse` | STD | net/http/httputil.txt |
| OBS-23 | Metrics export: Prometheus/OpenMetrics/OTLP/statsd formats or push | NO | expvar's JSON is nonstandard; runtime/metrics has no exporter — client_golang/OTel SDK are ecosystem; the biggest observability gap |
| OBS-24 | Distributed tracing: W3C traceparent propagation, spans, OTel API | NO | nothing in std touches trace context; slog/pprof/trace are all process-local |
| OBS-25 | Error tracking/aggregation & alerting (Sentry/error-email analog) | NO | no `mail_admins`, no error reporter; panics hit `ErrorLog` and that's it |
| OBS-26 | Health/readiness check conventions | NO | trivially hand-written but unstandardized — no `/healthz` helper, no checker registry |

## P21 — Admin & operational UIs

**Problem.** Give staff a production-ready CRUD interface over the domain models without building one. **Answer.** Nothing. There is no admin site, no CRUD scaffolding, no ops dashboard anywhere in the standard library or toolchain — the only built-in web pages a Go binary can serve about itself are the developer debug indexes (`/debug/pprof/`, `/debug/vars`), which are diagnostics, not administration. This entire problem area is greenfield for Volt.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ADMIN-1 | Model-driven CRUD admin (register a model → list/search/filter/edit screens) | NO | no models to derive from (no ORM, P4) and no UI generator; Django admin's whole category is absent |
| ADMIN-2 | Staff auth, permissions and audit integration for an admin UI | NO | no auth/permission system in std to integrate (cross-ref P7); would be built from scratch |
| ADMIN-3 | Record history / change audit log | NO | nothing |
| ADMIN-4 | Operational dashboards (job queues, cache, connections) | NO | nothing to dashboard — std has no queue (P9) or cache (P12) to inspect; `sql.DBStats` and runtime/metrics provide raw numbers only |
| ADMIN-5 | Built-in diagnostic pages: `/debug/pprof/` index and `/debug/vars` | STD | net/http/pprof.txt, expvar.txt — the only self-served UIs in std; unauthenticated by default (they self-register on DefaultServeMux — gate or isolate them in production); operational, not administrative |
| ADMIN-6 | Admin scaffolding/codegen (generate CRUD screens from schema) | NO | raw materials exist (html/template, ServeMux, database/sql) but no generator, no conventions — everything Volt would have to invent |

## Signature design decisions

The choices that make the standard library what it is — visible on every page of the corpus, and the constraints any Go framework inherits:

1. **Small interfaces as the universal seam.** The extension mechanism everywhere is a one-to-four-method interface, satisfied implicitly: `http.Handler` makes controllers, middleware, muxes and reverse proxies literally the same type; `fs.FS` lets disk, embedded assets, zip archives and in-memory test fakes interchange behind a single `Open` method; `driver.Driver` and its companions let every database plug into (and every tracer wrap) the same pool; `io.Reader`/`io.Writer` are the streaming lingua franca that every codec, hash, template and copy helper speaks; `slog.Handler` makes the logging backend swappable. Where other stacks ship plugin registries and base classes, Go ships method sets — and any Volt seam will be judged against these.

2. **Batteries for protocols, not products.** The stdlib is complete at the wire level — a production HTTP/1.1+HTTP/2 server, TLS 1.3 with post-quantum key exchange on by default, SMTP, MIME in all its encodings, JSON/XML/CSV, cookies, multipart, gzip — and deliberately empty one level up: an SMTP client but no mailer, a JSON codec but no serializer layer, cookies but no sessions, SQL but no ORM. The line is consistent: if an RFC or WHATWG spec defines it, std implements it; the moment a product decision would be required (naming conventions, storage policy, opinionated defaults), std abstains.

3. **The compatibility promise shapes everything.** Programs written against Go 1 in 2012 still compile with go1.26.5. That guarantee explains the library's texture: additions are conservative (real ServeMux routing patterns landed in 1.22, thirteen years in), mistakes are frozen rather than fixed (`net/smtp` and `testing/quick` are officially frozen; `CloseNotifier` is deprecated but never removed), and revisions arrive alongside rather than instead (`encoding/json/v2` behind `GOEXPERIMENT`, explicitly exempt from the promise until it stabilizes). A framework built on these APIs inherits a foundation that will not shift underneath it — and a culture in which its own API stability will be expected.

4. **The single static binary is the deployment philosophy.** `go build` emits a self-contained artifact; `//go:embed` compiles templates, assets and migration files into it; `GOOS`/`GOARCH` cross-compile it from anywhere; `crypto/tls` + the in-process production server mean no app server, interpreter or reverse proxy is required on the host; `os/signal` + `Server.Shutdown` handle the lifecycle. The whole P18 deployment story is the toolchain — the one problem area where Go doesn't just supply primitives but delivers the finished product.

5. **Explicit error handling, no exception machinery.** Errors are ordinary values: `(T, error)` returns, sentinel values for control flow (`sql.ErrNoRows`, `fs.ErrNotExist`), taxonomy via `errors.Is`/`As`/`Join` and `%w` wrapping. There is no exception→error-page pipeline to hook because there are no exceptions; net/http's panic recovery exists only to stop one bad request from killing the process, not to render anything. A framework's error rendering, debug pages and validation-error shaping must all be built as ordinary value plumbing.

6. **Concurrency is a language feature, not a framework feature.** Goroutines, channels, `select` and `context` cancellation mean the stdlib never needed an async runtime, a worker abstraction or a WSGI/ASGI split: the HTTP server is goroutine-per-request, a blocked streaming handler costs one cheap goroutine, and "run this in the background" is `go f()`. What other frameworks sell as their concurrency story, Go gives away below the framework — leaving only durability (work that survives a restart) as framework territory.

7. **Struct tags are the reflection-lite metadata channel.** The language's one declarative mechanism — `json:"name,omitempty"`, `xml:"a>b,attr"` — lets a type carry field-level metadata without base classes, decorators or codegen. std itself consumes tags only in `encoding/*`, but the mechanism is why the ecosystem's ORMs (`db:`), validators (`validate:`) and config binders all converge on the same surface: Volt's model and validation layers have a ready-made, idiomatic place to declare themselves.

## The gap Volt fills

Read the `NO` rows of P1–P21 as a single list and they cluster into the framework Volt has to be. In every cluster the stdlib supplies a load-bearing primitive and stops exactly one level below the product (smaller point-gaps — named routes/reverse URLs, content negotiation, a writable `fs.FS` — thread through the sections alongside these):

- **ORM, relations & migrations (P4, P5).** `database/sql` delivers the pool, transactions and the `Scanner`/`Valuer` type bridge; struct tags and `//go:embed`-ed `.sql` files are the substrate for the model layer, query builder and migration engine that never came.
- **Validation framework (P6).** Strict one-value parsers (`strconv`, `time.Parse`, `mail.ParseAddress`, `netip`) and `errors.Join` exist; declarative rules, field-keyed error maps and request→struct binding do not.
- **Sessions, auth & token CSRF (P2, P7, P8).** `crypto/hmac`, `crypto/rand.Text`, AEAD and complete cookie plumbing are the bricks for signed sessions and tokens; there is no store, user model, login flow, password-hasher lifecycle or permission system.
- **WebSockets & real-time (P10).** `http.Flusher` makes SSE work today and `Hijacker` hands over the raw conn; RFC 6455 itself, broadcast hubs, presence and reconnect semantics are absent.
- **Durable jobs & scheduling (P9).** Goroutines, channels and `time.Ticker` are a richer in-process substrate than any framework's — and entirely volatile: no queue that survives a crash, no retry/backoff, no cron.
- **Mail composition, templates & queue (P11).** Frozen `net/smtp` plus every MIME encoding brick; no message builder, no attachment API, no pluggable/dev-mode backends, no outbox.
- **Cache framework (P12).** `sync.Map`/`Once`/`Pool`, `maphash` and built-in conditional GET (`ServeContent`) are the raw material; no unified backend API, no TTL/LRU store, no fragment or per-view caching.
- **i18n message catalogs (P15).** UTF-8-native strings, the Unicode tables and the IANA tz database are in std; catalogs, plural rules and locale-aware formatting sit in x/text with no extraction workflow or per-request locale pipeline.
- **Asset pipeline (P3).** `embed.FS` + `FileServer` ship raw files inside the binary; bundling, fingerprinting, transpilation and Accept-Encoding negotiation don't exist.
- **Scaffolding & codegen CLI (P17).** `flag.FlagSet`, `go generate` and `text/template` are the substrate; there is no project generator, no management-command registry, no watch mode.
- **Config loading (P18).** `os.Getenv` + `flag`, full stop: no `.env`, no config-file framework, no per-env layering, no secrets management.
- **DI & events (P19).** Implicit interfaces, the `init()`+blank-import idiom and `context.WithValue` are the seams; no container, no event bus/signals, no component registry with lifecycle.
- **Metrics export & tracing (P20).** `slog`, pprof, `runtime/metrics` and the flight recorder are world-class in-process; nothing exports Prometheus/OTLP and nothing propagates distributed trace context.
- **Admin (P21).** `html/template` + `ServeMux` + `database/sql` are the raw materials; there is no CRUD generator — and with no ORM, nothing to derive one from.

Top to bottom, that list is a fair first draft of Volt's table of contents.
