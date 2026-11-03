package db

table members {
	id    int    [pk]
	email string [required, unique]
	name  string [required]

	validators {
		EmailShape(email)
	}
}
