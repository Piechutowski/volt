# Django shortcut functions

<div class="module" synopsis="Convenience shortcuts that span multiple levels of Django's MVC stack.">

django.shortcuts

</div>

<div class="index">

shortcuts

</div>

The package `django.shortcuts` collects helper functions and classes that "span" multiple levels of MVC. In other words, these functions/classes introduce controlled coupling for convenience's sake.

## `render()`

<div class="function">

render(request, template_name, context=None, content_type=None, status=None, using=None)

Combines a given template with a given context dictionary and returns an `~django.http.HttpResponse` object with that rendered text.

Django does not provide a shortcut function which returns a `~django.template.response.TemplateResponse` because the constructor of `~django.template.response.TemplateResponse` offers the same level of convenience as `render`.

</div>

### Required arguments

`request`  
The request object used to generate this response.

`template_name`  
The full name of a template to use or sequence of template names. If a sequence is given, the first template that exists will be used. See the `template loading documentation <template-loading>` for more information on how templates are found.

### Optional arguments

`context`  
A dictionary of values to add to the template context. By default, this is an empty dictionary. If a value in the dictionary is callable, the view will call it just before rendering the template.

`content_type`  
The MIME type to use for the resulting document. Defaults to `'text/html'`.

`status`  
The status code for the response. Defaults to `200`.

`using`  
The `NAME <TEMPLATES-NAME>` of a template engine to use for loading the template.

### Example

The following example renders the template `myapp/index.html` with the MIME type `application/xhtml+xml`:

    from django.shortcuts import render


    def my_view(request):
        # View code here...
        return render(
            request,
            "myapp/index.html",
            {
                "foo": "bar",
            },
            content_type="application/xhtml+xml",
        )

This example is equivalent to:

    from django.http import HttpResponse
    from django.template import loader


    def my_view(request):
        # View code here...
        t = loader.get_template("myapp/index.html")
        c = {"foo": "bar"}
        return HttpResponse(t.render(c, request), content_type="application/xhtml+xml")

## `redirect()`

<div class="function">

redirect(to, *args, permanent=False, preserve_request=False, max_length=MAX_URL_REDIRECT_LENGTH,*\*kwargs)

Returns an `~django.http.HttpResponseRedirect` to the appropriate URL for the arguments passed.

The arguments could be:

- A model: the model's `~django.db.models.Model.get_absolute_url` function will be called.
- A view name, possibly with arguments: `~django.urls.reverse` will be used to reverse-resolve the name.
- An absolute or relative URL, which will be used as-is for the redirect location.

By default, a temporary redirect is issued with a 302 status code. If `permanent=True`, a permanent redirect is issued with a 301 status code.

If `preserve_request=True`, the response instructs the user agent to preserve the method and body of the original request when issuing the redirect. In this case, temporary redirects use a 307 status code, and permanent redirects use a 308 status code. This is better illustrated in the following table:

| permanent | preserve_request | HTTP status code |
|-----------|------------------|------------------|
| `True`    | `False`          | 301              |
| `False`   | `False`          | 302              |
| `False`   | `True`           | 307              |
| `True`    | `True`           | 308              |

An optional `max_length` keyword argument can be provided to override the maximum allowed length for the redirect URL. Set it to `None` to disable the length check.

<div class="versionchanged">

6.1

`max_length` was added.

</div>

</div>

### Examples

You can use the `redirect` function in a number of ways.

1.  By passing some object; that object's `~django.db.models.Model.get_absolute_url` method will be called to figure out the redirect URL:

        from django.shortcuts import redirect


        def my_view(request):
            ...
            obj = MyModel.objects.get(...)
            return redirect(obj)

2.  By passing the name of a view and optionally some positional or keyword arguments; the URL will be reverse resolved using the `~django.urls.reverse` method:

        def my_view(request):
            ...
            return redirect("some-view-name", foo="bar")

3.  By passing a hardcoded URL to redirect to: :

        def my_view(request):
            ...
            return redirect("/some/url/")

    This also works with full URLs: :

        def my_view(request):
            ...
            return redirect("https://example.com/")

By default, `redirect` returns a temporary redirect. All of the above forms accept a `permanent` argument; if set to `True` a permanent redirect will be returned:

    def my_view(request):
        ...
        obj = MyModel.objects.get(...)
        return redirect(obj, permanent=True)

Additionally, the `preserve_request` argument can be used to preserve the original HTTP method:

    def my_view(request):
        # ...
        obj = MyModel.objects.get(...)
        if request.method in ("POST", "PUT"):
            # Redirection preserves the original request method.
            return redirect(obj, preserve_request=True)
        # ...

## `resolve_url()`

<div class="function">

resolve_url(to, *args,*\*kwargs)

Returns a URL string by resolving and normalizing the given `to` argument into a concrete URL. The parameter `to` may be:

- An object implementing `~django.db.models.Model.get_absolute_url`, in which case the method will be called and its result returned.
- A view name, view function, or view class, possibly with arguments passed as `*args` and `**kwargs`, in which case `~django.urls.reverse` will be used to reverse-resolve the view.
- A URL string, which will be returned unchanged.

This function is used internally by the `redirect` shortcut to determine the target URL for the redirect location.

</div>

### Examples

1.  Resolving a URL for a model that defines `~django.db.models.Model.get_absolute_url`:

    ``` python
    from django.db import models
    from django.urls import reverse


    class Article(models.Model):
        title = models.CharField(max_length=100)

        def get_absolute_url(self):
            return reverse("article-detail", args=[self.pk])
    ```

    ``` python
    from django.http import JsonResponse
    from django.shortcuts import get_object_or_404, resolve_url
    from .models import Article


    def article_api_view(request, pk):
        """Return metadata about an article, including its canonical URL."""
        article = get_object_or_404(Article, pk=pk)
        return JsonResponse(
            {
                "id": article.pk,
                "title": article.title,
                "url": resolve_url(article),
            }
        )
    ```

2.  Resolving a target URL for use outside of a redirect, such as in an HTTP response header:

        from django.conf import settings
        from django.http import HttpResponse
        from django.shortcuts import resolve_url


        def login_success(request):
            response = HttpResponse("Login successful")
            response["X-Next-URL"] = resolve_url(settings.LOGIN_REDIRECT_URL)
            return response

## `get_object_or_404()`

<div class="function">

get_object_or_404(klass, *args,*\*kwargs)

</div>

<div class="function">

aget_object_or_404(klass, *args,*\*kwargs)

*Asynchronous version*: `aget_object_or_404()`

Calls `~django.db.models.query.QuerySet.get` on a given model manager, but it raises `~django.http.Http404` instead of the model's `~django.db.models.Model.DoesNotExist` exception.

</div>

### Arguments

`klass`  
A `~django.db.models.Model` class, a `~django.db.models.Manager`, or a `~django.db.models.query.QuerySet` instance from which to get the object.

`*args`  
`q-objects`.

`**kwargs`  
Lookup parameters, which should be in the format accepted by `get()` and `filter()`.

### Example

The following example gets the object with the primary key of 1 from `MyModel`:

    from django.shortcuts import get_object_or_404


    def my_view(request):
        obj = get_object_or_404(MyModel, pk=1)

This example is equivalent to:

    from django.http import Http404


    def my_view(request):
        try:
            obj = MyModel.objects.get(pk=1)
        except MyModel.DoesNotExist:
            raise Http404("No MyModel matches the given query.")

The most common use case is to pass a `~django.db.models.Model`, as shown above. However, you can also pass a `~django.db.models.query.QuerySet` instance:

    queryset = Book.objects.filter(title__startswith="M")
    get_object_or_404(queryset, pk=1)

The above example is a bit contrived since it's equivalent to doing:

    get_object_or_404(Book, title__startswith="M", pk=1)

but it can be useful if you are passed the `queryset` variable from somewhere else.

Finally, you can also use a `~django.db.models.Manager`. This is useful for example if you have a `custom manager<custom-managers>`:

    get_object_or_404(Book.dahl_objects, title="Matilda")

You can also use `related managers<django.db.models.fields.related.RelatedManager>`:

    author = Author.objects.get(name="Roald Dahl")
    get_object_or_404(author.book_set, title="Matilda")

Note: As with `get()`, a `~django.core.exceptions.MultipleObjectsReturned` exception will be raised if more than one object is found.

## `get_list_or_404()`

<div class="function">

get_list_or_404(klass, *args,*\*kwargs)

</div>

<div class="function">

aget_list_or_404(klass, *args,*\*kwargs)

*Asynchronous version*: `aget_list_or_404()`

Returns the result of `~django.db.models.query.QuerySet.filter` on a given model manager cast to a list, raising `~django.http.Http404` if the resulting list is empty.

</div>

### Arguments

`klass`  
A `~django.db.models.Model`, `~django.db.models.Manager` or `~django.db.models.query.QuerySet` instance from which to get the list.

`*args`  
`q-objects`.

`**kwargs`  
Lookup parameters, which should be in the format accepted by `get()` and `filter()`.

### Example

The following example gets all published objects from `MyModel`:

    from django.shortcuts import get_list_or_404


    def my_view(request):
        my_objects = get_list_or_404(MyModel, published=True)

This example is equivalent to:

    from django.http import Http404


    def my_view(request):
        my_objects = list(MyModel.objects.filter(published=True))
        if not my_objects:
            raise Http404("No MyModel matches the given query.")
