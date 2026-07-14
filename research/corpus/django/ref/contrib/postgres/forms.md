# PostgreSQL specific form fields and widgets

All of these fields and widgets are available from the `django.contrib.postgres.forms` module.

<div class="currentmodule">

django.contrib.postgres.forms

</div>

## Fields

### `SimpleArrayField`

<div class="SimpleArrayField(base_field, delimiter=',', max_length=None, min_length=None)">

A field which maps to an array. It is represented by an HTML `<input>`.

<div class="attribute">

base_field

This is a required argument.

It specifies the underlying form field for the array. This is not used to render any HTML, but it is used to process the submitted data and validate it. For example:

``` pycon
>>> from django import forms
>>> from django.contrib.postgres.forms import SimpleArrayField

>>> class NumberListForm(forms.Form):
...     numbers = SimpleArrayField(forms.IntegerField())
...

>>> form = NumberListForm({"numbers": "1,2,3"})
>>> form.is_valid()
True
>>> form.cleaned_data
{'numbers': [1, 2, 3]}

>>> form = NumberListForm({"numbers": "1,2,a"})
>>> form.is_valid()
False
```

</div>

<div class="attribute">

delimiter

This is an optional argument which defaults to a comma: `,`. This value is used to split the submitted data. It allows you to chain `SimpleArrayField` for multidimensional data:

``` pycon
>>> from django import forms
>>> from django.contrib.postgres.forms import SimpleArrayField

>>> class GridForm(forms.Form):
...     places = SimpleArrayField(SimpleArrayField(IntegerField()), delimiter="|")
...

>>> form = GridForm({"places": "1,2|2,1|4,3"})
>>> form.is_valid()
True
>>> form.cleaned_data
{'places': [[1, 2], [2, 1], [4, 3]]}
```

<div class="note">

<div class="title">

Note

</div>

The field does not support escaping of the delimiter, so be careful in cases where the delimiter is a valid character in the underlying field. The delimiter does not need to be only one character.

</div>

</div>

<div class="attribute">

max_length

This is an optional argument which validates that the array does not exceed the stated length.

</div>

<div class="attribute">

min_length

This is an optional argument which validates that the array reaches at least the stated length.

</div>

<div class="admonition">

User friendly forms

`SimpleArrayField` is not particularly user friendly in most cases, however it is a useful way to format data from a client-side widget for submission to the server.

</div>

</div>

### `SplitArrayField`

<div class="SplitArrayField(base_field, size, remove_trailing_nulls=False)">

This field handles arrays by reproducing the underlying field a fixed number of times.

<div class="attribute">

base_field

This is a required argument. It specifies the form field to be repeated.

</div>

<div class="attribute">

size

This is the fixed number of times the underlying field will be used.

</div>

<div class="attribute">

remove_trailing_nulls

By default, this is set to `False`. When `False`, each value from the repeated fields is stored. When set to `True`, any trailing values which are blank will be stripped from the result. If the underlying field has `required=True`, but `remove_trailing_nulls` is `True`, then null values are only allowed at the end, and will be stripped.

Some examples:

    SplitArrayField(IntegerField(required=True), size=3, remove_trailing_nulls=False)

    ["1", "2", "3"]  # -> [1, 2, 3]
    ["1", "2", ""]  # -> ValidationError - third entry required.
    ["1", "", "3"]  # -> ValidationError - second entry required.
    ["", "2", ""]  # -> ValidationError - first and third entries required.

    SplitArrayField(IntegerField(required=False), size=3, remove_trailing_nulls=False)

    ["1", "2", "3"]  # -> [1, 2, 3]
    ["1", "2", ""]  # -> [1, 2, None]
    ["1", "", "3"]  # -> [1, None, 3]
    ["", "2", ""]  # -> [None, 2, None]

    SplitArrayField(IntegerField(required=True), size=3, remove_trailing_nulls=True)

    ["1", "2", "3"]  # -> [1, 2, 3]
    ["1", "2", ""]  # -> [1, 2]
    ["1", "", "3"]  # -> ValidationError - second entry required.
    ["", "2", ""]  # -> ValidationError - first entry required.

    SplitArrayField(IntegerField(required=False), size=3, remove_trailing_nulls=True)

    ["1", "2", "3"]  # -> [1, 2, 3]
    ["1", "2", ""]  # -> [1, 2]
    ["1", "", "3"]  # -> [1, None, 3]
    ["", "2", ""]  # -> [None, 2]

</div>

</div>

### `HStoreField`

<div class="HStoreField">

A field which accepts JSON encoded data for an `~django.contrib.postgres.fields.HStoreField`. It casts all values (except nulls) to strings. It is represented by an HTML `<textarea>`.

<div class="admonition">

User friendly forms

`HStoreField` is not particularly user friendly in most cases, however it is a useful way to format data from a client-side widget for submission to the server.

</div>

<div class="note">

<div class="title">

Note

</div>

On occasions it may be useful to require or restrict the keys which are valid for a given field. This can be done using the `~django.contrib.postgres.validators.KeysValidator`.

</div>

</div>

### Range Fields

This group of fields all share similar functionality for accepting range data. They are based on `~django.forms.MultiValueField`. They treat one omitted value as an unbounded range. They also validate that the lower bound is not greater than the upper bound. All of these fields use `~django.contrib.postgres.forms.RangeWidget`.

#### `IntegerRangeField`

<div class="IntegerRangeField">

Based on `~django.forms.IntegerField` and translates its input into `django.db.backends.postgresql.psycopg_any.NumericRange`. Default for `~django.contrib.postgres.fields.IntegerRangeField` and `~django.contrib.postgres.fields.BigIntegerRangeField`.

</div>

#### `DecimalRangeField`

<div class="DecimalRangeField">

Based on `~django.forms.DecimalField` and translates its input into `django.db.backends.postgresql.psycopg_any.NumericRange`. Default for `~django.contrib.postgres.fields.DecimalRangeField`.

</div>

#### `DateTimeRangeField`

<div class="DateTimeRangeField">

Based on `~django.forms.DateTimeField` and translates its input into `django.db.backends.postgresql.psycopg_any.DateTimeTZRange`. Default for `~django.contrib.postgres.fields.DateTimeRangeField`.

</div>

#### `DateRangeField`

<div class="DateRangeField">

Based on `~django.forms.DateField` and translates its input into `django.db.backends.postgresql.psycopg_any.DateRange`. Default for `~django.contrib.postgres.fields.DateRangeField`.

</div>

## Widgets

### `RangeWidget`

<div class="RangeWidget(base_widget, attrs=None)">

Widget used by all of the range fields. Based on `~django.forms.MultiWidget`.

`~RangeWidget` has one required argument:

<div class="attribute">

base_widget

A `~RangeWidget` comprises a 2-tuple of `base_widget`.

</div>

<div class="method">

decompress(value)

Takes a single "compressed" value of a field, for example a `~django.contrib.postgres.fields.DateRangeField`, and returns a tuple representing a lower and upper bound.

</div>

</div>
