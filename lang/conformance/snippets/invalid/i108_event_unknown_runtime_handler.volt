// spec: §V4.11 — the runtime provides exactly one handler, volt.Events
// want: the runtime provides no handler volt.Stream
package app

Scope / {
	get /events volt.Stream
}
