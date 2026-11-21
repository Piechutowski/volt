// spec/projects-and-packages, a package's .volt files and its Go files declare one name
// want: "routes" disagrees with package app, which the directory's Go file auth.go declares
package routes

pipeline guarded {
	use BearerAuth
}
