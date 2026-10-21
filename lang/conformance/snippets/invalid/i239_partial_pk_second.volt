// spec: §columns, §tablepartial, the one primary key is counted after partial injection
// want: declares a second primary key
TablePartial keyed {
  id int [pk]
}

Table posts {
  ~keyed
  slug string [pk]
}
