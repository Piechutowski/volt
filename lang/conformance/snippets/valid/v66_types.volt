// spec: §types — declared Go types as column types, qualified with a from path and bare for the package's own
Type money.Money int    from 'example.com/app/money'
Type png.Png     []byte from 'image/png'
Type Slug        string

Enum status { active }

Table products {
  id    int         [pk, increment]
  slug  Slug        [unique, required]
  price money.Money [default: 0]
  icon  png.Png
  state status      [default: status.active]
  data  []byte
  at    time.Time   [default: `CURRENT_TIMESTAMP`]
  ratio float64     [default: 1]
  ok    bool        [default: true]
}
