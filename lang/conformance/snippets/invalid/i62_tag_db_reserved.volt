// spec: §columns tag extension — db is the scan contract, not overridable
Table users {
  id   int [pk]
  name string [tag: 'db:"other_name"']
}
