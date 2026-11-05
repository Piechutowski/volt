// spec: §selects-over-groups, §relationships-ref — a composite key joins on every pair, under the relation named after the referenced table
package db

table merchants {
  id           int    [pk]
  country_code string [pk]
  name         string
}

table merchant_periods {
  id           int    [pk]
  merchant_id  int
  country_code string
}

ref: merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)

select placed (id, merchants.name) for merchant_periods join merchants
select spans (id, name, merchant_periods.id) for merchants join merchant_periods
