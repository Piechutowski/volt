# Signals

A list of all the signals that Django sends. All built-in signals are sent using the `~django.dispatch.Signal.send` method.

<div class="seealso">

See the documentation on the `signal dispatcher </topics/signals>` for information regarding how to register for and receive signals.

The `authentication framework </topics/auth/index>` sends `signals when a user is logged in / out <topics-auth-signals>`.

</div>

## Model signals

<div class="module" synopsis="Signals sent by the model system.">

django.db.models.signals

</div>

The `django.db.models.signals` module defines a set of signals sent by the model system.

<div class="warning">

<div class="title">

Warning

</div>

Signals can make your code harder to maintain. Consider implementing a helper method on a `custom manager <custom-managers>`, to both update your models and perform additional logic, or else `overriding model methods <overriding-model-methods>` before using model signals.

</div>

<div class="warning">

<div class="title">

Warning

</div>

Many of these signals are sent by various model methods like `__init__()` or `~django.db.models.Model.save` that you can override in your own code.

If you override these methods on your model, you must call the parent class' methods for these signals to be sent.

Note also that Django stores signal handlers as weak references by default, so if your handler is a local function, it may be garbage collected. To prevent this, pass `weak=False` when you call the signal's `~django.dispatch.Signal.connect`.

</div>

<div class="note">

<div class="title">

Note

</div>

Model signals `sender` model can be lazily referenced when connecting a receiver by specifying its full application label. For example, an `Question` model defined in the `polls` application could be referenced as `'polls.Question'`. This sort of reference can be quite handy when dealing with circular import dependencies and swappable models.

</div>

### `pre_init`

<div class="attribute" module="">

django.db.models.signals.pre_init

</div>

Whenever you instantiate a Django model, this signal is sent at the beginning of the model's `__init__()` method.

Arguments sent with this signal:

`sender`  
The model class that just had an instance created.

`args`  
A list of positional arguments passed to `__init__()`.

`kwargs`  
A dictionary of keyword arguments passed to `__init__()`.

For example, the `tutorial </intro/tutorial02>` has this line:

    q = Question(question_text="What's new?", pub_date=timezone.now())

The arguments sent to a `pre_init` handler would be:

| Argument | Value                                                                                                                   |
|----------|-------------------------------------------------------------------------------------------------------------------------|
| `sender` | `Question` (the class itself)                                                                                           |
| `args`   | `[]` (an empty list because there were no positional arguments passed to `__init__()`)                                  |
| `kwargs` | `{'question_text': "What's new?",` `'pub_date': datetime.datetime(2012, 2, 26, 13, 0, 0, 775217, tzinfo=datetime.UTC)}` |

### `post_init`

<div class="data" module="">

django.db.models.signals.post_init

</div>

Like pre_init, but this one is sent when the `__init__()` method finishes.

Arguments sent with this signal:

`sender`  
As above: the model class that just had an instance created.

`instance`  
The actual instance of the model that's just been created.

<div class="note">

<div class="title">

Note

</div>

`instance._state <django.db.models.Model._state>` isn't set before sending the `post_init` signal, so `_state` attributes always have their default values. For example, `_state.db` is `None`.

</div>

<div class="warning">

<div class="title">

Warning

</div>

For performance reasons, you shouldn't perform queries in receivers of `pre_init` or `post_init` signals because they would be executed for each instance returned during queryset iteration.

</div>

### `pre_save`

<div class="data" module="">

django.db.models.signals.pre_save

</div>

This is sent at the beginning of a model's `~django.db.models.Model.save` method.

Arguments sent with this signal:

`sender`  
The model class.

`instance`  
The actual instance being saved.

`raw`  
A boolean; `True` if the model is saved exactly as presented (i.e. when loading a `fixture <fixtures-explanation>`). One should not query/modify other records in the database as the database might not be in a consistent state yet.

`using`  
The database alias being used.

`update_fields`  
The set of fields to update as passed to `.Model.save`, or `None` if `update_fields` wasn't passed to `save()`.

### `post_save`

<div class="data" module="">

django.db.models.signals.post_save

</div>

Like `pre_save`, but sent at the end of the `~django.db.models.Model.save` method.

Arguments sent with this signal:

`sender`  
The model class.

`instance`  
The actual instance being saved.

`created`  
A boolean; `True` if a new record was created.

`raw`  
A boolean; `True` if the model is saved exactly as presented (i.e. when loading a `fixture <fixtures-explanation>`). One should not query/modify other records in the database as the database might not be in a consistent state yet.

`using`  
The database alias being used.

`update_fields`  
The set of fields to update as passed to `.Model.save`, or `None` if `update_fields` wasn't passed to `save()`.

### `pre_delete`

<div class="data" module="">

django.db.models.signals.pre_delete

</div>

Sent at the beginning of a model's `~django.db.models.Model.delete` method and a queryset's `~django.db.models.query.QuerySet.delete` method.

Arguments sent with this signal:

`sender`  
The model class.

`instance`  
The actual instance being deleted.

`using`  
The database alias being used.

`origin`  
The `Model` or `QuerySet` instance from which the deletion originated, that is, the instance whose `delete()` method was invoked.

### `post_delete`

<div class="data" module="">

django.db.models.signals.post_delete

</div>

Like `pre_delete`, but sent at the end of a model's `~django.db.models.Model.delete` method and a queryset's `~django.db.models.query.QuerySet.delete` method.

Arguments sent with this signal:

`sender`  
The model class.

`instance`  
The actual instance being deleted.

Note that the object will no longer be in the database, so be very careful what you do with this instance.

`using`  
The database alias being used.

`origin`  
The `Model` or `QuerySet` instance from which the deletion originated, that is, the instance whose `delete()` method was invoked.

### `m2m_changed`

<div class="data" module="">

django.db.models.signals.m2m_changed

</div>

Sent when a `~django.db.models.ManyToManyField` is changed on a model instance. Strictly speaking, this is not a model signal since it is sent by the `~django.db.models.ManyToManyField`, but since it complements the `pre_save`/`post_save` and `pre_delete`/`post_delete` when it comes to tracking changes to models, it is included here.

Arguments sent with this signal:

`sender`  
The intermediate model class describing the `~django.db.models.ManyToManyField`. This class is automatically created when a many-to-many field is defined; you can access it using the `through` attribute on the many-to-many field.

`instance`  
The instance whose many-to-many relation is updated. This can be an instance of the `sender`, or of the class the `~django.db.models.ManyToManyField` is related to.

`action`  
A string indicating the type of update that is done on the relation. This can be one of the following:

`"pre_add"`  
Sent *before* one or more objects are added to the relation.

`"post_add"`  
Sent *after* one or more objects are added to the relation.

`"pre_remove"`  
Sent *before* one or more objects are removed from the relation.

`"post_remove"`  
Sent *after* one or more objects are removed from the relation.

`"pre_clear"`  
Sent *before* the relation is cleared.

`"post_clear"`  
Sent *after* the relation is cleared.

`reverse`  
Indicates which side of the relation is updated (i.e., if it is the forward or reverse relation that is being modified).

`model`  
The class of the objects that are added to, removed from or cleared from the relation.

`pk_set`  
For the `pre_add` and `post_add` actions, this is a set of primary key values that will be, or have been, added to the relation. This may be a subset of the values submitted to be added, since inserts must filter existing values in order to avoid a database `IntegrityError`.

For the `pre_remove` and `post_remove` actions, this is a set of primary key values that was submitted to be removed from the relation. This is not dependent on whether the values actually will be, or have been, removed. In particular, non-existent values may be submitted, and will appear in `pk_set`, even though they have no effect on the database.

For the `pre_clear` and `post_clear` actions, this is `None`.

`using`  
The database alias being used.

`raw`

> <div class="versionadded">
>
> 6.1
>
> </div>
>
> A boolean; `True` if the model is saved exactly as presented (i.e. when loading a `fixture <fixtures-explanation>`). One should not query/modify other records in the database as the database might not be in a consistent state yet.

For example, if a `Pizza` can have multiple `Topping` objects, modeled like this:

    class Topping(models.Model):
        # ...
        pass


    class Pizza(models.Model):
        # ...
        toppings = models.ManyToManyField(Topping)

If we connected a handler like this:

    from django.db.models.signals import m2m_changed


    def toppings_changed(sender, **kwargs):
        # Do something
        pass


    m2m_changed.connect(toppings_changed, sender=Pizza.toppings.through)

and then did something like this:

``` pycon
>>> p = Pizza.objects.create(...)
>>> t = Topping.objects.create(...)
>>> p.toppings.add(t)
```

the arguments sent to a `m2m_changed` handler (`toppings_changed` in the example above) would be:

| Argument   | Value                                                                                                          |
|------------|----------------------------------------------------------------------------------------------------------------|
| `sender`   | `Pizza.toppings.through` (the intermediate m2m class)                                                          |
| `instance` | `p` (the `Pizza` instance being modified)                                                                      |
| `action`   | `"pre_add"` (followed by a separate signal with `"post_add"`)                                                  |
| `reverse`  | `False` (`Pizza` contains the `~django.db.models.ManyToManyField`, so this call modifies the forward relation) |
| `model`    | `Topping` (the class of the objects added to the `Pizza`)                                                      |
| `pk_set`   | `{t.id}` (since only `Topping t` was added to the relation)                                                    |
| `using`    | `"default"` (since the default router sends writes here)                                                       |
| `raw`      | `False` (since it is not loaded from a fixture)                                                                |

And if we would then do something like this:

``` pycon
>>> t.pizza_set.remove(p)
```

the arguments sent to a `m2m_changed` handler would be:

| Argument   | Value                                                                                                         |
|------------|---------------------------------------------------------------------------------------------------------------|
| `sender`   | `Pizza.toppings.through` (the intermediate m2m class)                                                         |
| `instance` | `t` (the `Topping` instance being modified)                                                                   |
| `action`   | `"pre_remove"` (followed by a separate signal with `"post_remove"`)                                           |
| `reverse`  | `True` (`Pizza` contains the `~django.db.models.ManyToManyField`, so this call modifies the reverse relation) |
| `model`    | `Pizza` (the class of the objects removed from the `Topping`)                                                 |
| `pk_set`   | `{p.id}` (since only `Pizza p` was removed from the relation)                                                 |
| `using`    | `"default"` (since the default router sends writes here)                                                      |
| `raw`      | `False` (since it is not loaded from a fixture)                                                               |

### `class_prepared`

<div class="data" module="">

django.db.models.signals.class_prepared

</div>

Sent whenever a model class has been "prepared" -- that is, once a model has been defined and registered with Django's model system. Django uses this signal internally; it's not generally used in third-party applications.

Since this signal is sent during the app registry population process, and `AppConfig.ready() <django.apps.AppConfig.ready>` runs after the app registry is fully populated, receivers cannot be connected in that method. One possibility is to connect them `AppConfig.__init__()` instead, taking care not to import models or trigger calls to the app registry.

Arguments that are sent with this signal:

`sender`  
The model class which was just prepared.

## Management signals

Signals sent by `django-admin </ref/django-admin>`.

### `pre_migrate`

<div class="data" module="">

django.db.models.signals.pre_migrate

</div>

Sent by the `migrate` command before it starts to install an application. It's not emitted for applications that lack a `models` module.

Arguments sent with this signal:

`sender`  
An `~django.apps.AppConfig` instance for the application about to be migrated/synced.

`app_config`  
Same as `sender`.

`verbosity`  
Indicates how much information `manage.py` is printing on screen. See the `--verbosity` flag for details.

Functions which listen for `pre_migrate` should adjust what they output to the screen based on the value of this argument.

`interactive`  
If `interactive` is `True`, it's safe to prompt the user to input things on the command line. If `interactive` is `False`, functions which listen for this signal should not try to prompt for anything.

For example, the `django.contrib.auth` app only prompts to create a superuser when `interactive` is `True`.

`stdout`  
A stream-like object where verbose output should be redirected.

`using`  
The alias of database on which a command will operate.

`plan`  
The migration plan that is going to be used for the migration run. While the plan is not public API, this allows for the rare cases when it is necessary to know the plan. A plan is a list of 2-tuples with the first item being the instance of a migration class and the second item showing if the migration was rolled back (`True`) or applied (`False`).

`apps`  
An instance of `Apps <django.apps>` containing the state of the project before the migration run. It should be used instead of the global `apps <django.apps.apps>` registry to retrieve the models you want to perform operations on.

### `post_migrate`

<div class="data" module="">

django.db.models.signals.post_migrate

</div>

Sent at the end of the `migrate` (even if no migrations are run) and `flush` commands. It's not emitted for applications that lack a `models` module.

Handlers of this signal must not perform database schema alterations as doing so may cause the `flush` command to fail if it runs during the `migrate` command.

Arguments sent with this signal:

`sender`  
An `~django.apps.AppConfig` instance for the application that was just installed.

`app_config`  
Same as `sender`.

`verbosity`  
Indicates how much information `manage.py` is printing on screen. See the `--verbosity` flag for details.

Functions which listen for `post_migrate` should adjust what they output to the screen based on the value of this argument.

`interactive`  
If `interactive` is `True`, it's safe to prompt the user to input things on the command line. If `interactive` is `False`, functions which listen for this signal should not try to prompt for anything.

For example, the `django.contrib.auth` app only prompts to create a superuser when `interactive` is `True`.

`stdout`  
A stream-like object where verbose output should be redirected.

`using`  
The database alias used for synchronization. Defaults to the `default` database.

`plan`  
The migration plan that was used for the migration run. While the plan is not public API, this allows for the rare cases when it is necessary to know the plan. A plan is a list of 2-tuples with the first item being the instance of a migration class and the second item showing if the migration was rolled back (`True`) or applied (`False`).

`apps`  
An instance of `Apps <django.apps.apps>` containing the state of the project after the migration run. It should be used instead of the global `apps <django.apps.apps>` registry to retrieve the models you want to perform operations on.

For example, you could register a callback in an `~django.apps.AppConfig` like this:

    from django.apps import AppConfig
    from django.db.models.signals import post_migrate


    def my_callback(sender, **kwargs):
        # Your specific logic here
        pass


    class MyAppConfig(AppConfig):
        ...

        def ready(self):
            post_migrate.connect(my_callback, sender=self)

<div class="note">

<div class="title">

Note

</div>

If you provide an `~django.apps.AppConfig` instance as the sender argument, please ensure that the signal is registered in `~django.apps.AppConfig.ready`. `AppConfig`s are recreated for tests that run with a modified set of `INSTALLED_APPS` (such as when settings are overridden) and such signals should be connected for each new `AppConfig` instance.

</div>

## Request/response signals

<div class="module" synopsis="Core signals sent by the request/response system.">

django.core.signals

</div>

Signals sent by the core framework when processing a request.

<div class="warning">

<div class="title">

Warning

</div>

Signals can make your code harder to maintain. Consider `using a
middleware </topics/http/middleware>` before using request/response signals.

</div>

### `request_started`

<div class="data" module="">

django.core.signals.request_started

</div>

Sent when Django begins processing an HTTP request.

Arguments sent with this signal:

`sender`  
The handler class -- e.g. `django.core.handlers.wsgi.WsgiHandler` -- that handled the request.

`environ`  
The `environ` dictionary provided to the request.

### `request_finished`

<div class="data" module="">

django.core.signals.request_finished

</div>

Sent when Django finishes delivering an HTTP response to the client.

Arguments sent with this signal:

`sender`  
The handler class, as above.

### `got_request_exception`

<div class="data" module="">

django.core.signals.got_request_exception

</div>

This signal is sent whenever Django encounters an exception while processing an incoming HTTP request.

Arguments sent with this signal:

`sender`  
Unused (always `None`).

`request`  
The `~django.http.HttpRequest` object.

## Test signals

<div class="module" synopsis="Signals sent during testing.">

django.test.signals

</div>

Signals only sent when `running tests <running-tests>`.

### `setting_changed`

<div class="data" module="">

django.test.signals.setting_changed

</div>

This signal is sent when the value of a setting is changed through the `django.test.TestCase.settings()` context manager or the `django.test.override_settings` decorator/context manager.

It's actually sent twice: when the new value is applied ("setup") and when the original value is restored ("teardown"). Use the `enter` argument to distinguish between the two.

You can also import this signal from `django.core.signals` to avoid importing from `django.test` in non-test situations.

Arguments sent with this signal:

`sender`  
The settings handler.

`setting`  
The name of the setting.

`value`  
The value of the setting after the change. For settings that initially don't exist, in the "teardown" phase, `value` is `None`.

`enter`  
A boolean; `True` if the setting is applied, `False` if restored.

### `template_rendered`

<div class="data" module="">

django.test.signals.template_rendered

</div>

Sent when the test system renders a template. This signal is not emitted during normal operation of a Django server -- it is only available during testing.

Arguments sent with this signal:

`sender`  
The `~django.template.Template` object which was rendered.

`template`  
Same as sender

`context`  
The `~django.template.Context` with which the template was rendered.

## Database Wrappers

<div class="module" synopsis="Core signals sent by the database wrapper.">

django.db.backends

</div>

Signals sent by the database wrapper when a database connection is initiated.

### `connection_created`

<div class="data" module="">

django.db.backends.signals.connection_created

</div>

Sent when the database wrapper makes the initial connection to the database. This is particularly useful if you'd like to send any post connection commands to the SQL backend.

Arguments sent with this signal:

`sender`  
The database wrapper class -- i.e. `django.db.backends.postgresql.DatabaseWrapper` or `django.db.backends.mysql.DatabaseWrapper`, etc.

`connection`  
The database connection that was opened. This can be used in a multiple-database configuration to differentiate connection signals from different databases.

## Tasks signals

<div class="versionadded">

6.0

</div>

Signals sent by the `tasks </ref/tasks>` framework.

### `task_enqueued`

<div class="data" module="">

django.tasks.signals.task_enqueued

</div>

Sent once a Task has been enqueued.

Arguments sent with this signal:

`sender`  
The backend class which the Task was enqueued on to.

`task_result`  
The enqueued `TaskResult <django.tasks.TaskResult>`.

### `task_started`

<div class="data" module="">

django.tasks.signals.task_started

</div>

Sent when a Task has started executing.

Arguments sent with this signal:

`sender`  
The backend class which the Task was enqueued on to.

`task_result`  
The started `TaskResult <django.tasks.TaskResult>`.

### `task_finished`

<div class="data" module="">

django.tasks.signals.task_finished

</div>

Sent once a Task has finished executing, successfully or otherwise.

Arguments sent with this signal:

`sender`  
The backend class which the Task was enqueued on to.

`task_result`  
The finished `TaskResult <django.tasks.TaskResult>`.
