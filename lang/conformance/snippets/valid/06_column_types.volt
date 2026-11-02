// spec: §columns, every built-in type, the [] of []byte, declared types
type JobStatus string
type Grade string
table t {
  a bool
  b int
  c float64
  d string
  e []byte
  f time.Time
  g Grade
  h JobStatus
}
