package db

Table users {
	id         int       [pk, increment]
	email      string    [required, unique]
	created_at time.Time [default: `CURRENT_TIMESTAMP`]

	Validators {
		EmailShape(email)
		EmailValid(email)
	}
}

Select picked for users where id in :ids [order: (id asc)]
Select since for users where created_at > :since [order: (id asc)]

Table tags {
	slug string [pk]
	name string
}

Table ms_revenue {
	id   int [pk, increment]
	org  string
	year int
}

Table ms_usage {
	id   int [pk, increment]
	org  string
	year int
}

Group series {
	ms_revenue
	ms_usage
}

Select browse for series where year = :year [order: (id asc)]
