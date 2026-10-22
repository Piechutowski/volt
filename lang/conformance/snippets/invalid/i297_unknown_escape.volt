// spec: §escape-sequences, a string takes the escapes \\, \", \n and \t only
// want: unknown escape sequence \q
Table posts {
  id    int    [pk]
  a string [note: "unknown \q escape"]
  b string [note: "no \u0041 escapes"]
  c string [note: "no \' escape either"]
  d string [note: "nor \r"]
}
