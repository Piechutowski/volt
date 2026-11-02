// spec: §relationships-ref, a composite that pairs the same column sets differently is another relationship
table b {
  p int [pk]
  q int [pk]
}

table a {
  x int
  y int
}

ref: a.(x, y) > b.(p, q)

ref: a.(x, y) > b.(q, p)
