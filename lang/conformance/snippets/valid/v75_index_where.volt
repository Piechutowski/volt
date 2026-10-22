// spec: §indexes, where: makes an index partial and a key of columns names it
Table bookings {
  id int [pk]
  country string
  booking_date time.Time

  Indexes {
    booking_date [where: `country <> ''`]
    (country, booking_date) [unique]
  }
}
