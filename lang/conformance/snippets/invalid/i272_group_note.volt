// spec: §notes, §groups, a group carries no note
package db

table posts {
  id int [pk]
}

group content [note: "what readers see"] {
  posts
}
