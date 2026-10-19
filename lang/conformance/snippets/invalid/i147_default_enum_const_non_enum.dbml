// spec: §default-values, an enum constant is a default only on an enum column
// want: is an enum constant but the column is not an enum
Enum status { active }
Table t {
  n int [default: status.active]
}
