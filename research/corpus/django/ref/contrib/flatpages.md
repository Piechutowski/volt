# The flatpages app

<div class="module" synopsis="A framework for managing &quot;flat&quot; HTML content in a database.">

django.contrib.flatpages

</div>

Django comes with an optional "flatpages" application. It lets you store "flat" HTML content in a database and handles the management for you via Django's admin interface and a Python API.

A flatpage is an object with a URL, title and content. Use it for one-off, special-case pages, such as "About" or "Privacy Policy" pages, that you want to store in a database but for which you don't want to develop a custom Django application.

A flatpage can use a custom template or a default, systemwide flatpage template. It can be associated with one, or multiple, sites.

The content field may optionally be left blank if you prefer to put your content in a custom template.

## Installation

To install the flatpages app, follow these steps:

1.  Install the `sites framework <django.contrib.sites>` by adding `'django.contrib.sites'` to your `INSTALLED_APPS` setting, if it's not already in there.

    Also make sure you've correctly set `SITE_ID` to the ID of the site the settings file represents. This will usually be `1` (i.e. `SITE_ID = 1`), but if you're using the sites framework to manage multiple sites, it could be the ID of a different site.

2.  Add `'django.contrib.flatpages'` to your `INSTALLED_APPS` setting.

Then either:

3.  Add an entry in your URLconf. For example:

        urlpatterns = [
            path("pages/", include("django.contrib.flatpages.urls")),
        ]

or:

3.  Add `'django.contrib.flatpages.middleware.FlatpageFallbackMiddleware'` to your `MIDDLEWARE` setting.
4.  Run the command `manage.py migrate <migrate>`.

<div class="currentmodule">

django.contrib.flatpages.middleware

</div>

## How it works

`manage.py migrate` creates two tables in your database: `django_flatpage` and `django_flatpage_sites`. `django_flatpage` is a lookup table that maps a URL to a title and bunch of text content. `django_flatpage_sites` associates a flatpage with a site.

### Using the URLconf

There are several ways to include the flatpages in your URLconf. You can dedicate a particular path to flatpages:

    urlpatterns = [
        path("pages/", include("django.contrib.flatpages.urls")),
    ]

You can also set it up as a "catchall" pattern. In this case, it is important to place the pattern at the end of the other urlpatterns:

    from django.contrib.flatpages import views

    # Your other patterns here
    urlpatterns += [
        re_path(r"^(?P<url>.*/)$", views.flatpage),
    ]

<div class="warning">

<div class="title">

Warning

</div>

If you set `APPEND_SLASH` to `False`, you must remove the slash in the catchall pattern or flatpages without a trailing slash will not be matched.

</div>

Another common setup is to use flatpages for a limited set of known pages and to hardcode their URLs in the `URLconf </topics/http/urls>`:

    from django.contrib.flatpages import views

    urlpatterns += [
        path("about-us/", views.flatpage, kwargs={"url": "/about-us/"}, name="about"),
        path("license/", views.flatpage, kwargs={"url": "/license/"}, name="license"),
    ]

The `kwargs` argument sets the `url` value used for the `FlatPage` model lookup in the flatpage view.

The `name` argument allows the URL to be reversed in templates, for example using the `url` template tag.

### Using the middleware

The `~django.contrib.flatpages.middleware.FlatpageFallbackMiddleware` can do all of the work.

<div class="FlatpageFallbackMiddleware">

Each time any Django application raises a 404 error, this middleware checks the flatpages database for the requested URL as a last resort. Specifically, it checks for a flatpage with the given URL with a site ID that corresponds to the `SITE_ID` setting.

If it finds a match, it follows this algorithm:

- If the flatpage has a custom template, it loads that template. Otherwise, it loads the template `flatpages/default.html`.
- It passes that template a single context variable, `flatpage`, which is the flatpage object. It uses `~django.template.RequestContext` in rendering the template.

The middleware will only add a trailing slash and redirect (by looking at the `APPEND_SLASH` setting) if the resulting URL refers to a valid flatpage. Redirects are permanent (301 status code).

If it doesn't find a match, the request continues to be processed as usual.

The middleware only gets activated for 404s -- not for 500s or responses of any other status code.

</div>

<div class="admonition">

Flatpages will not apply view middleware

Because the `FlatpageFallbackMiddleware` is applied only after URL resolution has failed and produced a 404, the response it returns will not apply any `view middleware <view-middleware>` methods. Only requests which are successfully routed to a view via normal URL resolution apply view middleware.

</div>

Note that the order of `MIDDLEWARE` matters. Generally, you can put `~django.contrib.flatpages.middleware.FlatpageFallbackMiddleware` at the end of the list. This means it will run first when processing the response, and ensures that any other response-processing middleware see the real flatpage response rather than the 404.

For more on middleware, read the `middleware docs
</topics/http/middleware>`.

<div class="admonition">

Ensure that your 404 template works

Note that the `~django.contrib.flatpages.middleware.FlatpageFallbackMiddleware` only steps in once another view has successfully produced a 404 response. If another view or middleware class attempts to produce a 404 but ends up raising an exception instead, the response will become an HTTP 500 ("Internal Server Error") and the `~django.contrib.flatpages.middleware.FlatpageFallbackMiddleware` will not attempt to serve a flatpage.

</div>

<div class="currentmodule">

django.contrib.flatpages.models

</div>

## How to add, change and delete flatpages

<div class="warning">

<div class="title">

Warning

</div>

Permissions to add or edit flatpages should be restricted to trusted users. Flatpages are defined by raw HTML and are **not sanitized** by Django. As a consequence, a malicious flatpage can lead to various security vulnerabilities, including permission escalation.

</div>

### Via the admin interface

If you've activated the automatic Django admin interface, you should see a "Flatpages" section on the admin index page. Edit flatpages as you edit any other object in the system.

The `FlatPage` model has an `enable_comments` field that isn't used by `contrib.flatpages`, but that could be useful for your project or third-party apps. It doesn't appear in the admin interface, but you can add it by registering a custom `ModelAdmin` for `FlatPage`:

    from django.contrib import admin
    from django.contrib.flatpages.admin import FlatPageAdmin
    from django.contrib.flatpages.models import FlatPage
    from django.utils.translation import gettext_lazy as _


    # Define a new FlatPageAdmin
    class FlatPageAdmin(FlatPageAdmin):
        fieldsets = [
            (None, {"fields": ["url", "title", "content", "sites"]}),
            (
                _("Advanced options"),
                {
                    "classes": ["collapse"],
                    "fields": [
                        "enable_comments",
                        "registration_required",
                        "template_name",
                    ],
                },
            ),
        ]


    # Re-register FlatPageAdmin
    admin.site.unregister(FlatPage)
    admin.site.register(FlatPage, FlatPageAdmin)

### Via the Python API

Flatpages are represented by a standard `Django model </topics/db/models>`, `.FlatPage`. You can access flatpage objects via the `Django database API </topics/db/queries>`.

<div class="admonition">

Check for duplicate flatpage URLs.

If you add or modify flatpages via your own code, you will likely want to check for duplicate flatpage URLs within the same site. The flatpage form used in the admin performs this validation check, and can be imported from `django.contrib.flatpages.forms.FlatpageForm` and used in your own views.

</div>

<div class="currentmodule">

django.contrib.flatpages

</div>

## `FlatPage` model

### Fields

`~django.contrib.flatpages.models.FlatPage` objects have the following fields:

<div class="models.FlatPage" noindex="">

<div class="attribute">

url

Required. 100 characters or fewer. Indexed for faster lookups.

</div>

<div class="attribute">

title

Required. 200 characters or fewer.

</div>

<div class="attribute">

content

Optional (`blank=True <django.db.models.Field.blank>`). `~django.db.models.TextField` that typically, contains the HTML content of the page.

</div>

<div class="attribute">

enable_comments

Boolean. This field is not used by `~django.contrib.flatpages` by default and does not appear in the admin interface. Please see `flatpages admin interface section <flatpages-admin>` for a detailed explanation.

</div>

<div class="attribute">

template_name

Optional (`blank=True <django.db.models.Field.blank>`). 70 characters or fewer. Specifies the template used to render the page. Defaults to `flatpages/default.html` if not provided.

</div>

> Boolean. When `True`, restricts the page access to logged-in users only.

<div class="attribute">

sites

Many-to-many relationship to `~django.contrib.sites.models.Site`, which determines the `sites </ref/contrib/sites>` the flatpage is available on.

</div>

</div>

### Methods

<div class="models.FlatPage" noindex="">

<div class="method">

get_absolute_url()

Returns the relative URL path of the page based on the `~django.contrib.flatpages.models.FlatPage.url` attribute.

</div>

</div>

## Flatpage templates

By default, flatpages are rendered via the template `flatpages/default.html`, but you can override that for a particular flatpage: in the admin, a collapsed fieldset titled "Advanced options" (clicking will expand it) contains a field for specifying a template name. If you're creating a flatpage via the Python API you can set the template name as the field `template_name` on the `FlatPage` object.

Creating the `flatpages/default.html` template is your responsibility; in your template directory, create a `flatpages` directory containing a file `default.html`.

Flatpage templates are passed a single context variable, `flatpage`, which is the flatpage object.

Here's a sample `flatpages/default.html` template:

``` html+django
<!DOCTYPE html>
<html lang="en">
<head>
<title>{{ flatpage.title }}</title>
</head>
<body>
{{ flatpage.content }}
</body>
</html>
```

Since you're already entering raw HTML into the admin page for a flatpage, both `flatpage.title` and `flatpage.content` are marked as **not** requiring `automatic HTML escaping <automatic-html-escaping>` in the template.

## Getting a list of `~django.contrib.flatpages.models.FlatPage` objects in your templates

The flatpages app provides a template tag that allows you to iterate over all of the available flatpages on the `current site
<hooking-into-current-site-from-views>`.

Like all custom template tags, you'll need to `load its custom
tag library <loading-custom-template-libraries>` before you can use it. After loading the library, you can retrieve all current flatpages via the `get_flatpages` tag:

``` html+django
{% load flatpages %}
{% get_flatpages as flatpages %}
<ul>
    {% for page in flatpages %}
        <li><a href="{{ page.url }}">{{ page.title }}</a></li>
    {% endfor %}
</ul>
```

<div class="templatetag">

get_flatpages

</div>

### Displaying `registration_required` flatpages

By default, the `get_flatpages` template tag will only show flatpages that are marked `registration_required = False`. If you want to display registration-protected flatpages, you need to specify an authenticated user using a `for` clause.

For example:

``` html+django
{% get_flatpages for someuser as about_pages %}
```

If you provide an anonymous user, `get_flatpages` will behave the same as if you hadn't provided a user -- i.e., it will only show you public flatpages.

### Limiting flatpages by base URL

An optional argument, `starts_with`, can be applied to limit the returned pages to those beginning with a particular base URL. This argument may be passed as a string, or as a variable to be resolved from the context.

For example:

``` html+django
{% get_flatpages '/about/' as about_pages %}
{% get_flatpages about_prefix as about_pages %}
{% get_flatpages '/about/' for someuser as about_pages %}
```

## Integrating with `django.contrib.sitemaps`

<div class="currentmodule">

django.contrib.flatpages.sitemaps

</div>

<div class="FlatPageSitemap">

The `sitemaps.FlatPageSitemap
<django.contrib.flatpages.sitemaps.FlatPageSitemap>` class looks at all publicly visible `~django.contrib.flatpages` defined for the current `SITE_ID` (see the `sites documentation
<django.contrib.sites>`) and creates an entry in the sitemap. These entries include only the `~django.contrib.sitemaps.Sitemap.location` attribute -- not `~django.contrib.sitemaps.Sitemap.lastmod`, `~django.contrib.sitemaps.Sitemap.changefreq` or `~django.contrib.sitemaps.Sitemap.priority`.

</div>

### Example

Here's an example of a URLconf using `FlatPageSitemap`:

    from django.contrib.flatpages.sitemaps import FlatPageSitemap
    from django.contrib.sitemaps.views import sitemap
    from django.urls import path

    urlpatterns = [
        # ...
        # the sitemap
        path(
            "sitemap.xml",
            sitemap,
            {"sitemaps": {"flatpages": FlatPageSitemap}},
            name="django.contrib.sitemaps.views.sitemap",
        ),
    ]
