# PostgreSQL specific model fields

All of these fields are available from the `django.contrib.postgres.fields` module.

<div class="currentmodule">

django.contrib.postgres.fields

</div>

## Indexing these fields

`~django.db.models.Index` and `.Field.db_index` both create a B-tree index, which isn't particularly helpful when querying complex data types. Indexes such as `~django.contrib.postgres.indexes.GinIndex` and `~django.contrib.postgres.indexes.GistIndex` are better suited, though the index choice is dependent on the queries that you're using. Generally, GiST may be a good choice for the `range fields <range-fields>` and `HStoreField`, and GIN may be helpful for `ArrayField`.

## `ArrayField`

<div class="ArrayField(base_field, size=None, **options)">

A field for storing lists of data. Most field types can be used, and you pass another field instance as the `base_field
<ArrayField.base_field>`. You may also specify a `size
<ArrayField.size>`. `ArrayField` can be nested to store multi-dimensional arrays.

If you give the field a `~django.db.models.Field.default`, ensure it's a callable such as `list` (for an empty default) or a callable that returns a list (such as a function). Incorrectly using `default=[]` creates a mutable default that is shared between all instances of `ArrayField`.

<div class="attribute">

base_field

This is a required argument.

Specifies the underlying data type and behavior for the array. It should be an instance of a subclass of `~django.db.models.Field`. For example, it could be an `~django.db.models.IntegerField` or a `~django.db.models.CharField`. Most field types are permitted, with the exception of those handling relational data (`~django.db.models.ForeignKey`, `~django.db.models.OneToOneField` and `~django.db.models.ManyToManyField`) and file fields (`~django.db.models.FileField` and `~django.db.models.ImageField`).

It is possible to nest array fields - you can specify an instance of `ArrayField` as the `base_field`. For example:

    from django.contrib.postgres.fields import ArrayField
    from django.db import models


    class ChessBoard(models.Model):
        board = ArrayField(
            ArrayField(
                models.CharField(max_length=10, blank=True),
                size=8,
            ),
            size=8,
        )

Transformation of values between the database and the model, validation of data and configuration, and serialization are all delegated to the underlying base field.

</div>

<div class="attribute">

size

This is an optional argument.

If passed, the array will have a maximum size as specified. This will be passed to the database, although PostgreSQL at present does not enforce the restriction.

</div>

</div>

<div class="note">

<div class="title">

Note

</div>

When nesting `ArrayField`, whether you use the `size` parameter or not, PostgreSQL requires that the arrays are rectangular:

    from django.contrib.postgres.fields import ArrayField
    from django.db import models


    class Board(models.Model):
        pieces = ArrayField(ArrayField(models.IntegerField()))


    # Valid
    Board(
        pieces=[
            [2, 3],
            [2, 1],
        ]
    )

    # Not valid
    Board(
        pieces=[
            [2, 3],
            [2],
        ]
    )

If irregular shapes are required, then the underlying field should be made nullable and the values padded with `None`.

</div>

### Querying `ArrayField`

There are a number of custom lookups and transforms for `ArrayField`. We will use the following example model:

    from django.contrib.postgres.fields import ArrayField
    from django.db import models


    class Post(models.Model):
        name = models.CharField(max_length=200)
        tags = ArrayField(models.CharField(max_length=200), blank=True)

        def __str__(self):
            return self.name

<div class="fieldlookup">

arrayfield.contains

</div>

#### `contains`

The `contains` lookup is overridden on `ArrayField`. The returned objects will be those where the values passed are a subset of the data. It uses the SQL operator `@>`. For example:

``` pycon
>>> Post.objects.create(name="First post", tags=["thoughts", "django"])
>>> Post.objects.create(name="Second post", tags=["thoughts"])
>>> Post.objects.create(name="Third post", tags=["tutorial", "django"])

>>> Post.objects.filter(tags__contains=["thoughts"])
<QuerySet [<Post: First post>, <Post: Second post>]>

>>> Post.objects.filter(tags__contains=["django"])
<QuerySet [<Post: First post>, <Post: Third post>]>

>>> Post.objects.filter(tags__contains=["django", "thoughts"])
<QuerySet [<Post: First post>]>
```

<div class="fieldlookup">

arrayfield.contained_by

</div>

#### `contained_by`

This is the inverse of the `contains <arrayfield.contains>` lookup -the objects returned will be those where the data is a subset of the values passed. It uses the SQL operator `<@`. For example:

``` pycon
>>> Post.objects.create(name="First post", tags=["thoughts", "django"])
>>> Post.objects.create(name="Second post", tags=["thoughts"])
>>> Post.objects.create(name="Third post", tags=["tutorial", "django"])

>>> Post.objects.filter(tags__contained_by=["thoughts", "django"])
<QuerySet [<Post: First post>, <Post: Second post>]>

>>> Post.objects.filter(tags__contained_by=["thoughts", "django", "tutorial"])
<QuerySet [<Post: First post>, <Post: Second post>, <Post: Third post>]>
```

<div class="fieldlookup">

arrayfield.overlap

</div>

#### `overlap`

Returns objects where the data shares any results with the values passed. Uses the SQL operator `&&`. For example:

``` pycon
>>> Post.objects.create(name="First post", tags=["thoughts", "django"])
>>> Post.objects.create(name="Second post", tags=["thoughts", "tutorial"])
>>> Post.objects.create(name="Third post", tags=["tutorial", "django"])

>>> Post.objects.filter(tags__overlap=["thoughts"])
<QuerySet [<Post: First post>, <Post: Second post>]>

>>> Post.objects.filter(tags__overlap=["thoughts", "tutorial"])
<QuerySet [<Post: First post>, <Post: Second post>, <Post: Third post>]>

>>> Post.objects.filter(tags__overlap=Post.objects.values_list("tags"))
<QuerySet [<Post: First post>, <Post: Second post>, <Post: Third post>]>
```

<div class="fieldlookup">

arrayfield.len

</div>

#### `len`

Returns the length of the array. The lookups available afterward are those available for `~django.db.models.IntegerField`. For example:

``` pycon
>>> Post.objects.create(name="First post", tags=["thoughts", "django"])
>>> Post.objects.create(name="Second post", tags=["thoughts"])

>>> Post.objects.filter(tags__len=1)
<QuerySet [<Post: Second post>]>
```

<div class="fieldlookup">

arrayfield.index

</div>

#### Index transforms

Index transforms index into the array. Any non-negative integer can be used. There are no errors if it exceeds the `size <ArrayField.size>` of the array. The lookups available after the transform are those from the `base_field <ArrayField.base_field>`. For example:

``` pycon
>>> Post.objects.create(name="First post", tags=["thoughts", "django"])
>>> Post.objects.create(name="Second post", tags=["thoughts"])

>>> Post.objects.filter(tags__0="thoughts")
<QuerySet [<Post: First post>, <Post: Second post>]>

>>> Post.objects.filter(tags__1__iexact="Django")
<QuerySet [<Post: First post>]>

>>> Post.objects.filter(tags__276="javascript")
<QuerySet []>
```

<div class="note">

<div class="title">

Note

</div>

PostgreSQL uses 1-based indexing for array fields when writing raw SQL. However these indexes and those used in `slices <arrayfield.slice>` use 0-based indexing to be consistent with Python.

</div>

<div class="fieldlookup">

arrayfield.slice

</div>

#### Slice transforms

Slice transforms take a slice of the array. Any two non-negative integers can be used, separated by a single underscore. The lookups available after the transform do not change. For example:

``` pycon
>>> Post.objects.create(name="First post", tags=["thoughts", "django"])
>>> Post.objects.create(name="Second post", tags=["thoughts"])
>>> Post.objects.create(name="Third post", tags=["django", "python", "thoughts"])

>>> Post.objects.filter(tags__0_1=["thoughts"])
<QuerySet [<Post: First post>, <Post: Second post>]>

>>> Post.objects.filter(tags__0_2__contains=["thoughts"])
<QuerySet [<Post: First post>, <Post: Second post>]>
```

<div class="note">

<div class="title">

Note

</div>

PostgreSQL uses 1-based indexing for array fields when writing raw SQL. However these slices and those used in `indexes <arrayfield.index>` use 0-based indexing to be consistent with Python.

</div>

<div class="admonition">

Multidimensional arrays with indexes and slices

PostgreSQL has some rather esoteric behavior when using indexes and slices on multidimensional arrays. It will always work to use indexes to reach down to the final underlying data, but most other slices behave strangely at the database level and cannot be supported in a logical, consistent fashion by Django.

</div>

## `HStoreField`

<div class="HStoreField(**options)">

A field for storing key-value pairs. The Python data type used is a `dict`. Keys must be strings, and values may be either strings or nulls (`None` in Python).

To use this field, you'll need to:

1.  Add `'django.contrib.postgres'` in your `INSTALLED_APPS`.
2.  `Set up the hstore extension <create-postgresql-extensions>` in PostgreSQL.

You'll see an error like `can't adapt type 'dict'` if you skip the first step, or `type "hstore" does not exist` if you skip the second.

</div>

<div class="note">

<div class="title">

Note

</div>

On occasions it may be useful to require or restrict the keys which are valid for a given field. This can be done using the `~django.contrib.postgres.validators.KeysValidator`.

</div>

### Querying `HStoreField`

In addition to the ability to query by key, there are a number of custom lookups available for `HStoreField`.

We will use the following example model:

    from django.contrib.postgres.fields import HStoreField
    from django.db import models


    class Dog(models.Model):
        name = models.CharField(max_length=200)
        data = HStoreField()

        def __str__(self):
            return self.name

<div class="fieldlookup">

hstorefield.key

</div>

#### Key lookups

To query based on a given key, you can use that key as the lookup name:

``` pycon
>>> Dog.objects.create(name="Rufus", data={"breed": "labrador"})
>>> Dog.objects.create(name="Meg", data={"breed": "collie"})

>>> Dog.objects.filter(data__breed="collie")
<QuerySet [<Dog: Meg>]>
```

You can chain other lookups after key lookups:

``` pycon
>>> Dog.objects.filter(data__breed__contains="l")
<QuerySet [<Dog: Rufus>, <Dog: Meg>]>
```

or use `F()` expressions to annotate a key value. For example:

``` pycon
>>> from django.db.models import F
>>> rufus = Dog.objects.annotate(breed=F("data__breed"))[0]
>>> rufus.breed
'labrador'
```

If the key you wish to query by clashes with the name of another lookup, you need to use the `hstorefield.contains` lookup instead.

<div class="note">

<div class="title">

Note

</div>

Key transforms can also be chained with: `contains`, `icontains`, `endswith`, `iendswith`, `iexact`, `regex`, `iregex`, `startswith`, and `istartswith` lookups.

</div>

<div class="warning">

<div class="title">

Warning

</div>

Since any string could be a key in a hstore value, any lookup other than those listed below will be interpreted as a key lookup. No errors are raised. Be extra careful for typing mistakes, and always check your queries work as you intend.

</div>

<div class="fieldlookup">

hstorefield.contains

</div>

#### `contains`

The `contains` lookup is overridden on `~django.contrib.postgres.fields.HStoreField`. The returned objects are those where the given `dict` of key-value pairs are all contained in the field. It uses the SQL operator `@>`. For example:

``` pycon
>>> Dog.objects.create(name="Rufus", data={"breed": "labrador", "owner": "Bob"})
>>> Dog.objects.create(name="Meg", data={"breed": "collie", "owner": "Bob"})
>>> Dog.objects.create(name="Fred", data={})

>>> Dog.objects.filter(data__contains={"owner": "Bob"})
<QuerySet [<Dog: Rufus>, <Dog: Meg>]>

>>> Dog.objects.filter(data__contains={"breed": "collie"})
<QuerySet [<Dog: Meg>]>
```

<div class="fieldlookup">

hstorefield.contained_by

</div>

#### `contained_by`

This is the inverse of the `contains <hstorefield.contains>` lookup -the objects returned will be those where the key-value pairs on the object are a subset of those in the value passed. It uses the SQL operator `<@`. For example:

``` pycon
>>> Dog.objects.create(name="Rufus", data={"breed": "labrador", "owner": "Bob"})
>>> Dog.objects.create(name="Meg", data={"breed": "collie", "owner": "Bob"})
>>> Dog.objects.create(name="Fred", data={})

>>> Dog.objects.filter(data__contained_by={"breed": "collie", "owner": "Bob"})
<QuerySet [<Dog: Meg>, <Dog: Fred>]>

>>> Dog.objects.filter(data__contained_by={"breed": "collie"})
<QuerySet [<Dog: Fred>]>
```

<div class="fieldlookup">

hstorefield.has_key

</div>

#### `has_key`

Returns objects where the given key is in the data. Uses the SQL operator `?`. For example:

``` pycon
>>> Dog.objects.create(name="Rufus", data={"breed": "labrador"})
>>> Dog.objects.create(name="Meg", data={"breed": "collie", "owner": "Bob"})

>>> Dog.objects.filter(data__has_key="owner")
<QuerySet [<Dog: Meg>]>
```

<div class="fieldlookup">

hstorefield.has_any_keys

</div>

#### `has_any_keys`

Returns objects where any of the given keys are in the data. Uses the SQL operator `?|`. For example:

``` pycon
>>> Dog.objects.create(name="Rufus", data={"breed": "labrador"})
>>> Dog.objects.create(name="Meg", data={"owner": "Bob"})
>>> Dog.objects.create(name="Fred", data={})

>>> Dog.objects.filter(data__has_any_keys=["owner", "breed"])
<QuerySet [<Dog: Rufus>, <Dog: Meg>]>
```

<div class="fieldlookup">

hstorefield.has_keys

</div>

#### `has_keys`

Returns objects where all of the given keys are in the data. Uses the SQL operator `?&`. For example:

``` pycon
>>> Dog.objects.create(name="Rufus", data={})
>>> Dog.objects.create(name="Meg", data={"breed": "collie", "owner": "Bob"})

>>> Dog.objects.filter(data__has_keys=["breed", "owner"])
<QuerySet [<Dog: Meg>]>
```

<div class="fieldlookup">

hstorefield.keys

</div>

#### `keys`

Returns objects where the array of keys is the given value. Note that the order is not guaranteed to be reliable, so this transform is mainly useful for using in conjunction with lookups on `~django.contrib.postgres.fields.ArrayField`. Uses the SQL function `akeys()`. For example:

``` pycon
>>> Dog.objects.create(name="Rufus", data={"toy": "bone"})
>>> Dog.objects.create(name="Meg", data={"breed": "collie", "owner": "Bob"})

>>> Dog.objects.filter(data__keys__overlap=["breed", "toy"])
<QuerySet [<Dog: Rufus>, <Dog: Meg>]>
```

<div class="fieldlookup">

hstorefield.values

</div>

#### `values`

Returns objects where the array of values is the given value. Note that the order is not guaranteed to be reliable, so this transform is mainly useful for using in conjunction with lookups on `~django.contrib.postgres.fields.ArrayField`. Uses the SQL function `avals()`. For example:

``` pycon
>>> Dog.objects.create(name="Rufus", data={"breed": "labrador"})
>>> Dog.objects.create(name="Meg", data={"breed": "collie", "owner": "Bob"})

>>> Dog.objects.filter(data__values__contains=["collie"])
<QuerySet [<Dog: Meg>]>
```

## Range Fields

There are five range field types, corresponding to the built-in range types in PostgreSQL. These fields are used to store a range of values; for example the start and end timestamps of an event, or the range of ages an activity is suitable for.

All of the range fields translate to `psycopg Range objects
<psycopg:adapt-range>` in Python, but also accept tuples as input if no bounds information is necessary. The default is lower bound included, upper bound excluded, that is `[)` (see the PostgreSQL documentation for details about [different bounds](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-IO)). The default bounds can be changed for non-discrete range fields (`.DateTimeRangeField` and `.DecimalRangeField`) by using the `default_bounds` argument.

<div class="admonition">

PostgreSQL normalizes a range with no points to the empty range

A range with equal values specified for an included lower bound and an excluded upper bound, such as `Range(datetime.date(2005, 6, 21), datetime.date(2005, 6, 21))` or `[4, 4)`, has no points. PostgreSQL will normalize the value to empty when saving to the database, and the original bound values will be lost. See the [PostgreSQL documentation for details](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-IO).

</div>

### `IntegerRangeField`

<div class="IntegerRangeField(**options)">

Stores a range of integers. Based on an `~django.db.models.IntegerField`. Represented by an `int4range` in the database and a `django.db.backends.postgresql.psycopg_any.NumericRange` in Python.

Regardless of the bounds specified when saving the data, PostgreSQL always returns a range in a canonical form that includes the lower bound and excludes the upper bound, that is `[)`.

</div>

### `BigIntegerRangeField`

<div class="BigIntegerRangeField(**options)">

Stores a range of large integers. Based on a `~django.db.models.BigIntegerField`. Represented by an `int8range` in the database and a `django.db.backends.postgresql.psycopg_any.NumericRange` in Python.

Regardless of the bounds specified when saving the data, PostgreSQL always returns a range in a canonical form that includes the lower bound and excludes the upper bound, that is `[)`.

</div>

### `DecimalRangeField`

<div class="DecimalRangeField(default_bounds='[)', **options)">

Stores a range of floating point values. Based on a `~django.db.models.DecimalField`. Represented by a `numrange` in the database and a `django.db.backends.postgresql.psycopg_any.NumericRange` in Python.

<div class="attribute">

DecimalRangeField.default_bounds

Optional. The value of `bounds` for list and tuple inputs. The default is lower bound included, upper bound excluded, that is `[)` (see the PostgreSQL documentation for details about [different bounds](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-IO)). `default_bounds` is not used for `django.db.backends.postgresql.psycopg_any.NumericRange` inputs.

</div>

</div>

### `DateTimeRangeField`

<div class="DateTimeRangeField(default_bounds='[)', **options)">

Stores a range of timestamps. Based on a `~django.db.models.DateTimeField`. Represented by a `tstzrange` in the database and a `django.db.backends.postgresql.psycopg_any.DateTimeTZRange` in Python.

<div class="attribute">

DateTimeRangeField.default_bounds

Optional. The value of `bounds` for list and tuple inputs. The default is lower bound included, upper bound excluded, that is `[)` (see the PostgreSQL documentation for details about [different bounds](https://www.postgresql.org/docs/current/rangetypes.html#RANGETYPES-IO)). `default_bounds` is not used for `django.db.backends.postgresql.psycopg_any.DateTimeTZRange` inputs.

</div>

</div>

### `DateRangeField`

<div class="DateRangeField(**options)">

Stores a range of dates. Based on a `~django.db.models.DateField`. Represented by a `daterange` in the database and a `django.db.backends.postgresql.psycopg_any.DateRange` in Python.

Regardless of the bounds specified when saving the data, PostgreSQL always returns a range in a canonical form that includes the lower bound and excludes the upper bound, that is `[)`.

</div>

### Querying Range Fields

There are a number of custom lookups and transforms for range fields. They are available on all the above fields, but we will use the following example model:

    from django.contrib.postgres.fields import IntegerRangeField
    from django.db import models


    class Event(models.Model):
        name = models.CharField(max_length=200)
        ages = IntegerRangeField()
        start = models.DateTimeField()

        def __str__(self):
            return self.name

We will also use the following example objects:

``` pycon
>>> import datetime
>>> from django.utils import timezone
>>> now = timezone.now()
>>> Event.objects.create(name="Soft play", ages=(0, 10), start=now)
>>> Event.objects.create(
...     name="Pub trip", ages=(21, None), start=now - datetime.timedelta(days=1)
... )
```

and `NumericRange`:

> \>\>\> from django.db.backends.postgresql.psycopg_any import NumericRange

#### Containment functions

As with other PostgreSQL fields, there are three standard containment operators: `contains`, `contained_by` and `overlap`, using the SQL operators `@>`, `<@`, and `&&` respectively.

<div class="fieldlookup">

rangefield.contains

</div>

##### `contains`

> \>\>\> Event.objects.filter(ages\_\_contains=NumericRange(4, 5)) \<QuerySet \[\<Event: Soft play\>\]\>

<div class="fieldlookup">

rangefield.contained_by

</div>

##### `contained_by`

> \>\>\> Event.objects.filter(ages\_\_contained_by=NumericRange(0, 15)) \<QuerySet \[\<Event: Soft play\>\]\>

The `contained_by` lookup is also available on the non-range field types: `~django.db.models.SmallAutoField`, `~django.db.models.AutoField`, `~django.db.models.BigAutoField`, `~django.db.models.SmallIntegerField`, `~django.db.models.IntegerField`, `~django.db.models.BigIntegerField`, `~django.db.models.DecimalField`, `~django.db.models.FloatField`, `~django.db.models.DateField`, and `~django.db.models.DateTimeField`. For example:

``` pycon
>>> from django.db.backends.postgresql.psycopg_any import DateTimeTZRange
>>> Event.objects.filter(
...     start__contained_by=DateTimeTZRange(
...         timezone.now() - datetime.timedelta(hours=1),
...         timezone.now() + datetime.timedelta(hours=1),
...     ),
... )
<QuerySet [<Event: Soft play>]>
```

<div class="fieldlookup">

rangefield.overlap

</div>

##### `overlap`

> \>\>\> Event.objects.filter(ages\_\_overlap=NumericRange(8, 12)) \<QuerySet \[\<Event: Soft play\>\]\>

#### Comparison functions

Range fields support the standard lookups: `lt`, `gt`, `lte` and `gte`. These are not particularly helpful - they compare the lower bounds first and then the upper bounds only if necessary. This is also the strategy used to order by a range field. It is better to use the specific range comparison operators.

<div class="fieldlookup">

rangefield.fully_lt

</div>

##### `fully_lt`

The returned ranges are strictly less than the passed range. In other words, all the points in the returned range are less than all those in the passed range.

> \>\>\> Event.objects.filter(ages\_\_fully_lt=NumericRange(11, 15)) \<QuerySet \[\<Event: Soft play\>\]\>

<div class="fieldlookup">

rangefield.fully_gt

</div>

##### `fully_gt`

The returned ranges are strictly greater than the passed range. In other words, the all the points in the returned range are greater than all those in the passed range.

> \>\>\> Event.objects.filter(ages\_\_fully_gt=NumericRange(11, 15)) \<QuerySet \[\<Event: Pub trip\>\]\>

<div class="fieldlookup">

rangefield.not_lt

</div>

##### `not_lt`

The returned ranges do not contain any points less than the passed range, that is the lower bound of the returned range is at least the lower bound of the passed range.

> \>\>\> Event.objects.filter(ages\_\_not_lt=NumericRange(0, 15)) \<QuerySet \[\<Event: Soft play\>, \<Event: Pub trip\>\]\>

<div class="fieldlookup">

rangefield.not_gt

</div>

##### `not_gt`

The returned ranges do not contain any points greater than the passed range, that is the upper bound of the returned range is at most the upper bound of the passed range.

> \>\>\> Event.objects.filter(ages\_\_not_gt=NumericRange(3, 10)) \<QuerySet \[\<Event: Soft play\>\]\>

<div class="fieldlookup">

rangefield.adjacent_to

</div>

##### `adjacent_to`

The returned ranges share a bound with the passed range.

> \>\>\> Event.objects.filter(ages\_\_adjacent_to=NumericRange(10, 21)) \<QuerySet \[\<Event: Soft play\>, \<Event: Pub trip\>\]\>

#### Querying using the bounds

Range fields support several extra lookups.

<div class="fieldlookup">

rangefield.startswith

</div>

##### `startswith`

Returned objects have the given lower bound. Can be chained to valid lookups for the base field.

> \>\>\> Event.objects.filter(ages\_\_startswith=21) \<QuerySet \[\<Event: Pub trip\>\]\>

<div class="fieldlookup">

rangefield.endswith

</div>

##### `endswith`

Returned objects have the given upper bound. Can be chained to valid lookups for the base field.

> \>\>\> Event.objects.filter(ages\_\_endswith=10) \<QuerySet \[\<Event: Soft play\>\]\>

<div class="fieldlookup">

rangefield.isempty

</div>

##### `isempty`

Returned objects are empty ranges. Can be chained to valid lookups for a `~django.db.models.BooleanField`.

> \>\>\> Event.objects.filter(ages\_\_isempty=True) \<QuerySet \[\]\>

<div class="fieldlookup">

rangefield.lower_inc

</div>

##### `lower_inc`

Returns objects that have inclusive or exclusive lower bounds, depending on the boolean value passed. Can be chained to valid lookups for a `~django.db.models.BooleanField`.

> \>\>\> Event.objects.filter(ages\_\_lower_inc=True) \<QuerySet \[\<Event: Soft play\>, \<Event: Pub trip\>\]\>

<div class="fieldlookup">

rangefield.lower_inf

</div>

##### `lower_inf`

Returns objects that have unbounded (infinite) or bounded lower bound, depending on the boolean value passed. Can be chained to valid lookups for a `~django.db.models.BooleanField`.

> \>\>\> Event.objects.filter(ages\_\_lower_inf=True) \<QuerySet \[\]\>

<div class="fieldlookup">

rangefield.upper_inc

</div>

##### `upper_inc`

Returns objects that have inclusive or exclusive upper bounds, depending on the boolean value passed. Can be chained to valid lookups for a `~django.db.models.BooleanField`.

> \>\>\> Event.objects.filter(ages\_\_upper_inc=True) \<QuerySet \[\]\>

<div class="fieldlookup">

rangefield.upper_inf

</div>

##### `upper_inf`

Returns objects that have unbounded (infinite) or bounded upper bound, depending on the boolean value passed. Can be chained to valid lookups for a `~django.db.models.BooleanField`.

> \>\>\> Event.objects.filter(ages\_\_upper_inf=True) \<QuerySet \[\<Event: Pub trip\>\]\>

### Defining your own range types

PostgreSQL allows the definition of custom range types. Django's model and form field implementations use base classes below, and `psycopg` provides a `~psycopg:psycopg.types.range.register_range` to allow use of custom range types.

<div class="RangeField(**options)">

Base class for model range fields.

<div class="attribute">

base_field

The model field class to use.

</div>

<div class="attribute">

range_type

The range type to use.

</div>

<div class="attribute">

form_field

The form field class to use. Should be a subclass of `django.contrib.postgres.forms.BaseRangeField`.

</div>

</div>

<div class="django.contrib.postgres.forms.BaseRangeField">

Base class for form range fields.

<div class="attribute">

base_field

The form field to use.

</div>

<div class="attribute">

range_type

The range type to use.

</div>

</div>

### Range operators

<div class="RangeOperators">

PostgreSQL provides a set of SQL operators that can be used together with the range data types (see [the PostgreSQL documentation for the full details of range operators \<https://www.postgresql.org/docs/current/ functions-range.html#RANGE-OPERATORS-TABLE\>]()). This class is meant as a convenient method to avoid typos. The operator names overlap with the names of corresponding lookups.

</div>

``` python
class RangeOperators:
    EQUAL = "="
    NOT_EQUAL = "<>"
    CONTAINS = "@>"
    CONTAINED_BY = "<@"
    OVERLAPS = "&&"
    FULLY_LT = "<<"
    FULLY_GT = ">>"
    NOT_LT = "&>"
    NOT_GT = "&<"
    ADJACENT_TO = "-|-"
```

### RangeBoundary() expressions

<div class="RangeBoundary(inclusive_lower=True, inclusive_upper=False)">

<div class="attribute">

inclusive_lower

If `True` (default), the lower bound is inclusive `'['`, otherwise it's exclusive `'('`.

</div>

<div class="attribute">

inclusive_upper

If `False` (default), the upper bound is exclusive `')'`, otherwise it's inclusive `']'`.

</div>

</div>

A `RangeBoundary()` expression represents the range boundaries. It can be used with a custom range functions that expected boundaries, for example to define `~django.contrib.postgres.constraints.ExclusionConstraint`. See [the PostgreSQL documentation for the full details \<https://www.postgresql.org/ docs/current/rangetypes.html#RANGETYPES-INCLUSIVITY\>]().
