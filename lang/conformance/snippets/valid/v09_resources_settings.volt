// spec: §V5.3
package app

Table users {
	id integer [pk]
}

Table posts {
	id integer [pk]
}

Table drafts {
	id integer [pk]
}

Scope / {
	resources users  [api, param: uid]
	resources posts  [only: (index, show)]
	resources drafts [except: (delete)]
}
