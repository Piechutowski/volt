// spec: §predicates: a declared type stored as time.Time cannot take a list parameter
// want: cannot take a list parameter
package db

Type Stamp time.Time

Table events {
  id int   [pk, increment]
  at Stamp [not null]
}

Select picked for events where at in :stamps
