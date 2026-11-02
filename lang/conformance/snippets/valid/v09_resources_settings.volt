// spec: §settings
package app

table users {
	id int [pk]
}

table posts {
	id int [pk]
}

table drafts {
	id int [pk]
}

scope / {
	resources users  [api, param: uid]
	resources posts  [only: (index, show)]
	resources drafts [except: (delete)]
}
