// spec: §indexes — all four index-key forms and every index setting
Table bookings {
  id int
  country string
  booking_date time.Time
  created_at time.Time

  Indexes {
    (id, country) [pk]
    created_at [name: "created_at_index"]
    booking_date
    (country, booking_date) [unique]
    (`id*2`) [name: "idx_bookings_double"]
    (`id*3`, `getdate()`) [name: "idx_bookings_triple_now"]
    (`id*3`, id) [name: "idx_bookings_triple_id"]
    country [where: `country != ''`]
  }
}
