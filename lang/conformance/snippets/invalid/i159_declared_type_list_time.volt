// spec: §predicates: a declared type stored as time.Time cannot take a list parameter
// want: cannot take a list parameter
package db

type stamp.Stamp time.Time from "example.com/app/stamp"

table events {
  id int   [pk, increment]
  at stamp.Stamp
}

select picked for events where at in :stamps
