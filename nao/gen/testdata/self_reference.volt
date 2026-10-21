// self-referencing hierarchy with a nullable FK to the same table
Table employees {
  id int [pk, increment]
  full_name string
  manager_id int [null, ref: employees.id, note: 'NULL for the CEO']
}
