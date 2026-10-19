// spec: §records-sample-data, a declared type column takes a literal of its storage type
Type money.Money int from 'example.com/app/money'
Type Slug string
Table products {
  id int [pk]
  price money.Money [not null]
  slug Slug [not null]
  records {
    1, 1999, 'first'
    2, -5, 'second'
  }
}
