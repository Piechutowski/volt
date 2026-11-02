// spec: §indexes, type: and note: are not index settings
// want: unknown setting "type"
table bookings {
  id int [pk]
  country string
  indexes {
    (country, id) [type: hash]
    (id, country) [note: "by id"]
  }
}
