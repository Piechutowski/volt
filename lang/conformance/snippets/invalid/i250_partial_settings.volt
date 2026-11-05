// spec: §tablepartial, a TablePartial takes no settings; its body holds columns, indexes and validators
// want: a table_partial takes no settings
table_partial stamps [model: "Stamp"] {
  created_at time.Time
}
table users {
  id int [pk]
  ~stamps
}
