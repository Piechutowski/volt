# PostgreSQL specific database constraints

<div class="module" synopsis="PostgreSQL specific database constraint">

django.contrib.postgres.constraints

</div>

PostgreSQL supports additional data integrity constraints available from the `django.contrib.postgres.constraints` module. They are added in the model `Meta.constraints <django.db.models.Options.constraints>` option.

## `ExclusionConstraint`

<div class="ExclusionConstraint(*, name, expressions, index_type=None, condition=None, deferrable=None, include=None, violation_error_code=None, violation_error_message=None)">

Creates an exclusion constraint in the database. Internally, PostgreSQL implements exclusion constraints using indexes. The default index type is [GiST](https://www.postgresql.org/docs/current/gist.html). To use them, you need to activate the [btree_gist extension](https://www.postgresql.org/docs/current/btree-gist.html) on PostgreSQL. You can install it using the `~django.contrib.postgres.operations.BtreeGistExtension` migration operation.

If you attempt to insert a new row that conflicts with an existing row, an `~django.db.IntegrityError` is raised. Similarly, when update conflicts with an existing row.

Exclusion constraints are checked during the `model validation
<validating-objects>`.

</div>

### `name`

<div class="attribute">

ExclusionConstraint.name

</div>

See `.BaseConstraint.name`.

### `expressions`

<div class="attribute">

ExclusionConstraint.expressions

</div>

An iterable of 2-tuples. The first element is an expression or string. The second element is an SQL operator represented as a string. To avoid typos, you may use `~django.contrib.postgres.fields.RangeOperators` which maps the operators with strings. For example:

    expressions = [
        ("timespan", RangeOperators.ADJACENT_TO),
        (F("room"), RangeOperators.EQUAL),
    ]

<div class="admonition">

Restrictions on operators.

Only commutative operators can be used in exclusion constraints.

</div>

The `OpClass() <django.contrib.postgres.indexes.OpClass>` expression can be used to specify a custom [operator class](https://www.postgresql.org/docs/current/indexes-opclass.html) for the constraint expressions. For example:

    expressions = [
        (OpClass("circle", name="circle_ops"), RangeOperators.OVERLAPS),
    ]

creates an exclusion constraint on `circle` using `circle_ops`.

### `index_type`

<div class="attribute">

ExclusionConstraint.index_type

</div>

The index type of the constraint. Accepted values are `GiST`, `Hash`, or `SPGiST`. Matching is case insensitive. If not provided, the default index type is `GIST`.

<div class="versionchanged">

6.1

Support for exclusion constraints using Hash indexes was added.

</div>

### `condition`

<div class="attribute">

ExclusionConstraint.condition

</div>

A `~django.db.models.Q` object that specifies the condition to restrict a constraint to a subset of rows. For example, `condition=Q(cancelled=False)`.

These conditions have the same database restrictions as `django.db.models.Index.condition`.

### `deferrable`

<div class="attribute">

ExclusionConstraint.deferrable

</div>

Set this parameter to create a deferrable exclusion constraint. Accepted values are `Deferrable.DEFERRED` or `Deferrable.IMMEDIATE`. For example:

    from django.contrib.postgres.constraints import ExclusionConstraint
    from django.contrib.postgres.fields import RangeOperators
    from django.db.models import Deferrable

    ExclusionConstraint(
        name="exclude_overlapping_deferred",
        expressions=[
            ("timespan", RangeOperators.OVERLAPS),
        ],
        deferrable=Deferrable.DEFERRED,
    )

By default constraints are not deferred. A deferred constraint will not be enforced until the end of the transaction. An immediate constraint will be enforced immediately after every command.

<div class="warning">

<div class="title">

Warning

</div>

Deferred exclusion constraints may lead to a [performance penalty](https://www.postgresql.org/docs/current/sql-createtable.html#id-1.9.3.85.9.4).

</div>

### `include`

<div class="attribute">

ExclusionConstraint.include

</div>

A list or tuple of the names of the fields to be included in the covering exclusion constraint as non-key columns. This allows index-only scans to be used for queries that select only included fields (`~ExclusionConstraint.include`) and filter only by indexed fields (`~ExclusionConstraint.expressions`).

`include` is supported for GiST and SP-GiST indexes.

### `violation_error_code`

<div class="attribute">

ExclusionConstraint.violation_error_code

</div>

The error code used when `ValidationError` is raised during `model validation <validating-objects>`. Defaults to `None`.

### `violation_error_message`

The error message used when `ValidationError` is raised during `model validation <validating-objects>`. Defaults to `.BaseConstraint.violation_error_message`.

### Examples

The following example restricts overlapping reservations in the same room, not taking canceled reservations into account:

    from django.contrib.postgres.constraints import ExclusionConstraint
    from django.contrib.postgres.fields import DateTimeRangeField, RangeOperators
    from django.db import models
    from django.db.models import Q


    class Room(models.Model):
        number = models.IntegerField()


    class Reservation(models.Model):
        room = models.ForeignKey("Room", on_delete=models.CASCADE)
        timespan = DateTimeRangeField()
        cancelled = models.BooleanField(default=False)

        class Meta:
            constraints = [
                ExclusionConstraint(
                    name="exclude_overlapping_reservations",
                    expressions=[
                        ("timespan", RangeOperators.OVERLAPS),
                        ("room", RangeOperators.EQUAL),
                    ],
                    condition=Q(cancelled=False),
                ),
            ]

In case your model defines a range using two fields, instead of the native PostgreSQL range types, you should write an expression that uses the equivalent function (e.g. `TsTzRange()`), and use the delimiters for the field. Most often, the delimiters will be `'[)'`, meaning that the lower bound is inclusive and the upper bound is exclusive. You may use the `~django.contrib.postgres.fields.RangeBoundary` that provides an expression mapping for the [range boundaries \<https://www.postgresql.org/docs/ current/rangetypes.html#RANGETYPES-INCLUSIVITY\>](). For example:

    from django.contrib.postgres.constraints import ExclusionConstraint
    from django.contrib.postgres.fields import (
        DateTimeRangeField,
        RangeBoundary,
        RangeOperators,
    )
    from django.db import models
    from django.db.models import Func, Q


    class TsTzRange(Func):
        function = "TSTZRANGE"
        output_field = DateTimeRangeField()


    class Reservation(models.Model):
        room = models.ForeignKey("Room", on_delete=models.CASCADE)
        start = models.DateTimeField()
        end = models.DateTimeField()
        cancelled = models.BooleanField(default=False)

        class Meta:
            constraints = [
                ExclusionConstraint(
                    name="exclude_overlapping_reservations",
                    expressions=[
                        (
                            TsTzRange("start", "end", RangeBoundary()),
                            RangeOperators.OVERLAPS,
                        ),
                        ("room", RangeOperators.EQUAL),
                    ],
                    condition=Q(cancelled=False),
                ),
            ]
