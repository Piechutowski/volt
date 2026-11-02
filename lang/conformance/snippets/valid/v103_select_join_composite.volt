// spec: §selects-over-groups, §relationships-ref — a composite key joins on every pair, under the relation named after the referenced table
package db

Table merchants {
  id           int    [pk]
  country_code string [pk]
  name         string
}

Table merchant_periods {
  id           int    [pk, increment]
  merchant_id  int
  country_code string
}

Ref: merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)

Select placed (id, merchants.name) for merchant_periods join merchants
Select spans (id, name, merchant_periods.id) for merchants join merchant_periods
