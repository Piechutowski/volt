# View decorators

<div class="module">

django.views.decorators.http

</div>

Django provides several decorators that can be applied to views to support various HTTP features.

See `decorating-class-based-views` for how to use these decorators with class-based views.

## Allowed HTTP methods

The decorators in `django.views.decorators.http` can be used to restrict access to views based on the request method. These decorators will return a `django.http.HttpResponseNotAllowed` if the conditions are not met.

<div class="function">

require_http_methods(request_method_list)

Decorator to require that a view only accepts particular request methods. Usage:

    from django.views.decorators.http import require_http_methods


    @require_http_methods(["GET", "POST"])
    def my_view(request):
        # I can assume now that only GET or POST requests make it this far
        # ...
        pass

Note that request methods should be in uppercase.

</div>

<div class="function">

require_GET()

Decorator to require that a view only accepts the GET method.

</div>

<div class="function">

require_POST()

Decorator to require that a view only accepts the POST method.

</div>

<div class="function">

require_safe()

Decorator to require that a view only accepts the GET and HEAD methods. These methods are commonly considered "safe" because they should not have the significance of taking an action other than retrieving the requested resource.

<div class="note">

<div class="title">

Note

</div>

Web servers should automatically strip the content of responses to HEAD requests while leaving the headers unchanged, so you may handle HEAD requests exactly like GET requests in your views. Since some software, such as link checkers, rely on HEAD requests, you might prefer using `require_safe` instead of `require_GET`.

</div>

</div>

## Conditional view processing

The following decorators in `django.views.decorators.http` can be used to control caching behavior on particular views.

<div class="function">

condition(etag_func=None, last_modified_func=None)

</div>

<div class="function">

conditional_page()

This decorator provides the conditional GET operation handling of `~django.middleware.http.ConditionalGetMiddleware` to a view.

</div>

<div class="function">

etag(etag_func)

</div>

<div class="function">

last_modified(last_modified_func)

These decorators can be used to generate `ETag` and `Last-Modified` headers; see `conditional view processing </topics/conditional-view-processing>`.

</div>

<div class="module">

django.views.decorators.gzip

</div>

## GZip compression

The decorators in `django.views.decorators.gzip` control content compression on a per-view basis.

<div class="function">

gzip_page()

This decorator compresses content if the browser allows gzip compression. It sets the `Vary` header accordingly, so that caches will base their storage on the `Accept-Encoding` header.

</div>

<div class="module">

django.views.decorators.vary

</div>

## Vary headers

The decorators in `django.views.decorators.vary` can be used to control caching based on specific request headers.

<div class="function">

vary_on_cookie(func)

</div>

<div class="function">

vary_on_headers(\*headers)

The `Vary` header defines which request headers a cache mechanism should take into account when building its cache key.

See `using vary headers <using-vary-headers>`.

</div>

<div class="module">

django.views.decorators.cache

</div>

## Caching

The decorators in `django.views.decorators.cache` control server and client-side caching.

<div class="function">

cache_control(\*\*kwargs)

This decorator patches the response's `Cache-Control` header by adding all of the keyword arguments to it. See `~django.utils.cache.patch_cache_control` for the details of the transformation.

</div>

<div class="function">

never_cache(view_func)

This decorator adds an `Expires` header to the current date/time.

This decorator adds a `Cache-Control: max-age=0, no-cache, no-store, must-revalidate, private` header to a response to indicate that a page should never be cached.

Each header is only added if it isn't already set.

</div>

<div class="module">

django.views.decorators.common

</div>

## Common

The decorators in `django.views.decorators.common` allow per-view customization of `~django.middleware.common.CommonMiddleware` behavior.

<div class="function">

no_append_slash()

This decorator allows individual views to be excluded from `APPEND_SLASH` URL normalization.

</div>
