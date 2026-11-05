// Package db is the itest data fixture: the schema whose generated models
// and queries sit beside the hand-written check targets (§validators).
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
select latest for users [order: (id desc), limit: :limit]

table profiles {
	user_id int    [pk]
	bio     string
}

ref: profiles.user_id > users.id [delete: cascade]

select described for users where exists profiles (bio like :bio?) and id >= :from? [order: (id asc)]

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
