// spec: §types, the from path is an import path
// want: which is not a Go import path
Type money.Money int from "not a path!"

Table products {
  id    int         [pk]
  price money.Money
}
