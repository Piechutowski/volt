// spec: §validation-checks — date/time columns are out: the Go tier cannot mirror
// SQL's text-time comparison
package db

Table posts {
  id int [pk]
  created_at time.Time [not null]

  checks {
    created_at > '2020-01-01'
  }
}
