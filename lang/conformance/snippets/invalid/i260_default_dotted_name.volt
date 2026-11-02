// spec: §default-values, a dotted name is not a default; a declared type takes a literal of its storage type
// want: found the dotted name state.pending
type State string
table orders {
  id     int   [pk]
  status State [default: state.pending]
}
