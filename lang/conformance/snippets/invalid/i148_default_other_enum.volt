// spec: §default-values, an enum default names the column's own enum, not another
// want: the column's enum is
Enum status { active }
Enum role { admin }
Table t {
  s status [default: role.admin]
}
