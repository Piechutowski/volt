package db

Table users {
	id         int       [pk, increment]
	email      string    [not null, required, unique]
	created_at time.Time [not null, default: `CURRENT_TIMESTAMP`]

	validators {
		EmailShape(email)
		EmailValid(email)
	}
}

Select picked for users where id in :ids [order: (id asc)]
Select since for users where created_at > :since [order: (id asc)]

Table tags {
	slug string [pk]
	name string [not null]
}

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
