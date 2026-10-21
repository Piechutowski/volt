// spec: §case-sensitivity-and-terminology — keywords are case-insensitive
TABLE users {
  id int [PK]
}
enum status { active }
table posts { user_id int [Ref: > users.id] }
table likes { user_id int }
REF r: users.id < likes.user_id
