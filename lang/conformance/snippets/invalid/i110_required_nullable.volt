// spec: §validators, §columns — required needs not null: NULL is absence, not emptiness
// want: required on "email" needs not null
package db

Table accounts {
  id    int [pk]
  email string [required]
}
