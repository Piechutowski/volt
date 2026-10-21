// spec: §default-values, null is not a default value, on a nullable column or any other
// want: null is not a default value
Table t {
  n int [null, default: null]
  m int [default: null]
}
