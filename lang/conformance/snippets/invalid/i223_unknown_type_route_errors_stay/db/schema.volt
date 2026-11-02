package db

table authors {
  id    int64  [pk, increment]
  email string [unique]
  rank  int32
}

select ranked for authors where rank >= :min [order: (rank desc)]
select shown (rank, email) for authors where email = :e
