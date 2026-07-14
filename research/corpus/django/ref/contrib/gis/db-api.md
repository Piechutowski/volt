# GeoDjango Database API

## Spatial Backends

<div class="module" synopsis="GeoDjango's spatial database backends.">

django.contrib.gis.db.backends

</div>

GeoDjango currently provides the following spatial database backends:

- `django.contrib.gis.db.backends.postgis`
- `django.contrib.gis.db.backends.mysql`
- `django.contrib.gis.db.backends.oracle`
- `django.contrib.gis.db.backends.spatialite`

### MySQL Spatial Limitations

Django supports spatial functions operating on real geometries available in modern MySQL versions. However, the spatial functions are not as rich as other backends like PostGIS.

### Raster Support

`RasterField` is currently only implemented for the PostGIS backend. Spatial lookups are available for raster fields, but spatial database functions and aggregates aren't implemented for raster fields.

## Creating and Saving Models with Geometry Fields

Here is an example of how to create a geometry object (assuming the `Zipcode` model):

``` pycon
>>> from zipcode.models import Zipcode
>>> z = Zipcode(code=77096, poly="POLYGON(( 10 10, 10 20, 20 20, 20 15, 10 10))")
>>> z.save()
```

`~django.contrib.gis.geos.GEOSGeometry` objects may also be used to save geometric models:

``` pycon
>>> from django.contrib.gis.geos import GEOSGeometry
>>> poly = GEOSGeometry("POLYGON(( 10 10, 10 20, 20 20, 20 15, 10 10))")
>>> z = Zipcode(code=77096, poly=poly)
>>> z.save()
```

Moreover, if the `GEOSGeometry` is in a different coordinate system (has a different SRID value) than that of the field, then it will be implicitly transformed into the SRID of the model's field, using the spatial database's transform procedure:

``` pycon
>>> poly_3084 = GEOSGeometry(
...     "POLYGON(( 10 10, 10 20, 20 20, 20 15, 10 10))", srid=3084
... )  # SRID 3084 is 'NAD83(HARN) / Texas Centric Lambert Conformal'
>>> z = Zipcode(code=78212, poly=poly_3084)
>>> z.save()
>>> from django.db import connection
>>> print(
...     connection.queries[-1]["sql"]
... )  # printing the last SQL statement executed (requires DEBUG=True)
INSERT INTO "geoapp_zipcode" ("code", "poly") VALUES (78212, ST_Transform(ST_GeomFromWKB('\\001 ... ', 3084), 4326))
```

Thus, geometry parameters may be passed in using the `GEOSGeometry` object, WKT (Well Known Text[^1]), HEXEWKB (PostGIS specific -- a WKB geometry in hexadecimal[^2]), and GeoJSON (see `7946`). Essentially, if the input is not a `GEOSGeometry` object, the geometry field will attempt to create a `GEOSGeometry` instance from the input.

For more information creating `~django.contrib.gis.geos.GEOSGeometry` objects, refer to the `GEOS tutorial <geos-tutorial>`.

## Creating and Saving Models with Raster Fields

When creating raster models, the raster field will implicitly convert the input into a `~django.contrib.gis.gdal.GDALRaster` using lazy-evaluation. The raster field will therefore accept any input that is accepted by the `~django.contrib.gis.gdal.GDALRaster` constructor.

Here is an example of how to create a raster object from a raster file `volcano.tif` (assuming the `Elevation` model):

``` pycon
>>> from elevation.models import Elevation
>>> dem = Elevation(name="Volcano", rast="/path/to/raster/volcano.tif")
>>> dem.save()
```

`~django.contrib.gis.gdal.GDALRaster` objects may also be used to save raster models:

``` pycon
>>> from django.contrib.gis.gdal import GDALRaster
>>> rast = GDALRaster(
...     {
...         "width": 10,
...         "height": 10,
...         "name": "Canyon",
...         "srid": 4326,
...         "scale": [0.1, -0.1],
...         "bands": [{"data": range(100)}],
...     }
... )
>>> dem = Elevation(name="Canyon", rast=rast)
>>> dem.save()
```

Note that this equivalent to:

``` pycon
>>> dem = Elevation.objects.create(
...     name="Canyon",
...     rast={
...         "width": 10,
...         "height": 10,
...         "name": "Canyon",
...         "srid": 4326,
...         "scale": [0.1, -0.1],
...         "bands": [{"data": range(100)}],
...     },
... )
```

## Spatial Lookups

GeoDjango's lookup types may be used with any manager method like `filter()`, `exclude()`, etc. However, the lookup types unique to GeoDjango are only available on spatial fields.

Filters on 'normal' fields (e.g. `~django.db.models.CharField`) may be chained with those on geographic fields. Geographic lookups accept geometry and raster input on both sides and input types can be mixed freely.

The general structure of geographic lookups is described below. A complete reference can be found in the `spatial lookup reference<spatial-lookups>`.

### Geometry Lookups

Geographic queries with geometries take the following general form (assuming the `Zipcode` model used in the `/ref/contrib/gis/model-api`):

``` text
>>> qs = Zipcode.objects.filter(<field>__<lookup_type>=<parameter>)
>>> qs = Zipcode.objects.exclude(...)
```

For example:

``` pycon
>>> qs = Zipcode.objects.filter(poly__contains=pnt)
>>> qs = Elevation.objects.filter(poly__contains=rst)
```

In this case, `poly` is the geographic field, `contains <gis-contains>` is the spatial lookup type, `pnt` is the parameter (which may be a `~django.contrib.gis.geos.GEOSGeometry` object or a string of GeoJSON , WKT, or HEXEWKB), and `rst` is a `~django.contrib.gis.gdal.GDALRaster` object.

### Raster Lookups

The raster lookup syntax is similar to the syntax for geometries. The only difference is that a band index can be specified as additional input. If no band index is specified, the first band is used by default (index `0`). In that case the syntax is identical to the syntax for geometry lookups.

To specify the band index, an additional parameter can be specified on both sides of the lookup. On the left hand side, the double underscore syntax is used to pass a band index. On the right hand side, a tuple of the raster and band index can be specified.

This results in the following general form for lookups involving rasters (assuming the `Elevation` model used in the `/ref/contrib/gis/model-api`):

``` text
>>> qs = Elevation.objects.filter(<field>__<lookup_type>=<parameter>)
>>> qs = Elevation.objects.filter(<field>__<band_index>__<lookup_type>=<parameter>)
>>> qs = Elevation.objects.filter(<field>__<lookup_type>=(<raster_input, <band_index>)
```

For example:

``` pycon
>>> qs = Elevation.objects.filter(rast__contains=geom)
>>> qs = Elevation.objects.filter(rast__contains=rst)
>>> qs = Elevation.objects.filter(rast__1__contains=geom)
>>> qs = Elevation.objects.filter(rast__contains=(rst, 1))
>>> qs = Elevation.objects.filter(rast__1__contains=(rst, 1))
```

On the left hand side of the example, `rast` is the geographic raster field and `contains <gis-contains>` is the spatial lookup type. On the right hand side, `geom` is a geometry input and `rst` is a `~django.contrib.gis.gdal.GDALRaster` object. The band index defaults to `0` in the first two queries and is set to `1` on the others.

While all spatial lookups can be used with raster objects on both sides, not all underlying operators natively accept raster input. For cases where the operator expects geometry input, the raster is automatically converted to a geometry. It's important to keep this in mind when interpreting the lookup results.

The type of raster support is listed for all lookups in the `compatibility
table <spatial-lookup-compatibility>`. Lookups involving rasters are currently only available for the PostGIS backend.

## Distance Queries

### Introduction

Distance calculations with spatial data is tricky because, unfortunately, the Earth is not flat. Some distance queries with fields in a geographic coordinate system may have to be expressed differently because of limitations in PostGIS. Please see the `selecting-an-srid` section for more details.

### Distance Lookups

*Availability*: PostGIS, MariaDB, MySQL, Oracle, SpatiaLite, PGRaster (Native)

The following distance lookups are available:

- `distance_lt`
- `distance_lte`
- `distance_gt`
- `distance_gte`
- `dwithin` (except MariaDB and MySQL)

<div class="note">

<div class="title">

Note

</div>

For *measuring*, rather than querying on distances, use the `~django.contrib.gis.db.models.functions.Distance` function.

</div>

Distance lookups take a tuple parameter comprising:

1.  A geometry or raster to base calculations from; and
2.  A number or `~django.contrib.gis.measure.Distance` object containing the distance.

If a `~django.contrib.gis.measure.Distance` object is used, it may be expressed in any units (the SQL generated will use units converted to those of the field); otherwise, numeric parameters are assumed to be in the units of the field.

<div class="note">

<div class="title">

Note

</div>

In PostGIS, `ST_Distance_Sphere` does *not* limit the geometry types geographic distance queries are performed with.[^3] However, these queries may take a long time, as great-circle distances must be calculated on the fly for *every* row in the query. This is because the spatial index on traditional geometry fields cannot be used.

For much better performance on WGS84 distance queries, consider using `geography columns <geography-type>` in your database instead because they are able to use their spatial index in distance queries. You can tell GeoDjango to use a geography column by setting `geography=True` in your field definition.

</div>

For example, let's say we have a `SouthTexasCity` model (from the `GeoDjango distance tests <tests/gis_tests/distapp/models.py>` ) on a *projected* coordinate system valid for cities in southern Texas:

    from django.contrib.gis.db import models


    class SouthTexasCity(models.Model):
        name = models.CharField(max_length=30)
        # A projected coordinate system (only valid for South Texas!)
        # is used, units are in meters.
        point = models.PointField(srid=32140)

Then distance queries may be performed as follows:

``` pycon
>>> from django.contrib.gis.geos import GEOSGeometry
>>> from django.contrib.gis.measure import D  # ``D`` is a shortcut for ``Distance``
>>> from geoapp.models import SouthTexasCity
# Distances will be calculated from this point, which does not have to be projected.
>>> pnt = GEOSGeometry("POINT(-96.876369 29.905320)", srid=4326)
# If numeric parameter, units of field (meters in this case) are assumed.
>>> qs = SouthTexasCity.objects.filter(point__distance_lte=(pnt, 7000))
# Find all Cities within 7 km, > 20 miles away, and > 100 chains away (an obscure unit)
>>> qs = SouthTexasCity.objects.filter(point__distance_lte=(pnt, D(km=7)))
>>> qs = SouthTexasCity.objects.filter(point__distance_gte=(pnt, D(mi=20)))
>>> qs = SouthTexasCity.objects.filter(point__distance_gte=(pnt, D(chain=100)))
```

Raster queries work the same way by replacing the geometry field `point` with a raster field, or the `pnt` object with a raster object, or both. To specify the band index of a raster input on the right hand side, a 3-tuple can be passed to the lookup as follows:

``` pycon
>>> qs = SouthTexasCity.objects.filter(point__distance_gte=(rst, 2, D(km=7)))
```

Where the band with index 2 (the third band) of the raster `rst` would be used for the lookup.

## Compatibility Tables

### Spatial Lookups

The following table provides a summary of what spatial lookups are available for each spatial database backend. The PostGIS Raster (PGRaster) lookups are divided into the three categories described in the `raster lookup details
<spatial-lookup-raster>`: native support `N`, bilateral native support `B`, and geometry conversion support `C`.

<table>
<thead>
<tr class="header">
<th>Lookup Type</th>
<th>PostGIS</th>
<th>Oracle</th>
<th>MariaDB</th>
<th>MySQL<a href="#fn1" class="footnote-ref" id="fnref1" role="doc-noteref"><sup>1</sup></a></th>
<th>SpatiaLite</th>
<th>PGRaster</th>
</tr>
</thead>
<tbody>
<tr class="odd">
<td><p><code class="interpreted-text" role="lookup">bbcontains</code> <code class="interpreted-text" role="lookup">bboverlaps</code> <code class="interpreted-text" role="lookup">contained</code></p></td>
<td><p>X X X</p></td>
<td></td>
<td><p>X X X</p></td>
<td><p>X X X</p></td>
<td><p>X X X</p></td>
<td><p>N N N</p></td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="lookup">contains &lt;gis-contains&gt;</code> <code class="interpreted-text" role="lookup">contains_properly</code></p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>B B</p></td>
</tr>
<tr class="odd">
<td><p><code class="interpreted-text" role="lookup">coveredby</code> <code class="interpreted-text" role="lookup">covers</code> <code class="interpreted-text" role="lookup">crosses</code></p></td>
<td><p>X X X</p></td>
<td><p>X X</p></td>
<td><p>X (≥ 12.0.1)</p>
<p>X</p></td>
<td><p>X X X</p></td>
<td><p>X X X</p></td>
<td><p>B B C</p></td>
</tr>
<tr class="even">
<td><code class="interpreted-text" role="lookup">disjoint</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>B</td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="lookup">distance_gt</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>N</td>
</tr>
<tr class="even">
<td><code class="interpreted-text" role="lookup">distance_gte</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>N</td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="lookup">distance_lt</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>N</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="lookup">distance_lte</code> <code class="interpreted-text" role="lookup">dwithin</code></p></td>
<td><p>X X</p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X</p></td>
<td><p>N B</p></td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="lookup">equals</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>C</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="lookup">exact &lt;same_as&gt;</code> <code class="interpreted-text" role="lookup">geom_type</code></p></td>
<td><p>X X</p></td>
<td><p>X X (≥ 23c)</p></td>
<td><p>X X</p></td>
<td><p>X X</p></td>
<td><p>X X</p></td>
<td><p>B</p></td>
</tr>
<tr class="odd">
<td><p><code class="interpreted-text" role="lookup">intersects</code> <code class="interpreted-text" role="lookup">isempty</code> <code class="interpreted-text" role="lookup">isvalid</code> <code class="interpreted-text" role="lookup">num_dimensions</code></p></td>
<td><p>X X X X</p></td>
<td><p>X</p>
<p>X</p></td>
<td><p>X</p>
<p>X (≥ 12.0.1)</p></td>
<td><p>X</p>
<p>X</p></td>
<td><p>X X X X</p></td>
<td><p>B</p></td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="lookup">overlaps</code> <code class="interpreted-text" role="lookup">relate</code></p></td>
<td><p>X X</p></td>
<td><p>X X</p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X X</p></td>
<td><p>B C</p></td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="lookup">same_as</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>B</td>
</tr>
<tr class="even">
<td><code class="interpreted-text" role="lookup">touches</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>B</td>
</tr>
<tr class="odd">
<td><p><code class="interpreted-text" role="lookup">within</code> <code class="interpreted-text" role="lookup">left</code> <code class="interpreted-text" role="lookup">right</code> <code class="interpreted-text" role="lookup">overlaps_left</code> <code class="interpreted-text" role="lookup">overlaps_right</code> <code class="interpreted-text" role="lookup">overlaps_above</code> <code class="interpreted-text" role="lookup">overlaps_below</code> <code class="interpreted-text" role="lookup">strictly_above</code> <code class="interpreted-text" role="lookup">strictly_below</code></p></td>
<td><p>X X X X X X X X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>B C C B B C C C C</p></td>
</tr>
</tbody>
</table>
<aside id="footnotes" class="footnotes footnotes-end-of-document" role="doc-endnotes">
<hr />
<ol>
<li id="fn1"><p>Refer <code class="interpreted-text" role="ref">mysql-spatial-limitations</code> section for more details.<a href="#fnref1" class="footnote-back" role="doc-backlink">↩︎</a></p></li>
</ol>
</aside>

### Database functions

The following table provides a summary of what geography-specific database functions are available on each spatial backend.

<div class="currentmodule">

django.contrib.gis.db.models.functions

</div>

<table>
<thead>
<tr class="header">
<th>Function</th>
<th>PostGIS</th>
<th>Oracle</th>
<th>MariaDB</th>
<th>MySQL</th>
<th>SpatiaLite</th>
</tr>
</thead>
<tbody>
<tr class="odd">
<td><code class="interpreted-text" role="class">Area</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="class">AsGeoJSON</code> <code class="interpreted-text" role="class">AsGML</code> <code class="interpreted-text" role="class">AsKML</code> <code class="interpreted-text" role="class">AsSVG</code></p></td>
<td><p>X X X X</p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X X X</p></td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="class">AsWKB</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="class">AsWKT</code> <code class="interpreted-text" role="class">Azimuth</code> <code class="interpreted-text" role="class">BoundingCircle</code></p></td>
<td><p>X X X</p></td>
<td><p>X</p>
<p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X (LWGEOM/RTTOPO) X (≥ 5.1)</p></td>
</tr>
<tr class="odd">
<td><p><code class="interpreted-text" role="class">Centroid</code> <code class="interpreted-text" role="class">ClosestPoint</code></p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X</p></td>
</tr>
<tr class="even">
<td><code class="interpreted-text" role="class">Difference</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="class">Distance</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="class">Envelope</code> <code class="interpreted-text" role="class">ForcePolygonCW</code></p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X</p></td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="class">FromWKB</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="class">FromWKT</code> <code class="interpreted-text" role="class">GeoHash</code> <code class="interpreted-text" role="class">GeometryDistance</code></p></td>
<td><p>X X X</p></td>
<td><p>X</p></td>
<td><p>X X (≥ 12.0.1)</p></td>
<td><p>X X</p></td>
<td><p>X X (LWGEOM/RTTOPO)</p></td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="class">GeometryType</code></td>
<td>X</td>
<td>X (≥ 23c)</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="class">Intersection</code> <code class="interpreted-text" role="class">IsEmpty</code></p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X</p></td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="class">IsValid</code></td>
<td>X</td>
<td>X</td>
<td>X (≥ 12.0.1)</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="class">Length</code> <code class="interpreted-text" role="class">LineLocatePoint</code> <code class="interpreted-text" role="class">MakeValid</code> <code class="interpreted-text" role="class">MemSize</code> <code class="interpreted-text" role="class">NumDimensions</code></p></td>
<td><p>X X X X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X X (LWGEOM/RTTOPO)</p>
<p>X</p></td>
</tr>
<tr class="odd">
<td><code class="interpreted-text" role="class">NumGeometries</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
<tr class="even">
<td><p><code class="interpreted-text" role="class">NumPoints</code> <code class="interpreted-text" role="class">Perimeter</code> <code class="interpreted-text" role="class">PointOnSurface</code> <code class="interpreted-text" role="class">Reverse</code> <code class="interpreted-text" role="class">Rotate</code> <code class="interpreted-text" role="class">Scale</code> <code class="interpreted-text" role="class">SnapToGrid</code></p></td>
<td><p>X X X X X X X</p></td>
<td><p>X X X X</p></td>
<td><p>X</p>
<p>X</p></td>
<td><p>X</p></td>
<td><p>X X X X</p>
<p>X X</p></td>
</tr>
<tr class="odd">
<td><p><code class="interpreted-text" role="class">SymDifference</code> <code class="interpreted-text" role="class">Transform</code> <code class="interpreted-text" role="class">Translate</code></p></td>
<td><p>X X X</p></td>
<td><p>X X</p></td>
<td><p>X</p></td>
<td><p>X</p></td>
<td><p>X X X</p></td>
</tr>
<tr class="even">
<td><code class="interpreted-text" role="class">Union</code></td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
<td>X</td>
</tr>
</tbody>
</table>

### Aggregate Functions

The following table provides a summary of what GIS-specific aggregate functions are available on each spatial backend.

<div class="currentmodule">

django.contrib.gis.db.models

</div>

<table>
<thead>
<tr class="header">
<th>Aggregate</th>
<th>PostGIS</th>
<th>Oracle</th>
<th>MariaDB</th>
<th>MySQL</th>
<th>SpatiaLite</th>
</tr>
</thead>
<tbody>
<tr class="odd">
<td><p><code class="interpreted-text" role="class">Collect</code> <code class="interpreted-text" role="class">Extent</code> <code class="interpreted-text" role="class">Extent3D</code> <code class="interpreted-text" role="class">MakeLine</code> <code class="interpreted-text" role="class">Union</code></p></td>
<td><p>X X X X X</p></td>
<td><p>X</p>
<p>X</p></td>
<td><p>X (≥ 12.0.1)</p></td>
<td><p>X (≥ 8.0.24)</p></td>
<td><p>X X</p>
<p>X X</p></td>
</tr>
</tbody>
</table>

**Footnotes**

[^1]: *See* Open Geospatial Consortium, Inc., [OpenGIS Simple Feature Specification For SQL](https://portal.ogc.org/files/?artifact_id=829), Document 99-049 (May 5, 1999), at Ch. 3.2.5, p. 3-11 (SQL Textual Representation of Geometry).

[^2]: *See* [PostGIS EWKB, EWKT and Canonical Forms](https://postgis.net/docs/using_postgis_dbmanagement.html#EWKB_EWKT), PostGIS documentation at Ch. 4.1.2.

[^3]: *See* [PostGIS documentation](https://postgis.net/docs/ST_DistanceSphere.html) on `ST_DistanceSphere`.
