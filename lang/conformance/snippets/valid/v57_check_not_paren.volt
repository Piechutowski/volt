// spec: §V12 — a check may start with the not keyword and a paren; it is
// a predicate, never a Go-reference call named "not"
package db

Table page_views {
  id integer [pk]
  hits integer [not null]
  site varchar [not null]

  checks {
    not (hits < 0 or site = '')
  }
}
