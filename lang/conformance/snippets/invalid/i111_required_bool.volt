// spec: §V12.8, §6.3 — a boolean has no empty value; required means nothing on it
// want: has no empty value
package db

Table accounts {
  id     integer [pk]
  active boolean [not null, required]
}
