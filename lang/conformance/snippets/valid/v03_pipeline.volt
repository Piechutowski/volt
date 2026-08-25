// spec: §V3.1, §V3.2
package app

Pipeline api {
	use volt.RequestID
	use volt.Logger
	use BearerAuth
	use app.Session
}
