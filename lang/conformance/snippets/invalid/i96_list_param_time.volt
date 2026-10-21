// spec: §predicates — a date/time column cannot take a list parameter: JSON carries no date/time value
// want: cannot take a list parameter
package db

Table page_views {
  id int [pk]
  at time.Time [not null]
}

Select picked for page_views where at in :stamps
