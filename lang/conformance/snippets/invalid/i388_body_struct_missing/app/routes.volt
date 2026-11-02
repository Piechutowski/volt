// spec: §request-bodies — a struct body names a type declared in the package's Go files
// want: no type Signup in package app
package app

Scope / {
	post /signup Session.Signup [body: Signup]
}
