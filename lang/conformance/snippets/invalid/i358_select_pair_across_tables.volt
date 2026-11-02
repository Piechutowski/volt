// spec: §selects-over-groups, two selects on different tables that mint one method collide
// want: both generate UserTagRows
package app

table users {
	id int [pk]
}

table user_tags {
	id   int [pk]
	name string
}

select tag_rows for users
select rows     for user_tags
