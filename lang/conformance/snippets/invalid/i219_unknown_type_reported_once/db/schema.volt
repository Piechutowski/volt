package db

table authors {
  id    int64  [pk, increment]
  email string [unique]
  rank  int32

  validators {
    RankSane(rank)
  }
}

select ranked for authors where rank >= :min [order: (rank desc)]
