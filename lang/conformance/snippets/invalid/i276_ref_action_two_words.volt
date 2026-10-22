// spec: §settings-lists, §relationships-ref, a setting value is one identifier: set null in two words is a syntax error, set_null is the action
// want: expected ',' or ']' in settings list, found identifier "null"
Table users { id int [pk] }
Table posts { user_id int [null] }
Ref: posts.user_id > users.id [delete: set null]
