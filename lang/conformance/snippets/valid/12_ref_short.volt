// spec/relationships-ref, the Ref element with every referential action
table users { id int }
table posts { user_id int }
table a { x int }
table b { x int [null] }
table c { x int [null] }
table d { x int }
ref: posts.user_id > users.id [delete: cascade]
ref: a.x > users.id [delete: restrict]
ref: b.x > users.id [update: null_set]
ref: c.x > users.id [delete: default_set]
ref: d.x > users.id
