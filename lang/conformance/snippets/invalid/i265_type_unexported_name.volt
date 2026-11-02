// spec: §types, a declared type names an exported Go type: a bare name begins with an upper-case letter, and pkg.Name names an exported type
// want: is not an exported Go name
type slug string
type money.amount int from "example.com/money"
table posts {
  id    int    [pk]
  slug  slug
  price money.amount
}
