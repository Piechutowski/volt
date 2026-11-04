// spec: §generated-client, a client method does not take the name of the embedded volt.Client's Header field
// want: client method "Header" would take the name of the embedded volt.Client's field Header
package app

scope / {
	get /metrics/header Metrics.Show [name: header]
}
