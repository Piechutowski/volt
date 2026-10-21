// spec: §declaration
package app

Table posts {
	id int [pk]
}

// Names are case-sensitive: the table is "posts", not "Posts".
Scope / {
	resources Posts
}
