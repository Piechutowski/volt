// analyzers: selfref
table employees {
  id int [pk]
  manager_id int [null]
}
ref: employees.manager_id > employees.id
ref: employees.id > employees.id //WANT selfref
