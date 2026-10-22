// spec: §columns, §settings-lists, §line-structure-and-whitespace, the key setting is pk; primary key is a syntax error at key, where the setting ends, and the table whose only column it drops is not reported empty
// want: expected ',' or ']' in settings list, found identifier "key"
Table posts {
  id int [primary key]
}
