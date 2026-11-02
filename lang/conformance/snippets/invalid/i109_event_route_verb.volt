// spec: §event-routes — an event stream is read with get
// want: read with get
package app

scope / {
	post /events volt.Events
}
