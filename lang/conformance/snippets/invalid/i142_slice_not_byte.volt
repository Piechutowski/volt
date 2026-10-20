// spec: §columns, the [] belongs to []byte alone, a slice of anything else is an unknown type
// want: unknown type "[]string"
Table t {
  tags []string
}
