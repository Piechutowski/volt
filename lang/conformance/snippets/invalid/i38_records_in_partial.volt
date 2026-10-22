// spec: §tablepartial — partial body has no records
TablePartial p {
  id int
  Records (id) { 1 }
}
