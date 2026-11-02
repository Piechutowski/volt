// spec: §columns, SQLite's keyword is spelled autoincrement; increment is no setting of the language
// want: unknown setting "increment"
table posts {
  id    int [pk, increment]
  title string
}
