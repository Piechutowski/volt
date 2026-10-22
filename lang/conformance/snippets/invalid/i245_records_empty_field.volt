// spec: §records-sample-data, every field of a row holds a value; NULL is written null and an empty field is an error
// want: empty record field
Table t {
  id int [pk]
  a string [null]
  b string [null]
  Records (id, a, b) {
    1, , "x"
    , "y", "z"
    3, "w",
    4, null, null
  }
}
