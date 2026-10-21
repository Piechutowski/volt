// spec: §default-values, a default is a number, string, boolean, expression or enum constant
// want: default value must be a number, string, boolean, expression or enum constant
Table t {
  id int [default: (a, b)]
}
