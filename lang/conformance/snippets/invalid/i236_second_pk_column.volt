// spec: §columns, a table has one primary key: a second pk column is an error
// want: declares a second primary key
Table posts {
  id   int    [pk]
  slug string [pk]
}
