// spec: §keywords, §relationships-ref, a referential action is a lowercase keyword
// want: write cascade, not CASCADE
table users { id int [pk] }
table posts { user_id int [null] }
ref: posts.user_id > users.id [delete: CASCADE, update: Set_Null]
