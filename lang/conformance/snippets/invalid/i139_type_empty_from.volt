// spec: §types, the from path is a non-empty import path
// want: has an empty from path
Type m.M int from ''
Table t {
  m m.M
}
