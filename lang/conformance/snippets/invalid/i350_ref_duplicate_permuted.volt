// spec: §relationships-ref, a composite relationship that lists the same column pairs in another order is the same relationship
// want: duplicate relationship
table b {
  p int [pk]
  q int [pk]
}

table a {
  x int
  y int
}

ref: a.(x, y) > b.(p, q)

ref: a.(y, x) > b.(q, p)
