// analyzers: handlerreceiver
package app

import "github.com/Piechutowski/volt"

type Home struct{}

func (Home) Index(r volt.Request) volt.Response { return volt.NoContent() }

type Users struct{ db string } //WANT handlerreceiver

func (Users) Show(r volt.Request, id int) volt.Response { return volt.NoContent() }

type Plain struct{ n int }

func (p Plain) Helper() int { return p.n }
