// records at both positions; SQL backends turn them into INSERTs
Table currencies {
  code string [pk]
  name string

  records (code, name) {
    'EUR', 'Euro'
    'PLN', 'Polish złoty'
  }
}
Table rates [note: 'Seeded exchange rates'] {
  id int [pk, increment]
  currency_code string [ref: currencies.code]
  rate float64
  fetched_at time.Time
  active bool
  comment string [null]
}
records rates(id, currency_code, rate, fetched_at, active, comment) {
  1, 'EUR', 4.32, '2024-01-15 10:30:00', true, null
  2, 'PLN', 1.0, `datetime('now')`, false, null
}
