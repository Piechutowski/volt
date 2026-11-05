// spec: §multi-line-string-literals, a multi-line string closes at the first """, so a body that ends in a raw double quote is an error naming \", reported once
// want: a multi-line string cannot end with a double quote
table users [model: """say "hi""""] {
  id int [pk]
}
