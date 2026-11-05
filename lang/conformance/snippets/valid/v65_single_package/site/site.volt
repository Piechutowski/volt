// spec: §handler-references, §query-routes, §default-handlers, §dataset-declaration — one package declares its tables and routes them:
// a query of the package itself, and a resources or dataset of its own tables, is named
// bare (§imports)
package site

table posts {
  id    int [pk]
  title string
}

table tags {
  id   int [pk]
  name string    [unique]
}

select picked for posts where id in :ids [order: (id asc)]

table ms_revenue {
  id   int [pk]
  org  string
  year int
}

table ms_usage {
  id   int [pk]
  org  string
  year int
}

group series {
  ms_revenue
  ms_usage
}

select browse for series where year = :year [order: (id asc)]

scope /api [name: api] {
	resources posts     [default]
	resources tags      [default, except: (delete)]

	get    /picked          PostPicked
	get    /chosen          PostPicked [name: chosen]
	delete /tags/:id(int) Tags.Purge

	dataset browse [strip: "ms_"]
}
