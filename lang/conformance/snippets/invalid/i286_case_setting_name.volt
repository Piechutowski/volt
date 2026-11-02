// spec: §keywords, §settings-lists, a setting name is a lowercase keyword: PK and Null are errors naming pk and null
// want: write pk, not PK
table users {
  id   int    [PK]
  name string [Null, Note: "shown"]
}
