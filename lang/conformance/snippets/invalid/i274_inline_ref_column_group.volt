// spec: §relationships-ref, an inline ref names one column: a composite relationship is a Ref element
// want: an inline ref endpoint has exactly one column
Table merchants {
  id           int    [pk]
  country_code string [pk]
}
Table periods {
  id          int [pk]
  merchant_id int [ref: merchants.(id, country_code)]
}
