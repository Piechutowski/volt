package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"

	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

type runsScreen struct {
	guigui.DefaultWidget

	filterLabel basicwidget.Text
	filter      basicwidget.Select[string]
	pageText    basicwidget.Text
	prev        basicwidget.Button
	next        basicwidget.Button
	table       basicwidget.Table[int]
	tagsTitle   basicwidget.Text
	tagsText    basicwidget.Text
	slug        field
	name        field
	add         basicwidget.Button
	rename      basicwidget.Button
	remove      basicwidget.Button

	rows []basicwidget.TableRow[int]
	lay  layouts
}

func (r *runsScreen) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	for _, w := range []guigui.Widget{&r.filterLabel, &r.filter, &r.pageText, &r.prev, &r.next, &r.table, &r.tagsTitle, &r.tagsText, &r.slug, &r.name, &r.add, &r.rename, &r.remove} {
		adder.AddWidget(w)
	}
	m := modelOf(context, r)
	if m == nil {
		return nil
	}
	s := m.s
	u := basicwidget.UnitSize(context)

	caption(&r.filterLabel, "Tag", nil)
	items := []basicwidget.SelectItem[string]{{Text: "Every run", Value: ""}}
	var names []string
	for _, t := range s.Tags {
		items = append(items, basicwidget.SelectItem[string]{Text: t.Name, Value: t.Slug})
		names = append(names, t.Slug+" ("+t.Name+")")
	}
	r.filter.SetItems(items)
	r.filter.SelectItemByValue(s.Runs.Tag)
	r.filter.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := r.filter.ItemByIndex(index); ok && item.Value != m.s.Runs.Tag {
			m.dispatch(state.RunsFilter{Tag: item.Value})
		}
	})

	page := s.Runs.Page
	from := len(s.Runs.Back)*state.RunsLimit + 1
	switch {
	case !s.Runs.Loaded:
		caption(&r.pageText, "Loading runs", nil)
	case len(page.Runs) == 0:
		caption(&r.pageText, "No runs", nil)
	default:
		caption(&r.pageText, fmt.Sprintf("Runs %d to %d of %d", from, from+len(page.Runs)-1, page.Total), nil)
	}
	r.pageText.SetHorizontalAlign(basicwidget.HorizontalAlignRight)
	r.prev.SetText("Previous")
	context.SetEnabled(&r.prev, len(s.Runs.Back) > 0)
	r.prev.OnDown(func(context *guigui.Context) { m.dispatch(state.RunsPrev{}) })
	r.next.SetText("Next")
	context.SetEnabled(&r.next, page.Next != 0)
	r.next.OnDown(func(context *guigui.Context) { m.dispatch(state.RunsNext{}) })

	r.table.SetColumns([]basicwidget.TableColumn{
		{HeaderText: "Run", HeaderTextHorizontalAlign: basicwidget.HorizontalAlignRight, Width: guigui.FlexibleSize(1), MinWidth: 2 * u},
		columnOf("Commit", 2, false),
		columnOf("Tag", 2, false),
		columnOf("Note", 4, false),
		columnOf("Created", 3, false),
	})
	r.rows = r.rows[:0]
	for _, run := range page.Runs {
		commit := run.Commit.Short()
		if run.Dirty {
			commit += " dirty"
		}
		r.rows = append(r.rows, basicwidget.TableRow[int]{Value: run.ID, Cells: []basicwidget.TableCell{
			cellRight(strconv.Itoa(run.ID)),
			cell(commit),
			cell(tagName(s, run.Tag.V)),
			cell(run.Note.V),
			cell(run.CreatedAt.Local().Format("2006-01-02 15:04")),
		}})
	}
	r.table.SetItems(r.rows)
	r.table.SelectItemByIndex(-1)
	r.table.OnItemSelected(func(context *guigui.Context, index int) {
		if row, ok := r.table.ItemByIndex(index); ok {
			m.dispatch(state.RunOpen{ID: row.Value})
		}
	})

	heading(&r.tagsTitle, "Tags", 1)
	if len(names) == 0 {
		names = []string{"none yet"}
	}
	caption(&r.tagsText, strings.Join(names, ", "), basicwidgetdraw.TextColor(context.ColorMode(), false))
	r.slug.FieldSet(state.FieldTagSlug, "Slug", "nightly")
	r.name.FieldSet(state.FieldTagName, "Name", "Nightly")
	r.name.OnSubmitted(func(context *guigui.Context) { m.dispatch(state.TagAdd{}) })
	r.add.SetText("Add tag")
	r.add.SetType(basicwidget.ButtonTypePrimary)
	r.add.OnDown(func(context *guigui.Context) { m.dispatch(state.TagAdd{}) })
	r.rename.SetText("Rename")
	r.rename.OnDown(func(context *guigui.Context) { m.dispatch(state.TagRename{}) })
	r.remove.SetText("Delete")
	r.remove.OnDown(func(context *guigui.Context) { m.dispatch(state.TagRemove{}) })
	return nil
}

func (r *runsScreen) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	if m := modelOf(context, r); m != nil && m.probe != nil {
		for i, row := range r.rows {
			m.probe.place(probeRow("runs", row.Value), r.table.ItemBounds(i))
		}
	}
	return nil
}

func tagName(s state.State, slug string) string {
	for _, t := range s.Tags {
		if t.Slug == slug {
			return t.Name
		}
	}
	return slug
}

func (r *runsScreen) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	a := r.lay.reset()
	u := basicwidget.UnitSize(context)
	bar := a.row(u/2, fixed(&r.filterLabel, 2*u), fixed(&r.filter, 8*u), flex(&r.pageText, 1), fixed(&r.prev, 4*u), fixed(&r.next, 4*u))
	tagForm := a.row(u/2, flex(&r.slug, 2), flex(&r.name, 3),
		a.besideField(&r.add, u, 4*u), a.besideField(&r.rename, u, 4*u), a.besideField(&r.remove, u, 4*u))
	padded(a.column(u/2,
		a.nested(bar, guigui.FixedSize(u*3/2)),
		flex(&r.table, 1),
		a.nested(a.row(u/2, fixed(&r.tagsTitle, 3*u), flex(&r.tagsText, 1)), guigui.FixedSize(u)),
		a.nested(tagForm, guigui.Size{}),
	), u/2).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
