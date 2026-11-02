// spec: §records-sample-data — bare identifiers other than true/false/null are not record values
type S string
table t {
  x S
  records (x) { a }
}
