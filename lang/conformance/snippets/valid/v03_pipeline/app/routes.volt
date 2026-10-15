// spec: §pipelines rule 1, §pipelines rule 2
package app

Pipeline api {
	use volt.RequestID
	use volt.Logger
	use BearerAuth
	use app.Session
}
