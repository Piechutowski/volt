// spec: §columns, required is an error on the rowid alias, with or without autoincrement, since the database assigns it whenever an insert leaves it unmarked
// want: the rowid alias of table "posts"
table posts {
  id    int [pk, required]
  title string
}

table events {
  id   int [pk, autoincrement, required]
  name string
}
