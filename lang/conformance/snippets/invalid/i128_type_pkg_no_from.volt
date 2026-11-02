// spec: §types — a qualified declared name needs its from path
// want: has no from path
type money.Money int
table t {
  m money.Money
}
