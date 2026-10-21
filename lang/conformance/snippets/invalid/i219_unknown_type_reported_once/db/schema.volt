package db

Table authors {
  id    int64  [pk, increment]
  email string [unique]
  rank  int32

  validators {
    RankSane(rank)
  }
}

Select ranked for authors where rank >= :min [order: (rank desc)]
