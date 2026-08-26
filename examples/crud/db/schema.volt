package db

// The schema is the source of truth for the key type: `id integer [pk]`
// is what makes every generated handler take an int32 id (§V5.2).
Table posts {
	id    integer [pk, increment]
	title text    [not null]
	body  text    [not null, default: '']
}

Table comments {
	id      integer [pk, increment]
	post_id integer [not null]
	author  text    [not null]
	body    text    [not null, default: '']
}
