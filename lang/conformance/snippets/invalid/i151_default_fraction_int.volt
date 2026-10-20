// spec: §default-values, an int default is an integer literal, no fraction
// want: must be an integer, found number
Table t {
  n int [default: 1.5]
}
