// spec: §default-values, null is not a default value, on a nullable column or any other
// want: null is not a default value
table t {
  n int [null, default: null]
  m int [default: null]
}
