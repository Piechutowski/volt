package db

Table authors {
  id    int64  [pk, increment]
  email string [unique]
  rank  int32

  Validators {
    RankSane(rank)
  }
}

Select ranked for authors where rank >= :min [order: (rank desc)]
