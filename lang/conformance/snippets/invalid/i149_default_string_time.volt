// spec: §default-values, a time.Time column takes only an expression literal or null as default
// want: must be an expression literal, found string
Table t {
  at time.Time [default: '2024-01-15 10:30:00']
}
