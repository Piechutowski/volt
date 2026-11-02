// spec: §settings-lists, §selects-over-groups, a Select takes each setting at most once
// want: setting "order" may appear at most once
package app

Table posts {
	id    int [pk]
	title string
}

Select recent for posts [order: (id desc), order: (title asc)]
