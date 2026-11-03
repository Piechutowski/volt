package main

import (
	"image"
	"image/color"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

func fixed(w guigui.Widget, px int) guigui.LinearLayoutItem {
	return guigui.LinearLayoutItem{Widget: w, Size: guigui.FixedSize(px)}
}

func flex(w guigui.Widget, weight int) guigui.LinearLayoutItem {
	return guigui.LinearLayoutItem{Widget: w, Size: guigui.FlexibleSize(weight)}
}

func natural(w guigui.Widget) guigui.LinearLayoutItem {
	return guigui.LinearLayoutItem{Widget: w}
}

func spacer() guigui.LinearLayoutItem {
	return guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)}
}

type layouts struct {
	items [][]guigui.LinearLayoutItem
	boxes []*guigui.LinearLayout
	n, m  int
}

func (a *layouts) reset() *layouts {
	a.n, a.m = 0, 0
	return a
}

func (a *layouts) list(items ...guigui.LinearLayoutItem) []guigui.LinearLayoutItem {
	if a.n == len(a.items) {
		a.items = append(a.items, nil)
	}
	a.items[a.n] = append(a.items[a.n][:0], items...)
	a.n++
	return a.items[a.n-1]
}

func (a *layouts) keep(items []guigui.LinearLayoutItem) []guigui.LinearLayoutItem {
	a.items[a.n-1] = items
	return items
}

func (a *layouts) nested(l guigui.LinearLayout, size guigui.Size) guigui.LinearLayoutItem {
	if a.m == len(a.boxes) {
		a.boxes = append(a.boxes, new(guigui.LinearLayout))
	}
	box := a.boxes[a.m]
	a.m++
	*box = l
	return guigui.LinearLayoutItem{Layout: box, Size: size}
}

func (a *layouts) column(gap int, items ...guigui.LinearLayoutItem) guigui.LinearLayout {
	return guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Gap: gap, Items: a.list(items...)}
}

func (a *layouts) row(gap int, items ...guigui.LinearLayoutItem) guigui.LinearLayout {
	return guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Gap: gap, Items: a.list(items...)}
}

func (a *layouts) besideField(w guigui.Widget, u, width int) guigui.LinearLayoutItem {
	return a.nested(a.column(u/8, fixed(nil, u*3/4), natural(w)), guigui.FixedSize(width))
}

func padded(l guigui.LinearLayout, p int) guigui.LinearLayout {
	l.Padding = guigui.Padding{Start: p, Top: p, End: p, Bottom: p}
	return l
}

func heading(t *basicwidget.Text, value string, scale float64) {
	var style basicwidget.TextStyle
	style.SetBold(true)
	t.SetBaseStyle(&style)
	t.SetScale(scale)
	t.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.SetValue(value)
}

func caption(t *basicwidget.Text, value string, clr color.Color) {
	var style basicwidget.TextStyle
	if clr != nil {
		style.SetColor(clr)
	}
	t.SetBaseStyle(&style)
	t.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.SetValue(value)
}

func cellRight(text string) basicwidget.TableCell {
	c := basicwidget.TableCell{Text: text}
	c.TextStyle.HorizontalAlign = basicwidget.HorizontalAlignRight
	c.TextStyle.Style.SetTabular(true)
	return c
}

func cell(text string) basicwidget.TableCell { return basicwidget.TableCell{Text: text} }

func columnOf(header string, weight int, right bool) basicwidget.TableColumn {
	c := basicwidget.TableColumn{HeaderText: header, Width: guigui.FlexibleSize(weight)}
	if right {
		c.HeaderTextHorizontalAlign = basicwidget.HorizontalAlignRight
	}
	return c
}

var eventFieldSubmitted = guigui.GenerateEventKey()

type field struct {
	guigui.DefaultWidget

	label basicwidget.Text
	input basicwidget.TextInput

	name        state.Field
	labelText   string
	placeholder string
	shown       string
	pushed      bool
	lay         layouts
}

func (f *field) FieldSet(name state.Field, label, placeholder string) {
	f.name, f.labelText, f.placeholder = name, label, placeholder
}

// OnSubmitted is called when Enter is pressed in the field. A commit
// alone does not submit, since setting the value from State commits it
// too. The input shows what the member types; State's value is pushed
// into it, focused or not, only when State changed it, such as a form
// cleared after a save, so the input never shows one value while State
// holds another.
func (f *field) OnSubmitted(fn func(context *guigui.Context)) {
	guigui.SetEventHandler(f, eventFieldSubmitted, fn)
}

func (f *field) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&f.label)
	adder.AddWidget(&f.input)
	m := modelOf(context, f)
	if m == nil {
		return nil
	}
	caption(&f.label, f.labelText, nil)
	f.input.SetPlaceholder(f.placeholder)
	if want := m.s.Fields[f.name]; !f.pushed || want != f.shown {
		f.input.ForceSetValue(want)
		f.shown, f.pushed = want, true
	}
	msg := m.s.Errors[f.name]
	f.input.SetError(msg != "")
	f.input.SetSupportText(msg)
	name := f.name
	f.input.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		f.shown = text
		if m.s.Fields[name] != text {
			m.dispatch(state.FieldEdit{Field: name, Value: text})
		}
	})
	f.input.OnHandleButtonInput(func(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if text := f.input.Value(); m.s.Fields[name] != text {
				f.shown = text
				m.dispatch(state.FieldEdit{Field: name, Value: text})
			}
			guigui.DispatchEvent(f, eventFieldSubmitted)
			return guigui.HandleInputByWidget(&f.input)
		}
		return guigui.HandleInputResult{}
	})
	return nil
}

func (f *field) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if m := modelOf(context, f); m != nil && m.probe != nil {
		u := basicwidget.UnitSize(context)
		b := widgetBounds.Bounds()
		b.Min.Y += u*3/4 + u/8
		m.probe.place("field:"+string(f.name), b)
	}
	return nil
}

func (f *field) layout(context *guigui.Context) guigui.LinearLayout {
	a := f.lay.reset()
	u := basicwidget.UnitSize(context)
	return a.column(u/8, fixed(&f.label, u*3/4), natural(&f.input))
}

func (f *field) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	f.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (f *field) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return f.layout(context).Measure(context, constraints)
}

var eventPhasePicked = guigui.GenerateEventKey()

type phasePicker struct {
	guigui.DefaultWidget

	control basicwidget.SegmentedControl[db.Phase]

	phase  db.Phase
	phases []db.Phase
	name   string
	bounds image.Rectangle
}

func (p *phasePicker) NameSet(name string) { p.name = name }

func (p *phasePicker) PhaseSet(phase db.Phase, phases []db.Phase) {
	p.phase, p.phases = phase, phases
}

func (p *phasePicker) OnPicked(fn func(context *guigui.Context, phase db.Phase)) {
	guigui.SetEventHandler(p, eventPhasePicked, fn)
}

func (p *phasePicker) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&p.control)
	items := make([]basicwidget.SegmentedControlItem[db.Phase], 0, len(p.phases))
	for _, ph := range p.phases {
		items = append(items, basicwidget.SegmentedControlItem[db.Phase]{Text: string(ph), Value: ph})
	}
	p.control.SetItems(items)
	p.control.SelectItemByValue(p.phase)
	p.control.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := p.control.ItemByIndex(index)
		if ok && item.Value != p.phase {
			guigui.DispatchEvent(p, eventPhasePicked, item.Value)
		}
	})
	return nil
}

func (p *phasePicker) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	p.bounds = widgetBounds.Bounds()
	layouter.LayoutWidget(&p.control, p.bounds)
}

func (p *phasePicker) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	m := modelOf(context, p)
	if m == nil || m.probe == nil || p.name == "" || len(p.phases) == 0 {
		return nil
	}
	w := p.bounds.Dx() / len(p.phases)
	for i, ph := range p.phases {
		r := p.bounds
		r.Min.X += i * w
		r.Max.X = r.Min.X + w
		m.probe.place(p.name+":"+string(ph), r)
	}
	return nil
}

func (p *phasePicker) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return p.control.Measure(context, constraints)
}
