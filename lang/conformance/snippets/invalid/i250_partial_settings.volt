// spec: §tablepartial, a TablePartial takes no settings; its body holds columns, indexes and validators
// want: a TablePartial takes no settings
TablePartial stamps [note: 'timestamps'] {
  created_at time.Time
}
Table users {
  id int [pk]
  ~stamps
}
