// spec: §pipelines, §pipelines
package app

pipeline api {
	use volt.RequestID
	use volt.Logger
	use BearerAuth
	use Session
}
