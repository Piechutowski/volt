// spec: §relationships-ref — composite foreign keys
table merchants { id int
  country_code string }
table merchant_periods { merchant_id int
  country_code string }
ref: merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)
