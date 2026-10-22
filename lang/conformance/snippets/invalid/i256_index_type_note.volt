// spec: §indexes, type: and note: are not index settings
// want: unknown setting "type"
Table bookings {
  id int [pk]
  country string
  Indexes {
    country [type: hash]
    id [note: "by id"]
  }
}
