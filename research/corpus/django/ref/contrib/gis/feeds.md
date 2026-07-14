# Geographic Feeds

<div class="module" synopsis="GeoDjango's framework for generating spatial feeds.">

django.contrib.gis.feeds

</div>

GeoDjango has its own `Feed` subclass that may embed location information in RSS/Atom feeds formatted according to either the [Simple GeoRSS](https://www.ogc.org/standard/georss/) or [W3C Geo](https://www.w3.org/2003/01/geo/) standards. Because GeoDjango's syndication API is a superset of Django's, please consult `Django's syndication documentation
</ref/contrib/syndication>` for details on general usage.

## Example

## API Reference

### `Feed` Subclass

<div class="Feed">

In addition to methods provided by the `django.contrib.syndication.views.Feed` base class, GeoDjango's `Feed` class provides the following overrides. Note that these overrides may be done in multiple ways:

    from django.contrib.gis.feeds import Feed


    class MyFeed(Feed):
        # First, as a class attribute.
        geometry = ...
        item_geometry = ...

        # Also a function with no arguments
        def geometry(self): ...

        def item_geometry(self): ...

        # And as a function with a single argument
        def geometry(self, obj): ...

        def item_geometry(self, item): ...

<div class="method">

geometry(obj)

</div>

Takes the object returned by `get_object()` and returns the *feed's* geometry. Typically this is a `GEOSGeometry` instance, or can be a tuple to represent a point or a box. For example:

    class ZipcodeFeed(Feed):
        def geometry(self, obj):
            # Can also return: `obj.poly`, and `obj.poly.centroid`.
            return obj.poly.extent  # tuple like: (X0, Y0, X1, Y1).

<div class="method">

item_geometry(item)

</div>

Set this to return the geometry for each *item* in the feed. This can be a `GEOSGeometry` instance, or a tuple that represents a point coordinate or bounding box. For example:

    class ZipcodeFeed(Feed):
        def item_geometry(self, obj):
            # Returns the polygon.
            return obj.poly

</div>

### `SyndicationFeed` Subclasses

The following `django.utils.feedgenerator.SyndicationFeed` subclasses are available:

<div class="GeoRSSFeed">

<div class="GeoAtom1Feed">

<div class="W3CGeoFeed">

<div class="note">

<div class="title">

Note

</div>

[W3C Geo](https://www.w3.org/2003/01/geo/) formatted feeds only support `~django.contrib.gis.db.models.PointField` geometries.

</div>

</div>

</div>

</div>
