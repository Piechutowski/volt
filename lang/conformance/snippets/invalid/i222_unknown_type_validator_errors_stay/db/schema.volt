// spec: §columns, §validators (an unknown type silences only the parameter it fills)
// want: no function RankSane
package db

Table authors {
  id    int    [pk, increment]
  email string [not null]
  rank  int32  [not null]

  validators {
    RankSane(rank)
    TwoArgs(rank)
    Mixed(rank, email)
    Later(rank, nope)
    Fine(rank)
  }
}
