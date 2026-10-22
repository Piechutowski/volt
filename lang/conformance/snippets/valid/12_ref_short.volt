// spec: §relationships-ref, the Ref element with every referential action
Table users { id int }
Table posts { user_id int }
Table a { x int }
Table b { x int [null] }
Table c { x int [null] }
Table d { x int }
Ref: posts.user_id > users.id [delete: cascade]
Ref: a.x > users.id [delete: restrict]
Ref: b.x > users.id [update: set_null]
Ref: c.x > users.id [delete: set_default]
Ref: d.x > users.id
