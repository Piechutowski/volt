// analyzers: casecollision
table users { id int }
table Users { id int } //WANT casecollision
table clean {
  total int
  Total int //WANT casecollision
}
