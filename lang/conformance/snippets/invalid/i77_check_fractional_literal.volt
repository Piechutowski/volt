// spec: §V12.4 — a fractional literal needs a float column, or the Go
// tier would not compile
package db

Table page_views {
  id integer [pk]
  hits integer [not null]

  checks {
    hits >= 1.5
  }
}
