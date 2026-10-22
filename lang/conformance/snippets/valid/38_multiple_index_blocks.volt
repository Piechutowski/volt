// spec: §indexes, §validators, multiple Indexes and Validators blocks per body; contents accumulate
Table t {
  id int
  name string
  Indexes { (id, name) }
  Validators { IDPositive(id) }
  Indexes { name [where: `id > 0`] }
  Validators { IDSmall(id) }
}
