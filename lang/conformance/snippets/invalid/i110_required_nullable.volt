// spec: §validation-checks, §columns — required needs not null: NULL is absence, not emptiness
// want: required on "email" needs not null
package db

Table accounts {
  id    integer [pk]
  email varchar [required]
}
