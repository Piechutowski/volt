package main

import (
	"fmt"
	"strconv"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

type budgetsScreen struct {
	guigui.DefaultWidget

	listTitle   basicwidget.Text
	reset       basicwidget.Button
	table       basicwidget.Table[int]
	addTitle    basicwidget.Text
	phaseLabel  basicwidget.Text
	phase       phasePicker
	tables      field
	maxMS       field
	add         basicwidget.Button
	detailTitle basicwidget.Text
	detailState basicwidget.Text
	ceiling     field
	save        basicwidget.Button
	breaches    basicwidget.Table[int]

	rows       []basicwidget.TableRow[int]
	breachRows []basicwidget.TableRow[int]
	lay        layouts
}

func (b *budgetsScreen) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	for _, w := range []guigui.Widget{&b.listTitle, &b.reset, &b.table, &b.addTitle, &b.phaseLabel, &b.phase, &b.tables, &b.maxMS, &b.add, &b.detailTitle, &b.detailState, &b.ceiling, &b.save, &b.breaches} {
		adder.AddWidget(w)
	}
	m := modelOf(context, b)
	if m == nil {
		return nil
	}
	s := m.s
	heading(&b.listTitle, "Ceilings on wall time", 1)
	b.reset.SetText("Reset breached")
	b.reset.OnDown(func(context *guigui.Context) { m.dispatch(state.BudgetsReset{}) })
	b.table.SetColumns([]basicwidget.TableColumn{columnOf("Budget", 1, true), columnOf("Phase", 2, false), columnOf("Tables", 1, true), columnOf("Ceiling", 2, true), columnOf("State", 2, false)})
	b.rows = b.rows[:0]
	for _, x := range s.Budgets.List {
		st := cell(string(x.State))
		if x.State == db.HealthBreached {
			st.TextStyle.Style.SetColor(basicwidget.DangerTintColor())
			st.TextStyle.Style.SetBold(true)
		}
		b.rows = append(b.rows, basicwidget.TableRow[int]{Value: x.ID, Cells: []basicwidget.TableCell{
			cellRight(strconv.Itoa(x.ID)), cell(string(x.Phase)), cellRight(strconv.Itoa(x.Tables)), cellRight(units.Millis(x.MaxMs).String()), st,
		}})
	}
	b.table.SetItems(b.rows)
	b.table.SelectItemByValue(s.Budgets.Selected)
	b.table.OnItemSelected(func(context *guigui.Context, index int) {
		if row, ok := b.table.ItemByIndex(index); ok && row.Value != m.s.Budgets.Selected {
			m.dispatch(state.BudgetPick{ID: row.Value})
		}
	})

	heading(&b.addTitle, "Add a budget", 1)
	caption(&b.phaseLabel, "Phase", nil)
	b.phase.SetPhase(db.Phase(s.Fields[state.FieldBudgetPhase]), db.Phases)
	b.phase.OnPicked(func(context *guigui.Context, phase db.Phase) {
		m.dispatch(state.FieldEdit{Field: state.FieldBudgetPhase, Value: string(phase)})
	})
	b.tables.SetField(state.FieldBudgetTables, "Tables", "160")
	b.maxMS.SetField(state.FieldBudgetMax, "Ceiling in ms", "40")
	b.maxMS.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.BudgetAdd{}) })
	b.add.SetText("Add budget")
	b.add.SetType(basicwidget.ButtonTypePrimary)
	b.add.OnDown(func(context *guigui.Context) { m.dispatch(state.BudgetAdd{}) })
	if msg := s.Errors[state.FieldBudgetPhase]; msg != "" {
		caption(&b.phaseLabel, "Phase: "+msg, basicwidget.DangerTintColor())
	}

	sel, ok := s.BudgetSelected()
	muted := basicwidgetdraw.TextColor(context.ColorMode(), false)
	if !ok {
		heading(&b.detailTitle, "No budget selected", 1)
		caption(&b.detailState, "Add a budget, then pick it to edit its ceiling.", muted)
	} else {
		heading(&b.detailTitle, fmt.Sprintf("Budget %d: %s at %d tables", sel.ID, sel.Phase, sel.Tables), 1)
		text := "OK: no upload has gone over " + units.Millis(sel.MaxMs).String() + " since this budget was set or reset; earlier runs are not judged"
		var clr = muted
		if sel.State == db.HealthBreached {
			text = "Breached: a run went over " + units.Millis(sel.MaxMs).String()
			clr = basicwidget.DangerTintColor()
		}
		caption(&b.detailState, text, clr)
	}
	context.SetEnabled(&b.save, ok)
	b.ceiling.SetField(state.FieldCeiling, "Ceiling in ms", "")
	b.ceiling.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.BudgetCeilingSave{}) })
	b.save.SetText("Save ceiling")
	b.save.SetType(basicwidget.ButtonTypePrimary)
	b.save.OnDown(func(context *guigui.Context) { m.dispatch(state.BudgetCeilingSave{}) })
	b.breaches.SetColumns([]basicwidget.TableColumn{columnOf("Breach", 1, true), columnOf("Run", 1, true), columnOf("Wall time", 2, true), columnOf("When", 3, false)})
	b.breachRows = b.breachRows[:0]
	if ok && s.Budgets.Detail.Budget.ID == sel.ID {
		for _, x := range s.Budgets.Detail.Breaches {
			b.breachRows = append(b.breachRows, basicwidget.TableRow[int]{Value: x.ID, Cells: []basicwidget.TableCell{
				cellRight(strconv.Itoa(x.ID)), cellRight(strconv.Itoa(x.RunID)), cellRight(units.Millis(x.WallMs).String()), cell(x.CreatedAt.Local().Format("2006-01-02 15:04:05")),
			}})
		}
	}
	b.breaches.SetItems(b.breachRows)
	return nil
}

func (b *budgetsScreen) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	a := b.lay.reset()
	u := basicwidget.UnitSize(context)
	left := a.column(u/2,
		a.nested(a.row(u/2, flex(&b.listTitle, 1), fixed(&b.reset, 6*u)), guigui.FixedSize(u*3/2)),
		flex(&b.table, 1),
		fixed(&b.addTitle, u),
		a.nested(a.column(u/8, fixed(&b.phaseLabel, u*3/4), fixed(&b.phase, u*3/2)), guigui.Size{}),
		a.nested(a.row(u/2, flex(&b.tables, 1), flex(&b.maxMS, 1), a.besideField(&b.add, u, 5*u)), guigui.Size{}),
	)
	right := a.column(u/2,
		fixed(&b.detailTitle, u*3/2),
		fixed(&b.detailState, u),
		a.nested(a.row(u/2, flex(&b.ceiling, 1), a.besideField(&b.save, u, 6*u)), guigui.Size{}),
		flex(&b.breaches, 1),
	)
	padded(a.row(u, a.nested(left, guigui.FlexibleSize(1)), a.nested(right, guigui.FlexibleSize(1))), u/2).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
