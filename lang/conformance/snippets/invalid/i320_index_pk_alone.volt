// spec: §indexes, §columns, pk on an index is one error naming the column form: the entry is neither a plain single-column index nor an index whose name another shares
// want: pk is a column setting, not an index setting: write pk on each key column
table t {
  id int [pk]
  a  int
  b  int

  indexes {
    a [pk]
    (a, b) [pk]
    (a, b)
  }
}
