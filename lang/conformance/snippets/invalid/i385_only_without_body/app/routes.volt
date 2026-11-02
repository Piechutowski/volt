// spec: §request-bodies — only: narrows a body: changeset, so it needs one
// want: the route has no body
package app

scope / {
	patch /settings/:id(int) Settings.Save [only: (refresh)]
}
