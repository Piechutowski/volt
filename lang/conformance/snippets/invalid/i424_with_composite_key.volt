// spec: §selects-over-groups — with takes a relation over one key column
// want: over one key column
package db

table merchants {
  id           int    [pk]
  country_code string [pk]
}

table periods {
  id           int    [pk, increment]
  merchant_id  int
  country_code string
}

ref: periods.(merchant_id, country_code) > merchants.(id, country_code)

select rows for merchants with periods
