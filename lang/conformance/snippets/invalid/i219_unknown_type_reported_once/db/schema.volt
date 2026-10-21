package db

Table authors {
  id    int64  [pk, increment]
  email string [not null, unique]
  rank  int32  [not null]

  validators {
    RankSane(rank)
  }
}

Select ranked for authors where rank >= :min [order: (rank desc)]
