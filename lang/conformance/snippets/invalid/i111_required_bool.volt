// spec: §validation-checks, §columns — a boolean has no empty value; required means nothing on it
// want: has no empty value
package db

Table accounts {
  id     integer [pk]
  active boolean [not null, required]
}
