// spec: §indexes, type: and note: are not index settings
// want: unknown setting "type"
Table bookings {
  id int [pk]
  country string
  Indexes {
    (country, id) [type: hash]
    (id, country) [note: "by id"]
  }
}
