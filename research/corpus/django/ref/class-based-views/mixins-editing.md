# Editing mixins

The following mixins are used to construct Django's editing views:

- `django.views.generic.edit.FormMixin`
- `django.views.generic.edit.ModelFormMixin`
- `django.views.generic.edit.ProcessFormView`
- `django.views.generic.edit.DeletionMixin`

<div class="note">

<div class="title">

Note

</div>

Examples of how these are combined into editing views can be found at the documentation on `/ref/class-based-views/generic-editing`.

</div>

## `FormMixin`

<div class="django.views.generic.edit.FormMixin">

A mixin class that provides facilities for creating and displaying forms.

**Mixins**

- `django.views.generic.base.ContextMixin`

**Methods and Attributes**

<div class="attribute">

initial

A dictionary containing initial data for the form.

</div>

<div class="attribute">

form_class

The form class to instantiate.

</div>

<div class="attribute">

success_url

The URL to redirect to when the form is successfully processed.

</div>

<div class="attribute">

prefix

The `~django.forms.Form.prefix` for the generated form.

</div>

<div class="method">

get_initial()

Retrieve initial data for the form. By default, returns a copy of `~django.views.generic.edit.FormMixin.initial`.

</div>

<div class="method">

get_form_class()

Retrieve the form class to instantiate. By default `~django.views.generic.edit.FormMixin.form_class`.

</div>

<div class="method">

get_form(form_class=None)

Instantiate an instance of `form_class` using `~django.views.generic.edit.FormMixin.get_form_kwargs`. If `form_class` isn't provided `get_form_class` will be used.

</div>

<div class="method">

get_form_kwargs()

Build the keyword arguments required to instantiate the form.

The `initial` argument is set to `.get_initial`. If the request is a `POST` or `PUT`, the request data (`request.POST` and `request.FILES`) will also be provided.

</div>

<div class="method">

get_prefix()

Determine the `~django.forms.Form.prefix` for the generated form. Returns `~django.views.generic.edit.FormMixin.prefix` by default.

</div>

<div class="method">

get_success_url()

Determine the URL to redirect to when the form is successfully validated. Returns `~django.views.generic.edit.FormMixin.success_url` by default.

</div>

<div class="method">

form_valid(form)

Redirects to `~django.views.generic.edit.FormMixin.get_success_url`.

</div>

<div class="method">

form_invalid(form)

Renders a response, providing the invalid form as context.

</div>

<div class="method">

get_context_data(\*\*kwargs)

Calls `get_form` and adds the result to the context data with the name 'form'.

</div>

</div>

## `ModelFormMixin`

<div class="django.views.generic.edit.ModelFormMixin">

A form mixin that provides facilities for working with a `ModelForm`, rather than a standalone form.

Since this is a subclass of `~django.views.generic.detail.SingleObjectMixin`, instances of this mixin have access to the `~django.views.generic.detail.SingleObjectMixin.model` and `~django.views.generic.detail.SingleObjectMixin.queryset` attributes, describing the type of object that the `ModelForm` is manipulating.

If you specify both the `~django.views.generic.edit.ModelFormMixin.fields` and `~django.views.generic.edit.FormMixin.form_class` attributes, an `~django.core.exceptions.ImproperlyConfigured` exception will be raised.

**Mixins**

- `django.views.generic.edit.FormMixin`
- `django.views.generic.detail.SingleObjectMixin`

**Methods and Attributes**

<div class="attribute">

model

A model class. Can be explicitly provided, otherwise will be determined by examining `self.object` or `~django.views.generic.detail.SingleObjectMixin.queryset`.

</div>

<div class="attribute">

fields

A list of names of fields. This is interpreted the same way as the `Meta.fields` attribute of `~django.forms.ModelForm`.

This is a required attribute if you are generating the form class automatically (e.g. using `model`). Omitting this attribute will result in an `~django.core.exceptions.ImproperlyConfigured` exception.

</div>

<div class="attribute">

success_url

The URL to redirect to when the form is successfully processed.

`success_url` may contain dictionary string formatting, which will be interpolated against the object's field attributes. For example, you could use `success_url="/polls/{slug}/"` to redirect to a URL composed out of the `slug` field on a model.

</div>

<div class="method">

get_form_class()

Retrieve the form class to instantiate. If `~django.views.generic.edit.FormMixin.form_class` is provided, that class will be used. Otherwise, a `ModelForm` will be instantiated using the model associated with the `~django.views.generic.detail.SingleObjectMixin.queryset`, or with the `~django.views.generic.detail.SingleObjectMixin.model`, depending on which attribute is provided.

</div>

<div class="method">

get_form_kwargs()

Add the current instance (`self.object`) to the standard `~django.views.generic.edit.FormMixin.get_form_kwargs`.

</div>

<div class="method">

get_success_url()

Determine the URL to redirect to when the form is successfully validated. Returns `django.views.generic.edit.ModelFormMixin.success_url` if it is provided; otherwise, attempts to use the `get_absolute_url()` of the object.

</div>

<div class="method">

form_valid(form)

Saves the form instance, sets the current object for the view, and redirects to `~django.views.generic.edit.FormMixin.get_success_url`.

</div>

<div class="method">

form_invalid(form)

Renders a response, providing the invalid form as context.

</div>

</div>

## `ProcessFormView`

<div class="django.views.generic.edit.ProcessFormView">

A mixin that provides basic HTTP GET and POST workflow.

<div class="note">

<div class="title">

Note

</div>

This is named 'ProcessFormView' and inherits directly from `django.views.generic.base.View`, but breaks if used independently, so it is more of a mixin.

</div>

**Extends**

- `django.views.generic.base.View`

**Methods and Attributes**

<div class="method">

get(request, *args,*\*kwargs)

Renders a response using a context created with `~django.views.generic.edit.FormMixin.get_context_data`.

</div>

<div class="method">

post(request, *args,*\*kwargs)

Constructs a form, checks the form for validity, and handles it accordingly.

</div>

<div class="method">

put(*args,*\*kwargs)

The `PUT` action is also handled and passes all parameters through to `post`.

</div>

</div>

## `DeletionMixin`

<div class="django.views.generic.edit.DeletionMixin">

Enables handling of the `DELETE` HTTP action.

**Methods and Attributes**

<div class="attribute">

success_url

The url to redirect to when the nominated object has been successfully deleted.

`success_url` may contain dictionary string formatting, which will be interpolated against the object's field attributes. For example, you could use `success_url="/parent/{parent_id}/"` to redirect to a URL composed out of the `parent_id` field on a model.

</div>

<div class="method">

delete(request, *args,*\*kwargs)

Retrieves the target object and calls its `delete()` method, then redirects to the success URL.

</div>

<div class="method">

get_success_url()

Returns the url to redirect to when the nominated object has been successfully deleted. Returns `~django.views.generic.edit.DeletionMixin.success_url` by default.

</div>

</div>
