// spec: §settings-lists, a Table's settings list stays on the line of the Table, even with the bracket in the first column
// want: a settings list stays on the line of the Table it modifies
Table users
  [note: "x"] {
  id int [pk]
}

Table posts
[note: "y"] {
  id int [pk]
}
