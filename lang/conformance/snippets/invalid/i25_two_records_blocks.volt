// spec: §records-sample-data — at most one records block per table
table t {
  id int
  records (id) { 1 }
  records (id) { 2 }
}
