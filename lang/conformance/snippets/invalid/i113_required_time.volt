// spec: §validation-checks, §columns — a date/time has no empty value
// want: has no empty value
package db

Table accounts {
  id int [pk]
  at time.Time [not null, required]
}
