// polymorphic association: one comments table pointing at several owners
Type CommentableType string
Table comments [note: '''
  Polymorphic: (commentable_type, commentable_id) addresses a row
  in whichever table the type names. No FK can enforce this.
'''] {
  id int [pk, increment]
  commentable_type CommentableType
  commentable_id int [note: 'ID within the table selected by commentable_type']
  body string

  indexes {
    (commentable_type, commentable_id)
  }

  validators {
    CommentableTypeKnown(commentable_type)
  }
}
