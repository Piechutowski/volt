// spec: §settings-lists, §settings, a resources declaration takes each setting at most once
// want: setting "only" may appear at most once
package app

table posts {
	id int [pk]
}

scope / {
	resources posts [only: (index), only: (show)]
}
