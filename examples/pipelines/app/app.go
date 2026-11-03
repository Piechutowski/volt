// Package app is the hand-written side of the routes: the plugs the two
// pipelines of routes.volt use (plugs.go) and the controllers.
package app

import "github.com/Piechutowski/volt"

// Message is what every handler renders.
type Message struct {
	Text string
}

// Pages are the routes the api pipeline alone guards.
type Pages struct{}

// Open renders a line every caller gets.
func (Pages) Open(r volt.Request) volt.Response {
	return volt.Render(Message{Text: "anyone may read " + r.Path()})
}

// Vault are the routes the vault pipeline guards as well.
type Vault struct{}

// Secret renders a line only a caller with the key gets.
func (Vault) Secret(r volt.Request) volt.Response {
	return volt.Render(Message{Text: "the key opened " + r.Path()})
}
