# Formset Functions

Formset API reference. For introductory material about formsets, see the `/topics/forms/formsets` topic guide.

<div class="module" synopsis="Django's functions for building formsets.">

django.forms.formsets

</div>

## `formset_factory`

<div class="function">

formset_factory(form, formset=BaseFormSet, extra=1, can_order=False, can_delete=False, max_num=None, validate_max=False, min_num=None, validate_min=False, absolute_max=None, can_delete_extra=True, renderer=None)

Returns a `FormSet` class for the given `form` class.

See `formsets </topics/forms/formsets>` for example usage.

</div>
