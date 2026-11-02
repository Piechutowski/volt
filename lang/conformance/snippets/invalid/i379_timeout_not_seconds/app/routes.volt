// spec: §route-loads — timeout: takes a positive integer of seconds
// want: timeout: takes a positive integer of seconds
package app

scope / {
	get /charts Pages.Charts [timeout: "5s"]
	get /slow   Pages.Slow   [timeout: 0]
}
