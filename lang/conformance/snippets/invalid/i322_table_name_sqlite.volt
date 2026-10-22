// spec: §namespaces-and-uniqueness, a table name does not begin with sqlite_, in any case, which SQLite keeps for its own objects
// want: table name "sqlite_stats" begins with sqlite_
Table sqlite_stats {
  id int [pk]
}

Table SQLite_Daily [model: "Daily"] {
  id int [pk]
}
