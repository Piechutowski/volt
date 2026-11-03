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

func nested(l guigui.LinearLayout, size guigui.Size) guigui.LinearLayoutItem {
	return guigui.LinearLayoutItem{Layout: l, Size: size}
}

func column(gap int, items ...guigui.LinearLayoutItem) guigui.LinearLayout {
	return guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Gap: gap, Items: items}
}

func row(gap int, items ...guigui.LinearLayoutItem) guigui.LinearLayout {
	return guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Gap: gap, Items: items}
}

func besideField(w guigui.Widget, u, width int) guigui.LinearLayoutItem {
	return nested(column(u/8, fixed(nil, u*3/4), natural(w)), guigui.FixedSize(width))
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
}

func (f *field) SetField(name state.Field, label, placeholder string) {
	f.name, f.labelText, f.placeholder = name, label, placeholder
}

// OnSubmitted is called when Enter is pressed in the field. A commit
// alone does not submit, since setting the value from State commits it
// too.
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
	f.input.SetValue(m.s.Fields[f.name])
	msg := m.s.Errors[f.name]
	f.input.SetError(msg != "")
	f.input.SetSupportText(msg)
	name := f.name
	f.input.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if m.s.Fields[name] != text {
			m.dispatch(state.FieldEdit{Field: name, Value: text})
		}
	})
	f.input.OnHandleButtonInput(func(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
		if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			if text := f.input.Value(); m.s.Fields[name] != text {
				m.dispatch(state.FieldEdit{Field: name, Value: text})
			}
			guigui.DispatchEvent(f, eventFieldSubmitted)
			return guigui.HandleInputByWidget(&f.input)
		}
		return guigui.HandleInputResult{}
	})
	return nil
}

func (f *field) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	return column(u/8, fixed(&f.label, u*3/4), natural(&f.input))
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
}

func (p *phasePicker) SetPhase(phase db.Phase, phases []db.Phase) {
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
	layouter.LayoutWidget(&p.control, widgetBounds.Bounds())
}

func (p *phasePicker) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return p.control.Measure(context, constraints)
}
