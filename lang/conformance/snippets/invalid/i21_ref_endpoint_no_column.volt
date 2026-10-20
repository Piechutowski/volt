// spec: §relationships-ref — an endpoint must reference a column
Table users { id int }
Table posts { user_id int }
Ref: users > posts.user_id
