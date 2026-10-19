// spec: §validation-checks — a typed check references not-null columns only: the
// Go tier does not mirror three-valued NULL logic
package db

Table page_views {
  id int [pk]
  hits int

  checks {
    hits >= 0
  }
}
