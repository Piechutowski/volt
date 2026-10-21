// spec: §records-sample-data, a dotted name is not a record value; a declared type takes a literal of its storage type
// want: a dotted name is not a record value
Type State string
Table orders {
  id     int   [pk]
  status State

  Records (id, status) {
    1, state.paid
    2, 'paid'
  }
}
