// spec: §default-values, a default is a number, string, boolean or expression literal
// want: default value must be a number, string, boolean or expression literal
Table t {
  id int [default: (a, b)]
}
