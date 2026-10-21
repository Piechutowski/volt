// spec: §default-values — an enum default names the column's own enum and one of its values
// want: is not a value of enum
Enum status { active }
Table t {
  s status [default: status.gone]
}
