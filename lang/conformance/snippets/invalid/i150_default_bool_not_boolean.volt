// spec: §default-values, a bool default is true or false
// want: must be a boolean, found integer
Table t {
  live bool [default: 1]
}
