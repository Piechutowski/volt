// spec: §indexes, §columns, a single-column index is the column setting unique or index; an Indexes block holds every composite, expression and partial key form with every index setting
Table bookings {
  id int [pk]
  country string [pk]
  booking_date time.Time [index]
  created_at time.Time [unique]

  Indexes {
    (country, booking_date) [unique]
    (country, created_at) [name: "idx_bookings_by_country"]
    (`id*2`) [name: "idx_bookings_double"]
    `id*4` [name: "idx_bookings_quadruple"]
    (`id*3`, `getdate()`) [name: "idx_bookings_triple_now"]
    (`id*3`, id) [name: "idx_bookings_triple_id"]
    country [where: `country != ''`]
    created_at [unique, name: "idx_bookings_created_live", where: `country != ''`]
  }
}
