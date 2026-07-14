# Model forms

<div class="module" synopsis="ModelForm API reference for inner ``Meta`` class and factory
functions">

django.forms.models

</div>

<div class="currentmodule">

django.forms

</div>

`ModelForm` API reference. For introductory material about using a `ModelForm`, see the `/topics/forms/modelforms` topic guide.

## Model form `Meta` API

<div class="ModelFormOptions">

The `_meta` API is used to build forms that reflect a Django model. It is accessible through the `_meta` attribute of each model form, and is a `django.forms.models.ModelFormOptions` instance.

</div>

The structure of the generated form can be customized by defining metadata options as attributes of an inner `Meta` class. For example:

    from django.forms import ModelForm
    from myapp.models import Book


    class BookForm(ModelForm):
        class Meta:
            model = Book
            fields = ["title", "author"]
            help_texts = {
                "title": "The title of the book",
                "author": "The author of the book",
            }
            # ... other attributes

Required attributes are `~ModelFormOptions.model`, and either `~ModelFormOptions.fields` or `~ModelFormOptions.exclude`. All other `Meta` attributes are optional.

Optional attributes, other than `~ModelFormOptions.localized_fields` and `~ModelFormOptions.formfield_callback`, expect a dictionary that maps a model field name to a value. Any field that is not defined in the dictionary falls back to the field's default value.

<div class="admonition">

Invalid field names

Invalid or excluded field names in an optional dictionary attribute have no effect, since fields that are not included are not accessed.

</div>

<div class="admonition">

Invalid Meta class attributes

You may define any attribute on a `Meta` class. Typos or incorrect attribute names do not raise an error.

</div>

### `error_messages`

<div class="attribute">

ModelFormOptions.error_messages

A dictionary that maps a model field name to a dictionary of error message keys (`null`, `blank`, `invalid`, `unique`, etc.) mapped to custom error messages.

When a field is not specified, Django will fall back on the error messages defined in that model field's `django.db.models.Field.error_messages` and then finally on the default error messages for that field type.

</div>

### `exclude`

<div class="attribute">

ModelFormOptions.exclude

A tuple or list of `~ModelFormOptions.model` field names to be excluded from the form.

Either `~ModelFormOptions.fields` or `~ModelFormOptions.exclude` must be set. If neither are set, an `~django.core.exceptions.ImproperlyConfigured` exception will be raised. If `~ModelFormOptions.exclude` is set and `~ModelFormOptions.fields` is unset, all model fields, except for those specified in `~ModelFormOptions.exclude`, are included in the form.

</div>

### `field_classes`

<div class="attribute">

ModelFormOptions.field_classes

A dictionary that maps a model field name to a `~django.forms.Field` class, which overrides the `form_class` used in the model field's `.Field.formfield` method.

When a field is not specified, Django will fall back on the model field's `default field class <model-form-field-types>`.

</div>

### `fields`

<div class="attribute">

ModelFormOptions.fields

A tuple or list of `~ModelFormOptions.model` field names to be included in the form. The value `'__all__'` can be used to specify that all fields should be included.

If any field is specified in `~ModelFormOptions.exclude`, this will not be included in the form despite being specified in `~ModelFormOptions.fields`.

Either `~ModelFormOptions.fields` or `~ModelFormOptions.exclude` must be set. If neither are set, an `~django.core.exceptions.ImproperlyConfigured` exception will be raised.

</div>

### `formfield_callback`

<div class="attribute">

ModelFormOptions.formfield_callback

A function or callable that takes a model field and returns a `django.forms.Field` object.

</div>

### `help_texts`

<div class="attribute">

ModelFormOptions.help_texts

A dictionary that maps a model field name to a help text string.

When a field is not specified, Django will fall back on that model field's `~django.db.models.Field.help_text`.

</div>

### `labels`

<div class="attribute">

ModelFormOptions.labels

A dictionary that maps a model field names to a label string.

When a field is not specified, Django will fall back on that model field's `~django.db.models.Field.verbose_name` and then the field's attribute name.

</div>

### `localized_fields`

<div class="attribute">

ModelFormOptions.localized_fields

A tuple or list of `~ModelFormOptions.model` field names to be localized. The value `'__all__'` can be used to specify that all fields should be localized.

By default, form fields are not localized, see `enabling localization of fields
<modelforms-enabling-localization-of-fields>` for more details.

</div>

### `model`

<div class="attribute">

ModelFormOptions.model

Required. The `django.db.models.Model` to be used for the `~django.forms.ModelForm`.

</div>

### `widgets`

<div class="attribute">

ModelFormOptions.widgets

A dictionary that maps a model field name to a `django.forms.Widget`.

When a field is not specified, Django will fall back on the default widget for that particular type of `django.db.models.Field`.

</div>

## Model form factory functions

<div class="currentmodule">

django.forms.models

</div>

### `modelform_factory`

<div class="function">

modelform_factory(model, form=ModelForm, fields=None, exclude=None, formfield_callback=None, widgets=None, localized_fields=None, labels=None, help_texts=None, error_messages=None, field_classes=None)

Returns a `~django.forms.ModelForm` class for the given `model`. You can optionally pass a `form` argument to use as a starting point for constructing the `ModelForm`.

`fields` is an optional list of field names. If provided, only the named fields will be included in the returned fields.

`exclude` is an optional list of field names. If provided, the named fields will be excluded from the returned fields, even if they are listed in the `fields` argument.

`formfield_callback` is a callable that takes a model field and returns a form field.

`widgets` is a dictionary of model field names mapped to a widget.

`localized_fields` is a list of names of fields which should be localized.

`labels` is a dictionary of model field names mapped to a label.

`help_texts` is a dictionary of model field names mapped to a help text.

`error_messages` is a dictionary of model field names mapped to a dictionary of error messages.

`field_classes` is a dictionary of model field names mapped to a form field class.

See `modelforms-factory` for example usage.

You must provide the list of fields explicitly, either via keyword arguments `fields` or `exclude`, or the corresponding attributes on the form's inner `Meta` class. See `modelforms-selecting-fields` for more information. Omitting any definition of the fields to use will result in an `~django.core.exceptions.ImproperlyConfigured` exception.

</div>

### `modelformset_factory`

<div class="function">

modelformset_factory(model, form=ModelForm, formfield_callback=None, formset=BaseModelFormSet, extra=1, can_delete=False, can_order=False, max_num=None, fields=None, exclude=None, widgets=None, validate_max=False, localized_fields=None, labels=None, help_texts=None, error_messages=None, min_num=None, validate_min=False, field_classes=None, absolute_max=None, can_delete_extra=True, renderer=None, edit_only=False)

Returns a `FormSet` class for the given `model` class.

Arguments `model`, `form`, `fields`, `exclude`, `formfield_callback`, `widgets`, `localized_fields`, `labels`, `help_texts`, `error_messages`, and `field_classes` are all passed through to `~django.forms.models.modelform_factory`.

Arguments `formset`, `extra`, `can_delete`, `can_order`, `max_num`, `validate_max`, `min_num`, `validate_min`, `absolute_max`, `can_delete_extra`, and `renderer` are passed through to `~django.forms.formsets.formset_factory`. See `formsets </topics/forms/formsets>` for details.

The `edit_only` argument allows `preventing new objects creation
<model-formsets-edit-only>`.

See `model-formsets` for example usage.

</div>

### `inlineformset_factory`

<div class="function">

inlineformset_factory(parent_model, model, form=ModelForm, formset=BaseInlineFormSet, fk_name=None, fields=None, exclude=None, extra=3, can_order=False, can_delete=True, max_num=None, formfield_callback=None, widgets=None, validate_max=False, localized_fields=None, labels=None, help_texts=None, error_messages=None, min_num=None, validate_min=False, field_classes=None, absolute_max=None, can_delete_extra=True, renderer=None, edit_only=False)

Returns an `InlineFormSet` using `modelformset_factory` with defaults of `formset=``~django.forms.models.BaseInlineFormSet`, `can_delete=True`, and `extra=3`.

If your model has more than one `~django.db.models.ForeignKey` to the `parent_model`, you must specify a `fk_name`.

See `inline-formsets` for example usage.

</div>
