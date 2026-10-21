// spec: §program-structure, TableGroup, DiagramView and sticky notes are not elements
// want: unknown element type "TableGroup"
Table users {
  id int [pk]
}
TableGroup accounts {
  users
}
DiagramView overview {
  Tables { * }
}
Note reminder {
  'remember'
}
