// spec: §records-sample-data, a declared type column takes a literal of its storage type
type money.Money int from "example.com/app/money"
type Slug string
table products {
  id int [pk]
  price money.Money
  slug Slug
  records {
    1, 1999, "first"
    2, -5, "second"
  }
}
