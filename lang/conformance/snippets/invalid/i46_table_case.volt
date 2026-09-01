// spec: §V5.1
package app

Table posts {
	id integer [pk]
}

// Names are case-sensitive: the table is "posts", not "Posts".
Scope / {
	resources Posts
}
