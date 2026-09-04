// spec: §V10.3 — one param name, one type: a list use and a scalar use of :site disagree
// want: one name, one type
package db

Table page_views {
  id integer [pk]
  site text [not null]
}

Select picked for page_views where site in :site or site = :site
