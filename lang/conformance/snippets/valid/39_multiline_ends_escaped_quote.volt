// spec: §multi-line-string-literals, a body that ends in a double quote writes it \", and one or two double quotes in a row stand in a body as themselves
table posts {
  id int
  body string [default: """say "hi", ""twice"" and end on \""""]
}

table tags {
  id int
  body string [default: """
  a quote "inside", ""two"", then \"\"\" three
  and a last line that ends on \"
"""]
}
