// spec: §indexes, type: and note: are not index settings
// want: unknown setting "type"
Table bookings {
  id int [pk]
  country string
  indexes {
    country [type: hash]
    id [note: 'by id']
  }
}
