// spec: §indexes, type: is not an index setting
// want: unknown setting "type"
table bookings {
  id int [pk]
  country string
  indexes {
    (country, id) [type: hash]
  }
}
