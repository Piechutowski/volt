# Installing Geospatial libraries

## Geospatial libraries

GeoDjango uses and/or provides interfaces for the following open source geospatial libraries:

| Program                                          | Description                         | Required                         | Supported Versions                                        |
|--------------------------------------------------|-------------------------------------|----------------------------------|-----------------------------------------------------------|
| `GEOS <geos-overview>`                           | Geometry Engine Open Source         | Yes                              | 3.14, 3.13, 3.12, 3.11, 3.10                              |
| [PROJ](#proj)                                    | Cartographic Projections library    | Yes (PostgreSQL and SQLite only) | 9.x, 8.x, 7.x, 6.x                                        |
| `GDAL <gdal-overview>`                           | Geospatial Data Abstraction Library | Yes                              | 3.13, 3.12, 3.11, 3.10, 3.9, 3.8, 3.7, 3.6, 3.5, 3.4, 3.3 |
| `GeoIP <geoip2-overview>`                        | IP-based geolocation library        | No                               | 2                                                         |
| [PostGIS](https://postgis.net/)                  | Spatial extensions for PostgreSQL   | Yes (PostgreSQL only)            | 3.6, 3.5, 3.4, 3.3, 3.2                                   |
| [SpatiaLite](https://www.gaia-gis.it/gaia-sins/) | Spatial extensions for SQLite       | Yes (SQLite only)                | 5.1, 5.0, 4.3                                             |

Note that older or more recent versions of these libraries *may* also work totally fine with GeoDjango. Your mileage may vary.

<div class="note">

<div class="title">

Note

</div>

The GeoDjango interfaces to GEOS, GDAL, and GeoIP may be used independently of Django. In other words, no database or settings file required -- import them as normal from `django.contrib.gis`.

</div>

On Debian/Ubuntu, you are advised to install the following packages which will install, directly or by dependency, the required geospatial libraries:

``` console
$ sudo apt-get install binutils libproj-dev gdal-bin
```

Please also consult platform-specific instructions if you are on `macos` or `windows`.

## Building from source

When installing from source on UNIX and GNU/Linux systems, please follow the installation instructions carefully, and install the libraries in the given order. If using MySQL or Oracle as the spatial database, only GEOS is required.

<div class="note">

<div class="title">

Note

</div>

On Linux platforms, it may be necessary to run the `ldconfig` command after installing each library. For example:

``` shell
$ sudo make install
$ sudo ldconfig
```

</div>

<div class="note">

<div class="title">

Note

</div>

macOS users must install [Xcode](https://developer.apple.com/xcode/) in order to compile software from source.

</div>

### GEOS

GEOS is a C++ library for performing geometric operations, and is the default internal geometry representation used by GeoDjango (it's behind the "lazy" geometries). Specifically, the C API library is called (e.g., `libgeos_c.so`) directly from Python using ctypes.

First, download GEOS from the GEOS website and untar the source archive:

``` shell
$ wget https://download.osgeo.org/geos/geos-X.Y.Z.tar.bz2
$ tar xjf geos-X.Y.Z.tar.bz2
```

Then step into the GEOS directory, create a `build` folder, and step into it:

``` shell
$ cd geos-X.Y.Z
$ mkdir build
$ cd build
```

Then build and install the package:

``` shell
$ cmake -DCMAKE_BUILD_TYPE=Release ..
$ cmake --build .
$ sudo cmake --build . --target install
```

#### Troubleshooting

##### Can't find GEOS library

When GeoDjango can't find GEOS, this error is raised:

``` text
ImportError: Could not find the GEOS library (tried "geos_c"). Try setting GEOS_LIBRARY_PATH in your settings.
```

The most common solution is to properly configure your `libsettings` *or* set `geoslibrarypath` in your settings.

If using a binary package of GEOS (e.g., on Ubuntu), you may need to `binutils`.

##### `GEOS_LIBRARY_PATH`

If your GEOS library is in a non-standard location, or you don't want to modify the system's library path then the `GEOS_LIBRARY_PATH` setting may be added to your Django settings file with the full path to the GEOS C library. For example:

``` shell
GEOS_LIBRARY_PATH = '/home/bob/local/lib/libgeos_c.so'
```

<div class="note">

<div class="title">

Note

</div>

The setting must be the *full* path to the **C** shared library; in other words you want to use `libgeos_c.so`, not `libgeos.so`.

</div>

See also `My logs are filled with GEOS-related errors
<geos-exceptions-in-logfile>`.

### PROJ

[PROJ](#proj) is a library for converting geospatial data to different coordinate reference systems.

First, download the PROJ source code:

``` shell
$ wget https://download.osgeo.org/proj/proj-X.Y.Z.tar.gz
```

... and datum shifting files (download `proj-datumgrid-X.Y.tar.gz` for PROJ \< 7.x)[^1]:

``` shell
$ wget https://download.osgeo.org/proj/proj-data-X.Y.tar.gz
```

Next, untar the source code archive, and extract the datum shifting files in the `data` subdirectory. This must be done *prior* to configuration:

``` shell
$ tar xzf proj-X.Y.Z.tar.gz
$ cd proj-X.Y.Z/data
$ tar xzf ../../proj-data-X.Y.tar.gz
$ cd ../..
```

For PROJ 9.x and greater, releases only support builds using `CMake` (see [PROJ RFC-7](https://proj.org/community/rfc/rfc-7.html#rfc7)).

To build with `CMake` ensure your system meets the [build requirements](https://proj.org/install.html#build-requirements). Then create a `build` folder in the PROJ directory, and step into it:

``` shell
$ cd proj-X.Y.Z
$ mkdir build
$ cd build
```

Finally, configure, make and install PROJ:

``` shell
$ cmake ..
$ cmake --build .
$ sudo cmake --build . --target install
```

### GDAL

[GDAL](https://gdal.org/) is an excellent open source geospatial library that has support for reading most vector and raster spatial data formats. Currently, GeoDjango only supports `GDAL's vector data <gdal_vector_data>` capabilities[^2]. `geosbuild` and `proj4` should be installed prior to building GDAL.

First download the latest GDAL release version and untar the archive:

``` shell
$ wget https://download.osgeo.org/gdal/X.Y.Z/gdal-X.Y.Z.tar.gz
$ tar xzf gdal-X.Y.Z.tar.gz
```

For GDAL 3.6.x and greater, releases only support builds using `CMake`. To build with `CMake` create a `build` folder in the GDAL directory, and step into it:

``` shell
$ cd gdal-X.Y.Z
$ mkdir build
$ cd build
```

Finally, configure, make and install GDAL:

``` shell
$ cmake -DCMAKE_BUILD_TYPE=Release ..
$ cmake --build .
$ sudo cmake --build . --target install
```

If you have any problems, please see the troubleshooting section below for suggestions and solutions.

#### Troubleshooting

##### Can't find GDAL library

When GeoDjango can't find the GDAL library, configure your `libsettings` *or* set `gdallibrarypath` in your settings.

##### `GDAL_LIBRARY_PATH`

If your GDAL library is in a non-standard location, or you don't want to modify the system's library path then the `GDAL_LIBRARY_PATH` setting may be added to your Django settings file with the full path to the GDAL library. For example:

``` shell
GDAL_LIBRARY_PATH = '/home/sue/local/lib/libgdal.so'
```

**Footnotes**

[^1]: The datum shifting files are needed for converting data to and from certain projections. For example, the PROJ string for the [Google projection (900913 or 3857)](https://spatialreference.org/ref/epsg/3857/) requires the `null` grid file only included in the extra datum shifting files. It is easier to install the shifting files now, then to have debug a problem caused by their absence later.

[^2]: Specifically, GeoDjango provides support for the [OGR](https://gdal.org/user/vector_data_model.html) library, a component of GDAL.
