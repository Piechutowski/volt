// spec: §event-routes — an event route serves the runtime's broker; PathEvents and the client's Events come with it
package app

scope / {
	get /events volt.Events
}
