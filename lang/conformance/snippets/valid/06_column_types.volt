// spec: §columns — every built-in type, the [] of []byte, schema-qualified and bare enum types
Enum v2.job_status { created }
Enum job_status { queued }
Table t {
  a bool
  b int
  c float64
  d string
  e []byte
  f time.Time
  g v2.job_status
  h job_status
}
