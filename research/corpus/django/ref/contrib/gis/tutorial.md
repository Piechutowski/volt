# GeoDjango Tutorial

## Introduction

GeoDjango is an included contrib module for Django that turns it into a world-class geographic web framework. GeoDjango strives to make it as simple as possible to create geographic web applications, like location-based services. Its features include:

- Django model fields for [OGC](https://www.ogc.org/) geometries and raster data.
- Extensions to Django's ORM for querying and manipulating spatial data.
- Loosely-coupled, high-level Python interfaces for GIS geometry and raster operations and data manipulation in different formats.
- Editing geometry fields from the admin.

This tutorial assumes familiarity with Django; thus, if you're brand new to Django, please read through the `regular tutorial </intro/tutorial01>` to familiarize yourself with Django first.

<div class="note">

<div class="title">

Note

</div>

GeoDjango has additional requirements beyond what Django requires -- please consult the `installation documentation
</ref/contrib/gis/install/index>` for more details.

</div>

This tutorial will guide you through the creation of a geographic web application for viewing the [world borders](#world-borders).[^1] Some of the code used in this tutorial is taken from and/or inspired by the [GeoDjango basic apps](https://code.google.com/archive/p/geodjango-basic-apps) project.[^2]

<div class="note">

<div class="title">

Note

</div>

Proceed through the tutorial sections sequentially for step-by-step instructions.

</div>

## Setting Up

### Create a Spatial Database

Typically no special setup is required, so you can create a database as you would for any other project. We provide some tips for selected databases:

- `/ref/contrib/gis/install/postgis`
- `/ref/contrib/gis/install/spatialite`

### Create a New Project

Use the standard `django-admin` script to create a project called `geodjango`:

<div class="console">

\$ django-admin startproject geodjango

</div>

This will initialize a new project. Now, create a `world` Django application within the `geodjango` project:

<div class="console">

\$ cd geodjango \$ python manage.py startapp world

</div>

### Configure `settings.py`

The `geodjango` project settings are stored in the `geodjango/settings.py` file. Edit the database connection settings to match your setup:

    DATABASES = {
        "default": {
            "ENGINE": "django.contrib.gis.db.backends.postgis",
            "NAME": "geodjango",
            "USER": "geo",
        },
    }

In addition, modify the `INSTALLED_APPS` setting to include `django.contrib.admin`, `django.contrib.gis`, and `world` (your newly created application):

    INSTALLED_APPS = [
        "django.contrib.admin",
        "django.contrib.auth",
        "django.contrib.contenttypes",
        "django.contrib.sessions",
        "django.contrib.messages",
        "django.contrib.staticfiles",
        "django.contrib.gis",
        "world",
    ]

## Geographic Data

### World Borders

The world borders data is available in this [zip file](https://web.archive.org/web/20231220150759/https://thematicmapping.org/downloads/TM_WORLD_BORDERS-0.3.zip). Create a `data` directory in the `world` application, download the world borders data, and unzip. On GNU/Linux platforms, use the following commands:

<div class="console">

\$ mkdir world/data \$ cd world/data \$ wget <https://web.archive.org/web/20231220150759/https://thematicmapping.org/downloads/TM_WORLD_BORDERS-0.3.zip> \$ unzip TM_WORLD_BORDERS-0.3.zip \$ cd ../..

</div>

The world borders ZIP file contains a set of data files collectively known as an [ESRI Shapefile](https://en.wikipedia.org/wiki/Shapefile), one of the most popular geospatial data formats. When unzipped, the world borders dataset includes files with the following extensions:

- `.shp`: Holds the vector data for the world borders geometries.
- `.shx`: Spatial index file for geometries stored in the `.shp`.
- `.dbf`: Database file for holding non-geometric attribute data (e.g., integer and character fields).
- `.prj`: Contains the spatial reference information for the geographic data stored in the shapefile.

### Use `ogrinfo` to examine spatial data

The GDAL `ogrinfo` utility allows examining the metadata of shapefiles or other vector data sources:

<div class="console">

\$ ogrinfo world/data/TM_WORLD_BORDERS-0.3.shp INFO: Open of \`world/data/TM_WORLD_BORDERS-0.3.shp' using driver \`ESRI Shapefile' successful. 1: TM_WORLD_BORDERS-0.3 (Polygon)

</div>

`ogrinfo` tells us that the shapefile has one layer, and that this layer contains polygon data. To find out more, we'll specify the layer name and use the `-so` option to get only the important summary information:

<div class="console">

\$ ogrinfo -so world/data/TM_WORLD_BORDERS-0.3.shp TM_WORLD_BORDERS-0.3 INFO: Open of \`world/data/TM_WORLD_BORDERS-0.3.shp' using driver \`ESRI Shapefile' successful.

Layer name: TM_WORLD_BORDERS-0.3 Metadata: DBF_DATE_LAST_UPDATE=2008-07-30 Geometry: Polygon Feature Count: 246 Extent: (-180.000000, -90.000000) - (180.000000, 83.623596) Layer SRS WKT: GEOGCRS\["WGS 84", DATUM\["World Geodetic System 1984", ELLIPSOID\["WGS 84",6378137,298.257223563, LENGTHUNIT\["metre",1\]\]\], PRIMEM\["Greenwich",0, ANGLEUNIT\["degree",0.0174532925199433\]\], CS\[ellipsoidal,2\], AXIS\["latitude",north, ORDER\[1\], ANGLEUNIT\["degree",0.0174532925199433\]\], AXIS\["longitude",east, ORDER\[2\], ANGLEUNIT\["degree",0.0174532925199433\]\], ID\["EPSG",4326\]\] Data axis to CRS axis mapping: 2,1 FIPS: String (2.0) ISO2: String (2.0) ISO3: String (3.0) UN: Integer (3.0) NAME: String (50.0) AREA: Integer (7.0) POP2005: Integer64 (10.0) REGION: Integer (3.0) SUBREGION: Integer (3.0) LON: Real (8.3) LAT: Real (7.3)

</div>

This detailed summary information tells us the number of features in the layer (246), the geographic bounds of the data, the spatial reference system ("SRS WKT"), as well as type information for each attribute field. For example, `FIPS: String (2.0)` indicates that the `FIPS` character field has a maximum length of 2. Similarly, `LON: Real (8.3)` is a floating-point field that holds a maximum of 8 digits up to three decimal places.

## Geographic Models

### Defining a Geographic Model

Now that you've examined your dataset using `ogrinfo`, create a GeoDjango model to represent this data:

    from django.contrib.gis.db import models


    class WorldBorder(models.Model):
        # Regular Django fields corresponding to the attributes in the
        # world borders shapefile.
        name = models.CharField(max_length=50)
        area = models.IntegerField()
        pop2005 = models.IntegerField("Population 2005")
        fips = models.CharField("FIPS Code", max_length=2, null=True)
        iso2 = models.CharField("2 Digit ISO", max_length=2)
        iso3 = models.CharField("3 Digit ISO", max_length=3)
        un = models.IntegerField("United Nations Code")
        region = models.IntegerField("Region Code")
        subregion = models.IntegerField("Sub-Region Code")
        lon = models.FloatField()
        lat = models.FloatField()

        # GeoDjango-specific: a geometry field (MultiPolygonField)
        mpoly = models.MultiPolygonField()

        # Returns the string representation of the model.
        def __str__(self):
            return self.name

Note that the `models` module is imported from `django.contrib.gis.db`.

The default spatial reference system for geometry fields is WGS84 (meaning the [SRID](https://en.wikipedia.org/wiki/SRID) is 4326) -- in other words, the field coordinates are in longitude, latitude pairs in units of degrees. To use a different coordinate system, set the SRID of the geometry field with the `srid` argument. Use an integer representing the coordinate system's EPSG code.

### Run `migrate`

After defining your model, you need to sync it with the database. First, create a database migration:

<div class="console">

\$ python manage.py makemigrations Migrations for 'world': world/migrations/0001_initial.py: + Create model WorldBorder

</div>

Let's look at the SQL that will generate the table for the `WorldBorder` model:

<div class="console">

\$ python manage.py sqlmigrate world 0001

</div>

This command should produce the following output:

``` sql
BEGIN;
--
-- Create model WorldBorder
--
CREATE TABLE "world_worldborder" (
    "id" bigint NOT NULL PRIMARY KEY GENERATED BY DEFAULT AS IDENTITY,
    "name" varchar(50) NOT NULL,
    "area" integer NOT NULL,
    "pop2005" integer NOT NULL,
    "fips" varchar(2) NOT NULL,
    "iso2" varchar(2) NOT NULL,
    "iso3" varchar(3) NOT NULL,
    "un" integer NOT NULL,
    "region" integer NOT NULL,
    "subregion" integer NOT NULL,
    "lon" double precision NOT NULL,
    "lat" double precision NOT NULL
    "mpoly" geometry(MULTIPOLYGON,4326) NOT NULL
)
;
CREATE INDEX "world_worldborder_mpoly_id" ON "world_worldborder" USING GIST ("mpoly");
COMMIT;
```

If this looks correct, run `migrate` to create this table in the database:

<div class="console">

\$ python manage.py migrate Operations to perform: Apply all migrations: admin, auth, contenttypes, sessions, world Running migrations: ... Applying world.0001_initial... OK

</div>

## Importing Spatial Data

This section will show you how to import the world borders shapefile into the database via GeoDjango models using the `/ref/contrib/gis/layermapping`.

There are many different ways to import data into a spatial database --besides the tools included within GeoDjango, you may also use the following:

- [ogr2ogr](https://gdal.org/programs/ogr2ogr.html): A command-line utility included with GDAL that can import many vector data formats into PostGIS, MySQL, and Oracle databases.
- [shp2pgsql](https://postgis.net/docs/using_postgis_dbmanagement.html#shp2pgsql_usage): This utility included with PostGIS imports ESRI shapefiles into PostGIS.

### GDAL Interface

Earlier, you used `ogrinfo` to examine the contents of the world borders shapefile. GeoDjango also includes a Pythonic interface to GDAL's powerful OGR library that can work with all the vector data sources that OGR supports.

First, invoke the Django shell:

<div class="console">

\$ python manage.py shell

</div>

If you downloaded the `worldborders` data earlier in the tutorial, then you can determine its path using Python's `pathlib.Path`:

``` pycon
>>> from pathlib import Path
>>> import world
>>> world_shp = Path(world.__file__).resolve().parent / "data" / "TM_WORLD_BORDERS-0.3.shp"
```

Now, open the world borders shapefile using GeoDjango's `~django.contrib.gis.gdal.DataSource` interface:

``` pycon
>>> from django.contrib.gis.gdal import DataSource
>>> ds = DataSource(world_shp)
>>> print(ds)
/ ... /geodjango/world/data/TM_WORLD_BORDERS-0.3.shp (ESRI Shapefile)
```

Data source objects can have different layers of geospatial features; however, shapefiles are only allowed to have one layer:

``` pycon
>>> print(len(ds))
1
>>> lyr = ds[0]
>>> print(lyr)
TM_WORLD_BORDERS-0.3
```

You can see the layer's geometry type and how many features it contains:

``` pycon
>>> print(lyr.geom_type)
Polygon
>>> print(len(lyr))
246
```

<div class="note">

<div class="title">

Note

</div>

Unfortunately, the shapefile data format does not allow for greater specificity with regards to geometry types. This shapefile, like many others, actually includes `MultiPolygon` geometries, not Polygons. It's important to use a more general field type in models: a GeoDjango `MultiPolygonField` will accept a `Polygon` geometry, but a `PolygonField` will not accept a `MultiPolygon` type geometry. This is why the `WorldBorder` model defined above uses a `MultiPolygonField`.

</div>

The `~django.contrib.gis.gdal.Layer` may also have a spatial reference system associated with it. If it does, the `srs` attribute will return a `~django.contrib.gis.gdal.SpatialReference` object:

``` pycon
>>> srs = lyr.srs
>>> print(srs)
GEOGCS["WGS 84",
DATUM["WGS_1984",
    SPHEROID["WGS 84",6378137,298.257223563,
        AUTHORITY["EPSG","7030"]],
    AUTHORITY["EPSG","6326"]],
PRIMEM["Greenwich",0,
    AUTHORITY["EPSG","8901"]],
UNIT["degree",0.0174532925199433,
    AUTHORITY["EPSG","9122"]],
AXIS["Latitude",NORTH],
AXIS["Longitude",EAST],
AUTHORITY["EPSG","4326"]]
>>> srs.proj  # PROJ representation
'+proj=longlat +datum=WGS84 +no_defs'
```

This shapefile is in the popular WGS84 spatial reference system -- in other words, the data uses longitude, latitude pairs in units of degrees.

In addition, shapefiles also support attribute fields that may contain additional data. Here are the fields on the World Borders layer:

``` pycon
>>> print(lyr.fields)
['FIPS', 'ISO2', 'ISO3', 'UN', 'NAME', 'AREA', 'POP2005', 'REGION', 'SUBREGION', 'LON', 'LAT']
```

The following code will let you examine the OGR types (e.g. integer or string) associated with each of the fields:

``` pycon
>>> [fld.__name__ for fld in lyr.field_types]
['OFTString', 'OFTString', 'OFTString', 'OFTInteger', 'OFTString', 'OFTInteger', 'OFTInteger64', 'OFTInteger', 'OFTInteger', 'OFTReal', 'OFTReal']
```

You can iterate over each feature in the layer and extract information from both the feature's geometry (accessed via the `geom` attribute) as well as the feature's attribute fields (whose **values** are accessed via `get()` method):

``` pycon
>>> for feat in lyr:
...     print(feat.get("NAME"), feat.geom.num_points)
...
Guernsey 18
Jersey 26
South Georgia South Sandwich Islands 338
Taiwan 363
```

`~django.contrib.gis.gdal.Layer` objects may be sliced:

``` pycon
>>> lyr[0:2]
[<django.contrib.gis.gdal.feature.Feature object at 0x2f47690>, <django.contrib.gis.gdal.feature.Feature object at 0x2f47650>]
```

And individual features may be retrieved by their feature ID:

``` pycon
>>> feat = lyr[234]
>>> print(feat.get("NAME"))
San Marino
```

Boundary geometries may be exported as WKT and GeoJSON:

``` pycon
>>> geom = feat.geom
>>> print(geom.wkt)
POLYGON ((12.415798 43.957954,12.450554 ...
>>> print(geom.json)
{ "type": "Polygon", "coordinates": [ [ [ 12.415798, 43.957954 ], [ 12.450554, 43.979721 ], ...
```

### `LayerMapping`

To import the data, use a `LayerMapping` in a Python script. Create a file called `load.py` inside the `world` application, with the following code:

    from pathlib import Path
    from django.contrib.gis.utils import LayerMapping
    from .models import WorldBorder

    world_mapping = {
        "fips": "FIPS",
        "iso2": "ISO2",
        "iso3": "ISO3",
        "un": "UN",
        "name": "NAME",
        "area": "AREA",
        "pop2005": "POP2005",
        "region": "REGION",
        "subregion": "SUBREGION",
        "lon": "LON",
        "lat": "LAT",
        "mpoly": "MULTIPOLYGON",
    }

    world_shp = Path(__file__).resolve().parent / "data" / "TM_WORLD_BORDERS-0.3.shp"


    def run(verbose=True):
        lm = LayerMapping(WorldBorder, world_shp, world_mapping, transform=False)
        lm.save(strict=True, verbose=verbose)

A few notes about what's going on:

- Each key in the `world_mapping` dictionary corresponds to a field in the `WorldBorder` model. The value is the name of the shapefile field that data will be loaded from.
- The key `mpoly` for the geometry field is `MULTIPOLYGON`, the geometry type GeoDjango will import the field as. Even simple polygons in the shapefile will automatically be converted into collections prior to insertion into the database.
- The path to the shapefile is not absolute -- in other words, if you move the `world` application (with `data` subdirectory) to a different location, the script will still work.
- The `transform` keyword is set to `False` because the data in the shapefile does not need to be converted -- it's already in WGS84 (SRID=4326).

Afterward, invoke the Django shell from the `geodjango` project directory:

<div class="console">

\$ python manage.py shell

</div>

Next, import the `load` module, call the `run` routine, and watch `LayerMapping` do the work:

``` pycon
>>> from world import load
>>> load.run()
```

### Try `ogrinspect`

Now that you've seen how to define geographic models and import data with the `/ref/contrib/gis/layermapping`, it's possible to further automate this process with use of the `ogrinspect` management command. The `ogrinspect` command introspects a GDAL-supported vector data source (e.g., a shapefile) and generates a model definition and `LayerMapping` dictionary automatically.

The general usage of the command goes as follows:

<div class="console">

\$ python manage.py ogrinspect \[options\] \<data_source\> \<model_name\> \[options\]

</div>

`data_source` is the path to the GDAL-supported data source and `model_name` is the name to use for the model. Command-line options may be used to further define how the model is generated.

For example, the following command nearly reproduces the `WorldBorder` model and mapping dictionary created above, automatically:

<div class="console">

\$ python manage.py ogrinspect world/data/TM_WORLD_BORDERS-0.3.shp WorldBorder --srid=4326 --mapping --multi

</div>

A few notes about the command-line options given above:

- The `--srid=4326` option sets the SRID for the geographic field.
- The `--mapping` option tells `ogrinspect` to also generate a mapping dictionary for use with `~django.contrib.gis.utils.LayerMapping`.
- The `--multi` option is specified so that the geographic field is a `~django.contrib.gis.db.models.MultiPolygonField` instead of just a `~django.contrib.gis.db.models.PolygonField`.

The command produces the following output, which may be copied directly into the `models.py` of a GeoDjango application:

    # This is an auto-generated Django model module created by ogrinspect.
    from django.contrib.gis.db import models


    class WorldBorder(models.Model):
        fips = models.CharField(max_length=2)
        iso2 = models.CharField(max_length=2)
        iso3 = models.CharField(max_length=3)
        un = models.IntegerField()
        name = models.CharField(max_length=50)
        area = models.IntegerField()
        pop2005 = models.IntegerField()
        region = models.IntegerField()
        subregion = models.IntegerField()
        lon = models.FloatField()
        lat = models.FloatField()
        geom = models.MultiPolygonField(srid=4326)


    # Auto-generated `LayerMapping` dictionary for WorldBorder model
    worldborders_mapping = {
        "fips": "FIPS",
        "iso2": "ISO2",
        "iso3": "ISO3",
        "un": "UN",
        "name": "NAME",
        "area": "AREA",
        "pop2005": "POP2005",
        "region": "REGION",
        "subregion": "SUBREGION",
        "lon": "LON",
        "lat": "LAT",
        "geom": "MULTIPOLYGON",
    }

## Spatial Queries

### Spatial Lookups

GeoDjango adds spatial lookups to the Django ORM. For example, you can find the country in the `WorldBorder` table that contains a particular point. First, fire up the management shell:

<div class="console">

\$ python manage.py shell

</div>

Now, define a point of interest[^3]:

``` pycon
>>> pnt_wkt = "POINT(-95.3385 29.7245)"
```

The `pnt_wkt` string represents the point at -95.3385 degrees longitude, 29.7245 degrees latitude. The geometry is in a format known as Well Known Text (WKT), a standard issued by the Open Geospatial Consortium (OGC).[^4] Import the `WorldBorder` model, and perform a `contains` lookup using the `pnt_wkt` as the parameter:

``` pycon
>>> from world.models import WorldBorder
>>> WorldBorder.objects.filter(mpoly__contains=pnt_wkt)
<QuerySet [<WorldBorder: United States>]>
```

Here, you retrieved a `QuerySet` with only one model instance: the border of the United States (exactly what you would expect).

Similarly, you may also use a `GEOS geometry object
</ref/contrib/gis/geos>`. Here, you can combine the `intersects` spatial lookup with the `get` method to retrieve only the `WorldBorder` instance for San Marino instead of a queryset:

``` pycon
>>> from django.contrib.gis.geos import Point
>>> pnt = Point(12.4604, 43.9420)
>>> WorldBorder.objects.get(mpoly__intersects=pnt)
<WorldBorder: San Marino>
```

The `contains` and `intersects` lookups are just a subset of the available queries -- the `/ref/contrib/gis/db-api` documentation has more.

### Automatic Spatial Transformations

When doing spatial queries, GeoDjango automatically transforms geometries if they're in a different coordinate system. In the following example, coordinates will be expressed in [EPSG SRID 32140](https://spatialreference.org/ref/epsg/32140/), a coordinate system specific to south Texas **only** and in units of **meters**, not degrees:

``` pycon
>>> from django.contrib.gis.geos import GEOSGeometry, Point
>>> pnt = Point(954158.1, 4215137.1, srid=32140)
```

Note that `pnt` may also be constructed with EWKT, an "extended" form of WKT that includes the SRID:

``` pycon
>>> pnt = GEOSGeometry("SRID=32140;POINT(954158.1 4215137.1)")
```

GeoDjango's ORM will automatically wrap geometry values in transformation SQL, allowing the developer to work at a higher level of abstraction:

``` pycon
>>> qs = WorldBorder.objects.filter(mpoly__intersects=pnt)
>>> print(qs.query)  # Generating the SQL
SELECT "world_worldborder"."id", "world_worldborder"."name", "world_worldborder"."area",
"world_worldborder"."pop2005", "world_worldborder"."fips", "world_worldborder"."iso2",
"world_worldborder"."iso3", "world_worldborder"."un", "world_worldborder"."region",
"world_worldborder"."subregion", "world_worldborder"."lon", "world_worldborder"."lat",
"world_worldborder"."mpoly" FROM "world_worldborder"
WHERE ST_Intersects("world_worldborder"."mpoly", ST_Transform(%s, 4326))
>>> qs  # printing evaluates the queryset
<QuerySet [<WorldBorder: United States>]>
```

<div id="gis-raw-sql">

<div class="admonition">

Raw queries

When using `raw queries </topics/db/sql>`, you must wrap your geometry fields so that the field value can be recognized by GEOS:

``` pycon
>>> from django.db import connection
>>> # or if you're querying a non-default database:
>>> from django.db import connections
>>> connection = connections["your_gis_db_alias"]
>>> City.objects.raw(
...     "SELECT id, name, %s as point from myapp_city" % (connection.ops.select % "point")
... )
```

You should only use raw queries when you know exactly what you're doing.

</div>

</div>

### Lazy Geometries

GeoDjango loads geometries in a standardized textual representation. When the geometry field is first accessed, GeoDjango creates a `~django.contrib.gis.geos.GEOSGeometry` object, exposing powerful functionality, such as serialization properties for popular geospatial formats:

``` pycon
>>> sm = WorldBorder.objects.get(name="San Marino")
>>> sm.mpoly
<MultiPolygon object at 0x24c6798>
>>> sm.mpoly.wkt  # WKT
MULTIPOLYGON (((12.4157980000000006 43.9579540000000009, 12.4505540000000003 43.9797209999999978, ...
>>> sm.mpoly.wkb  # WKB (as Python binary buffer)
<read-only buffer for 0x1fe2c70, size -1, offset 0 at 0x2564c40>
>>> sm.mpoly.geojson  # GeoJSON
'{ "type": "MultiPolygon", "coordinates": [ [ [ [ 12.415798, 43.957954 ], [ 12.450554, 43.979721 ], ...
```

This includes access to all of the advanced geometric operations provided by the GEOS library:

``` pycon
>>> pnt = Point(12.4604, 43.9420)
>>> sm.mpoly.contains(pnt)
True
>>> pnt.contains(sm.mpoly)
False
```

### Geographic annotations

GeoDjango also offers a set of geographic annotations to compute distances and several other operations (intersection, difference, etc.). See the `/ref/contrib/gis/functions` documentation.

## Putting your data on the map

### Geographic Admin

`Django's admin application </ref/contrib/admin/index>` supports editing geometry fields.

#### Basics

The Django admin allows users to create and modify geometries on a JavaScript slippy map (powered by [OpenLayers](https://openlayers.org/)).

Let's dive right in. Create a file called `admin.py` inside the `world` application with the following code:

    from django.contrib.gis import admin
    from .models import WorldBorder

    admin.site.register(WorldBorder, admin.ModelAdmin)

Next, edit your `urls.py` in the `geodjango` application folder as follows:

    from django.contrib import admin
    from django.urls import include, path

    urlpatterns = [
        path("admin/", admin.site.urls),
    ]

Create an admin user:

<div class="console">

\$ python manage.py createsuperuser

</div>

Next, start up the Django development server:

<div class="console">

\$ python manage.py runserver

</div>

Finally, browse to `http://localhost:8000/admin/`, and log in with the user you just created. Browse to any of the `WorldBorder` entries -- the borders may be edited by clicking on a polygon and dragging the vertices to the desired position.

#### `GISModelAdmin`

With the `~django.contrib.gis.admin.GISModelAdmin`, GeoDjango uses an [OpenStreetMap](https://www.openstreetmap.org/) layer in the admin. This provides more context (including street and thoroughfare details) than available with the `~django.contrib.admin.ModelAdmin` (which uses the [Vector Map Level 0](http://web.archive.org/web/20201024202709/https://earth-info.nga.mil/publications/vmap0.html) WMS dataset hosted at [OSGeo](https://www.osgeo.org/)).

The PROJ datum shifting files must be installed (see the `PROJ
installation instructions <proj4>` for more details).

If you meet this requirement, then use the `GISModelAdmin` option class in your `admin.py` file:

    admin.site.register(WorldBorder, admin.GISModelAdmin)

**Footnotes**

[^1]: Special thanks to Bjørn Sandvik of [mastermaps.net](https://mastermaps.net/) for providing and maintaining this dataset.

[^2]: GeoDjango basic apps was written by Dane Springmeyer, Josh Livni, and Christopher Schmidt.

[^3]: This point is the [University of Houston Law Center](https://www.law.uh.edu/).

[^4]: Open Geospatial Consortium, Inc., [OpenGIS Simple Feature Specification For SQL](https://www.ogc.org/standards/sfs).
