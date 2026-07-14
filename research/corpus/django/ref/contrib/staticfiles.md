# The `staticfiles` app

<div class="module" synopsis="An app for handling static files.">

django.contrib.staticfiles

</div>

`django.contrib.staticfiles` collects static files from each of your applications (and any other places you specify) into a single location that can easily be served in production.

<div class="seealso">

For an introduction to the static files app and some usage examples, see `/howto/static-files/index`. For guidelines on deploying static files, see `/howto/static-files/deployment`.

</div>

## Settings

See `staticfiles settings <settings-staticfiles>` for details on the following settings:

- `STORAGES`
- `STATIC_ROOT`
- `STATIC_URL`
- `STATICFILES_DIRS`
- `STATICFILES_FINDERS`

## Management Commands

`django.contrib.staticfiles` exposes three management commands.

### `collectstatic`

<div class="django-admin">

collectstatic

</div>

Collects the static files into `STATIC_ROOT`.

Duplicate file names are by default resolved in a similar way to how template resolution works: the file that is first found in one of the specified locations will be used. If you're confused, the `findstatic` command can help show you which files are found.

On subsequent `collectstatic` runs (if `STATIC_ROOT` isn't empty), files are copied only if they have a modified timestamp greater than the timestamp of the file in `STATIC_ROOT`. Therefore if you remove an application from `INSTALLED_APPS`, it's a good idea to use the `collectstatic
--clear` option in order to remove stale static files.

Files are searched by using the `enabled finders
<STATICFILES_FINDERS>`. The default is to look in all locations defined in `STATICFILES_DIRS` and in the `'static'` directory of apps specified by the `INSTALLED_APPS` setting.

The `collectstatic` management command calls the `~django.contrib.staticfiles.storage.StaticFilesStorage.post_process` method of the `staticfiles` storage backend from `STORAGES` after each run and passes a list of paths that have been found by the management command. It also receives all command line options of `collectstatic`. This is used by the `~django.contrib.staticfiles.storage.ManifestStaticFilesStorage` by default.

By default, collected files receive permissions from `FILE_UPLOAD_PERMISSIONS` and collected directories receive permissions from `FILE_UPLOAD_DIRECTORY_PERMISSIONS`. If you would like different permissions for these files and/or directories, you can subclass either of the `static files storage classes <staticfiles-storages>` and specify the `file_permissions_mode` and/or `directory_permissions_mode` parameters, respectively. For example:

    from django.contrib.staticfiles import storage


    class MyStaticFilesStorage(storage.StaticFilesStorage):
        def __init__(self, *args, **kwargs):
            kwargs["file_permissions_mode"] = 0o640
            kwargs["directory_permissions_mode"] = 0o760
            super().__init__(*args, **kwargs)

Then set the `staticfiles` storage backend in `STORAGES` setting to `'path.to.MyStaticFilesStorage'`.

Some commonly used options are:

<div class="django-admin-option">

--noinput, --no-input

Do NOT prompt the user for input of any kind.

</div>

<div class="django-admin-option">

--ignore PATTERN, -i PATTERN

Ignore files, directories, or paths matching this glob-style pattern. Use multiple times to ignore more. When specifying a path, always use forward slashes, even on Windows.

</div>

<div class="django-admin-option">

--dry-run, -n

Do everything except modify the filesystem.

</div>

<div class="django-admin-option">

--clear, -c

Clear the existing files before trying to copy or link the original file.

</div>

<div class="django-admin-option">

--link, -l

Create a symbolic link to each file instead of copying.

</div>

<div class="django-admin-option">

--no-post-process

Don't call the `~django.contrib.staticfiles.storage.StaticFilesStorage.post_process` method of the configured `staticfiles` storage backend from `STORAGES`.

</div>

<div class="django-admin-option">

--no-default-ignore

Don't ignore the common private glob-style patterns `'CVS'`, `'.*'` and `'*~'`.

</div>

For a full list of options, refer to the commands own help by running:

<div class="console">

\$ python manage.py collectstatic --help

</div>

#### Customizing the ignored pattern list

The default ignored pattern list, `['CVS', '.*', '*~']`, can be customized in a more persistent way than providing the `--ignore` command option at each `collectstatic` invocation. Provide a custom `~django.apps.AppConfig` class, override the `ignore_patterns` attribute of this class and replace `'django.contrib.staticfiles'` with that class path in your `INSTALLED_APPS` setting:

    from django.contrib.staticfiles.apps import StaticFilesConfig


    class MyStaticFilesConfig(StaticFilesConfig):
        ignore_patterns = [...]  # your custom ignore list

### `findstatic`

<div class="django-admin">

findstatic staticfile \[staticfile ...\]

</div>

Searches for one or more relative paths with the enabled finders.

For example:

<div class="console">

\$ python manage.py findstatic css/base.css admin/js/core.js Found 'css/base.css' here: /home/special.polls.com/core/static/css/base.css /home/polls.com/core/static/css/base.css Found 'admin/js/core.js' here: /home/polls.com/src/django/contrib/admin/media/js/core.js

</div>

<div class="django-admin-option">

findstatic --first

</div>

By default, all matching locations are found. To only return the first match for each relative path, use the `--first` option:

<div class="console">

\$ python manage.py findstatic css/base.css --first Found 'css/base.css' here: /home/special.polls.com/core/static/css/base.css

</div>

This is a debugging aid; it'll show you exactly which static file will be collected for a given path.

By setting the `--verbosity` flag to 0, you can suppress the extra output and just get the path names:

<div class="console">

\$ python manage.py findstatic css/base.css --verbosity 0 /home/special.polls.com/core/static/css/base.css /home/polls.com/core/static/css/base.css

</div>

On the other hand, by setting the `--verbosity` flag to 2, you can get all the directories which were searched:

<div class="console">

\$ python manage.py findstatic css/base.css --verbosity 2 Found 'css/base.css' here: /home/special.polls.com/core/static/css/base.css /home/polls.com/core/static/css/base.css Looking in the following locations: /home/special.polls.com/core/static /home/polls.com/core/static /some/other/path/static

</div>

### `runserver`

<div class="django-admin" noindex="">

runserver \[addrport\]

</div>

Overrides the core `runserver` command if the `staticfiles` app is `installed<INSTALLED_APPS>` and adds automatic serving of static files. File serving doesn't run through `MIDDLEWARE`.

The command adds these options:

<div class="django-admin-option">

--nostatic

</div>

Use the `--nostatic` option to disable serving of static files with the `staticfiles </ref/contrib/staticfiles>` app entirely. This option is only available if the `staticfiles </ref/contrib/staticfiles>` app is in your project's `INSTALLED_APPS` setting.

Example usage:

<div class="console">

\$ django-admin runserver --nostatic

</div>

<div class="django-admin-option">

--insecure

</div>

Use the `--insecure` option to force serving of static files with the `staticfiles </ref/contrib/staticfiles>` app even if the `DEBUG` setting is `False`. By using this you acknowledge the fact that it's **grossly inefficient** and probably **insecure**. This is only intended for local development, should **never be used in production** and is only available if the `staticfiles </ref/contrib/staticfiles>` app is in your project's `INSTALLED_APPS` setting.

`--insecure` doesn't work with `~.storage.ManifestStaticFilesStorage`.

Example usage:

<div class="console">

\$ django-admin runserver --insecure

</div>

## Storages

### `StaticFilesStorage`

<div class="storage.StaticFilesStorage">

A subclass of the `~django.core.files.storage.FileSystemStorage` storage backend that uses the `STATIC_ROOT` setting as the base file system location and the `STATIC_URL` setting respectively as the base URL.

</div>

<div class="method">

storage.StaticFilesStorage.post_process(paths, \*\*options)

</div>

If this method is defined on a storage, it's called by the `collectstatic` management command after each run and gets passed the local storages and paths of found files as a dictionary, as well as the command line options. It yields tuples of three values: `original_path, processed_path, processed`. The path values are strings and `processed` is a boolean indicating whether or not the value was post-processed, or an exception if post-processing failed.

The `~django.contrib.staticfiles.storage.ManifestStaticFilesStorage` uses this behind the scenes to replace the paths with their hashed counterparts and update the cache appropriately.

### `ManifestStaticFilesStorage`

<div class="storage.ManifestStaticFilesStorage">

A subclass of the `~django.contrib.staticfiles.storage.StaticFilesStorage` storage backend which stores the file names it handles by appending the MD5 hash of the file's content to the filename. For example, the file `css/styles.css` would also be saved as `css/styles.55e7cbb9ba48.css`.

</div>

The purpose of this storage is to keep serving the old files in case some pages still refer to those files, e.g. because they are cached by you or a 3rd party proxy server. Additionally, it's very helpful if you want to apply [far future Expires headers](https://developer.yahoo.com/performance/rules.html#expires) to the deployed files to speed up the load time for subsequent page visits.

The storage backend automatically replaces the paths found in the saved files matching other saved files with the path of the cached copy (using the `~django.contrib.staticfiles.storage.StaticFilesStorage.post_process` method). The regular expressions used to find those paths (`django.contrib.staticfiles.storage.HashedFilesMixin.patterns`) cover:

- The [@import](https://www.w3.org/TR/CSS2/cascade.html#at-import) rule and [url()](https://www.w3.org/TR/CSS2/syndata.html#uri) statement of [Cascading Style Sheets](https://www.w3.org/Style/CSS/).
- [Source map](https://firefox-source-docs.mozilla.org/devtools-user/debugger/how_to/use_a_source_map/) comments in CSS and JavaScript files.

Subclass `ManifestStaticFilesStorage` and set the `support_js_module_import_aggregation` attribute to `True`, if you want to use the experimental regular expressions to cover:

- The [modules import](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Modules#importing_features_into_your_script) in JavaScript.
- The [modules aggregation](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Modules#aggregating_modules) in JavaScript.

For example, the `'css/styles.css'` file with this content:

``` css
@import url("../admin/css/base.css");
```

...would be replaced by calling the `~django.core.files.storage.Storage.url` method of the `ManifestStaticFilesStorage` storage backend, ultimately saving a `'css/styles.55e7cbb9ba48.css'` file with the following content:

``` css
@import url("../admin/css/base.27e20196a850.css");
```

<div class="admonition">

Usage of the `integrity` HTML attribute with local files

When using the optional `integrity` attribute within tags like `<script>` or `<link>`, its value should be calculated based on the files as they are served, not as stored in the filesystem. This is particularly important because depending on how static files are collected, their checksum may have changed (for example when using `collectstatic`). At the moment, there is no out-of-the-box tooling available for this.

</div>

You can change the location of the manifest file by using a custom `ManifestStaticFilesStorage` subclass that sets the `manifest_storage` argument. For example:

    from django.conf import settings
    from django.contrib.staticfiles.storage import (
        ManifestStaticFilesStorage,
        StaticFilesStorage,
    )


    class MyManifestStaticFilesStorage(ManifestStaticFilesStorage):
        def __init__(self, *args, **kwargs):
            manifest_storage = StaticFilesStorage(location=settings.BASE_DIR)
            super().__init__(*args, manifest_storage=manifest_storage, **kwargs)

<div class="attribute">

storage.ManifestStaticFilesStorage.manifest_hash

</div>

This attribute provides a single hash that changes whenever a file in the manifest changes. This can be useful to communicate to SPAs that the assets on the server have changed (due to a new deployment).

<div class="attribute">

storage.ManifestStaticFilesStorage.max_post_process_passes

</div>

Since static files might reference other static files that need to have their paths replaced, multiple passes of replacing paths may be needed until the file hashes converge. To prevent an infinite loop due to hashes not converging (for example, if `'foo.css'` references `'bar.css'` which references `'foo.css'`) there is a maximum number of passes before post-processing is abandoned. In cases with a large number of references, a higher number of passes might be needed. Increase the maximum number of passes by subclassing `ManifestStaticFilesStorage` and setting the `max_post_process_passes` attribute. It defaults to 5.

To enable the `ManifestStaticFilesStorage` you have to make sure the following requirements are met:

- the `staticfiles` storage backend in `STORAGES` setting is set to `'django.contrib.staticfiles.storage.ManifestStaticFilesStorage'`
- the `DEBUG` setting is set to `False`
- you've collected all your static files by using the `collectstatic` management command

Since creating the MD5 hash can be a performance burden to your website during runtime, `staticfiles` will automatically store the mapping with hashed names for all processed files in a file called `staticfiles.json`. This happens once when you run the `collectstatic` management command.

<div class="attribute">

storage.ManifestStaticFilesStorage.manifest_strict

</div>

If a file isn't found in the `staticfiles.json` manifest at runtime, a `ValueError` is raised. This behavior can be disabled by subclassing `ManifestStaticFilesStorage` and setting the `manifest_strict` attribute to `False` -- nonexistent paths will remain unchanged.

Due to the requirement of running `collectstatic`, this storage typically shouldn't be used when running tests as `collectstatic` isn't run as part of the normal test setup. During testing, ensure that `staticfiles` storage backend in the `STORAGES` setting is set to something else like `'django.contrib.staticfiles.storage.StaticFilesStorage'` (the default).

<div class="method">

storage.ManifestStaticFilesStorage.file_hash(name, content=None)

</div>

The method that is used when creating the hashed name of a file. Needs to return a hash for the given file name and content. By default it calculates a MD5 hash from the content's chunks as mentioned above. Feel free to override this method to use your own hashing algorithm.

### `ManifestFilesMixin`

<div class="storage.ManifestFilesMixin">

Use this mixin with a custom storage to append the MD5 hash of the file's content to the filename as `~storage.ManifestStaticFilesStorage` does.

</div>

## Finders Module

`staticfiles` finders has a `searched_locations` attribute which is a list of directory paths in which the finders searched. Example usage:

    from django.contrib.staticfiles import finders

    result = finders.find("css/base.css")
    searched_locations = finders.searched_locations

## Other Helpers

There are a few other helpers outside of the `staticfiles <django.contrib.staticfiles>` app to work with static files:

- The `django.template.context_processors.static` context processor which adds `STATIC_URL` to every template context rendered with `~django.template.RequestContext` contexts.
- The builtin template tag `static` which takes a path and urljoins it with the static prefix `STATIC_URL`. If `django.contrib.staticfiles` is installed, the tag uses the `url()` method of the `staticfiles` storage backend from `STORAGES` instead.
- The builtin template tag `get_static_prefix` which populates a template variable with the static prefix `STATIC_URL` to be used as a variable or directly.
- The similar template tag `get_media_prefix` which works like `get_static_prefix` but uses `MEDIA_URL`.
- The `staticfiles` key in `django.core.files.storage.storages` contains a ready-to-use instance of the staticfiles storage backend.

### Static file development view

<div class="currentmodule">

django.contrib.staticfiles

</div>

The static files tools are mostly designed to help with getting static files successfully deployed into production. This usually means a separate, dedicated static file server, which is a lot of overhead to mess with when developing locally. Thus, the `staticfiles` app ships with a **quick and dirty helper view** that you can use to serve files locally in development.

<div class="function">

views.serve(request, path)

</div>

This view function serves static files in development.

<div class="warning">

<div class="title">

Warning

</div>

This view will only work if `DEBUG` is `True`.

That's because this view is **grossly inefficient** and probably **insecure**. This is only intended for local development, and should **never be used in production**.

</div>

<div class="note">

<div class="title">

Note

</div>

To guess the served files' content types, this view relies on the `mimetypes` module from the Python standard library, which itself relies on the underlying platform's map files. If you find that this view doesn't return proper content types for certain files, it is most likely that the platform's map files are incorrect or need to be updated. This can be achieved, for example, by installing or updating the `mailcap` package on a Red Hat distribution, `mime-support` on a Debian distribution, or by editing the keys under `HKEY_CLASSES_ROOT` in the Windows registry.

</div>

This view is automatically enabled by `runserver` (with a `DEBUG` setting set to `True`). To use the view with a different local development server, add the following snippet to the end of your primary URL configuration:

    from django.conf import settings
    from django.contrib.staticfiles import views
    from django.urls import re_path

    if settings.DEBUG:
        urlpatterns += [
            re_path(r"^static/(?P<path>.*)$", views.serve),
        ]

Note, the beginning of the pattern (`r'^static/'`) should be your `STATIC_URL` setting.

Since this is a bit finicky, there's also a helper function that'll do this for you:

<div class="function">

urls.staticfiles_urlpatterns()

</div>

This will return the proper URL pattern for serving static files to your already defined pattern list. Use it like this:

    from django.contrib.staticfiles.urls import staticfiles_urlpatterns

    # ... the rest of your URLconf here ...

    urlpatterns += staticfiles_urlpatterns()

This will inspect your `STATIC_URL` setting and wire up the view to serve static files accordingly. Don't forget to set the `STATICFILES_DIRS` setting appropriately to let `django.contrib.staticfiles` know where to look for files in addition to files in app directories.

<div class="warning">

<div class="title">

Warning

</div>

This helper function will only work if `DEBUG` is `True` and your `STATIC_URL` setting is neither empty nor a full URL such as `http://static.example.com/`.

That's because this view is **grossly inefficient** and probably **insecure**. This is only intended for local development, and should **never be used in production**.

</div>

### Specialized test case to support 'live testing'

<div class="testing.StaticLiveServerTestCase">

This unittest TestCase subclass extends `django.test.LiveServerTestCase`.

</div>

Just like its parent, you can use it to write tests that involve running the code under test and consuming it with testing tools through HTTP (e.g. Selenium, PhantomJS, etc.), because of which it's needed that the static assets are also published.

But given the fact that it makes use of the `django.contrib.staticfiles.views.serve` view described above, it can transparently overlay at test execution-time the assets provided by the `staticfiles` finders. This means you don't need to run `collectstatic` before or as a part of your tests setup.
