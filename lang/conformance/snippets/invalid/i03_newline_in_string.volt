// spec: §string-literals, a single-line string must not contain a line break
Table t {
  id int [note: "spans
lines"]
}
