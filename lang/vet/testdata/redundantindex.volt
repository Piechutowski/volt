// analyzers: redundantindex
Table t {
  id int [pk]
  email string [unique]
  country string

  indexes {
    id [unique] //WANT redundantindex
    email [unique] //WANT redundantindex
    (country, email) [pk, unique] //WANT redundantindex
    country
    country //WANT redundantindex
    (email, country)
    (email, country) //WANT redundantindex
    (country, id)
  }
}
