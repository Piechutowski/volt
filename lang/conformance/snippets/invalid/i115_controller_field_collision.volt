// spec: §handler-references — a controller cannot take the name of a Controllers field; DB holds the database the query routes run on
// want: takes the name of the Controllers field
package app

Table posts {
  id    int [pk, increment]
  title string
}

Scope /api {
	get /posts       PostList
	get /queries/:id DB.Show
}
