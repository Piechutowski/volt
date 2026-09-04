// spec: §V12.8, §6.3 — required needs not null: NULL is absence, not emptiness
// want: required on "email" needs not null
package db

Table accounts {
  id    integer [pk]
  email varchar [required]
}
