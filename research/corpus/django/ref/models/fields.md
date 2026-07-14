# Model field reference

<div class="module" synopsis="Built-in field types.">

django.db.models.fields

</div>

<div class="currentmodule">

django.db.models

</div>

This document contains all the API references of `Field` including the [field options](#field-options) and [field types](#field-types) Django offers.

<div class="note">

<div class="title">

Note

</div>

Fields are defined in `django.db.models.fields`, but for convenience they're imported into `django.db.models`. The standard convention is to use `from django.db import models` and refer to fields as `models.<Foo>Field`.

</div>

## Field options

The following arguments are available to all field types. All are optional.

### `null`

<div class="attribute">

Field.null

</div>

If `True`, Django will store empty values as `NULL` in the database. Default is `False`.

Avoid using `~Field.null` on string-based fields such as `CharField` and `TextField`. The Django convention is to use an empty string, not `NULL`, as the "no data" state for string-based fields. If a string-based field has `null=False`, empty strings can still be saved for "no data". If a string-based field has `null=True`, that means it has two possible values for "no data": `NULL`, and the empty string. In most cases, it's redundant to have two possible values for "no data". One exception is when a `CharField` has both `unique=True` and `blank=True` set. In this situation, `null=True` is required to avoid unique constraint violations when saving multiple objects with blank values.

For both string-based and non-string-based fields, you will also need to set `blank=True` if you wish to permit empty values in forms, as the `~Field.null` parameter only affects database storage (see `~Field.blank`).

<div class="note">

<div class="title">

Note

</div>

When using the Oracle database backend, the value `NULL` will be stored to denote the empty string regardless of this attribute.

</div>

### `blank`

<div class="attribute">

Field.blank

</div>

If `True`, the field is allowed to be blank. Default is `False`.

Note that this is different than `~Field.null`. `~Field.null` is purely database-related, whereas `~Field.blank` is validation-related. If a field has `blank=True`, form validation will allow entry of an empty value. If a field has `blank=False`, the field will be required.

<div class="admonition">

Supplying missing values

`blank=True` can be used with fields having `null=False`, but this will require implementing `~django.db.models.Model.clean` on the model in order to programmatically supply any missing values.

</div>

### `choices`

<div class="attribute">

Field.choices

</div>

A mapping or iterable in the format described below to use as choices for this field. If choices are given, they're enforced by `model validation <validating-objects>` and the default form widget will be a select box with these choices instead of the standard text field.

If a mapping is given, the key element is the actual value to be set on the model, and the second element is the human readable name. For example:

    YEAR_IN_SCHOOL_CHOICES = {
        "FR": "Freshman",
        "SO": "Sophomore",
        "JR": "Junior",
        "SR": "Senior",
        "GR": "Graduate",
    }

You can also pass a `sequence` consisting itself of iterables of exactly two items (e.g. `[(A1, B1), (A2, B2), …]`). The first element in each tuple is the actual value to be set on the model, and the second element is the human-readable name. For example:

    YEAR_IN_SCHOOL_CHOICES = [
        ("FR", "Freshman"),
        ("SO", "Sophomore"),
        ("JR", "Junior"),
        ("SR", "Senior"),
        ("GR", "Graduate"),
    ]

`choices` can also be defined as a callable that expects no arguments and returns any of the formats described above. For example:

    def get_currencies():
        return {i: i for i in settings.CURRENCIES}


    class Expense(models.Model):
        amount = models.DecimalField(max_digits=10, decimal_places=2)
        currency = models.CharField(max_length=3, choices=get_currencies)

Passing a callable for `choices` can be particularly handy when, for example, the choices are:

- the result of I/O-bound operations (which could potentially be cached), such as querying a table in the same or an external database, or accessing the choices from a static file.
- a list that is mostly stable but could vary from time to time or from project to project. Examples in this category are using third-party apps that provide a well-known inventory of values, such as currencies, countries, languages, time zones, etc.

Generally, it's best to define choices inside a model class, and to define a suitably-named constant for each value:

    from django.db import models


    class Student(models.Model):
        FRESHMAN = "FR"
        SOPHOMORE = "SO"
        JUNIOR = "JR"
        SENIOR = "SR"
        GRADUATE = "GR"
        YEAR_IN_SCHOOL_CHOICES = {
            FRESHMAN: "Freshman",
            SOPHOMORE: "Sophomore",
            JUNIOR: "Junior",
            SENIOR: "Senior",
            GRADUATE: "Graduate",
        }
        year_in_school = models.CharField(
            max_length=2,
            choices=YEAR_IN_SCHOOL_CHOICES,
            default=FRESHMAN,
        )

        def is_upperclass(self):
            return self.year_in_school in {self.JUNIOR, self.SENIOR}

Though you can define a choices list outside of a model class and then refer to it, defining the choices and names for each choice inside the model class keeps all of that information with the class that uses it, and helps reference the choices (e.g, `Student.SOPHOMORE` will work anywhere that the `Student` model has been imported).

<div id="field-choices-named-groups">

You can also collect your available choices into named groups that can be used for organizational purposes:

    MEDIA_CHOICES = {
        "Audio": {
            "vinyl": "Vinyl",
            "cd": "CD",
        },
        "Video": {
            "vhs": "VHS Tape",
            "dvd": "DVD",
        },
        "unknown": "Unknown",
    }

</div>

The key of the mapping is the name to apply to the group and the value is the choices inside that group, consisting of the field value and a human-readable name for an option. Grouped options may be combined with ungrouped options within a single mapping (such as the `"unknown"` option in this example).

You can also use a sequence, e.g. a list of 2-tuples:

    MEDIA_CHOICES = [
        (
            "Audio",
            (
                ("vinyl", "Vinyl"),
                ("cd", "CD"),
            ),
        ),
        (
            "Video",
            (
                ("vhs", "VHS Tape"),
                ("dvd", "DVD"),
            ),
        ),
        ("unknown", "Unknown"),
    ]

Note that choices can be any sequence object -- not necessarily a list or tuple. This lets you construct choices dynamically. But if you find yourself hacking `~Field.choices` to be dynamic, you're probably better off using a proper database table with a `ForeignKey`. `~Field.choices` is meant for static data that doesn't change much, if ever.

<div class="note">

<div class="title">

Note

</div>

A new migration is created each time the order of `choices` changes.

</div>

For each model field that has `~Field.choices` set, Django will normalize the choices to a list of 2-tuples and add a method to retrieve the human-readable name for the field's current value. See `~django.db.models.Model.get_FOO_display` in the database API documentation.

<div id="field-choices-blank-label">

Unless `blank=False<Field.blank>` is set on the field along with a `~Field.default` then a label containing `"- Select an option -"` will be rendered with the select box. To override this behavior, add a tuple to `choices` containing `None`; e.g. `(None, 'Your String For Display')`. Alternatively, you can use an empty string instead of `None` where this makes sense - such as on a `~django.db.models.CharField`.

</div>

<div class="versionchanged">

6.1

The default blank label was changed from `"---------"` to `"- Select an option -"`.

</div>

#### Enumeration types

In addition, Django provides enumeration types that you can subclass to define choices in a concise way:

    from django.utils.translation import gettext_lazy as _


    class Student(models.Model):
        class YearInSchool(models.TextChoices):
            FRESHMAN = "FR", _("Freshman")
            SOPHOMORE = "SO", _("Sophomore")
            JUNIOR = "JR", _("Junior")
            SENIOR = "SR", _("Senior")
            GRADUATE = "GR", _("Graduate")

        year_in_school = models.CharField(
            max_length=2,
            choices=YearInSchool,
            default=YearInSchool.FRESHMAN,
        )

        def is_upperclass(self):
            return self.year_in_school in {
                self.YearInSchool.JUNIOR,
                self.YearInSchool.SENIOR,
            }

These work similar to `enum` from Python's standard library, but with some modifications:

- Enum member values are a tuple of arguments to use when constructing the concrete data type. Django supports adding an extra string value to the end of this tuple to be used as the human-readable name, or `label`. The `label` can be a lazy translatable string. Thus, in most cases, the member value will be a `(value, label)` 2-tuple. See below for `an example
  of subclassing choices <field-choices-enum-subclassing>` using a more complex data type. If a tuple is not provided, or the last item is not a (lazy) string, the `label` is `automatically generated
  <field-choices-enum-auto-label>` from the member name.

- A `.label` property is added on values, to return the human-readable name.

- A number of custom properties are added to the enumeration classes --`.choices`, `.labels`, `.values`, and `.names` -- to make it easier to access lists of those separate parts of the enumeration.

  <div class="warning">

  <div class="title">

  Warning

  </div>

  These property names cannot be used as member names as they would conflict.

  </div>

- The use of `enum.unique` is enforced to ensure that values cannot be defined multiple times. This is unlikely to be expected in choices for a field.

Note that using `YearInSchool.SENIOR`, `YearInSchool['SENIOR']`, or `YearInSchool('SR')` to access or lookup enum members work as expected, as do the `.name` and `.value` properties on the members.

<div id="field-choices-enum-auto-label">

If you don't need to have the human-readable names translated, you can have them inferred from the member name (replacing underscores with spaces and using title-case):

</div>

``` pycon
>>> class Vehicle(models.TextChoices):
...     CAR = "C"
...     TRUCK = "T"
...     JET_SKI = "J"
...
>>> Vehicle.JET_SKI.label
'Jet Ski'
```

Since the case where the enum values need to be integers is extremely common, Django provides an `IntegerChoices` class. For example:

    class Card(models.Model):
        class Suit(models.IntegerChoices):
            DIAMOND = 1
            SPADE = 2
            HEART = 3
            CLUB = 4

        suit = models.IntegerField(choices=Suit)

It is also possible to make use of the [Enum Functional API](https://docs.python.org/3/howto/enum.html#functional-api) with the caveat that labels are automatically generated as highlighted above:

``` pycon
>>> MedalType = models.TextChoices("MedalType", "GOLD SILVER BRONZE")
>>> MedalType.choices
[('GOLD', 'Gold'), ('SILVER', 'Silver'), ('BRONZE', 'Bronze')]
>>> Place = models.IntegerChoices("Place", "FIRST SECOND THIRD")
>>> Place.choices
[(1, 'First'), (2, 'Second'), (3, 'Third')]
```

<div id="field-choices-enum-subclassing">

If you require support for a concrete data type other than `int` or `str`, you can subclass `Choices` and the required concrete data type, e.g. `~datetime.date` for use with `~django.db.models.DateField`:

    class MoonLandings(datetime.date, models.Choices):
        APOLLO_11 = 1969, 7, 20, "Apollo 11 (Eagle)"
        APOLLO_12 = 1969, 11, 19, "Apollo 12 (Intrepid)"
        APOLLO_14 = 1971, 2, 5, "Apollo 14 (Antares)"
        APOLLO_15 = 1971, 7, 30, "Apollo 15 (Falcon)"
        APOLLO_16 = 1972, 4, 21, "Apollo 16 (Orion)"
        APOLLO_17 = 1972, 12, 11, "Apollo 17 (Challenger)"

</div>

There are some additional caveats to be aware of:

- Enumeration types do not support `named groups
  <field-choices-named-groups>`.

- Because an enumeration with a concrete data type requires all values to match the type, overriding the `blank label <field-choices-blank-label>` cannot be achieved by creating a member with a value of `None`. Instead, set the `__empty__` attribute on the class:

      class Answer(models.IntegerChoices):
          NO = 0, _("No")
          YES = 1, _("Yes")

          __empty__ = _("(Unknown)")

### `db_column`

<div class="attribute">

Field.db_column

</div>

The name of the database column to use for this field. If this isn't given, Django will use the field's name.

If your database column name is an SQL reserved word, or contains characters that aren't allowed in Python variable names -- notably, the hyphen -- that's OK. Django quotes column and table names behind the scenes.

### `db_comment`

<div class="attribute">

Field.db_comment

</div>

The comment on the database column to use for this field. It is useful for documenting fields for individuals with direct database access who may not be looking at your Django code. For example:

    pub_date = models.DateTimeField(
        db_comment="Date and time when the article was published",
    )

### `db_default`

<div class="attribute">

Field.db_default

</div>

The database-computed default value for this field. This can be a literal value or a database function, such as `~django.db.models.functions.Now`:

    created = models.DateTimeField(db_default=Now())

More complex expressions can be used, as long as they are made from literals and database functions:

    month_due = models.DateField(
        db_default=TruncMonth(
            Now() + timedelta(days=90),
            output_field=models.DateField(),
        )
    )

Database defaults cannot reference other fields or models. For example, this is invalid:

    end = models.IntegerField(db_default=F("start") + 50)

If both `db_default` and `Field.default` are set, `default` will take precedence when creating instances in Python code. `db_default` will still be set at the database level and will be used when inserting rows outside of the ORM or when adding a new field in a migration.

If a field has a `db_default` without a `default` set and no value is assigned to the field, a `DatabaseDefault` object is returned as the field value on unsaved model instances. The actual value for the field is determined by the database when the model instance is saved.

### `db_index`

<div class="attribute">

Field.db_index

</div>

If `True`, a database index will be created for this field.

<div class="admonition">

Use the `~Options.indexes` option instead.

Where possible, use the `Meta.indexes <Options.indexes>` option instead. In nearly all cases, `~Options.indexes` provides more functionality than `db_index`. `db_index` may be deprecated in the future.

</div>

### `db_tablespace`

<div class="attribute">

Field.db_tablespace

</div>

The name of the `database tablespace </topics/db/tablespaces>` to use for this field's index, if this field is indexed. The default is the project's `DEFAULT_INDEX_TABLESPACE` setting, if set, or the `~Options.db_tablespace` of the model, if any. If the backend doesn't support tablespaces for indexes, this option is ignored.

### `default`

<div class="attribute">

Field.default

</div>

The default value for the field. This can be a value or a callable object. If callable it will be called every time a new object is created.

The default can't be a mutable object (model instance, `list`, `set`, etc.), as a reference to the same instance of that object would be used as the default value in all new model instances. Instead, wrap the desired default in a callable. For example, if you want to specify a default `dict` for `~django.db.models.JSONField`, use a function:

    def contact_default():
        return {"email": "to1@example.com"}


    contact_info = JSONField("ContactInfo", default=contact_default)

`lambda`s can't be used for field options like `default` because they can't be `serialized by migrations <migration-serializing>`. See that documentation for other caveats.

For fields like `ForeignKey` that map to model instances, defaults should be the value of the field they reference (`pk` unless `~ForeignKey.to_field` is set) instead of model instances.

The default value is used when new model instances are created and a value isn't provided for the field. When the field is a primary key, the default is also used when the field is set to `None`.

The default value can also be set at the database level with `Field.db_default`.

### `editable`

<div class="attribute">

Field.editable

</div>

If `False`, the field will not be displayed in the admin or any other `~django.forms.ModelForm`. It will also be skipped during `model
validation <validating-objects>`. Default is `True`.

### `error_messages`

<div class="attribute">

Field.error_messages

</div>

The `error_messages` argument lets you override the default messages that the field will raise. Pass in a dictionary with keys matching the error messages you want to override.

Error message keys include `null`, `blank`, `invalid`, `invalid_choice`, `unique`, and `unique_for_date`. Additional error message keys are specified for each field in the [Field types](#field-types) section below.

These error messages often don't propagate to forms. See `considerations-regarding-model-errormessages`.

### `help_text`

<div class="attribute">

Field.help_text

</div>

Extra "help" text to be displayed with the form widget. It's useful for documentation even if your field isn't used on a form.

Note that this value is *not* HTML-escaped in automatically-generated forms. This lets you include HTML in `~Field.help_text` if you so desire. For example:

    help_text = "Please use the following format: <em>YYYY-MM-DD</em>."

Alternatively you can use plain text and `django.utils.html.escape` to escape any HTML special characters. Ensure that you escape any help text that may come from untrusted users to avoid a cross-site scripting attack.

### `primary_key`

<div class="attribute">

Field.primary_key

</div>

If `True`, this field is the primary key for the model.

If you don't specify `primary_key=True` for any field in your model and have not defined a composite primary key, Django will automatically add a field to hold the primary key. So, you don't need to set `primary_key=True` on any of your fields unless you want to override the default primary-key behavior. The type of auto-created primary key fields can be specified per app in `AppConfig.default_auto_field <django.apps.AppConfig.default_auto_field>` or globally in the `DEFAULT_AUTO_FIELD` setting. For more, see `automatic-primary-key-fields`.

`primary_key=True` implies `null=False <Field.null>` and `unique=True <Field.unique>`. Only one field per model can set `primary_key=True`. Composite primary keys must be defined using `CompositePrimaryKey` instead of setting this flag to `True` for all fields to maintain this invariant.

The primary key field is read-only. If you change the value of the primary key on an existing object and then save it, a new object will be created alongside the old one.

The primary key field is set to `None` when calling a model instance’s `~django.db.models.Model.delete` method.

### `unique`

<div class="attribute">

Field.unique

</div>

If `True`, this field must be unique throughout the table.

This is enforced at the database level and by model validation. If you try to save a model with a duplicate value in a `~Field.unique` field, a `django.db.IntegrityError` will be raised by the model's `~django.db.models.Model.save` method.

This option is valid on all field types except `ManyToManyField` and `OneToOneField`.

Note that when `unique` is `True`, you don't need to specify `~Field.db_index`, because `unique` implies the creation of an index.

### `unique_for_date`

<div class="attribute">

Field.unique_for_date

</div>

Set this to the name of a `DateField` or `DateTimeField` to require that this field be unique for the value of the date field.

For example, if you have a field `title` that has `unique_for_date="pub_date"`, then Django wouldn't allow the entry of two records with the same `title` and `pub_date`.

Note that if you set this to point to a `DateTimeField`, only the date portion of the field will be considered. Besides, when `USE_TZ` is `True`, the check will be performed in the `current time zone
<default-current-time-zone>` at the time the object gets saved.

This is enforced by `Model.validate_unique` during model validation but not at the database level. If any `~Field.unique_for_date` constraint involves fields that are not part of a `~django.forms.ModelForm` (for example, if one of the fields is listed in `exclude` or has `editable=False<Field.editable>`), `Model.validate_unique` will skip validation for that particular constraint.

### `unique_for_month`

<div class="attribute">

Field.unique_for_month

</div>

Like `~Field.unique_for_date`, but requires the field to be unique with respect to the month.

### `unique_for_year`

<div class="attribute">

Field.unique_for_year

</div>

Like `~Field.unique_for_date` and `~Field.unique_for_month`.

### `verbose_name`

<div class="attribute">

Field.verbose_name

</div>

A human-readable name for the field. If the verbose name isn't given, Django will automatically create it using the field's attribute name, converting underscores to spaces. See `Verbose field names <verbose-field-names>`.

### `validators`

<div class="attribute">

Field.validators

</div>

A list of validators to run for this field. See the `validators
documentation </ref/validators>` for more information.

## Field types

<div class="currentmodule">

django.db.models

</div>

### `AutoField`

<div class="AutoField(**options)">

An `IntegerField` that automatically increments according to available IDs. You usually won't need to use this directly; a primary key field will automatically be added to your model if you don't specify otherwise. See `automatic-primary-key-fields`.

</div>

### `BigAutoField`

<div class="BigAutoField(**options)">

A 64-bit integer, much like an `AutoField` except that it is guaranteed to fit numbers from `1` to `9223372036854775807`.

</div>

### `BigIntegerField`

<div class="BigIntegerField(**options)">

A 64-bit integer, much like an `IntegerField` except that it is guaranteed to fit numbers from `-9223372036854775808` to `9223372036854775807`. The default form widget for this field is a `~django.forms.NumberInput`.

</div>

### `BinaryField`

<div class="BinaryField(max_length=None, **options)">

A field to store raw binary data. It can be assigned `bytes`, `bytearray`, or `memoryview`.

</div>

By default, `BinaryField` sets `~Field.editable` to `False`, in which case it can't be included in a `~django.forms.ModelForm`.

<div class="attribute">

BinaryField.max_length

Optional. The maximum length (in bytes) of the field. The maximum length is enforced in Django's validation using `~django.core.validators.MaxLengthValidator`.

</div>

<div class="admonition">

Abusing `BinaryField`

Although you might think about storing files in the database, consider that it is bad design in 99% of the cases. This field is *not* a replacement for proper `static files </howto/static-files/index>` handling.

</div>

### `BooleanField`

<div class="BooleanField(**options)">

A true/false field.

</div>

The default form widget for this field is `~django.forms.CheckboxInput`, or `~django.forms.NullBooleanSelect` if `null=True <Field.null>`.

The default value of `BooleanField` is `None` when `Field.default` isn't defined.

### `CompositePrimaryKey`

<div class="CompositePrimaryKey(*field_names, **options)">

A virtual field used for defining a composite primary key.

</div>

This field must be defined as the model's `pk` attribute. If present, Django will create the underlying model table with a composite primary key.

The `*field_names` argument is a list of positional field names that compose the primary key.

See `/topics/composite-primary-key` for more details.

### `CharField`

<div class="CharField(max_length=None, **options)">

A string field, for small- to large-sized strings.

</div>

For large amounts of text, use `~django.db.models.TextField`.

The default form widget for this field is a `~django.forms.TextInput`.

`CharField` has the following extra arguments:

<div class="attribute">

CharField.max_length

The maximum length (in characters) of the field. The `max_length` is enforced at the database level and in Django's validation using `~django.core.validators.MaxLengthValidator`. It's required for all database backends included with Django except PostgreSQL and SQLite, which supports unlimited `VARCHAR` columns.

<div class="note">

<div class="title">

Note

</div>

If you are writing an application that must be portable to multiple database backends, you should be aware that there are restrictions on `max_length` for some backends. Refer to the `database backend
notes </ref/databases>` for details.

</div>

</div>

<div class="attribute">

CharField.db_collation

Optional. The database collation name of the field.

<div class="note">

<div class="title">

Note

</div>

Collation names are not standardized. As such, this will not be portable across multiple database backends.

</div>

<div class="admonition">

Oracle

Oracle supports collations only when the `MAX_STRING_SIZE` database initialization parameter is set to `EXTENDED`.

</div>

</div>

### `DateField`

<div class="DateField(auto_now=False, auto_now_add=False, **options)">

A date, represented in Python by a `datetime.date` instance. Has a few extra, optional arguments:

</div>

<div class="attribute">

DateField.auto_now

Automatically set the field to now every time the object is saved. Useful for "last-modified" timestamps. Note that the current date is *always* used; it's not just a default value that you can override.

The field is only automatically updated when calling `Model.save()
<django.db.models.Model.save>`. The field isn't updated when making updates to other fields in other ways such as `QuerySet.update()
<django.db.models.query.QuerySet.update>`, though you can specify a custom value for the field in an update like that.

</div>

<div class="attribute">

DateField.auto_now_add

Automatically set the field to now when the object is first created. Useful for creation of timestamps. Note that the current date is *always* used; it's not just a default value that you can override. So even if you set a value for this field when creating the object, it will be ignored. If you want to be able to modify this field, set the following instead of `auto_now_add=True`:

- For `DateField`: `default=date.today` - from `datetime.date.today`
- For `DateTimeField`: `default=timezone.now` - from `django.utils.timezone.now`

</div>

The default form widget for this field is a `~django.forms.DateInput`. The admin adds a JavaScript calendar, and a shortcut for "Today". Includes an additional `invalid_date` error message key.

The options `auto_now_add`, `auto_now`, and `default` are mutually exclusive. Any combination of these options will result in an error.

<div class="note">

<div class="title">

Note

</div>

As currently implemented, setting `auto_now` or `auto_now_add` to `True` will cause the field to have `editable=False` and `blank=True` set.

</div>

<div class="note">

<div class="title">

Note

</div>

The `auto_now` and `auto_now_add` options will always use the date in the `default timezone <default-current-time-zone>` at the moment of creation or update. If you need something different, you may want to consider using your own callable default or overriding `save()` instead of using `auto_now` or `auto_now_add`; or using a `DateTimeField` instead of a `DateField` and deciding how to handle the conversion from datetime to date at display time.

</div>

<div class="warning">

<div class="title">

Warning

</div>

Always use `DateField` with a `datetime.date` instance.

If you have a `datetime.datetime` instance, it's recommended to convert it to a `datetime.date` first. If you don't, `DateField` will localize the `datetime.datetime` to the `default timezone
<default-current-time-zone>` and convert it to a `datetime.date` instance, removing its time component. This is true for both storage and comparison.

</div>

<div class="warning">

<div class="title">

Warning

</div>

On PostgreSQL and MySQL, arithmetic operations on a `DateField` with a `~datetime.timedelta` return a `datetime` instead of a `date`. This occurs because Python's `timedelta` is converted to SQL `INTERVAL`, and the SQL operation `date +/- interval` returns a `timestamp` on these databases.

To ensure a `date` result, use one of the following approaches. Either explicitly cast the result to a date:

    import datetime
    from django.db.models import DateField, F
    from django.db.models.functions import Cast

    qs = MyModel.objects.annotate(
        previous_day=Cast(
            F("date_field") - datetime.timedelta(days=1),
            output_field=DateField(),
        )
    )

Or on PostgreSQL only, use integer arithmetic to represent days:

    from django.db.models import DateField, ExpressionWrapper, F

    qs = MyModel.objects.annotate(
        previous_day=ExpressionWrapper(
            F("date_field") - 1,  # Subtract 1 day as integer
            output_field=DateField(),
        )
    )

</div>

### `DateTimeField`

<div class="DateTimeField(auto_now=False, auto_now_add=False, **options)">

A date and time, represented in Python by a `datetime.datetime` instance. Takes the same extra arguments as `DateField`.

</div>

The default form widget for this field is a single `~django.forms.DateTimeInput`. The admin uses two separate `~django.forms.TextInput` widgets with JavaScript shortcuts.

<div class="warning">

<div class="title">

Warning

</div>

Always use `DateTimeField` with a `datetime.datetime` instance.

If you have a `datetime.date` instance, it's recommended to convert it to a `datetime.datetime` first. If you don't, `DateTimeField` will use midnight in the `default timezone <default-current-time-zone>` for the time component. This is true for both storage and comparison. To compare the date portion of a `DateTimeField` with a `datetime.date` instance, use the `date` lookup.

</div>

### `DecimalField`

<div class="DecimalField(max_digits=None, decimal_places=None, **options)">

A fixed-precision decimal number, represented in Python by a `~decimal.Decimal` instance. It validates the input using `~django.core.validators.DecimalValidator`.

</div>

Has the following arguments:

<div class="attribute">

DecimalField.max_digits

The maximum number of digits allowed in the number. Note that this number must be greater than or equal to `decimal_places`. It's always required on MySQL because this database doesn't support numeric fields with no precision. It's also required for all database backends when `~DecimalField.decimal_places` is provided.

</div>

<div class="attribute">

DecimalField.decimal_places

The number of decimal places to store with the number. It's always required on MySQL because this database doesn't support numeric fields with no precision. It's also required for all database backends when `~DecimalField.max_digits` is provided.

</div>

For example, to store numbers up to `999.99` with a resolution of 2 decimal places, you'd use:

    models.DecimalField(..., max_digits=5, decimal_places=2)

And to store numbers up to approximately one billion with a resolution of 10 decimal places:

    models.DecimalField(..., max_digits=19, decimal_places=10)

The default form widget for this field is a `~django.forms.NumberInput` when `~django.forms.Field.localize` is `False` or `~django.forms.TextInput` otherwise.

<div class="note">

<div class="title">

Note

</div>

For more information about the differences between the `FloatField` and `DecimalField` classes, please see `FloatField vs. DecimalField <floatfield_vs_decimalfield>`. You should also be aware of `SQLite limitations <sqlite-decimal-handling>` of decimal fields.

</div>

<div class="versionchanged">

6.1

Support for `DecimalField` with no precision was added on Oracle, PostgreSQL, and SQLite.

</div>

### `DurationField`

<div class="DurationField(**options)">

A field for storing periods of time - modeled in Python by `~python:datetime.timedelta`. When used on PostgreSQL, the data type used is an `interval` and on Oracle the data type is `INTERVAL DAY(9) TO SECOND(6)`. Otherwise a `bigint` of microseconds is used.

</div>

<div class="note">

<div class="title">

Note

</div>

Arithmetic with `DurationField` works in most cases. However on all databases other than PostgreSQL, comparing the value of a `DurationField` to arithmetic on `DateTimeField` instances will not work as expected.

</div>

### `EmailField`

<div class="EmailField(max_length=254, **options)">

A `CharField` that checks that the value is a valid email address using `~django.core.validators.EmailValidator`.

</div>

### `FileField`

<div class="FileField(upload_to='', storage=None, max_length=100, **options)">

A file-upload field.

</div>

<div class="note">

<div class="title">

Note

</div>

The `primary_key` argument isn't supported and will raise an error if used.

</div>

Has the following optional arguments:

<div class="attribute">

FileField.upload_to

This attribute provides a way of setting the upload directory and file name, and can be set in two ways. In both cases, the value is passed to the `Storage.save() <django.core.files.storage.Storage.save>` method.

If you specify a string value or a `~pathlib.Path`, it may contain `~time.strftime` formatting, which will be replaced by the date/time of the file upload (so that uploaded files don't fill up the given directory). For example:

    class MyModel(models.Model):
        # file will be uploaded to MEDIA_ROOT/uploads
        upload = models.FileField(upload_to="uploads/")
        # or...
        # file will be saved to MEDIA_ROOT/uploads/2015/01/30
        upload = models.FileField(upload_to="uploads/%Y/%m/%d/")

If you are using the default `~django.core.files.storage.FileSystemStorage`, the string value will be appended to your `MEDIA_ROOT` path to form the location on the local filesystem where uploaded files will be stored. If you are using a different storage, check that storage's documentation to see how it handles `upload_to`.

`upload_to` may also be a callable, such as a function. This will be called to obtain the upload path, including the filename. This callable must accept two arguments and return a Unix-style path (with forward slashes) to be passed along to the storage system. The two arguments are:

<table>
<thead>
<tr class="header">
<th>Argument</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr class="odd">
<td><p><code>instance</code></p></td>
<td><p>An instance of the model where the <code>FileField</code> is defined. More specifically, this is the particular instance where the current file is being attached.</p>
<p>In most cases, this object will not have been saved to the database yet, so if it uses the default <code>AutoField</code>, <em>it might not yet have a value for its primary key field</em>.</p></td>
</tr>
<tr class="even">
<td><p><code>filename</code></p></td>
<td><p>The filename that was originally given to the file. This may or may not be taken into account when determining the final destination path.</p></td>
</tr>
</tbody>
</table>

For example:

    def user_directory_path(instance, filename):
        # file will be uploaded to MEDIA_ROOT/user_<id>/<filename>
        return "user_{0}/{1}".format(instance.user.id, filename)


    class MyModel(models.Model):
        upload = models.FileField(upload_to=user_directory_path)

</div>

<div class="attribute">

FileField.storage

A storage object, or a callable which returns a storage object. This handles the storage and retrieval of your files. See `/topics/files` for details on how to provide this object.

</div>

The default form widget for this field is a `~django.forms.ClearableFileInput`.

Using a `FileField` or an `ImageField` (see below) in a model takes a few steps:

1.  In your settings file, you'll need to define `MEDIA_ROOT` as the full path to a directory where you'd like Django to store uploaded files. (For performance, these files are not stored in the database.) Define `MEDIA_URL` as the base public URL of that directory. Make sure that this directory is writable by the web server's user account.
2.  Add the `FileField` or `ImageField` to your model, defining the `~FileField.upload_to` option to specify a subdirectory of `MEDIA_ROOT` to use for uploaded files.
3.  All that will be stored in your database is a path to the file (relative to `MEDIA_ROOT`). You'll most likely want to use the convenience `~django.db.models.fields.files.FieldFile.url` attribute provided by Django. For example, if your `ImageField` is called `mug_shot`, you can get the absolute path to your image in a template with `{{ object.mug_shot.url }}`.

For example, say your `MEDIA_ROOT` is set to `'/home/media'`, and `~FileField.upload_to` is set to `'photos/%Y/%m/%d'`. The `'%Y/%m/%d'` part of `~FileField.upload_to` is `~time.strftime` formatting; `'%Y'` is the four-digit year, `'%m'` is the two-digit month and `'%d'` is the two-digit day. If you upload a file on Jan. 15, 2007, it will be saved in the directory `/home/media/photos/2007/01/15`.

If you wanted to retrieve the uploaded file's on-disk filename, or the file's size, you could use the `~django.core.files.File.name` and `~django.core.files.File.size` attributes respectively; for more information on the available attributes and methods, see the `~django.core.files.File` class reference and the `/topics/files` topic guide.

<div class="note">

<div class="title">

Note

</div>

The file is saved as part of saving the model in the database, so the actual file name used on disk cannot be relied on until after the model has been saved.

</div>

The uploaded file's relative URL can be obtained using the `~django.db.models.fields.files.FieldFile.url` attribute. Internally, this calls the `~django.core.files.storage.Storage.url` method of the underlying `~django.core.files.storage.Storage` class.

<div id="file-upload-security">

Note that whenever you deal with uploaded files, you should pay close attention to where you're uploading them and what type of files they are, to avoid security holes. *Validate all uploaded files* so that you're sure the files are what you think they are. For example, if you blindly let somebody upload files, without validation, to a directory that's within your web server's document root, then somebody could upload a CGI or PHP script and execute that script by visiting its URL on your site. Don't allow that.

</div>

Also note that even an uploaded HTML file, since it can be executed by the browser (though not by the server), can pose security threats that are equivalent to XSS or CSRF attacks.

`FileField` instances are created in your database as `varchar` columns with a default max length of 100 characters. As with other fields, you can change the maximum length using the `~CharField.max_length` argument.

#### `FileField` and `FieldFile`

<div class="currentmodule">

django.db.models.fields.files

</div>

<div class="FieldFile">

When you access a `~django.db.models.FileField` on a model, you are given an instance of `FieldFile` as a proxy for accessing the underlying file.

</div>

The API of `FieldFile` mirrors that of `~django.core.files.File`, with one key difference: *The object wrapped by the class is not necessarily a wrapper around Python's built-in file object.* Instead, it is a wrapper around the result of the `Storage.open()<django.core.files.storage.Storage.open>` method, which may be a `~django.core.files.File` object, or it may be a custom storage's implementation of the `~django.core.files.File` API.

In addition to the API inherited from `~django.core.files.File` such as `read()` and `write()`, `FieldFile` includes several methods that can be used to interact with the underlying file:

<div class="warning">

<div class="title">

Warning

</div>

Two methods of this class, `~FieldFile.save` and `~FieldFile.delete`, default to saving the model object of the associated `FieldFile` in the database.

</div>

<div class="attribute">

FieldFile.name

</div>

The name of the file including the relative path from the root of the `~django.core.files.storage.Storage` of the associated `~django.db.models.FileField`.

<div class="attribute">

FieldFile.path

</div>

A read-only property to access the file's local filesystem path by calling the `~django.core.files.storage.Storage.path` method of the underlying `~django.core.files.storage.Storage` class.

<div class="attribute">

FieldFile.size

</div>

The result of the underlying `Storage.size()
<django.core.files.storage.Storage.size>` method.

<div class="attribute">

FieldFile.url

</div>

A read-only property to access the file's relative URL by calling the `~django.core.files.storage.Storage.url` method of the underlying `~django.core.files.storage.Storage` class.

<div class="method">

FieldFile.open(mode='rb')

</div>

Opens or reopens the file associated with this instance in the specified `mode`. Unlike the standard Python `open()` method, it doesn't return a file descriptor.

Since the underlying file is opened implicitly when accessing it, it may be unnecessary to call this method except to reset the pointer to the underlying file or to change the `mode`.

<div class="method">

FieldFile.close()

</div>

Behaves like the standard Python `file.close()` method and closes the file associated with this instance.

<div class="method">

FieldFile.save(name, content, save=True)

</div>

This method takes a filename and file contents and passes them to the storage class for the field, then associates the stored file with the model field. If you want to manually associate file data with `~django.db.models.FileField` instances on your model, the `save()` method is used to persist that file data.

Takes two required arguments: `name` which is the name of the file, and `content` which is an object containing the file's contents. The optional `save` argument controls whether or not the model instance is saved after the file associated with this field has been altered. Defaults to `True`.

Note that the `content` argument should be an instance of `django.core.files.File`, not Python's built-in file object. You can construct a `~django.core.files.File` from an existing Python file object like this:

    from django.core.files import File

    # Open an existing file using Python's built-in open()
    f = open("/path/to/hello.world")
    myfile = File(f)

Or you can construct one from a Python string like this:

    from django.core.files.base import ContentFile

    myfile = ContentFile("hello world")

For more information, see `/topics/files`.

<div class="method">

FieldFile.delete(save=True)

</div>

Deletes the file associated with this instance and clears all attributes on the field. Note: This method will close the file if it happens to be open when `delete()` is called.

The optional `save` argument controls whether or not the model instance is saved after the file associated with this field has been deleted. Defaults to `True`.

Note that when a model is deleted, related files are not deleted. If you need to cleanup orphaned files, you'll need to handle it yourself (for instance, with a custom management command that can be run manually or scheduled to run periodically via e.g. cron).

<div class="currentmodule">

django.db.models

</div>

### `FilePathField`

<div class="FilePathField(path='', match=None, recursive=False, allow_files=True, allow_folders=False, max_length=100, **options)">

A `CharField` whose choices are limited to the filenames in a certain directory on the filesystem. Has some special arguments, of which the first is **required**:

</div>

<div class="attribute">

FilePathField.path

Required. The absolute filesystem path to a directory from which this `FilePathField` should get its choices. Example: `"/home/images"`.

`path` may also be a callable, such as a function to dynamically set the path at runtime. Example:

    import os
    from django.conf import settings
    from django.db import models


    def images_path():
        return os.path.join(settings.LOCAL_FILE_DIR, "images")


    class MyModel(models.Model):
        file = models.FilePathField(path=images_path)

</div>

<div class="attribute">

FilePathField.match

Optional. A regular expression, as a string, that `FilePathField` will use to filter filenames. Note that the regex will be applied to the base filename, not the full path. Example: `"foo.*\.txt$"`, which will match a file called `foo23.txt` but not `bar.txt` or `foo23.png`.

</div>

<div class="attribute">

FilePathField.recursive

Optional. Either `True` or `False`. Default is `False`. Specifies whether all subdirectories of `~FilePathField.path` should be included

</div>

<div class="attribute">

FilePathField.allow_files

Optional. Either `True` or `False`. Default is `True`. Specifies whether files in the specified location should be included. Either this or `~FilePathField.allow_folders` must be `True`.

</div>

<div class="attribute">

FilePathField.allow_folders

Optional. Either `True` or `False`. Default is `False`. Specifies whether folders in the specified location should be included. Either this or `~FilePathField.allow_files` must be `True`.

</div>

The one potential gotcha is that `~FilePathField.match` applies to the base filename, not the full path. So, this example:

    FilePathField(path="/home/images", match="foo.*", recursive=True)

...will match `/home/images/foo.png` but not `/home/images/foo/bar.png` because the `~FilePathField.match` applies to the base filename (`foo.png` and `bar.png`).

`FilePathField` instances are created in your database as `varchar` columns with a default max length of 100 characters. As with other fields, you can change the maximum length using the `~CharField.max_length` argument.

### `FloatField`

<div class="FloatField(**options)">

A floating-point number represented in Python by a `float` instance.

</div>

The default form widget for this field is a `~django.forms.NumberInput` when `~django.forms.Field.localize` is `False` or `~django.forms.TextInput` otherwise.

<div id="floatfield_vs_decimalfield">

<div class="admonition">

`FloatField` vs. `DecimalField`

The `FloatField` class is sometimes mixed up with the `DecimalField` class. Although they both represent real numbers, they represent those numbers differently. `FloatField` uses Python's `float` type internally, while `DecimalField` uses Python's `Decimal` type. For information on the difference between the two, see Python's documentation for the `decimal` module.

</div>

</div>

### `GeneratedField`

<div class="GeneratedField(*, expression, output_field, db_persist, **kwargs)">

A field that is always computed based on other fields in the model. This field is managed and updated by the database itself. Uses the `GENERATED ALWAYS` SQL syntax.

</div>

There are two kinds of generated columns: stored and virtual. A stored generated column is computed when it is written (inserted or updated) and occupies storage as if it were a regular column. A virtual generated column occupies no storage and is computed when it is read. Thus, a virtual generated column is similar to a view and a stored generated column is similar to a materialized view.

<div class="attribute">

GeneratedField.expression

An `Expression` used by the database to automatically set the field value each time the model is changed.

The expressions should be deterministic and only reference fields within the model (in the same database table). Generated fields cannot reference other generated fields. Database backends can impose further restrictions.

</div>

<div class="attribute">

GeneratedField.output_field

A model field instance to define the field's data type.

</div>

<div class="attribute">

GeneratedField.db_persist

Determines if the database column should occupy storage as if it were a real column. If `False`, the column acts as a virtual column and does not occupy database storage space.

PostgreSQL \< 18 only supports persisted columns. Oracle \< 23ai/26ai (23.7) only supports virtual columns.

<div class="versionchanged">

6.1

Support for virtual `GeneratedField`s was added on Postgres 18+ and for stored ones on Oracle 23ai/26ai (23.7+).

</div>

</div>

<div class="admonition">

Database limitations

There are many database-specific restrictions on generated fields that Django doesn't validate and the database may raise an error e.g. PostgreSQL requires functions and operators referenced in a generated column to be marked as `IMMUTABLE`.

You should always check that `expression` is supported on your database. Check out [MariaDB](https://mariadb.com/kb/en/generated-columns/#expression-support), [MySQL](https://dev.mysql.com/doc/refman/en/create-table-generated-columns.html), [Oracle](https://docs.oracle.com/en/database/oracle/oracle-database/21/sqlrf/CREATE-TABLE.html#GUID-F9CE0CC3-13AE-4744-A43C-EAC7A71AAAB6__BABIIGBD), [PostgreSQL](https://www.postgresql.org/docs/current/ddl-generated-columns.html), or [SQLite](https://www.sqlite.org/gencol.html#limitations) docs.

</div>

<div class="versionchanged">

6.0

`GeneratedField`s are now automatically refreshed from the database on backends that support it (SQLite, PostgreSQL, and Oracle) and marked as deferred otherwise.

</div>

`~Field.null` has no effect on `GeneratedField` since whether the column is nullable depends on the database and expression used.

### `GenericIPAddressField`

<div class="GenericIPAddressField(protocol='both', unpack_ipv4=False, **options)">

An IPv4 or IPv6 address, in string format (e.g. `192.0.2.30` or `2a02:42fe::4`). The default form widget for this field is a `~django.forms.TextInput`.

</div>

The IPv6 address normalization follows `4291#section-2.2` section 2.2, including using the IPv4 format suggested in paragraph 3 of that section, like `::ffff:192.0.2.0`. For example, `2001:0::0:01` would be normalized to `2001::1`, and `::ffff:0a0a:0a0a` to `::ffff:10.10.10.10`. All characters are converted to lowercase.

<div class="attribute">

GenericIPAddressField.protocol

Limits valid inputs to the specified protocol. Accepted values are `'both'` (default), `'IPv4'` or `'IPv6'`. Matching is case insensitive.

</div>

<div class="attribute">

GenericIPAddressField.unpack_ipv4

Unpacks IPv4 mapped addresses like `::ffff:192.0.2.1`. If this option is enabled that address would be unpacked to `192.0.2.1`. Default is disabled. Can only be used when `protocol` is set to `'both'`.

</div>

If you allow for blank values, you have to allow for null values since blank values are stored as null.

### `ImageField`

<div class="ImageField(upload_to=None, height_field=None, width_field=None, max_length=100, **options)">

Inherits all attributes and methods from `FileField`, but also validates that the uploaded object is a valid image.

</div>

In addition to the special attributes that are available for `FileField`, an `ImageField` also has `height` and `width` attributes.

To facilitate querying on those attributes, `ImageField` has the following optional arguments:

<div class="attribute">

ImageField.height_field

Name of a model field which is auto-populated with the height of the image each time an image object is set.

</div>

<div class="attribute">

ImageField.width_field

Name of a model field which is auto-populated with the width of the image each time an image object is set.

</div>

Requires the `pillow` library.

`ImageField` instances are created in your database as `varchar` columns with a default max length of 100 characters. As with other fields, you can change the maximum length using the `~CharField.max_length` argument.

The default form widget for this field is a `~django.forms.ClearableFileInput`.

### `IntegerField`

<div class="IntegerField(**options)">

An integer. Values are only allowed between certain (database-dependent) points. Values from `-2147483648` to `2147483647` are compatible in all databases supported by Django.

</div>

It uses `~django.core.validators.MinValueValidator` and `~django.core.validators.MaxValueValidator` to validate the input based on the values that the default database supports.

The default form widget for this field is a `~django.forms.NumberInput` when `~django.forms.Field.localize` is `False` or `~django.forms.TextInput` otherwise.

### `JSONField`

<div class="JSONField(encoder=None, decoder=None, **options)">

A field for storing JSON encoded data. In Python the data is represented in its Python native format: dictionaries, lists, strings, numbers, booleans and `None`.

</div>

`JSONField` is supported on MariaDB, MySQL, Oracle, PostgreSQL, and SQLite (with the `JSON1 extension enabled <sqlite-json1>`).

<div class="attribute">

JSONField.encoder

An optional `json.JSONEncoder` subclass to serialize data types not supported by the standard JSON serializer (e.g. `datetime.datetime` or `~python:uuid.UUID`). For example, you can use the `~django.core.serializers.json.DjangoJSONEncoder` class.

Defaults to `json.JSONEncoder`.

</div>

<div class="attribute">

JSONField.decoder

An optional `json.JSONDecoder` subclass to deserialize the value retrieved from the database. The value will be in the format chosen by the custom encoder (most often a string). Your deserialization may need to account for the fact that you can't be certain of the input type. For example, you run the risk of returning a `datetime` that was actually a string that just happened to be in the same format chosen for `datetime`s.

Defaults to `json.JSONDecoder`.

</div>

To query `JSONField` in the database, see `querying-jsonfield`.

<div class="admonition">

Default value

If you give the field a `~django.db.models.Field.default`, ensure it's a callable such as the `dict` class or a function that returns a fresh object each time. Incorrectly using a mutable object like `default={}` or `default=[]` creates a mutable default that is shared between all instances.

</div>

<div class="admonition">

Indexing

`~django.db.models.Index` and `.Field.db_index` both create a B-tree index, which isn't particularly helpful when querying `JSONField`. On PostgreSQL only, you can use `~django.contrib.postgres.indexes.GinIndex` that is better suited.

</div>

<div class="admonition">

PostgreSQL users

PostgreSQL has two native JSON based data types: `json` and `jsonb`. The main difference between them is how they are stored and how they can be queried. PostgreSQL's `json` field is stored as the original string representation of the JSON and must be decoded on the fly when queried based on keys. The `jsonb` field is stored based on the actual structure of the JSON which allows indexing. The trade-off is a small additional cost on writing to the `jsonb` field. `JSONField` uses `jsonb`.

</div>

<div class="admonition">

Oracle users

Oracle Database does not support storing JSON scalar values. Only JSON objects and arrays (represented in Python using `dict` and `list`) are supported.

</div>

### `PositiveBigIntegerField`

<div class="PositiveBigIntegerField(**options)">

Like a `PositiveIntegerField`, but only allows values under a certain (database-dependent) point. Values from `0` to `9223372036854775807` are compatible in all databases supported by Django.

</div>

### `PositiveIntegerField`

<div class="PositiveIntegerField(**options)">

Like an `IntegerField`, but must be either positive or zero (`0`). Values are only allowed under a certain (database-dependent) point. Values from `0` to `2147483647` are compatible in all databases supported by Django. The value `0` is accepted for backward compatibility reasons.

</div>

### `PositiveSmallIntegerField`

<div class="PositiveSmallIntegerField(**options)">

Like a `PositiveIntegerField`, but only allows values under a certain (database-dependent) point. Values from `0` to `32767` are compatible in all databases supported by Django.

</div>

### `SlugField`

<div class="SlugField(max_length=50, **options)">

`Slug <slug>` is a newspaper term. A slug is a short label for something, containing only letters, numbers, underscores or hyphens. They're generally used in URLs.

</div>

Like a CharField, you can specify `~CharField.max_length` (read the note about database portability and `~CharField.max_length` in that section, too). If `~CharField.max_length` is not specified, Django will use a default length of 50.

Implies setting `Field.db_index` to `True`.

It is often useful to automatically prepopulate a SlugField based on the value of some other value. You can do this automatically in the admin using `~django.contrib.admin.ModelAdmin.prepopulated_fields`.

It uses `~django.core.validators.validate_slug` or `~django.core.validators.validate_unicode_slug` for validation.

<div class="attribute">

SlugField.allow_unicode

If `True`, the field accepts Unicode letters in addition to ASCII letters. Defaults to `False`.

</div>

### `SmallAutoField`

<div class="SmallAutoField(**options)">

Like an `AutoField`, but only allows values under a certain (database-dependent) limit. Values from `1` to `32767` are compatible in all databases supported by Django.

</div>

### `SmallIntegerField`

<div class="SmallIntegerField(**options)">

Like an `IntegerField`, but only allows values under a certain (database-dependent) point. Values from `-32768` to `32767` are compatible in all databases supported by Django.

</div>

### `TextField`

<div class="TextField(**options)">

A large text field. The default form widget for this field is a `~django.forms.Textarea`.

</div>

If you specify a `max_length` attribute, it will be reflected in the `~django.forms.Textarea` widget of the auto-generated form field. However it is not enforced at the model or database level. Use a `CharField` for that.

<div class="attribute">

TextField.db_collation

Optional. The database collation name of the field.

<div class="note">

<div class="title">

Note

</div>

Collation names are not standardized. As such, this will not be portable across multiple database backends.

</div>

<div class="admonition">

Oracle

Oracle does not support collations for a `TextField`.

</div>

</div>

### `TimeField`

<div class="TimeField(auto_now=False, auto_now_add=False, **options)">

A time, represented in Python by a `datetime.time` instance. Accepts the same auto-population options as `DateField`.

</div>

The default form widget for this field is a `~django.forms.TimeInput`. The admin adds some JavaScript shortcuts.

### `URLField`

<div class="URLField(max_length=200, **options)">

A `CharField` for a URL, validated by `~django.core.validators.URLValidator`.

</div>

The default form widget for this field is a `~django.forms.URLInput`.

Like all `CharField` subclasses, `URLField` takes the optional `~CharField.max_length` argument. If you don't specify `~CharField.max_length`, a default of 200 is used.

### `UUIDField`

<div class="UUIDField(**options)">

A field for storing universally unique identifiers. Uses Python's `~python:uuid.UUID` class. When used on PostgreSQL and MariaDB, this stores in a `uuid` datatype, otherwise in a `char(32)`.

</div>

Universally unique identifiers are a good alternative to `AutoField` for `~Field.primary_key`. The database will not generate the UUID for you, so it is recommended to use `~Field.default`:

    import uuid
    from django.db import models


    class MyUUIDModel(models.Model):
        id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
        # other fields

Note that a callable (with the parentheses omitted) is passed to `default`, not an instance of `UUID`.

<div class="admonition">

Lookups on PostgreSQL and MariaDB

Using `iexact`, `contains`, `icontains`, `startswith`, `istartswith`, `endswith`, or `iendswith` lookups on PostgreSQL don't work for values without hyphens, because PostgreSQL and MariaDB store them in a hyphenated uuid datatype type.

</div>

## Relationship fields

<div class="module" synopsis="Related field types">

django.db.models.fields.related

</div>

<div class="currentmodule">

django.db.models

</div>

Django also defines a set of fields that represent relations.

### `ForeignKey`

<div class="ForeignKey(to, on_delete, **options)">

A many-to-one relationship. Requires two positional arguments: the class to which the model is related and the `~ForeignKey.on_delete` option:

    from django.db import models


    class Manufacturer(models.Model):
        name = models.TextField()


    class Car(models.Model):
        manufacturer = models.ForeignKey(Manufacturer, on_delete=models.CASCADE)

</div>

The first positional argument can be either a concrete model class or a `lazy reference <lazy-relationships>` to a model class. `Recursive relationships <recursive-relationships>`, where a model has a relationship with itself, are also supported.

See `ForeignKey.on_delete` for details on the second positional argument.

A database index is automatically created on the `ForeignKey`. You can disable this by setting `~Field.db_index` to `False`. You may want to avoid the overhead of an index if you are creating a foreign key for consistency rather than joins, or if you will be creating an alternative index like a partial or multiple column index.

#### Database Representation

Behind the scenes, Django appends `"_id"` to the field name to create its database column name. In the above example, the database table for the `Car` model will have a `manufacturer_id` column. You can change this explicitly by specifying `~Field.db_column`, however, your code should never have to deal with the database column name (unless you write custom SQL). You'll always deal with the field names of your model object.

#### Arguments

`ForeignKey` accepts other arguments that define the details of how the relation works.

<div class="attribute">

ForeignKey.on_delete

When an object referenced by a `ForeignKey` is deleted, the referring objects need updating. The `on_delete` argument specifies how this is done, and whether Django or your database makes the updates. For example, if you have a nullable `ForeignKey` and you want Django to set it to `None` when the referenced object is deleted:

    user = models.ForeignKey(
        User,
        models.SET_NULL,
        blank=True,
        null=True,
    )

The possible values for `~ForeignKey.on_delete` are listed below. Import them from `django.db.models`. The `DB_*` variants use the database to prevent deletions or update referring objects, whilst the other values make Django perform the relevant actions.

The database variants are more efficient because they avoid fetching related objects, but `pre_delete` and `post_delete` signals won't be sent when `DB_CASCADE` is used.

The database variants cannot be mixed with Python variants (other than `DO_NOTHING`) in the same model and in models related to each other.

</div>

<div class="versionchanged">

6.1

Support for `DB_*` variants of the `on_delete` attribute was added.

</div>

The possible values for `~ForeignKey.on_delete` are found in `django.db.models`:

- <div class="attribute">

  CASCADE

  Cascade deletes. Django emulates the behavior of the SQL constraint `ON DELETE CASCADE` and also deletes the object containing the `ForeignKey`.

  `.Model.delete` isn't called on related models, but the `~django.db.models.signals.pre_delete` and `~django.db.models.signals.post_delete` signals are sent for all deleted objects.

  </div>

- <div class="attribute">

  DB_CASCADE

  <div class="versionadded">

  6.1

  </div>

  Cascade deletes. Database-level version of `CASCADE`: the database deletes referred-to rows and the one containing the `ForeignKey`.

  </div>

- <div class="attribute">

  PROTECT

  Prevent deletion of the referenced object by raising `~django.db.models.ProtectedError`, a subclass of `django.db.IntegrityError`.

  </div>

- <div class="attribute">

  RESTRICT

  Prevent deletion of the referenced object by raising `~django.db.models.RestrictedError` (a subclass of `django.db.IntegrityError`). Unlike `PROTECT`, deletion of the referenced object is allowed if it also references a different object that is being deleted in the same operation, but via a `CASCADE` relationship.

  Consider this set of models:

      class Artist(models.Model):
          name = models.CharField(max_length=10)


      class Album(models.Model):
          artist = models.ForeignKey(Artist, on_delete=models.CASCADE)


      class Song(models.Model):
          artist = models.ForeignKey(Artist, on_delete=models.CASCADE)
          album = models.ForeignKey(Album, on_delete=models.RESTRICT)

  `Artist` can be deleted even if that implies deleting an `Album` which is referenced by a `Song`, because `Song` also references `Artist` itself through a cascading relationship. For example:

  ``` pycon
  >>> artist_one = Artist.objects.create(name="artist one")
  >>> artist_two = Artist.objects.create(name="artist two")
  >>> album_one = Album.objects.create(artist=artist_one)
  >>> album_two = Album.objects.create(artist=artist_two)
  >>> song_one = Song.objects.create(artist=artist_one, album=album_one)
  >>> song_two = Song.objects.create(artist=artist_one, album=album_two)
  >>> album_one.delete()
  # Raises RestrictedError.
  >>> artist_two.delete()
  # Raises RestrictedError.
  >>> artist_one.delete()
  (4, {'Song': 2, 'Album': 1, 'Artist': 1})
  ```

  </div>

- <div class="attribute">

  SET_NULL

  Set the `ForeignKey` null; this is only possible if `~Field.null` is `True`.

  </div>

- <div class="attribute">

  DB_SET_NULL

  <div class="versionadded">

  6.1

  </div>

  Set the `ForeignKey` value to `NULL`. This is only possible if `~Field.null` is `True`. Database-level version of `SET_NULL`.

  </div>

- <div class="attribute">

  SET_DEFAULT

  Set the `ForeignKey` to its default value; a default for the `ForeignKey` must be set.

  </div>

- <div class="attribute">

  DB_SET_DEFAULT

  <div class="versionadded">

  6.1

  </div>

  Set the `ForeignKey` value to its `Field.db_default` value, which must be set. If a row in the referenced table is deleted, the foreign key values in the referencing table will be updated to their `Field.db_default` values.

  `DB_SET_DEFAULT` is not supported on MySQL and MariaDB.

  </div>

- <div class="function">

  SET()

  Set the `ForeignKey` to the value passed to `~django.db.models.SET`, or if a callable is passed in, the result of calling it. In most cases, passing a callable will be necessary to avoid executing queries at the time your `models.py` is imported:

      from django.conf import settings
      from django.contrib.auth import get_user_model
      from django.db import models


      def get_sentinel_user():
          return get_user_model().objects.get_or_create(username="deleted")[0]


      class MyModel(models.Model):
          user = models.ForeignKey(
              settings.AUTH_USER_MODEL,
              on_delete=models.SET(get_sentinel_user),
          )

  </div>

- <div class="attribute">

  DO_NOTHING

  Take no action. If your database backend enforces referential integrity, this will cause an `~django.db.IntegrityError` unless you manually add an SQL `ON DELETE` constraint to the database field.

  </div>

<div class="attribute">

ForeignKey.limit_choices_to

Sets a limit to the available choices for this field when this field is rendered using a `ModelForm` or the admin (by default, all objects in the queryset are available to choose). Either a dictionary, a `~django.db.models.Q` object, or a callable returning a dictionary or `~django.db.models.Q` object can be used.

For example:

    staff_member = models.ForeignKey(
        User,
        on_delete=models.CASCADE,
        limit_choices_to={"is_staff": True},
    )

causes the corresponding field on the `ModelForm` to list only `User` instances that have `is_staff=True`. This may be helpful in the Django admin.

The callable form can be helpful, for instance, when used in conjunction with the Python `datetime` module to limit selections by date range. For example:

    def limit_pub_date_choices():
        return {"pub_date__lte": datetime.date.today()}


    limit_choices_to = limit_pub_date_choices

If `limit_choices_to` is or returns a `Q object
<django.db.models.Q>`, which is useful for `complex queries
<complex-lookups-with-q>`, then it will only have an effect on the choices available in the admin when the field is not listed in `~django.contrib.admin.ModelAdmin.raw_id_fields` in the `ModelAdmin` for the model.

<div class="note">

<div class="title">

Note

</div>

If a callable is used for `limit_choices_to`, it will be invoked every time a new form is instantiated. It may also be invoked when a model is validated, for example by management commands or the admin. The admin constructs querysets to validate its form inputs in various edge cases multiple times, so there is a possibility your callable may be invoked several times.

</div>

</div>

<div class="attribute">

ForeignKey.related_name

The name to use for the relation from the related object back to this one. It's also the default value for `related_query_name` (the name to use for the reverse filter name from the target model). See the `related
objects documentation <backwards-related-objects>` for a full explanation and example. Note that you must set this value when defining relations on `abstract models <abstract-base-classes>`; and when you do so `some special syntax <abstract-related-name>` is available.

If you'd prefer Django not to create a backwards relation, set `related_name` to `'+'` or end it with `'+'`. For example, this will ensure that the `User` model won't have a backwards relation to this model:

    user = models.ForeignKey(
        User,
        on_delete=models.CASCADE,
        related_name="+",
    )

</div>

<div class="attribute">

ForeignKey.related_query_name

The name to use for the reverse filter name from the target model. It defaults to the value of `related_name` or `~django.db.models.Options.default_related_name` if set, otherwise it defaults to the name of the model:

    # Declare the ForeignKey with related_query_name
    class Tag(models.Model):
        article = models.ForeignKey(
            Article,
            on_delete=models.CASCADE,
            related_name="tags",
            related_query_name="tag",
        )
        name = models.CharField(max_length=255)


    # That's now the name of the reverse filter
    Article.objects.filter(tag__name="important")

Like `related_name`, `related_query_name` supports app label and class interpolation via `some special syntax <abstract-related-name>`.

</div>

<div class="attribute">

ForeignKey.to_field

The field on the related object that the relation is to. By default, Django uses the primary key of the related object. If you reference a different field, that field must have `unique=True`.

</div>

<div class="attribute">

ForeignKey.db_constraint

Controls whether or not a constraint should be created in the database for this foreign key. The default is `True`, and that's almost certainly what you want; setting this to `False` can be very bad for data integrity. That said, here are some scenarios where you might want to do this:

- You have legacy data that is not valid.
- You're sharding your database.

If this is set to `False`, accessing a related object that doesn't exist will raise its `DoesNotExist` exception.

</div>

<div class="attribute">

ForeignKey.swappable

Controls the migration framework's reaction if this `ForeignKey` is pointing at a swappable model. If it is `True` - the default -then if the `ForeignKey` is pointing at a model which matches the current value of `settings.AUTH_USER_MODEL` (or another swappable model setting) the relationship will be stored in the migration using a reference to the setting, not to the model directly.

You only want to override this to be `False` if you are sure your model should always point toward the swapped-in model - for example, if it is a profile model designed specifically for your custom user model.

Setting it to `False` does not mean you can reference a swappable model even if it is swapped out - `False` means that the migrations made with this `ForeignKey` will always reference the exact model you specify (so it will fail hard if the user tries to run with a `User` model you don't support, for example).

If in doubt, leave it to its default of `True`.

</div>

### `ManyToManyField`

<div class="ManyToManyField(to, **options)">

A many-to-many relationship. Requires a positional argument: the class to which the model is related, which works exactly the same as it does for `ForeignKey`, including `recursive <recursive-relationships>` and `lazy <lazy-relationships>` relationships.

</div>

Related objects can be added, removed, or created with the field's `~django.db.models.fields.related.RelatedManager`.

#### Database Representation

Behind the scenes, Django creates an intermediary join table to represent the many-to-many relationship. By default, this table name is generated using the name of the many-to-many field and the name of the table for the model that contains it. Since some databases don't support table names above a certain length, these table names will be automatically truncated and a uniqueness hash will be used, e.g. `author_books_9cdf`. You can manually provide the name of the join table using the `~ManyToManyField.db_table` option.

#### Arguments

`ManyToManyField` accepts an extra set of arguments -- all optional --that control how the relationship functions.

<div class="attribute">

ManyToManyField.related_name

Same as `ForeignKey.related_name`.

</div>

<div class="attribute">

ManyToManyField.related_query_name

Same as `ForeignKey.related_query_name`.

</div>

<div class="attribute">

ManyToManyField.limit_choices_to

Same as `ForeignKey.limit_choices_to`.

</div>

<div class="attribute">

ManyToManyField.symmetrical

Only used in the definition of ManyToManyFields on self. Consider the following model:

    from django.db import models


    class Person(models.Model):
        friends = models.ManyToManyField("self")

When Django processes this model, it identifies that it has a `ManyToManyField` on itself, and as a result, it doesn't add a `person_set` attribute to the `Person` class. Instead, the `ManyToManyField` is assumed to be symmetrical -- that is, if I am your friend, then you are my friend.

If you do not want symmetry in many-to-many relationships with `self`, set `~ManyToManyField.symmetrical` to `False`. This will force Django to add the descriptor for the reverse relationship, allowing `ManyToManyField` relationships to be non-symmetrical.

</div>

<div class="attribute">

ManyToManyField.through

Django will automatically generate a table to manage many-to-many relationships. However, if you want to manually specify the intermediary table, you can use the `~ManyToManyField.through` option to specify the Django model that represents the intermediate table that you want to use.

The `through` model can be specified using either the model class directly or a `lazy reference <lazy-relationships>` to the model class.

The most common use for this option is when you want to associate `extra data with a many-to-many relationship
<intermediary-manytomany>`.

<div class="note">

<div class="title">

Note

</div>

Recursive relationships using an intermediary model can't determine the reverse accessors names, as they would be the same. You need to set a `~ForeignKey.related_name` to at least one of them. If you'd prefer Django not to create a backwards relation, set `related_name` to `'+'`.

</div>

<div class="admonition">

Foreign key order in intermediary models

When defining an asymmetric many-to-many relationship from a model to itself using an intermediary model without defining `through_fields`, the first foreign key in the intermediary model will be treated as representing the source side of the `ManyToManyField`, and the second as the target side. For example:

    from django.db import models


    class Manufacturer(models.Model):
        name = models.CharField(max_length=255)
        clients = models.ManyToManyField(
            "self", symmetrical=False, related_name="suppliers", through="Supply"
        )


    class Supply(models.Model):
        supplier = models.ForeignKey(
            Manufacturer, models.CASCADE, related_name="supplies_given"
        )
        client = models.ForeignKey(
            Manufacturer, models.CASCADE, related_name="supplies_received"
        )
        product = models.CharField(max_length=255)

Here, the `Manufacturer` model defines the many-to-many relationship with `clients` in its role as a supplier. Therefore, the `supplier` foreign key (the source) must come before the `client` foreign key (the target) in the intermediary `Supply` model.

Specifying `through_fields=("supplier", "client")
<.ManyToManyField.through_fields>` on the `ManyToManyField` makes the order of foreign keys on the `through` model irrelevant.

</div>

If you don't specify an explicit `through` model, there is still an implicit `through` model class you can use to directly access the table created to hold the association. It has three fields to link the models, a primary key and two foreign keys. There is a unique constraint on the two foreign keys.

If the source and target models differ, the following fields are generated:

- `id`: the primary key of the relation.
- `<containing_model>_id`: the `id` of the model that declares the `ManyToManyField`.
- `<other_model>_id`: the `id` of the model that the `ManyToManyField` points to.

If the `ManyToManyField` points from and to the same model, the following fields are generated:

- `id`: the primary key of the relation.
- `from_<model>_id`: the `id` of the instance which points at the model (i.e. the source instance).
- `to_<model>_id`: the `id` of the instance to which the relationship points (i.e. the target model instance).

This class can be used to query associated records for a given model instance like a normal model:

    Model.m2mfield.through.objects.all()

</div>

<div class="attribute">

ManyToManyField.through_fields

Only used when a custom intermediary model is specified. Django will normally determine which fields of the intermediary model to use in order to establish a many-to-many relationship automatically. However, consider the following models:

    from django.db import models


    class Person(models.Model):
        name = models.CharField(max_length=50)


    class Group(models.Model):
        name = models.CharField(max_length=128)
        members = models.ManyToManyField(
            Person,
            through="Membership",
            through_fields=("group", "person"),
        )


    class Membership(models.Model):
        group = models.ForeignKey(Group, on_delete=models.CASCADE)
        person = models.ForeignKey(Person, on_delete=models.CASCADE)
        inviter = models.ForeignKey(
            Person,
            on_delete=models.CASCADE,
            related_name="membership_invites",
        )
        invite_reason = models.CharField(max_length=64)

`Membership` has *two* foreign keys to `Person` (`person` and `inviter`), which makes the relationship ambiguous and Django can't know which one to use. In this case, you must explicitly specify which foreign keys Django should use using `through_fields`, as in the example above.

`through_fields` accepts a 2-tuple `('field1', 'field2')`, where `field1` is the name of the foreign key to the model the `ManyToManyField` is defined on (`group` in this case), and `field2` the name of the foreign key to the target model (`person` in this case).

When you have more than one foreign key on an intermediary model to any (or even both) of the models participating in a many-to-many relationship, you *must* specify `through_fields`. This also applies to `recursive relationships <recursive-relationships>` when an intermediary model is used and there are more than two foreign keys to the model, or you want to explicitly specify which two Django should use.

</div>

<div class="attribute">

ManyToManyField.db_table

The name of the table to create for storing the many-to-many data. If this is not provided, Django will assume a default name based upon the names of: the table for the model defining the relationship and the name of the field itself.

</div>

<div class="attribute">

ManyToManyField.db_constraint

Controls whether or not constraints should be created in the database for the foreign keys in the intermediary table. The default is `True`, and that's almost certainly what you want; setting this to `False` can be very bad for data integrity. That said, here are some scenarios where you might want to do this:

- You have legacy data that is not valid.
- You're sharding your database.

It is an error to pass both `db_constraint` and `through`.

</div>

<div class="attribute">

ManyToManyField.swappable

Controls the migration framework's reaction if this `ManyToManyField` is pointing at a swappable model. If it is `True` - the default - then if the `ManyToManyField` is pointing at a model which matches the current value of `settings.AUTH_USER_MODEL` (or another swappable model setting) the relationship will be stored in the migration using a reference to the setting, not to the model directly.

You only want to override this to be `False` if you are sure your model should always point toward the swapped-in model - for example, if it is a profile model designed specifically for your custom user model.

If in doubt, leave it to its default of `True`.

</div>

`ManyToManyField` does not support `~Field.validators`.

`~Field.null` has no effect since there is no way to require a relationship at the database level.

### `OneToOneField`

<div class="OneToOneField(to, on_delete, parent_link=False, **options)">

A one-to-one relationship. Conceptually, this is similar to a `ForeignKey` with `unique=True <Field.unique>`, but the "reverse" side of the relation will directly return a single object.

</div>

This is most useful as the primary key of a model which "extends" another model in some way; `multi-table-inheritance` is implemented by adding an implicit one-to-one relation from the child model to the parent model, for example.

One positional argument is required: the class to which the model will be related. This works exactly the same as it does for `ForeignKey`, including all the options regarding `recursive <recursive-relationships>` and `lazy <lazy-relationships>` relationships.

If you do not specify the `~ForeignKey.related_name` argument for the `OneToOneField`, Django will use the lowercase name of the current model as default value.

With the following example:

    from django.conf import settings
    from django.db import models


    class MySpecialUser(models.Model):
        user = models.OneToOneField(
            settings.AUTH_USER_MODEL,
            on_delete=models.CASCADE,
        )
        supervisor = models.OneToOneField(
            settings.AUTH_USER_MODEL,
            on_delete=models.CASCADE,
            related_name="supervisor_of",
        )

your resulting `User` model will have the following attributes:

``` pycon
>>> user = User.objects.get(pk=1)
>>> hasattr(user, "myspecialuser")
True
>>> hasattr(user, "supervisor_of")
True
```

A `RelatedObjectDoesNotExist` exception is raised when accessing the reverse relationship if an entry in the related table doesn't exist. This is a subclass of the target model's `Model.DoesNotExist
<django.db.models.Model.DoesNotExist>` exception and can be accessed as an attribute of the reverse accessor. For example, if a user doesn't have a supervisor designated by `MySpecialUser`:

    try:
        user.supervisor_of
    except User.supervisor_of.RelatedObjectDoesNotExist:
        pass

<div id="onetoone-arguments">

Additionally, `OneToOneField` accepts all of the extra arguments accepted by `ForeignKey`, plus one extra argument:

</div>

<div class="attribute">

OneToOneField.parent_link

When `True` and used in a model which inherits from another `concrete model`, indicates that this field should be used as the link back to the parent class, rather than the extra `OneToOneField` which would normally be implicitly created by subclassing.

</div>

See `One-to-one relationships </topics/db/examples/one_to_one>` for usage examples of `OneToOneField`.

### Lazy relationships

Lazy relationships allow referencing models by their names (as strings) or creating recursive relationships. Strings can be used as the first argument in any relationship field to reference models lazily. A lazy reference can be either `recursive <recursive-relationships>`, `relative <relative-relationships>` or `absolute <absolute-relationships>`.

#### Recursive

To define a relationship where a model references itself, use `"self"` as the first argument of the relationship field:

    from django.db import models


    class Manufacturer(models.Model):
        name = models.TextField()
        suppliers = models.ManyToManyField("self", symmetrical=False)

When used in an `abstract model <abstract-base-classes>`, the recursive relationship resolves such that each concrete subclass references itself.

#### Relative

When a relationship needs to be created with a model that has not been defined yet, it can be referenced by its name rather than the model object itself:

    from django.db import models


    class Car(models.Model):
        manufacturer = models.ForeignKey(
            "Manufacturer",
            on_delete=models.CASCADE,
        )


    class Manufacturer(models.Model):
        name = models.TextField()
        suppliers = models.ManyToManyField("self", symmetrical=False)

Relationships defined this way on `abstract models
<abstract-base-classes>` are resolved when the model is subclassed as a concrete model and are not relative to the abstract model's `app_label`:

``` python
from django.db import models


class AbstractCar(models.Model):
    manufacturer = models.ForeignKey("Manufacturer", on_delete=models.CASCADE)

    class Meta:
        abstract = True
```

``` python
from django.db import models
from products.models import AbstractCar


class Manufacturer(models.Model):
    name = models.TextField()


class Car(AbstractCar):
    pass
```

In this example, the `Car.manufacturer` relationship will resolve to `production.Manufacturer`, as it points to the concrete model defined within the `production/models.py` file.

<div class="admonition">

Reusable models with relative references

Relative references allow the creation of reusable abstract models with relationships that can resolve to different implementations of the referenced models in various subclasses across different applications.

</div>

#### Absolute

Absolute references specify a model using its `app_label` and class name, allowing for model references across different applications. This type of lazy relationship can also help resolve circular imports.

For example, if the `Manufacturer` model is defined in another application called `thirdpartyapp`, it can be referenced as:

    class Car(models.Model):
        manufacturer = models.ForeignKey(
            "thirdpartyapp.Manufacturer",
            on_delete=models.CASCADE,
        )

Absolute references always point to the same model, even when used in an `abstract model <abstract-base-classes>`.

## Field API reference

<div class="Field">

`Field` is an abstract class that represents a database table column. Django uses fields to create the database table (`db_type`), to map Python types to database (`get_prep_value`) and vice-versa (`from_db_value`).

A field is thus a fundamental piece in different Django APIs, notably the `~django.db.models.Model` and the `~django.db.models.query.QuerySet` APIs.

In models, a field is instantiated as a class attribute and represents a particular table column, see `/topics/db/models`. It has attributes such as `null` and `unique`, and methods that Django uses to map the field value to database-specific values.

A `Field` is a subclass of `~django.db.models.lookups.RegisterLookupMixin` and thus both `~django.db.models.Transform` and `~django.db.models.Lookup` can be registered on it to be used in `QuerySet`s (e.g. `field_name__exact="foo"`). All `built-in
lookups <field-lookups>` are registered by default.

All of Django's built-in fields, such as `CharField`, are particular implementations of `Field`. If you need a custom field, you can either subclass any of the built-in fields or write a `Field` from scratch. In either case, see `/howto/custom-model-fields`.

<div class="attribute">

description

A verbose description of the field, e.g. for the `django.contrib.admindocs` application.

The description can be of the form:

    description = _("String (up to %(max_length)s)")

where the arguments are interpolated from the field's `__dict__`.

</div>

<div class="attribute">

descriptor_class

A class implementing the `descriptor protocol <descriptors>` that is instantiated and assigned to the model instance attribute. The constructor must accept a single argument, the `Field` instance. Overriding this class attribute allows for customizing the get and set behavior.

</div>

To map a `Field` to a database-specific type, Django exposes several methods:

<div class="method">

get_internal_type()

Returns a string naming this field for backend specific purposes. By default, it returns the class name.

See `emulating-built-in-field-types` for usage in custom fields.

</div>

<div class="method">

db_type(connection)

Returns the database column data type for the `Field`, taking into account the `connection`.

See `custom-database-types` for usage in custom fields.

</div>

<div class="method">

rel_db_type(connection)

Returns the database column data type for fields such as `ForeignKey` and `OneToOneField` that point to the `Field`, taking into account the `connection`.

See `custom-database-types` for usage in custom fields.

</div>

There are three main situations where Django needs to interact with the database backend and fields:

- when it queries the database (Python value -\> database backend value)
- when it loads data from the database (database backend value -\> Python value)
- when it saves to the database (Python value -\> database backend value)

When querying, `get_db_prep_value` and `get_prep_value` are used:

<div class="method">

get_prep_value(value)

`value` is the current value of the model's attribute, and the method should return data in a format that has been prepared for use as a parameter in a query.

See `converting-python-objects-to-query-values` for usage.

</div>

<div class="method">

get_db_prep_value(value, connection, prepared=False)

Converts `value` to a backend-specific value. By default it returns `value` if `prepared=True`, and `get_prep_value(value)
<Field.get_prep_value>` otherwise.

See `converting-query-values-to-database-values` for usage.

</div>

When loading data, `from_db_value` is used:

<div class="method">

from_db_value(value, expression, connection)

Converts a value as returned by the database to a Python object. It is the reverse of `get_prep_value`.

This method is not used for most built-in fields as the database backend already returns the correct Python type, or the backend itself does the conversion.

`expression` is the same as `self`.

See `converting-values-to-python-objects` for usage.

<div class="note">

<div class="title">

Note

</div>

For performance reasons, `from_db_value` is not implemented as a no-op on fields which do not require it (all Django fields). Consequently you may not call `super` in your definition.

</div>

</div>

When saving, `pre_save` and `get_db_prep_save` are used:

<div class="method">

get_db_prep_save(value, connection)

Same as the `get_db_prep_value`, but called when the field value must be *saved* to the database. By default returns `get_db_prep_value`.

</div>

<div class="method">

pre_save(model_instance, add)

Method called prior to `get_db_prep_save` to prepare the value before being saved (e.g. for `DateField.auto_now`).

`model_instance` is the instance this field belongs to and `add` is whether the instance is being saved to the database for the first time.

It should return the value of the appropriate attribute from `model_instance` for this field. The attribute name is in `self.attname` (this is set up by `~django.db.models.Field`).

See `preprocessing-values-before-saving` for usage.

</div>

Fields often receive their values as a different type, either from serialization or from forms.

<div class="method">

to_python(value)

Converts the value into the correct Python object. It acts as the reverse of `value_to_string`, and is also called in `~django.db.models.Model.clean`.

See `converting-values-to-python-objects` for usage.

</div>

Besides saving to the database, the field also needs to know how to serialize its value:

<div class="method">

value_from_object(obj)

Returns the field's value for the given model instance.

This method is often used by `value_to_string`.

</div>

<div class="method">

value_to_string(obj)

Converts `obj` to a string. Used to serialize the value of the field.

See `converting-model-field-to-serialization` for usage.

</div>

When using `model forms </topics/forms/modelforms>`, the `Field` needs to know which form field it should be represented by:

<div class="method">

formfield(form_class=None, choices_form_class=None, \*\*kwargs)

Returns the default `django.forms.Field` of this field for `~django.forms.ModelForm`.

If `~Field.formfield` is overridden to return `None`, this field is excluded from the `~django.forms.ModelForm`.

By default, if both `form_class` and `choices_form_class` are `None`, it uses `~django.forms.CharField`. If the field has `~django.db.models.Field.choices` and `choices_form_class` isn't specified, it uses `~django.forms.TypedChoiceField`.

See `specifying-form-field-for-model-field` for usage.

</div>

<div class="method">

deconstruct()

Returns a 4-tuple with enough information to recreate the field:

1.  The name of the field on the model.
2.  The import path of the field (e.g. `"django.db.models.IntegerField"`). This should be the most portable version, so less specific may be better.
3.  A list of positional arguments.
4.  A dict of keyword arguments.

This method must be added to fields prior to 1.7 to migrate its data using `/topics/migrations`.

</div>

</div>

## Registering and fetching lookups

`Field` implements the `lookup registration API
<lookup-registration-api>`. The API can be used to customize which lookups are available for a field class and its instances, and how lookups are fetched from a field.

# Field attribute reference

Every `Field` instance contains several attributes that allow introspecting its behavior. Use these attributes instead of `isinstance` checks when you need to write code that depends on a field's functionality. These attributes can be used together with the `Model._meta API
<model-meta-field-api>` to narrow down a search for specific field types. Custom model fields should implement these flags.

## Attributes for fields

<div class="attribute">

Field.auto_created

Boolean flag that indicates if the field was automatically created, such as the `OneToOneField` used by model inheritance.

</div>

<div class="attribute">

Field.concrete

Boolean flag that indicates if the field has a database column associated with it.

</div>

<div class="attribute">

Field.hidden

Boolean flag that indicates if a field is hidden and should not be returned by `Options.get_fields()
<django.db.models.options.Options.get_fields>` by default. An example is the reverse field for a `~django.db.models.ForeignKey` with a `related_name` that starts with `'+'`.

</div>

<div class="attribute">

Field.is_relation

Boolean flag that indicates if a field contains references to one or more other models for its functionality (e.g. `ForeignKey`, `ManyToManyField`, `OneToOneField`, etc.).

</div>

<div class="attribute">

Field.model

Returns the model on which the field is defined. If a field is defined on a superclass of a model, `model` will refer to the superclass, not the class of the instance.

</div>

## Attributes for fields with relations

These attributes are used to query for the cardinality and other details of a relation. These attribute are present on all fields; however, they will only have boolean values (rather than `None`) if the field is a relation type (`Field.is_relation=True <Field.is_relation>`).

<div class="attribute">

Field.many_to_many

Boolean flag that is `True` if the field has a many-to-many relation; `False` otherwise. The only field included with Django where this is `True` is `ManyToManyField`.

</div>

<div class="attribute">

Field.many_to_one

Boolean flag that is `True` if the field has a many-to-one relation, such as a `ForeignKey`; `False` otherwise.

</div>

<div class="attribute">

Field.one_to_many

Boolean flag that is `True` if the field has a one-to-many relation, such as a `GenericRelation` or the reverse of a `ForeignKey`; `False` otherwise.

</div>

<div class="attribute">

Field.one_to_one

Boolean flag that is `True` if the field has a one-to-one relation, such as a `OneToOneField`; `False` otherwise.

</div>

<div class="attribute">

Field.related_model

Points to the model the field relates to. For example, `Author` in `ForeignKey(Author, on_delete=models.CASCADE)`. The `related_model` for a `GenericForeignKey` is always `None`.

</div>
