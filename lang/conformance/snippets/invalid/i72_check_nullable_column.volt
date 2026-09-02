// spec: §V12.3 — a typed check references not-null columns only: the
// Go tier does not mirror three-valued NULL logic
package db

Table page_views {
  id integer [pk]
  hits integer

  checks {
    hits >= 0
  }
}
