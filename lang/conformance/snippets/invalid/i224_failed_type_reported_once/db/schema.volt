// spec: §columns, §types, §validators, §selects-over-groups (a Type declaration in error stands for the columns that use it)
// want: is not a built-in
package db

type Score int32

table_partial weighted {
  weight Score
}

table posts {
  id    int   [pk, increment]
  score Score
  ~weighted

  validators {
    ScoreSane(score)
  }
}

table comments {
  id int [pk, increment]
  ~weighted
}

select top for posts where score > :min [order: (score desc)]
