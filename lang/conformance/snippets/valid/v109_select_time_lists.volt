// spec: §predicates, §selects-over-groups, a date/time column takes a list parameter and keys a with, a declared type stored as time.Time too: JSON carries each value as the canonical text the column holds (§go-types)
package db

type stamp.Stamp time.Time from "example.com/app/stamp"

table runs {
  started_at time.Time [unique]
}

table samples {
  id             int       [pk]
  run_started_at time.Time [ref: runs.started_at]
  at             stamp.Stamp
}

select picked  for samples where run_started_at in :stamps
select stamped for samples where at in :ats
select trail   for runs with samples
