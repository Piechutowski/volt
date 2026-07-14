# Installing PostGIS

[PostGIS](https://postgis.net/) adds geographic object support to PostgreSQL, turning it into a spatial database. `geosbuild`, `proj4` and `gdalbuild` should be installed prior to building PostGIS. You might also need additional libraries, see [PostGIS requirements](https://postgis.net/docs/postgis_installation.html#install_requirements).

The [psycopg](https://www.psycopg.org/psycopg3/) or [psycopg2](https://www.psycopg.org/) module is required for use as the database adapter when using GeoDjango with PostGIS.

On Debian/Ubuntu, you are advised to install the following packages: `postgresql-x`, `postgresql-x-postgis-3`, `postgresql-server-dev-x`, and `python3-psycopg3` (x matching the PostgreSQL version you want to install). Alternately, you can [build from source](https://postgis.net/docs/postgis_installation.html#install_short_version). Consult the platform-specific instructions if you are on `macos` or `windows`.

## Post-installation

### Creating a spatial database

PostGIS includes an extension for PostgreSQL that's used to enable spatial functionality:

``` shell
$ createdb  <db name>
$ psql <db name>
> CREATE EXTENSION postgis;
```

The database user must be a superuser in order to run `CREATE EXTENSION postgis;`. The command is run during the `migrate` process. An alternative is to use a migration operation in your project:

    from django.contrib.postgres.operations import CreateExtension
    from django.db import migrations


    class Migration(migrations.Migration):
        operations = [CreateExtension("postgis"), ...]

If you plan to use PostGIS raster functionality, you should also activate the `postgis_raster` extension. You can install the extension using the `~django.contrib.postgres.operations.CreateExtension` migration operation, or directly by running `CREATE EXTENSION postgis_raster;`.

GeoDjango does not currently leverage any [PostGIS topology functionality](https://postgis.net/docs/Topology.html). If you plan to use those features at some point, you can also install the `postgis_topology` extension by issuing `CREATE EXTENSION postgis_topology;`.

### Managing the database

To administer the database, you can either use the pgAdmin III program (`Start --> PostgreSQL X --> pgAdmin III`) or the SQL Shell (`Start --> PostgreSQL X --> SQL Shell`). For example, to create a `geodjango` spatial database and user, the following may be executed from the SQL Shell as the `postgres` user:

``` psql
postgres# CREATE USER geodjango PASSWORD 'my_passwd';
postgres# CREATE DATABASE geodjango OWNER geodjango;
```
