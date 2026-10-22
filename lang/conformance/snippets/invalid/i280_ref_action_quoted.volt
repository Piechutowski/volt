// spec: §relationships-ref, a referential action is a word: a quoted action and a missing one are reported once each
// want: "delete" must be a referential action: cascade, restrict, set_null or set_default
Table users { id int [pk] }
Table posts { user_id int [null] }
Ref: posts.user_id > users.id [delete: 'set null', update]
