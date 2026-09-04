package db

Table users {
	id    integer [pk, increment]
	email text    [not null, unique]

	checks {
		email like '%_@%_' [name: 'email_shape']
		EmailValid(email)
	}
}

Select picked for users where id in :ids [order: (id asc)]

Table ms_revenue {
	id   integer [pk, increment]
	org  text    [not null]
	year integer [not null]
}

Table ms_usage {
	id   integer [pk, increment]
	org  text    [not null]
	year integer [not null]
}

Group series {
	ms_revenue
	ms_usage
}

Select browse for series where year = :year [order: (id asc)]
