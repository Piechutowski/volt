// spec: §route-paths, a bare path parameter is a string and int is the one type written: (string) is an error naming the bare form, reported once where it is written, a Scope prefix included
// want: write :slug, not :slug(string)
package app

Scope /blogs/:blog(string) {
	get /tags/:slug(string)  Tags.Find
	get /posts/:slug(String) Posts.Read
	get /pages/:num(int)     Pages.Page
}
