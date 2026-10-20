// spec: §indexes, §validators — multiple indexes and validators blocks per body; contents accumulate
Table t {
  id int
  name string
  indexes { id }
  validators { IDPositive(id) }
  indexes { name }
  validators { IDSmall(id) }
}
