// spec: §selects-over-groups, a select's method does not take the name of another table's CRUD method
// want: which is already the CRUD method UserTagList of table "user_tags"
package app

table users {
	id int [pk]
}

table user_tags {
	id   int [pk]
	name string
}

select tag_list for users
