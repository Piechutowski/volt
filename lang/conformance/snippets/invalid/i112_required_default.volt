// spec: §V12.8, §6.3 — required and default contradict: a defaulted column is the database's value
// want: "required" and "default" cannot both be set
package db

Table accounts {
  id   integer [pk]
  name varchar [not null, required, default: '']
}
