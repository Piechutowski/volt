# `contrib` packages

Django aims to follow Python's `"batteries included" philosophy
<tut-batteries-included>`. It ships with a variety of extra, optional tools that solve common web development problems.

This code lives in `django/contrib` in the Django distribution. This document gives a rundown of the packages in `contrib`, along with any dependencies those packages have.

<div class="admonition">

Including `contrib` packages in `INSTALLED_APPS`

For most of these add-ons -- specifically, the add-ons that include either models or template tags -- you'll need to add the package name (e.g., `'django.contrib.redirects'`) to your `INSTALLED_APPS` setting and rerun `manage.py migrate`.

</div>

<div class="toctree" maxdepth="1">

admin/index auth contenttypes flatpages gis/index humanize messages postgres/index redirects sitemaps sites staticfiles syndication

</div>

## `admin`

The automatic Django administrative interface. For more information, see `Tutorial 2 </intro/tutorial02>` and the `admin documentation </ref/contrib/admin/index>`.

Requires the [auth](#auth) and [contenttypes](#contenttypes) contrib packages to be installed.

## `auth`

Django's authentication framework.

See `/topics/auth/index`.

## `contenttypes`

A light framework for hooking into "types" of content, where each installed Django model is a separate content type.

See the `contenttypes documentation </ref/contrib/contenttypes>`.

## `flatpages`

A framework for managing "flat" HTML content in a database.

See the `flatpages documentation </ref/contrib/flatpages>`.

Requires the [sites](#sites) contrib package to be installed as well.

## `gis`

A world-class geospatial framework built on top of Django, that enables storage, manipulation and display of spatial data.

See the `/ref/contrib/gis/index` documentation for more.

## `humanize`

A set of Django template filters useful for adding a "human touch" to data.

See the `humanize documentation </ref/contrib/humanize>`.

## `messages`

A framework for storing and retrieving temporary cookie- or session-based messages

See the `messages documentation </ref/contrib/messages>`.

## `postgres`

A collection of PostgreSQL specific features.

See the `contrib.postgres documentation </ref/contrib/postgres/index>`.

## `redirects`

A framework for managing redirects.

See the `redirects documentation </ref/contrib/redirects>`.

## `sessions`

A framework for storing data in anonymous sessions.

See the `sessions documentation </topics/http/sessions>`.

## `sites`

A light framework that lets you operate multiple websites off of the same database and Django installation. It gives you hooks for associating objects to one or more sites.

See the `sites documentation </ref/contrib/sites>`.

## `sitemaps`

A framework for generating Google sitemap XML files.

See the `sitemaps documentation </ref/contrib/sitemaps>`.

## `syndication`

A framework for generating syndication feeds, in RSS and Atom, quite easily.

See the `syndication documentation </ref/contrib/syndication>`.
