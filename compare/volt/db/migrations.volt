package db

migration 0001__blog {
	create table members {
		id    int    [pk]
		name  string
		email string [unique]
	}
	create table posts {
		id           int       [pk]
		slug         string    [unique]
		title        string
		body         string
		state        string    [default: "draft"]
		author_id    int       [ref: members.id]
		published_at time.Time [null]
		created_at   time.Time [default: `CURRENT_TIMESTAMP`]
	}
	create table comments {
		id         int       [pk]
		post_id    int       [ref: posts.id, delete: cascade]
		member_id  int       [ref: members.id]
		body       string
		created_at time.Time [default: `CURRENT_TIMESTAMP`]
	}
	create table audits {
		id        int       [pk]
		post_id   int       [ref: posts.id]
		member_id int       [ref: members.id]
		action    string
		at        time.Time
	}
	exec `INSERT INTO members (id, name, email) VALUES (1, 'Ada', 'ada@blog.test'), (2, 'Grace', 'grace@blog.test'), (3, 'Ken', 'ken@blog.test')` [down: `DELETE FROM members`]
}

migration 0002__tags {
	create table post_tags {
		post_id int    [pk, ref: posts.id, delete: cascade]
		tag     string [pk]
	}
}
