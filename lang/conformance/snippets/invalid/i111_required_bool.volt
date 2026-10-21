// spec: §validators, §columns — a boolean has no empty value; required means nothing on it
// want: has no empty value
package db

Table accounts {
  id     int [pk]
  active bool [required]
}
