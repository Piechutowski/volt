// spec: §selects-over-groups — with takes a relation over one key column
// want: over one key column
package db

Table merchants {
  id           int    [pk]
  country_code string [pk]
}

Table periods {
  id           int    [pk, increment]
  merchant_id  int
  country_code string
}

Ref: periods.(merchant_id, country_code) > merchants.(id, country_code)

Select rows for merchants with periods
