package db

Table users {
	id    integer [pk, increment]
	email text    [not null, unique]
}

Select picked for users where id in :ids [order: (id asc)]
