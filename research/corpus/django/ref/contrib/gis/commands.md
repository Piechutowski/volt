# GeoDjango Management Commands

## `inspectdb`

<div class="describe">

django-admin inspectdb

</div>

When `django.contrib.gis` is in your `INSTALLED_APPS`, the `inspectdb` management command is overridden with one from GeoDjango. The overridden command is spatially-aware, and places geometry fields in the auto-generated model definition, where appropriate.

## `ogrinspect`

<div class="django-admin">

ogrinspect data_source model_name

</div>

The `ogrinspect` management command will inspect the given OGR-compatible `~django.contrib.gis.gdal.DataSource` (e.g., a shapefile) and will output a GeoDjango model with the given model name. There's a detailed example of using `ogrinspect` `in the tutorial <ogrinspect-intro>`.

<div class="django-admin-option">

--blank BLANK

Use a comma separated list of OGR field names to add the `blank=True` keyword option to the field definition. Set with `true` to apply to all applicable fields.

</div>

<div class="django-admin-option">

--decimal DECIMAL

Use a comma separated list of OGR float fields to generate `~django.db.models.DecimalField` instead of the default `~django.db.models.FloatField`. Set to `true` to apply to all OGR float fields.

</div>

<div class="django-admin-option">

--geom-name GEOM_NAME

Specifies the model attribute name to use for the geometry field. Defaults to `'geom'`.

</div>

<div class="django-admin-option">

--layer LAYER_KEY

The key for specifying which layer in the OGR `~django.contrib.gis.gdal.DataSource` source to use. Defaults to 0 (the first layer). May be an integer or a string identifier for the `~django.contrib.gis.gdal.Layer`. When inspecting databases, `layer` is generally the table name you want to inspect.

</div>

<div class="django-admin-option">

--mapping

Automatically generate a mapping dictionary for use with `~django.contrib.gis.utils.LayerMapping`.

</div>

<div class="django-admin-option">

--multi-geom

When generating the geometry field, treat it as a geometry collection. For example, if this setting is enabled then a `~django.contrib.gis.db.models.MultiPolygonField` will be placed in the generated model rather than `~django.contrib.gis.db.models.PolygonField`.

</div>

<div class="django-admin-option">

--name-field NAME_FIELD

Generates a `__str__()` method on the model that returns the given field name.

</div>

<div class="django-admin-option">

--no-imports

Suppresses the `from django.contrib.gis.db import models` import statement.

</div>

<div class="django-admin-option">

--null NULL

Use a comma separated list of OGR field names to add the `null=True` keyword option to the field definition. Set with `true` to apply to all applicable fields.

</div>

<div class="django-admin-option">

--srid SRID

The SRID to use for the geometry field. If not set, `ogrinspect` attempts to automatically determine of the SRID of the data source.

</div>
