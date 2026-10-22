// spec: §keywords, §relationships-ref, a referential action is a lowercase keyword
// want: write cascade, not CASCADE
Table users { id int [pk] }
Table posts { user_id int [null] }
Ref: posts.user_id > users.id [delete: CASCADE, update: Set_Null]
