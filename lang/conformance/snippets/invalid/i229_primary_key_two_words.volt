// spec: §columns, §settings-lists, the key setting is pk; primary key, two identifiers in a row, is a syntax error
// want: expected ',' or ']' in settings list, found identifier "key"
Table posts {
  id    int [primary key]
  title string
}
