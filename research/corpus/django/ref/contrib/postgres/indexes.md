# PostgreSQL specific model indexes

<div class="module">

django.contrib.postgres.indexes

</div>

The following are PostgreSQL specific `indexes </ref/models/indexes>` available from the `django.contrib.postgres.indexes` module.

## `BloomIndex`

<div class="BloomIndex(*expressions, length=None, columns=(), **options)">

Creates a [bloom]() index.

To use this index access you need to activate the [bloom]() extension on PostgreSQL. You can install it using the `~django.contrib.postgres.operations.BloomExtension` migration operation.

Provide an integer number of bits from 1 to 4096 to the `length` parameter to specify the length of each index entry. PostgreSQL's default is 80.

The `columns` argument takes a tuple or list of up to 32 values that are integer number of bits from 1 to 4095.

</div>

## `BrinIndex`

<div class="BrinIndex(*expressions, autosummarize=None, pages_per_range=None, **options)">

Creates a [BRIN index](https://www.postgresql.org/docs/current/brin.html).

Set the `autosummarize` parameter to `True` to enable [automatic summarization]() to be performed by autovacuum.

The `pages_per_range` argument takes a positive integer.

</div>

## `BTreeIndex`

<div class="BTreeIndex(*expressions, fillfactor=None, deduplicate_items=None, **options)">

Creates a B-Tree index.

Provide an integer value from 10 to 100 to the [fillfactor]() parameter to tune how packed the index pages will be. PostgreSQL's default is 90.

Provide a boolean value to the [deduplicate_items]() parameter to control whether deduplication is enabled. PostgreSQL enables deduplication by default.

</div>

## `GinIndex`

<div class="GinIndex(*expressions, fastupdate=None, gin_pending_list_limit=None, **options)">

Creates a [gin index](https://www.postgresql.org/docs/current/gin.html).

To use this index on data types not in the [built-in operator classes](https://www.postgresql.org/docs/current/gin.html#GIN-BUILTIN-OPCLASSES), you need to activate the [btree_gin extension](https://www.postgresql.org/docs/current/btree-gin.html) on PostgreSQL. You can install it using the `~django.contrib.postgres.operations.BtreeGinExtension` migration operation.

Set the `fastupdate` parameter to `False` to disable the [GIN Fast Update Technique]() that's enabled by default in PostgreSQL.

Provide an integer number of kilobytes to the [gin_pending_list_limit]() parameter to tune the maximum size of the GIN pending list which is used when `fastupdate` is enabled.

</div>

## `GistIndex`

<div class="GistIndex(*expressions, buffering=None, fillfactor=None, **options)">

Creates a [GiST index](https://www.postgresql.org/docs/current/gist.html). These indexes are automatically created on spatial fields with `spatial_index=True
<django.contrib.gis.db.models.BaseSpatialField.spatial_index>`. They're also useful on other types, such as `~django.contrib.postgres.fields.HStoreField` or the `range
fields <range-fields>`.

To use this index on data types not in the built-in [gist operator classes](https://www.postgresql.org/docs/current/gist.html#GIST-BUILTIN-OPCLASSES), you need to activate the [btree_gist extension](https://www.postgresql.org/docs/current/btree-gist.html) on PostgreSQL. You can install it using the `~django.contrib.postgres.operations.BtreeGistExtension` migration operation.

Set the `buffering` parameter to `True` or `False` to manually enable or disable [buffering build]() of the index.

Provide an integer value from 10 to 100 to the [fillfactor]() parameter to tune how packed the index pages will be. PostgreSQL's default is 90.

</div>

## `HashIndex`

<div class="HashIndex(*expressions, fillfactor=None, **options)">

Creates a hash index.

Provide an integer value from 10 to 100 to the [fillfactor]() parameter to tune how packed the index pages will be. PostgreSQL's default is 90.

</div>

## `SpGistIndex`

<div class="SpGistIndex(*expressions, fillfactor=None, **options)">

Creates an [SP-GiST index](https://www.postgresql.org/docs/current/spgist.html).

Provide an integer value from 10 to 100 to the [fillfactor]() parameter to tune how packed the index pages will be. PostgreSQL's default is 90.

</div>

## `OpClass()` expressions

<div class="OpClass(expression, name)">

An `OpClass()` expression represents the `expression` with a custom [operator class]() that can be used to define functional indexes, functional unique constraints, or exclusion constraints. To use it, you need to add `'django.contrib.postgres'` in your `INSTALLED_APPS`. Set the `name` parameter to the name of the [operator class]().

For example:

    Index(
        OpClass(Lower("username"), name="varchar_pattern_ops"),
        name="lower_username_idx",
    )

creates an index on `Lower('username')` using `varchar_pattern_ops`. :

    UniqueConstraint(
        OpClass(Upper("description"), name="text_pattern_ops"),
        name="upper_description_unique",
    )

creates a unique constraint on `Upper('description')` using `text_pattern_ops`. :

    ExclusionConstraint(
        name="exclude_overlapping_ops",
        expressions=[
            (OpClass("circle", name="circle_ops"), RangeOperators.OVERLAPS),
        ],
    )

creates an exclusion constraint on `circle` using `circle_ops`.

</div>
