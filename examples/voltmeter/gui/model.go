package main

import (
	"context"

	"github.com/guigui-gui/guigui"

	"github.com/Piechutowski/volt/examples/voltmeter/gui/remote"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

var envKeyModel = guigui.GenerateEnvKey()

type model struct {
	s        state.State
	r        *remote.Remote
	ctx      context.Context
	results  chan state.Intent
	events   chan state.Intent
	inflight int
	rev      int
	probe    *probe
}

func modelNew(ctx context.Context, r *remote.Remote) *model {
	return &model{r: r, ctx: ctx, results: make(chan state.Intent, 64), events: make(chan state.Intent, 64)}
}

func (m *model) dispatch(in state.Intent) {
	next, cmds := state.Update(m.s, in)
	m.s = next
	m.rev++
	session := next.Session
	for _, c := range cmds {
		m.inflight++
		go func() { m.results <- state.Answer{Session: session, Result: m.r.Perform(m.ctx, c)} }()
	}
}

func (m *model) drain() {
	for {
		select {
		case in := <-m.results:
			m.inflight--
			m.dispatch(in)
		case in := <-m.events:
			m.dispatch(in)
		default:
			return
		}
	}
}

func modelOf(context *guigui.Context, w guigui.Widget) *model {
	v, ok := context.Env(w, envKeyModel)
	if !ok {
		return nil
	}
	return v.(*model)
}
