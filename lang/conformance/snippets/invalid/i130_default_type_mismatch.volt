// spec: §default-values — a default must fit the column's type
// want: must be an integer, found string
table t {
  n int [default: "seven"]
}
