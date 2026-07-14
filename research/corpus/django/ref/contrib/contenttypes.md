# The contenttypes framework

<div class="module" synopsis="Provides generic interface to installed models.">

django.contrib.contenttypes

</div>

Django includes a `~django.contrib.contenttypes` application that can track all of the models installed in your Django-powered project, providing a high-level, generic interface for working with your models.

## Overview

At the heart of the contenttypes application is the `~django.contrib.contenttypes.models.ContentType` model, which lives at `django.contrib.contenttypes.models.ContentType`. Instances of `~django.contrib.contenttypes.models.ContentType` represent and store information about the models installed in your project, and new instances of `~django.contrib.contenttypes.models.ContentType` are automatically created whenever new models are installed.

Instances of `~django.contrib.contenttypes.models.ContentType` have methods for returning the model classes they represent and for querying objects from those models. `~django.contrib.contenttypes.models.ContentType` also has a `custom manager <custom-managers>` that adds methods for working with `~django.contrib.contenttypes.models.ContentType` and for obtaining instances of `~django.contrib.contenttypes.models.ContentType` for a particular model.

Relations between your models and `~django.contrib.contenttypes.models.ContentType` can also be used to enable "generic" relationships between an instance of one of your models and instances of any model you have installed.

## Installing the contenttypes framework

The contenttypes framework is included in the default `INSTALLED_APPS` list created by `django-admin startproject`, but if you've removed it or if you manually set up your `INSTALLED_APPS` list, you can enable it by adding `'django.contrib.contenttypes'` to your `INSTALLED_APPS` setting.

It's generally a good idea to have the contenttypes framework installed; several of Django's other bundled applications require it:

- The admin application uses it to log the history of each object added or changed through the admin interface.
- Django's `authentication framework <django.contrib.auth>` uses it to tie user permissions to specific models.

<div class="currentmodule">

django.contrib.contenttypes.models

</div>

## The `ContentType` model

<div class="ContentType">

Each instance of `~django.contrib.contenttypes.models.ContentType` has two fields which, taken together, uniquely describe an installed model:

<div class="attribute">

app_label

The name of the application the model is part of. This is taken from the `app_label` attribute of the model, and includes only the *last* part of the application's Python import path; `django.contrib.contenttypes`, for example, becomes an `app_label` of `contenttypes`.

</div>

<div class="attribute">

model

The name of the model class.

</div>

Additionally, the following property is available:

<div class="attribute">

name

The human-readable name of the content type. This is taken from the `verbose_name <django.db.models.Field.verbose_name>` attribute of the model.

</div>

</div>

Let's look at an example to see how this works. If you already have the `~django.contrib.contenttypes` application installed, and then add `the sites application <django.contrib.sites>` to your `INSTALLED_APPS` setting and run `manage.py migrate` to install it, the model `django.contrib.sites.models.Site` will be installed into your database. Along with it a new instance of `~django.contrib.contenttypes.models.ContentType` will be created with the following values:

- `~django.contrib.contenttypes.models.ContentType.app_label` will be set to `'sites'` (the last part of the Python path `django.contrib.sites`).
- `~django.contrib.contenttypes.models.ContentType.model` will be set to `'site'`.

## Methods on `ContentType` instances

Each `~django.contrib.contenttypes.models.ContentType` instance has methods that allow you to get from a `~django.contrib.contenttypes.models.ContentType` instance to the model it represents, or to retrieve objects from that model:

<div class="method">

ContentType.get_object_for_this_type(using=None, \*\*kwargs)

Takes a set of valid `lookup arguments <field-lookups-intro>` for the model the `~django.contrib.contenttypes.models.ContentType` represents, and does a `~django.db.models.query.QuerySet.get` lookup on that model, returning the corresponding object. The `using` argument can be used to specify a different database than the default one.

</div>

<div class="method">

ContentType.model_class()

Returns the model class represented by this `~django.contrib.contenttypes.models.ContentType` instance.

</div>

For example, we could look up the `~django.contrib.contenttypes.models.ContentType` for the `~django.contrib.auth.models.User` model:

``` pycon
>>> from django.contrib.contenttypes.models import ContentType
>>> user_type = ContentType.objects.get(app_label="auth", model="user")
>>> user_type
<ContentType: user>
```

And then use it to query for a particular `~django.contrib.auth.models.User`, or to get access to the `User` model class:

``` pycon
>>> user_type.model_class()
<class 'django.contrib.auth.models.User'>
>>> user_type.get_object_for_this_type(username="Guido")
<User: Guido>
```

Together, `~django.contrib.contenttypes.models.ContentType.get_object_for_this_type` and `~django.contrib.contenttypes.models.ContentType.model_class` enable two extremely important use cases:

1.  Using these methods, you can write high-level generic code that performs queries on any installed model -- instead of importing and using a single specific model class, you can pass an `app_label` and `model` into a `~django.contrib.contenttypes.models.ContentType` lookup at runtime, and then work with the model class or retrieve objects from it.
2.  You can relate another model to `~django.contrib.contenttypes.models.ContentType` as a way of tying instances of it to particular model classes, and use these methods to get access to those model classes.

Several of Django's bundled applications make use of the latter technique. For example, `the permissions system <topic-authorization>` in Django's authentication framework uses a `~django.contrib.auth.models.Permission` model with a foreign key to `~django.contrib.contenttypes.models.ContentType`; this lets `~django.contrib.auth.models.Permission` represent concepts like "can add blog entry" or "can delete news story".

### The `ContentTypeManager`

<div class="ContentTypeManager">

`~django.contrib.contenttypes.models.ContentType` also has a custom manager, `~django.contrib.contenttypes.models.ContentTypeManager`, which adds the following methods:

<div class="method">

clear_cache()

Clears an internal cache used by `~django.contrib.contenttypes.models.ContentType` to keep track of models for which it has created `~django.contrib.contenttypes.models.ContentType` instances. You probably won't need to call this method in application code yourself; Django will call it automatically when it's needed.

You may need to clear the cache when testing, to reset between tests, or after preparing test state. For example:

    class ContentTypesTests(TestCase):
        def setUp(self):
            ContentType.objects.clear_cache()
            self.addCleanup(ContentType.objects.clear_cache)

</div>

<div class="method">

get_for_id(id)

Lookup a `~django.contrib.contenttypes.models.ContentType` by ID. Since this method uses the same shared cache as `~django.contrib.contenttypes.models.ContentTypeManager.get_for_model`, it's preferred to use this method over the usual `ContentType.objects.get(pk=id)`

</div>

<div class="method">

get_for_model(model, for_concrete_model=True)

Takes either a model class or an instance of a model, and returns the `~django.contrib.contenttypes.models.ContentType` instance representing that model. `for_concrete_model=False` allows fetching the `~django.contrib.contenttypes.models.ContentType` of a proxy model.

</div>

<div class="method">

get_for_models(\*models, for_concrete_models=True)

Takes a variadic number of model classes, and returns a dictionary mapping the model classes to the `~django.contrib.contenttypes.models.ContentType` instances representing them. `for_concrete_models=False` allows fetching the `~django.contrib.contenttypes.models.ContentType` of proxy models.

</div>

<div class="method">

get_by_natural_key(app_label, model)

Returns the `~django.contrib.contenttypes.models.ContentType` instance uniquely identified by the given application label and model name. The primary purpose of this method is to allow `~django.contrib.contenttypes.models.ContentType` objects to be referenced via a `natural key<topics-serialization-natural-keys>` during deserialization.

</div>

</div>

The `~ContentTypeManager.get_for_model` method is especially useful when you know you need to work with a `ContentType <django.contrib.contenttypes.models.ContentType>` but don't want to go to the trouble of obtaining the model's metadata to perform a manual lookup:

``` pycon
>>> from django.contrib.auth.models import User
>>> ContentType.objects.get_for_model(User)
<ContentType: user>
```

<div class="module">

django.contrib.contenttypes.fields

</div>

## Generic relations

Adding a foreign key from one of your own models to `~django.contrib.contenttypes.models.ContentType` allows your model to effectively tie itself to another model class, as in the example of the `~django.contrib.auth.models.Permission` model above. But it's possible to go one step further and use `~django.contrib.contenttypes.models.ContentType` to enable truly generic (sometimes called "polymorphic") relationships between models.

For example, it could be used for a tagging system like so:

    from django.contrib.contenttypes.fields import GenericForeignKey
    from django.contrib.contenttypes.models import ContentType
    from django.db import models


    class TaggedItem(models.Model):
        tag = models.SlugField()
        content_type = models.ForeignKey(ContentType, on_delete=models.CASCADE)
        object_id = models.PositiveBigIntegerField()
        content_object = GenericForeignKey("content_type", "object_id")

        def __str__(self):
            return self.tag

        class Meta:
            indexes = [
                models.Index(fields=["content_type", "object_id"]),
            ]

A normal `~django.db.models.ForeignKey` can only "point to" one other model, which means that if the `TaggedItem` model used a `~django.db.models.ForeignKey` it would have to choose one and only one model to store tags for. The contenttypes application provides a special field type (`GenericForeignKey`) which works around this and allows the relationship to be with any model:

<div class="GenericForeignKey">

There are three parts to setting up a `~django.contrib.contenttypes.fields.GenericForeignKey`:

1.  Give your model a `~django.db.models.ForeignKey` to `~django.contrib.contenttypes.models.ContentType`. The usual name for this field is "content_type".
2.  Give your model a field that can store primary key values from the models you'll be relating to. For most models, this means a `~django.db.models.PositiveBigIntegerField`. The usual name for this field is "object_id".
3.  Give your model a `~django.contrib.contenttypes.fields.GenericForeignKey`, and pass it the names of the two fields described above. If these fields are named "content_type" and "object_id", you can omit this -- those are the default field names `~django.contrib.contenttypes.fields.GenericForeignKey` will look for.

Unlike for the `~django.db.models.ForeignKey`, a database index is *not* automatically created on the `~django.contrib.contenttypes.fields.GenericForeignKey`, so it's recommended that you use `Meta.indexes <django.db.models.Options.indexes>` to add your own multiple column index. This behavior `may change <23435>` in the future.

<div class="attribute">

GenericForeignKey.for_concrete_model

If `False`, the field will be able to reference proxy models. Default is `True`. This mirrors the `for_concrete_model` argument to `~django.contrib.contenttypes.models.ContentTypeManager.get_for_model`.

</div>

</div>

<div class="admonition">

Primary key type compatibility

The "object_id" field doesn't have to be the same type as the primary key fields on the related models, but their primary key values must be coercible to the same type as the "object_id" field by its `~django.db.models.Field.get_db_prep_value` method.

For example, if you want to allow generic relations to models with either `~django.db.models.IntegerField` or `~django.db.models.CharField` primary key fields, you can use `~django.db.models.CharField` for the "object_id" field on your model since integers can be coerced to strings by `~django.db.models.Field.get_db_prep_value`.

For maximum flexibility you can use a `~django.db.models.TextField` which doesn't have a maximum length defined, however this may incur significant performance penalties depending on your database backend.

There is no one-size-fits-all solution for which field type is best. You should evaluate the models you expect to be pointing to and determine which solution will be most effective for your use case.

</div>

<div class="admonition">

Serializing references to `ContentType` objects

If you're serializing data (for example, when generating `~django.test.TransactionTestCase.fixtures`) from a model that implements generic relations, you should probably be using a natural key to uniquely identify related `~django.contrib.contenttypes.models.ContentType` objects. See `natural keys<topics-serialization-natural-keys>` and `dumpdata --natural-foreign` for more information.

</div>

This will enable an API similar to the one used for a normal `~django.db.models.ForeignKey`; each `TaggedItem` will have a `content_object` field that returns the object it's related to, and you can also assign to that field or use it when creating a `TaggedItem`:

``` pycon
>>> from django.contrib.auth.models import User
>>> guido = User.objects.get(username="Guido")
>>> t = TaggedItem(content_object=guido, tag="bdfl")
>>> t.save()
>>> t.content_object
<User: Guido>
```

If the related object is deleted, the `content_type` and `object_id` fields remain set to their original values and the `GenericForeignKey` returns `None`:

``` pycon
>>> guido.delete()
>>> t.content_object  # returns None
```

Due to the way `~django.contrib.contenttypes.fields.GenericForeignKey` is implemented, you cannot use such fields directly with filters (`filter()` and `exclude()`, for example) via the database API. Because a `~django.contrib.contenttypes.fields.GenericForeignKey` isn't a normal field object, these examples will *not* work:

``` pycon
# This will fail
>>> TaggedItem.objects.filter(content_object=guido)
# This will also fail
>>> TaggedItem.objects.get(content_object=guido)
```

Likewise, `~django.contrib.contenttypes.fields.GenericForeignKey`s do not appear in `~django.forms.ModelForm`s.

### Reverse generic relations

<div class="GenericRelation">

<div class="attribute">

related_query_name

The relation on the related object back to this object doesn't exist by default. Setting `related_query_name` creates a relation from the related object back to this one. This allows querying and filtering from the related object.

</div>

</div>

If you know which models you'll be using most often, you can also add a "reverse" generic relationship to enable an additional API. For example:

    from django.contrib.contenttypes.fields import GenericRelation
    from django.db import models


    class Bookmark(models.Model):
        url = models.URLField()
        tags = GenericRelation(TaggedItem)

`Bookmark` instances will each have a `tags` attribute, which can be used to retrieve their associated `TaggedItems`:

``` pycon
>>> b = Bookmark(url="https://www.djangoproject.com/")
>>> b.save()
>>> t1 = TaggedItem(content_object=b, tag="django")
>>> t1.save()
>>> t2 = TaggedItem(content_object=b, tag="python")
>>> t2.save()
>>> b.tags.all()
<QuerySet [<TaggedItem: django>, <TaggedItem: python>]>
```

You can also use `add()`, `create()`, or `set()` to create relationships:

``` pycon
>>> t3 = TaggedItem(tag="Web development")
>>> b.tags.add(t3, bulk=False)
>>> b.tags.create(tag="Web framework")
<TaggedItem: Web framework>
>>> b.tags.all()
<QuerySet [<TaggedItem: django>, <TaggedItem: python>, <TaggedItem: Web development>, <TaggedItem: Web framework>]>
>>> b.tags.set([t1, t3])
>>> b.tags.all()
<QuerySet [<TaggedItem: django>, <TaggedItem: Web development>]>
```

The `remove()` call will bulk delete the specified model objects:

``` pycon
>>> b.tags.remove(t3)
>>> b.tags.all()
<QuerySet [<TaggedItem: django>]>
>>> TaggedItem.objects.all()
<QuerySet [<TaggedItem: django>]>
```

The `clear()` method can be used to bulk delete all related objects for an instance:

``` pycon
>>> b.tags.clear()
>>> b.tags.all()
<QuerySet []>
>>> TaggedItem.objects.all()
<QuerySet []>
```

Defining `~django.contrib.contenttypes.fields.GenericRelation` with `related_query_name` set allows querying from the related object:

    tags = GenericRelation(TaggedItem, related_query_name="bookmark")

This enables filtering, ordering, and other query operations on `Bookmark` from `TaggedItem`:

``` pycon
>>> # Get all tags belonging to bookmarks containing `django` in the url
>>> TaggedItem.objects.filter(bookmark__url__contains="django")
<QuerySet [<TaggedItem: django>, <TaggedItem: python>]>
```

If you don't add the `related_query_name`, you can do the same types of lookups manually:

``` pycon
>>> bookmarks = Bookmark.objects.filter(url__contains="django")
>>> bookmark_type = ContentType.objects.get_for_model(Bookmark)
>>> TaggedItem.objects.filter(content_type__pk=bookmark_type.id, object_id__in=bookmarks)
<QuerySet [<TaggedItem: django>, <TaggedItem: python>]>
```

Just as `~django.contrib.contenttypes.fields.GenericForeignKey` accepts the names of the content-type and object-ID fields as arguments, so too does `~django.contrib.contenttypes.fields.GenericRelation`; if the model which has the generic foreign key is using non-default names for those fields, you must pass the names of the fields when setting up a `.GenericRelation` to it. For example, if the `TaggedItem` model referred to above used fields named `content_type_fk` and `object_primary_key` to create its generic foreign key, then a `.GenericRelation` back to it would need to be defined like so:

    tags = GenericRelation(
        TaggedItem,
        content_type_field="content_type_fk",
        object_id_field="object_primary_key",
    )

Note also, that if you delete an object that has a `~django.contrib.contenttypes.fields.GenericRelation`, any objects which have a `~django.contrib.contenttypes.fields.GenericForeignKey` pointing at it will be deleted as well. In the example above, this means that if a `Bookmark` object were deleted, any `TaggedItem` objects pointing at it would be deleted at the same time.

Unlike `~django.db.models.ForeignKey`, `~django.contrib.contenttypes.fields.GenericForeignKey` does not accept an `~django.db.models.ForeignKey.on_delete` argument to customize this behavior; if desired, you can avoid the cascade-deletion by not using `~django.contrib.contenttypes.fields.GenericRelation`, and alternate behavior can be provided via the `~django.db.models.signals.pre_delete` signal.

### Generic relations and aggregation

`Django's database aggregation API </topics/db/aggregation>` works with a `~django.contrib.contenttypes.fields.GenericRelation`. For example, you can find out how many tags all the bookmarks have:

``` pycon
>>> Bookmark.objects.aggregate(Count("tags"))
{'tags__count': 3}
```

<div class="module">

django.contrib.contenttypes.forms

</div>

### Generic relation in forms

The `django.contrib.contenttypes.forms` module provides:

- `BaseGenericInlineFormSet`
- A formset factory, `generic_inlineformset_factory`, for use with `~django.contrib.contenttypes.fields.GenericForeignKey`.

<div class="BaseGenericInlineFormSet">

<div class="function">

generic_inlineformset_factory(model, form=ModelForm, formset=BaseGenericInlineFormSet, ct_field="content_type", fk_field="object_id", fields=None, exclude=None, extra=3, can_order=False, can_delete=True, max_num=None, formfield_callback=None, validate_max=False, for_concrete_model=True, min_num=None, validate_min=False, absolute_max=None, can_delete_extra=True)

Returns a `GenericInlineFormSet` using `~django.forms.models.modelformset_factory`.

You must provide `ct_field` and `fk_field` if they are different from the defaults, `content_type` and `object_id` respectively. Other parameters are similar to those documented in `~django.forms.models.modelformset_factory` and `~django.forms.models.inlineformset_factory`.

The `for_concrete_model` argument corresponds to the `~django.contrib.contenttypes.fields.GenericForeignKey.for_concrete_model` argument on `GenericForeignKey`.

</div>

</div>

<div class="module">

django.contrib.contenttypes.admin

</div>

### Generic relations in admin

The `django.contrib.contenttypes.admin` module provides `~django.contrib.contenttypes.admin.GenericTabularInline` and `~django.contrib.contenttypes.admin.GenericStackedInline` (subclasses of `~django.contrib.contenttypes.admin.GenericInlineModelAdmin`)

These classes and functions enable the use of generic relations in forms and the admin. See the `model formset </topics/forms/modelforms>` and `admin <using-generic-relations-as-an-inline>` documentation for more information.

<div class="GenericInlineModelAdmin">

The `~django.contrib.contenttypes.admin.GenericInlineModelAdmin` class inherits all properties from an `~django.contrib.admin.InlineModelAdmin` class. However, it adds a couple of its own for working with the generic relation:

<div class="attribute">

ct_field

The name of the `~django.contrib.contenttypes.models.ContentType` foreign key field on the model. Defaults to `content_type`.

</div>

<div class="attribute">

ct_fk_field

The name of the integer field that represents the ID of the related object. Defaults to `object_id`.

</div>

</div>

<div class="GenericTabularInline">

<div class="GenericStackedInline">

Subclasses of `GenericInlineModelAdmin` with stacked and tabular layouts, respectively.

</div>

</div>

<div class="module">

django.contrib.contenttypes.prefetch

</div>

### `GenericPrefetch()`

<div class="GenericPrefetch(lookup, querysets, to_attr=None)">

This lookup is similar to `Prefetch()` and it should only be used on `GenericForeignKey`. The `querysets` argument accepts a list of querysets, each for a different `ContentType`. This is useful for `GenericForeignKey` with non-homogeneous set of results.

</div>

``` pycon
>>> from django.contrib.contenttypes.prefetch import GenericPrefetch
>>> bookmark = Bookmark.objects.create(url="https://www.djangoproject.com/")
>>> animal = Animal.objects.create(name="lion", weight=100)
>>> TaggedItem.objects.create(tag="great", content_object=bookmark)
>>> TaggedItem.objects.create(tag="awesome", content_object=animal)
>>> prefetch = GenericPrefetch(
...     "content_object", [Bookmark.objects.all(), Animal.objects.only("name")]
... )
>>> TaggedItem.objects.prefetch_related(prefetch).all()
<QuerySet [<TaggedItem: Great>, <TaggedItem: Awesome>]>
```
