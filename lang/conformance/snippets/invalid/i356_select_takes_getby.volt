// spec: §selects-over-groups, a select's method does not take the name of a GetBy method
// want: select "get_by_email" mints UserGetByEmail for table "users", which is already the method UserGetByEmail
package app

table users {
	id    int    [pk]
	email string [unique]
}

select get_by_email for users where email = :e
