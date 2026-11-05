// spec: §keywords, §settings-lists, a setting name is a lowercase keyword: PK, Null and Unique are errors naming pk, null and unique
// want: write pk, not PK
table users {
  id   int    [PK]
  name string [Null, Unique]
}
