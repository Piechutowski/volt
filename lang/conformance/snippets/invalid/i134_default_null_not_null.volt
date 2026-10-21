// spec: §default-values — null is a default only on a nullable column
// want: is null, but the column is not nullable
Table t {
  n int [default: null]
}
