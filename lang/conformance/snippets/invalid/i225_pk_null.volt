// spec: §columns, a key column never carries null: pk and null together are an error
// want: carries pk and null
Table posts {
  id    int    [pk, null]
  title string
}
