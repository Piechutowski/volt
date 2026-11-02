// spec: §selects-over-groups, a select's method does not take the name of a GetBy method
// want: select "get_by_email" mints UserGetByEmail for table "users", which is already the method UserGetByEmail
package app

Table users {
	id    int    [pk]
	email string [unique]
}

Select get_by_email for users where email = :e
