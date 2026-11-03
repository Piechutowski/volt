// Command gui is Voltmeter's desktop client, built with guigui: it
// signs a member in and shows the runs, one run's chart, the trends,
// the thesis line counts, the budgets with their breaches arriving live,
// and the member's dashboards, all through the generated client.
//
// The window holds one state.State. Widgets render it and send intents;
// state.Update answers each with the next State and the commands to
// perform, which remote.Remote performs on goroutines of their own; the
// answers, and the events a goroutine started here reads from the
// server's stream, are applied once per tick (spec §values-and-effects,
// §event-routes).
//
// With -shot or -shots it runs a script instead of waiting for a
// person: it signs in, opens the newest run, saves the window as PNG
// and exits, which is how the window smoke test and the screenshots
// are made. With -probe it signs in and stays open, writing its state
// and where its rows, fields and sidebar entries are drawn to stdout,
// so a test can drive the widgets with real pointer and key input.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	"os"
	"time"

	"github.com/guigui-gui/guigui"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Piechutowski/volt/examples/voltmeter/gui/remote"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

func main() {
	server := flag.String("server", "http://localhost:8737", "the Voltmeter server")
	export := flag.String("export", os.TempDir(), "the directory run CSV exports are saved to")
	shot := flag.String("shot", "", "scripted: sign in, open the newest run, save the window to this PNG and exit")
	shots := flag.String("shots", "", "scripted: save every screen as a PNG into this directory and exit")
	email := flag.String("email", "", "scripted: the member to sign in as; the first the server lists by default")
	timeout := flag.Duration("timeout", time.Minute, "scripted: give up after this long")
	probed := flag.Bool("probe", false, "scripted: sign in, then stay open for synthetic input, writing the state and where its targets are drawn to stdout as JSON lines")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := remote.New(*server, *export)
	m := modelNew(ctx, r)
	go func() {
		for ev := range r.Events(ctx) {
			if in, ok := state.ChangedOf(ev); ok {
				m.events <- in
			}
		}
	}()
	m.dispatch(state.Started{})

	w := &root{m: m}
	switch {
	case *shot != "":
		w.script = &script{steps: scriptSmoke(*email, *shot)}
	case *shots != "":
		w.script = &script{steps: scriptTour(*email, *shots)}
	case *probed:
		m.probe = &probe{out: os.Stdout}
		w.script = &script{steps: []step{membersLoaded(), signIn(*email)}, stay: true}
	}
	if w.script != nil {
		w.script.deadline = int(timeout.Seconds() * float64(ebiten.DefaultTPS))
	}
	op := &guigui.RunOptions{
		Title:         "Voltmeter",
		WindowSize:    image.Pt(1280, 800),
		WindowMinSize: image.Pt(960, 640),
	}
	err := guigui.RunWithCustomFunc(w, op, func(game ebiten.Game, options *ebiten.RunGameOptions) error {
		return ebiten.RunGameWithOptions(&shooter{Game: game, script: w.script}, options)
	})
	if err == nil && w.script != nil {
		err = w.script.err
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "voltmeter gui:", err)
		os.Exit(1)
	}
}
