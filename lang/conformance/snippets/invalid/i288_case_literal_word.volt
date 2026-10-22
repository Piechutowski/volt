// spec: §keywords, §boolean-and-null-literals, true, false and null are lowercase keywords where a value stands
// want: write true, not TRUE
Table flags {
  id     int    [pk]
  active bool   [default: TRUE]
  note   string [null]
}

Records flags(id, active, note) {
  1, False, NULL
}
