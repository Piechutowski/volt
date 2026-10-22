// spec: §records-sample-data, every record value kind, null, a string for a declared type stored as string
Type Status string
Table users {
  id int
  name string
  age int [null]
  status Status
  created_at time.Time

  Records (id, name, age, status, created_at) {
    1, "Alice", 30, "active", "2024-01-15 10:30:00"
    2, "Bob", null, "inactive", `now()`
    3, "Charlie", null, "pending", "2024-01-15"
    4, "", -5, "pending", "2024-01-15T10:30:00.000+07:00"
    5, "Dora", 15000000000, "active", `uuid_generate_v4()`
  }
}
