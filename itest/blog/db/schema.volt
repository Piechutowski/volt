package db

Table users {
	id    int [pk, increment]
	email string    [not null, required, unique]

	validators {
		EmailShape(email)
		EmailValid(email)
	}
}

Select picked for users where id in :ids [order: (id asc)]

Table ms_revenue {
	id   int [pk, increment]
	org  string    [not null]
	year int [not null]
}

Table ms_usage {
	id   int [pk, increment]
	org  string    [not null]
	year int [not null]
}

Group series {
	ms_revenue
	ms_usage
}

Select browse for series where year = :year [order: (id asc)]
