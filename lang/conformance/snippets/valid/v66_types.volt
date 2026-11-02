// spec: §types — declared Go types as column types, qualified with a from path and bare for the package's own
type money.Money int    from "example.com/app/money"
type png.Png     []byte from "image/png"
type Slug        string

type Status string

table products {
  id    int         [pk, increment]
  slug  Slug        [unique, required]
  price money.Money [default: 0]
  icon  png.Png
  state Status      [default: "active"]
  data  []byte
  at    time.Time   [default: `CURRENT_TIMESTAMP`]
  ratio float64     [default: 1]
  ok    bool        [default: true]
}
