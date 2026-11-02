// spec: §routes — any registers without a method
package app

scope / {
	any /webhook Hooks.Receive
}
