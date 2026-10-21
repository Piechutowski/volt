// spec: §program-structure, Enum is not an element: a value set is a declared Type with Go constants beside the schema
// want: unknown element type "Enum"
Enum order_state {
  pending
  paid
}
Table orders {
  id int [pk]
}
