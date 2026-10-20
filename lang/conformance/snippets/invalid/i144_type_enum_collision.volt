// spec: §types, a declared type cannot spell an enum name
// want: collides with the enum of that name
Enum status { active }
Type status string
Table t {
  s status
}
