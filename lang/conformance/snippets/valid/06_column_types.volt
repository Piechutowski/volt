// spec: §columns, every built-in type, the [] of []byte, enum types
Enum job_status { queued }
Enum grade { a }
Table t {
  a bool
  b int
  c float64
  d string
  e []byte
  f time.Time
  g grade
  h job_status
}
