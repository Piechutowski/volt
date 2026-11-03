// spec: §columns, required is an error on the database-assigned key, with or without autoincrement, since the database assigns it whenever an insert leaves it unmarked
// want: the database-assigned key of table "posts"
table posts {
  id    int [pk, required]
  title string
}

table events {
  id   int [pk, autoincrement, required]
  name string
}
