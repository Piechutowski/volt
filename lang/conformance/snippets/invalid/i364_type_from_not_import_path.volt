// spec: §types, the from path is an import path
// want: which is not a Go import path
type money.Money int from "not a path!"

table products {
  id    int         [pk]
  price money.Money
}
