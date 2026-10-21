// spec: §columns, §types, §validators, §selects-over-groups (a Type declaration in error stands for the columns that use it)
// want: is not a built-in
package db

Type Score int32

TablePartial weighted {
  weight Score [not null]
}

Table posts {
  id    int   [pk, increment]
  score Score [not null]
  ~weighted

  validators {
    ScoreSane(score)
  }
}

Table comments {
  id int [pk, increment]
  ~weighted
}

Select top for posts where score > :min [order: (score desc)]
