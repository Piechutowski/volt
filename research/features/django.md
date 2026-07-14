# Django — Feature Inventory

An exhaustive inventory of every capability documented in the official **Django 6.1.x**
documentation corpus, organized by problem solved. Derived from
`research/corpus/django/` (source: `github.com/django/django`, ref `stable/6.1.x`,
commit `bc529fb8a6c289e066f3aadc8a6944333e35f12f`, fetched 2026-07-14, 669 markdown files).
This is an inventory of what exists — research input for the Volt framework design, not a
build plan. It was assembled per-problem from section-level inventories of the corpus; the
section skeleton (P1–P21 + extras) is shared with the Laravel/Rails/Phoenix inventories so
rows can be aligned in a comparison matrix.

Tier legend:

- `CORE` — ships in Django's default install and is on by default (or one setting/import away) in a `startproject` skeleton.
- `OPT` — ships with Django but is opt-in. For Django this largely means `django.contrib.*` apps (add to `INSTALLED_APPS`), opt-in middleware, or opt-in settings/API parameters.
- `ECO` — requires a third-party package or external component that the docs name (psycopg/mysqlclient, Jinja2, Pillow, argon2-cffi, redis-py, Gunicorn/Daphne, task workers, django-storages…).
- `DIY` — a documented extension point or recipe you implement yourself (subclass, protocol, pattern).

---

## P1 — Routing & HTTP dispatch

**Problem.** Map incoming URLs to handler code, extract typed parameters, and generate URLs back out. **Answer.** The URLconf — ordinary Python modules holding `urlpatterns` lists, matched top-down against the path only (never the HTTP verb); `path()` with pluggable typed converters or `re_path()` regexes, arbitrarily nestable via `include()` with app/instance namespaces, and `reverse()`/`{% url %}` for generation. Verb dispatch happens in the view layer (P2).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ROUTE-1 | URLconf modules (`urlpatterns` sequence) | CORE | topics/http/urls.md — root from `ROOT_URLCONF`; matched against `request.path_info`; first match wins; pure Python, can be built dynamically |
| ROUTE-2 | Per-request URLconf override | OPT | topics/http/urls.md, ref/request-response.md — middleware sets `request.urlconf`; `None` reverts to `ROOT_URLCONF` |
| ROUTE-3 | `path(route, view, kwargs=None, name=None)` | CORE | ref/urls.md — angle-bracket captures `<conv:name>`; route may be `gettext_lazy` (translated URLs, see I18N-); view = function, `as_view()` result, or `include()` |
| ROUTE-4 | `re_path(route, view, kwargs=None, name=None)` | CORE | ref/urls.md — Python regex; named groups → kwargs, unnamed → positional, all values passed as str; trailing `$` uses `re.fullmatch` |
| ROUTE-5 | Built-in path converters `str`/`int`/`slug`/`uuid`/`path` | CORE | topics/http/urls.md — `str` is default; `int` returns int; `uuid` requires lowercase+dashes, returns UUID; `path` matches across `/` |
| ROUTE-6 | Custom path converters + `register_converter(converter, type_name)` | DIY | topics/http/urls.md, ref/urls.md — class with `regex` attr, `to_python()` (ValueError → try next pattern / 404), `to_url()` (ValueError → NoReverseMatch) |
| ROUTE-7 | Unnamed regex groups | OPT | topics/http/urls.md — discouraged; when mixed with named groups, unnamed are ignored |
| ROUTE-8 | Nested regex arguments | OPT | topics/http/urls.md — resolvable; reversing fills only outer captured args; recommend non-capturing `(?:...)` |
| ROUTE-9 | `include()` (module, pattern list, or `(pattern_list, app_namespace)` 2-tuple) | CORE | ref/urls.md, topics/http/urls.md — chops matched prefix, passes remainder; `namespace=` kwarg sets instance namespace; arbitrary nesting and prefix factoring |
| ROUTE-10 | Captured params flow into included URLconfs | CORE | topics/http/urls.md — parent captures passed to included views |
| ROUTE-11 | Extra options dict to views (`path(..., {"foo": "bar"})`) | OPT | topics/http/urls.md — dict kwargs win over same-name URL captures; used by syndication framework |
| ROUTE-12 | Extra options passed to `include()` | OPT | topics/http/urls.md — applied to *every* line in the included conf |
| ROUTE-13 | Default view arguments | OPT | topics/http/urls.md — multiple patterns → one view with Python default params |
| ROUTE-14 | Request method ignored by URLconf | CORE | topics/http/urls.md — GET/POST/HEAD all route to the same view; query string/domain not matched |
| ROUTE-15 | URL naming (`name=`) | CORE | topics/http/urls.md — any characters allowed; same name reusable if args differ; deliberate override of e.g. `login` by later pattern |
| ROUTE-16 | App/instance namespaces (`app_name`, `include(..., namespace=)`) | CORE | topics/http/urls.md — `'ns:name'` syntax, nestable (`'sports:polls:index'`); resolution order: current_app → default instance → last deployed → direct instance-ns lookup |
| ROUTE-17 | `reverse(viewname, urlconf, args, kwargs, current_app, *, query, fragment)` | CORE | ref/urlresolvers.md — args XOR kwargs; `query=` accepts QueryDict/urlencode-compatible, `fragment=` appends `#…`; raises `NoReverseMatch`; cannot reverse `\|` alternation; output already URL-quoted; callable view objects reversible (not namespaced ones) |
| ROUTE-18 | `reverse_lazy()` | CORE | ref/urlresolvers.md — for use before the URLconf loads (CBV `url` attrs, decorator args, function defaults) |
| ROUTE-19 | `resolve(path, urlconf=None)` | CORE | ref/urlresolvers.md — returns `ResolverMatch`; raises `Resolver404` (an `Http404` subclass); unpackable as `func, args, kwargs` |
| ROUTE-20 | `ResolverMatch` attributes | CORE | ref/urlresolvers.md — `func`, `args`, `kwargs`, `captured_kwargs`, `extra_kwargs`, `url_name`, `route`, `tried`, `app_name(s)`, `namespace(s)`, `view_name` |
| ROUTE-21 | `get_script_prefix()` | OPT | ref/urlresolvers.md — script-prefix portion of the project URL; only valid inside the request-response cycle |
| ROUTE-22 | `handler400/403/404/500` in root URLconf (custom error views) | DIY | topics/http/urls.md, ref/urls.md, topics/http/views.md — callables or dotted-path strings; root URLconf only; defaults `django.views.defaults.*`; custom views take `(request, exception)` (500: `request` only); testing pattern with `@override_settings(ROOT_URLCONF=__name__)`; CSRF error view overridden separately via `CSRF_FAILURE_VIEW` |
| ROUTE-23 | URLconf compilation caching | CORE | topics/http/urls.md — patterns compiled on first access, cached by the resolver |
| ROUTE-24 | contrib.redirects: `Redirect` model + `RedirectFallbackMiddleware` | OPT | ref/contrib/redirects.md, ref/middleware.md — site_id/old_path/new_path; last-resort 404 fallback → 301 (410 on empty new_path); overridable `response_gone_class`/`response_redirect_class`; requires contrib.sites; place near bottom of `MIDDLEWARE` |

## P2 — Request handling: controllers & middleware

**Problem.** Give handler code structured access to request data (input, headers, cookies, files), build responses, compose cross-cutting behavior around handlers, and carry per-user state across requests. **Answer.** Views are plain callables (`HttpRequest → HttpResponse`) — no controller class required; a middleware onion configured centrally by the `MIDDLEWARE` setting; class-based views layer reusable generic handlers (list/detail/edit/date archives) on top; contrib sessions and messages carry request state; sync and async views are both first-class citizens.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CTRL-1 | `HttpRequest.scheme` / `.method` / `.path` / `.path_info` | CORE | ref/request-response.md — `method` guaranteed uppercase; `path_info` excludes script prefix (portable across deployments) |
| CTRL-2 | `HttpRequest.body` | CORE | ref/request-response.md — raw bytestring; `RawPostDataException` if accessed after `read()`/`readline()` |
| CTRL-3 | `HttpRequest.GET` / `.POST` | CORE | ref/request-response.md — QueryDicts; `POST` excludes file uploads; check `request.method == "POST"`, not truthiness of `POST` |
| CTRL-4 | `HttpRequest.COOKIES` / `.FILES` | CORE | ref/request-response.md — `FILES` maps input name → `UploadedFile`; populated only for multipart POST |
| CTRL-5 | `HttpRequest.META` | CORE | ref/request-response.md — CGI-style vars; headers become `HTTP_*` keys (upper, `-`→`_`); runserver strips underscore headers (anti-spoofing) |
| CTRL-6 | `HttpRequest.headers` | CORE | ref/request-response.md — case-insensitive dict-like; title-cased display; underscore lookup for templates; includes Content-Length/Content-Type |
| CTRL-7 | `HttpRequest.encoding` (writable) / `.content_type` / `.content_params` | CORE | ref/request-response.md — encoding override affects subsequent GET/POST reads; defaults to `DEFAULT_CHARSET` |
| CTRL-8 | `HttpRequest.resolver_match` | CORE | ref/request-response.md — `ResolverMatch`; unavailable in pre-resolution middleware (OK in `process_view`) |
| CTRL-9 | `HttpRequest.multipart_parser_class` | OPT | ref/request-response.md — **6.1** — settable custom `MultiPartParser` per middleware/view |
| CTRL-10 | App-set attrs: `current_app`, `urlconf`, `exception_reporter_filter`, `exception_reporter_class` | OPT | ref/request-response.md — hooks read by `{% url %}` / URL resolver / error reporting |
| CTRL-11 | Middleware-set attrs: `session`, `site`, `user` | CORE | ref/request-response.md — from Session/CurrentSite/Authentication middleware respectively |
| CTRL-12 | `HttpRequest.auser()` | CORE | ref/request-response.md — coroutine variant of `.user` for async contexts |
| CTRL-13 | `get_host()` / `get_port()` | CORE | ref/request-response.md — honor `X-Forwarded-Host/Port` when `USE_X_FORWARDED_HOST/PORT` on; raises `DisallowedHost` vs `ALLOWED_HOSTS`; multi-proxy needs rewrite middleware (documented pattern) |
| CTRL-14 | `get_full_path()` / `get_full_path_info()` | CORE | ref/request-response.md — path (or path_info) + query string |
| CTRL-15 | `build_absolute_uri(location=None)` | CORE | ref/request-response.md — absolute URI from request; always keeps current scheme |
| CTRL-16 | `get_signed_cookie(key, default, salt, max_age)` | OPT | ref/request-response.md — `BadSignature`/`SignatureExpired` unless `default` given; `SIGNED_COOKIE_LEGACY_SALT_FALLBACK=False` rejects pre-5.2.15 signatures (5.2.15 fixed (key, salt) pair ambiguity) |
| CTRL-17 | `is_secure()` | CORE | ref/request-response.md — True for HTTPS |
| CTRL-18 | `accepts(mime_type)` | CORE | ref/request-response.md — matches `Accept` header; `*/*` (browser default) → True for everything |
| CTRL-19 | `get_preferred_type(media_types)` | CORE | ref/request-response.md — content negotiation per RFC 9110 incl. media-type parameters and q-values; `None` if nothing acceptable; pair with `vary_on_headers('Accept')` when caching |
| CTRL-20 | File-like request reading: `read()`, `readline()`, `readlines()`, `__iter__()` | CORE | ref/request-response.md — stream the request body (e.g. feed directly to `ElementTree.iterparse`) |
| CTRL-21 | `QueryDict` multi-value dict | CORE | ref/request-response.md — request GET/POST are immutable; `copy()` for mutable deep copy; `mutable=True` on own instances |
| CTRL-22 | `QueryDict` API | CORE | ref/request-response.md — `__getitem__` returns *last* value (`MultiValueDictKeyError`, a `KeyError`); `fromkeys()`, `get`, `setdefault`, `update` (appends), `items`/`values` (last-value iterators), `getlist(default)`, `setlist`, `appendlist`, `setlistdefault`, `lists()`, `pop` (list), `popitem`, `dict()`, `urlencode(safe=)` |
| CTRL-23 | `HttpResponse(content, content_type, status, reason, charset, headers)` | CORE | ref/request-response.md — content as str/bytes/memoryview/iterator (iterator consumed eagerly, closables closed); `http.HTTPStatus` usable for `status` |
| CTRL-24 | `HttpResponse.headers` + dict-style header access | CORE | ref/request-response.md — `response["Age"] = 120` proxies to `.headers`; `del` never raises; `headers=` at init; newline in header → `BadHeaderError`; use `patch_cache_control`/`patch_vary_headers` for multi-valued headers |
| CTRL-25 | Response attributes: `content`, `text`, `cookies`, `charset`, `status_code`, `reason_phrase`, `streaming` (False), `closed` | CORE | ref/request-response.md — `text` decodes via `charset`; `status_code` change syncs `reason_phrase` |
| CTRL-26 | Header methods: `__setitem__`/`__getitem__`/`__delitem__`, `get`, `has_header`, `items`, `setdefault` | CORE | ref/request-response.md — case-insensitive |
| CTRL-27 | `set_cookie(key, value, max_age, expires, path, domain, secure, httponly, samesite)` | CORE | ref/request-response.md — `max_age` accepts timedelta; `samesite` Strict/Lax/None; 4096-byte browser limit not enforced by Django |
| CTRL-28 | `set_signed_cookie(...)` / `delete_cookie(...)` | CORE | ref/request-response.md — signed variant pairs with `request.get_signed_cookie` (same `salt`); delete must match original path/domain |
| CTRL-29 | Response file/stream-like API: `write`, `flush`, `tell`, `getvalue`, `writelines`, `writable`, `readable`, `seekable`, `close` | CORE | ref/request-response.md — incremental content building; `close()` called by the WSGI/ASGI server |
| CTRL-30 | Content-Disposition attachment pattern | CORE | ref/request-response.md — set Content-Type + Content-Disposition headers |
| CTRL-31 | `render()`-method emulation on custom responses | DIY | ref/request-response.md — custom HttpResponse subclass with `render` treated as SimpleTemplateResponse-alike |
| CTRL-32 | `HttpResponseRedirect` | CORE | ref/request-response.md — 302; `preserve_request=True` → 307; `max_length=` override/disable of redirect-URL length check (**6.1**); read-only `.url` |
| CTRL-33 | `HttpResponsePermanentRedirect` | CORE | ref/request-response.md — 301; 308 with `preserve_request=True` |
| CTRL-34 | `HttpResponseNotModified` | CORE | ref/request-response.md — 304; no args, no content |
| CTRL-35 | `HttpResponseBadRequest` / `NotFound` / `Forbidden` / `NotAllowed` / `Gone` / `ServerError` | CORE | ref/request-response.md — 400/404/403/405/410/500; `NotAllowed` requires permitted-methods list |
| CTRL-36 | Custom response classes | DIY | ref/request-response.md — subclass with `status_code = HTTPStatus.X` |
| CTRL-37 | `JsonResponse(data, encoder, safe, json_dumps_params)` | CORE | ref/request-response.md — `application/json`; dict-only unless `safe=False`; default `DjangoJSONEncoder`, swappable |
| CTRL-38 | `StreamingHttpResponse` | OPT | ref/request-response.md — sync iterator under WSGI, async iterator under ASGI (mismatched type adapted with warning by full consumption); `streaming_content`, `is_async`, `streaming=True`; no `content`/`text`/`tell()`/`write()`; ETag/Content-Length middleware can't apply; WSGI ties a worker for the duration, ASGI enables long-poll/SSE (see LIVE-) |
| CTRL-39 | `FileResponse(open_file, as_attachment, filename)` | CORE | ref/request-response.md — StreamingHttpResponse subclass; uses `wsgi.file_wrapper` when available; auto Content-Length/Type/Disposition (`set_headers()`); closes the file itself; `seek()` BytesIO yourself |
| CTRL-40 | Async file streaming under ASGI | ECO | ref/request-response.md — Python file API is sync; docs name third-party `aiofiles` |
| CTRL-41 | `HttpResponseBase` | CORE | ref/request-response.md — common base of HttpResponse & StreamingHttpResponse; for type-checking only |
| CTRL-42 | Function-based views | CORE | topics/http/views.md — any callable taking `HttpRequest`, returning `HttpResponse`; location-agnostic (`views.py` by convention) |
| CTRL-43 | Arbitrary status via `HttpResponse(status=...)` | CORE | topics/http/views.md — for codes with no dedicated subclass |
| CTRL-44 | `Http404` exception | CORE | topics/http/views.md — caught by framework → standard 404 page; custom `404.html` served when `DEBUG=False`; message shown only in debug 404 |
| CTRL-45 | Default error views (`django.views.defaults.*`) | CORE | ref/views.md — `page_not_found` (context: `request_path`, `exception`; RequestContext), `server_error` (empty context), `permission_denied` (`PermissionDenied` → 403.html), `bad_request` (`BadRequest`/unhandled `SuspiciousOperation` → 400, no exception info in context); all bypassed when `DEBUG=True`; override via ROUTE-22 |
| CTRL-46 | Async views (`async def`) | CORE | topics/http/views.md, topics/async.md — auto-detected via `inspect.iscoroutinefunction` (use `markcoroutinefunction` for custom coroutine factories); need ASGI for real benefits |
| CTRL-47 | `render(request, template_name, context, content_type, status, using)` | CORE | topics/http/shortcuts.md — template + context → HttpResponse; `template_name` may be a sequence (first existing wins) |
| CTRL-48 | `redirect(to, *args, permanent=False, preserve_request=False, max_length=..., **kwargs)` | CORE | topics/http/shortcuts.md — `to` = model (`get_absolute_url`), view name (reverse), or URL; status matrix 301/302/307/308; `max_length` (**6.1**) |
| CTRL-49 | `resolve_url(to, *args, **kwargs)` | CORE | topics/http/shortcuts.md — normalizes model/view-name/URL to a URL string; used internally by `redirect` |
| CTRL-50 | `get_object_or_404()` / `aget_object_or_404()` | CORE | topics/http/shortcuts.md — Model/Manager/QuerySet + Q objects/lookups; `Http404` instead of `DoesNotExist`; `MultipleObjectsReturned` still propagates; async variant |
| CTRL-51 | `get_list_or_404()` / `aget_list_or_404()` | CORE | topics/http/shortcuts.md — `filter()` cast to list, `Http404` if empty; async variant |
| CTRL-52 | `require_http_methods([...])` | CORE | topics/http/decorators.md — uppercase method names; violation → `HttpResponseNotAllowed` (405) |
| CTRL-53 | `require_GET` / `require_POST` / `require_safe` | CORE | topics/http/decorators.md — `require_safe` allows GET+HEAD (preferred over `require_GET` for link checkers) |
| CTRL-54 | `condition(etag_func=None, last_modified_func=None)` | CORE | topics/http/decorators.md, topics/conditional-view-processing.md — early bailout before the view runs; handles If-(None-)Match / If-(Un)Modified-Since → 304 or 412; funcs receive request + view args; sets validator headers only for GET/HEAD; keep `vary_on_*`/`cache_control` decorators *above* it; usable for POST/PUT/DELETE optimistic-concurrency checks |
| CTRL-55 | `etag(etag_func)` / `last_modified(func)` | CORE | topics/conditional-view-processing.md — single-condition shortcuts; do **not** chain both (use `condition`) |
| CTRL-56 | `conditional_page()` | CORE | topics/http/decorators.md — per-view equivalent of ConditionalGetMiddleware (see CACHE-) |
| CTRL-57 | `gzip_page()` | OPT | topics/http/decorators.md — per-view gzip; sets `Vary: Accept-Encoding`; middleware counterpart GZipMiddleware in CACHE- |
| CTRL-58 | `no_append_slash()` | OPT | topics/http/decorators.md — exempts a view from `APPEND_SLASH` rewriting |
| CTRL-59 | `method_decorator(decorator, name='')` | CORE | ref/utils.md — adapt function decorators to CBV methods/classes; accepts list/tuple |
| CTRL-60 | `decorator_from_middleware(_with_args)` | OPT | ref/utils.md — turn (old-style) middleware into a per-view decorator; e.g. `cache_page` built this way |
| CTRL-61 | All HTTP decorators sync+async compatible | CORE | topics/async.md — full documented list incl. csrf_*, csp_*, xframe_*, sensitive_variables/post_parameters |
| CTRL-62 | Middleware factory protocol | DIY | topics/http/middleware.md — callable taking `get_response`, returning request→response callable; function or callable-class style; lives anywhere on the Python path |
| CTRL-63 | Activation & ordering via `MIDDLEWARE` setting | CORE | topics/http/middleware.md — dotted-path strings; top-down on request, reverse on response ("onion"); can be empty (CommonMiddleware strongly suggested) |
| CTRL-64 | Middleware short-circuiting | CORE | topics/http/middleware.md — a layer returning without calling `get_response` skips inner layers and the view |
| CTRL-65 | `__init__(get_response)` once at server start | CORE | topics/http/middleware.md — no other args allowed; place for one-time state |
| CTRL-66 | `MiddlewareNotUsed` | OPT | topics/http/middleware.md — raise in `__init__` to remove middleware at startup; debug-logged when `DEBUG=True` |
| CTRL-67 | `process_view(request, view_func, view_args, view_kwargs)` | DIY | topics/http/middleware.md — pre-view hook; return response to skip the view; reading `request.POST` here blocks upload-handler modification |
| CTRL-68 | `process_exception(request, exception)` | DIY | topics/http/middleware.md — view exceptions only; run in reverse order; response short-circuits outer exception middleware |
| CTRL-69 | `process_template_response(request, response)` | DIY | topics/http/middleware.md — fires when the response has `render()`; may swap `template_name`/`context_data`; auto-rendered afterwards |
| CTRL-70 | Streaming-aware middleware | DIY | topics/http/middleware.md — test `response.streaming`, wrap `streaming_content` in a generator (never consume); match sync/async iterator via `is_async` |
| CTRL-71 | Automatic exception→response conversion between layers | CORE | topics/http/middleware.md — every `get_response` call returns *some* HttpResponse; disable with `DEBUG_PROPAGATE_EXCEPTIONS=True` |
| CTRL-72 | Sync/async capability flags (`sync_capable`, `async_capable`) | DIY | topics/http/middleware.md — Django adapts mismatched middleware (perf penalty); hybrid detects mode via `iscoroutinefunction(get_response)`; hooks adapted individually if not matched |
| CTRL-73 | `sync_only_middleware` / `async_only_middleware` / `sync_and_async_middleware` decorators | DIY | topics/http/middleware.md, ref/utils.md — flag factory functions; async-only gets wrapped in an event loop under WSGI |
| CTRL-74 | Async class middleware coroutine marking | DIY | topics/http/middleware.md — call `markcoroutinefunction(self)` in `__init__` when `get_response` is a coroutine fn; hybrid middleware may still be called sync between sync neighbors (Django minimizes transitions) |
| CTRL-75 | `MiddlewareMixin` (pre-1.10 style upgrade) | OPT | topics/http/middleware.md — maps `process_request`/`process_response` onto the new protocol; documented behavioral diffs vs `MIDDLEWARE_CLASSES` |
| CTRL-76 | Recommended ordering of built-in middleware | CORE | ref/middleware.md — 15-point ordering guidance (Security top, cache pair around Vary-modifiers, CSRF before auth, fallback middleware last, CSP near bottom after nonce users) |
| CTRL-77 | `SessionMiddleware` | CORE | ref/middleware.md — enables `request.session` (detail in CTRL-104…111) |
| CTRL-78 | `CommonMiddleware` | CORE | ref/middleware.md — `DISALLOWED_USER_AGENTS` blocking; `APPEND_SLASH` (redirect if slashed URL resolves; opt-out per view via `no_append_slash`); `PREPEND_WWW`; sets Content-Length on non-streaming responses; `response_redirect_class` overridable (default permanent redirect) |
| CTRL-79 | Async views under WSGI or ASGI | CORE | topics/async.md — work under WSGI in a one-off event loop (small adaptation cost, no long-lived requests); ASGI needed for hundreds of connections, slow streaming, long-polling |
| CTRL-80 | Coroutine detection | CORE | topics/async.md — `inspect.iscoroutinefunction`; custom coroutine-returning callables must `markcoroutinefunction` |
| CTRL-81 | Async-capable decorator set | CORE | topics/async.md — all cache/common/csp/csrf/debug/gzip/http/vary/clickjacking view decorators work on both sync and async views |
| CTRL-82 | Async ORM / other async APIs | CORE | topics/async.md — `a`-prefixed QuerySet methods, `async for`; async model methods (`asave`, `acreate`, `aset`); transactions not yet async (wrap sync fn); disable `CONN_MAX_AGE` in async mode, use backend pooling (detail in ORM-) |
| CTRL-83 | End-to-end async middleware stack | CORE | topics/async.md — sync middleware between an ASGI server and an async view runs in its own thread; adapted middleware logged on the `django.request` debug logger |
| CTRL-84 | Sync/async adaptation performance model | CORE | topics/async.md — tens of µs in-request (loop reused) vs hundreds of µs cold-start; Django minimizes transitions (single switch if all-sync under ASGI); move tight loops inside one `sync_to_async` crossing |
| CTRL-85 | Async safety (`SynchronousOnlyOperation`) | CORE | topics/async.md — async-unsafe parts (sync ORM etc.) refuse to run in a thread with a running event loop; applies to sync fns called from async without adapter; Jupyter/IPython impose a loop (`%autoawait off`) |
| CTRL-86 | `DJANGO_ALLOW_ASYNC_UNSAFE` env var | OPT | topics/async.md — disables the guard; data-corruption warning; not for production |
| CTRL-87 | `async_to_sync(fn, force_new_loop=False)` | CORE | topics/async.md — asgiref; reuses the current-thread loop or spins one up; preserves threadlocals/contextvars; enables `thread_sensitive` mode below it |
| CTRL-88 | `sync_to_async(fn, thread_sensitive=True)` | CORE | topics/async.md — default runs all thread-sensitive fns on one thread (main thread under `async_to_sync`); `False` = fresh thread per call; per-request worker thread → concurrent requests don't serialize; never pass DB `connection` attrs across the boundary |
| CTRL-89 | `asgiref` bundled dependency | CORE | topics/async.md — asgiref is a Django-project package, auto-installed |
| CTRL-90 | Base `View` class (`as_view`, `setup`, `dispatch`, `http_method_not_allowed`, `options`, `http_method_names`) | CORE | ref/class-based-views/base.md — import from `django.views`; verb dispatch by method name |
| CTRL-91 | Async class-based views | CORE | ref/class-based-views/base.md, topics/async.md — declare handlers (`get`/`post`) `async def`, not `__init__`/`as_view()`; `as_view()` marks the coroutine; no mixing sync/async handlers (`ImproperlyConfigured`); `method_decorator` on `dispatch` requires an `async def dispatch` override |
| CTRL-92 | `TemplateView` (template_name, get_context_data, extra_context) | CORE | ref/class-based-views/base.md, topics/class-based-views/generic-display.md |
| CTRL-93 | `RedirectView` (`url`/`pattern_name`/`permanent`/`query_string`/`preserve_request`, get_redirect_url) | CORE | ref/class-based-views/base.md — `preserve_request` (307/308) added **6.1** |
| CTRL-94 | `ListView` (model/queryset, `paginate_by`, `context_object_name`, `object_list`, get_queryset) | CORE | topics/class-based-views/generic-display.md, ref/class-based-views/flattened-index.md |
| CTRL-95 | `DetailView` (pk/slug lookup, get_object, `<model>_detail.html`) | CORE | topics/class-based-views/generic-display.md, mixins.md |
| CTRL-96 | `FormView` (form_class/initial/success_url, form_valid/form_invalid) | CORE | topics/class-based-views/generic-editing.md |
| CTRL-97 | `CreateView` / `UpdateView` (auto ModelForm, `fields`, `_form.html`, get_absolute_url success) | CORE | topics/class-based-views/generic-editing.md, flattened-index.md |
| CTRL-98 | `DeleteView` (DeletionMixin, `_confirm_delete.html`, reverse_lazy success_url) | CORE | topics/class-based-views/generic-editing.md |
| CTRL-99 | Date-based archive views: `ArchiveIndexView`/`YearArchiveView`/`MonthArchiveView`/`WeekArchiveView`/`DayArchiveView`/`TodayArchiveView`/`DateDetailView` (+ `Base*` non-template variants) | CORE | ref/class-based-views/generic-date-based.md — `date_field`, `allow_future`, `make_object_list`, week_format `%U`/`%W`/`%V` |
| CTRL-100 | View mixins: `ContextMixin`, `TemplateResponseMixin`, `SingleObjectMixin`/`SingleObjectTemplateResponseMixin`, `MultipleObjectMixin`/`MultipleObjectTemplateResponseMixin` | CORE | topics/class-based-views/mixins.md, ref flattened-index.md — `render_to_response`, `get_template_names`, `template_name_suffix` |
| CTRL-101 | Editing/date mixins: `FormMixin`, `ModelFormMixin`, `ProcessFormView`, `DeletionMixin`, `DateMixin`/`YearMixin`/`MonthMixin`/`WeekMixin`/`DayMixin` | CORE | ref/class-based-views/flattened-index.md, generic-date-based.md |
| CTRL-102 | CBV hook methods (`get_queryset`, `get_object`, `get_context_data`, `get_form_class`) | CORE | topics/class-based-views/generic-display.md — the customization surface |
| CTRL-103 | Decorating CBVs (`method_decorator` on dispatch, `login_required`/`permission_required` in URLconf) | CORE | topics/class-based-views/intro.md |
| CTRL-104 | Combining-mixins patterns (SingleObjectMixin+View/ListView, FormMixin+DetailView, dual as_view() dispatch) | DIY | topics/class-based-views/mixins.md — documented cautions/MRO |
| CTRL-105 | Custom response mixins (`JSONResponseMixin`, content negotiation via `get_preferred_type`) | DIY | topics/class-based-views/mixins.md, generic-editing.md |
| CTRL-106 | Session backends via `SESSION_ENGINE`: `db` (default), `cache`, `cached_db`, `file`, `signed_cookies` | OPT | topics/http/sessions.md — `django.contrib.sessions` + `SessionMiddleware` |
| CTRL-107 | `request.session` dict-like `SessionBase` API (get/set/pop/update/keys/items + `a*` async, `__bool__` **6.1**) | OPT | topics/http/sessions.md |
| CTRL-108 | Session lifecycle methods `flush`/`aflush`, `clear`, `cycle_key` (session-fixation mitigation), `set_expiry`/`get_expiry_*`, `set_test_cookie` helpers | OPT | topics/http/sessions.md |
| CTRL-109 | `SessionStore` out-of-view API `create`/`save`/`delete`/`exists`/`load`/`clear_expired` (+ async); `Session` model + `get_decoded` | OPT | topics/http/sessions.md |
| CTRL-110 | Session serialization `JSONSerializer` (default) + `SESSION_SERIALIZER` custom serializer | OPT/DIY | topics/http/sessions.md — pickle RCE warning for the cookie backend |
| CTRL-111 | `clearsessions` management command for purging expired sessions | OPT | topics/http/sessions.md |
| CTRL-112 | Session cookie/security settings: `SESSION_COOKIE_SECURE`/`_HTTPONLY`/`_SAMESITE`/`_AGE`/`_DOMAIN`/`_NAME`/`_PATH`, `SESSION_SAVE_EVERY_REQUEST`, `SESSION_EXPIRE_AT_BROWSER_CLOSE`, `SESSION_CACHE_ALIAS` | OPT | topics/http/sessions.md — subdomain session-fixation notes |
| CTRL-113 | Custom session engines via `AbstractBaseSession`/`BaseSessionManager` + `SessionStore` subclassing (`get_model_class`, `create_model_instance`, `cache_key_prefix`) | DIY | topics/http/sessions.md |
| CTRL-114 | Messages framework storage backends `SessionStorage`, `CookieStorage`, `FallbackStorage` (default) via `MESSAGE_STORAGE`; custom `BaseStorage` | OPT | ref/contrib/messages.md, ref/middleware.md — `django.contrib.messages` + `MessageMiddleware` + context processor |
| CTRL-115 | Message levels (DEBUG/INFO/SUCCESS/WARNING/ERROR + `MESSAGE_LEVEL`/`set_level`/`get_level`) and tags (`MESSAGE_TAGS`, `level_tag`, `extra_tags`) | OPT | ref/contrib/messages.md |
| CTRL-116 | Messages API `add_message` + shortcuts (`debug`/`info`/`success`/`warning`/`error`), `get_messages`, `Message` class, `fail_silently`, custom levels | OPT | ref/contrib/messages.md |
| CTRL-117 | `SuccessMessageMixin` for CBVs and `MessagesTestMixin.assertMessages` test helper | OPT | ref/contrib/messages.md |
| CTRL-118 | `Paginator`/`Page` classes (`count`/`num_pages`/`page_range`, `get_page`/`page`, `orphans`, `get_elided_page_range`, `ELLIPSIS`) | CORE | ref/paginator.md, topics/pagination.md — `InvalidPage`/`PageNotAnInteger`/`EmptyPage` |
| CTRL-119 | `AsyncPaginator`/`AsyncPage` (async `a`-prefixed methods, `acount`/`anum_pages`/`aget_page`/`aget_object_list`) | CORE | ref/paginator.md — **added 6.0** |
| CTRL-120 | `ListView` built-in pagination (`paginate_by` → `paginator`+`page_obj` context, template nav) | CORE | topics/pagination.md, topics/class-based-views/mixins.md |

## P3 — Views, templating & frontend assets

**Problem.** Render HTML server-side with layouts, components and safe interpolation, and get CSS/JS/images to the browser. **Answer.** The Django Template Language — a deliberately restricted, autoescaping-by-default text language with `{% block %}` inheritance, ~60 built-in filters and a custom-tag extension API — behind a multi-engine abstraction (Jinja2 as the blessed alternative); `django.contrib.staticfiles` collects per-app assets and cache-busts them with hashed manifests. There is no bundler/JS-toolchain integration; frontend tooling is explicitly out of scope.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VIEW-1 | DTL variables with dot-lookup (dict → attribute/method → list-index, auto-call of callables, `alters_data`/`do_not_call_in_templates`) | CORE | ref/templates/language.md, ref/templates/api.md — `{{ var }}`; private `_`-attrs blocked |
| VIEW-2 | DTL filters: pipe syntax, chaining, quoted args (~60 built-ins) | CORE | ref/templates/language.md, ref/templates/builtins.md — grouped in the rows below |
| VIEW-3 | String filters family (`lower`/`upper`/`title`/`capfirst`/`cut`/`slugify`/`stringformat`/`ljust`/`rjust`/`center`/`truncatechars(_html)`/`truncatewords(_html)`/`wordwrap`/`wordcount`/`make_list`/`addslashes`/`phone2numeric`) | CORE | ref/templates/builtins.md filter reference |
| VIEW-4 | Escaping/HTML filters family (`escape`/`safe`/`force_escape`/`escapejs`/`escapeseq`/`safeseq`/`striptags`/`linebreaks`/`linebreaksbr`/`linenumbers`/`urlize`/`urlizetrunc`/`json_script`/`iriencode`/`urlencode`) | CORE | ref/templates/builtins.md |
| VIEW-5 | List/collection filters family (`join`/`first`/`last`/`length`/`slice`/`random`/`dictsort`/`dictsortreversed`/`unordered_list`/`add`) | CORE | ref/templates/builtins.md |
| VIEW-6 | Number/date/logic formatting filters (`date`/`time`/`timesince`/`timeuntil`/`floatformat`/`filesizeformat`/`get_digit`/`divisibleby`/`pluralize`/`yesno`/`default`/`default_if_none`/`pprint`) | CORE | ref/templates/builtins.md — `date` uses PHP-style specifiers, locale-aware predefined formats |
| VIEW-7 | Control-flow tags `{% if %}/{% elif %}/{% else %}` (operators `==`, `in`, `is`, `and/or/not`, filters) and `{% for %}/{% empty %}` (forloop.counter/first/last/length etc.) | CORE | ref/templates/builtins.md — `forloop.length` added **6.0** |
| VIEW-8 | Template inheritance `{% extends %}` / `{% block %}` (block.super, named endblock, relative `./`,`../` paths) | CORE | ref/templates/language.md, builtins.md |
| VIEW-9 | `{% include %}` with `with ... only` context control | CORE | ref/templates/builtins.md — independent rendering, pre-evaluated blocks |
| VIEW-10 | `{% url %}` reverse tag (positional/kwargs, `as var`, namespaced) | CORE | ref/templates/builtins.md |
| VIEW-11 | `{% csrf_token %}` tag | CORE | ref/templates/builtins.md, ref/csrf.md — pairs with the csrf context processor; CSRF machinery in SEC- |
| VIEW-12 | `{% load %}` custom-library loading (`from`, per-template scope) | CORE | ref/templates/language.md, builtins.md |
| VIEW-13 | Presentation/utility tags `{% cycle %}`/`{% resetcycle %}`/`{% regroup %}`/`{% ifchanged %}`/`{% firstof %}`/`{% with %}`/`{% widthratio %}`/`{% spaceless %}`/`{% verbatim %}`/`{% templatetag %}`/`{% now %}`/`{% lorem %}`/`{% debug %}` | CORE | ref/templates/builtins.md |
| VIEW-14 | Comments `{# ... #}` and `{% comment %}` block | CORE | ref/templates/language.md, builtins.md |
| VIEW-15 | `{% querystring %}` tag (build/modify/remove query params, list handling) | CORE | ref/templates/builtins.md — `?` prepended for empty + multi-positional args **6.0** |
| VIEW-16 | Template partials `{% partialdef %}`/`{% partial %}` (inline option, `#name` direct access) | CORE | ref/templates/language.md, builtins.md — **added 6.0**, DjangoTemplates backend only |
| VIEW-17 | Automatic HTML escaping + `{% autoescape on/off %}`, `{% filter %}` block | CORE | ref/templates/language.md — escapes `< > ' " &`, on by default |
| VIEW-18 | `{% csp_nonce_attr %}` tag (renders `nonce=` on script/link, accepts `form.media`) | CORE | ref/templates/builtins.md — **added 6.1**, needs the csp context processor; CSP machinery in SEC- |
| VIEW-19 | i18n/l10n/tz and humanize tag libraries (loaded via `{% load %}`) | OPT | ref/templates/builtins.md tail — cross-ref I18N- and VIEW-49 |
| VIEW-20 | `TEMPLATES` setting: multiple engines, `DIRS`/`APP_DIRS`/`OPTIONS`/`NAME`, search order | CORE | topics/templates.md — default empty; startproject sets DjangoTemplates |
| VIEW-21 | DjangoTemplates backend + Engine/Template/Context/RequestContext components | CORE | topics/templates.md, ref/templates/api.md — OPTIONS autoescape/loaders/builtins/libraries/string_if_invalid |
| VIEW-22 | Jinja2 backend (`django.template.backends.jinja2.Jinja2`, custom `environment` callable) | ECO | topics/templates.md — requires `pip install Jinja2` |
| VIEW-23 | Custom template backend (subclass `BaseEngine`, debug/postmortem/origin hooks) | DIY | howto/custom-template-backend.md |
| VIEW-24 | Built-in template loaders: filesystem, app_directories, cached, locmem | CORE | ref/templates/api.md — cached loader auto-enabled when loaders unspecified |
| VIEW-25 | Custom template loaders (subclass `Loader`, `get_template_sources`/`get_contents`) | DIY | ref/templates/api.md |
| VIEW-26 | Context processors — built-in (`request`, `debug`, `i18n`, `media`, `static`, `csrf`, `csp`, `tz` core; `auth`, `messages` contrib) | CORE/OPT | ref/templates/api.md — `csp` added **6.0**; csrf hardcoded on RequestContext |
| VIEW-27 | Custom context processors (request→dict callable) | DIY | ref/templates/api.md, topics/templates.md |
| VIEW-28 | Loading/render API: `get_template`/`select_template`/`render_to_string`/`engines`, partial `template.html#name` loading | CORE | topics/templates.md — partial loading added **6.0** |
| VIEW-29 | Custom filters (`Library.filter`, `@stringfilter`, `is_safe`/`needs_autoescape`/`expects_localtime`) | CORE | howto/custom-template-tags.md — `mark_safe`/`conditional_escape` |
| VIEW-30 | Custom tag shortcuts: `simple_tag`, `simple_block_tag`, `inclusion_tag` (all support `takes_context`, `as var`, args/kwargs) | CORE | howto/custom-template-tags.md — `simple_block_tag` `content`/`end_name` |
| VIEW-31 | Advanced custom tags (compilation fn → `Node.render`, `parser.parse`, `token.split_contents`, `render_context` thread-safety) | DIY | howto/custom-template-tags.md |
| VIEW-32 | Overriding templates in other apps (project `DIRS` before app `APP_DIRS`, extend-an-overridden via `block.super`) | CORE | howto/overriding-templates.md |
| VIEW-33 | `django.contrib.staticfiles` app (collects per-app `static/` + `STATICFILES_DIRS`) | OPT | ref/contrib/staticfiles.md, howto/static-files/index.md — needs INSTALLED_APPS |
| VIEW-34 | `{% static %}` tag (+ `get_static_prefix`/`get_media_prefix`) | CORE/OPT | ref/contrib/staticfiles.md, howto/static-files/index.md — works with bare `STATIC_URL` (CORE); routes through storage `url()` when staticfiles installed (OPT) |
| VIEW-35 | `collectstatic` command (`--noinput`/`--ignore`/`--clear`/`--link`/`--no-post-process`/`--dry-run`) | OPT | ref/contrib/staticfiles.md — copies into `STATIC_ROOT` |
| VIEW-36 | `findstatic` command (`--first`, verbosity) | OPT | ref/contrib/staticfiles.md — debugging aid |
| VIEW-37 | Dev serving via staticfiles: `runserver` auto-serve (`--nostatic`/`--insecure`), `views.serve`, `staticfiles_urlpatterns()` | OPT | ref/contrib/staticfiles.md — only when DEBUG=True; "grossly inefficient" |
| VIEW-38 | Dev serving without staticfiles: `django.conf.urls.static.static()` helper / `django.views.static.serve` | CORE | howto/static-files/index.md, ref/urls.md, ref/views.md — DEBUG-only URL pattern for `MEDIA_URL`/`MEDIA_ROOT`; bypasses middleware; not hardened for production |
| VIEW-39 | `StaticFilesStorage` + `post_process` hook | OPT | ref/contrib/staticfiles.md — subclass the STORAGES `staticfiles` alias for permissions |
| VIEW-40 | `ManifestStaticFilesStorage` (MD5-hashed filenames, `staticfiles.json`, `manifest_strict`/`max_post_process_passes`/`file_hash`/`manifest_hash`, JS-module support) + `ManifestFilesMixin` | OPT | ref/contrib/staticfiles.md, topics/performance.md — cache-busting/far-future Expires; needs DEBUG=False + collectstatic |
| VIEW-41 | `STORAGES` setting `staticfiles` alias + `finders` module (`find`, `searched_locations`) | CORE/OPT | ref/contrib/staticfiles.md — `STATICFILES_FINDERS` |
| VIEW-42 | Production static deployment: collectstatic → same-server (Nginx/Apache), rsync to dedicated server, CDN/cloud (S3) via custom `STORAGES` backend | DIY/ECO | howto/static-files/deployment.md — Nginx/Apache named; third-party storage backends referenced (djangopackages); no WhiteNoise mention |
| VIEW-43 | Form `Media` class (`css` dict/media-types, `js`, `extend`, static vs dynamic `media` property, subscript filtering, `+` combining with order preservation) | CORE | topics/forms/media.md — form.media aggregates widget media; `MediaOrderConflictWarning` |
| VIEW-44 | `Script`/`Stylesheet` media asset objects (custom HTML attrs, e.g. crossorigin/async) | CORE | topics/forms/media.md — **added 6.1** |
| VIEW-45 | CSV output: Python `csv` writer over `HttpResponse`, streaming via `StreamingHttpResponse` (batched generator), or the template system | DIY | howto/outputting-csv.md — `content_type="text/csv"`, `addslashes` in templates |
| VIEW-46 | PDF output: ReportLab canvas over `FileResponse` (`as_attachment`/`filename`) | ECO | howto/outputting-pdf.md — `pip install reportlab`, not thread-safe |
| VIEW-47 | contrib.flatpages: `FlatPage` model (url/title/content/template_name/registration_required/enable_comments/sites M2M) + `FlatpageFallbackMiddleware` (404 fallback, 301) or URLconf/catchall usage | OPT | ref/contrib/flatpages.md, ref/middleware.md — requires contrib.sites; place middleware near bottom |
| VIEW-48 | contrib.flatpages extras: `{% get_flatpages %}` tag (`for user`, `starts_with`), `FlatpageForm`, `FlatPageSitemap` integration | OPT | ref/contrib/flatpages.md |
| VIEW-49 | contrib.humanize template filters: `apnumber`, `intcomma`, `intword` (to 10^100), `naturalday`, `naturaltime`, `ordinal` (i18n-aware) | OPT | ref/contrib/humanize.md — `{% load humanize %}` |

## P4 — Data layer: models, ORM & queries

**Problem.** Define domain models, map them to relational tables, and query/persist them safely and efficiently. **Answer.** An active-record ORM: declarative `models.Model` classes carry fields, metadata (`Meta`), constraints, indexes and managers; lazy, chainable QuerySets compose SQL from lookups, `F`/`Q`/expression objects, aggregates and window functions; persistence is explicit (`save()`), transactions run through `atomic`, multi-DB routing is pluggable, and a growing async surface (`a`-prefixed methods, fetch modes) is layered on top with raw-SQL escape hatches throughout.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| ORM-1 | Declarative models (class subclassing `models.Model`, one class = one table, auto DB-access API) | CORE | topics/db/models.md — app must be in `INSTALLED_APPS` |
| ORM-2 | Automatic PK (`id = BigAutoField(primary_key=True)`; type set per-app via `AppConfig.default_auto_field` or `DEFAULT_AUTO_FIELD`) | CORE | topics/db/models.md — exactly one `primary_key=True` field per model; `BigAutoField` default since **6.0** |
| ORM-3 | `Field.null` / `Field.blank` (DB-null vs validation-blank; avoid null on string fields) | CORE | ref/models/fields.md |
| ORM-4 | `Field.choices` — sequence of 2-tuples, mapping, named groups, or zero-arg callable | CORE | ref/models/fields.md — new migration on choices reorder; blank label default changed to "- Select an option -" (**6.1**) |
| ORM-5 | Enumeration choices: `TextChoices`, `IntegerChoices`, `Choices` + concrete type (e.g. `date`); `.choices/.labels/.values/.names`, auto labels, lazy-translatable labels, `__empty__`, Enum functional API | CORE | ref/models/fields.md — `enum.unique` enforced; no named groups |
| ORM-6 | `Field.default` (value or callable; mutables/lambdas forbidden; FK defaults are pk values) | CORE | ref/models/fields.md |
| ORM-7 | `Field.db_default` (database-computed default: literal or expression, e.g. `Now()`; cannot reference other fields; `DatabaseDefault` sentinel on unsaved instances; `default` wins in Python code) | CORE | ref/models/fields.md, topics/db/models.md |
| ORM-8 | `Field.db_column`, `Field.db_comment` | CORE | ref/models/fields.md — reserved-word column names OK (quoted) |
| ORM-9 | `Field.db_index` (single-column index; docs steer to `Meta.indexes`, may be deprecated) | CORE | ref/models/fields.md |
| ORM-10 | `Field.db_tablespace` (per-field index tablespace) | OPT | ref/models/fields.md — ignored if backend lacks tablespaces |
| ORM-11 | `Field.editable` (hidden from ModelForms/admin, skipped in validation) | CORE | ref/models/fields.md |
| ORM-12 | `Field.error_messages` (override keys: null/blank/invalid/invalid_choice/unique/unique_for_date) | OPT | ref/models/fields.md — often doesn't propagate to forms |
| ORM-13 | `Field.help_text` (not HTML-escaped) | CORE | ref/models/fields.md |
| ORM-14 | `Field.primary_key` (read-only; changing pk + save creates a new row; implies null=False, unique=True) | CORE | ref/models/fields.md |
| ORM-15 | `Field.unique` (DB-enforced + model validation; implies index; invalid on M2M/O2O) | CORE | ref/models/fields.md |
| ORM-16 | `Field.unique_for_date` / `unique_for_month` / `unique_for_year` (validation-only, not DB-level) | OPT | ref/models/fields.md — enforced by `Model.validate_unique` |
| ORM-17 | `Field.verbose_name` (first positional arg except on relation fields) | CORE | ref/models/fields.md, topics/db/models.md |
| ORM-18 | `Field.validators` (list of validators) | CORE | ref/models/fields.md — validators themselves in VAL- |
| ORM-19 | Auto fields: `AutoField`, `BigAutoField`, `SmallAutoField` | CORE | ref/models/fields.md |
| ORM-20 | Integer fields: `IntegerField`, `BigIntegerField`, `SmallIntegerField`, `PositiveIntegerField`, `PositiveBigIntegerField`, `PositiveSmallIntegerField` | CORE | ref/models/fields.md — min/max validators added per backend |
| ORM-21 | `FloatField` vs `DecimalField(max_digits, decimal_places)` | CORE | ref/models/fields.md — precision-less DecimalField on Oracle/PostgreSQL/SQLite (**6.1**); always required on MySQL |
| ORM-22 | String fields: `CharField(max_length, db_collation)`, `TextField(db_collation)`, `SlugField(allow_unicode)` (implies db_index), `EmailField`, `URLField`, `UUIDField`, `GenericIPAddressField(protocol, unpack_ipv4)` | CORE | ref/models/fields.md — max_length optional on PostgreSQL/SQLite; UUID stored native on PostgreSQL/MariaDB |
| ORM-23 | Date/time fields: `DateField`/`DateTimeField`/`TimeField` with `auto_now` / `auto_now_add` (mutually exclusive with `default`; force editable=False; not applied by `QuerySet.update()`); `DurationField` | CORE | ref/models/fields.md |
| ORM-24 | `BooleanField` (default `None` when no default; NullBooleanSelect widget if null=True) | CORE | ref/models/fields.md |
| ORM-25 | `BinaryField(max_length)` (bytes/bytearray/memoryview; editable=False by default) | CORE | ref/models/fields.md — docs warn against file storage in the DB |
| ORM-26 | `FileField(upload_to, storage, max_length=100)` — strftime or callable `upload_to(instance, filename)`, callable storage; `FieldFile` proxy API (`name/path/size/url/open/close/save/delete`) | CORE | ref/models/fields.md — needs MEDIA_ROOT/MEDIA_URL; orphan files not auto-deleted; primary_key unsupported |
| ORM-27 | `ImageField(height_field, width_field)` (validates image; auto-populates dimensions) | CORE | ref/models/fields.md — requires Pillow (ECO dependency) |
| ORM-28 | `FilePathField(path, match, recursive, allow_files, allow_folders)` (path may be callable; match on basename) | OPT | ref/models/fields.md |
| ORM-29 | `JSONField(encoder, decoder)` (jsonb on PostgreSQL; callable default required for dict/list; GinIndex advised over B-tree) | CORE | ref/models/fields.md — SQLite needs JSON1; Oracle no scalar values |
| ORM-30 | `GeneratedField(expression, output_field, db_persist)` — stored vs virtual `GENERATED ALWAYS` columns | CORE | ref/models/fields.md — auto-refreshed after save on SQLite/PG/Oracle (**6.0**); virtual on PostgreSQL 18+ & stored on Oracle 23ai (**6.1**); expression must be deterministic, same-table fields only |
| ORM-31 | `CompositePrimaryKey(*field_names)` — virtual `pk` field; tuple pk get/set/filter | OPT | ref/models/fields.md, topics/composite-primary-key.md — no FK to CPK models, excluded from ModelForms/admin, no migrate to/from CPK (use `--fake` or SeparateDatabaseAndState); `Max("pk")` ValueError (Count OK); introspect via `_meta.pk_fields` |
| ORM-32 | `ForeignObject(from_fields, to_fields)` workaround for relations to composite-PK models (no columns/constraints; on_delete ignored) | OPT | topics/composite-primary-key.md |
| ORM-33 | Field name restrictions (no Python reserved words, no `__`, no trailing `_`, not `check`); SQL reserved words allowed | CORE | topics/db/models.md — work around via db_column |
| ORM-34 | Custom field types (subclass `Field`; `db_type`, `rel_db_type`, `get_prep_value`, `get_db_prep_value/save`, `from_db_value`, `pre_save`, `to_python`, `value_to_string`, `formfield`, `deconstruct`, `get_internal_type`, `descriptor_class`) | DIY | ref/models/fields.md (Field API reference); howto/custom-model-fields referenced |
| ORM-35 | Field introspection flags (`auto_created`, `concrete`, `hidden`, `is_relation`, `many_to_many/many_to_one/one_to_many/one_to_one`, `related_model`, `model`) | OPT | ref/models/fields.md — preferred over isinstance checks with the `_meta` API |
| ORM-36 | Per-field lookup/transform registration (`Field` implements the lookup registration API) | DIY | ref/models/fields.md |
| ORM-37 | Custom model methods, `@property`, overriding `save()`/`delete()` (must call super, pass `**kwargs`; not called on bulk ops/cascades — use signals) | CORE | topics/db/models.md |
| ORM-38 | Organizing models in a `models/` package (import into `__init__.py`) | OPT | topics/db/models.md |
| ORM-39 | `ForeignKey(to, on_delete)` many-to-one; auto `_id` column + auto index (disable via db_index=False) | CORE | topics/db/models.md, ref/models/fields.md |
| ORM-40 | `on_delete` Python-emulated: `CASCADE` (signals sent, `delete()` not called), `PROTECT` (ProtectedError), `RESTRICT` (RestrictedError, allows same-op cascade), `SET_NULL`, `SET_DEFAULT`, `SET(value_or_callable)`, `DO_NOTHING` | CORE | ref/models/fields.md |
| ORM-41 | `on_delete` database-level: `DB_CASCADE`, `DB_SET_NULL`, `DB_SET_DEFAULT` (needs `db_default`) | OPT | ref/models/fields.md — **new in 6.1**; more efficient but no pre/post_delete signals under DB_CASCADE; cannot mix DB_* with Python variants (except DO_NOTHING) across related models; DB_SET_DEFAULT unsupported on MySQL/MariaDB |
| ORM-42 | FK args: `limit_choices_to` (dict/Q/callable), `related_name` (`'+'` disables reverse), `related_query_name`, `to_field` (must be unique), `db_constraint`, `swappable` | CORE | ref/models/fields.md — related_name mandatory + `%(app_label)s`/`%(class)s` syntax on abstract bases |
| ORM-43 | `ManyToManyField(to)` — auto join table (auto-named, hash-truncated); `db_table`, `db_constraint`, `swappable`, `related_name/related_query_name/limit_choices_to` | CORE | ref/models/fields.md — no `validators`; `null` has no effect; declare on one side only |
| ORM-44 | `symmetrical` (self-referential M2M; default symmetric, set False for directed) | CORE | ref/models/fields.md |
| ORM-45 | `through` intermediate model with extra fields; `through_fields=(source, target)` disambiguation; FK-count restrictions; implicit through model always queryable (`Model.m2mfield.through`) | CORE | topics/db/models.md, ref/models/fields.md — `through_defaults` for add/create/set; remove() drops all dup rows; clear() wipes relations |
| ORM-46 | `OneToOneField(to, on_delete, parent_link=False)` — reverse returns single object; default related_name = lowercased model; `RelatedObjectDoesNotExist` on missing reverse; multiple O2Os per model allowed | CORE | topics/db/models.md, ref/models/fields.md |
| ORM-47 | Recursive relations (`"self"`) — in abstract bases resolves to each concrete subclass | CORE | ref/models/fields.md |
| ORM-48 | Lazy string references: relative (`"Manufacturer"` — resolved in the concrete subclass's app) and absolute (`"app_label.ModelName"`) — solves circular imports, cross-app models | CORE | ref/models/fields.md, topics/db/models.md |
| ORM-49 | `Meta.abstract` (abstract base class, no table) | CORE | ref/models/options.md |
| ORM-50 | `Meta.app_label` (model outside INSTALLED_APPS app) | OPT | ref/models/options.md |
| ORM-51 | `Meta.base_manager_name` / `default_manager_name` | OPT | ref/models/options.md |
| ORM-52 | `Meta.db_table` (auto `applabel_modelname` otherwise; quoting for Oracle 30-char limit; lowercase advised on MySQL/MariaDB) | CORE | ref/models/options.md |
| ORM-53 | `Meta.db_table_comment` | OPT | ref/models/options.md |
| ORM-54 | `Meta.db_tablespace` | OPT | ref/models/options.md — ignored without backend support |
| ORM-55 | `Meta.default_related_name` (default reverse accessor + query name; `%(app_label)s`/`%(model_name)s` interpolation) | OPT | ref/models/options.md |
| ORM-56 | `Meta.get_latest_by` (field name or list incl. `-desc`; drives `latest()`/`earliest()`) | CORE | ref/models/options.md |
| ORM-57 | `Meta.managed` (False = Django never creates/alters/drops the table; for existing tables/views; affects auto-created M2M tables) | OPT | ref/models/options.md |
| ORM-58 | `Meta.order_with_respect_to` (adds `_order` column; provides `get_RELATED_order()`, `set_RELATED_order()`, `get_next_in_order()`, `get_previous_in_order()`; mutually exclusive with `ordering`) | OPT | ref/models/options.md — requires migration |
| ORM-59 | `Meta.ordering` (field names, `-` desc, `?` random, query expressions e.g. `F("author").asc(nulls_last=True)`) | CORE | ref/models/options.md — not applied in GROUP BY queries |
| ORM-60 | `Meta.permissions` (extra `(codename, name)` tuples) + `default_permissions` (default add/change/delete/view; customizable/emptiable, before first migrate) | CORE | ref/models/options.md — permission machinery in AUTH- |
| ORM-61 | `Meta.proxy` | CORE | ref/models/options.md |
| ORM-62 | `Meta.required_db_features` / `required_db_vendor` (conditionally sync model per backend capabilities/vendor) | OPT | ref/models/options.md |
| ORM-63 | `Meta.select_on_save` (pre-1.6 SELECT-then-UPDATE save algorithm; for `ON UPDATE` trigger edge cases) | OPT | ref/models/options.md |
| ORM-64 | `Meta.indexes` (list of `Index` instances) | CORE | ref/models/options.md |
| ORM-65 | `Meta.unique_together` (legacy; docs steer to `UniqueConstraint`, may be deprecated; single or list-of-lists form; no M2M fields) | OPT | ref/models/options.md |
| ORM-66 | `Meta.constraints` (list of CheckConstraint/UniqueConstraint) | CORE | ref/models/options.md |
| ORM-67 | `Meta.verbose_name` / `verbose_name_plural` | CORE | ref/models/options.md |
| ORM-68 | Read-only `Meta.label` / `label_lower` (`app_label.ObjectName`) | OPT | ref/models/options.md |
| ORM-69 | `BaseConstraint(name, violation_error_code, violation_error_message)` + `validate()` — base for custom constraints (implement `constraint_sql/create_sql/remove_sql/validate`) | DIY | ref/models/constraints.md |
| ORM-70 | Constraint validation during model validation (`full_clean` → `validate_constraints`) | CORE | ref/models/constraints.md, ref/models/instances.md — full validation pipeline in VAL- |
| ORM-71 | `CheckConstraint(condition=Q() or boolean Expression, name)` | CORE | ref/models/constraints.md — Q order preserved between Qs; Oracle <23c nullable-field caveat |
| ORM-72 | `UniqueConstraint(fields=[...], name)` | CORE | ref/models/constraints.md — violation error code/message fall back to unique/unique_together defaults |
| ORM-73 | `UniqueConstraint(*expressions)` functional unique constraints (e.g. `Lower("name").desc()`) | OPT | ref/models/constraints.md — same backend limits as Index.expressions |
| ORM-74 | `UniqueConstraint(condition=Q(...))` partial uniqueness | OPT | ref/models/constraints.md — same limits as Index.condition (ignored MySQL/MariaDB; unsupported Oracle) |
| ORM-75 | `UniqueConstraint(deferrable=Deferrable.DEFERRED/IMMEDIATE)` | OPT | ref/models/constraints.md — ignored on MySQL/MariaDB/SQLite; perf penalty warning |
| ORM-76 | `UniqueConstraint(include=[...])` covering unique index | OPT | ref/models/constraints.md — PostgreSQL-only |
| ORM-77 | `UniqueConstraint(opclasses=[...])` | OPT | ref/models/constraints.md — PostgreSQL-only |
| ORM-78 | `UniqueConstraint(nulls_distinct=False)` treat NULLs as equal | OPT | ref/models/constraints.md — PostgreSQL-only |
| ORM-79 | `Index(fields=[...])` B-tree; `-field` for descending column | CORE | ref/models/indexes.md |
| ORM-80 | `Index(*expressions)` functional index (`name` required) | OPT | ref/models/indexes.md — Oracle DETERMINISTIC / PostgreSQL IMMUTABLE required; ignored on MariaDB |
| ORM-81 | `Index.name` (auto-generated if omitted; ≤30 chars, no leading digit/underscore) | CORE | ref/models/indexes.md |
| ORM-82 | `Index(condition=Q(...))` partial index | OPT | ref/models/indexes.md — ignored MySQL/MariaDB; Oracle unsupported (emulate via Case); SQLite restrictions |
| ORM-83 | `Index(include=[...])` covering index | OPT | ref/models/indexes.md — PostgreSQL-only; name required |
| ORM-84 | `Index(opclasses=[...])`, `Index(db_tablespace=...)` | OPT | ref/models/indexes.md — opclasses PostgreSQL-only |
| ORM-85 | `%(app_label)s` / `%(class)s` placeholders in constraint/index names for abstract bases | OPT | ref/models/constraints.md, ref/models/indexes.md |
| ORM-86 | PostgreSQL-specific index classes (GinIndex, GistIndex, SpGistIndex…) | OPT | ref/models/indexes.md — pointer; full set in PG- |
| ORM-87 | Abstract base classes (`abstract=True`; fields copied into children; no table/manager/instantiation; fields overridable or removable with `= None`) | CORE | topics/db/models.md |
| ORM-88 | Meta inheritance from abstract bases (child inherits/extends parent Meta; abstract reset to False; multi-base Meta needs explicit `Meta(A.Meta, B.Meta)`) | CORE | topics/db/models.md |
| ORM-89 | `related_name`/`related_query_name` interpolation (`%(app_label)s`, `%(class)s`) in abstract bases | CORE | topics/db/models.md |
| ORM-90 | Multi-table inheritance (each model own table; implicit `<parent>_ptr = OneToOneField(parent_link=True, primary_key=True)`; parent→child lowercase accessor; child Meta not inherited except `ordering`/`get_latest_by`) | CORE | topics/db/models.md |
| ORM-91 | Explicit parent link (`OneToOneField(..., parent_link=True)`) | OPT | topics/db/models.md |
| ORM-92 | Reverse-relation name clashes in MTI subclasses require `related_name` | CORE | topics/db/models.md |
| ORM-93 | Proxy models (`proxy=True`; same table; alternate default manager/ordering/methods; must inherit exactly one non-abstract model; querysets return the requested class) | CORE | topics/db/models.md |
| ORM-94 | Proxy vs `managed=False` guidance (unmanaged for views/legacy tables; proxy for Python-behavior changes) | CORE | topics/db/models.md |
| ORM-95 | Multiple inheritance / mixins (first Meta wins; `id` PK clash requires explicit `AutoField` per base or common ancestor with explicit parent links) | OPT | topics/db/models.md |
| ORM-96 | Field-name hiding forbidden on concrete bases (`FieldError`); abstract fields overridable; depth-first field resolution across abstract parents | CORE | topics/db/models.md |
| ORM-97 | `Model(**kwargs)` instantiation (no DB touch); avoid overriding `__init__` — use classmethod or manager `create_*` patterns | CORE | ref/models/instances.md |
| ORM-98 | `Model.from_db(db, field_names, values)` — customize loading (e.g. record loaded values); set `_state.adding/db`; `DEFERRED` sentinel | DIY | ref/models/instances.md |
| ORM-99 | `refresh_from_db()` / `arefresh_from_db()` (`using`, `fields`, `from_queryset` — soft-delete/select_related/select_for_update reload); `del obj.field` reload; drives deferred-field loading (overridable) | CORE | ref/models/instances.md |
| ORM-100 | `get_deferred_fields()` | OPT | ref/models/instances.md |
| ORM-101 | `save()` / `asave()` (`force_insert`, `force_update`, `using`, `update_fields`) | CORE | ref/models/instances.md |
| ORM-102 | Save signal pipeline (pre_save signal → field `pre_save()` → `get_db_prep_save()` → SQL → post_save signal) | CORE | ref/models/instances.md |
| ORM-103 | UPDATE-vs-INSERT algorithm (pk set → try UPDATE, else INSERT; pk with default/db_default → UPDATE only for known-existing instances; `select_on_save` legacy fallback) | CORE | ref/models/instances.md |
| ORM-104 | `force_insert` with MTI parent tuple — `save(force_insert=(Place,))` / `(models.Model,)` forces INSERT per base | OPT | ref/models/instances.md |
| ORM-105 | `Model.NotUpdated` raised when a forced update affects no rows (subclass of `ObjectNotUpdated` + `db.DatabaseError`) | CORE | ref/models/class.md, ref/models/instances.md — **new in 6.0** (was generic DatabaseError) |
| ORM-106 | `Model.DoesNotExist`, `Model.MultipleObjectsReturned` per-model exceptions | CORE | ref/models/class.md |
| ORM-107 | `update_fields` tracking (only listed fields saved; empty iterable skips save; forces UPDATE; deferred-loaded instances auto-restrict; only listed fields' `pre_save()` run — auto_now skipped; custom `save()` should extend kwargs["update_fields"]) | CORE | ref/models/instances.md, topics/db/models.md |
| ORM-108 | `pk` property (alias for PK field(s), tuple for composite); explicit auto-PK assignment; `_is_pk_set()` | CORE | ref/models/instances.md |
| ORM-109 | `delete()` / `adelete()` (`using`, `keep_parents`; returns (count, per-model dict); pk set to None; DB_* on_delete counts only the queryset's model — **6.1**) | CORE | ref/models/instances.md |
| ORM-110 | `__str__()` (used by admin/templates) | CORE | ref/models/instances.md, topics/db/models.md |
| ORM-111 | `__eq__` (same concrete class + same pk; pk=None equal only to itself; proxy compares to concrete parent) and `__hash__` (= hash(pk); TypeError on unsaved) | CORE | ref/models/instances.md |
| ORM-112 | `get_absolute_url()` (canonical URL; prefer `reverse()`; admin "View on site"; ASCII/URL-encoded; avoid unvalidated user input) | CORE | ref/models/instances.md |
| ORM-113 | `get_FOO_display()` for choice fields | CORE | ref/models/instances.md |
| ORM-114 | `get_next_by_FOO()` / `get_previous_by_FOO()` for non-null Date/DateTime fields (pk tie-break; extra lookup kwargs) | OPT | ref/models/instances.md |
| ORM-115 | `_state` (`ModelState.adding`, `ModelState.db`) | OPT | ref/models/instances.md |
| ORM-116 | Instance pickling (state-at-pickle-time; not portable across Django versions — RuntimeWarning) | OPT | ref/models/instances.md |
| ORM-117 | Updating via `F()` expressions to avoid race conditions | CORE | ref/models/instances.md — expressions detail at ORM-164 |
| ORM-118 | Default `objects` manager (auto-added when none declared; class-level only, not on instances) | CORE | topics/db/managers.md, ref/models/class.md |
| ORM-119 | Renaming the default manager (any attribute of type `models.Manager()`) | CORE | topics/db/managers.md |
| ORM-120 | Custom manager methods (table-level logic; `self.model` access; may return anything) | CORE | topics/db/managers.md |
| ORM-121 | Overriding `Manager.get_queryset()`; multiple managers per model as named filters | CORE | topics/db/managers.md |
| ORM-122 | `Model._default_manager` (first declared, or `Meta.default_manager_name`; used by dumpdata etc.) | CORE | topics/db/managers.md |
| ORM-123 | `Model._base_manager` (used for related-object access; must not filter rows; `Meta.base_manager_name`) | CORE | topics/db/managers.md |
| ORM-124 | Custom QuerySet methods mirrored on the manager manually, or `QuerySet.as_manager()` (copy rules: public copied, `_private` not, `queryset_only` attr opts in/out; `delete()` never copied) | CORE | topics/db/managers.md |
| ORM-125 | `Manager.from_queryset(QuerySetClass)` — combine custom Manager + custom QuerySet | CORE | topics/db/managers.md |
| ORM-126 | Manager inheritance (always inherited from bases incl. abstract; default = Meta.default_manager_name → first declared → first parent's default; extra-managers-via-second-abstract-base pattern; cannot invoke managers on abstract models) | CORE | topics/db/managers.md |
| ORM-127 | Managers must be shallow-copyable (`copy.copy`) | DIY | topics/db/managers.md — implementation concern for custom managers |
| ORM-128 | Proxy-model manager rules (inherit parent managers; own manager becomes default) | CORE | topics/db/models.md |
| ORM-129 | Manager serialization into migrations via `use_in_migrations = True` (incl. from_queryset-generated classes — must subclass to be importable) | OPT | topics/migrations.md |
| ORM-130 | `filter()`/`exclude()`/`get()` retrieval with `**kwargs` lookups + `Q`/expression `*args`, chained AND | CORE | ref/models/querysets.md, topics/db/queries.md — `get()` raises `DoesNotExist`/`MultipleObjectsReturned` |
| ORM-131 | Lazy `QuerySet` construction; DB hit only on evaluation (iteration, slice, `repr`/`len`/`list`/`bool`, pickle) + result caching | CORE | ref/models/querysets.md "When QuerySets are evaluated" — `all()` returns fresh copy; pickle via `.query` |
| ORM-132 | Slicing (`qs[:5]`, `[a:b]`) → LIMIT/OFFSET, unevaluated unless step given; no negative/reverse-from-end slicing | CORE | ref/models/querysets.md — combine with `reverse()[:5]` |
| ORM-133 | `create()`/`acreate()` — instantiate + save in one step | CORE | ref/models/querysets.md — `force_insert`, IntegrityError on dup manual PK |
| ORM-134 | `get_or_create()` / `update_or_create()` (+ `defaults`/`create_defaults`, async variants) | CORE | ref/models/querysets.md — atomic only if DB-level uniqueness; returns `(obj, created)` |
| ORM-135 | `bulk_create()` (`batch_size`, `ignore_conflicts`, `update_conflicts`/`update_fields`/`unique_fields`) | CORE | ref/models/querysets.md, topics/db/optimization.md — skips `save()`/signals, no MTI/M2M |
| ORM-136 | `bulk_update(objs, fields, batch_size)` | CORE | ref/models/querysets.md — uses CASE/WHEN, skips signals, can't update PK |
| ORM-137 | `update(**kwargs)` bulk SQL UPDATE (main table only; `F()` supported; `order_by().update()` on MySQL/MariaDB) | CORE | ref/models/querysets.md — no `save()`/signals; returns rows matched |
| ORM-138 | `delete()` bulk delete with cascade emulation + `DB_CASCADE` fast-path (**6.1**) | CORE | ref/models/querysets.md — returns `(count, {label: n})`; pre/post_delete signals |
| ORM-139 | `in_bulk(id_list, field_name)` → dict keyed by field; chainable after `values`/`values_list` (**6.1**) | OPT | ref/models/querysets.md |
| ORM-140 | `count()`/`exists()`/`contains(obj)`/`first()`/`last()`/`latest()`/`earliest()` terminal helpers (+ async `a`-variants) | CORE | ref/models/querysets.md — `first`/`last` no longer PK-order when ordering cleared (**6.1**) |
| ORM-141 | `values(*fields, **exprs)` → dicts; grouping semantics with aggregates | CORE | ref/models/querysets.md, topics/db/aggregation.md |
| ORM-142 | `values_list(*fields, flat=, named=)` → tuples / namedtuples | CORE | ref/models/querysets.md |
| ORM-143 | `dates()` / `datetimes(field, kind, order, tzinfo)` → truncated date/datetime lists | OPT | ref/models/querysets.md |
| ORM-144 | `distinct(*fields)` → `SELECT DISTINCT` / `DISTINCT ON` (fields Postgres-only) | CORE | ref/models/querysets.md — interacts with `order_by`/`values` |
| ORM-145 | `order_by(*fields)` (`-`, `?` random, `__` spanning, expressions `.asc()/.desc()` w/ `nulls_first/last`) + `reverse()` | CORE | ref/models/querysets.md — each call clears prior; `.ordered`/`.totally_ordered` (**6.1**) attrs |
| ORM-146 | `select_related(*fields)` — FK/O2O JOIN eager load | CORE | ref/models/querysets.md — no-arg form deprecated **6.1** (see fetch modes, ORM-200) |
| ORM-147 | `prefetch_related(*lookups)` + `Prefetch(lookup, queryset, to_attr)` + `prefetch_related_objects()` | CORE | ref/models/querysets.md — separate query joined in Python; M2M/reverse/generic; also `GenericPrefetch` (EXT-) |
| ORM-148 | `defer(*fields)` / `only(*fields)` deferred-field loading | OPT | ref/models/querysets.md — raises `SynchronousOnlyOperation` on async lazy-load |
| ORM-149 | `union()`/`intersection()`/`difference()` set operators (`all=` for UNION ALL) | OPT | ref/models/querysets.md — restricted downstream ops |
| ORM-150 | `none()` (`EmptyQuerySet`) / `all()` copy | CORE | ref/models/querysets.md |
| ORM-151 | `fetch_mode(mode)` — per-QuerySet on-demand field fetch strategy | OPT | ref/models/querysets.md, topics/db/fetch-modes.md — **new 6.1** |
| ORM-152 | `select_for_update(nowait, skip_locked, of, no_key)` row locking | OPT | ref/models/querysets.md — needs transaction; `no_key`/`of` backend-dependent |
| ORM-153 | `as_manager()` — build Manager from QuerySet methods | OPT | ref/models/querysets.md |
| ORM-154 | `FilteredRelation(relation_name, condition=Q())` — annotated JOIN `ON`-clause filtering | OPT | ref/models/querysets.md |
| ORM-155 | Combined-queryset boolean operators `&` / `\|` / `^` (XOR) | CORE | ref/models/querysets.md — same-model only |
| ORM-156 | `extra(select, where, params, tables, order_by, select_params)` raw-SQL escape hatch | DIY | ref/models/querysets.md — legacy, discouraged, aims to be deprecated |
| ORM-157 | Equality/text lookups: `exact`/`iexact`, `contains`/`icontains`, `startswith`/`istartswith`, `endswith`/`iendswith` | CORE | ref/models/querysets.md — SQLite treats case-sensitive as insensitive |
| ORM-158 | Membership/comparison lookups: `in` (list/queryset subquery), `gt`/`gte`/`lt`/`lte`, `range` (BETWEEN) | CORE | ref/models/querysets.md — nested-query perf notes |
| ORM-159 | `isnull` (IS NULL / IS NOT NULL) | CORE | ref/models/querysets.md |
| ORM-160 | Date/time-part lookup family: `date`, `year`, `iso_year`, `month`, `day`, `week`, `week_day`, `iso_week_day`, `quarter`, `time`, `hour`, `minute`, `second` (chainable, tz-aware) | CORE | ref/models/querysets.md |
| ORM-161 | Regex lookups: `regex`/`iregex` (backend/Python `re` syntax) | OPT | ref/models/querysets.md |
| ORM-162 | `pk` lookup shortcut + `__` relationship traversal (forward & reverse, spanning joins) | CORE | topics/db/queries.md "Lookups that span relationships" |
| ORM-163 | JSONField lookup family: key/index/path transforms, `contains`/`contained_by`, `has_key`/`has_keys`/`has_any_keys`, `JSONNull()` matching (**6.1**) | OPT | topics/db/queries.md "Querying JSONField" — None-as-JSON-null deprecated **6.1** |
| ORM-164 | `F()` expressions — reference/compute on field values in the DB (arithmetic, slicing, `~F()`, filters, annotations, race-free updates) | CORE | ref/models/expressions.md — refreshed after `save()` (**6.0**) |
| ORM-165 | `Q()` objects — composable conditions with `&`/`\|`/`^`/`~` | CORE | ref/models/querysets.md, topics/db/queries.md "Complex lookups" |
| ORM-166 | `Subquery()` + `OuterRef()` correlated subqueries | OPT | ref/models/expressions.md — `values()[:1]` to limit column/row |
| ORM-167 | `Exists()` / `~Exists()` — `EXISTS`/`NOT EXISTS`, usable directly in filters | OPT | ref/models/expressions.md |
| ORM-168 | `Window()` functions with `partition_by`/`order_by`/`frame`; `RowRange`/`ValueRange` frames + `WindowFrameExclusion` | OPT | ref/models/expressions.md — MySQL/Postgres/Oracle |
| ORM-169 | Conditional expressions `Case()` / `When()` (if/elif/else in queries; filter/annotate/update) | OPT | ref/models/conditional-expressions.md |
| ORM-170 | `Value()`, `Func()`, `ExpressionWrapper()`, `RawSQL()`, `JSONNull()` (**6.1**) building blocks | OPT | ref/models/expressions.md — `RawSQL`/`ExpressionWrapper` for output_field |
| ORM-171 | Direct `Lookup`/`Transform` use in filters/annotations (e.g. `GreaterThan(F(...), ...)`) | OPT | ref/models/lookups.md, ref/models/expressions.md |
| ORM-172 | Custom lookups & transforms via `register_lookup`, `Lookup`/`Transform`, bilateral, backend `as_vendorname()` | DIY | howto/custom-lookups.md, ref/models/lookups.md |
| ORM-173 | Full-text/advanced search `__search`, `__unaccent`, `__trigram_similar` | ECO | topics/db/search.md — provided by `django.contrib.postgres` (PG-), not core |
| ORM-174 | `annotate()` per-row + `aggregate()` terminal summaries (auto/alias names, `**expressions` grouping via `values()`) | CORE | ref/models/querysets.md, topics/db/aggregation.md — annotate/filter/values order matters |
| ORM-175 | `alias()` — expression reuse without selecting (for filter/order/update) | OPT | ref/models/querysets.md |
| ORM-176 | Aggregate functions: `Avg`, `Count`, `Max`, `Min`, `Sum`, `StdDev`, `Variance` (common `output_field`/`filter`/`default`/`distinct`) | CORE | ref/models/querysets.md "Aggregation functions" |
| ORM-177 | Newer aggregates: `AnyValue` (**6.0**), `StringAgg` (**6.0**), `BitAnd`/`BitOr`/`BitXor` (**6.1**) | OPT | ref/models/querysets.md — `AnyValue` for MySQL `ONLY_FULL_GROUP_BY` |
| ORM-178 | Conditional aggregation via `filter=Q(...)` (SQL `FILTER WHERE`/`CASE`) + filtering on annotations | OPT | ref/models/conditional-expressions.md, topics/db/aggregation.md |
| ORM-179 | `Aggregate()` base incl. `order_by` arg (**6.0**), `distinct`, `default` (Coalesce wrap) — custom aggregate creation | DIY | ref/models/expressions.md |
| ORM-180 | DB functions — comparison/conversion: `Cast`, `Coalesce`, `Collate`, `Greatest`, `Least`, `NullIf` | OPT | ref/models/database-functions.md |
| ORM-181 | DB functions — date/time: `Extract` (+ `DateField`/`DateTimeField` extracts), `Now`, `Trunc` (date/datetime/time variants) | OPT | ref/models/database-functions.md |
| ORM-182 | DB functions — math: `Abs`, `Ceil`, `Floor`, `Round`, `Power`, `Sqrt`, `Exp`, `Ln`, `Log`, `Mod`, `Sign`, `Random`, `Pi`, trig (`Sin`/`Cos`/`Tan`/`ATan2`/`Radians`/`Degrees`…) | OPT | ref/models/database-functions.md |
| ORM-183 | DB functions — text: `Concat`, `Length`, `Lower`/`Upper`, `Substr`, `Left`/`Right`, `LPad`/`RPad`, `LTrim`/`RTrim`/`Trim`, `Replace`, `Reverse`, `Repeat`, `StrIndex`, `Chr`/`Ord`, `MD5`/`SHA1..SHA512` | OPT | ref/models/database-functions.md |
| ORM-184 | DB functions — JSON: `JSONArray`, `JSONObject` | OPT | ref/models/database-functions.md |
| ORM-185 | DB functions — UUID: `UUID4`, `UUID7` | OPT | ref/models/database-functions.md — **6.1** |
| ORM-186 | DB functions — window: `Rank`, `DenseRank`, `RowNumber`, `Ntile`, `Lag`/`Lead`, `FirstValue`/`LastValue`/`NthValue`, `CumeDist`, `PercentRank` | OPT | ref/models/database-functions.md — paired with `Window` |
| ORM-187 | Custom `Func`/backend function support via `as_vendorname()` monkey-patch | DIY | ref/models/expressions.md, ref/models/database-functions.md |
| ORM-188 | `Manager.raw(raw_query, params, translations)` → `RawQuerySet` model instances (field mapping, deferred fields, indexing) | DIY | topics/db/sql.md, ref/models/querysets.md |
| ORM-189 | Direct cursor SQL via `connection.cursor()` / `cursor.execute(sql, params)` / `fetchone`/`fetchall`, dict/namedtuple helpers | DIY | topics/db/sql.md "Executing custom SQL directly" |
| ORM-190 | Stored procedures `cursor.callproc(procname, params, kparams)` | DIY | topics/db/sql.md — kparams Oracle-only |
| ORM-191 | `RawSQL` fragments embedded in `annotate`/`filter` (params required for injection safety) | DIY | topics/db/sql.md, ref/models/expressions.md |
| ORM-192 | `transaction.atomic(using, savepoint, durable)` — decorator/context manager, nesting, `durable=True` | CORE | topics/db/transactions.md |
| ORM-193 | `ATOMIC_REQUESTS` per-request transactions + `non_atomic_requests` decorator | OPT | topics/db/transactions.md |
| ORM-194 | `transaction.on_commit(func, using, robust)` post-commit callbacks | OPT | topics/db/transactions.md — test via `captureOnCommitCallbacks` |
| ORM-195 | Savepoints: `savepoint_create` (renamed from `savepoint` in **6.1**)/`savepoint_commit`/`savepoint_rollback`/`clean_savepoints`, `get_rollback`/`set_rollback` | OPT | topics/db/transactions.md |
| ORM-196 | Low-level autocommit control: `get_autocommit`/`set_autocommit`, `commit`/`rollback`, `AUTOCOMMIT` setting | OPT | topics/db/transactions.md |
| ORM-197 | Async transactions **not supported** (raise `SynchronousOnlyOperation`; use `sync_to_async`) | OPT | topics/db/queries.md, topics/db/transactions.md |
| ORM-198 | `using(alias)` — select DB per QuerySet; `Model.save(using=)`/`delete(using=)`, `db_manager()` | CORE | topics/db/multi-db.md, ref/models/querysets.md |
| ORM-199 | Database routers (`db_for_read`/`db_for_write`/`allow_relation`/`allow_migrate`, hints, `DATABASE_ROUTERS`) | DIY | topics/db/multi-db.md |
| ORM-200 | Async ORM: `async for` iteration, `a`-prefixed blocking methods (`aget`/`acreate`/`aupdate`/`adelete`/`acount`/`aexists`/`afirst`/`aiterator`/`aaggregate`…) | CORE | topics/db/queries.md "Asynchronous queries" — queryset-returning methods stay sync |
| ORM-201 | Fetch modes `FETCH_ONE`/`FETCH_PEERS`/`RAISE` for on-demand field loading; `FieldFetchBlocked` | OPT | topics/db/fetch-modes.md — **new 6.1**; `FETCH_PEERS` auto-solves N+1; set default via custom manager `get_queryset()` |
| ORM-202 | `explain(format, **flags)` / `aexplain()` query-plan inspection | OPT | ref/models/querysets.md, topics/db/optimization.md — all backends except Oracle |
| ORM-203 | `iterator(chunk_size)` / `aiterator()` streaming without result caching (server-side cursors on Postgres/Oracle) | OPT | ref/models/querysets.md, topics/db/optimization.md |
| ORM-204 | Optimization guidance: `exists`/`count`/`contains` vs `bool`/`len`/`in`, unique-indexed `get`, FK-value-direct, drop ordering | OPT | topics/db/optimization.md |
| ORM-205 | Query instrumentation: `connection.execute_wrapper(wrapper)` context manager (execute/sql/params/many/context) | DIY | topics/db/instrumentation.md |
| ORM-206 | Tablespaces: `Meta.db_tablespace`, `Field.db_tablespace`, `Index.db_tablespace`, `DEFAULT_TABLESPACE`/`DEFAULT_INDEX_TABLESPACE` | ECO | topics/db/tablespaces.md — Postgres/Oracle only, ignored elsewhere |

## P5 — Schema evolution: migrations & seeding

**Problem.** Evolve the database schema alongside the code, repeatably across environments, and seed data. **Answer.** Autodetected, per-app migration files committed as "schema version control": `makemigrations` diffs model state into operation graphs, `migrate` applies them with cross-app dependencies, reversal, squashing and fake-application; data migrations run through `RunPython` with historical models; fixtures (JSON/XML/YAML) cover seed data.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MIG-1 | Commands: `makemigrations` (`--name`, `--empty`, `--dry-run`), `migrate` (`--fake`, `--fake-initial`, `--prune`), `sqlmigrate`, `showmigrations`, `squashmigrations` (`--squashed-name`, `--no-optimize`) | CORE | topics/migrations.md — full CLI flags in CLI- |
| MIG-2 | Per-app migration files as VCS-committed "schema version control"; migrations made for every model change (even non-DB options) | CORE | topics/migrations.md |
| MIG-3 | `MIGRATION_MODULES` setting (relocate the migrations package per app) | OPT | topics/migrations.md |
| MIG-4 | Backend capability tiers: PostgreSQL best; MySQL no DDL transactions + index-size limits; SQLite emulated via table copy-rebuild (not production-advised) | CORE | topics/migrations.md |
| MIG-5 | Atomic migrations (single transaction on DDL-transaction backends; `Migration.atomic = False` to opt out; partial atomicity via `transaction.atomic` / `RunPython(atomic=True)`) | CORE | topics/migrations.md, howto/writing-migrations.md |
| MIG-6 | `Migration.dependencies` (cross-app auto-added, e.g. FK targets); single-app runs are best-effort | CORE | topics/migrations.md |
| MIG-7 | `Migration.run_before` (inverse dependency; prefer dependencies) | OPT | howto/writing-migrations.md |
| MIG-8 | `swappable_dependency("app.Model")` for swappable models (AUTH_USER_MODEL) | OPT | topics/migrations.md |
| MIG-9 | Migration file anatomy: `Migration` class, `dependencies`, `operations`, `initial = True`, `replaces`; hand-editable | CORE | topics/migrations.md |
| MIG-10 | Conflicting-number linearization prompt on VCS merges; history-consistency check (refuses inconsistent applied-dependency state) | CORE | topics/migrations.md |
| MIG-11 | Adding migrations to existing apps (`makemigrations` then `migrate --fake-initial`) | CORE | topics/migrations.md |
| MIG-12 | Reversing migrations (`migrate app 0002`, `migrate app zero`); `IrreversibleError` on irreversible ops | CORE | topics/migrations.md |
| MIG-13 | Historical models (versioned models in RunPython; no custom methods/constructors; managers only with `use_in_migrations`; concrete base classes stored as pointers — keep referenced functions/classes/fields importable) | CORE | topics/migrations.md |
| MIG-14 | Field deprecation via `system_check_deprecated_details` / `system_check_removed_details` (keep stub fields for old migrations) | DIY | topics/migrations.md |
| MIG-15 | Data migrations (`makemigrations --empty` + `RunPython(apps, schema_editor)`; runs at test-DB setup; cross-app model access requires explicit dependency) | CORE | topics/migrations.md |
| MIG-16 | Non-atomic batched data migrations (`atomic = False` + per-batch `transaction.atomic`) | OPT | howto/writing-migrations.md |
| MIG-17 | Multi-DB awareness in migrations: `schema_editor.connection.alias` check; router `allow_migrate` + `hints` (incl. `model_name` hint best practice) | OPT | howto/writing-migrations.md |
| MIG-18 | Squashing (`squashmigrations`; operation optimizer; RunSQL/RunPython block optimization unless `elidable`; `replaces` list; coexists with old files; transition-to-normal procedure; CircularDependencyError manual fix) | CORE | topics/migrations.md — squashing already-squashed migrations supported (**6.0**) |
| MIG-19 | Migration value serialization (ints/str/datetimes/Decimal/enums/UUID/functools.partial/pathlib/os.PathLike/LazyObject/TextChoices/fields/top-level functions & classes; NOT lambdas, nested classes, arbitrary instances) | CORE | topics/migrations.md — `zoneinfo.ZoneInfo` serialization added (**6.0**) |
| MIG-20 | Custom serializers (`MigrationWriter.register_serializer(type, BaseSerializer)`) | DIY | topics/migrations.md |
| MIG-21 | `deconstruct()` for custom classes + `@deconstructible` decorator (+ `__eq__` to avoid spurious migrations) | DIY | topics/migrations.md |
| MIG-22 | Custom-field migration compat (never change positional-arg count; keyword args for new options) | DIY | topics/migrations.md |
| MIG-23 | Third-party apps: run `makemigrations` on the lowest supported Django; backward- but not forward-compatible files | OPT | topics/migrations.md |
| MIG-24 | Recipe: adding unique non-nullable fields (AddField null=True → RunPython populate → AlterField unique) | OPT | howto/writing-migrations.md |
| MIG-25 | Recipe: migrating data between (third-party) apps with conditional dependencies + LookupError handling | OPT | howto/writing-migrations.md |
| MIG-26 | Recipe: converting M2M to a `through` model via SeparateDatabaseAndState table rename | OPT | howto/writing-migrations.md |
| MIG-27 | Recipe: unmanaged→managed model (flip `managed` in its own migration first) | OPT | howto/writing-migrations.md |
| MIG-28 | `SchemaEditor` via `connection.schema_editor()` context manager — DDL abstraction: `execute`, `create_model`, `delete_model`, `add/remove/rename_index`, `add/remove_constraint`, `alter_unique_together`, `alter_index_together`, `alter_db_table(_comment)`, `alter_db_tablespace`, `add/remove/alter_field` (combined ALTERs when supported; M2M↔regular field refused) | OPT | ref/schema-editor.md — third-party backends implement it to gain migrations (DIY extension point) |
| MIG-29 | `CreateModel(name, fields, options, bases, managers)` | CORE | ref/migration-operations.md — bases may be historical string refs |
| MIG-30 | `DeleteModel(name)` / `RenameModel(old, new)` (manual RenameModel needed when renaming model + many fields at once) | CORE | ref/migration-operations.md |
| MIG-31 | `AlterModelTable(name, table)` / `AlterModelTableComment(name, table_comment)` | CORE | ref/migration-operations.md |
| MIG-32 | `AlterUniqueTogether(name, unique_together)` | CORE | ref/migration-operations.md |
| MIG-33 | `AlterIndexTogether` (legacy — pre-4.2 files only; use AddIndex/RemoveIndex) | OPT | ref/migration-operations.md |
| MIG-34 | `AlterOrderWithRespectTo(name, order_with_respect_to)` (`_order` column) | OPT | ref/migration-operations.md |
| MIG-35 | `AlterModelOptions(name, options)` (state-only Meta changes) / `AlterModelManagers(name, managers)` | CORE | ref/migration-operations.md |
| MIG-36 | `AddField(model_name, name, field, preserve_default=True)` (temp defaults; old-DB full-table-rewrite warning + two-step workaround) | CORE | ref/migration-operations.md |
| MIG-37 | `RemoveField(model_name, name)` (reverse = AddField; irreversible if non-null and defaultless) | CORE | ref/migration-operations.md — no longer CASCADEs to dependent DB objects e.g. views (**6.0**) |
| MIG-38 | `AlterField(model_name, name, field, preserve_default)` (type/null/unique/db_column changes; cross-type limits per backend) | CORE | ref/migration-operations.md |
| MIG-39 | `RenameField(model_name, old_name, new_name)` | CORE | ref/migration-operations.md |
| MIG-40 | `AddIndex(model_name, index)` / `RemoveIndex(model_name, name)` / `RenameIndex(model_name, new_name, old_name/old_fields)` (SQLite drop+recreate) | CORE | ref/migration-operations.md |
| MIG-41 | `AddConstraint(model_name, constraint)` / `RemoveConstraint(model_name, name)` / `AlterConstraint(model_name, name, constraint)` (state-only, no DB change) | CORE | ref/migration-operations.md |
| MIG-42 | `RunSQL(sql, reverse_sql, state_operations, hints, elidable)` (str / list / 2-tuples with params; `RunSQL.noop`; irreversible without reverse_sql; BEGIN/COMMIT caveat in non-atomic migrations) | CORE | ref/migration-operations.md |
| MIG-43 | `RunPython(code, reverse_code, atomic, hints, elidable)` (historical apps registry + schema_editor args; `RunPython.noop`; auto-transaction on non-DDL-transaction backends; must use `schema_editor.connection.alias` for non-default DB) | CORE | ref/migration-operations.md |
| MIG-44 | `SeparateDatabaseAndState(database_operations, state_operations)` (decouple schema SQL from autodetector state; data-loss caution) | OPT | ref/migration-operations.md |
| MIG-45 | `elidable` flag on RunSQL/RunPython (allow removal during squashing) | OPT | ref/migration-operations.md, topics/migrations.md |
| MIG-46 | `OperationCategory` symbols (+, -, ~, p, s, ?) shown by makemigrations | OPT | ref/migration-operations.md |
| MIG-47 | Custom `Operation` subclasses (`state_forwards`, `database_forwards/backwards`, `reversible`, `reduces_to_sql`, `category`, `describe`, `migration_name_fragment`; `clear_delayed_apps_cache()` for related models; never mutate reused `ModelState.fields`/`managers` instances) | DIY | ref/migration-operations.md — example: PostgreSQL `CREATE EXTENSION` operation |
| MIG-48 | Initial data via data migrations (auto-applied incl. test-database setup) | CORE | howto/initial-data.md |
| MIG-49 | Fixtures: serialized DB contents in JSON/XML/YAML; produced by `dumpdata` or handwritten | CORE | howto/initial-data.md, topics/db/fixtures.md — YAML requires PyYAML (ECO) |
| MIG-50 | `loaddata <fixturename>` (re-running overwrites changed rows); `TestCase.fixtures = [...]` | CORE | howto/initial-data.md, topics/db/fixtures.md |
| MIG-51 | Fixture discovery: `<app>/fixtures/`, `FIXTURE_DIRS`, literal/absolute path; directory components in names; namespacing advice; extension selects serializer format | CORE | topics/db/fixtures.md, howto/initial-data.md |
| MIG-52 | Fixture loading order (listed order; per name, all apps in INSTALLED_APPS order); cross-fixture FK caveat without deferred constraint checking | CORE | topics/db/fixtures.md |
| MIG-53 | Raw loading semantics: model `save()` not called; pre_save/post_save/m2m_changed signals fire with `raw=True` (disable-handler pattern) | CORE | topics/db/fixtures.md — `raw` arg on `m2m_changed` added (**6.1**) |
| MIG-54 | Compressed fixtures (`zip`, `gz`, `bz2`, `lzma`, `xz`); same-name/different-format conflict aborts load | OPT | topics/db/fixtures.md |
| MIG-55 | Database-specific fixtures (`mydata.users.json` loads only into the `users` DB alias) | OPT | topics/db/fixtures.md |
| MIG-56 | MyISAM caveat (no transactions/constraints → no validation/rollback) | OPT | topics/db/fixtures.md |

## P6 — Validation & data integrity

**Problem.** Validate untrusted input, surface errors back to users, and keep bad data out of the database. **Answer.** The forms system *is* Django's validation layer: declarative Form/ModelForm classes run a per-field pipeline (`to_python → validate → run_validators`), per-field `clean_<field>` hooks and cross-field `clean()`, with a widget layer for rendering and formsets for collections; reusable validator callables are shared with model fields; model-level `full_clean()` and DB constraints (ORM-69…78) back-stop the database.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| VAL-1 | Declare a `Form` subclass with fields as class attributes mapping to HTML inputs | CORE | topics/forms/index.md, ref/forms/fields.md — `class NameForm(forms.Form): your_name = forms.CharField()` |
| VAL-2 | Bound vs unbound instances; `Form(data)` binds, `is_bound` attribute; data immutable once constructed | CORE | ref/forms/api.md — empty dict `{}` still creates a bound form |
| VAL-3 | `is_valid()` runs validation → bool; populates `cleaned_data`; `errors` dict; validation runs once and is cached | CORE | ref/forms/api.md, topics/forms/index.md — `cleaned_data` holds Python-typed normalized values, only declared fields |
| VAL-4 | Machine-readable error output: `errors.as_data()`, `errors.as_json(escape_html)`, `errors.get_json_data()` | OPT | ref/forms/api.md — as_json emits `{message, code}` per error, benefits from ValidationError codes/params |
| VAL-5 | `add_error(field, error)`, `has_error(field, code)`, `non_field_errors()` (`__all__`/NON_FIELD_ERRORS) | OPT | ref/forms/api.md — add_error removes the field from cleaned_data |
| VAL-6 | Initial values: form-level `initial=` dict and field-level `Field.initial`; form-level wins; callables evaluated; only shown on unbound forms, not a validation fallback | CORE | ref/forms/api.md, ref/forms/fields.md — `BoundField.initial` caches, preferred over `get_initial_for_field` |
| VAL-7 | `has_changed()` / `changed_data` compare bound data against `initial` | OPT | ref/forms/api.md — per-field via `Field.has_changed` |
| VAL-8 | `prefix=` namespaces field `name`/`id` so multiple forms share one `<form>` | OPT | ref/forms/api.md — settable as class attr |
| VAL-9 | Default rendering wraps fields in `<div>` via `print(form)`/`{{ form }}`; excludes `<form>` tags & submit | CORE | ref/forms/api.md, topics/forms/index.md |
| VAL-10 | Output-style shortcuts `as_div()` (recommended, uses fieldset/legend), `as_p()`, `as_ul()`, `as_table()` | CORE | ref/forms/api.md — each pairs a method with a `template_name_*` attribute |
| VAL-11 | Template-based form rendering: `Form.template_name`, `render(template_name,…)`, `get_context()`; reusable form templates via custom `FORM_RENDERER` or per-form/per-instance | OPT | topics/forms/index.md, ref/forms/api.md |
| VAL-12 | Reusable field-group templates: `BoundField.as_field_group()` renders label+widget+errors+help via `Field.template_name` (default `django/forms/field.html`) | OPT | topics/forms/index.md, ref/forms/fields.md |
| VAL-13 | Manual/granular rendering via `BoundField` attrs (`errors`, `label_tag`, `legend_tag`, `id_for_label`, `value`, `use_fieldset`, `widget_type`); loop `{% for field in form %}`, `hidden_fields()`/`visible_fields()` | OPT | topics/forms/index.md, ref/forms/api.md |
| VAL-14 | Error styling hooks: `error_css_class`, `required_css_class`; `errorlist`/`errorlist nonfield`/`nonform` CSS classes; `aria-invalid`/`aria-describedby` accessibility | OPT | ref/forms/api.md, topics/forms/index.md, topics/forms/formsets.md |
| VAL-15 | ID/label config: `auto_id` (True/False/`'%s'` format), `label_suffix`, `use_required_attribute` | OPT | ref/forms/api.md |
| VAL-16 | Field ordering: `field_order` attr/arg and `order_fields()` | OPT | ref/forms/api.md |
| VAL-17 | Form subclassing/mixins; declaratively drop an inherited field by setting its name to `None` | OPT | ref/forms/api.md |
| VAL-18 | Custom `BoundField` via `Form.bound_field_class` / `Field.bound_field_class` / `get_bound_field` | DIY | ref/forms/api.md — e.g. add template properties/CSS |
| VAL-19 | Customizable `ErrorList` (error_class, renderer, field_id, `as_text()`/`as_ul()`, template overrides) | DIY | ref/forms/api.md |
| VAL-20 | Core field args on every `Field`: `required`, `label`, `label_suffix`, `initial`, `widget`, `help_text`, `error_messages`, `validators`, `localize`, `disabled`, `template_name` | CORE | ref/forms/fields.md — each field also defines error-message keys |
| VAL-21 | Text-ish fields: `CharField` (max/min_length, strip, empty_value), `EmailField` (max_length 320), `URLField` (assume_scheme), `SlugField` (allow_unicode), `RegexField` (regex), `GenericIPAddressField` (protocol/unpack_ipv4) | CORE | ref/forms/fields.md |
| VAL-22 | Numeric fields: `IntegerField`, `FloatField`, `DecimalField` (max_digits/decimal_places/step_size), `DurationField`; use Max/MinValue & StepValue validators | CORE | ref/forms/fields.md — NumberInput default unless localized |
| VAL-23 | Date/time fields: `DateField`, `DateTimeField`, `TimeField` (input_formats), `SplitDateTimeField` (input_date/time_formats) | CORE | ref/forms/fields.md — DateTimeField accepts ISO 8601 |
| VAL-24 | Choice fields: `ChoiceField`, `TypedChoiceField` (coerce/empty_value), `MultipleChoiceField`, `TypedMultipleChoiceField`; `choices` accepts iterable/enum/callable | CORE | ref/forms/fields.md — ChoiceField normalizes to strings, use Typed* for other types |
| VAL-25 | `FilePathField` (path/recursive/match/allow_files/allow_folders; `set_choices()` added **6.1**) | OPT | ref/forms/fields.md |
| VAL-26 | Boolean fields: `BooleanField` (needs `required=False` to allow unchecked), `NullBooleanField` (validates nothing) | CORE | ref/forms/fields.md |
| VAL-27 | `UUIDField` (→ uuid.UUID) and `JSONField` (encoder/decoder, Textarea default) | OPT | ref/forms/fields.md |
| VAL-28 | Combo/multi-value: `ComboField` (validate against list of fields), `MultiValueField` (abstract, subclass `compress()`, `require_all_fields`, pairs with `MultiWidget`) | DIY | ref/forms/fields.md |
| VAL-29 | Relationship fields: `ModelChoiceField` (queryset, empty_label default changed to "- Select an option -" in **6.1**, to_field_name, blank, label_from_instance), `ModelMultipleChoiceField` (→ QuerySet); `ModelChoiceIterator`/`ModelChoiceIteratorValue` | CORE | ref/forms/fields.md — for FK / M2M in ModelForms |
| VAL-30 | Custom `Field` subclass (implement `clean()`, accept core args) | DIY | ref/forms/fields.md |
| VAL-31 | Assign widget via `widget=` arg; widgets render HTML & extract data (`value_from_datadict`, `value_omitted_from_data`) | CORE | ref/forms/widgets.md |
| VAL-32 | Text-input widget family: `TextInput`, `NumberInput`, `EmailInput`, `URLInput`, `ColorInput`, `SearchInput`, `TelInput`, `PasswordInput` (render_value), `Textarea`, `DateInput`/`DateTimeInput`/`TimeInput` (format) | CORE | ref/forms/widgets.md |
| VAL-33 | Select/radio/checkbox widgets: `Select`, `SelectMultiple`, `NullBooleanSelect`, `RadioSelect` (loopable), `CheckboxSelectMultiple`, `CheckboxInput` (check_test); `Select.choices` sync from field | CORE | ref/forms/widgets.md |
| VAL-34 | Hidden widgets: `HiddenInput`, `MultipleHiddenInput` | OPT | ref/forms/widgets.md |
| VAL-35 | File upload widgets: `FileInput`, `ClearableFileInput` (adds clear checkbox when optional+has initial) | CORE | ref/forms/widgets.md |
| VAL-36 | Composite widgets: `MultiWidget` (widgets list/dict, `decompress()`), `SplitDateTimeWidget`, `SplitHiddenDateTimeWidget`, `SelectDateWidget` (years/months/empty_label) | OPT | ref/forms/widgets.md |
| VAL-37 | Customize widget instances via `Widget.attrs` (per-instance CSS/id/type, boolean attrs) | OPT | ref/forms/widgets.md |
| VAL-38 | Customize widget classes/templates: each widget `template_name`/`option_template_name`; override under `django/forms/widgets/…` (requires TemplatesSetting renderer) | DIY | ref/forms/widgets.md, ref/forms/renderers.md |
| VAL-39 | Widget assets via `Media` inner class / `media` property | OPT | ref/forms/widgets.md — Media system detail at VIEW-43 |
| VAL-40 | Field validation pipeline: `to_python()` → `validate()` → `run_validators()` orchestrated by `Field.clean()`, in field declaration order | CORE | ref/forms/validation.md |
| VAL-41 | `clean_<fieldname>()` per-field hook (reads/returns `self.cleaned_data[...]`) | CORE | ref/forms/validation.md |
| VAL-42 | `Form.clean()` cross-field validation; errors go to `__all__` non-field, or attach via `add_error()` | CORE | ref/forms/validation.md, ref/forms/api.md |
| VAL-43 | `ValidationError` best practices: `code`, `params` placeholders, `gettext`, lists for multiple errors | OPT | ref/forms/validation.md |
| VAL-44 | Attach validators to fields via `validators=` arg or `default_validators` class attr | OPT | ref/forms/validation.md, ref/validators.md |
| VAL-45 | Regex/format validators: `RegexValidator` (inverse_match/flags), `EmailValidator` (allowlist), `URLValidator` (schemes/max_length), `DomainNameValidator` (accept_idna) | OPT | ref/validators.md |
| VAL-46 | Slug/IP/list validator instances: `validate_slug`, `validate_unicode_slug`, `validate_ipv4/ipv6/ipv46_address`, `validate_comma_separated_integer_list`, `int_list_validator`, `validate_email`, `validate_domain_name` | OPT | ref/validators.md |
| VAL-47 | Range/length/decimal validators: `MaxValueValidator`, `MinValueValidator`, `MaxLengthValidator`, `MinLengthValidator`, `DecimalValidator`, `StepValueValidator` (offset) | OPT | ref/validators.md |
| VAL-48 | File & safety validators: `FileExtensionValidator`, `validate_image_file_extension` (Pillow), `ProhibitNullCharactersValidator` | OPT | ref/validators.md — image extension validator requires Pillow |
| VAL-49 | Write custom validators (callable raising ValidationError; class w/ `__call__`+`deconstruct`/`__eq__` for migrations) | DIY | ref/validators.md, ref/forms/validation.md |
| VAL-50 | `ModelForm` with inner `Meta` (`model`, `fields`/`'__all__'`/`exclude`); model→form field mapping table | CORE | topics/forms/modelforms.md, ref/forms/models.md — explicit `fields` strongly recommended for security |
| VAL-51 | ModelForm `Meta` customization: `widgets`, `labels`, `help_texts`, `error_messages`, `field_classes`, `formfield_callback`, `localized_fields` | OPT | topics/forms/modelforms.md, ref/forms/models.md |
| VAL-52 | `ModelForm.save(commit=True)` creates/updates instance; `save_m2m()` needed after `commit=False` with M2M | CORE | topics/forms/modelforms.md — raises ValueError if data invalid |
| VAL-53 | ModelForm validation integrates model validation: form clean then model `full_clean(validate_unique=False, validate_constraints=False)` then `validate_unique`/`validate_constraints`; override `clean()` must call super to keep uniqueness | CORE | topics/forms/modelforms.md |
| VAL-54 | Model validation pipeline: `full_clean(exclude, validate_unique, validate_constraints)` → `clean_fields()` → `clean()` → `validate_unique()` → `validate_constraints()`; NOT called by `save()`; ValidationError dicts / `message_dict` / NON_FIELD_ERRORS | CORE | ref/models/instances.md (cited by both the model and forms inventories) |
| VAL-55 | `modelform_factory(model, fields=…, widgets=…, …)` builds a ModelForm without a class def | OPT | topics/forms/modelforms.md, ref/forms/models.md |
| VAL-56 | `formset_factory(form, extra, max_num, min_num, absolute_max, can_order, can_delete, can_delete_extra, validate_max, validate_min, renderer)` | OPT | topics/forms/formsets.md, ref/forms/formsets.md |
| VAL-57 | Formset `initial=` list drives pre-filled forms alongside `extra` blanks; `empty_form` (`__prefix__`) for JS | OPT | topics/forms/formsets.md |
| VAL-58 | Formset `is_valid()`/`errors` list, `total_error_count()`, `has_changed()`; cross-form `clean()` → `non_form_errors()` | OPT | topics/forms/formsets.md |
| VAL-59 | `ManagementForm` (TOTAL_FORMS/INITIAL_FORMS/MIN/MAX); `total_form_count`/`initial_form_count`; render `{{ formset.management_form }}` | CORE | topics/forms/formsets.md — required or the formset is invalid |
| VAL-60 | Ordering/deletion: `can_order` (ORDER field, `ordered_forms`, ordering_widget), `can_delete` (DELETE field, `deleted_forms`/`deleted_objects`, deletion_widget), `can_delete_extra` | OPT | topics/forms/formsets.md |
| VAL-61 | Number-of-forms validation `validate_max`/`validate_min`; customizable `error_messages` (too_few/too_many/missing_management_form) | OPT | topics/forms/formsets.md |
| VAL-62 | Formset extras: `add_fields()`, `form_kwargs`/`get_form_kwargs()`, custom `prefix`, multiple formsets per view | OPT | topics/forms/formsets.md |
| VAL-63 | Formset rendering `{{ formset }}` / `as_div`/`as_p`/`as_ul`/`as_table`, `template_name_*`, `renderer` | OPT | topics/forms/formsets.md |
| VAL-64 | `modelformset_factory` (queryset, `edit_only`, `save()` → new/changed/deleted_objects, save_m2m); override `clean()` must call super and mutate `form.instance` | OPT | topics/forms/modelforms.md, ref/forms/models.md |
| VAL-65 | `inlineformset_factory(parent_model, model, fk_name=…)` for FK-related editing (defaults can_delete=True, extra=3) | OPT | topics/forms/modelforms.md, ref/forms/models.md |
| VAL-66 | Uploaded files arrive in `request.FILES` (needs POST + `enctype="multipart/form-data"`); bind via `Form(request.POST, request.FILES)`; `Form.is_multipart()` | CORE | topics/http/file-uploads.md, ref/forms/api.md — upload-handler machinery in FILE- |
| VAL-67 | ModelForm/model saves the file to `FileField.upload_to` on `form.save()`; or assign `request.FILES[...]`/`ContentFile` directly | OPT | topics/http/file-uploads.md |
| VAL-68 | Multiple-file upload: subclass widget with `allow_multiple_selected=True` + subclass `FileField.clean()` (form-level only, not model) | DIY | topics/http/file-uploads.md |

## P7 — Authentication & authorization

**Problem.** Identify users and control what they may do. **Answer.** `django.contrib.auth`: a swappable User model with session-based login, pluggable authentication backends behind `authenticate()`, model-level permissions with groups, view guards at every granularity (decorators, CBV mixins, site-wide middleware), packaged auth/password-reset views and forms, and a tunable, auto-upgrading password-hashing stack. Nearly everything here is OPT because it lives in contrib — but startproject enables it by default.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| AUTH-1 | `User` model with fields `username`, `password`, `email`, `first_name`, `last_name`, `is_staff`, `is_active`, `is_superuser`, `last_login`, `date_joined`, plus M2M `groups`/`user_permissions` | OPT | ref/contrib/auth.md, topics/auth/default.md — requires `django.contrib.auth` + `contenttypes` in INSTALLED_APPS |
| AUTH-2 | User password methods `set_password`, `check_password`/`acheck_password`, `set_unusable_password`, `has_usable_password`; identity `get_username`/`get_full_name`/`get_short_name`; `email_user()` | OPT | ref/contrib/auth.md — async `acheck_password` |
| AUTH-3 | `UserManager.create_user`/`create_superuser` (+ `acreate_user`/`acreate_superuser`) and `with_perm()`; `createsuperuser`/`changepassword` mgmt commands | OPT | ref/contrib/auth.md, topics/auth/default.md — command flags in CLI- |
| AUTH-4 | `authenticate()`/`aauthenticate()` low-level credential check across `AUTHENTICATION_BACKENDS` | OPT | topics/auth/default.md — returns User or None; sets `user.backend` |
| AUTH-5 | `login()`/`alogin()` and `logout()`/`alogout()` session helpers | OPT | topics/auth/default.md — logout flushes the session |
| AUTH-6 | `AnonymousUser` object implementing the User interface (id None, `is_authenticated` False) | OPT | ref/contrib/auth.md |
| AUTH-7 | `request.user` (sync) / `request.auser()` (async) via `AuthenticationMiddleware`; `is_authenticated`/`is_anonymous` attributes | OPT | topics/auth/default.md, ref/contrib/auth.md, ref/middleware.md — middleware must follow SessionMiddleware |
| AUTH-8 | `get_user_model()` / `settings.AUTH_USER_MODEL` for referencing the swappable user; `get_user()`/`aget_user()` utility | OPT | topics/auth/customizing.md, ref/contrib/auth.md |
| AUTH-9 | Auth views `LoginView`, `LogoutView`, `logout_then_login()`, `redirect_to_login()` (class-based, customizable forms/templates/`success_url_allowed_hosts`) | OPT | topics/auth/default.md — include via `django.contrib.auth.urls` |
| AUTH-10 | Password-management views `PasswordChangeView`, `PasswordChangeDoneView`, `PasswordResetView`, `PasswordResetDoneView`, `PasswordResetConfirmView`, `PasswordResetCompleteView` (token via `PasswordResetTokenGenerator`, optional `post_reset_login`, HTML email) | OPT | topics/auth/default.md — no async support noted for these views |
| AUTH-11 | Built-in forms: `AuthenticationForm` (`confirm_login_allowed`), `BaseUserCreationForm`/`UserCreationForm`, `AdminUserCreationForm`, `AdminPasswordChangeForm`, `PasswordChangeForm`, `SetPasswordForm`, `PasswordResetForm` (`send_mail`), `UserChangeForm` | OPT | topics/auth/default.md |
| AUTH-12 | Session invalidation on password change: `get_session_auth_hash` (HMAC of password) + `update_session_auth_hash()`/`aupdate_session_auth_hash()` | OPT | topics/auth/default.md — rotates the session key; ties to `SECRET_KEY_FALLBACKS` |
| AUTH-13 | Login/logout signals `user_logged_in`, `user_logged_out`, `user_login_failed` | OPT | ref/contrib/auth.md — `django.contrib.auth.signals` |
| AUTH-14 | Username field validators `ASCIIUsernameValidator`, `UnicodeUsernameValidator` (default) | OPT | ref/contrib/auth.md |
| AUTH-15 | Auth template context `{{ user }}`, `{{ perms }}` (`PermWrapper`) via the `auth` context processor | OPT | topics/auth/default.md |
| AUTH-16 | Remote/external auth: `RemoteUserMiddleware`, `PersistentRemoteUserMiddleware`, `RemoteUserBackend` (+ `AllowAllUsersRemoteUserBackend`), custom `header`/`clean_username`/`configure_user` | OPT | howto/auth-remote-user.md, ref/contrib/auth.md, ref/middleware.md — reads web-server `REMOTE_USER`; persistent variant for login-page-only setups |
| AUTH-17 | Async auth backend interface `aget_user`/`aauthenticate` auto-synthesized via `sync_to_async` for `BaseBackend` subclasses | OPT | topics/auth/customizing.md |
| AUTH-18 | Permissions framework + `Permission` model (`name`, `content_type`, `codename`); default add/change/delete/view perms per model created on `migrate` | OPT | topics/auth/default.md, ref/contrib/auth.md |
| AUTH-19 | `Permission.user_perm_str` helper property; rename-tracking of default perms | OPT | ref/contrib/auth.md, topics/auth/default.md — **new in 6.1** |
| AUTH-20 | `Group` model with M2M `permissions`; user `groups`/`user_permissions` set/add/remove/clear | OPT | ref/contrib/auth.md, topics/auth/default.md |
| AUTH-21 | Permission-check methods `has_perm`/`has_perms`/`has_module_perms` and `get_user_permissions`/`get_group_permissions`/`get_all_permissions` (all with `a*` async + optional `obj`) | OPT | ref/contrib/auth.md — per-object `obj` param hooks exist but core returns empty |
| AUTH-22 | Custom permissions via model `Meta.permissions`; programmatic `Permission.objects.create` with `ContentType` | OPT | topics/auth/customizing.md, topics/auth/default.md |
| AUTH-23 | Proxy-model permissions (own content type, `for_concrete_model=False`); permission caching on `ModelBackend` | OPT | topics/auth/default.md |
| AUTH-24 | `login_required` decorator + `login_not_required` (disables `LoginRequiredMiddleware` per view) | OPT | topics/auth/default.md — `redirect_field_name`/`login_url` args |
| AUTH-25 | `LoginRequiredMiddleware` — auth required on all views by default | OPT | topics/auth/default.md, ref/middleware.md — redirects unauthenticated requests to `LOGIN_URL`; opt-out per view with `login_not_required`; subclass hooks `redirect_field_name` (default `"next"`), `get_login_url()`, `get_redirect_field_name()`; must follow AuthenticationMiddleware; ensure the login view itself is exempt |
| AUTH-26 | `permission_required` decorator (`raise_exception`) and `user_passes_test(test_func)` decorator | OPT | topics/auth/default.md |
| AUTH-27 | CBV mixins `LoginRequiredMixin`, `PermissionRequiredMixin` (`get_permission_required`/`has_permission`), `UserPassesTestMixin` (`test_func`/`get_test_func`), base `AccessMixin` (`raise_exception`, `handle_no_permission`) | OPT | topics/auth/default.md, topics/class-based-views/generic-editing.md, intro.md — `django.contrib.auth.mixins` |
| AUTH-28 | Custom authentication backends: `BaseBackend`, `ModelBackend` (default), `AllowAllUsersModelBackend`, plus per-object/anon/inactive-user authorization hooks; `AUTHENTICATION_BACKENDS` setting | OPT/DIY | ref/contrib/auth.md, topics/auth/customizing.md |
| AUTH-29 | Custom user model: `AbstractBaseUser` (+ `USERNAME_FIELD`/`EMAIL_FIELD`/`REQUIRED_FIELDS`, `get_session_auth_hash`), `AbstractUser`, `PermissionsMixin`, `BaseUserManager`/`UserManager` | DIY | topics/auth/customizing.md — `AUTH_USER_MODEL` set before the first migration |
| AUTH-30 | Configurable password storage, PBKDF2-SHA256 default, `<algorithm>$<iterations>$<salt>$<hash>` format; `PASSWORD_HASHERS` setting | OPT | topics/auth/passwords.md — part of contrib.auth |
| AUTH-31 | Included hashers: `PBKDF2PasswordHasher`, `PBKDF2SHA1PasswordHasher`, `ScryptPasswordHasher`, `MD5PasswordHasher` | OPT | topics/auth/passwords.md — scrypt needs OpenSSL 1.1+ |
| AUTH-32 | `Argon2PasswordHasher` (Argon2id) and `BCryptSHA256PasswordHasher`/`BCryptPasswordHasher` | ECO | topics/auth/passwords.md — need `argon2-cffi` / `bcrypt` third-party libs |
| AUTH-33 | Automatic password upgrading on login; work-factor tuning (`iterations`/`rounds`/`time_cost`/`memory_cost`/`parallelism`); `salt_entropy` | OPT/DIY | topics/auth/passwords.md — never remove hashers from the list |
| AUTH-34 | Wrapped-hasher upgrade-without-login pattern; custom hasher `harden_runtime` (timing-attack mitigation) | DIY | topics/auth/passwords.md |
| AUTH-35 | Standalone hasher utils `check_password`/`acheck_password`, `make_password`, `is_password_usable` | OPT | topics/auth/passwords.md — usable independent of the User model |
| AUTH-36 | Password validation via `AUTH_PASSWORD_VALIDATORS`; built-ins `MinimumLengthValidator`, `UserAttributeSimilarityValidator`, `CommonPasswordValidator` (20k list), `NumericPasswordValidator` | OPT | topics/auth/passwords.md — default empty; enabled in the startproject template |
| AUTH-37 | Integration functions `validate_password`, `password_changed`, `password_validators_help_texts`/`_html`, `get_password_validators`; custom validator interface | OPT/DIY | topics/auth/passwords.md |

## P8 — Security

**Problem.** Blunt the standard web attack classes (XSS, SQLi, CSRF, clickjacking, host poisoning, transport downgrade). **Answer.** Secure-by-default framework behavior plus a stack of dedicated middleware, all present in the startproject template: template autoescaping, parameterized queries, CSRF tokens with Origin checking, `SecurityMiddleware` headers (HSTS, nosniff, referrer policy, COOP), host-header validation, CSP support (new in 6.0) and a signing API for tamper-proof round-trips.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| SEC-1 | XSS protection via template auto-escaping (`mark_safe`/`safe`/autoescape caveats) | CORE | topics/security.md — escaping mechanics at VIEW-17 |
| SEC-2 | SQL injection protection via queryset parameterization; caution on `raw()`/`extra()`/`RawSQL` | CORE | topics/security.md |
| SEC-3 | Clickjacking protection: `XFrameOptionsMiddleware` + `X_FRAME_OPTIONS` (DENY default) | CORE | ref/clickjacking.md, ref/middleware.md — `django.middleware.clickjacking`, in startproject |
| SEC-4 | Per-view X-Frame decorators `xframe_options_deny`, `xframe_options_sameorigin`, `xframe_options_exempt` | CORE | ref/clickjacking.md |
| SEC-5 | CSRF protection: `CsrfViewMiddleware` + `{% csrf_token %}` (secret cookie, per-response BREACH masking, Origin/Referer + `CSRF_TRUSTED_ORIGINS` checks, token rotation on login) | CORE | ref/csrf.md, howto/csrf.md, ref/middleware.md — in startproject; tag row at VIEW-11 |
| SEC-6 | CSRF AJAX support via `X-CSRFToken` header (`CSRF_HEADER_NAME`); `CSRF_USE_SESSIONS`; Jinja2 `{{ csrf_input }}` | CORE | howto/csrf.md |
| SEC-7 | CSRF decorators `csrf_exempt`, `csrf_protect`, `requires_csrf_token`, `ensure_csrf_cookie`; `CSRF_FAILURE_VIEW`; full `CSRF_COOKIE_*` settings suite | CORE | ref/csrf.md, howto/csrf.md |
| SEC-8 | `SecurityMiddleware` header/redirect suite: HSTS (`SECURE_HSTS_SECONDS`/`_INCLUDE_SUBDOMAINS`/`_PRELOAD`), `SECURE_SSL_REDIRECT` + `SECURE_SSL_HOST` + `SECURE_REDIRECT_EXEMPT`, `SECURE_REFERRER_POLICY` (8 documented values), `SECURE_CROSS_ORIGIN_OPENER_POLICY` (same-origin default), `SECURE_CONTENT_TYPE_NOSNIFF`, `SECURE_PROXY_SSL_HEADER` behind proxies | CORE | ref/middleware.md, topics/security.md — prefer the front-end server where possible; secure cookies via `SESSION_COOKIE_SECURE`/`CSRF_COOKIE_SECURE` |
| SEC-9 | Host-header validation against `ALLOWED_HOSTS` in `HttpRequest.get_host()`; `USE_X_FORWARDED_HOST` | CORE | topics/security.md |
| SEC-10 | Cryptographic signing: `Signer` (`sign`/`unsign`, `sign_object`/`unsign_object`, `salt`, `algorithm`), `BadSignature` | CORE | topics/signing.md — `django.core.signing`; swappable via `SIGNING_BACKEND` (ref/settings.md) |
| SEC-11 | `TimestampSigner` (`max_age` expiry, `SignatureExpired`) and module `dumps()`/`loads()` shortcuts (URL-safe signed JSON) | CORE | topics/signing.md |
| SEC-12 | Secret-key rotation: `SECRET_KEY` + `SECRET_KEY_FALLBACKS` (validate-only) used by signing/sessions/auth hash | CORE | topics/signing.md, topics/security.md |
| SEC-13 | Content Security Policy: `ContentSecurityPolicyMiddleware` + `SECURE_CSP`/`SECURE_CSP_REPORT_ONLY` | CORE | ref/csp.md, howto/csp.md, ref/middleware.md — **new in 6.0**; `django.middleware.csp`; place after anything reading the lazy `csp_nonce` |
| SEC-14 | CSP nonces: `CSP.NONCE` sentinel, `csp` context processor → `{{ csp_nonce }}`, `csp_nonce_attr` tag (**6.1**, supports `Media`); lazy generation + caching caveats | CORE | ref/csp.md, howto/csp.md — tag row at VIEW-18 |
| SEC-15 | CSP constants enum `django.utils.csp.CSP` (SELF/NONE/UNSAFE_INLINE/STRICT_DYNAMIC etc.) and per-view `csp_override`/`csp_report_only_override` decorators | CORE | ref/csp.md |
| SEC-16 | Brute-force throttling, object-level permissions, OAuth/3rd-party auth explicitly not provided | ECO/DIY | topics/auth/index.md, topics/security.md — use third-party packages |

## P9 — Background work

**Problem.** Run work outside the request/response cycle. **Answer.** The Tasks framework (new in 6.0): a `@task` decorator + `.enqueue()` over pluggable backends with results, priorities, queues and deferred execution — but Django deliberately ships only the definition/queueing/result API and two dev/test backends. The worker process and durable queue come from the ecosystem, and there is no scheduler/cron in core.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| JOB-1 | `@task(*, priority=, queue_name=, backend=, takes_context=, **kwargs)` decorator on a module-level fn → `Task` | CORE | topics/tasks.md, ref/tasks.md — **new in 6.0** (arbitrary `**kwargs` added **6.1**); `django-tasks` backport for older Django |
| JOB-2 | `Task.enqueue()`/`aenqueue()` (JSON-serializable args/return), `on_commit` transaction pattern | CORE | topics/tasks.md, ref/tasks.md — **new in 6.0** |
| JOB-3 | `Task.using(*, priority/backend/queue_name/run_after)` immutable modify; `run_after` defer | CORE | ref/tasks.md — **new in 6.0**; `run_after` needs `supports_defer` |
| JOB-4 | `TaskContext` (`takes_context=True`) exposing `task_result`/`attempt` | CORE | topics/tasks.md, ref/tasks.md — **new in 6.0** |
| JOB-5 | `TaskResult` (`id`/`status`/`return_value`/`errors`/`enqueued_at`/`attempts`/`worker_ids`, `refresh`/`arefresh`) + `get_result`/`aget_result` | CORE | topics/tasks.md, ref/tasks.md — **new in 6.0** |
| JOB-6 | `TaskResultStatus` enum READY/RUNNING/FAILED/SUCCESSFUL; `TaskError` (`traceback`/`exception_class`) | CORE | ref/tasks.md — **new in 6.0** |
| JOB-7 | `ImmediateBackend` (`backends.immediate`) runs tasks synchronously (default, dev/tests) | CORE | topics/tasks.md, ref/tasks.md — **new in 6.0**; no `get_result` |
| JOB-8 | `DummyBackend` (`backends.dummy`) stores results (READY forever), `results`/`clear` | CORE | topics/tasks.md, ref/tasks.md — **new in 6.0** |
| JOB-9 | `TASKS` setting, `task_backends` handler, `default_task_backend`; backend feature flags `supports_defer`/`supports_async_task`/`supports_get_result`/`supports_priority`; exceptions `InvalidTask`/`InvalidTaskBackend`/`TaskResultDoesNotExist`/`TaskResultMismatch` | CORE | ref/tasks.md — **new in 6.0** |
| JOB-10 | Third-party/production task backends supplying the worker process + durable queue — Django ships **no worker** or `db_worker` command; execution is left to external infra | ECO | topics/tasks.md, ref/tasks.md — **new in 6.0**; Community Ecosystem / Tasks grid |
| JOB-11 | Custom task backend subclassing `BaseTaskBackend` (implement `enqueue`, optional `task_class`/`validate_task`) | DIY | topics/tasks.md, ref/tasks.md — **new in 6.0** |

## P10 — Real-time

**Problem.** Push server events to connected browsers (websockets, presence, server push). **Answer.** Django core has no websocket or bidirectional-push story, and the 6.1 corpus does not document one — its well-known external answer (Channels) lives outside core and outside this corpus. What core provides is the foundation: async views on ASGI, streaming responses usable for SSE/long-polling, and disconnect handling. Everything beyond that is ecosystem territory.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| LIVE-1 | SSE / long-polling via `StreamingHttpResponse` with an async iterator under ASGI | OPT | ref/request-response.md — WSGI ties a worker for the duration; ASGI explicitly called out for long-poll/SSE; response class detail at CTRL-38 |
| LIVE-2 | Client-disconnect handling in async/streaming views (`asyncio.CancelledError` raised on disconnect; catch in generator, re-raise) | OPT | ref/request-response.md, topics/async.md |
| LIVE-3 | Async views + ASGI as the concurrency foundation (hundreds of open connections, slow streaming, long-polling) | CORE | topics/async.md — see CTRL-79 and CONF- for ASGI deployment |
| LIVE-4 | WebSockets / bidirectional push | ECO | No core API and no coverage in the corpus; ASGI servers (Daphne/Uvicorn/Hypercorn, CONF-19) speak the protocol but Django core does not expose it |

## P11 — Mail & notifications

**Problem.** Send templated transactional email reliably across environments and providers. **Answer.** `django.core.mail`: `EmailMessage`/`EmailMultiAlternatives` (rebuilt on Python's modern `email.message` API in 6.0) over pluggable backends, with 6.1's `MAILERS` setting bringing named multi-backend configuration (like `CACHES`/`DATABASES`); console/file/locmem/dummy backends serve dev and test. There is no multi-channel notification framework — contrib.messages (CTRL-114…117) covers in-app flash messages only, and SMS/chat/push are ecosystem territory.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| MAIL-1 | `send_mail(subject, message, from_email, recipient_list, *, html_message=, using=)` | CORE | topics/email.md — `fail_silently`/`auth_user`/`auth_password`/`connection` args deprecated **6.1** |
| MAIL-2 | `send_mass_mail(datatuple, *, using=)` — one connection for many messages | CORE | topics/email.md |
| MAIL-3 | `mail_admins()` / `mail_managers()` shortcuts (ADMINS/MANAGERS, `EMAIL_SUBJECT_PREFIX`) | CORE | topics/email.md |
| MAIL-4 | `EmailMessage` class: cc/bcc/reply_to/headers, `attach()`/`attach_file()`, `message(policy=)`, `recipients()`, `send(*, using=)` | CORE | topics/email.md — MIMEPart attachments added **6.0**, legacy MIMEBase deprecated **6.0** |
| MAIL-5 | `EmailMultiAlternatives` HTML email via `attach_alternative()`, `alternatives`, `body_contains()` | CORE | topics/email.md — `EmailAlternative`/`EmailAttachment` named tuples |
| MAIL-6 | `content_subtype` to change the default text body to `text/html` | CORE | topics/email.md |
| MAIL-7 | SMTP backend `backends.smtp.EmailBackend` (host/port/use_tls/use_ssl/ssl_certfile/timeout OPTIONS) | CORE | topics/email.md |
| MAIL-8 | Console backend `backends.console.EmailBackend` (dev) | CORE | topics/email.md — now the default in the `startproject` MAILERS (**6.1**) |
| MAIL-9 | File backend `backends.filebased.EmailBackend` (`file_path`) | CORE | topics/email.md |
| MAIL-10 | In-memory backend `backends.locmem.EmailBackend` → `mail.outbox`, `sent_using` attr | CORE | topics/email.md — auto-used in tests; `sent_using` added **6.1** |
| MAIL-11 | Dummy backend `backends.dummy.EmailBackend` | CORE | topics/email.md |
| MAIL-12 | Third-party email backends (ESP APIs, async task queues, dev preview tools) | ECO | topics/email.md — Community Ecosystem / Django Packages email grid |
| MAIL-13 | Custom email backend subclassing `BaseEmailBackend` (`send_messages`, `open`/`close`) | DIY | topics/email.md — alias-aware init guidance in howto/mailers-migration.md |
| MAIL-14 | `MAILERS` setting w/ multiple named mailers + `using=` arg; `mailers`/`mailers.default` factory, `MailerDoesNotExist`/`InvalidMailer` | CORE | topics/email.md — **new in 6.1**, replaces `EMAIL_BACKEND`/`get_connection` (deprecated) |
| MAIL-15 | Connection reuse: `backend.open()`/`close()`/`send_messages()`, backend as context manager | CORE | topics/email.md |
| MAIL-16 | Header-injection protection: CRLF `ValueError`, safe address building via `email.headerregistry.Address` | CORE | topics/email.md — `BadHeaderError`→`ValueError` (**6.0**) |
| MAIL-17 | Mailers migration guide (deprecated `EMAIL_*` settings → `MAILERS` OPTIONS, `fail_silently`/`auth`/`get_connection` replacements) | CORE | howto/mailers-migration.md — removals scheduled 7.0 |

## P12 — Caching & performance

**Problem.** Avoid recomputing/refetching expensive data and cut response cost. **Answer.** A unified multi-backend cache API (Redis/Memcached/database/file/local-memory behind one interface, per the "consistency" philosophy) usable at four granularities — whole site, per view, template fragment, low level — plus HTTP cache-header helpers (`Vary`, `Cache-Control`), conditional-GET and compression middleware.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CACHE-1 | Memcached backend `PyMemcacheCache`/`PyLibMCCache` (`django.core.cache.backends.memcached`), multi-server sharding, unix socket, pooling/`ignore_exc` OPTIONS | OPT | topics/cache.md — requires `pymemcache` or `pylibmc` binding (ECO dependency) |
| CACHE-2 | Redis backend `RedisCache` (`backends.redis`), leader/replica LOCATION list, `db`/`pool_class` OPTIONS | OPT | topics/cache.md — requires `redis-py` (hiredis recommended) |
| CACHE-3 | Database backend `DatabaseCache` (`backends.db`) + `createcachetable`, router support | CORE | topics/cache.md |
| CACHE-4 | Filesystem backend `FileBasedCache` (`backends.filebased`), pickle-serialized per-file | CORE | topics/cache.md |
| CACHE-5 | Local-memory cache `LocMemCache` (`backends.locmem`), default backend, per-process LRU, thread-safe | CORE | topics/cache.md |
| CACHE-6 | Dummy cache `DummyCache` (`backends.dummy`) no-op interface for dev | CORE | topics/cache.md |
| CACHE-7 | Custom cache backend via dotted `BACKEND` path / subclass `BaseCache`, override `validate_key` | DIY | topics/cache.md |
| CACHE-8 | Cache tuning args `TIMEOUT`, `OPTIONS` `MAX_ENTRIES`/`CULL_FREQUENCY` (locmem/filesystem/db culling) | CORE | topics/cache.md |
| CACHE-9 | Per-site cache: `UpdateCacheMiddleware` + `FetchFromCacheMiddleware`, `CACHE_MIDDLEWARE_*` settings; caches GET/HEAD 200 | CORE | topics/cache.md, ref/middleware.md — caches for `CACHE_MIDDLEWARE_SECONDS`; place around Vary-modifying middleware (CTRL-76) |
| CACHE-10 | Per-view `cache_page(timeout, *, cache=, key_prefix=)` decorator (also usable in the URLconf) | CORE | topics/cache.md — `django.views.decorators.cache` |
| CACHE-11 | Template fragment caching `{% cache timeout name vary... using= %}` tag + `make_template_fragment_key()` | CORE | topics/cache.md — `{% load cache %}`; falls back to `template_fragments`→default cache alias |
| CACHE-12 | Low-level cache API `set/get/add/get_or_set/get_many/set_many/delete/delete_many/touch/incr/decr/clear/close`, `caches`/`cache` accessors | CORE | topics/cache.md |
| CACHE-13 | Cache versioning (`version` arg, `incr_version`/`decr_version`, `VERSION`) + key prefixing (`KEY_PREFIX`) & `KEY_FUNCTION` transformation | CORE | topics/cache.md |
| CACHE-14 | Async cache variants (a-prefixed `aadd`/`aset`/`aget`/`ahas_key` …) | CORE | topics/cache.md — async *backends* developing; full async caching not yet supported |
| CACHE-15 | `Vary` header helpers: `vary_on_headers(*h)` / `vary_on_cookie` decorators + `patch_vary_headers` | CORE | topics/cache.md, topics/http/decorators.md — `django.views.decorators.vary`; control the cache key |
| CACHE-16 | `cache_control(private/public/max_age/...)` decorator + `patch_cache_control`; downstream Expires/Cache-Control auto-set | CORE | topics/cache.md, topics/http/decorators.md — CacheMiddleware varies on `Authorization` (chg 6.0.6) |
| CACHE-17 | `never_cache` decorator to disable browser/proxy caching (no-store headers) | CORE | topics/cache.md, topics/http/decorators.md |
| CACHE-18 | Cached sessions backend for performance | CORE | topics/performance.md — `cached-sessions-backend` pointer; backends at CTRL-106 |
| CACHE-19 | `cached_property` decorator (memoize an instance method result) | CORE | topics/performance.md — `django.utils.functional`; see also EXT-16 |
| CACHE-20 | `ConditionalGetMiddleware` (ETag/Last-Modified conditional GET) | OPT | ref/middleware.md, topics/performance.md — adds ETag if missing; If-None-Match/If-Modified-Since → `HttpResponseNotModified`; global, GET-only, response still generated (per-view `condition` decorator, CTRL-54, is finer-grained) |
| CACHE-21 | `GZipMiddleware` response compression | OPT | ref/middleware.md, topics/performance.md — compresses when body ≥200 bytes, no prior Content-Encoding, client sends `Accept-Encoding: gzip`; weakens ETag; BREACH mitigation via up to `max_random_bytes` (default 100) random bytes (Heal The Breach); place before body-touching middleware; TLS/BREACH risk noted |
| CACHE-22 | `ManifestStaticFilesStorage` content-hash filenames for long-term browser caching | CORE | topics/performance.md — detail at VIEW-40 |
| CACHE-23 | Cached template loader (`django.template.loaders.cached.Loader`) | CORE | topics/performance.md — auto-enabled when loaders unspecified (VIEW-24) |

## P13 — Files & storage

**Problem.** Accept file uploads without exhausting memory, and abstract file persistence behind a swappable API. **Answer.** A `File` wrapper hierarchy, chunked streaming upload handlers (memory → tempfile crossover at 2.5 MB), and a pluggable Storage API configured by the `STORAGES` setting — local filesystem and in-memory backends in core, cloud backends via the ecosystem (django-storages).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| FILE-1 | `File` object wrapper (`name`/`size`/`mode`/`file`, `open`/`chunks`/`multiple_chunks`/`read`/`write`/`close`, iteration) | CORE | ref/files/file.md |
| FILE-2 | `ContentFile(content, name=)` for in-memory string/bytes content | CORE | ref/files/file.md — `django.core.files.base` |
| FILE-3 | `ImageFile` (adds `width`/`height`) | CORE | ref/files/file.md — `django.core.files.images` |
| FILE-4 | `FileField`/`ImageField` file access on models: `.name`/`.path`/`.url`, `.open()`, `File.save()`/`File.delete()` | CORE | topics/files.md, ref/files/file.md — field definition at ORM-26/27 |
| FILE-5 | Uploaded files in `request.FILES`: `UploadedFile` (`read`/`chunks`/`content_type`/`charset`, wraps content+name; stream with `.chunks()` to avoid memory blowup) + `TemporaryUploadedFile`/`InMemoryUploadedFile` | CORE | ref/files/uploads.md, topics/http/file-uploads.md, ref/forms/fields.md |
| FILE-6 | Upload handlers via `FILE_UPLOAD_HANDLERS`: `MemoryFileUploadHandler` (<2.5 MB in memory) + `TemporaryFileUploadHandler` (large → temp file); custom `FileUploadHandler` (`receive_data_chunk`/`file_complete`/`new_file`/`handle_raw_input`) | CORE/DIY | ref/files/uploads.md, topics/http/file-uploads.md |
| FILE-7 | Per-request handler override via `request.upload_handlers` (insert/replace) before reading POST/FILES; CSRF exempt/protect interplay | DIY | topics/http/file-uploads.md |
| FILE-8 | Storage API: `save`/`open`/`delete`/`exists`/`url`/`listdir`/`size`/`path`/`get_valid_name`/`get_available_name`/`generate_filename`/`get_{created,modified,accessed}_time` | CORE | ref/files/storage.md |
| FILE-9 | `FileSystemStorage(location, base_url, permissions, allow_overwrite)` local storage | CORE | ref/files/storage.md, topics/files.md |
| FILE-10 | `InMemoryStorage` non-persistent memory storage (test speedups) | CORE | ref/files/storage.md |
| FILE-11 | `STORAGES` setting w/ `default`+`staticfiles` aliases, `storages` dict / `storages.create_storage()`, `default_storage`/`DefaultStorage` | CORE | ref/files/storage.md, topics/files.md — storage may be a callable/`LazyObject` |
| FILE-12 | Custom storage class subclassing `Storage` (`_open`/`_save`, `deconstructible`) | DIY | howto/custom-file-storage.md |
| FILE-13 | Community/remote storage backends (e.g. django-storages) | ECO | ref/files/storage.md — Storage Backends grid / Community Ecosystem |

## P14 — Building APIs

**Problem.** Serve machine-readable representations of data and standard interchange documents. **Answer.** Core stops well short of a REST framework: it provides model serialization (JSON/JSONL/XML/YAML with natural keys) aimed at fixtures/interchange, `JsonResponse` + content negotiation on the request object (CTRL-18/19/37), and contrib frameworks for sitemaps and RSS/Atom feeds. Versioned REST APIs, OpenAPI schemas and GraphQL are explicitly ecosystem territory (DRF, django-ninja — absent from this corpus).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| API-1 | `serializers.serialize(format, queryset, ...)` / `deserialize()` iterator over `DeserializedObject`; `get_serializer()`/`SerializerDoesNotExist` | CORE | topics/serialization.md |
| API-2 | JSON serializer + `DjangoJSONEncoder` (datetime/Decimal/UUID/etc.), custom `cls=` encoder | CORE | topics/serialization.md |
| API-3 | JSONL (JSON Lines) serializer for line-by-line large loads | CORE | topics/serialization.md |
| API-4 | XML serializer (`<django-objects>`/`<object>`/`<field>` dialect) | CORE | topics/serialization.md — `SuspiciousOperation` on nested tags (chg **6.1**) |
| API-5 | YAML serializer | OPT | topics/serialization.md — requires PyYAML |
| API-6 | Subset-of-fields serialization (`fields=[...]`, pk always emitted) | CORE | topics/serialization.md |
| API-7 | Deserialization controls: `DeserializedObject.save()`, `ignorenonexistent=`, `handle_forward_references=`/`save_deferred_fields()` | CORE | topics/serialization.md |
| API-8 | Natural keys: `natural_key()`/`get_by_natural_key()`, `use_natural_foreign_keys`/`use_natural_primary_keys`, `natural_key.dependencies`, opt-out via empty tuple | CORE | topics/serialization.md — empty-tuple opt-out added **6.1** |
| API-9 | Custom serialization format via `Serializer`/`Deserializer` classes + `SERIALIZATION_MODULES` | DIY | topics/serialization.md |
| API-10 | contrib.sitemaps: `Sitemap` class (`items`, `location`, `lastmod`, `changefreq`, `priority`, `protocol`, `limit`, `paginator`, `get_latest_lastmod()`); `views.sitemap` URLconf | OPT | ref/contrib/sitemaps.md — requires contrib.sites; no `ping_google` in 6.1 docs (Last-Modified + ConditionalGetMiddleware replaces it) |
| API-11 | contrib.sitemaps extras: i18n (`i18n`, `languages`, `alternates`, `x_default`, `get_languages_for_item()`), `GenericSitemap`, static-view sitemap, `views.index` sitemap index, template customization/context vars | OPT | ref/contrib/sitemaps.md |
| API-12 | contrib.syndication: high-level `Feed` class (title/link/description, `items()`, `item_*` hooks, `get_object()` for URL args, `get_context_data()`, title/description templates) | OPT | ref/contrib/syndication.md |
| API-13 | contrib.syndication options: `feed_type` (Rss201rev2Feed/RssUserland091Feed/Atom1Feed), Atom+RSS tandem (`subtitle`), enclosures (`item_enclosures`/`item_enclosure_*`), language, guid, stylesheets (`Stylesheet`) | OPT | ref/contrib/syndication.md |
| API-14 | Low-level `django.utils.feedgenerator`: `SyndicationFeed` (`add_item`/`write`/`writeString`), `Atom1Feed`, `Rss201rev2Feed`, `Enclosure`, `Stylesheet`; custom generator subclasses (`root_attributes`/`add_root_elements`/`item_attributes`/`add_item_elements`) | OPT/DIY | ref/contrib/syndication.md, ref/utils.md |

## P15 — i18n & l10n

**Problem.** Translate the UI, format data per locale, and handle time zones correctly. **Answer.** GNU gettext end-to-end: `gettext`/lazy marking in Python and `{% translate %}`/`{% blocktranslate %}` in templates, extraction/compilation via `makemessages`/`compilemessages`, per-request language selection through `LocaleMiddleware` (URL prefix → cookie → Accept-Language), JS catalogs for the client, locale-aware formatting, and UTC-in-the-database time-zone handling (`USE_TZ`, on by default).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| I18N-1 | Mark strings with `gettext`/`gettext_lazy`/`gettext_noop` (aliased `_`); only single-arg funcs may alias `_` | CORE | topics/i18n/translation.md — `django.utils.translation`, on by default via `USE_I18N` |
| I18N-2 | Contextual markers `pgettext`/`pgettext_lazy` (emit `msgctxt`) | CORE | topics/i18n/translation.md — disambiguates the same literal per context |
| I18N-3 | Pluralization `ngettext`/`ngettext_lazy`/`npgettext` (singular/plural/count) | CORE | topics/i18n/translation.md — lazy plural may pass a dict key as `number` |
| I18N-4 | Lazy translation objects + `django.utils.text.format_lazy` + `functional.lazy()` wrapper | CORE | topics/i18n/translation.md — required for module-load-time strings (model/form fields, `verbose_name`, `help_text`, `@admin.display`) |
| I18N-5 | Named-string interpolation `%(name)s`; f-strings unsupported by xgettext | CORE | topics/i18n/translation.md — extraction limitation; allows placeholder reorder |
| I18N-6 | Translator comments (`# Translators:` prefix / `{% comment %}` / `{# #}`) | CORE | topics/i18n/translation.md — surfaced in `.po` |
| I18N-7 | `{% translate %}`/`{% trans %}` tag with `noop`, `context`, `as var` | CORE | topics/i18n/translation.md — needs `{% load i18n %}`; output not auto-escaped |
| I18N-8 | `{% blocktranslate %}`/`{% blocktrans %}` with `with`, `count`/`{% plural %}`, `trimmed`, `asvar`, `context` | CORE | topics/i18n/translation.md — internally `ngettext`; no nested block tags |
| I18N-9 | Language switching `{% language %}` + `{% get_current_language %}`/`{% get_available_languages %}`/`{% get_current_language_bidi %}` | CORE | topics/i18n/translation.md |
| I18N-10 | Language-info tags/filters `{% get_language_info %}`/`_list`, `language_name`/`_local`/`_bidi`/`_translated`; Python `get_language_info()` | CORE | topics/i18n/translation.md — source `django.conf.locale` |
| I18N-11 | `i18n` context processor (exposes `LANGUAGES`/`LANGUAGE_CODE`/`LANGUAGE_BIDI`) | OPT | topics/i18n/translation.md — opt-in template context processor |
| I18N-12 | `JavaScriptCatalog` view + JS `gettext`/`ngettext`/`interpolate`/`get_format`/`gettext_noop`/`pgettext`/`npgettext`/`pluralidx`; `domain`/`packages` attrs | OPT | topics/i18n/translation.md — `django.views.i18n`; wrap in `i18n_patterns` if used |
| I18N-13 | `JSONCatalog` view (JSON catalog/formats/plural for other client libs) | OPT | topics/i18n/translation.md |
| I18N-14 | JS catalog caching (`cache_page`/`last_modified`, version key) | DIY | topics/i18n/translation.md — catalog regenerated per request from `.mo` |
| I18N-15 | Pre-generate the JS catalog as a static file via `django-statici18n` | ECO | topics/i18n/translation.md — third-party |
| I18N-16 | `i18n_patterns(*urls, prefix_default_language=True)` URL language prefixing | OPT | topics/i18n/translation.md — root URLconf only; requires `LocaleMiddleware` |
| I18N-17 | Translatable URL patterns via `gettext_lazy` routes (reverse in the active language) | OPT | topics/i18n/translation.md, topics/http/urls.md, ref/urls.md — best inside `i18n_patterns` |
| I18N-18 | `set_language` redirect view (`django.conf.urls.i18n`, `next` param, `django_language` cookie) | OPT | topics/i18n/translation.md — POST; keep out of `i18n_patterns` |
| I18N-19 | `LocaleMiddleware` language detection: URL prefix → cookie → `Accept-Language` → `LANGUAGE_CODE`; exposes `request.LANGUAGE_CODE` | CORE | topics/i18n/translation.md, ref/middleware.md — order after Session, before Common; `response_redirect_class` overridable |
| I18N-20 | `makemessages` (`-l`, `-a`, `-e`/`--extension`, `-d djangojs`) → `.po` in `locale/LANG/LC_MESSAGES` | CORE | topics/i18n/translation.md — wraps GNU `xgettext`/`msgmerge`/`msguniq`, gettext ≥0.19 |
| I18N-21 | `compilemessages` → binary `.mo`; fuzzy entries skipped, UTF-8/no-BOM required | CORE | topics/i18n/translation.md — uses `msgfmt` |
| I18N-22 | Customize `makemessages` via `xgettext_options`/custom Command args | DIY | topics/i18n/translation.md — subclass the management command |
| I18N-23 | Translation discovery precedence `LOCALE_PATHS` > app `locale/` > `django/conf/locale`; territorial→generic fallback | CORE | topics/i18n/translation.md |
| I18N-24 | Settings `USE_I18N`, `LANGUAGE_CODE`, `LANGUAGES` (restrict/lazy-mark names) | CORE | topics/i18n/translation.md, topics/i18n/index.md |
| I18N-25 | Runtime API `activate`/`deactivate`/`deactivate_all`/`get_language`/`check_for_language`/`override` (+ set cookie for persistence) | CORE | topics/i18n/translation.md — thread-scoped; `override` context manager |
| I18N-26 | Language cookie settings `LANGUAGE_COOKIE_NAME`/`_AGE`/`_DOMAIN`/`_HTTPONLY`/`_PATH`/`_SAMESITE`/`_SECURE` | OPT | topics/i18n/translation.md |
| I18N-27 | Jinja2 string extraction via Babel `babel.cfg` (makemessages can't parse Jinja2) | ECO | topics/i18n/translation.md — third-party |
| I18N-28 | `no-python-format`/`%%` escaping for percent-sign false positives; non-English base language caveats | DIY | topics/i18n/translation.md — troubleshooting guidance |
| I18N-29 | Locale-aware display of dates/times/numbers in templates + localized form input parsing | CORE | topics/i18n/formatting.md — per current locale; distinct display vs parse formats |
| I18N-30 | Form field `localize=True` argument for localized input/output | CORE | topics/i18n/formatting.md |
| I18N-31 | `{% localize on/off %}` tag (`l10n` lib) for block-level control | OPT | topics/i18n/formatting.md — needs `{% load l10n %}` |
| I18N-32 | `localize`/`unlocalize` template filters (per-variable) | OPT | topics/i18n/formatting.md |
| I18N-33 | Custom format files via `FORMAT_MODULE_PATH` + `<locale>/formats.py` | OPT | topics/i18n/formatting.md — exposed by `formats.get_format()`; avoid sensitive data |
| I18N-34 | `USE_THOUSAND_SEPARATOR`/`THOUSAND_SEPARATOR`/`DECIMAL_SEPARATOR`/`NUMBER_GROUPING` (or `intcomma`) | CORE | topics/i18n/formatting.md — `sanitize_separators` not mentioned in this corpus |
| I18N-35 | Country-specific fields via `django-localflavor`; provided-locale limits (Swiss context-sensitive) | ECO | topics/i18n/formatting.md — third-party + documented limitation |
| I18N-36 | `USE_TZ` (default True): store UTC, aware datetimes internally, convert per user | CORE | topics/i18n/timezones.md |
| I18N-37 | `zoneinfo` stdlib backend; `zoneinfo.available_timezones()` for choices | CORE | topics/i18n/timezones.md |
| I18N-38 | `django.utils.timezone` helpers `now`/`is_aware`/`is_naive`/`make_aware`/`make_naive`/`localtime`/`activate`/`deactivate`/`get_current_timezone` | CORE | topics/i18n/timezones.md — `now()` returns aware when `USE_TZ` |
| I18N-39 | `timezone.override` context manager | OPT | topics/i18n/timezones.md |
| I18N-40 | Default (`TIME_ZONE`) vs current time zone concept; naive vs aware guidance | CORE | topics/i18n/timezones.md — DST/`fold`; warns on naive save |
| I18N-41 | Current-tz selection is DIY (no `Accept-Language` equivalent); example session middleware + set-timezone view | DIY | topics/i18n/timezones.md — store in profile/session |
| I18N-42 | `{% localtime on/off %}` template tag (`tz` lib) | OPT | topics/i18n/timezones.md — needs `{% load tz %}` |
| I18N-43 | `{% timezone "…"/None %}` block tag | OPT | topics/i18n/timezones.md |
| I18N-44 | `{% get_current_timezone %}` tag + `tz` context processor (`TIME_ZONE` var) | OPT | topics/i18n/timezones.md |
| I18N-45 | `localtime`/`utc`/`timezone` template filters | OPT | topics/i18n/timezones.md — force conversion of a single value |
| I18N-46 | Time-zone-aware form input/output (aware in `cleaned_data`, invalid on DST gaps) | CORE | topics/i18n/timezones.md |
| I18N-47 | Per-connection `DATABASES['TIME_ZONE']` distinct from global `TIME_ZONE` | OPT | topics/i18n/timezones.md — for non-Django/local-time DBs |
| I18N-48 | Migration/fixtures guidance (aware serialization offset), MySQL tz definitions | DIY | topics/i18n/timezones.md — migration guide + FAQ |

## P16 — Testing

**Problem.** Test application behavior across the HTTP, view, ORM and template layers with fast, isolated databases. **Answer.** A unittest-based hierarchy (`SimpleTestCase` → `TransactionTestCase` → transaction-wrapped `TestCase`) with an in-process test `Client` (sync and async), per-class `setUpTestData`, rich framework-aware assertions, settings overrides, automatic email/task/storage test doubles, and managed test-database lifecycle with parallelization. The runner is swappable; pytest integration is left to the ecosystem (not mentioned in the corpus).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| TEST-1 | `SimpleTestCase` (no DB by default, `databases`, settings override, assertions) | CORE | topics/testing/tools.md — `django.test` |
| TEST-2 | `TransactionTestCase` (truncate-reset DB, `fixtures`, DB-feature skips, commit/rollback testing) | CORE | topics/testing/tools.md |
| TEST-3 | `TestCase` (nested `atomic` rollback, `setUpTestData`, `captureOnCommitCallbacks`) | CORE | topics/testing/tools.md, overview.md — the most common base |
| TEST-4 | `LiveServerTestCase` / `StaticLiveServerTestCase` (`live_server_url`) | OPT | topics/testing/tools.md, ref/contrib/staticfiles.md, howto/static-files/index.md — for in-browser clients; the Static variant serves assets without collectstatic |
| TEST-5 | Test `Client` methods `get/post/head/options/put/patch/delete/trace` with `follow`/`redirect_chain`, `secure`, `content_type`/JSON, `headers`, `query_params`, file upload, `enforce_csrf_checks` | CORE | topics/testing/tools.md |
| TEST-6 | Client auth `login`/`force_login`/`logout` (+ async `alogin`/`aforce_login`/`alogout`) | CORE | topics/testing/tools.md |
| TEST-7 | `AsyncClient` / `self.async_client`; auto-wrapped `async def` tests | OPT | topics/testing/tools.md — ASGIRequest; await requests |
| TEST-8 | `RequestFactory` (view-as-black-box, no middleware) | CORE | topics/testing/advanced.md |
| TEST-9 | `AsyncRequestFactory` (ASGI scope, sync callables) | OPT | topics/testing/advanced.md |
| TEST-10 | Response inspection: `context`, `templates`, `resolver_match`, `json()`, `status_code`, `exc_info`, cookies/session (`asession`) | CORE | topics/testing/tools.md — template attrs only during test run + DjangoTemplates backend |
| TEST-11 | Fixtures via the `fixtures` class attr (loaded in `setUpClass`, `dumpdata`) | CORE | topics/testing/tools.md — default-DB only unless `databases` |
| TEST-12 | `setUpTestData` (per-class data, deepcopy isolation) + `InMemoryStorage`/faster `PASSWORD_HASHERS` for speed | OPT | topics/testing/overview.md, tools.md — perf |
| TEST-13 | Assertions: `assertContains`/`NotContains`, `assertRedirects`, `assertTemplateUsed`/`NotUsed`, `assertURLEqual`, `assertHTMLEqual`/`NotEqual`, `assertInHTML`/`NotInHTML`, `assertXMLEqual`/`NotEqual`, `assertJSONEqual`/`NotEqual`, `assertFormError`, `assertFormSetError`, `assertFieldOutput`, `assertRaisesMessage`, `assertWarnsMessage` | CORE | topics/testing/tools.md — `msg_prefix` support |
| TEST-14 | DB assertions `assertNumQueries`, `assertQuerySetEqual` (on `TransactionTestCase`) | CORE | topics/testing/tools.md — context-manager forms |
| TEST-15 | `override_settings`/`modify_settings` (decorator + `self.settings()`/`self.modify_settings()` context mgrs); `setting_changed` signal | CORE | topics/testing/tools.md — caveats for init-time settings |
| TEST-16 | `isolate_apps` (temporary app registry for model tests) | OPT | topics/testing/tools.md — `django.test.utils` |
| TEST-17 | Email outbox capture `django.core.mail.outbox` (locmem backend, `sent_using` **6.1**), emptied per test | CORE | topics/testing/tools.md — auto-swaps the `MAILERS` backend |
| TEST-18 | Management command testing via `call_command` (+ `StringIO` capture) | CORE | topics/testing/tools.md |
| TEST-19 | Tagging tests `@tag` + `--tag`/`--exclude-tag` (inheritance, exclude precedence) | OPT | topics/testing/tools.md |
| TEST-20 | DB-feature skips `skipIfDBFeature`/`skipUnlessDBFeature` (plus unittest `skipIf`/`skipUnless`) | OPT | topics/testing/tools.md |
| TEST-21 | Selenium in-browser tests (`selenium>=4.23`, `WebDriverWait`) | ECO | topics/testing/tools.md — third-party |
| TEST-22 | Test DB lifecycle: auto-create/destroy, `--keepdb`, `--noinput`, `TEST` dict (`NAME`/`CHARSET`/`COLLATION`/`MIRROR`/`DEPENDENCIES`), SQLite in-memory, `DEBUG=False` | CORE | topics/testing/overview.md, advanced.md |
| TEST-23 | Parallel `--parallel` (per-process DB, needs `tblib`), `--shuffle`/`--reverse` ordering, `--durations` slowest-N | OPT | topics/testing/overview.md, advanced.md — `tblib` is ECO |
| TEST-24 | `serialized_rollback` + `TEST_NON_SERIALIZED_APPS` (reload migration data, ~3x slower) | OPT | topics/testing/overview.md, tools.md |
| TEST-25 | Multi-DB testing: `databases` attr, primary/replica `MIRROR`, creation-order `DEPENDENCIES` | OPT | topics/testing/tools.md, advanced.md |
| TEST-26 | `TransactionTestCase.available_apps` (private), `reset_sequences`, `SerializeMixin` sequential lockfile | OPT | topics/testing/advanced.md |
| TEST-27 | Custom test runner via `TEST_RUNNER`/`DiscoverRunner` (`run_tests`/`build_suite`/`setup_databases`/`add_arguments`, `test_suite`/`test_runner`/`test_loader`) + `django.test.utils` helpers | OPT | topics/testing/advanced.md — `get_runner` for reusable apps |
| TEST-28 | CBV testing via `view.setup(request)`; multi-host `ALLOWED_HOSTS` handling | DIY | topics/testing/advanced.md — guidance |
| TEST-29 | `coverage.py` integration (`coverage run --source='.' manage.py test`) | ECO | topics/testing/advanced.md — explicitly named third-party (pytest-django NOT mentioned) |
| TEST-30 | Mocking guidance: `mock.patch` + `async_to_sync` inside async tests (async-incompatible decorators) | DIY | topics/testing/tools.md — only mocking reference in the corpus |

## P17 — CLI, codegen & developer experience

**Problem.** Scaffold, run, inspect and administer the project from the command line. **Answer.** One command framework with three entry points (`django-admin`, `manage.py`, `python -m django`) hosting ~40 built-in commands — scaffolding, dev server with autoreload, migration suite, data dump/load, i18n, testing — every contrib app can add its own, and custom commands are a first-class extension point (`management/commands/`).

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CLI-1 | Three CLI entry points `django-admin`, `manage.py`, `python -m django`; `manage.py` auto-sets `DJANGO_SETTINGS_MODULE` | CORE | ref/django-admin.md — `help`, `help --commands`, `version` subcommands |
| CLI-2 | `startproject name [dir]` — scaffolds project pkg (manage.py, settings/urls/asgi/wsgi), `--template` dir/archive/URL, template context incl. `secret_key`, auto-creates dest dir (**6.0**) | CORE | ref/django-admin.md — render/trusted-code warnings |
| CLI-3 | `startapp name [dir]` — app skeleton, same `--template`/`--extension`/`--name`/`--exclude` mechanism, `.py-tpl` renaming | CORE | ref/django-admin.md |
| CLI-4 | `runserver [addrport]` — dev WSGI server, per-request Python autoreload, Watchman/`pywatchman` kernel-signal reload, `--noreload`/`--nothreading`/`--ipv6`, IPv6 brackets, `DJANGO_RUNSERVER_HIDE_WARNING`, `DJANGO_WATCHMAN_TIMEOUT` | CORE | ref/django-admin.md — WSGI-only, no built-in HTTPS; staticfiles overrides it; not for production |
| CLI-5 | ASGI in dev only via third-party `daphne-runserver` | ECO | ref/django-admin.md, howto/deployment/asgi/daphne.md |
| CLI-6 | `shell` — auto-imports all models + `connection/reset_queries/models/settings/timezone` (**6.0**), `-i {ipython,bpython,python}`, `--no-imports`, `-c COMMAND`, stdin exec, `--no-startup` | CORE | ref/django-admin.md |
| CLI-7 | Customize shell auto-imports by overriding `get_auto_imports()` on a `shell.Command` subclass (return `None` to disable) | DIY | howto/custom-shell.md |
| CLI-8 | `dbshell` — launches the native client (psql/mysql/sqlite3/sqlplus), `--database`, `-- ARGS` passthrough | CORE | ref/django-admin.md |
| CLI-9 | `check [app_label…]` — system check framework CLI; `--deploy`, `--tag/-t`, `--database`, `--list-tags`, `--fail-level` | CORE | ref/django-admin.md, topics/checks.md — framework itself in EXT- |
| CLI-10 | `diffsettings` — vs defaults; `--all`, `--default MODULE`, `--output {hash,unified}` | CORE | ref/django-admin.md, topics/settings.md |
| CLI-11 | `dumpdata [app.Model…]` — serialize to stdout/file; `--format`, `--natural-foreign/-primary`, `--pks`, `--exclude`, `-o` with bz2/gz/lzma/xz compression, progress bar | CORE | ref/django-admin.md |
| CLI-12 | `loaddata fixture…` — load fixtures; stdin via `-`, `--app`, `--exclude`, `--ignorenonexistent`, `--format` for stdin | CORE | ref/django-admin.md |
| CLI-13 | `makemigrations` — `--empty`, `--dry-run`, `--merge`, `--name`, `--check`, `--scriptable`, `--update`, `--no-header` | CORE | ref/django-admin.md, topics/migrations |
| CLI-14 | `migrate [app] [name]` — `--fake`, `--fake-initial`, `--plan`, `--run-syncdb`, `--check`, `--prune`, `zero` target | CORE | ref/django-admin.md — emits pre/post_migrate |
| CLI-15 | `showmigrations` — `--list/-l` (applied `[X]`, datetimes at -v2), `--plan/-p` | CORE | ref/django-admin.md |
| CLI-16 | `sqlmigrate`, `sqlflush`, `sqlsequencereset` — print SQL for a migration / flush / sequence reset | CORE | ref/django-admin.md — sqlmigrate `--backwards`, no colorization |
| CLI-17 | `flush` — wipe data, re-run post-sync handlers (keeps the migration table); `--noinput` | CORE | ref/django-admin.md |
| CLI-18 | `squashmigrations app [start] name` — `--no-optimize`, `--squashed-name`, `--no-header` | CORE | ref/django-admin.md |
| CLI-19 | `optimizemigration app name` — optimize a migration's operations, `--check` | CORE | ref/django-admin.md |
| CLI-20 | `inspectdb [table…]` — generate models from a legacy DB, `--include-views/--include-partitions`, unmanaged by default | CORE | ref/django-admin.md, howto/legacy-databases |
| CLI-21 | `test [labels]` — DiscoverRunner: `--parallel[/DJANGO_TEST_PROCESSES]`, `--shuffle[SEED]`, `--reverse`, `--keepdb`, `--tag/--exclude-tag`, `-k`, `--pdb`, `--buffer`, `--debug-sql`, `--debug-mode`, `--timing`, `--durations` | CORE | ref/django-admin.md |
| CLI-22 | `testserver [fixtures]` — runserver against a test DB loaded from fixtures | OPT | ref/django-admin.md |
| CLI-23 | `createcachetable`, `makemessages`, `compilemessages` — cache-table + gettext i18n commands | OPT | ref/django-admin.md — makemessages `--domain djangojs`, `--add-location`, etc. |
| CLI-24 | `changepassword [user]`, `createsuperuser` — auth commands; superuser via `DJANGO_SUPERUSER_PASSWORD`/`DJANGO_SUPERUSER_<FIELD>` env, `--noinput`, override `get_input_data()` | OPT | ref/django-admin.md — django.contrib.auth |
| CLI-25 | `remove_stale_contenttypes`, `clearsessions`, `collectstatic`, `findstatic`, `ogrinspect` — contrib-app commands (contenttypes/sessions/staticfiles/gis) | OPT | ref/django-admin.md |
| CLI-26 | `sendtestemail [emails]` — `--using ALIAS` (MAILERS, **6.1**), `--managers`, `--admins` | OPT | ref/django-admin.md |
| CLI-27 | Default options on every command: `--pythonpath`, `--settings`, `--traceback`, `--verbosity {0-3}`, `--no-color`, `--force-color`, `--skip-checks` | CORE | ref/django-admin.md |
| CLI-28 | Colored output + `DJANGO_COLORS` (dark/light/nocolor palettes, per-role fg/bg/options, palette extension); Windows needs `colorama`/ANSICON | CORE | ref/django-admin.md, howto/windows.md |
| CLI-29 | Bash tab-completion script (`extras/django_bash_completion`) | OPT | ref/django-admin.md |
| CLI-30 | Auto `black` formatting of files generated by startproject/startapp/makemigrations/squash/optimize when on PATH | OPT | ref/django-admin.md |
| CLI-31 | `call_command(name, *args, **options)` — run commands from code, `stdout`/`stderr` redirect | CORE | ref/django-admin.md |
| CLI-32 | Custom commands: `management/commands/<name>.py` defining `Command(BaseCommand)`, `add_arguments()`, `handle()`, `self.stdout`/`self.stderr`/`self.style`; attrs `help`/`requires_system_checks`/`requires_migrations_checks`/`output_transaction`/`suppressed_base_arguments` | DIY | howto/custom-management-commands.md |
| CLI-33 | `BaseCommand` subclasses `AppCommand` (`handle_app_config`) and `LabelCommand` (`handle_label`); `CommandError(returncode)`, `SystemCheckError` | DIY | howto/custom-management-commands.md |
| CLI-34 | `@no_translations` decorator on `handle()`; command overriding via INSTALLED_APPS ordering | DIY | howto/custom-management-commands.md |

## P18 — Configuration & deployment

**Problem.** Configure the app per environment and put it into production. **Answer.** Settings are a plain Python module named by `DJANGO_SETTINGS_MODULE`, read through the `django.conf.settings` object, with dict-of-alias families (`DATABASES`, `CACHES`, `STORAGES`, `TASKS`, `MAILERS`, `TEMPLATES`); deployment is WSGI or ASGI callables handed to ecosystem servers (Gunicorn/uWSGI/mod_wsgi, Daphne/Uvicorn/Hypercorn/Granian), gated by a `check --deploy` checklist. Multi-domain awareness comes from contrib.sites.

| ID | Feature | Tier | Notes |
|---|---|---|---|
| CONF-1 | Settings = Python module; `DJANGO_SETTINGS_MODULE` env var or `--settings`; defaults from `django/conf/global_settings.py` | CORE | topics/settings.md |
| CONF-2 | Access via the `django.conf.settings` object (not the module); no runtime mutation | CORE | topics/settings.md |
| CONF-3 | `settings.configure(default_settings, **kw)` for use without `DJANGO_SETTINGS_MODULE`; `settings.configured` property; custom `default_settings` module | OPT | topics/settings.md |
| CONF-4 | `django.setup(set_prefix=True)` — loads settings, logging, app registry for standalone scripts (once only) | OPT | topics/settings.md, ref/applications.md |
| CONF-5 | Create your own uppercase settings | DIY | topics/settings.md |
| CONF-6 | Deployment checklist + critical settings `SECRET_KEY`/`SECRET_KEY_FALLBACKS`, `DEBUG`, `ALLOWED_HOSTS`; run `manage.py check --deploy` against prod settings | CORE | howto/deployment/checklist.md |
| CONF-7 | Env-specific settings guidance — CACHES/DATABASES/MAILERS/STATIC_ROOT/MEDIA differ per env; HTTPS toggles `CSRF_COOKIE_SECURE`/`SESSION_COOKIE_SECURE`; perf `CONN_MAX_AGE`/cached template loader | CORE | howto/deployment/checklist.md |
| CONF-8 | WSGI deployment — `wsgi.py` with `application` callable, `WSGI_APPLICATION` setting, WSGI middleware wrapping | CORE | howto/deployment/wsgi/index.md |
| CONF-9 | ASGI deployment — `asgi.py` with `application`, `ASGI_APPLICATION`, async-safety caveats, ASGI middleware wrapping | CORE | howto/deployment/asgi/index.md |
| CONF-10 | Documented WSGI servers: Gunicorn, uWSGI, Apache+mod_wsgi (embedded/daemon mode), Granian (`--interface wsgi`) | ECO | howto/deployment/wsgi/{gunicorn,uwsgi,modwsgi,granian}.md |
| CONF-11 | Documented ASGI servers: Daphne (reference server + runserver integration), Uvicorn (+ gunicorn `uvicorn_worker`), Hypercorn, Granian (`--interface asgi`) | ECO | howto/deployment/asgi/{daphne,uvicorn,hypercorn,granian}.md |
| CONF-12 | Apache/mod_wsgi auth against the Django user DB — `check_password`/`groups_for_user` handlers, `WSGIAuthUserScript`/`WSGIAuthGroupScript` | OPT | howto/deployment/wsgi/apache-auth.md |
| CONF-13 | Config families — `STORAGES`, `CACHES`, `DATABASES`, `TASKS`, `MAILERS`, `TEMPLATES` (dict-of-alias config) | CORE | ref/settings.md |
| CONF-14 | Settings reference categories — Core, Auth, Messages, Sessions, Sites, Static Files + Core topical index (Cache/Database/Debugging/Email/Error reporting/File uploads/Forms/i18n/HTTP/Logging/Models/Security/Serialization/Templates/Testing/URLs) | CORE | ref/settings.md |
| CONF-15 | Static-files deployment pointer — `STATIC_ROOT`+`collectstatic`, `STATIC_URL` | OPT | howto/deployment/checklist.md, howto/static-files — detail at VIEW-35/42 |
| CONF-16 | Install — Python, pip/venv, DB drivers (psycopg/mysqlclient/oracledb), `DATABASES['default']` ENGINE/NAME | CORE | topics/install.md — drivers are ECO dependencies |
| CONF-17 | Upgrade guidance — read release notes + deprecation timeline, run `python -Wa manage.py test` to surface `DeprecationWarning`, incremental feature-version upgrades, `pip install -U Django` | CORE | howto/upgrade-version.md |
| CONF-18 | Windows install — `py` launcher, venv, `DJANGO_COLORS=nocolor`, colorama for legacy terminals, `PYTHONUTF8` | OPT | howto/windows.md |
| CONF-19 | Email settings migration from `EMAIL_*` to `MAILERS`; `EMAIL_BACKEND`/`EMAIL_HOST`/… + `get_connection()` deprecated, removal scheduled | OPT | ref/settings.md, internals/deprecation.md — 6.1 → 7.0 |
| CONF-20 | contrib.sites: `Site` model (`domain`/`name`), `SITE_ID`, `get_current_site(request)` shortcut, `Site.objects.get_current()`/`clear_cache()`, post_migrate default site | OPT | ref/contrib/sites.md |
| CONF-21 | contrib.sites helpers: `CurrentSiteManager` (auto-filter by `site`/`sites` field), `CurrentSiteMiddleware` (sets `request.site`), `RequestSite` fallback | OPT | ref/contrib/sites.md, ref/middleware.md |
