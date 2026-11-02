// spec: §event-routes — the runtime provides exactly one handler, volt.Events
// want: the runtime provides no handler volt.Stream
package app

scope / {
	get /events volt.Stream
}
