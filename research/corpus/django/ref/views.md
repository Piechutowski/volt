# Built-in Views

<div class="module" synopsis="Django's built-in views.">

django.views

</div>

Several of Django's built-in views are documented in `/topics/http/views` as well as elsewhere in the documentation.

## Serving files in development

<div class="function">

static.serve(request, path, document_root, show_indexes=False)

</div>

There may be files other than your project's static assets that, for convenience, you'd like to have Django serve for you in local development. The `~django.views.static.serve` view can be used to serve any directory you give it. (This view is **not** hardened for production use and should be used only as a development aid; you should serve these files in production using a real front-end web server).

The most likely example is user-uploaded content in `MEDIA_ROOT`. `django.contrib.staticfiles` is intended for static assets and has no built-in handling for user-uploaded files, but you can have Django serve your `MEDIA_ROOT` by appending something like this to your URLconf:

    from django.conf import settings
    from django.urls import re_path
    from django.views.static import serve

    # ... the rest of your URLconf goes here ...

    if settings.DEBUG:
        urlpatterns += [
            re_path(
                r"^media/(?P<path>.*)$",
                serve,
                {
                    "document_root": settings.MEDIA_ROOT,
                },
            ),
        ]

Note, the snippet assumes your `MEDIA_URL` has a value of `'media/'`. This will call the `~django.views.static.serve` view, passing in the path from the URLconf and the (required) `document_root` parameter.

Since it can become a bit cumbersome to define this URL pattern, Django ships with a small URL helper function `~django.conf.urls.static.static` that takes as parameters the prefix such as `MEDIA_URL` and a dotted path to a view, such as `'django.views.static.serve'`. Any other function parameter will be transparently passed to the view.

## Error views

Django comes with a few views by default for handling HTTP errors. To override these with your own custom views, see `customizing-error-views`.

### The 404 (page not found) view

<div class="function">

defaults.page_not_found(request, exception, template_name='404.html')

</div>

When you raise `~django.http.Http404` from within a view, Django loads a special view devoted to handling 404 errors. By default, it's the view `django.views.defaults.page_not_found`, which either produces a "Not Found" message or loads and renders the template `404.html` if you created it in your root template directory.

The default 404 view will pass two variables to the template: `request_path`, which is the URL that resulted in the error, and `exception`, which is a useful representation of the exception that triggered the view (e.g. containing any message passed to a specific `Http404` instance).

Three things to note about 404 views:

- The 404 view is also called if Django doesn't find a match after checking every regular expression in the URLconf.
- The 404 view is passed a `~django.template.RequestContext` and will have access to variables supplied by your template context processors (e.g. `MEDIA_URL`).
- If `DEBUG` is set to `True` (in your settings module), then your 404 view will never be used, and your URLconf will be displayed instead, with some debug information.

### The 500 (server error) view

<div class="function">

defaults.server_error(request, template_name='500.html')

</div>

Similarly, Django executes special-case behavior in the case of runtime errors in view code. If a view results in an exception, Django will, by default, call the view `django.views.defaults.server_error`, which either produces a "Server Error" message or loads and renders the template `500.html` if you created it in your root template directory.

The default 500 view passes no variables to the `500.html` template and is rendered with an empty `Context` to lessen the chance of additional errors.

If `DEBUG` is set to `True` (in your settings module), then your 500 view will never be used, and the traceback will be displayed instead, with some debug information.

### The 403 (HTTP Forbidden) view

<div class="function">

defaults.permission_denied(request, exception, template_name='403.html')

</div>

In the same vein as the 404 and 500 views, Django has a view to handle 403 Forbidden errors. If a view results in a 403 exception then Django will, by default, call the view `django.views.defaults.permission_denied`.

This view loads and renders the template `403.html` in your root template directory, or if this file does not exist, instead serves the text "403 Forbidden", as per `9110#section-15.5.4` (the HTTP 1.1 Specification). The template context contains `exception`, which is the string representation of the exception that triggered the view.

`django.views.defaults.permission_denied` is triggered by a `~django.core.exceptions.PermissionDenied` exception. To deny access in a view you can use code like this:

    from django.core.exceptions import PermissionDenied


    def edit(request, pk):
        if not request.user.is_staff:
            raise PermissionDenied
        # ...

### The 400 (bad request) view

<div class="function">

defaults.bad_request(request, exception, template_name='400.html')

</div>

Similarly, Django has a view to handle 400 Bad Request errors.

This view either produces a "Bad Request" message or loads and renders the template `400.html` if you created it in your root template directory. It returns with status code 400 indicating that the error condition was the result of a client operation. By default, nothing related to the exception that triggered the view is passed to the template context, as the exception message might contain sensitive information like filesystem paths.

`django.views.defaults.bad_request` is triggered by a `~django.core.exceptions.BadRequest` exception. Also, when a `~django.core.exceptions.SuspiciousOperation` is raised in Django, it may be handled by a component of Django (for example resetting the session data). If not specifically handled, Django will consider the current request a 'bad request' instead of a server error, and handle it with `bad_request`.

`bad_request` views are also only used when `DEBUG` is `False`.
