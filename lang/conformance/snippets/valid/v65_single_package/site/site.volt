// spec: §V4.3.1, §V4.8, §V5.5.3, §V13.1 — one package declares its tables and routes them:
// a query of the package itself is bare or self-qualified (as a plug is, §V3.2), and a
// resources or dataset of its own tables is unqualified
package site

Table posts {
  id    integer [pk, increment]
  title text    [not null]
}

Table tags {
  id   integer [pk, increment]
  name text    [not null, unique]
}

Select picked for posts where id in :ids [order: (id asc)]

Table ms_revenue {
  id   integer [pk, increment]
  org  text    [not null]
  year integer [not null]
}

Table ms_usage {
  id   integer [pk, increment]
  org  text    [not null]
  year integer [not null]
}

Group series {
  ms_revenue
  ms_usage
}

Select browse for series where year = :year [order: (id asc)]

Scope /api [name: api] {
	resources posts     [default]
	resources site.tags [default, except: (delete)]

	get    /picked          PostPicked
	get    /chosen          site.PostPicked [name: chosen]
	delete /tags/:id(int32) Tags.Purge

	dataset browse [strip: 'ms_']
}
