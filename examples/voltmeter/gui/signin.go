package main

import (
	"image"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

type signInScreen struct {
	guigui.DefaultWidget

	title    basicwidget.Text
	subtitle basicwidget.Text
	members  guigui.WidgetSlice[*basicwidget.Button]
	notice   basicwidget.Text
	retry    basicwidget.Button
	or       basicwidget.Text
	email    field
	submit   basicwidget.Button
	failed   bool
	lay      layouts
}

func (s *signInScreen) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := modelOf(context, s)
	if m == nil {
		return nil
	}
	s.members.SetLen(len(m.s.Members))
	adder.AddWidget(&s.title)
	adder.AddWidget(&s.subtitle)
	s.failed = m.s.Notice.Failed
	if s.failed {
		adder.AddWidget(&s.notice)
		adder.AddWidget(&s.retry)
	}
	for _, b := range s.members.All() {
		adder.AddWidget(b)
	}
	adder.AddWidget(&s.or)
	adder.AddWidget(&s.email)
	adder.AddWidget(&s.submit)

	heading(&s.title, "Voltmeter", 2)
	s.title.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	caption(&s.subtitle, "Volt measuring itself. Sign in as a member of the team.", basicwidgetdraw.TextColor(context.ColorMode(), false))
	s.subtitle.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	for i, b := range s.members.All() {
		u := m.s.Members[i]
		b.SetText(u.Name + "   " + u.Email)
		b.OnDown(func(context *guigui.Context) {
			m.dispatch(state.MemberPick{Email: u.Email})
		})
	}
	caption(&s.notice, m.s.Notice.Text, basicwidget.DangerTintColor())
	s.notice.SetMultiline(true)
	s.notice.SetWrapMode(basicwidget.WrapModeNormal)
	s.notice.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	s.retry.SetText("Try again")
	s.retry.OnDown(func(context *guigui.Context) { m.dispatch(state.MembersReload{}) })
	caption(&s.or, "or sign in by email", basicwidgetdraw.TextColor(context.ColorMode(), false))
	s.or.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	s.email.FieldSet(state.FieldEmail, "Email", "name@voltmeter.test")
	s.email.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.SignInSubmit{}) })
	s.submit.SetText("Sign in")
	s.submit.SetType(basicwidget.ButtonTypePrimary)
	s.submit.OnDown(func(context *guigui.Context) { m.dispatch(state.SignInSubmit{}) })
	return nil
}

func (s *signInScreen) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	a := s.lay.reset()
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	w := min(b.Dx()-2*u, 18*u)
	items := a.list(fixed(&s.title, 2*u), fixed(&s.subtitle, u), fixed(nil, u/2))
	if s.failed {
		items = append(items, natural(&s.notice), fixed(&s.retry, u*3/2))
	}
	for _, btn := range s.members.All() {
		items = append(items, fixed(btn, u*3/2))
	}
	items = a.keep(append(items, fixed(&s.or, u), natural(&s.email), fixed(&s.submit, u*3/2)))
	l := guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Gap: u / 2, Items: items}
	h := l.Measure(context, guigui.FixedWidthConstraints(w)).Y
	x := b.Min.X + (b.Dx()-w)/2
	y := b.Min.Y + max(u, (b.Dy()-h)/3)
	l.LayoutWidgets(context, image.Rect(x, y, x+w, y+h), layouter)
}
