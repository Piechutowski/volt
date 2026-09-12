// spec: §V4.3.4 — a controller cannot take the name of a Controllers field; Queries holds this package's query routes
// want: takes the name of the Controllers field
package app

Table posts {
  id    integer [pk, increment]
  title text    [not null]
}

Scope /api {
	get /posts       PostList
	get /queries/:id Queries.Show
}
