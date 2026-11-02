// spec: §validators, §columns — a date/time has no empty value
// want: has no empty value
package db

table accounts {
  id int [pk]
  at time.Time [required]
}
