// spec: §columns, an inline ref names one column, without parentheses: a composite relationship is a Ref element
// want: an inline ref endpoint is table.column, without parentheses
Table merchants {
  id           int    [pk]
  country_code string [pk]
}
Table periods {
  id          int [pk]
  merchant_id int [ref: merchants.(id, country_code)]
}
