// spec: §case-sensitivity-and-terminology — keywords are case-insensitive
TABLE users {
  id int [PK]
}
type Status string
table posts { user_id int [Ref: users.id] }
table likes { user_id int }
REF: likes.user_id > users.id
