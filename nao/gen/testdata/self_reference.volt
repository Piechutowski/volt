// self-referencing hierarchy with a nullable FK to the same table

table employees {
  id int [pk]
  full_name string
  // NULL for the CEO
  manager_id int [null, ref: employees.id]
}
