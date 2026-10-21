// spec: §handler-references, §query-routes, §default-handlers, §dataset-declaration — one package declares its tables and routes them:
// a query of the package itself is bare or self-qualified (as a plug is, §pipelines), and a
// resources or dataset of its own tables is unqualified
package site

Table posts {
  id    int [pk, increment]
  title string
}

Table tags {
  id   int [pk, increment]
  name string    [unique]
}

Select picked for posts where id in :ids [order: (id asc)]

Table ms_revenue {
  id   int [pk, increment]
  org  string
  year int
}

Table ms_usage {
  id   int [pk, increment]
  org  string
  year int
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
	delete /tags/:id(int) Tags.Purge

	dataset browse [strip: 'ms_']
}
