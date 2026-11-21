// polymorphic association: one comments table pointing at several owners

type CommentableType string
// Polymorphic: (commentable_type, commentable_id) addresses a row
// in whichever table the type names. No FK can enforce this.
table comments {
  id int [pk]
  commentable_type CommentableType
  // ID within the table selected by commentable_type
  commentable_id int
  body string

  indexes {
    (commentable_type, commentable_id)
  }

  validators {
    CommentableTypeKnown(commentable_type)
  }
}
