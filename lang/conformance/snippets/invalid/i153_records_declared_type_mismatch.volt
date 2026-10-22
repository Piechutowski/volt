// spec: §records-sample-data, a value for a declared type is a literal of its storage type
// want: must be an integer, found string
Type money.Money int from "example.com/app/money"
Table t {
  id int [pk]
  price money.Money
  Records {
    1, "free"
  }
}
