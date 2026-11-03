package main

import (
	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

type root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	sidebar    basicwidget.List[state.Screen]
	title      basicwidget.Text
	notice     basicwidget.Text
	live       basicwidget.Text
	who        basicwidget.Text
	signOut    basicwidget.Button

	signIn  signInScreen
	runs    runsScreen
	run     runScreen
	trends  trendsScreen
	thesis  thesisScreen
	budgets budgetsScreen
	boards  boardsScreen

	m      *model
	script *script
}

func (r *root) Env(context *guigui.Context, key guigui.EnvKey, source *guigui.EnvSource) (any, bool) {
	if key == envKeyModel {
		return r.m, true
	}
	return nil, false
}

func (r *root) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteInt(r.m.rev)
}

func (r *root) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	r.m.drain()
	if r.script != nil {
		return r.script.tick(r.m)
	}
	return nil
}

func (r *root) screen() guigui.Widget {
	switch r.m.s.Screen {
	case state.ScreenRuns:
		return &r.runs
	case state.ScreenRun:
		return &r.run
	case state.ScreenTrends:
		return &r.trends
	case state.ScreenThesis:
		return &r.thesis
	case state.ScreenBudgets:
		return &r.budgets
	case state.ScreenDashboards:
		return &r.boards
	}
	return &r.signIn
}

func (r *root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	s := r.m.s
	signedIn := s.User.ID != 0
	adder.AddWidget(&r.background)
	if signedIn {
		for _, w := range []guigui.Widget{&r.sidebar, &r.title, &r.notice, &r.live, &r.who, &r.signOut} {
			adder.AddWidget(w)
		}
	}
	adder.AddWidget(r.screen())

	items := make([]basicwidget.ListItem[state.Screen], 0, len(state.Screens))
	for _, sc := range state.Screens {
		items = append(items, basicwidget.ListItem[state.Screen]{Text: sc.String(), Value: sc})
	}
	r.sidebar.SetStyle(basicwidget.ListStyleSidebar)
	r.sidebar.SetItems(items)
	r.sidebar.SetItemHeight(basicwidget.UnitSize(context) * 3 / 2)
	r.sidebar.SelectItemByValue(s.Screen)
	r.sidebar.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := r.sidebar.ItemByIndex(index); ok && item.Value != r.m.s.Screen {
			r.m.dispatch(state.ScreenShow{Screen: item.Value})
		}
	})

	heading(&r.title, s.Screen.String(), 1.4)
	muted := basicwidgetdraw.TextColor(context.ColorMode(), false)
	if s.Notice.Failed {
		caption(&r.notice, s.Notice.Text, basicwidget.DangerTintColor())
	} else {
		caption(&r.notice, s.Notice.Text, muted)
	}
	live := "Live: waiting for changes"
	if s.Live != "" {
		live = "Live: " + s.Live
	}
	caption(&r.live, live, muted)
	caption(&r.who, s.User.Name+"  "+s.User.Email, nil)
	r.who.SetHorizontalAlign(basicwidget.HorizontalAlignRight)
	r.signOut.SetText("Sign out")
	r.signOut.OnDown(func(context *guigui.Context) { r.m.dispatch(state.SignOut{}) })
	return nil
}

func (r *root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	layouter.LayoutWidget(&r.background, b)
	if r.m.s.User.ID == 0 {
		layouter.LayoutWidget(r.screen(), b)
		return
	}
	u := basicwidget.UnitSize(context)
	header := padded(row(u/2,
		nested(column(0, fixed(&r.title, u*3/2), fixed(&r.notice, u)), guigui.FlexibleSize(3)),
		nested(column(0, fixed(&r.who, u*3/2), fixed(&r.live, u)), guigui.FlexibleSize(2)),
		nested(column(0, fixed(&r.signOut, u*3/2)), guigui.FixedSize(5*u)),
	), u/2)
	row(0,
		fixed(&r.sidebar, 8*u),
		nested(column(0, nested(header, guigui.FixedSize(3*u+u/2)), flex(r.screen(), 1)), guigui.FlexibleSize(1)),
	).LayoutWidgets(context, b, layouter)
}
