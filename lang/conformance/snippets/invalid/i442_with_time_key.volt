// spec: §selects-over-groups — the keys of a with travel as one JSON list, which carries no date/time
// want: cannot travel as one JSON list
package db

Table runs {
  started_at time.Time [unique]
}

Table samples {
  id             int       [pk, increment]
  run_started_at time.Time [ref: runs.started_at]
}

Select trail for runs with samples
