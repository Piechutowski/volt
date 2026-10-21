// spec: §records-sample-data — every value kind, typed against its column: integer, number, string, boolean, enum constant, null, empty field, expression
Enum status {
  active
  retired
}
Table metrics {
  id int [pk]
  ratio float64
  label string
  live bool
  state status
  memo string [null]
  seen time.Time [null]
  blob []byte [null]
  records {
    1, 0.5, 'first', true, status.active, null, '2024-01-15 10:30:00', `X'00'`
    2, 3, 'second', false, status.retired, null, null, null
  }
}
