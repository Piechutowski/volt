// spec: §columns, the auto-assigning setting is spelled autoincrement, SQLite's own word; increment is no setting of the language
// want: unknown setting "increment"
table posts {
  id    int [pk, increment]
  title string
}
