package main

import (
	"image"
	"strconv"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

type boardsScreen struct {
	guigui.DefaultWidget

	picker      basicwidget.Select[int]
	name        field
	addBoard    basicwidget.Button
	none        basicwidget.Text
	cards       guigui.WidgetSlice[*boardCard]
	widgetTitle basicwidget.Text
	phaseLabel  basicwidget.Text
	phase       phasePicker
	tables      field
	addWidget   basicwidget.Button
}

func (b *boardsScreen) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := modelOf(context, b)
	if m == nil {
		return nil
	}
	s := m.s
	board, ok := s.BoardSelected()
	b.cards.SetLen(len(board.Widgets))
	for _, w := range []guigui.Widget{&b.picker, &b.name, &b.addBoard, &b.widgetTitle, &b.phaseLabel, &b.phase, &b.tables, &b.addWidget} {
		adder.AddWidget(w)
	}
	if len(board.Widgets) == 0 {
		adder.AddWidget(&b.none)
	}
	for _, c := range b.cards.All() {
		adder.AddWidget(c)
	}

	items := make([]basicwidget.SelectItem[int], 0, len(s.Boards.List))
	for _, x := range s.Boards.List {
		items = append(items, basicwidget.SelectItem[int]{Text: x.Name, Value: x.ID})
	}
	b.picker.SetItems(items)
	b.picker.SelectItemByValue(s.Boards.Selected)
	b.picker.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := b.picker.ItemByIndex(index); ok && item.Value != m.s.Boards.Selected {
			m.dispatch(state.BoardPick{ID: item.Value})
		}
	})
	b.name.SetField(state.FieldBoardName, "New dashboard", "Front end")
	b.name.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.BoardAdd{}) })
	b.addBoard.SetText("Add dashboard")
	b.addBoard.OnDown(func(context *guigui.Context) { m.dispatch(state.BoardAdd{}) })

	msg := "This dashboard has no widgets yet. Add one below."
	if !ok {
		msg = "No dashboard yet. Name one above to start."
	}
	caption(&b.none, msg, basicwidgetdraw.TextColor(context.ColorMode(), false))
	b.none.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	for i, c := range b.cards.All() {
		w := board.Widgets[i]
		spot := state.Spot{Phase: w.Phase, Tables: w.Tables}
		c.SetWidget(w, s.Trends[spot])
		id := w.ID
		c.OnRemoved(func(context *guigui.Context) { m.dispatch(state.WidgetRemove{ID: id}) })
	}

	heading(&b.widgetTitle, "Add a widget", 1)
	caption(&b.phaseLabel, "Phase", nil)
	if msg := s.Errors[state.FieldWidgetPhase]; msg != "" {
		caption(&b.phaseLabel, "Phase: "+msg, basicwidget.DangerTintColor())
	}
	b.phase.SetPhase(db.Phase(s.Fields[state.FieldWidgetPhase]), db.Phases)
	b.phase.OnPicked(func(context *guigui.Context, phase db.Phase) {
		m.dispatch(state.FieldEdit{Field: state.FieldWidgetPhase, Value: string(phase)})
	})
	b.tables.SetField(state.FieldWidgetTables, "Tables", "160")
	b.tables.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.WidgetAdd{}) })
	b.addWidget.SetText("Add widget")
	b.addWidget.SetType(basicwidget.ButtonTypePrimary)
	context.SetEnabled(&b.addWidget, ok)
	b.addWidget.OnDown(func(context *guigui.Context) { m.dispatch(state.WidgetAdd{}) })
	return nil
}

func (b *boardsScreen) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	top := row(u/2, nested(column(u/8, fixed(nil, u*3/4), fixed(&b.picker, u*3/2)), guigui.FixedSize(10*u)), flex(&b.name, 1), besideField(&b.addBoard, u, 6*u))
	var grid guigui.LinearLayout
	if b.cards.Len() == 0 {
		grid = column(0, flex(&b.none, 1))
	} else {
		const perRow = 3
		var rows []guigui.LinearLayoutItem
		for i := 0; i < b.cards.Len(); i += perRow {
			var cells []guigui.LinearLayoutItem
			for j := i; j < i+perRow; j++ {
				if j < b.cards.Len() {
					cells = append(cells, flex(b.cards.At(j), 1))
				} else {
					cells = append(cells, spacer())
				}
			}
			rows = append(rows, nested(row(u/2, cells...), guigui.FlexibleSize(1)))
		}
		grid = column(u/2, rows...)
	}
	add := row(u/2, nested(column(u/8, fixed(&b.phaseLabel, u*3/4), fixed(&b.phase, u*3/2)), guigui.FixedSize(14*u)), flex(&b.tables, 1), besideField(&b.addWidget, u, 5*u))
	padded(column(u/2,
		nested(top, guigui.Size{}),
		nested(grid, guigui.FlexibleSize(1)),
		fixed(&b.widgetTitle, u),
		nested(add, guigui.Size{}),
	), u/2).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

var eventCardRemoved = guigui.GenerateEventKey()

type boardCard struct {
	guigui.DefaultWidget

	title  basicwidget.Text
	latest basicwidget.Text
	chart  chartView
	remove basicwidget.Button

	widget db.Widget
	points []state.TrendPoint
}

func (c *boardCard) SetWidget(w db.Widget, points []state.TrendPoint) {
	c.widget, c.points = w, points
}

func (c *boardCard) OnRemoved(fn func(context *guigui.Context)) {
	guigui.SetEventHandler(c, eventCardRemoved, fn)
}

func (c *boardCard) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	for _, w := range []guigui.Widget{&c.title, &c.latest, &c.chart, &c.remove} {
		adder.AddWidget(w)
	}
	spot := state.Spot{Phase: c.widget.Phase, Tables: c.widget.Tables}
	heading(&c.title, string(spot.Phase)+" at "+strconv.Itoa(spot.Tables)+" tables", 1)
	latest := "no runs measured this size"
	if n := len(c.points); n > 0 {
		latest = "run " + strconv.Itoa(c.points[n-1].RunID) + ": " + units.Millis(c.points[n-1].WallMS).String() + ", " + strconv.Itoa(n) + " runs"
	}
	caption(&c.latest, latest, basicwidgetdraw.TextColor(context.ColorMode(), false))
	c.chart.SetChart(state.TrendChart(spot, c.points), true)
	c.remove.SetText("Remove")
	c.remove.OnDown(func(context *guigui.Context) { guigui.DispatchEvent(c, eventCardRemoved) })
	return nil
}

func (c *boardCard) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	padded(column(u/4,
		nested(row(u/2, flex(&c.title, 1), fixed(&c.remove, 4*u)), guigui.FixedSize(u*3/2)),
		fixed(&c.latest, u),
		flex(&c.chart, 1),
	), u/2).LayoutWidgets(context, b, layouter)
}

func (c *boardCard) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	mode := context.ColorMode()
	radius := basicwidget.UnitSize(context) / 3
	basicwidgetdraw.DrawRoundedRect(context, dst, widgetBounds.Bounds(), basicwidgetdraw.BackgroundSecondaryColor(mode), radius)
	c1, c2 := basicwidgetdraw.BorderColors(mode, basicwidgetdraw.RoundedRectBorderTypeRegular)
	basicwidgetdraw.DrawRoundedRectBorder(context, dst, widgetBounds.Bounds(), c1, c2, radius, float32(context.Scale()), basicwidgetdraw.RoundedRectBorderTypeRegular)
}

func (c *boardCard) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	return image.Pt(10*u, 8*u)
}
