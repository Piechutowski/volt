// spec: §types, §line-structure-and-whitespace, a Type declaration is line-oriented: its storage type and from path stay on its line
// want: expected end of line after type declaration
type money.Money int from
  "example.com/app/money"

type Slug
  string

table products {
  id    int         [pk]
  price money.Money
  slug  Slug
}
