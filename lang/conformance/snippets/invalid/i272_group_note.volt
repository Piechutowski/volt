// spec: §notes, §groups, a group carries no note
package db

Table posts {
  id int [pk]
}

Group content [note: "what readers see"] {
  posts
}
