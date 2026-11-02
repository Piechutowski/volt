// spec: §table, a table whose only injection brings no column has no column
// want: table "t" must contain at least one column, directly or from an injected TablePartial
TablePartial e {
}

Table t {
  ~e
}
