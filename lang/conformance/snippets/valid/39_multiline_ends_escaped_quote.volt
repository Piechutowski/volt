// spec: §multi-line-string-literals, a body that ends in a double quote writes it \", and one or two double quotes in a row stand in a body as themselves
table posts [note: """say "hi", ""twice"" and end on \""""] {
  id int
}

table tags [note: """
  a quote "inside", ""two"", then \"\"\" three
  and a last line that ends on \"
"""] {
  id int
}
