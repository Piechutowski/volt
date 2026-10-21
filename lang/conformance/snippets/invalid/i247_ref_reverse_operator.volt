// spec: §relationships-ref, a relationship has one direction: the foreign key on the left of '>'; '<', '-' and '<>' are not operators
// want: expected '>' after the foreign key
Table users { id int [pk] }
Table posts { user_id int }
Table infos { user_id int }
Table books { id int }
Ref: users.id < posts.user_id
Ref: users.id - infos.user_id
Ref: users.id <> books.id
