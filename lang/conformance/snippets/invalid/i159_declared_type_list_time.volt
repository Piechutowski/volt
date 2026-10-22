// spec: §predicates: a declared type stored as time.Time cannot take a list parameter
// want: cannot take a list parameter
package db

Type stamp.Stamp time.Time from "example.com/app/stamp"

Table events {
  id int   [pk, increment]
  at stamp.Stamp
}

Select picked for events where at in :stamps
