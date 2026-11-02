// spec: §relationships-ref — an endpoint must reference a column
table users { id int }
table posts { user_id int }
ref: users > posts.user_id
