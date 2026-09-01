// spec: §V12.2 — a check takes no :params: it judges one row
package db

Table page_views {
  id integer [pk]
  hits integer [not null]

  checks {
    hits >= :min
  }
}
