// spec: §records-sample-data — at most one records block per table
Table t {
  id int
  Records (id) { 1 }
  Records (id) { 2 }
}
