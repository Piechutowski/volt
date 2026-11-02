// spec: §types — a declared type cannot spell a built-in
// want: spells a built-in type
type string string
table t {
  s string
}
