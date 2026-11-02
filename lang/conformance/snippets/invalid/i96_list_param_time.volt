// spec: §predicates — a date/time column cannot take a list parameter: JSON carries no date/time value
// want: cannot take a list parameter
package db

table page_views {
  id int [pk]
  at time.Time
}

select picked for page_views where at in :stamps
