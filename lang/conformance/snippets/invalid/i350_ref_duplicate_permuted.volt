// spec: §relationships-ref, a composite relationship that lists the same column pairs in another order is the same relationship
// want: duplicate relationship
Table b {
  p int [pk]
  q int [pk]
}

Table a {
  x int
  y int
}

Ref: a.(x, y) > b.(p, q)

Ref: a.(y, x) > b.(q, p)
