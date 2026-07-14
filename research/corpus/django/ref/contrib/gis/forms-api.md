# GeoDjango Forms API

<div class="module" synopsis="GeoDjango forms API.">

django.contrib.gis.forms

</div>

GeoDjango provides some specialized form fields and widgets in order to visually display and edit geolocalized data on a map. By default, they use [OpenLayers](https://openlayers.org/)-powered maps, with a base WMS layer provided by [NASA](https://www.earthdata.nasa.gov/).

## Field arguments

In addition to the regular `form field arguments <core-field-arguments>`, GeoDjango form fields take the following optional arguments.

### `srid`

<div class="attribute">

Field.srid

This is the SRID code that the field value should be transformed to. For example, if the map widget SRID is different from the SRID more generally used by your application or database, the field will automatically convert input values into that SRID.

</div>

### `geom_type`

<div class="attribute">

Field.geom_type

You generally shouldn't have to set or change that attribute which should be set up depending on the field class. It matches the OpenGIS standard geometry name.

</div>

## Form field classes

### `GeometryField`

### `PointField`

### `LineStringField`

### `PolygonField`

### `MultiPointField`

### `MultiLineStringField`

### `MultiPolygonField`

### `GeometryCollectionField`

## Form widgets

<div class="module" synopsis="GeoDjango widgets API.">

django.contrib.gis.forms.widgets

</div>

GeoDjango form widgets allow you to display and edit geographic data on a visual map. Note that none of the currently available widgets supports 3D geometries, hence geometry fields will fallback using a `Textarea` widget for such data.

### Widget attributes

GeoDjango widgets are template-based, so their attributes are mostly different from other Django widget attributes.

<div class="attribute">

BaseGeometryWidget.base_layer

<div class="versionadded">

6.0

</div>

A string that specifies the identifier for the default base map layer to be used by the corresponding JavaScript map widget. It is passed as part of the widget options when rendering, allowing the `MapWidget` to determine which map tile provider or base layer to initialize (default is `None`).

</div>

<div class="attribute">

BaseGeometryWidget.geom_type

The OpenGIS geometry type, generally set by the form field.

</div>

<div class="attribute">

BaseGeometryWidget.map_srid

SRID code used by the map (default is 4326).

</div>

<div class="attribute">

BaseGeometryWidget.display_raw

Boolean value specifying if a textarea input showing the serialized representation of the current geometry is visible, mainly for debugging purposes (default is `False`).

</div>

<div class="attribute">

BaseGeometryWidget.supports_3d

Indicates if the widget supports edition of 3D data (default is `False`).

</div>

<div class="attribute">

BaseGeometryWidget.template_name

The template used to render the map widget.

</div>

You can pass widget attributes in the same manner that for any other Django widget. For example:

    from django.contrib.gis import forms


    class MyGeoForm(forms.Form):
        point = forms.PointField(widget=forms.OSMWidget(attrs={"display_raw": True}))

### Widget classes

`BaseGeometryWidget`

<div class="BaseGeometryWidget">

This is an abstract base widget containing the logic needed by subclasses. You cannot directly use this widget for a geometry field. Note that the rendering of GeoDjango widgets is based on a base layer name, identified by the `base_layer` class attribute.

</div>

`OpenLayersWidget`

<div class="OpenLayersWidget">

This is the default widget used by all GeoDjango form fields. Attributes are:

<div class="attribute">

base_layer

<div class="versionadded">

6.0

</div>

`nasaWorldview`

</div>

<div class="attribute">

template_name

`gis/openlayers.html`.

</div>

<div class="attribute">

map_srid

`3857`

</div>

`OpenLayersWidget` and `OSMWidget` include the `ol.js` and `ol.css` files hosted on the `cdn.jsdelivr.net` content-delivery network. These files can be overridden by subclassing the widget and setting the `js` and `css` properties of the inner `Media` class (see `assets-as-a-static-definition`).

<div class="admonition">

External assets with CSP

When `~django.middleware.csp.ContentSecurityPolicyMiddleware` is enabled, the default OpenLayers CDN assets (`ol.js` and `ol.css`) will be blocked unless explicitly allowed. This can be addressed in one of two ways: **serve assets locally** by subclassing the widget and provide local copies of the JavaScript and CSS files, or **allow the CDN in the CSP policy**.

For example, to allow the default NASA Worldview base layer (replace `x.y.z` with the actual version):

    from django.utils.csp import CSP

    SECURE_CSP = {
        "default-src": [CSP.SELF],
        "script-src": [CSP.SELF, "https://cdn.jsdelivr.net/npm/ol@x.y.z/dist/ol.js"],
        "style-src": [CSP.SELF, "https://cdn.jsdelivr.net/npm/ol@x.y.z/ol.css"],
        "img-src": [CSP.SELF, "https://*.earthdata.nasa.gov"],
    }

</div>

</div>

`OSMWidget`

<div class="OSMWidget">

This widget specialized `OpenLayersWidget` and uses an OpenStreetMap base layer to display geographic objects on. Attributes are:

<div class="attribute">

base_layer

<div class="versionadded">

6.0

</div>

`osm`

</div>

<div class="attribute">

default_lat

</div>

<div class="attribute">

default_lon

The default center latitude and longitude are `47` and `5`, respectively, which is a location in eastern France.

</div>

<div class="attribute">

default_zoom

The default map zoom is `12`.

</div>

The `OpenLayersWidget` note about using external assets also applies here. See also this [FAQ answer]() about `https` access to map tiles.

<div class="admonition">

OpenStreetMap tiles with CSP

This widget uses OpenStreetMap tiles instead of NASA Worldview. If `security-csp` enabled, both the OpenLayers CDN resources (as required by `OpenLayersWidget`) and the OpenStreetMap tile servers must be allowed:

    from django.utils.csp import CSP

    SECURE_CSP = {
        # other directives
        "img-src": [CSP.SELF, "https://tile.openstreetmap.org"],
    }

</div>

<div class="versionchanged">

6.0

The `OSMWidget` no longer uses a custom template. Consequently, the `gis/openlayers-osm.html` template was removed.

</div>

</div>

### Customizing the base layer used in OpenLayers-based widgets

<div class="versionadded">

6.0

</div>

To customize the base layer displayed in OpenLayers-based geometry widgets, define a new layer builder in a custom JavaScript file. For example:

``` javascript
MapWidget.layerBuilder.custom_layer_name = function () {
    // Return an OpenLayers layer instance.
    return new ol.layer.Tile({source: new ol.source.<ChosenSource>()});
};
```

Then, subclass a standard geometry widget and set the `base_layer`:

    from django.contrib.gis.forms.widgets import OpenLayersWidget


    class YourCustomWidget(OpenLayersWidget):
        base_layer = "custom_layer_name"

        class Media:
            js = ["path-to-file.js"]
