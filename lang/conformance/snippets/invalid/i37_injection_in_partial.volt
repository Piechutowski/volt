// spec: §tablepartial — partial body has no nested injections
TablePartial a { id int }
TablePartial b {
  ~a
  name string
}
