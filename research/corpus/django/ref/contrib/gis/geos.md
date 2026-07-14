# GEOS API

<div class="module" synopsis="GeoDjango's high-level interface to the GEOS library.">

django.contrib.gis.geos

</div>

## Background

### What is GEOS?

[GEOS](https://libgeos.org/) stands for **Geometry Engine - Open Source**, and is a C++ library, ported from the [Java Topology Suite](https://sourceforge.net/projects/jts-topo-suite/). GEOS implements the OpenGIS [Simple Features for SQL](https://www.ogc.org/standard/sfs/) spatial predicate functions and spatial operators. GEOS, now an OSGeo project, was initially developed and maintained by [Refractions Research](http://www.refractions.net/) of Victoria, Canada.

### Features

GeoDjango implements a high-level Python wrapper for the GEOS library, its features include:

- A BSD-licensed interface to the GEOS geometry routines, implemented purely in Python using `ctypes`.
- Loosely-coupled to GeoDjango. For example, `GEOSGeometry` objects may be used outside of a Django project/application. In other words, no need to have `DJANGO_SETTINGS_MODULE` set or use a database, etc.
- Mutability: `GEOSGeometry` objects may be modified.
- Cross-platform tested.

## Tutorial

This section contains a brief introduction and tutorial to using `GEOSGeometry` objects.

### Creating a Geometry

`GEOSGeometry` objects may be created in a few ways. The first is to simply instantiate the object on some spatial input -- the following are examples of creating the same geometry from WKT, HEX, WKB, and GeoJSON:

``` pycon
>>> from django.contrib.gis.geos import GEOSGeometry
>>> pnt = GEOSGeometry("POINT(5 23)")  # WKT
>>> pnt = GEOSGeometry("010100000000000000000014400000000000003740")  # HEX
>>> pnt = GEOSGeometry(
...     memoryview(
...         b"\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x14@\x00\x00\x00\x00\x00\x007@"
...     )
... )  # WKB
>>> pnt = GEOSGeometry(
...     '{ "type": "Point", "coordinates": [ 5.000000, 23.000000 ] }'
... )  # GeoJSON
```

Another option is to use the constructor for the specific geometry type that you wish to create. For example, a `Point` object may be created by passing in the X and Y coordinates into its constructor:

``` pycon
>>> from django.contrib.gis.geos import Point
>>> pnt = Point(5, 23)
```

All these constructors take the keyword argument `srid`. For example:

``` pycon
>>> from django.contrib.gis.geos import GEOSGeometry, LineString, Point
>>> print(GEOSGeometry("POINT (0 0)", srid=4326))
SRID=4326;POINT (0 0)
>>> print(LineString((0, 0), (1, 1), srid=4326))
SRID=4326;LINESTRING (0 0, 1 1)
>>> print(Point(0, 0, srid=32140))
SRID=32140;POINT (0 0)
```

Finally, there is the `fromfile` factory method which returns a `GEOSGeometry` object from a file:

``` pycon
>>> from django.contrib.gis.geos import fromfile
>>> pnt = fromfile("/path/to/pnt.wkt")
>>> pnt = fromfile(open("/path/to/pnt.wkt"))
```

<div id="geos-exceptions-in-logfile">

<div class="admonition">

My logs are filled with GEOS-related errors

You find many `TypeError` or `AttributeError` exceptions filling your web server's log files. This generally means that you are creating GEOS objects at the top level of some of your Python modules. Then, due to a race condition in the garbage collector, your module is garbage collected before the GEOS object. To prevent this, create `GEOSGeometry` objects inside the local scope of your functions/methods.

</div>

</div>

### Geometries are Pythonic

`GEOSGeometry` objects are 'Pythonic', in other words components may be accessed, modified, and iterated over using standard Python conventions. For example, you can iterate over the coordinates in a `Point`:

``` pycon
>>> pnt = Point(5, 23)
>>> [coord for coord in pnt]
[5.0, 23.0]
```

With any geometry object, the `GEOSGeometry.coords` property may be used to get the geometry coordinates as a Python tuple:

``` pycon
>>> pnt.coords
(5.0, 23.0)
```

You can get/set geometry components using standard Python indexing techniques. However, what is returned depends on the geometry type of the object. For example, indexing on a `LineString` returns a coordinate tuple:

``` pycon
>>> from django.contrib.gis.geos import LineString
>>> line = LineString((0, 0), (0, 50), (50, 50), (50, 0), (0, 0))
>>> line[0]
(0.0, 0.0)
>>> line[-2]
(50.0, 0.0)
```

Whereas indexing on a `Polygon` will return the ring (a `LinearRing` object) corresponding to the index:

``` pycon
>>> from django.contrib.gis.geos import Polygon
>>> poly = Polygon(((0.0, 0.0), (0.0, 50.0), (50.0, 50.0), (50.0, 0.0), (0.0, 0.0)))
>>> poly[0]
<LinearRing object at 0x1044395b0>
>>> poly[0][-2]  # second-to-last coordinate of external ring
(50.0, 0.0)
```

In addition, coordinates/components of the geometry may added or modified, just like a Python list:

``` pycon
>>> line[0] = (1.0, 1.0)
>>> line.pop()
(0.0, 0.0)
>>> line.append((1.0, 1.0))
>>> line.coords
((1.0, 1.0), (0.0, 50.0), (50.0, 50.0), (50.0, 0.0), (1.0, 1.0))
```

Geometries support set-like operators:

``` pycon
>>> from django.contrib.gis.geos import LineString
>>> ls1 = LineString((0, 0), (2, 2))
>>> ls2 = LineString((1, 1), (3, 3))
>>> print(ls1 | ls2)  # equivalent to `ls1.union(ls2)`
MULTILINESTRING ((0 0, 1 1), (1 1, 2 2), (2 2, 3 3))
>>> print(ls1 & ls2)  # equivalent to `ls1.intersection(ls2)`
LINESTRING (1 1, 2 2)
>>> print(ls1 - ls2)  # equivalent to `ls1.difference(ls2)`
LINESTRING(0 0, 1 1)
>>> print(ls1 ^ ls2)  # equivalent to `ls1.sym_difference(ls2)`
MULTILINESTRING ((0 0, 1 1), (2 2, 3 3))
```

<div class="admonition">

Equality operator doesn't check spatial equality

The `~GEOSGeometry` equality operator uses `~GEOSGeometry.equals_exact`, not `~GEOSGeometry.equals`, i.e. it requires the compared geometries to have the same coordinates in the same positions with the same SRIDs:

``` pycon
>>> from django.contrib.gis.geos import LineString
>>> ls1 = LineString((0, 0), (1, 1))
>>> ls2 = LineString((1, 1), (0, 0))
>>> ls3 = LineString((1, 1), (0, 0), srid=4326)
>>> ls1.equals(ls2)
True
>>> ls1 == ls2
False
>>> ls3 == ls2  # different SRIDs
False
```

</div>

## Geometry Objects

### `GEOSGeometry`

<div class="GEOSGeometry(geo_input, srid=None)">

param geo_input  
Geometry input value (string or `memoryview`)

param srid  
spatial reference identifier

type srid  
int

</div>

This is the base class for all GEOS geometry objects. It initializes on the given `geo_input` argument, and then assumes the proper geometry subclass (e.g., `GEOSGeometry('POINT(1 1)')` will create a `Point` object).

The `srid` parameter, if given, is set as the SRID of the created geometry if `geo_input` doesn't have an SRID. If different SRIDs are provided through the `geo_input` and `srid` parameters, `ValueError` is raised:

``` pycon
>>> from django.contrib.gis.geos import GEOSGeometry
>>> GEOSGeometry("POINT EMPTY", srid=4326).ewkt
'SRID=4326;POINT EMPTY'
>>> GEOSGeometry("SRID=4326;POINT EMPTY", srid=4326).ewkt
'SRID=4326;POINT EMPTY'
>>> GEOSGeometry("SRID=1;POINT EMPTY", srid=4326)
Traceback (most recent call last):
...
ValueError: Input geometry already has SRID: 1.
```

The following input formats, along with their corresponding Python types, are accepted:

| Format           | Input Type   |
|------------------|--------------|
| WKT / EWKT       | `str`        |
| HEX / HEXEWKB    | `str`        |
| WKB / EWKB       | `memoryview` |
| `GeoJSON <7946>` | `str`        |

For the GeoJSON format, the SRID is set based on the `crs` member. If `crs` isn't provided, the SRID defaults to 4326.

<div class="classmethod">

GEOSGeometry.from_gml(gml_string)

Constructs a `GEOSGeometry` from the given GML string.

</div>

#### Properties

<div class="attribute">

GEOSGeometry.coords

Returns the coordinates of the geometry as a tuple.

</div>

<div class="attribute">

GEOSGeometry.dims

Returns the dimension of the geometry:

- `0` for `Point`s and `MultiPoint`s
- `1` for `LineString`s and `MultiLineString`s
- `2` for `Polygon`s and `MultiPolygon`s
- `-1` for empty `GeometryCollection`s
- the maximum dimension of its elements for non-empty `GeometryCollection`s

</div>

<div class="attribute">

GEOSGeometry.empty

Returns whether or not the set of points in the geometry is empty.

</div>

<div class="attribute">

GEOSGeometry.geom_type

Returns a string corresponding to the type of geometry. For example:

``` pycon
>>> pnt = GEOSGeometry("POINT(5 23)")
>>> pnt.geom_type
'Point'
```

</div>

<div class="attribute">

GEOSGeometry.geom_typeid

Returns the GEOS geometry type identification number. The following table shows the value for each geometry type:

| Geometry             | ID  |
|----------------------|-----|
| `Point`              | 0   |
| `LineString`         | 1   |
| `LinearRing`         | 2   |
| `Polygon`            | 3   |
| `MultiPoint`         | 4   |
| `MultiLineString`    | 5   |
| `MultiPolygon`       | 6   |
| `GeometryCollection` | 7   |

</div>

<div class="attribute">

GEOSGeometry.num_coords

Returns the number of coordinates in the geometry.

</div>

<div class="attribute">

GEOSGeometry.num_geom

Returns the number of geometries in this geometry. In other words, will return 1 on anything but geometry collections.

</div>

<div class="attribute">

GEOSGeometry.hasz

Returns a boolean indicating whether the geometry has the Z dimension.

</div>

<div class="attribute">

GEOSGeometry.hasm

<div class="versionadded">

6.0

</div>

Returns a boolean indicating whether the geometry has the M dimension. Requires GEOS 3.12.

</div>

<div class="attribute">

GEOSGeometry.ring

Returns a boolean indicating whether the geometry is a `LinearRing`.

</div>

<div class="attribute">

GEOSGeometry.simple

Returns a boolean indicating whether the geometry is 'simple'. A geometry is simple if and only if it does not intersect itself (except at boundary points). For example, a `LineString` object is not simple if it intersects itself. Thus, `LinearRing` and `Polygon` objects are always simple because they cannot intersect themselves, by definition.

</div>

<div class="attribute">

GEOSGeometry.valid

Returns a boolean indicating whether the geometry is valid.

</div>

<div class="attribute">

GEOSGeometry.valid_reason

Returns a string describing the reason why a geometry is invalid.

</div>

<div class="attribute">

GEOSGeometry.srid

Property that may be used to retrieve or set the SRID associated with the geometry. For example:

``` pycon
>>> pnt = Point(5, 23)
>>> print(pnt.srid)
None
>>> pnt.srid = 4326
>>> pnt.srid
4326
```

</div>

#### Output Properties

The properties in this section export the `GEOSGeometry` object into a different. This output may be in the form of a string, buffer, or even another object.

<div class="attribute">

GEOSGeometry.ewkt

Returns the "extended" Well-Known Text of the geometry. This representation is specific to PostGIS and is a superset of the OGC WKT standard.[^1] Essentially the SRID is prepended to the WKT representation, for example `SRID=4326;POINT(5 23)`.

<div class="note">

<div class="title">

Note

</div>

The output from this property does not include the 3dm, 3dz, and 4d information that PostGIS supports in its EWKT representations.

</div>

</div>

<div class="attribute">

GEOSGeometry.hex

Returns the WKB of this Geometry in hexadecimal form. Please note that the SRID value is not included in this representation because it is not a part of the OGC specification (use the `GEOSGeometry.hexewkb` property instead).

</div>

<div class="attribute">

GEOSGeometry.hexewkb

Returns the EWKB of this Geometry in hexadecimal form. This is an extension of the WKB specification that includes the SRID value that are a part of this geometry.

</div>

<div class="attribute">

GEOSGeometry.json

Returns the GeoJSON representation of the geometry. Note that the result is not a complete GeoJSON structure but only the `geometry` key content of a GeoJSON structure. See also `/ref/contrib/gis/serializers`.

</div>

<div class="attribute">

GEOSGeometry.geojson

Alias for `GEOSGeometry.json`.

</div>

<div class="attribute">

GEOSGeometry.kml

Returns a [KML](https://developers.google.com/kml/documentation/) (Keyhole Markup Language) representation of the geometry. This should only be used for geometries with an SRID of 4326 (WGS84), but this restriction is not enforced.

</div>

<div class="attribute">

GEOSGeometry.ogr

Returns an `~django.contrib.gis.gdal.OGRGeometry` object corresponding to the GEOS geometry.

</div>

<div id="wkb">

<div class="attribute">

GEOSGeometry.wkb

Returns the WKB (Well-Known Binary) representation of this Geometry as a Python buffer. SRID value is not included, use the `GEOSGeometry.ewkb` property instead.

</div>

</div>

<div id="ewkb">

<div class="attribute">

GEOSGeometry.ewkb

Return the EWKB representation of this Geometry as a Python buffer. This is an extension of the WKB specification that includes any SRID value that are a part of this geometry.

</div>

</div>

<div class="attribute">

GEOSGeometry.wkt

Returns the Well-Known Text of the geometry (an OGC standard).

</div>

#### Spatial Predicate Methods

All of the following spatial predicate methods take another `GEOSGeometry` instance (`other`) as a parameter, and return a boolean.

<div class="method">

GEOSGeometry.contains(other)

Returns `True` if `other.within(this) <GEOSGeometry.within>` returns `True`.

</div>

<div class="method">

GEOSGeometry.covers(other)

Returns `True` if this geometry covers the specified geometry.

The `covers` predicate has the following equivalent definitions:

- Every point of the other geometry is a point of this geometry.
- The [DE-9IM]() Intersection Matrix for the two geometries is `T*****FF*`, `*T****FF*`, `***T**FF*`, or `****T*FF*`.

If either geometry is empty, returns `False`.

This predicate is similar to `GEOSGeometry.contains`, but is more inclusive (i.e. returns `True` for more cases). In particular, unlike `~GEOSGeometry.contains` it does not distinguish between points in the boundary and in the interior of geometries. For most situations, `covers()` should be preferred to `~GEOSGeometry.contains`. As an added benefit, `covers()` is more amenable to optimization and hence should outperform `~GEOSGeometry.contains`.

</div>

<div class="method">

GEOSGeometry.crosses(other)

Returns `True` if the DE-9IM intersection matrix for the two Geometries is `T*T******` (for a point and a curve,a point and an area or a line and an area) `0********` (for two curves).

</div>

<div class="method">

GEOSGeometry.disjoint(other)

Returns `True` if the DE-9IM intersection matrix for the two geometries is `FF*FF****`.

</div>

<div class="method">

GEOSGeometry.equals(other)

Returns `True` if the DE-9IM intersection matrix for the two geometries is `T*F**FFF*`.

</div>

<div class="method">

GEOSGeometry.equals_exact(other, tolerance=0)

Returns true if the two geometries are exactly equal, up to a specified tolerance. The `tolerance` value should be a floating point number representing the error tolerance in the comparison, e.g., `poly1.equals_exact(poly2, 0.001)` will compare equality to within one thousandth of a unit.

</div>

<div class="method">

GEOSGeometry.equals_identical(other)

Returns `True` if the two geometries are point-wise equivalent by checking that the structure, ordering, and values of all vertices are identical in all dimensions. `NaN` values are considered to be equal to other `NaN` values. Requires GEOS 3.12.

</div>

<div class="method">

GEOSGeometry.intersects(other)

Returns `True` if `GEOSGeometry.disjoint` is `False`.

</div>

<div class="method">

GEOSGeometry.overlaps(other)

Returns true if the DE-9IM intersection matrix for the two geometries is `T*T***T**` (for two points or two surfaces) `1*T***T**` (for two curves).

</div>

<div class="method">

GEOSGeometry.relate_pattern(other, pattern)

Returns `True` if the elements in the DE-9IM intersection matrix for this geometry and the other matches the given `pattern` -- a string of nine characters from the alphabet: {`T`, `F`, `*`, `0`}.

</div>

<div class="method">

GEOSGeometry.touches(other)

Returns `True` if the DE-9IM intersection matrix for the two geometries is `FT*******`, `F**T*****` or `F***T****`.

</div>

<div class="method">

GEOSGeometry.within(other)

Returns `True` if the DE-9IM intersection matrix for the two geometries is `T*F**F***`.

</div>

#### Topological Methods

<div class="method">

GEOSGeometry.buffer(width, quadsegs=8)

Returns a `GEOSGeometry` that represents all points whose distance from this geometry is less than or equal to the given `width`. The optional `quadsegs` keyword sets the number of segments used to approximate a quarter circle (defaults is 8).

</div>

<div class="method">

GEOSGeometry.buffer_with_style(width, quadsegs=8, end_cap_style=1, join_style=1, mitre_limit=5.0)

Same as `buffer`, but allows customizing the style of the buffer.

- `end_cap_style` can be round (`1`), flat (`2`), or square (`3`).
- `join_style` can be round (`1`), mitre (`2`), or bevel (`3`).
- Mitre ratio limit (`mitre_limit`) only affects mitered join style.

</div>

<div class="method">

GEOSGeometry.difference(other)

Returns a `GEOSGeometry` representing the points making up this geometry that do not make up other.

</div>

<div class="method">

GEOSGeometry.interpolate(distance)

</div>

<div class="method">

GEOSGeometry.interpolate_normalized(distance)

Given a distance (float), returns the point (or closest point) within the geometry (`LineString` or `MultiLineString`) at that distance. The normalized version takes the distance as a float between 0 (origin) and 1 (endpoint).

Reverse of `GEOSGeometry.project`.

</div>

<div class="method">

GEOSGeometry.intersection(other)

Returns a `GEOSGeometry` representing the points shared by this geometry and other.

</div>

<div class="method">

GEOSGeometry.project(point)

</div>

<div class="method">

GEOSGeometry.project_normalized(point)

Returns the distance (float) from the origin of the geometry (`LineString` or `MultiLineString`) to the point projected on the geometry (that is to a point of the line the closest to the given point). The normalized version returns the distance as a float between 0 (origin) and 1 (endpoint).

Reverse of `GEOSGeometry.interpolate`.

</div>

<div class="method">

GEOSGeometry.relate(other)

Returns the DE-9IM intersection matrix (a string) representing the topological relationship between this geometry and the other.

</div>

<div class="method">

GEOSGeometry.simplify(tolerance=0.0, preserve_topology=False)

Returns a new `GEOSGeometry`, simplified to the specified tolerance using the Douglas-Peucker algorithm. A higher tolerance value implies fewer points in the output. If no tolerance is provided, it defaults to 0.

By default, this function does not preserve topology. For example, `Polygon` objects can be split, be collapsed into lines, or disappear. `Polygon` holes can be created or disappear, and lines may cross. By specifying `preserve_topology=True`, the result will have the same dimension and number of components as the input; this is significantly slower, however.

</div>

<div class="method">

GEOSGeometry.sym_difference(other)

Returns a `GEOSGeometry` combining the points in this geometry not in other, and the points in other not in this geometry.

</div>

<div class="method">

GEOSGeometry.union(other)

Returns a `GEOSGeometry` representing all the points in this geometry and the other.

</div>

#### Topological Properties

<div class="attribute">

GEOSGeometry.boundary

Returns the boundary as a newly allocated Geometry object.

</div>

<div class="attribute">

GEOSGeometry.centroid

Returns a `Point` object representing the geometric center of the geometry. The point is not guaranteed to be on the interior of the geometry.

</div>

<div class="attribute">

GEOSGeometry.convex_hull

Returns the smallest `Polygon` that contains all the points in the geometry.

</div>

<div class="attribute">

GEOSGeometry.envelope

Returns a `Polygon` that represents the bounding envelope of this geometry. Note that it can also return a `Point` if the input geometry is a point.

</div>

<div class="attribute">

GEOSGeometry.point_on_surface

Computes and returns a `Point` guaranteed to be on the interior of this geometry.

</div>

<div class="attribute">

GEOSGeometry.unary_union

Computes the union of all the elements of this geometry.

The result obeys the following contract:

- Unioning a set of `LineString`s has the effect of fully noding and dissolving the linework.
- Unioning a set of `Polygon`s will always return a `Polygon` or `MultiPolygon` geometry (unlike `GEOSGeometry.union`, which may return geometries of lower dimension if a topology collapse occurs).

</div>

#### Other Properties & Methods

<div class="attribute">

GEOSGeometry.area

This property returns the area of the Geometry.

</div>

<div class="attribute">

GEOSGeometry.extent

This property returns the extent of this geometry as a 4-tuple, consisting of `(xmin, ymin, xmax, ymax)`.

</div>

<div class="method">

GEOSGeometry.clone()

This method returns a `GEOSGeometry` that is a clone of the original.

</div>

<div class="method">

GEOSGeometry.distance(geom)

Returns the distance between the closest points on this geometry and the given `geom` (another `GEOSGeometry` object).

<div class="note">

<div class="title">

Note

</div>

GEOS distance calculations are linear -- in other words, GEOS does not perform a spherical calculation even if the SRID specifies a geographic coordinate system.

</div>

</div>

<div class="attribute">

GEOSGeometry.length

Returns the length of this geometry (e.g., 0 for a `Point`, the length of a `LineString`, or the circumference of a `Polygon`).

</div>

<div class="attribute">

GEOSGeometry.prepared

Returns a GEOS `PreparedGeometry` for the contents of this geometry. `PreparedGeometry` objects are optimized for the contains, intersects, covers, crosses, disjoint, overlaps, touches and within operations. Refer to the `prepared-geometries` documentation for more information.

</div>

<div class="attribute">

GEOSGeometry.srs

Returns a `~django.contrib.gis.gdal.SpatialReference` object corresponding to the SRID of the geometry or `None`.

</div>

<div class="method">

GEOSGeometry.transform(ct, clone=False)

Transforms the geometry according to the given coordinate transformation parameter (`ct`), which may be an integer SRID, spatial reference WKT string, a PROJ string, a `~django.contrib.gis.gdal.SpatialReference` object, or a `~django.contrib.gis.gdal.CoordTransform` object. By default, the geometry is transformed in-place and nothing is returned. However if the `clone` keyword is set, then the geometry is not modified and a transformed clone of the geometry is returned instead.

<div class="note">

<div class="title">

Note

</div>

Raises `~django.contrib.gis.geos.GEOSException` if GDAL is not available or if the geometry's SRID is `None` or less than 0. It doesn't impose any constraints on the geometry's SRID if called with a `~django.contrib.gis.gdal.CoordTransform` object.

</div>

</div>

<div class="method">

GEOSGeometry.make_valid()

Returns a valid `GEOSGeometry` equivalent, trying not to lose any of the input vertices. If the geometry is already valid, it is returned untouched. This is similar to the `~django.contrib.gis.db.models.functions.MakeValid` database function.

</div>

<div class="method">

GEOSGeometry.normalize(clone=False)

Converts this geometry to canonical form. If the `clone` keyword is set, then the geometry is not modified and a normalized clone of the geometry is returned instead:

``` pycon
>>> g = MultiPoint(Point(0, 0), Point(2, 2), Point(1, 1))
>>> print(g)
MULTIPOINT (0 0, 2 2, 1 1)
>>> g.normalize()
>>> print(g)
MULTIPOINT (2 2, 1 1, 0 0)
```

</div>

### `Point`

<div class="Point(x=None, y=None, z=None, srid=None)">

`Point` objects are instantiated using arguments that represent the component coordinates of the point or with a single sequence coordinates. For example, the following are equivalent:

``` pycon
>>> pnt = Point(5, 23)
>>> pnt = Point([5, 23])
```

Empty `Point` objects may be instantiated by passing no arguments or an empty sequence. The following are equivalent:

``` pycon
>>> pnt = Point()
>>> pnt = Point([])
```

</div>

### `LineString`

<div class="LineString(*args, **kwargs)">

`LineString` objects are instantiated using arguments that are either a sequence of coordinates or `Point` objects. For example, the following are equivalent:

``` pycon
>>> ls = LineString((0, 0), (1, 1))
>>> ls = LineString(Point(0, 0), Point(1, 1))
```

In addition, `LineString` objects may also be created by passing in a single sequence of coordinate or `Point` objects:

``` pycon
>>> ls = LineString(((0, 0), (1, 1)))
>>> ls = LineString([Point(0, 0), Point(1, 1)])
```

Empty `LineString` objects may be instantiated by passing no arguments or an empty sequence. The following are equivalent:

``` pycon
>>> ls = LineString()
>>> ls = LineString([])
```

<div class="attribute">

closed

Returns whether or not this `LineString` is closed.

</div>

</div>

### `LinearRing`

<div class="LinearRing(*args, **kwargs)">

`LinearRing` objects are constructed in the exact same way as `LineString` objects, however the coordinates must be *closed*, in other words, the first coordinates must be the same as the last coordinates. For example:

``` pycon
>>> ls = LinearRing((0, 0), (0, 1), (1, 1), (0, 0))
```

Notice that `(0, 0)` is the first and last coordinate -- if they were not equal, an error would be raised.

<div class="attribute">

is_counterclockwise

Returns whether this `LinearRing` is counterclockwise.

</div>

</div>

### `Polygon`

<div class="Polygon(*args, **kwargs)">

`Polygon` objects may be instantiated by passing in parameters that represent the rings of the polygon. The parameters must either be `LinearRing` instances, or a sequence that may be used to construct a `LinearRing`:

``` pycon
>>> ext_coords = ((0, 0), (0, 1), (1, 1), (1, 0), (0, 0))
>>> int_coords = ((0.4, 0.4), (0.4, 0.6), (0.6, 0.6), (0.6, 0.4), (0.4, 0.4))
>>> poly = Polygon(ext_coords, int_coords)
>>> poly = Polygon(LinearRing(ext_coords), LinearRing(int_coords))
```

<div class="classmethod">

from_bbox(bbox)

Returns a polygon object from the given bounding-box, a 4-tuple comprising `(xmin, ymin, xmax, ymax)`.

</div>

<div class="attribute">

num_interior_rings

Returns the number of interior rings in this geometry.

</div>

</div>

<div class="admonition">

Comparing Polygons

Note that it is possible to compare `Polygon` objects directly with `<` or `>`, but as the comparison is made through Polygon's `LineString`, it does not mean much (but is consistent and quick). You can always force the comparison with the `~GEOSGeometry.area` property:

``` pycon
>>> if poly_1.area > poly_2.area:
...     pass
...
```

</div>

## Geometry Collections

### `MultiPoint`

<div class="MultiPoint(*args, **kwargs)">

`MultiPoint` objects may be instantiated by passing in `Point` objects as arguments, or a single sequence of `Point` objects:

``` pycon
>>> mp = MultiPoint(Point(0, 0), Point(1, 1))
>>> mp = MultiPoint((Point(0, 0), Point(1, 1)))
```

</div>

### `MultiLineString`

<div class="MultiLineString(*args, **kwargs)">

`MultiLineString` objects may be instantiated by passing in `LineString` objects as arguments, or a single sequence of `LineString` objects:

``` pycon
>>> ls1 = LineString((0, 0), (1, 1))
>>> ls2 = LineString((2, 2), (3, 3))
>>> mls = MultiLineString(ls1, ls2)
>>> mls = MultiLineString([ls1, ls2])
```

<div class="attribute">

merged

Returns a `LineString` representing the line merge of all the components in this `MultiLineString`.

</div>

<div class="attribute">

closed

Returns `True` if and only if all elements are closed.

</div>

</div>

### `MultiPolygon`

<div class="MultiPolygon(*args, **kwargs)">

`MultiPolygon` objects may be instantiated by passing `Polygon` objects as arguments, or a single sequence of `Polygon` objects:

``` pycon
>>> p1 = Polygon(((0, 0), (0, 1), (1, 1), (0, 0)))
>>> p2 = Polygon(((1, 1), (1, 2), (2, 2), (1, 1)))
>>> mp = MultiPolygon(p1, p2)
>>> mp = MultiPolygon([p1, p2])
```

</div>

### `GeometryCollection`

<div class="GeometryCollection(*args, **kwargs)">

`GeometryCollection` objects may be instantiated by passing in other `GEOSGeometry` as arguments, or a single sequence of `GEOSGeometry` objects:

``` pycon
>>> poly = Polygon(((0, 0), (0, 1), (1, 1), (0, 0)))
>>> gc = GeometryCollection(Point(0, 0), MultiPoint(Point(0, 0), Point(1, 1)), poly)
>>> gc = GeometryCollection((Point(0, 0), MultiPoint(Point(0, 0), Point(1, 1)), poly))
```

</div>

## Prepared Geometries

In order to obtain a prepared geometry, access the `GEOSGeometry.prepared` property. Once you have a `PreparedGeometry` instance its spatial predicate methods, listed below, may be used with other `GEOSGeometry` objects. An operation with a prepared geometry can be orders of magnitude faster -- the more complex the geometry that is prepared, the larger the speedup in the operation. For more information, please consult the [GEOS wiki page on prepared geometries](https://trac.osgeo.org/geos/wiki/PreparedGeometry).

For example:

``` pycon
>>> from django.contrib.gis.geos import Point, Polygon
>>> poly = Polygon.from_bbox((0, 0, 5, 5))
>>> prep_poly = poly.prepared
>>> prep_poly.contains(Point(2.5, 2.5))
True
```

### `PreparedGeometry`

<div class="PreparedGeometry">

All methods on `PreparedGeometry` take an `other` argument, which must be a `GEOSGeometry` instance.

<div class="method">

contains(other)

</div>

<div class="method">

contains_properly(other)

</div>

<div class="method">

covers(other)

</div>

<div class="method">

crosses(other)

</div>

<div class="method">

disjoint(other)

</div>

<div class="method">

intersects(other)

</div>

<div class="method">

overlaps(other)

</div>

<div class="method">

touches(other)

</div>

<div class="method">

within(other)

</div>

</div>

## Geometry Factories

<div class="function">

fromfile(file_h)

param file_h  
input file that contains spatial data

type file_h  
a Python `file` object or a string path to the file

rtype  
a `GEOSGeometry` corresponding to the spatial data in the file

Example:

``` pycon
>>> from django.contrib.gis.geos import fromfile
>>> g = fromfile("/home/bob/geom.wkt")
```

</div>

<div class="function">

fromstr(string, srid=None)

param string  
string that contains spatial data

type string  
str

param srid  
spatial reference identifier

type srid  
int

rtype  
a `GEOSGeometry` corresponding to the spatial data in the string

`fromstr(string, srid)` is equivalent to `GEOSGeometry(string, srid) <GEOSGeometry>`.

Example:

``` pycon
>>> from django.contrib.gis.geos import fromstr
>>> pnt = fromstr("POINT(-90.5 29.5)", srid=4326)
```

</div>

## I/O Objects

### Reader Objects

The reader I/O classes return a `GEOSGeometry` instance from the WKB and/or WKT input given to their `read(geom)` method.

<div class="WKBReader">

Example:

``` pycon
>>> from django.contrib.gis.geos import WKBReader
>>> wkb_r = WKBReader()
>>> wkb_r.read("0101000000000000000000F03F000000000000F03F")
<Point object at 0x103a88910>
```

</div>

<div class="WKTReader">

Example:

``` pycon
>>> from django.contrib.gis.geos import WKTReader
>>> wkt_r = WKTReader()
>>> wkt_r.read("POINT(1 1)")
<Point object at 0x103a88b50>
```

</div>

### Writer Objects

All writer objects have a `write(geom)` method that returns either the WKB or WKT of the given geometry. In addition, `WKBWriter` objects also have properties that may be used to change the byte order, and or include the SRID value (in other words, EWKB).

<div class="WKBWriter(dim=2)">

`WKBWriter` provides the most control over its output. By default it returns OGC-compliant WKB when its `write` method is called. However, it has properties that allow for the creation of EWKB, a superset of the WKB standard that includes additional information. See the `WKBWriter.outdim` documentation for more details about the `dim` argument.

<div class="method">

WKBWriter.write(geom)

</div>

Returns the WKB of the given geometry as a Python `buffer` object. Example:

``` pycon
>>> from django.contrib.gis.geos import Point, WKBWriter
>>> pnt = Point(1, 1)
>>> wkb_w = WKBWriter()
>>> wkb_w.write(pnt)
<read-only buffer for 0x103a898f0, size -1, offset 0 at 0x103a89930>
```

<div class="method">

WKBWriter.write_hex(geom)

</div>

Returns WKB of the geometry in hexadecimal. Example:

``` pycon
>>> from django.contrib.gis.geos import Point, WKBWriter
>>> pnt = Point(1, 1)
>>> wkb_w = WKBWriter()
>>> wkb_w.write_hex(pnt)
'0101000000000000000000F03F000000000000F03F'
```

<div class="attribute">

WKBWriter.byteorder

</div>

This property may be set to change the byte-order of the geometry representation.

| Byteorder Value | Description                                       |
|-----------------|---------------------------------------------------|
| 0               | Big Endian (e.g., compatible with RISC systems)   |
| 1               | Little Endian (e.g., compatible with x86 systems) |

Example:

``` pycon
>>> from django.contrib.gis.geos import Point, WKBWriter
>>> wkb_w = WKBWriter()
>>> pnt = Point(1, 1)
>>> wkb_w.write_hex(pnt)
'0101000000000000000000F03F000000000000F03F'
>>> wkb_w.byteorder = 0
'00000000013FF00000000000003FF0000000000000'
```

<div class="attribute">

WKBWriter.outdim

</div>

This property may be set to change the output dimension of the geometry representation. In other words, if you have a 3D geometry then set to 3 so that the Z value is included in the WKB.

| Outdim Value | Description                 |
|--------------|-----------------------------|
| 2            | The default, output 2D WKB. |
| 3            | Output 3D WKB.              |

Example:

``` pycon
>>> from django.contrib.gis.geos import Point, WKBWriter
>>> wkb_w = WKBWriter()
>>> wkb_w.outdim
2
>>> pnt = Point(1, 1, 1)
>>> wkb_w.write_hex(pnt)  # By default, no Z value included:
'0101000000000000000000F03F000000000000F03F'
>>> wkb_w.outdim = 3  # Tell writer to include Z values
>>> wkb_w.write_hex(pnt)
'0101000080000000000000F03F000000000000F03F000000000000F03F'
```

<div class="attribute">

WKBWriter.srid

</div>

Set this property with a boolean to indicate whether the SRID of the geometry should be included with the WKB representation. Example:

``` pycon
>>> from django.contrib.gis.geos import Point, WKBWriter
>>> wkb_w = WKBWriter()
>>> pnt = Point(1, 1, srid=4326)
>>> wkb_w.write_hex(pnt)  # By default, no SRID included:
'0101000000000000000000F03F000000000000F03F'
>>> wkb_w.srid = True  # Tell writer to include SRID
>>> wkb_w.write_hex(pnt)
'0101000020E6100000000000000000F03F000000000000F03F'
```

</div>

<div class="WKTWriter(dim=2, trim=False, precision=None)">

This class allows outputting the WKT representation of a geometry. See the `WKBWriter.outdim`, `trim`, and `precision` attributes for details about the constructor arguments.

<div class="method">

WKTWriter.write(geom)

</div>

Returns the WKT of the given geometry. Example:

``` pycon
>>> from django.contrib.gis.geos import Point, WKTWriter
>>> pnt = Point(1, 1)
>>> wkt_w = WKTWriter()
>>> wkt_w.write(pnt)
'POINT (1.0000000000000000 1.0000000000000000)'
```

<div class="attribute">

WKTWriter.outdim

See `WKBWriter.outdim`.

</div>

<div class="attribute">

WKTWriter.trim

</div>

This property is used to enable or disable trimming of unnecessary decimals.

``` pycon
>>> from django.contrib.gis.geos import Point, WKTWriter
>>> pnt = Point(1, 1)
>>> wkt_w = WKTWriter()
>>> wkt_w.trim
False
>>> wkt_w.write(pnt)
'POINT (1.0000000000000000 1.0000000000000000)'
>>> wkt_w.trim = True
>>> wkt_w.write(pnt)
'POINT (1 1)'
```

<div class="attribute">

WKTWriter.precision

</div>

This property controls the rounding precision of coordinates; if set to `None` rounding is disabled.

> \>\>\> from django.contrib.gis.geos import Point, WKTWriter \>\>\> pnt = Point(1.44, 1.66) \>\>\> wkt_w = WKTWriter() \>\>\> print(wkt_w.precision) None \>\>\> wkt_w.write(pnt) 'POINT (1.4399999999999999 1.6599999999999999)' \>\>\> wkt_w.precision = 0 \>\>\> wkt_w.write(pnt) 'POINT (1 2)' \>\>\> wkt_w.precision = 1 \>\>\> wkt_w.write(pnt) 'POINT (1.4 1.7)'

</div>

**Footnotes**

## Settings

<div class="setting">

GEOS_LIBRARY_PATH

</div>

### `GEOS_LIBRARY_PATH`

A string specifying the location of the GEOS C library. Typically, this setting is only used if the GEOS C library is in a non-standard location (e.g., `/home/bob/lib/libgeos_c.so`).

<div class="note">

<div class="title">

Note

</div>

The setting must be the *full* path to the **C** shared library; in other words you want to use `libgeos_c.so`, not `libgeos.so`.

</div>

## Exceptions

<div class="exception">

GEOSException

The base GEOS exception, indicates a GEOS-related error.

</div>

[^1]: *See* [PostGIS EWKB, EWKT and Canonical Forms](https://postgis.net/docs/using_postgis_dbmanagement.html#EWKB_EWKT), PostGIS documentation at Ch. 4.1.2.
