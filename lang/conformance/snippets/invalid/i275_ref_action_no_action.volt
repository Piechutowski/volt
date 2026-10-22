// spec: §relationships-ref, the referential actions are cascade, restrict, set_null and set_default; SQLite's default is written by leaving the setting out, and no_action is not an action
// want: invalid referential action "no_action" for 'update'
Table users { id int [pk] }
Table posts { user_id int }
Ref: posts.user_id > users.id [delete: cascade, update: no_action]
