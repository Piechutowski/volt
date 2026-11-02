// spec: §selects-over-groups, a select's method does not take the name of another table's CRUD method
// want: which is already the CRUD method UserTagList of table "user_tags"
package app

Table users {
	id int [pk]
}

Table user_tags {
	id   int [pk]
	name string
}

Select tag_list for users
