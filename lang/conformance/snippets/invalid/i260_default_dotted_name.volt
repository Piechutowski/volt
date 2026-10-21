// spec: §default-values, a dotted name is not a default; a declared type takes a literal of its storage type
// want: found the dotted name state.pending
Type State string
Table orders {
  id     int   [pk]
  status State [default: state.pending]
}
