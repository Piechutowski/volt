// spec: §records-sample-data — a record value must fit the column's type
// want: must be an integer, found string
table t {
  id int [pk]
  n int
  records {
    1, "seven"
  }
}
