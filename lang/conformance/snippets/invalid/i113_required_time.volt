// spec: §validation-checks rule 8, §columns — a date/time has no empty value
// want: has no empty value
package db

Table accounts {
  id integer [pk]
  at timestamp [not null, required]
}
