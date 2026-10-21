// analyzers: shadowedcolumn
TablePartial base {
  id int
  created_at time.Time
}
Table t {
  ~base
  id string //WANT shadowedcolumn
  name string
}

// between two partials, the later injection silently wins (§tablepartial)
TablePartial audit_a {
  updated_at time.Time
}
TablePartial audit_b {
  updated_at time.Time //WANT shadowedcolumn
}
Table u {
  ~audit_a
  ~audit_b
  x int
}
