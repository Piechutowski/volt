// analyzers: shadowedcolumn

table_partial base {
  id int
  created_at time.Time
}
table t {
  ~base
  id string //WANT shadowedcolumn
  name string
}

// between two partials, the later injection silently wins (§tablepartial)
table_partial audit_a {
  updated_at time.Time
}
table_partial audit_b {
  updated_at time.Time //WANT shadowedcolumn
}
table u {
  ~audit_a
  ~audit_b
  x int
}

// a partial injected twice brings each of its columns twice
table_partial stamps {
  created_at time.Time //WANT shadowedcolumn
}
table v {
  id int
  ~stamps
  ~stamps
}
