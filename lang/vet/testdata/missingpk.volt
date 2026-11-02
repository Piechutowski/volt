// analyzers: missingpk
table has_col_pk {
  id int [pk]
}
table has_composite_pk {
  a int [pk]
  b int [pk]
}
table naked { //WANT missingpk
  id int
}
