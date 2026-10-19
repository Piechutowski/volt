// spec: §validation-checks — a check takes no :params: it judges one row
package db

Table page_views {
  id int [pk]
  hits int [not null]

  checks {
    hits >= :min
  }
}
