// spec: §settings-lists, §relationships-ref, a setting value is one identifier: set null in two words is a syntax error, set_null is the action
// want: expected ',' or ']' in settings list, found identifier "null"
table users { id int [pk] }
table posts { user_id int [null] }
ref: posts.user_id > users.id [delete: set null]
