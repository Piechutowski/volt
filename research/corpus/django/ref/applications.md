# Applications

<div class="module">

django.apps

</div>

Django contains a registry of installed applications that stores configuration and provides introspection. It also maintains a list of available `models
</topics/db/models>`.

This registry is called `~django.apps.apps` and it's available in `django.apps`:

``` pycon
>>> from django.apps import apps
>>> apps.get_app_config("admin").verbose_name
'Administration'
```

## Projects and applications

The term **project** describes a Django web application. The project Python package is defined primarily by a settings module, but it usually contains other things. For example, when you run `django-admin startproject mysite` you'll get a `mysite` project directory that contains a `mysite` Python package with `settings.py`, `urls.py`, `asgi.py` and `wsgi.py`. The project package is often extended to include things like fixtures, CSS, and templates which aren't tied to a particular application.

A **project's root directory** (the one that contains `manage.py`) is usually the container for all of a project's applications which aren't installed separately.

The term **application** describes a Python package that provides some set of features. Applications `may be reused </intro/reusable-apps/>` in various projects.

Applications include some combination of models, views, templates, template tags, static files, URLs, middleware, etc. They're generally wired into projects with the `INSTALLED_APPS` setting and optionally with other mechanisms such as URLconfs, the `MIDDLEWARE` setting, or template inheritance.

It is important to understand that a Django application is a set of code that interacts with various parts of the framework. There's no such thing as an `Application` object. However, there's a few places where Django needs to interact with installed applications, mainly for configuration and also for introspection. That's why the application registry maintains metadata in an `~django.apps.AppConfig` instance for each installed application.

There's no restriction that a project package can't also be considered an application and have models, etc. (which would require adding it to `INSTALLED_APPS`).

## Configuring applications

To configure an application, create an `apps.py` module inside the application, then define a subclass of `AppConfig` there.

When `INSTALLED_APPS` contains the dotted path to an application module, by default, if Django finds exactly one `AppConfig` subclass in the `apps.py` submodule, it uses that configuration for the application. This behavior may be disabled by setting `AppConfig.default` to `False`.

If the `apps.py` module contains more than one `AppConfig` subclass, Django will look for a single one where `AppConfig.default` is `True`.

If no `AppConfig` subclass is found, the base `AppConfig` class will be used.

Alternatively, `INSTALLED_APPS` may contain the dotted path to a configuration class to specify it explicitly:

    INSTALLED_APPS = [
        ...,
        "polls.apps.PollsAppConfig",
        ...,
    ]

### For application authors

If you're creating a pluggable app called "Rock ’n’ roll", here's how you would provide a proper name for the admin:

    # rock_n_roll/apps.py

    from django.apps import AppConfig


    class RockNRollConfig(AppConfig):
        name = "rock_n_roll"
        verbose_name = "Rock ’n’ roll"

`RockNRollConfig` will be loaded automatically when `INSTALLED_APPS` contains `'rock_n_roll'`. If you need to prevent this, set `~AppConfig.default` to `False` in the class definition.

You can provide several `AppConfig` subclasses with different behaviors. To tell Django which one to use by default, set `~AppConfig.default` to `True` in its definition. If your users want to pick a non-default configuration, they must replace `'rock_n_roll'` with the dotted path to that specific class in their `INSTALLED_APPS` setting.

The `AppConfig.name` attribute tells Django which application this configuration applies to. You can define any other attribute documented in the `~django.apps.AppConfig` API reference.

`AppConfig` subclasses may be defined anywhere. The `apps.py` convention merely allows Django to load them automatically when `INSTALLED_APPS` contains the path to an application module rather than the path to a configuration class.

<div class="note">

<div class="title">

Note

</div>

If your code imports the application registry in an application's `__init__.py`, the name `apps` will clash with the `apps` submodule. The best practice is to move that code to a submodule and import it. A workaround is to import the registry under a different name:

    from django.apps import apps as django_apps

</div>

### For application users

If you're using "Rock ’n’ roll" in a project called `anthology`, but you want it to show up as "Jazz Manouche" instead, you can provide your own configuration:

    # anthology/apps.py

    from rock_n_roll.apps import RockNRollConfig


    class JazzManoucheConfig(RockNRollConfig):
        verbose_name = "Jazz Manouche"


    # anthology/settings.py

    INSTALLED_APPS = [
        "anthology.apps.JazzManoucheConfig",
        # ...
    ]

This example shows project-specific configuration classes located in a submodule called `apps.py`. This is a convention, not a requirement. `AppConfig` subclasses may be defined anywhere.

In this situation, `INSTALLED_APPS` must contain the dotted path to the configuration class because it lives outside of an application and thus cannot be automatically detected.

## Application configuration

<div class="AppConfig">

Application configuration objects store metadata for an application. Some attributes can be configured in `~django.apps.AppConfig` subclasses. Others are set by Django and read-only.

</div>

### Configurable attributes

<div class="attribute">

AppConfig.name

Full Python path to the application, e.g. `'django.contrib.admin'`.

This attribute defines which application the configuration applies to. It must be set in all `~django.apps.AppConfig` subclasses.

It must be unique across a Django project.

</div>

<div class="attribute">

AppConfig.label

Short name for the application, e.g. `'admin'`

This attribute allows relabeling an application when two applications have conflicting labels. It defaults to the last component of `name`. It should be a valid Python identifier.

It must be unique across a Django project.

<div class="warning">

<div class="title">

Warning

</div>

Changing this attribute after migrations have been applied for an application will result in breaking changes to a project or, in the case of a reusable app, any existing installs of that app. This is because `AppConfig.label` is used in database tables and migration files when referencing an app in the dependencies list.

</div>

</div>

<div class="attribute">

AppConfig.verbose_name

Human-readable name for the application, e.g. "Administration".

This attribute defaults to `label.title()`.

</div>

<div class="attribute">

AppConfig.path

Filesystem path to the application directory, e.g. `'/usr/lib/pythonX.Y/dist-packages/django/contrib/admin'`.

In most cases, Django can automatically detect and set this, but you can also provide an explicit override as a class attribute on your `~django.apps.AppConfig` subclass. In a few situations this is required; for instance if the app package is a [namespace package](#namespace package) with multiple paths.

</div>

<div class="attribute">

AppConfig.default

Set this attribute to `False` to prevent Django from selecting a configuration class automatically. This is useful when `apps.py` defines only one `AppConfig` subclass but you don't want Django to use it by default.

Set this attribute to `True` to tell Django to select a configuration class automatically. This is useful when `apps.py` defines more than one `AppConfig` subclass and you want Django to use one of them by default.

By default, this attribute isn't set.

</div>

<div class="attribute">

AppConfig.default_auto_field

The implicit primary key type to add to models within this app.

Reusable applications that define models **must** set this attribute to ensure migrations included in the application match the models. Otherwise, migrations will be generated for the application when a user runs `makemigrations` with a different `DEFAULT_AUTO_FIELD`, which may conflict with future migrations shipped by the application.

For example, to keep using `~django.db.models.AutoField`:

    default_auto_field = "django.db.models.AutoField"

By default, this is the value of `DEFAULT_AUTO_FIELD`.

</div>

### Read-only attributes

<div class="attribute">

AppConfig.module

Root module for the application, e.g. `<module 'django.contrib.admin' from 'django/contrib/admin/__init__.py'>`.

</div>

<div class="attribute">

AppConfig.models_module

Module containing the models, e.g. `<module 'django.contrib.admin.models' from 'django/contrib/admin/models.py'>`.

It may be `None` if the application doesn't contain a `models` module. Note that the database related signals such as `~django.db.models.signals.pre_migrate` and `~django.db.models.signals.post_migrate` are only emitted for applications that have a `models` module.

</div>

### Methods

<div class="method">

AppConfig.get_models(include_auto_created=False, include_swapped=False)

Returns an iterable of `~django.db.models.Model` classes for this application.

Requires the app registry to be fully populated.

</div>

<div class="method">

AppConfig.get_model(model_name, require_ready=True)

Returns the `~django.db.models.Model` with the given `model_name`. `model_name` is case-insensitive.

Raises `LookupError` if no such model exists in this application.

Requires the app registry to be fully populated unless the `require_ready` argument is set to `False`. `require_ready` behaves exactly as in `apps.get_model`.

</div>

<div class="method">

AppConfig.ready()

Subclasses can override this method to perform initialization tasks such as registering signals. It is called as soon as the registry is fully populated.

Although you can't import models at the module-level where `~django.apps.AppConfig` classes are defined, you can import them in `ready()`, using either an `import` statement or `~AppConfig.get_model`.

If you're registering `model signals <django.db.models.signals>`, you can refer to the sender by its string label instead of using the model class itself.

Example:

    from django.apps import AppConfig
    from django.db.models.signals import pre_save


    class RockNRollConfig(AppConfig):
        # ...

        def ready(self):
            # importing model classes
            from .models import MyModel  # or...

            MyModel = self.get_model("MyModel")

            # registering signals with the model's string label
            pre_save.connect(receiver, sender="app_label.MyModel")

<div class="warning">

<div class="title">

Warning

</div>

Although you can access model classes as described above, avoid interacting with the database in your `ready` implementation. This includes model methods that execute queries (`~django.db.models.Model.save`, `~django.db.models.Model.delete`, manager methods etc.), and also raw SQL queries via `django.db.connection`. Your `ready` method will run during startup of every management command. For example, even though the test database configuration is separate from the production settings, `manage.py test` would still execute some queries against your **production** database!

</div>

<div class="note">

<div class="title">

Note

</div>

In the usual initialization process, the `ready` method is only called once by Django. But in some corner cases, particularly in tests which are fiddling with installed applications, `ready` might be called more than once. In that case, either write idempotent methods, or put a flag on your `AppConfig` classes to prevent rerunning code which should be executed exactly one time.

</div>

</div>

### Namespace packages as apps

Python packages without an `__init__.py` file are known as "namespace packages" and may be spread across multiple directories at different locations on `sys.path` (see `420`).

Django applications require a single base filesystem path where Django (depending on configuration) will search for templates, static assets, etc. Thus, namespace packages may only be Django applications if one of the following is true:

1.  The namespace package actually has only a single location (i.e. is not spread across more than one directory.)
2.  The `~django.apps.AppConfig` class used to configure the application has a `~django.apps.AppConfig.path` class attribute, which is the absolute directory path Django will use as the single base path for the application.

If neither of these conditions is met, Django will raise `~django.core.exceptions.ImproperlyConfigured`.

## Application registry

<div class="data">

apps

The application registry provides the following public API. Methods that aren't listed below are considered private and may change without notice.

</div>

<div class="attribute">

apps.ready

Boolean attribute that is set to `True` after the registry is fully populated and all `AppConfig.ready` methods are called.

</div>

<div class="method">

apps.get_app_configs()

Returns an iterable of `~django.apps.AppConfig` instances.

</div>

<div class="method">

apps.get_app_config(app_label)

Returns an `~django.apps.AppConfig` for the application with the given `app_label`. Raises `LookupError` if no such application exists.

</div>

<div class="method">

apps.is_installed(app_name)

Checks whether an application with the given name exists in the registry. `app_name` is the full name of the app, e.g. `'django.contrib.admin'`.

</div>

<div class="method">

apps.get_model(app_label, model_name, require_ready=True)

Returns the `~django.db.models.Model` with the given `app_label` and `model_name`. As a shortcut, this method also accepts a single argument in the form `app_label.model_name`. `model_name` is case-insensitive.

Raises `LookupError` if no such application or model exists. Raises `ValueError` when called with a single argument that doesn't contain exactly one dot.

Requires the app registry to be fully populated unless the `require_ready` argument is set to `False`.

Setting `require_ready` to `False` allows looking up models `while the app registry is being populated <app-loading-process>`, specifically during the second phase where it imports models. Then `get_model()` has the same effect as importing the model. The main use case is to configure model classes with settings, such as `AUTH_USER_MODEL`.

When `require_ready` is `False`, `get_model()` returns a model class that may not be fully functional (reverse accessors may be missing, for example) until the app registry is fully populated. For this reason, it's best to leave `require_ready` to the default value of `True` whenever possible.

</div>

## Initialization process

### How applications are loaded

When Django starts, `django.setup` is responsible for populating the application registry.

<div class="currentmodule">

django

</div>

<div class="function">

setup(set_prefix=True)

Configures Django by:

- Loading the settings.
- Setting up logging.
- If `set_prefix` is True, setting the URL resolver script prefix to `FORCE_SCRIPT_NAME` if defined, or `/` otherwise.
- Initializing the application registry.

This function is called automatically:

- When running an HTTP server via Django's ASGI or WSGI support.
- When invoking a management command.

It must be called explicitly in other cases, for instance in plain Python scripts.

</div>

<div class="currentmodule">

django.apps

</div>

The application registry is initialized in three stages. At each stage, Django processes all applications in the order of `INSTALLED_APPS`.

1.  First Django imports each item in `INSTALLED_APPS`.

    If it's an application configuration class, Django imports the root package of the application, defined by its `~AppConfig.name` attribute. If it's a Python package, Django looks for an application configuration in an `apps.py` submodule, or else creates a default application configuration.

    *At this stage, your code shouldn't import any models!*

    In other words, your applications' root packages and the modules that define your application configuration classes shouldn't import any models, even indirectly.

    Strictly speaking, Django allows importing models once their application configuration is loaded. However, in order to avoid needless constraints on the order of `INSTALLED_APPS`, it's strongly recommended not import any models at this stage.

    Once this stage completes, APIs that operate on application configurations such as `~apps.get_app_config` become usable.

2.  Then Django attempts to import the `models` submodule of each application, if there is one.

    You must define or import all models in your application's `models.py` or `models/__init__.py`. Otherwise, the application registry may not be fully populated at this point, which could cause the ORM to malfunction.

    Once this stage completes, APIs that operate on models such as `~apps.get_model` become usable.

3.  Finally Django runs the `~AppConfig.ready` method of each application configuration.

### Troubleshooting

Here are some common problems that you may encounter during initialization:

- `~django.core.exceptions.AppRegistryNotReady`: This happens when importing an application configuration or a models module triggers code that depends on the app registry.

  For example, `~django.utils.translation.gettext` uses the app registry to look up translation catalogs in applications. To translate at import time, you need `~django.utils.translation.gettext_lazy` instead. (Using `~django.utils.translation.gettext` would be a bug, because the translation would happen at import time, rather than at each request depending on the active language.)

  Executing database queries with the ORM at import time in models modules will also trigger this exception. The ORM cannot function properly until all models are available.

  This exception also happens if you forget to call `django.setup` in a standalone Python script.

- `ImportError: cannot import name ...` This happens if the import sequence ends up in a loop.

  To eliminate such problems, you should minimize dependencies between your models modules and do as little work as possible at import time. To avoid executing code at import time, you can move it into a function and cache its results. The code will be executed when you first need its results. This concept is known as "lazy evaluation".

- `django.contrib.admin` automatically performs autodiscovery of `admin` modules in installed applications. To prevent it, change your `INSTALLED_APPS` to contain `'django.contrib.admin.apps.SimpleAdminConfig'` instead of `'django.contrib.admin'`.

- `RuntimeWarning: Accessing the database during app initialization is discouraged.` This warning is triggered for database queries executed before apps are ready, such as during module imports or in the `AppConfig.ready` method. Such premature database queries are discouraged because they will run during the startup of every management command, which will slow down your project startup, potentially cache stale data, and can even fail if migrations are pending.

  For example, a common mistake is making a database query to populate form field choices:

      class LocationForm(forms.Form):
          country = forms.ChoiceField(choices=[c.name for c in Country.objects.all()])

  In the example above, the query from `Country.objects.all()` is executed during module import, because the `QuerySet` is iterated over. To avoid the warning, the form could use a `~django.forms.ModelChoiceField` instead:

      class LocationForm(forms.Form):
          country = forms.ModelChoiceField(queryset=Country.objects.all())

  To make it easier to find the code that triggered this warning, you can make Python `treat warnings as errors <python:warning-filter>` to reveal the stack trace, for example with `python -Werror manage.py shell`.
