// spec: §settings
package app

Table users {
	id int [pk]
}

Table posts {
	id int [pk]
}

Table drafts {
	id int [pk]
}

Scope / {
	resources users  [api, param: uid]
	resources posts  [only: (index, show)]
	resources drafts [except: (delete)]
}
