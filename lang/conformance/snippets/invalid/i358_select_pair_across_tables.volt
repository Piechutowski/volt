// spec: §selects-over-groups, two selects on different tables that mint one method collide
// want: both generate UserTagRows
package app

Table users {
	id int [pk]
}

Table user_tags {
	id   int [pk]
	name string
}

Select tag_rows for users
Select rows     for user_tags
