package main

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

const quietTicks = 30

type step struct {
	name   string
	intent func(state.State) (state.Intent, error)
	ready  func(state.State) bool
	shot   string
}

type script struct {
	steps    []step
	i        int
	sent     bool
	quiet    int
	ticks    int
	deadline int
	capture  string
	stay     bool
	over     bool
	err      error
}

func signIn(email string) step {
	return step{
		name: "sign in",
		intent: func(s state.State) (state.Intent, error) {
			if email != "" {
				return state.MemberPick{Email: email}, nil
			}
			if len(s.Members) == 0 {
				return nil, errors.New("the server lists no member to sign in as")
			}
			return state.MemberPick{Email: s.Members[0].Email}, nil
		},
		ready: func(s state.State) bool { return s.User.ID != 0 && s.Runs.Loaded },
	}
}

func membersLoaded() step {
	return step{name: "list the team", ready: func(s state.State) bool { return len(s.Members) > 0 }}
}

func newestOpen() step {
	return step{
		name: "open the newest run",
		intent: func(s state.State) (state.Intent, error) {
			if len(s.Runs.Page.Runs) == 0 {
				return nil, errors.New("the server holds no run to open")
			}
			return state.RunOpen{ID: s.Runs.Page.Runs[0].ID}, nil
		},
		ready: func(s state.State) bool { return s.Run.Loaded },
	}
}

func screenOpen(sc state.Screen, ready func(state.State) bool) step {
	return step{
		name:   "show " + sc.String(),
		intent: func(state.State) (state.Intent, error) { return state.ScreenShow{Screen: sc}, nil },
		ready:  ready,
	}
}

func scriptSmoke(email, path string) []step {
	open := newestOpen()
	open.shot = path
	return []step{membersLoaded(), signIn(email), open}
}

func scriptTour(email, dir string) []step {
	at := func(name string) string { return filepath.Join(dir, name+".png") }
	members := membersLoaded()
	members.shot = at("signin")
	in := signIn(email)
	in.shot = at("runs")
	open := newestOpen()
	open.shot = at("run")
	trends := screenOpen(state.ScreenTrends, func(s state.State) bool { _, ok := s.Trends[s.Trend]; return ok })
	trends.shot = at("trends")
	thesis := screenOpen(state.ScreenThesis, func(s state.State) bool { return s.Thesis.Loaded })
	thesis.shot = at("thesis")
	budgets := screenOpen(state.ScreenBudgets, func(s state.State) bool { return s.Budgets.Loaded })
	budgets.shot = at("budgets")
	boards := screenOpen(state.ScreenDashboards, func(s state.State) bool { return s.Boards.Loaded })
	boards.shot = at("dashboards")
	return []step{members, in, open, trends, thesis, budgets, boards}
}

func (sc *script) tick(m *model) error {
	sc.ticks++
	if sc.over || sc.capture != "" {
		return nil
	}
	if sc.ticks > sc.deadline && !(sc.stay && sc.i == len(sc.steps)) {
		name := "the end"
		if sc.i < len(sc.steps) {
			name = sc.steps[sc.i].name
		}
		sc.fail(fmt.Errorf("timed out at %q (notice %q)", name, m.s.Notice.Text))
		return nil
	}
	if sc.i == len(sc.steps) {
		sc.over = !sc.stay
		return nil
	}
	st := sc.steps[sc.i]
	if !sc.sent {
		sc.sent = true
		if st.intent != nil {
			in, err := st.intent(m.s)
			if err != nil {
				sc.fail(fmt.Errorf("%s: %w", st.name, err))
				return nil
			}
			m.dispatch(in)
		}
		sc.quiet = 0
		return nil
	}
	if m.inflight > 0 || len(m.results) > 0 || len(m.events) > 0 || (st.ready != nil && !st.ready(m.s)) {
		sc.quiet = 0
		return nil
	}
	if sc.quiet++; sc.quiet < quietTicks {
		return nil
	}
	if st.shot != "" {
		sc.capture = st.shot
		return nil
	}
	sc.next()
	return nil
}

func (sc *script) next() {
	sc.i++
	sc.sent = false
	sc.quiet = 0
}

func (sc *script) fail(err error) {
	sc.err = err
	sc.over = true
}

func (sc *script) captured(err error) {
	sc.capture = ""
	if err != nil {
		sc.fail(err)
		return
	}
	sc.next()
}

type shooter struct {
	ebiten.Game
	script *script
}

func (g *shooter) Update() error {
	if err := g.Game.Update(); err != nil {
		return err
	}
	if g.script != nil && g.script.over {
		return ebiten.Termination
	}
	return nil
}

func (g *shooter) Draw(screen *ebiten.Image) {
	g.Game.Draw(screen)
	if g.script != nil && g.script.capture != "" {
		g.script.captured(pngSave(screen, g.script.capture))
	}
}

func (g *shooter) LayoutF(outsideWidth, outsideHeight float64) (float64, float64) {
	if l, ok := g.Game.(ebiten.LayoutFer); ok {
		return l.LayoutF(outsideWidth, outsideHeight)
	}
	w, h := g.Game.Layout(int(outsideWidth), int(outsideHeight))
	return float64(w), float64(h)
}

func pngSave(screen *ebiten.Image, path string) error {
	b := screen.Bounds()
	img := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	screen.ReadPixels(img.Pix)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
