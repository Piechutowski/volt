package main

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

type runScreen struct {
	guigui.DefaultWidget

	none    basicwidget.Text
	title   basicwidget.Text
	tag     field
	note    field
	save    basicwidget.Button
	export  basicwidget.Button
	saved   basicwidget.Text
	chart   chartView
	table   basicwidget.Table[int]
	footer  basicwidget.Text
	latency basicwidget.Text

	rows []basicwidget.TableRow[int]
	lay  layouts
}

func (r *runScreen) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	m := modelOf(context, r)
	if m == nil {
		return nil
	}
	s := m.s
	if s.Run.ID == 0 || !s.Run.Loaded {
		adder.AddWidget(&r.none)
		msg := "Open a run from the Runs screen."
		if s.Run.ID != 0 {
			msg = "Loading run " + strconv.Itoa(s.Run.ID)
		}
		caption(&r.none, msg, basicwidgetdraw.TextColor(context.ColorMode(), false))
		r.none.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		return nil
	}
	for _, w := range []guigui.Widget{&r.title, &r.tag, &r.note, &r.save, &r.export, &r.saved, &r.chart, &r.table, &r.footer, &r.latency} {
		adder.AddWidget(w)
	}
	d := s.Run.Detail
	dirty := ""
	if d.Run.Dirty {
		dirty = " with uncommitted changes"
	}
	heading(&r.title, fmt.Sprintf("Run %d at %s%s, %s", d.Run.ID, d.Run.Commit.Short(), dirty, d.Run.CreatedAt.Local().Format("2006-01-02 15:04")), 1.2)
	r.tag.SetField(state.FieldRunTag, "Tag slug", "none")
	r.tag.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.RunAnnotate{}) })
	r.note.SetField(state.FieldRunNote, "Note", "what this run measured")
	r.note.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.RunAnnotate{}) })
	r.save.SetText("Save")
	r.save.SetType(basicwidget.ButtonTypePrimary)
	r.save.OnDown(func(context *guigui.Context) { m.dispatch(state.RunAnnotate{}) })
	r.export.SetText("Export CSV")
	r.export.OnDown(func(context *guigui.Context) { m.dispatch(state.RunExport{}) })
	exported := ""
	if s.Run.Export != "" {
		exported = "Saved to " + s.Run.Export
	}
	caption(&r.saved, exported, basicwidgetdraw.TextColor(context.ColorMode(), false))

	r.chart.SetChart(state.RunChart(d), false)

	cols := []basicwidget.TableColumn{columnOf("Tables", 1, true)}
	for _, ph := range db.Phases {
		cols = append(cols, columnOf(string(ph)+" wall", 2, true))
	}
	cols = append(cols, columnOf("Source", 2, true), columnOf("Generated", 2, true))
	r.table.SetColumns(cols)
	var sizes []int
	for _, p := range d.Phases {
		if !slices.Contains(sizes, p.Tables) {
			sizes = append(sizes, p.Tables)
		}
	}
	slices.Sort(sizes)
	r.rows = r.rows[:0]
	for _, n := range sizes {
		cells := []basicwidget.TableCell{cellRight(strconv.Itoa(n))}
		var source, output float64
		for _, ph := range db.Phases {
			text := ""
			for _, p := range d.Phases {
				if p.Phase == ph && p.Tables == n {
					text = p.WallMS.String()
					source = max(source, p.SourceKB)
					output = max(output, p.OutputMB)
				}
			}
			cells = append(cells, cellRight(text))
		}
		cells = append(cells, cellRight(fmt.Sprintf("%.1f KB", source)), cellRight(fmt.Sprintf("%.2f MB", output)))
		r.rows = append(r.rows, basicwidget.TableRow[int]{Value: n, Cells: cells})
	}
	r.table.SetItems(r.rows)

	var keys []string
	for _, k := range d.Keystrokes {
		keys = append(keys, fmt.Sprintf("%s per keystroke at %d tables", units.Millis(k.MsPerOp), k.Tables))
	}
	if len(keys) == 0 {
		keys = []string{"no keystroke benchmark in this run"}
	}
	caption(&r.footer, "Language server: "+strings.Join(keys, ", "), nil)
	lat := "Routes: no latency samples"
	if d.Latency.N > 0 {
		lat = fmt.Sprintf("Routes: %d timed calls, mean %s, best %s, worst %s", d.Latency.N, units.Millis(d.Latency.Mean.V), units.Millis(d.Latency.Best.V), units.Millis(d.Latency.Worst.V))
	}
	caption(&r.latency, lat, nil)
	return nil
}

func (r *runScreen) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	a := r.lay.reset()
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	m := modelOf(context, r)
	if m == nil || m.s.Run.ID == 0 || !m.s.Run.Loaded {
		layouter.LayoutWidget(&r.none, b)
		return
	}
	form := a.row(u/2, flex(&r.tag, 2), flex(&r.note, 5), a.besideField(&r.save, u, 4*u), a.besideField(&r.export, u, 5*u))
	tableHeight := u * (len(r.rows) + 2)
	padded(a.column(u/2,
		fixed(&r.title, u*3/2),
		a.nested(form, guigui.Size{}),
		fixed(&r.saved, u),
		flex(&r.chart, 1),
		fixed(&r.table, tableHeight),
		fixed(&r.footer, u),
		fixed(&r.latency, u),
	), u/2).LayoutWidgets(context, b, layouter)
}
