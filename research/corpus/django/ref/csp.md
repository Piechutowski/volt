# Content Security Policy

<div class="versionadded">

6.0

</div>

<div class="module" synopsis="Middleware for Content Security Policy headers">

django.middleware.csp

</div>

Content Security Policy (CSP) is a web security standard that helps prevent content injection attacks by restricting the sources from which content can be loaded. It plays an important role in a comprehensive `security strategy
<security-csp>`.

For configuration instructions in a Django project, see the `Using CSP
<csp-config>` documentation. For an HTTP guide about CSP, see the [MDN Guide on CSP](https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/CSP).

## Overview

The [Content-Security-Policy specification](https://www.w3.org/TR/CSP3/) defines two complementary headers:

- `Content-Security-Policy`: Enforces the CSP policy, blocking content that violates the defined directives.
- `Content-Security-Policy-Report-Only`: Reports CSP violations without blocking content, allowing for non-intrusive testing.

Each policy is composed of one or more directives and their values, which together instruct the browser on how to handle specific types of content.

When the `~django.middleware.csp.ContentSecurityPolicyMiddleware` is enabled, Django automatically builds and attaches the appropriate headers to each response based on the configured `settings <csp-settings>`, unless they have already been set by another layer.

## Settings

The `~django.middleware.csp.ContentSecurityPolicyMiddleware` is configured using the following settings:

- `SECURE_CSP`: defines the **enforced Content Security Policy**.
- `SECURE_CSP_REPORT_ONLY`: defines a **report-only Content Security Policy**.

<div class="admonition">

These settings can be used independently or together

- Use `SECURE_CSP` alone to enforce a policy that has already been tested and verified.
- Use `SECURE_CSP_REPORT_ONLY` on its own to evaluate a new policy without disrupting site behavior. This mode does not block violations, it only logs them. It's useful for testing and monitoring, but provides no protection against active threats.
- Use *both* to maintain an enforced baseline while experimenting with changes. Even for well-established policies, continuing to collect reports reports can help detect regressions, unexpected changes in behavior, or potential tampering in production environments.

</div>

## Policy violation reports

When a CSP violation occurs, browsers typically log details to the developer console, providing immediate feedback during development. To also receive these reports programmatically, the policy must include a [reporting directive](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy#reporting_directives) such as `report-uri` that specifies where violation data should be sent.

Django supports configuring these directives via the `SECURE_CSP_REPORT_ONLY` settings, but reports will only be issued by the browser if the policy explicitly includes a valid reporting directive.

Django does not provide built-in functionality to receive, store, or process violation reports. To collect and analyze them, you must implement your own reporting endpoint or integrate with a third-party monitoring service.

## CSP constants

Django provides predefined constants representing common CSP source expression keywords such as `'self'`, `'none'`, and `'unsafe-inline'`. These constants are intended for use in the directive values defined in the settings.

They are available through the `~django.utils.csp.CSP` enum, and using them is recommended over raw strings. This helps avoid common mistakes such as typos, improper quoting, or inconsistent formatting, and ensures compliance with the CSP specification.

<div class="module" synopsis="Constants for Content Security Policy">

django.utils.csp

</div>

<div class="CSP">

Enum providing standardized constants for common CSP source expressions.

<div class="attribute">

NONE

Represents `'none'`. Blocks loading resources for the given directive.

</div>

<div class="attribute">

REPORT_SAMPLE

Represents `'report-sample'`. Instructs the browser to include a sample of the violating code in reports. Note that this may expose sensitive data.

</div>

<div class="attribute">

SELF

Represents `'self'`. Allows loading resources from the same origin (same scheme, host, and port).

</div>

<div class="attribute">

STRICT_DYNAMIC

Represents `'strict-dynamic'`. Allows execution of scripts loaded by a trusted script (e.g., one with a valid nonce or hash), without needing `'unsafe-inline'`.

</div>

<div class="attribute">

UNSAFE_EVAL

Represents `'unsafe-eval'`. Allows use of `eval()` and similar JavaScript functions. Strongly discouraged.

</div>

<div class="attribute">

UNSAFE_HASHES

Represents `'unsafe-hashes'`. Allows inline event handlers and some `javascript:` URIs when their content hashes match a policy rule. Requires CSP Level 3+.

</div>

<div class="attribute">

UNSAFE_INLINE

Represents `'unsafe-inline'`. Allows execution of inline scripts, styles, and `javascript:` URLs. Generally discouraged, especially for scripts.

</div>

<div class="attribute">

WASM_UNSAFE_EVAL

Represents `'wasm-unsafe-eval'`. Permits compilation and execution of WebAssembly code without enabling `'unsafe-eval'` for scripts.

</div>

<div class="attribute">

NONCE

Django-specific placeholder value (`"<CSP_NONCE_SENTINEL>"`) used in `script-src` or `style-src` directives to activate nonce-based CSP. This string is replaced at runtime by the `~django.middleware.csp.ContentSecurityPolicyMiddleware` with a secure, random nonce that is generated for each request. See detailed explanation in `csp-nonce`.

</div>

</div>

## Decorators

<div class="module">

django.views.decorators.csp

</div>

Django provides decorators to control the Content Security Policy headers on a per-view basis. These allow overriding or disabling the enforced or report-only policy for specific views, providing fine-grained control when the global settings are not sufficient. Applying these overrides fully replaces the base CSP: they do not merge with existing rules. They can be used alongside the constants defined in `~django.utils.csp.CSP`.

<div class="warning">

<div class="title">

Warning

</div>

Weakening or disabling a CSP policy on any page can compromise the security of the entire site. Because of the "same origin" policy, an attacker could exploit a vulnerability on one page to access other parts of the site.

</div>

<div class="function">

csp_override(config)(view)

Overrides the `Content-Security-Policy` header for the decorated view using directives in the same format as the `SECURE_CSP` setting.

The `config` argument must be a mapping with the desired CSP directives. If `config` is an empty mapping (`{}`), no CSP enforcement header will be added to the response returned by that view, effectively disabling CSP for that view.

Examples:

    from django.http import HttpResponse
    from django.utils.csp import CSP
    from django.views.decorators.csp import csp_override


    @csp_override(
        {
            "default-src": [CSP.SELF],
            "img-src": [CSP.SELF, "data:"],
        }
    )
    def my_view(request):
        return HttpResponse("Custom Content-Security-Policy header applied")


    @csp_override({})
    def my_other_view(request):
        return HttpResponse("No Content-Security-Policy header added")

</div>

<div class="function">

csp_report_only_override(config)(view)

Overrides the `Content-Security-Policy-Report-Only` header for the decorated view using directives in the same format as the `SECURE_CSP_REPORT_ONLY` setting.

Like `csp_override`, the `config` argument must be a mapping with the desired CSP directives. If `config` is an empty mapping (`{}`), no CSP report-only header will be added to the response returned by that view, effectively disabling report-only CSP for that view.

Examples:

    from django.http import HttpResponse
    from django.utils.csp import CSP
    from django.views.decorators.csp import csp_report_only_override


    @csp_report_only_override(
        {
            "default-src": [CSP.SELF],
            "img-src": [CSP.SELF, "data:"],
            "report-uri": "https://mysite.com/csp-report/",
        }
    )
    def my_view(request):
        return HttpResponse("Custom Content-Security-Policy-Report-Only header applied")


    @csp_report_only_override({})
    def my_other_view(request):
        return HttpResponse("No Content-Security-Policy-Report-Only header added")

</div>

The examples above assume function-based views. For class-based views, see the `guide for decorating class-based views <decorating-class-based-views>`.

## Nonce usage

A CSP nonce ("number used once") is a unique, random value generated per HTTP response. Django supports nonces as a secure way to allow specific inline `<script>` or `<style>` elements to execute without relying on `'unsafe-inline'`.

Nonces are enabled by including the special placeholder `~django.utils.csp.CSP.NONCE` in the relevant directive(s) of your `CSP settings <csp-settings>`, such as `script-src` or `style-src`. When present, the `~django.middleware.csp.ContentSecurityPolicyMiddleware` will generate a nonce and insert the corresponding `nonce-<value>` source expression into the CSP header.

To use this nonce in templates, the `~django.template.context_processors.csp` context processor needs to be enabled. It adds a `csp_nonce` variable to the template context.

<div class="versionchanged">

6.1

The `csp_nonce_attr` template tag was added, including support for rendering `~django.forms.Media` objects.

</div>

For inline `<script>` and `<style>` elements, include the nonce directly using the context variable:

``` html+django
<script nonce="{{ csp_nonce }}">
  // This inline JavaScript will be allowed.
</script>
```

For external `<script src="...">` and `<link rel="stylesheet">` elements, use the `csp_nonce_attr` template tag:

``` html+django
<script src="/path/to/script.js" {% csp_nonce_attr %}></script>
<link rel="stylesheet" href="/path/to/style.css" {% csp_nonce_attr %}>
```

To render a `~django.forms.Media` object's assets with the nonce applied, pass the object to the `csp_nonce_attr` template tag:

``` html+django
{% csp_nonce_attr form.media %}
```

The browser will only execute inline elements that include a `nonce=<value>` attribute matching the one specified in the `Content-Security-Policy` (or `Content-Security-Policy-Report-Only`) header. This mechanism provides fine-grained control over which inline code is allowed to run.

If a template includes the CSP nonce but the policy does not include `~django.utils.csp.CSP.NONCE`, the HTML will include a nonce attribute, but the header will lack the required source expression. In this case, the browser will block the inline script or style (or report it for report-only configurations).

### Nonce generation and caching

Django's nonce generation is **lazy**: the middleware only generates a nonce if `{{ csp_nonce }}` is accessed during template rendering. This avoids unnecessary work for pages that do not use nonces.

However, because nonces must be unique per request, extra care is needed when using full-page caching (e.g., Django's cache middleware, CDN caching). Serving cached responses with previously generated nonces may result in reuse across users and requests. Although such responses may still appear to work (since the nonce in the CSP header and HTML content match), reuse defeats the purpose of the nonce and weakens security.

To ensure nonce-based policies remain effective:

- Avoid caching full responses that include `{{ csp_nonce }}` or `csp_nonce_attr`.
- If caching is necessary, use a strategy that injects a fresh nonce on each request, or consider refactoring your application to avoid inline scripts and styles altogether.
