// spec: §relationships-ref — composite foreign keys
Table merchants { id int
  country_code string }
Table merchant_periods { merchant_id int
  country_code string }
Ref: merchant_periods.(merchant_id, country_code) > merchants.(id, country_code)
