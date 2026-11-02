// spec: §selects-over-groups — the keys of a with travel as one JSON list, which carries no date/time
// want: cannot travel as one JSON list
package db

table runs {
  started_at time.Time [unique]
}

table samples {
  id             int       [pk, increment]
  run_started_at time.Time [ref: runs.started_at]
}

select trail for runs with samples
