// spec: §settings-lists, §settings, a resources declaration takes each setting at most once
// want: setting "only" may appear at most once
package app

Table posts {
	id int [pk]
}

Scope / {
	resources posts [only: (index), only: (show)]
}
