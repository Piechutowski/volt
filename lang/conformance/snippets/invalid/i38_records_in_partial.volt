// spec: §tablepartial — partial body has no records
table_partial p {
  id int
  records (id) { 1 }
}
