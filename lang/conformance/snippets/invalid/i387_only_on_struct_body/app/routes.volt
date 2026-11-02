// spec: §request-bodies — only: narrows a changeset, not a struct body
// want: is a struct of this package
package app

scope / {
	post /login Session.Login [body: LoginForm, only: (Email)]
}
