// spec: §relationships-ref — composite endpoints must have equal column counts
Table a { x int
  y int }
Table b { x int }
Ref: a.(x, y) > b.(x)
