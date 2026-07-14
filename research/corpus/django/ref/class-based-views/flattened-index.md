# Class-based generic views - flattened index

This index provides an alternate organization of the reference documentation for class-based views. For each view, the effective attributes and methods from the class tree are represented under that view. For the reference documentation organized by the class which defines the behavior, see `Class-based views</ref/class-based-views/index>`.

<div class="seealso">

[Classy Class-Based Views](https://ccbv.co.uk/) provides a nice interface to navigate the class hierarchy of the built-in class-based views.

</div>

## Simple generic views

### `View`

<div class="View()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.View.http_method_names`

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.base.View.setup`

### `TemplateView`

<div class="TemplateView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.base.ContextMixin.get_context_data`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `RedirectView`

<div class="RedirectView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.base.RedirectView.pattern_name`
- `~django.views.generic.base.RedirectView.permanent`
- `~django.views.generic.base.RedirectView.query_string`
- `~django.views.generic.base.RedirectView.url` \[`~django.views.generic.base.RedirectView.get_redirect_url`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `delete()`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `options()`
- `post()`
- `put()`
- `~django.views.generic.base.View.setup`

## Detail Views

### `DetailView`

<div class="DetailView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.detail.SingleObjectMixin.context_object_name` \[`~django.views.generic.detail.SingleObjectMixin.get_context_object_name`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.detail.SingleObjectMixin.model`
- `~django.views.generic.detail.SingleObjectMixin.pk_url_kwarg`
- `~django.views.generic.detail.SingleObjectMixin.query_pk_and_slug`
- `~django.views.generic.detail.SingleObjectMixin.queryset` \[`~django.views.generic.detail.SingleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_field` \[`~django.views.generic.detail.SingleObjectMixin.get_slug_field`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_url_kwarg`
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_field`
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_suffix`

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `~django.views.generic.detail.BaseDetailView.get`
- `~django.views.generic.detail.SingleObjectMixin.get_context_data`
- `~django.views.generic.detail.SingleObjectMixin.get_object`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

## List Views

### `ListView`

<div class="ListView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.list.MultipleObjectMixin.allow_empty` \[`~django.views.generic.list.MultipleObjectMixin.get_allow_empty`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.list.MultipleObjectMixin.context_object_name` \[`~django.views.generic.list.MultipleObjectMixin.get_context_object_name`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.list.MultipleObjectMixin.model`
- `~django.views.generic.list.MultipleObjectMixin.ordering` \[`~django.views.generic.list.MultipleObjectMixin.get_ordering`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_by` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_by`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_orphans` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_orphans`\]
- `~django.views.generic.list.MultipleObjectMixin.paginator_class`
- `~django.views.generic.list.MultipleObjectMixin.queryset` \[`~django.views.generic.list.MultipleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.list.MultipleObjectTemplateResponseMixin.template_name_suffix`

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `~django.views.generic.list.BaseListView.get`
- `~django.views.generic.list.MultipleObjectMixin.get_context_data`
- `~django.views.generic.list.MultipleObjectMixin.get_paginator`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.list.MultipleObjectMixin.paginate_queryset`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

## Editing views

### `FormView`

<div class="FormView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.edit.FormMixin.form_class` \[`~django.views.generic.edit.FormMixin.get_form_class`\]
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.edit.FormMixin.initial` \[`~django.views.generic.edit.FormMixin.get_initial`\]
- `~django.views.generic.edit.FormMixin.prefix` \[`~django.views.generic.edit.FormMixin.get_prefix`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.edit.FormMixin.success_url` \[`~django.views.generic.edit.FormMixin.get_success_url`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `~django.views.generic.edit.FormMixin.form_invalid`
- `~django.views.generic.edit.FormMixin.form_valid`
- `~django.views.generic.edit.ProcessFormView.get`
- `~django.views.generic.edit.FormMixin.get_context_data`
- `~django.views.generic.edit.FormMixin.get_form`
- `~django.views.generic.edit.FormMixin.get_form_kwargs`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.edit.ProcessFormView.post`
- `~django.views.generic.edit.ProcessFormView.put`
- `~django.views.generic.base.View.setup`

### `CreateView`

<div class="CreateView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.detail.SingleObjectMixin.context_object_name` \[`~django.views.generic.detail.SingleObjectMixin.get_context_object_name`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.edit.ModelFormMixin.fields`
- `~django.views.generic.edit.FormMixin.form_class` \[`~django.views.generic.edit.ModelFormMixin.get_form_class`\]
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.edit.FormMixin.initial` \[`~django.views.generic.edit.FormMixin.get_initial`\]
- `~django.views.generic.detail.SingleObjectMixin.model`
- `~django.views.generic.detail.SingleObjectMixin.pk_url_kwarg`
- `~django.views.generic.edit.FormMixin.prefix` \[`~django.views.generic.edit.FormMixin.get_prefix`\]
- `~django.views.generic.detail.SingleObjectMixin.query_pk_and_slug`
- `~django.views.generic.detail.SingleObjectMixin.queryset` \[`~django.views.generic.detail.SingleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_field` \[`~django.views.generic.detail.SingleObjectMixin.get_slug_field`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_url_kwarg`
- `~django.views.generic.edit.FormMixin.success_url` \[`~django.views.generic.edit.ModelFormMixin.get_success_url`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_field`
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_suffix`

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `~django.views.generic.edit.FormMixin.form_invalid`
- `~django.views.generic.edit.ModelFormMixin.form_valid`
- `~django.views.generic.edit.ProcessFormView.get`
- `~django.views.generic.edit.FormMixin.get_context_data`
- `~django.views.generic.edit.FormMixin.get_form`
- `~django.views.generic.edit.ModelFormMixin.get_form_kwargs`
- `~django.views.generic.detail.SingleObjectMixin.get_object`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.edit.ProcessFormView.post`
- `put()`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `UpdateView`

<div class="UpdateView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.detail.SingleObjectMixin.context_object_name` \[`~django.views.generic.detail.SingleObjectMixin.get_context_object_name`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.edit.ModelFormMixin.fields`
- `~django.views.generic.edit.FormMixin.form_class` \[`~django.views.generic.edit.ModelFormMixin.get_form_class`\]
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.edit.FormMixin.initial` \[`~django.views.generic.edit.FormMixin.get_initial`\]
- `~django.views.generic.detail.SingleObjectMixin.model`
- `~django.views.generic.detail.SingleObjectMixin.pk_url_kwarg`
- `~django.views.generic.edit.FormMixin.prefix` \[`~django.views.generic.edit.FormMixin.get_prefix`\]
- `~django.views.generic.detail.SingleObjectMixin.query_pk_and_slug`
- `~django.views.generic.detail.SingleObjectMixin.queryset` \[`~django.views.generic.detail.SingleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_field` \[`~django.views.generic.detail.SingleObjectMixin.get_slug_field`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_url_kwarg`
- `~django.views.generic.edit.FormMixin.success_url` \[`~django.views.generic.edit.ModelFormMixin.get_success_url`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_field`
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_suffix`

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `~django.views.generic.edit.FormMixin.form_invalid`
- `~django.views.generic.edit.ModelFormMixin.form_valid`
- `~django.views.generic.edit.ProcessFormView.get`
- `~django.views.generic.edit.FormMixin.get_context_data`
- `~django.views.generic.edit.FormMixin.get_form`
- `~django.views.generic.edit.ModelFormMixin.get_form_kwargs`
- `~django.views.generic.detail.SingleObjectMixin.get_object`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.edit.ProcessFormView.post`
- `put()`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `DeleteView`

<div class="DeleteView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.detail.SingleObjectMixin.context_object_name` \[`~django.views.generic.detail.SingleObjectMixin.get_context_object_name`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.detail.SingleObjectMixin.model`
- `~django.views.generic.detail.SingleObjectMixin.pk_url_kwarg`
- `~django.views.generic.detail.SingleObjectMixin.query_pk_and_slug`
- `~django.views.generic.detail.SingleObjectMixin.queryset` \[`~django.views.generic.detail.SingleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_field` \[`~django.views.generic.detail.SingleObjectMixin.get_slug_field`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_url_kwarg`
- `~django.views.generic.edit.DeletionMixin.success_url` \[`~django.views.generic.edit.DeletionMixin.get_success_url`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_field`
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_suffix`

**Methods**

- `~django.views.generic.base.View.as_view`
- `delete()`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.detail.SingleObjectMixin.get_context_data`
- `~django.views.generic.detail.SingleObjectMixin.get_object`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `post()`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

## Date-based views

### `ArchiveIndexView`

<div class="ArchiveIndexView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.list.MultipleObjectMixin.allow_empty` \[`~django.views.generic.list.MultipleObjectMixin.get_allow_empty`\]
- `~django.views.generic.dates.DateMixin.allow_future` \[`~django.views.generic.dates.DateMixin.get_allow_future`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.list.MultipleObjectMixin.context_object_name` \[`~django.views.generic.list.MultipleObjectMixin.get_context_object_name`\]
- `~django.views.generic.dates.DateMixin.date_field` \[`~django.views.generic.dates.DateMixin.get_date_field`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.list.MultipleObjectMixin.model`
- `~django.views.generic.list.MultipleObjectMixin.ordering` \[`~django.views.generic.list.MultipleObjectMixin.get_ordering`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_by` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_by`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_orphans` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_orphans`\]
- `~django.views.generic.list.MultipleObjectMixin.paginator_class`
- `~django.views.generic.list.MultipleObjectMixin.queryset` \[`~django.views.generic.list.MultipleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.list.MultipleObjectTemplateResponseMixin.template_name_suffix`

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.list.MultipleObjectMixin.get_context_data`
- `~django.views.generic.dates.BaseDateListView.get_date_list`
- `~django.views.generic.dates.BaseDateListView.get_dated_items`
- `~django.views.generic.dates.BaseDateListView.get_dated_queryset`
- `~django.views.generic.list.MultipleObjectMixin.get_paginator`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.list.MultipleObjectMixin.paginate_queryset`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `YearArchiveView`

<div class="YearArchiveView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.list.MultipleObjectMixin.allow_empty` \[`~django.views.generic.list.MultipleObjectMixin.get_allow_empty`\]
- `~django.views.generic.dates.DateMixin.allow_future` \[`~django.views.generic.dates.DateMixin.get_allow_future`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.list.MultipleObjectMixin.context_object_name` \[`~django.views.generic.list.MultipleObjectMixin.get_context_object_name`\]
- `~django.views.generic.dates.DateMixin.date_field` \[`~django.views.generic.dates.DateMixin.get_date_field`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.dates.YearArchiveView.make_object_list` \[`~django.views.generic.dates.YearArchiveView.get_make_object_list`\]
- `~django.views.generic.list.MultipleObjectMixin.model`
- `~django.views.generic.list.MultipleObjectMixin.ordering` \[`~django.views.generic.list.MultipleObjectMixin.get_ordering`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_by` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_by`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_orphans` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_orphans`\]
- `~django.views.generic.list.MultipleObjectMixin.paginator_class`
- `~django.views.generic.list.MultipleObjectMixin.queryset` \[`~django.views.generic.list.MultipleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.list.MultipleObjectTemplateResponseMixin.template_name_suffix`
- `~django.views.generic.dates.YearMixin.year` \[`~django.views.generic.dates.YearMixin.get_year`\]
- `~django.views.generic.dates.YearMixin.year_format` \[`~django.views.generic.dates.YearMixin.get_year_format`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.list.MultipleObjectMixin.get_context_data`
- `~django.views.generic.dates.BaseDateListView.get_date_list`
- `~django.views.generic.dates.BaseDateListView.get_dated_items`
- `~django.views.generic.dates.BaseDateListView.get_dated_queryset`
- `~django.views.generic.list.MultipleObjectMixin.get_paginator`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.list.MultipleObjectMixin.paginate_queryset`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `MonthArchiveView`

<div class="MonthArchiveView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.list.MultipleObjectMixin.allow_empty` \[`~django.views.generic.list.MultipleObjectMixin.get_allow_empty`\]
- `~django.views.generic.dates.DateMixin.allow_future` \[`~django.views.generic.dates.DateMixin.get_allow_future`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.list.MultipleObjectMixin.context_object_name` \[`~django.views.generic.list.MultipleObjectMixin.get_context_object_name`\]
- `~django.views.generic.dates.DateMixin.date_field` \[`~django.views.generic.dates.DateMixin.get_date_field`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.list.MultipleObjectMixin.model`
- `~django.views.generic.dates.MonthMixin.month` \[`~django.views.generic.dates.MonthMixin.get_month`\]
- `~django.views.generic.dates.MonthMixin.month_format` \[`~django.views.generic.dates.MonthMixin.get_month_format`\]
- `~django.views.generic.list.MultipleObjectMixin.ordering` \[`~django.views.generic.list.MultipleObjectMixin.get_ordering`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_by` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_by`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_orphans` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_orphans`\]
- `~django.views.generic.list.MultipleObjectMixin.paginator_class`
- `~django.views.generic.list.MultipleObjectMixin.queryset` \[`~django.views.generic.list.MultipleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.list.MultipleObjectTemplateResponseMixin.template_name_suffix`
- `~django.views.generic.dates.YearMixin.year` \[`~django.views.generic.dates.YearMixin.get_year`\]
- `~django.views.generic.dates.YearMixin.year_format` \[`~django.views.generic.dates.YearMixin.get_year_format`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.list.MultipleObjectMixin.get_context_data`
- `~django.views.generic.dates.BaseDateListView.get_date_list`
- `~django.views.generic.dates.BaseDateListView.get_dated_items`
- `~django.views.generic.dates.BaseDateListView.get_dated_queryset`
- `~django.views.generic.dates.MonthMixin.get_next_month`
- `~django.views.generic.list.MultipleObjectMixin.get_paginator`
- `~django.views.generic.dates.MonthMixin.get_previous_month`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.list.MultipleObjectMixin.paginate_queryset`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `WeekArchiveView`

<div class="WeekArchiveView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.list.MultipleObjectMixin.allow_empty` \[`~django.views.generic.list.MultipleObjectMixin.get_allow_empty`\]
- `~django.views.generic.dates.DateMixin.allow_future` \[`~django.views.generic.dates.DateMixin.get_allow_future`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.list.MultipleObjectMixin.context_object_name` \[`~django.views.generic.list.MultipleObjectMixin.get_context_object_name`\]
- `~django.views.generic.dates.DateMixin.date_field` \[`~django.views.generic.dates.DateMixin.get_date_field`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.list.MultipleObjectMixin.model`
- `~django.views.generic.list.MultipleObjectMixin.ordering` \[`~django.views.generic.list.MultipleObjectMixin.get_ordering`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_by` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_by`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_orphans` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_orphans`\]
- `~django.views.generic.list.MultipleObjectMixin.paginator_class`
- `~django.views.generic.list.MultipleObjectMixin.queryset` \[`~django.views.generic.list.MultipleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.list.MultipleObjectTemplateResponseMixin.template_name_suffix`
- `~django.views.generic.dates.WeekMixin.week` \[`~django.views.generic.dates.WeekMixin.get_week`\]
- `~django.views.generic.dates.WeekMixin.week_format` \[`~django.views.generic.dates.WeekMixin.get_week_format`\]
- `~django.views.generic.dates.YearMixin.year` \[`~django.views.generic.dates.YearMixin.get_year`\]
- `~django.views.generic.dates.YearMixin.year_format` \[`~django.views.generic.dates.YearMixin.get_year_format`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.list.MultipleObjectMixin.get_context_data`
- `~django.views.generic.dates.BaseDateListView.get_date_list`
- `~django.views.generic.dates.BaseDateListView.get_dated_items`
- `~django.views.generic.dates.BaseDateListView.get_dated_queryset`
- `~django.views.generic.list.MultipleObjectMixin.get_paginator`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.list.MultipleObjectMixin.paginate_queryset`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `DayArchiveView`

<div class="DayArchiveView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.list.MultipleObjectMixin.allow_empty` \[`~django.views.generic.list.MultipleObjectMixin.get_allow_empty`\]
- `~django.views.generic.dates.DateMixin.allow_future` \[`~django.views.generic.dates.DateMixin.get_allow_future`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.list.MultipleObjectMixin.context_object_name` \[`~django.views.generic.list.MultipleObjectMixin.get_context_object_name`\]
- `~django.views.generic.dates.DateMixin.date_field` \[`~django.views.generic.dates.DateMixin.get_date_field`\]
- `~django.views.generic.dates.DayMixin.day` \[`~django.views.generic.dates.DayMixin.get_day`\]
- `~django.views.generic.dates.DayMixin.day_format` \[`~django.views.generic.dates.DayMixin.get_day_format`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.list.MultipleObjectMixin.model`
- `~django.views.generic.dates.MonthMixin.month` \[`~django.views.generic.dates.MonthMixin.get_month`\]
- `~django.views.generic.dates.MonthMixin.month_format` \[`~django.views.generic.dates.MonthMixin.get_month_format`\]
- `~django.views.generic.list.MultipleObjectMixin.ordering` \[`~django.views.generic.list.MultipleObjectMixin.get_ordering`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_by` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_by`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_orphans` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_orphans`\]
- `~django.views.generic.list.MultipleObjectMixin.paginator_class`
- `~django.views.generic.list.MultipleObjectMixin.queryset` \[`~django.views.generic.list.MultipleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.list.MultipleObjectTemplateResponseMixin.template_name_suffix`
- `~django.views.generic.dates.YearMixin.year` \[`~django.views.generic.dates.YearMixin.get_year`\]
- `~django.views.generic.dates.YearMixin.year_format` \[`~django.views.generic.dates.YearMixin.get_year_format`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.list.MultipleObjectMixin.get_context_data`
- `~django.views.generic.dates.BaseDateListView.get_date_list`
- `~django.views.generic.dates.BaseDateListView.get_dated_items`
- `~django.views.generic.dates.BaseDateListView.get_dated_queryset`
- `~django.views.generic.dates.DayMixin.get_next_day`
- `~django.views.generic.dates.MonthMixin.get_next_month`
- `~django.views.generic.list.MultipleObjectMixin.get_paginator`
- `~django.views.generic.dates.DayMixin.get_previous_day`
- `~django.views.generic.dates.MonthMixin.get_previous_month`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.list.MultipleObjectMixin.paginate_queryset`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `TodayArchiveView`

<div class="TodayArchiveView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.list.MultipleObjectMixin.allow_empty` \[`~django.views.generic.list.MultipleObjectMixin.get_allow_empty`\]
- `~django.views.generic.dates.DateMixin.allow_future` \[`~django.views.generic.dates.DateMixin.get_allow_future`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.list.MultipleObjectMixin.context_object_name` \[`~django.views.generic.list.MultipleObjectMixin.get_context_object_name`\]
- `~django.views.generic.dates.DateMixin.date_field` \[`~django.views.generic.dates.DateMixin.get_date_field`\]
- `~django.views.generic.dates.DayMixin.day` \[`~django.views.generic.dates.DayMixin.get_day`\]
- `~django.views.generic.dates.DayMixin.day_format` \[`~django.views.generic.dates.DayMixin.get_day_format`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.list.MultipleObjectMixin.model`
- `~django.views.generic.dates.MonthMixin.month` \[`~django.views.generic.dates.MonthMixin.get_month`\]
- `~django.views.generic.dates.MonthMixin.month_format` \[`~django.views.generic.dates.MonthMixin.get_month_format`\]
- `~django.views.generic.list.MultipleObjectMixin.ordering` \[`~django.views.generic.list.MultipleObjectMixin.get_ordering`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_by` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_by`\]
- `~django.views.generic.list.MultipleObjectMixin.paginate_orphans` \[`~django.views.generic.list.MultipleObjectMixin.get_paginate_orphans`\]
- `~django.views.generic.list.MultipleObjectMixin.paginator_class`
- `~django.views.generic.list.MultipleObjectMixin.queryset` \[`~django.views.generic.list.MultipleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.list.MultipleObjectTemplateResponseMixin.template_name_suffix`
- `~django.views.generic.dates.YearMixin.year` \[`~django.views.generic.dates.YearMixin.get_year`\]
- `~django.views.generic.dates.YearMixin.year_format` \[`~django.views.generic.dates.YearMixin.get_year_format`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.list.MultipleObjectMixin.get_context_data`
- `~django.views.generic.dates.BaseDateListView.get_date_list`
- `~django.views.generic.dates.BaseDateListView.get_dated_items`
- `~django.views.generic.dates.BaseDateListView.get_dated_queryset`
- `~django.views.generic.dates.DayMixin.get_next_day`
- `~django.views.generic.dates.MonthMixin.get_next_month`
- `~django.views.generic.list.MultipleObjectMixin.get_paginator`
- `~django.views.generic.dates.DayMixin.get_previous_day`
- `~django.views.generic.dates.MonthMixin.get_previous_month`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.list.MultipleObjectMixin.paginate_queryset`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`

### `DateDetailView`

<div class="DateDetailView()">

**Attributes** (with optional accessor):

</div>

- `~django.views.generic.dates.DateMixin.allow_future` \[`~django.views.generic.dates.DateMixin.get_allow_future`\]
- `~django.views.generic.base.TemplateResponseMixin.content_type`
- `~django.views.generic.detail.SingleObjectMixin.context_object_name` \[`~django.views.generic.detail.SingleObjectMixin.get_context_object_name`\]
- `~django.views.generic.dates.DateMixin.date_field` \[`~django.views.generic.dates.DateMixin.get_date_field`\]
- `~django.views.generic.dates.DayMixin.day` \[`~django.views.generic.dates.DayMixin.get_day`\]
- `~django.views.generic.dates.DayMixin.day_format` \[`~django.views.generic.dates.DayMixin.get_day_format`\]
- `~django.views.generic.base.ContextMixin.extra_context`
- `~django.views.generic.base.View.http_method_names`
- `~django.views.generic.detail.SingleObjectMixin.model`
- `~django.views.generic.dates.MonthMixin.month` \[`~django.views.generic.dates.MonthMixin.get_month`\]
- `~django.views.generic.dates.MonthMixin.month_format` \[`~django.views.generic.dates.MonthMixin.get_month_format`\]
- `~django.views.generic.detail.SingleObjectMixin.pk_url_kwarg`
- `~django.views.generic.detail.SingleObjectMixin.query_pk_and_slug`
- `~django.views.generic.detail.SingleObjectMixin.queryset` \[`~django.views.generic.detail.SingleObjectMixin.get_queryset`\]
- `~django.views.generic.base.TemplateResponseMixin.response_class` \[`~django.views.generic.base.TemplateResponseMixin.render_to_response`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_field` \[`~django.views.generic.detail.SingleObjectMixin.get_slug_field`\]
- `~django.views.generic.detail.SingleObjectMixin.slug_url_kwarg`
- `~django.views.generic.base.TemplateResponseMixin.template_engine`
- `~django.views.generic.base.TemplateResponseMixin.template_name` \[`~django.views.generic.base.TemplateResponseMixin.get_template_names`\]
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_field`
- `~django.views.generic.detail.SingleObjectTemplateResponseMixin.template_name_suffix`
- `~django.views.generic.dates.YearMixin.year` \[`~django.views.generic.dates.YearMixin.get_year`\]
- `~django.views.generic.dates.YearMixin.year_format` \[`~django.views.generic.dates.YearMixin.get_year_format`\]

**Methods**

- `~django.views.generic.base.View.as_view`
- `~django.views.generic.base.View.dispatch`
- `get()`
- `~django.views.generic.detail.SingleObjectMixin.get_context_data`
- `~django.views.generic.dates.DayMixin.get_next_day`
- `~django.views.generic.dates.MonthMixin.get_next_month`
- `~django.views.generic.detail.SingleObjectMixin.get_object`
- `~django.views.generic.dates.DayMixin.get_previous_day`
- `~django.views.generic.dates.MonthMixin.get_previous_month`
- `head()`
- `~django.views.generic.base.View.http_method_not_allowed`
- `~django.views.generic.base.TemplateResponseMixin.render_to_response`
- `~django.views.generic.base.View.setup`
