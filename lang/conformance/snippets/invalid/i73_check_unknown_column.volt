// spec: §validation-checks — check columns are columns of the enclosing table
package db

Table page_views {
  id int [pk]

  checks {
    hits >= 0
  }
}
