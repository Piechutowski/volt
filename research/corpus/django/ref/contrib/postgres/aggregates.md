# PostgreSQL specific aggregation functions

<div class="module" synopsis="PostgreSQL specific aggregation functions">

django.contrib.postgres.aggregates

</div>

These functions are available from the `django.contrib.postgres.aggregates` module. They are described in more detail in the [PostgreSQL docs](https://www.postgresql.org/docs/current/functions-aggregate.html).

<div class="note">

<div class="title">

Note

</div>

All functions come without default aliases, so you must explicitly provide one. For example:

``` pycon
>>> SomeModel.objects.aggregate(arr=ArrayAgg("somefield"))
{'arr': [0, 1, 2]}
```

</div>

<div class="admonition">

Common aggregate options

All aggregates have the `filter <aggregate-filter>` keyword argument and most also have the `default <aggregate-default>` keyword argument.

</div>

## General-purpose aggregation functions

### `ArrayAgg`

<div class="ArrayAgg(expression, distinct=False, filter=None, default=None, order_by=(), **extra)">

Returns a list of values, including nulls, concatenated into an array, or `default` if there are no values.

<div class="attribute">

distinct

An optional boolean argument that determines if array values will be distinct. Defaults to `False`.

</div>

<div class="attribute">

order_by

An optional string of a field name (with an optional `"-"` prefix which indicates descending order) or an expression (or a tuple or list of strings and/or expressions) that specifies the ordering of the elements in the result list.

Examples:

    from django.db.models import F

    ArrayAgg("a_field", order_by="-some_field")
    ArrayAgg("a_field", order_by=F("some_field").desc())

</div>

</div>

### `BitAnd`

<div class="BitAnd(expression, filter=None, default=None, **extra)">

Returns an `int` of the bitwise `AND` of all non-null input values, or `default` if all values are null.

<div class="deprecated">

6.1

This class is deprecated in favor of the generally available `~django.db.models.BitAnd` class.

</div>

</div>

### `BitOr`

<div class="BitOr(expression, filter=None, default=None, **extra)">

Returns an `int` of the bitwise `OR` of all non-null input values, or `default` if all values are null.

<div class="deprecated">

6.1

This class is deprecated in favor of the generally available `~django.db.models.BitOr` class.

</div>

</div>

### `BitXor`

<div class="BitXor(expression, filter=None, default=None, **extra)">

Returns an `int` of the bitwise `XOR` of all non-null input values, or `default` if all values are null.

<div class="deprecated">

6.1

This class is deprecated in favor of the generally available `~django.db.models.BitXor` class.

</div>

</div>

### `BoolAnd`

<div class="BoolAnd(expression, filter=None, default=None, **extra)">

Returns `True`, if all input values are true, `default` if all values are null or if there are no values, otherwise `False`.

Usage example:

    class Comment(models.Model):
        body = models.TextField()
        published = models.BooleanField()
        rank = models.IntegerField()

``` pycon
>>> from django.db.models import Q
>>> from django.contrib.postgres.aggregates import BoolAnd
>>> Comment.objects.aggregate(booland=BoolAnd("published"))
{'booland': False}
>>> Comment.objects.aggregate(booland=BoolAnd(Q(rank__lt=100)))
{'booland': True}
```

</div>

### `BoolOr`

<div class="BoolOr(expression, filter=None, default=None, **extra)">

Returns `True` if at least one input value is true, `default` if all values are null or if there are no values, otherwise `False`.

Usage example:

    class Comment(models.Model):
        body = models.TextField()
        published = models.BooleanField()
        rank = models.IntegerField()

``` pycon
>>> from django.db.models import Q
>>> from django.contrib.postgres.aggregates import BoolOr
>>> Comment.objects.aggregate(boolor=BoolOr("published"))
{'boolor': True}
>>> Comment.objects.aggregate(boolor=BoolOr(Q(rank__gt=2)))
{'boolor': False}
```

</div>

### `JSONBAgg`

<div class="JSONBAgg(expressions, distinct=False, filter=None, default=None, order_by=(), **extra)">

Returns the input values as a `JSON` array, or `default` if there are no values. You can query the result using `key and index lookups
<jsonfield.key>`.

<div class="attribute">

distinct

An optional boolean argument that determines if array values will be distinct. Defaults to `False`.

</div>

<div class="attribute">

order_by

An optional string of a field name (with an optional `"-"` prefix which indicates descending order) or an expression (or a tuple or list of strings and/or expressions) that specifies the ordering of the elements in the result list.

Examples are the same as for `ArrayAgg.order_by`.

</div>

Usage example:

    class Room(models.Model):
        number = models.IntegerField(unique=True)


    class HotelReservation(models.Model):
        room = models.ForeignKey("Room", on_delete=models.CASCADE)
        start = models.DateTimeField()
        end = models.DateTimeField()
        requirements = models.JSONField(blank=True, null=True)

``` pycon
>>> from django.contrib.postgres.aggregates import JSONBAgg
>>> Room.objects.annotate(
...     requirements=JSONBAgg(
...         "hotelreservation__requirements",
...         order_by="-hotelreservation__start",
...     )
... ).filter(requirements__0__sea_view=True).values("number", "requirements")
<QuerySet [{'number': 102, 'requirements': [
    {'parking': False, 'sea_view': True, 'double_bed': False},
    {'parking': True, 'double_bed': True}
]}]>
```

</div>

### `StringAgg`

<div class="StringAgg(expression, delimiter, distinct=False, filter=None, default=None, order_by=())">

<div class="deprecated">

6.0

The PostgreSQL `StringAgg` class is deprecated in favor of the generally available `~django.db.models.StringAgg` class.

</div>

Returns the input values concatenated into a string, separated by the `delimiter` string, or `default` if there are no values.

<div class="attribute">

delimiter

Required argument. A string, `~django.db.models.Value`, or expression representing the string for separating values. For example, `Value(",")`.

<div class="versionadded">

6.0

Support for providing a `Value` or expression rather than a string was added.

</div>

<div class="deprecated">

6.0

Support for providing a string is deprecated.

</div>

</div>

<div class="attribute">

distinct

An optional boolean argument that determines if concatenated values will be distinct. Defaults to `False`.

</div>

<div class="attribute">

order_by

An optional string of a field name (with an optional `"-"` prefix which indicates descending order) or an expression (or a tuple or list of strings and/or expressions) that specifies the ordering of the elements in the result string.

Examples are the same as for `ArrayAgg.order_by`.

</div>

Usage example:

    class Publication(models.Model):
        title = models.CharField(max_length=30)


    class Article(models.Model):
        headline = models.CharField(max_length=100)
        publications = models.ManyToManyField(Publication)

``` pycon
>>> article = Article.objects.create(headline="NASA uses Python")
>>> article.publications.create(title="The Python Journal")
<Publication: Publication object (1)>
>>> article.publications.create(title="Science News")
<Publication: Publication object (2)>
>>> from django.contrib.postgres.aggregates import StringAgg
>>> Article.objects.annotate(
...     publication_names=StringAgg(
...         "publications__title",
...         delimiter=", ",
...         order_by="publications__title",
...     )
... ).values("headline", "publication_names")
<QuerySet [{
    'headline': 'NASA uses Python', 'publication_names': 'Science News, The Python Journal'
}]>
```

</div>

## Aggregate functions for statistics

### `y` and `x`

The arguments `y` and `x` for all these functions can be the name of a field or an expression returning a numeric data. Both are required.

### `Corr`

<div class="Corr(y, x, filter=None, default=None)">

Returns the correlation coefficient as a `float`, or `default` if there aren't any matching rows.

</div>

### `CovarPop`

<div class="CovarPop(y, x, sample=False, filter=None, default=None)">

Returns the population covariance as a `float`, or `default` if there aren't any matching rows.

<div class="attribute">

sample

Optional. By default `CovarPop` returns the general population covariance. However, if `sample=True`, the return value will be the sample population covariance.

</div>

</div>

### `RegrAvgX`

<div class="RegrAvgX(y, x, filter=None, default=None)">

Returns the average of the independent variable (`sum(x)/N`) as a `float`, or `default` if there aren't any matching rows.

</div>

### `RegrAvgY`

<div class="RegrAvgY(y, x, filter=None, default=None)">

Returns the average of the dependent variable (`sum(y)/N`) as a `float`, or `default` if there aren't any matching rows.

</div>

### `RegrCount`

<div class="RegrCount(y, x, filter=None)">

Returns an `int` of the number of input rows in which both expressions are not null.

<div class="note">

<div class="title">

Note

</div>

The `default` argument is not supported.

</div>

</div>

### `RegrIntercept`

<div class="RegrIntercept(y, x, filter=None, default=None)">

Returns the y-intercept of the least-squares-fit linear equation determined by the `(x, y)` pairs as a `float`, or `default` if there aren't any matching rows.

</div>

### `RegrR2`

<div class="RegrR2(y, x, filter=None, default=None)">

Returns the square of the correlation coefficient as a `float`, or `default` if there aren't any matching rows.

</div>

### `RegrSlope`

<div class="RegrSlope(y, x, filter=None, default=None)">

Returns the slope of the least-squares-fit linear equation determined by the `(x, y)` pairs as a `float`, or `default` if there aren't any matching rows.

</div>

### `RegrSXX`

<div class="RegrSXX(y, x, filter=None, default=None)">

Returns `sum(x^2) - sum(x)^2/N` ("sum of squares" of the independent variable) as a `float`, or `default` if there aren't any matching rows.

</div>

### `RegrSXY`

<div class="RegrSXY(y, x, filter=None, default=None)">

Returns `sum(x*y) - sum(x) * sum(y)/N` ("sum of products" of independent times dependent variable) as a `float`, or `default` if there aren't any matching rows.

</div>

### `RegrSYY`

<div class="RegrSYY(y, x, filter=None, default=None)">

Returns `sum(y^2) - sum(y)^2/N` ("sum of squares" of the dependent variable) as a `float`, or `default` if there aren't any matching rows.

</div>

## Usage examples

We will use this example table:

``` text
| FIELD1 | FIELD2 | FIELD3 |
|--------|--------|--------|
|    foo |      1 |     13 |
|    bar |      2 | (null) |
|   test |      3 |     13 |
```

Here's some examples of some of the general-purpose aggregation functions:

``` pycon
>>> TestModel.objects.aggregate(result=StringAgg("field1", delimiter=";"))
{'result': 'foo;bar;test'}
>>> TestModel.objects.aggregate(result=ArrayAgg("field2"))
{'result': [1, 2, 3]}
>>> TestModel.objects.aggregate(result=ArrayAgg("field1"))
{'result': ['foo', 'bar', 'test']}
```

The next example shows the usage of statistical aggregate functions. The underlying math will be not described (you can read about this, for example, at [wikipedia](https://en.wikipedia.org/wiki/Regression_analysis)):

``` pycon
>>> TestModel.objects.aggregate(count=RegrCount(y="field3", x="field2"))
{'count': 2}
>>> TestModel.objects.aggregate(
...     avgx=RegrAvgX(y="field3", x="field2"), avgy=RegrAvgY(y="field3", x="field2")
... )
{'avgx': 2, 'avgy': 13}
```
