// spec: §request-bodies — routes sharing an action share its body type
// want: reached by routes with different signatures
package app

Scope / {
	post /login  Session.Login [body: LoginForm]
	post /login2 Session.Login [name: login2]
}
