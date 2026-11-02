// spec: §numeric-literals — digits with dot followed by letters is a lexical error
table t {
  id int [default: 12.3abc]
}
