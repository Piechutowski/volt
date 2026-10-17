// spec: §validation-checks — check columns are columns of the enclosing table
package db

Table page_views {
  id integer [pk]

  checks {
    hits >= 0
  }
}
