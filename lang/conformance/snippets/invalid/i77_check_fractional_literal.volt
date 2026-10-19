// spec: §validation-checks — a fractional literal needs a float column, or the Go
// tier would not compile
package db

Table page_views {
  id int [pk]
  hits int [not null]

  checks {
    hits >= 1.5
  }
}
