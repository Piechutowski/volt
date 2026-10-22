// analyzers: missingpk
Table has_col_pk {
  id int [pk]
}
Table has_composite_pk {
  a int [pk]
  b int [pk]
}
Table naked { //WANT missingpk
  id int
}
