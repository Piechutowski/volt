// spec: §pipelines, §pipelines
package app

Pipeline api {
	use volt.RequestID
	use volt.Logger
	use BearerAuth
	use Session
}
