// spec: §validation-checks, §columns — required and default contradict: a defaulted column is the database's value
// want: "required" and "default" cannot both be set
package db

Table accounts {
  id   int [pk]
  name string [not null, required, default: '']
}
