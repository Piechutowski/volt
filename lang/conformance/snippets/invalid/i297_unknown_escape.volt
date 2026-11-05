// spec: §escape-sequences, a string takes the escapes \\, \", \n and \t only
// want: unknown escape sequence \q
table posts {
  id    int    [pk]
  a string [default: "unknown \q escape"]
  b string [default: "no \u0041 escapes"]
  c string [default: "no \' escape either"]
  d string [default: "nor \r"]
}
