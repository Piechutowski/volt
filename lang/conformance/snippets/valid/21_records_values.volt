// spec: §records-sample-data — every record value kind, empty fields, enum constants
Enum Status { active
  inactive
  pending }
Table users {
  id int
  name string
  age int [null]
  status Status
  created_at time.Time

  records (id, name, age, status, created_at) {
    1, 'Alice', 30, Status.active, '2024-01-15 10:30:00'
    2, 'Bob', null, Status.inactive, `now()`
    3, 'Charlie', , Status.pending, '2024-01-15'
    4, '', -5, Status.pending, '2024-01-15T10:30:00.000+07:00'
    5, 'Dora', 15000000000, Status.active, `uuid_generate_v4()`
  }
}
