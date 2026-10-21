// spec: §records-sample-data, a literal null is allowed only on a nullable column
// want: is null, but the column is not nullable
Table t {
  id int [pk]
  n int
  records {
    1, null
  }
}
