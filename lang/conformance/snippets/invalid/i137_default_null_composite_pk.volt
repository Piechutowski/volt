// spec: §default-values — a column of a composite primary key is not nullable, so null is not its default
// want: is null, but the column is not nullable
Table t {
  a int [default: null]
  b int
  indexes {
    (a, b) [pk]
  }
}
