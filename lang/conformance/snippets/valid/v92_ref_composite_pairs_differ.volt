// spec: §relationships-ref, a composite that pairs the same column sets differently is another relationship
Table b {
  p int [pk]
  q int [pk]
}

Table a {
  x int
  y int
}

Ref: a.(x, y) > b.(p, q)

Ref: a.(x, y) > b.(q, p)
