// spec: §columns, every built-in type, the [] of []byte, declared types
Type JobStatus string
Type Grade string
Table t {
  a bool
  b int
  c float64
  d string
  e []byte
  f time.Time
  g Grade
  h JobStatus
}
