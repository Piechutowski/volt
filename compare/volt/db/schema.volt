package db

table members {
	id    int    [pk]
	name  string [required]
	email string [required, unique]
}

table posts {
	id           int       [pk]
	slug         string    [required, unique]
	title        string    [required]
	body         string
	state        string    [default: "draft"]
	author_id    int       [ref: members.id]
	published_at time.Time [null]
	created_at   time.Time [default: `CURRENT_TIMESTAMP`]

	validators {
		SlugShape(slug)
		TitleLength(title)
	}
}

table post_tags {
	post_id int    [pk]
	tag     string [pk]

	validators {
		TagShape(tag)
	}
}

ref: post_tags.post_id > posts.id [delete: cascade, as: post, reverse: tags]

table comments {
	id         int       [pk]
	post_id    int
	member_id  int       [ref: members.id]
	body       string    [required]
	created_at time.Time [default: `CURRENT_TIMESTAMP`]

	validators {
		CommentLength(body)
	}
}

ref: comments.post_id > posts.id [delete: cascade, as: post, reverse: comments]

table audits {
	id        int       [pk]
	post_id   int       [ref: posts.id]
	member_id int       [ref: members.id]
	action    string
	at        time.Time
}
