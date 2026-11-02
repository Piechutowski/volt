// spec: §settings
package app

table users {
	id int [pk]
}

scope / {
	resources users [only: (browse)]
}
