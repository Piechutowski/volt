// spec: §declaration
package app

table posts {
	id int [pk]
}

// Names are case-sensitive: the table is "posts", not "Posts".
scope / {
	resources Posts
}
