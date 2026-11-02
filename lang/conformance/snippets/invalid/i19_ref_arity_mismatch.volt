// spec: §relationships-ref — composite endpoints must have equal column counts
table a { x int
  y int }
table b { x int }
ref: a.(x, y) > b.(x)
