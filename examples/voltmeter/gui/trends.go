package main

import (
	"strconv"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

type trendsScreen struct {
	guigui.DefaultWidget

	phase      phasePicker
	sizesLabel basicwidget.Text
	sizes      basicwidget.SegmentedControl[int]
	chart      chartView
	table      basicwidget.Table[int]

	rows  []basicwidget.TableRow[int]
	lay   layouts
	spots spots
}

func (t *trendsScreen) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	for _, w := range []guigui.Widget{&t.phase, &t.sizesLabel, &t.sizes, &t.chart, &t.table} {
		adder.AddWidget(w)
	}
	m := modelOf(context, t)
	if m == nil {
		return nil
	}
	s := m.s
	t.phase.PhaseSet(s.Trend.Phase, db.Phases)
	t.phase.NameSet("trend_phase")
	t.phase.OnPicked(func(context *guigui.Context, phase db.Phase) {
		m.dispatch(state.TrendPick{Spot: state.Spot{Phase: phase, Tables: m.s.Trend.Tables}})
	})
	caption(&t.sizesLabel, "Tables", nil)
	t.sizesLabel.SetHorizontalAlign(basicwidget.HorizontalAlignRight)
	var items []basicwidget.SegmentedControlItem[int]
	for _, n := range s.Sizes() {
		items = append(items, basicwidget.SegmentedControlItem[int]{Text: strconv.Itoa(n), Value: n})
	}
	t.sizes.SetItems(items)
	t.sizes.SelectItemByValue(s.Trend.Tables)
	t.sizes.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := t.sizes.ItemByIndex(index); ok && item.Value != m.s.Trend.Tables {
			m.dispatch(state.TrendPick{Spot: state.Spot{Phase: m.s.Trend.Phase, Tables: item.Value}})
		}
	})
	points := s.Trends[s.Trend]
	t.chart.ChartSet(state.TrendChart(s.Trend, points), false)
	t.table.SetColumns([]basicwidget.TableColumn{columnOf("Run", 1, true), columnOf("Wall time", 2, true), columnOf("Change on the run before", 3, true)})
	t.rows = t.rows[:0]
	for i := len(points) - 1; i >= 0; i-- {
		p := points[i]
		change := ""
		if i > 0 && points[i-1].WallMS > 0 {
			change = strconv.FormatFloat(100*(float64(p.WallMS)/float64(points[i-1].WallMS)-1), 'f', 1, 64) + " %"
		}
		t.rows = append(t.rows, basicwidget.TableRow[int]{Value: p.RunID, Cells: []basicwidget.TableCell{
			cellRight(strconv.Itoa(p.RunID)), cellRight(units.Millis(p.WallMS).String()), cellRight(change),
		}})
	}
	t.table.SetItems(t.rows)
	t.table.SelectItemByIndex(-1)
	t.table.OnItemSelected(func(context *guigui.Context, index int) {
		if row, ok := t.table.ItemByIndex(index); ok {
			m.dispatch(state.RunOpen{ID: row.Value})
		}
	})
	return nil
}

func (t *trendsScreen) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if m := modelOf(context, t); m != nil && m.probe != nil {
		for i, row := range t.rows {
			m.probe.place(probeRow("trends", row.Value), t.table.ItemBounds(i))
		}
		var sizes []string
		for _, n := range m.s.Sizes() {
			sizes = append(sizes, strconv.Itoa(n))
		}
		t.spots.segmentsPlaced(m, "trend_sizes", sizes)
	}
	return nil
}

func (t *trendsScreen) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	a := t.lay.reset()
	u := basicwidget.UnitSize(context)
	bar := a.row(u/2, fixed(&t.phase, 14*u), spacer(), fixed(&t.sizesLabel, 3*u), natural(&t.sizes))
	recorded := t.spots.recorded(modelOf(context, t), layouter, func(name func(guigui.Widget, string)) {
		name(&t.sizes, "trend_sizes")
	})
	padded(a.column(u/2,
		a.nested(bar, guigui.FixedSize(u*3/2)),
		flex(&t.chart, 3),
		flex(&t.table, 2),
	), u/2).LayoutWidgets(context, widgetBounds.Bounds(), recorded)
}

type thesisScreen struct {
	guigui.DefaultWidget

	fixture basicwidget.SegmentedControl[string]
	chart   chartView
	table   basicwidget.Table[string]

	rows     []basicwidget.TableRow[string]
	subjects guigui.WidgetSlice[*basicwidget.Text]
	lay      layouts
	spots    spots
}

func (t *thesisScreen) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	for _, w := range []guigui.Widget{&t.fixture, &t.chart, &t.table} {
		adder.AddWidget(w)
	}
	m := modelOf(context, t)
	if m == nil {
		return nil
	}
	s := m.s
	var items []basicwidget.SegmentedControlItem[string]
	for _, f := range state.Fixtures {
		items = append(items, basicwidget.SegmentedControlItem[string]{Text: f, Value: f})
	}
	t.fixture.SetItems(items)
	t.fixture.SelectItemByValue(s.Thesis.Fixture)
	t.fixture.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := t.fixture.ItemByIndex(index); ok && item.Value != m.s.Thesis.Fixture {
			m.dispatch(state.ThesisPick{Fixture: item.Value})
		}
	})
	t.chart.ChartSet(state.ThesisChart(s.Thesis.Rows), false)
	t.table.SetColumns([]basicwidget.TableColumn{
		columnOf("Commit", 2, false), columnOf("Subject", 8, false), columnOf("Volt", 2, true), columnOf("Hand Go", 2, true), columnOf("Generated Go", 2, true), columnOf("Generated per Volt line", 3, true),
	})
	t.rows = t.rows[:0]
	t.subjects.SetLen(len(s.Thesis.Rows))
	for i := len(s.Thesis.Rows) - 1; i >= 0; i-- {
		r := s.Thesis.Rows[i]
		subject := t.subjects.At(i)
		caption(subject, r.CommitSubject, nil)
		subject.SetEllipsisString("...")
		ratio := ""
		if r.VoltLines > 0 {
			ratio = strconv.FormatFloat(float64(r.GenLines)/float64(r.VoltLines), 'f', 1, 64)
		}
		t.rows = append(t.rows, basicwidget.TableRow[string]{Value: r.Sha, Cells: []basicwidget.TableCell{
			cell(units.Commit(r.Sha).Short()), {Content: subject}, cellRight(strconv.Itoa(r.VoltLines)), cellRight(strconv.Itoa(r.HandLines)), cellRight(strconv.Itoa(r.GenLines)), cellRight(ratio),
		}})
	}
	t.table.SetItems(t.rows)
	return nil
}

func (t *thesisScreen) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	a := t.lay.reset()
	u := basicwidget.UnitSize(context)
	padded(a.column(u/2,
		a.nested(a.row(u/2, fixed(&t.fixture, 14*u), spacer()), guigui.FixedSize(u*3/2)),
		flex(&t.chart, 3),
		flex(&t.table, 2),
	), u/2).LayoutWidgets(context, widgetBounds.Bounds(), t.spots.recorded(modelOf(context, t), layouter, func(name func(guigui.Widget, string)) {
		name(&t.fixture, "thesis_fixture")
	}))
}

func (t *thesisScreen) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	t.spots.segmentsPlaced(modelOf(context, t), "thesis_fixture", state.Fixtures)
	return nil
}
