# Geolocation with GeoIP2

<div class="module" synopsis="Python interface for MaxMind's GeoIP2 databases.">

django.contrib.gis.geoip2

</div>

## Overview

The `GeoIP2` object is a wrapper for the `MaxMind geoip2 Python
library <geoip2>`.[^1]

In order to perform IP-based geolocation, the `GeoIP2` object requires the `geoip2` Python package and the GeoIP `Country` and/or `City` datasets in binary format (the CSV files will not work!), downloaded from e.g. [MaxMind](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) or [DB-IP](https://db-ip.com/db/lite.php) websites. Grab the `GeoLite2-Country.mmdb.gz` and `GeoLite2-City.mmdb.gz` files and unzip them in a directory corresponding to the `GEOIP_PATH` setting.

Additionally, it is recommended to install the [libmaxminddb C library](https://github.com/maxmind/libmaxminddb/), so that `geoip2` can leverage the C library's faster speed.

## Example

Here is an example of its usage:

``` pycon
>>> from django.contrib.gis.geoip2 import GeoIP2
>>> g = GeoIP2()
>>> g.country("google.com")
{'continent_code': 'NA',
 'continent_name': 'North America',
 'country_code': 'US',
 'country_name': 'United States',
 'is_in_european_union': False}
>>> g.city("72.14.207.99")
{'accuracy_radius': 1000,
 'city': 'Mountain View',
 'continent_code': 'NA',
 'continent_name': 'North America',
 'country_code': 'US',
 'country_name': 'United States',
 'is_in_european_union': False,
 'latitude': 37.419200897216797,
 'longitude': -122.05740356445312,
 'metro_code': 807,
 'postal_code': '94043',
 'region_code': 'CA',
 'region_name': 'California',
 'time_zone': 'America/Los_Angeles',
 'dma_code': 807,
 'region': 'CA'}
>>> g.lat_lon("salon.com")
(39.0437, -77.4875)
>>> g.lon_lat("uh.edu")
(-95.4342, 29.834)
>>> g.geos("24.124.1.80").wkt
'POINT (-97 38)'
```

## API Reference

<div class="GeoIP2(path=None, cache=0, country=None, city=None)">

The `GeoIP` object does not require any parameters to use the default settings. However, at the very least the `GEOIP_PATH` setting should be set with the path of the location of your GeoIP datasets. The following initialization keywords may be used to customize any of the defaults.

</div>

| Keyword Arguments | Description                                                                                                                                                                                                                                                  |
|-------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `path`            | Base directory to where GeoIP data is located or the full path to where the city or country data files (`.mmdb`) are located. Assumes that both the city and country datasets are located in this directory; overrides the `GEOIP_PATH` setting.             |
| `cache`           | The cache settings when opening up the GeoIP datasets. May be an integer in (0, 1, 2, 4, 8) corresponding to the `MODE_AUTO`, `MODE_MMAP_EXT`, `MODE_MMAP`, and `GEOIP_INDEX_CACHE` `MODE_MEMORY` C API settings, respectively. Defaults to 0 (`MODE_AUTO`). |
| `country`         | The name of the GeoIP country data file. Defaults to `GeoLite2-Country.mmdb`. Setting this keyword overrides the `GEOIP_COUNTRY` setting.                                                                                                                    |
| `city`            | The name of the GeoIP city data file. Defaults to `GeoLite2-City.mmdb`. Setting this keyword overrides the `GEOIP_CITY` setting.                                                                                                                             |

## Methods

### Querying

All the following querying routines may take an instance of `~ipaddress.IPv4Address` or `~ipaddress.IPv6Address`, a string IP address, or a fully qualified domain name (FQDN). For example, `IPv4Address("205.186.163.125")`, `"205.186.163.125"`, and `"djangoproject.com"` would all be valid query parameters.

<div class="method">

GeoIP2.city(query)

</div>

Returns a dictionary of city information for the given query. Some of the values in the dictionary may be undefined (`None`).

<div class="method">

GeoIP2.country(query)

</div>

Returns a dictionary with the country code and country for the given query.

<div class="method">

GeoIP2.country_code(query)

</div>

Returns the country code corresponding to the query.

<div class="method">

GeoIP2.country_name(query)

</div>

Returns the country name corresponding to the query.

### Coordinate Retrieval

<div class="method">

GeoIP2.lon_lat(query)

</div>

Returns a coordinate tuple of (longitude, latitude).

<div class="method">

GeoIP2.lat_lon(query)

</div>

Returns a coordinate tuple of (latitude, longitude),

<div class="method">

GeoIP2.geos(query)

</div>

Returns a `~django.contrib.gis.geos.Point` object corresponding to the query.

## Settings

<div class="setting">

GEOIP_PATH

</div>

### `GEOIP_PATH`

A string or `pathlib.Path` specifying the directory where the GeoIP data files are located. This setting is *required* unless manually specified with `path` keyword when initializing the `GeoIP2` object.

<div class="setting">

GEOIP_COUNTRY

</div>

### `GEOIP_COUNTRY`

The basename to use for the GeoIP country data file. Defaults to `'GeoLite2-Country.mmdb'`.

<div class="setting">

GEOIP_CITY

</div>

### `GEOIP_CITY`

The basename to use for the GeoIP city data file. Defaults to `'GeoLite2-City.mmdb'`.

## Exceptions

<div class="exception">

GeoIP2Exception

The exception raised when an error occurs in the `GeoIP2` wrapper. Exceptions from the underlying `geoip2` library are passed through unchanged.

</div>

**Footnotes**

[^1]: GeoIP(R) is a registered trademark of MaxMind, Inc.
