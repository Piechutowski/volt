// spec: §indexes, §validators — multiple indexes and validators blocks per body; contents accumulate
Table t {
  id int
  name string
  Indexes { id }
  Validators { IDPositive(id) }
  Indexes { name }
  Validators { IDSmall(id) }
}
