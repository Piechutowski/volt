// spec: §indexes, §validators, multiple Indexes and Validators blocks per body; contents accumulate
table t {
  id int
  name string
  indexes { (id, name) }
  validators { IDPositive(id) }
  indexes { name [where: `id > 0`] }
  validators { IDSmall(id) }
}
