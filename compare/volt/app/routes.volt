package app

import (
	volt/db
)

scope / [error_handler: Errors] {
	load member CurrentMember

	post   /posts                              Posts.Create    [name: post_create, body: PostForm]
	get    /posts                              Posts.Feed      [name: feed]
	get    /posts/:id(int)                     Posts.Show      [name: post] { post db.PostGet(id) }
	patch  /posts/:id(int)                     Posts.Edit      [name: post_edit, body: PostEdit]
	post   /posts/:id(int)/publish             Posts.Publish   [name: publish]
	get    /posts/:id(int)/audit               Posts.Audit     [name: audit] {
		post  db.PostGet(id)
		trail db.AuditTrail(id)
	}
	post   /posts/:id(int)/comments            Comments.Create [name: comment_create, body: CommentForm] { post db.PostGet(id) }
	get    /posts/:id(int)/comments            Comments.List   [name: comments] { thread db.CommentThread(id) }
	delete /posts/:id(int)/comments/:cid(int)  Comments.Delete [name: comment_delete] {
		post    db.PostGet(id)
		comment db.CommentGet(cid)
	}
	get    /me/stats                           Me.Stats        [name: stats]
}
