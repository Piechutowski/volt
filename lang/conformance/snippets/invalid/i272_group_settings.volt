// spec: §groups, a group takes no settings list
package db

table posts {
  id int [pk]
}

group content [model: "Content"] {
  posts
}
