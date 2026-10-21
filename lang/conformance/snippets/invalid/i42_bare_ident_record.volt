// spec: §records-sample-data — bare identifiers other than true/false/null are not record values
Type S string
Table t {
  x S
  records (x) { a }
}
