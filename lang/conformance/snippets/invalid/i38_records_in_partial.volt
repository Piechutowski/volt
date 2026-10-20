// spec: §tablepartial — partial body has no records
TablePartial p {
  id int
  records (id) { 1 }
}
