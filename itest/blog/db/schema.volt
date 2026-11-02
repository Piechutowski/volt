package db

table users {
	id         int       [pk]
	email      string    [required, unique]
	created_at time.Time [default: `CURRENT_TIMESTAMP`]

	validators {
		EmailShape(email)
		EmailValid(email)
	}
}

select picked for users where id in :ids [order: (id asc)]
select since for users where created_at > :since [order: (id asc)]

table tags {
	slug string [pk]
	name string
}

table ms_revenue {
	id   int [pk]
	org  string
	year int
}

table ms_usage {
	id   int [pk]
	org  string
	year int
}

group series {
	ms_revenue
	ms_usage
}

select browse for series where year = :year [order: (id asc)]
