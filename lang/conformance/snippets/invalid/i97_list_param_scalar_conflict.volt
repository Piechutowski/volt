// spec: §predicates — one param name, one type: a list use and a scalar use of :site disagree
// want: one name, one type
package db

table page_views {
  id int [pk]
  site string
}

select picked for page_views where site in :site or site = :site
