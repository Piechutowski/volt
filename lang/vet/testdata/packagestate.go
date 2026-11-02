// analyzers: packagestate
package app

import "github.com/Piechutowski/volt"

var statuses = map[string]int{"ok": 200}

var counter int //WANT packagestate

var started = volt.Render(1) //WANT packagestate

func init() { //WANT packagestate
	counter = 1 //WANT packagestate
}

type Home struct{}

func (Home) Index(r volt.Request) volt.Response {
	counter++ //WANT packagestate
	local := 0
	local++
	return volt.Render(statuses["ok"] + local)
}
