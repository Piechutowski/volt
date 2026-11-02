// spec: §types, the from path is a non-empty import path
// want: has an empty from path
type m.M int from ""
table t {
  m m.M
}
