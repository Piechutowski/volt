// spec: §relationships-ref, a relationship has one direction: the foreign key on the left of '>'; '<', '-' and '<>' are not operators
// want: expected '>' after the foreign key
table users { id int [pk] }
table posts { user_id int }
table infos { user_id int }
table books { id int }
ref: users.id < posts.user_id
ref: users.id - infos.user_id
ref: users.id <> books.id
