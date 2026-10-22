// spec: §indexes, a written name does not begin with sqlite_, in any case, since SQLite keeps those names for its own objects
// want: a name beginning with sqlite_ is SQLite's own and refused for an index
Table stats {
  id int [pk]
  a  int
  b  int

  Indexes {
    (a, b) [name: "sqlite_ab"]
    (b, a) [name: "SQLite_BA"]
  }
}
