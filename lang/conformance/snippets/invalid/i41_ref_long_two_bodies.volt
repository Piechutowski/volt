// spec: §relationships-ref — a Ref element contains exactly one relationship
Table a { x int }
Table b { x int }
Table c { x int }
Ref {
  a.x > b.x
  a.x > c.x
}
