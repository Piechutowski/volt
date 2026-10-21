// self-referencing hierarchy with a nullable FK to the same table
Table employees {
  id int [pk, increment]
  full_name string [not null]
  manager_id int [ref: > employees.id, note: 'NULL for the CEO']
}
