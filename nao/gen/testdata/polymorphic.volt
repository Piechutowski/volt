// polymorphic association: one comments table pointing at several owners
Enum commentable_type {
  post
  photo
  video
}
Table comments [note: '''
  Polymorphic: (commentable_type, commentable_id) addresses a row
  in whichever table the type names. No FK can enforce this.
'''] {
  id int [pk, increment]
  commentable_type commentable_type [not null]
  commentable_id int [not null, note: 'ID within the table selected by commentable_type']
  body string [not null]

  indexes {
    (commentable_type, commentable_id)
  }
}
