// spec: §records-sample-data — row must supply exactly as many values as columns
Table t {
  id int
  name string
  Records (id, name) {
    1, "a", "extra"
  }
}
