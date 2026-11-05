// spec: §settings-lists, a Table's settings list stays on the line of the Table, even with the bracket in the first column
// want: a settings list stays on the line of the table it modifies
table users
  [model: "x"] {
  id int [pk]
}

table posts
[model: "y"] {
  id int [pk]
}
