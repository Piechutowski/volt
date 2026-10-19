// spec: §validation-checks — a check may start with the not keyword and a paren; it is
// a predicate, never a Go-reference call named "not"
package db

Table page_views {
  id int [pk]
  hits int [not null]
  site string [not null]

  checks {
    not (hits < 0 or site = '')
  }
}
