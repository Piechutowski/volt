// Tables is the smallest Volt application: one table in db/schema.volt,
// its CRUD routes from one resources line in app/routes.volt, and a
// window that calls them through the generated client.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"image"
	"net"
	"net/http"
	"os"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	_ "github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt/examples/tables/app"
	"github.com/Piechutowski/volt/examples/tables/app/client"
	"github.com/Piechutowski/volt/examples/tables/db"
	"github.com/Piechutowski/volt/nao/run"
)

//go:embed db/volt_schema.sql
var schema string

func main() {
	base, err := serve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root := &Root{client: client.New(base)}
	root.reload()
	if err := guigui.Run(root, &guigui.RunOptions{Title: "Volt: tables", WindowMinSize: image.Pt(480, 360)}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() (string, error) {
	d, err := run.Open("sqlite3", ":memory:")
	if err != nil {
		return "", err
	}
	if _, err := d.Pool().Exec(schema); err != nil {
		return "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go http.Serve(ln, app.NewRouter(app.Controllers{DB: d}))
	return "http://" + ln.Addr().String(), nil
}

// Root is the window: a field to add a note, one row per note, and the
// last call the client made.
type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	title      basicwidget.TextInput
	add        basicwidget.Button
	rows       guigui.WidgetSlice[*noteRow]
	status     basicwidget.Text

	client *client.Client
	notes  []db.Note
	last   string

	rowItems []guigui.LinearLayoutItem
	items    []guigui.LinearLayoutItem
}

func (r *Root) reload() {
	notes, err := r.client.Notes(context.Background())
	if err != nil {
		r.last = "client.Notes: " + err.Error()
		return
	}
	r.notes = notes
}

func (r *Root) call(what string, err error) {
	r.last = what
	if err != nil {
		r.last += ": " + err.Error()
	}
	r.reload()
}

func (r *Root) Build(gc *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.title)
	adder.AddWidget(&r.add)
	r.rows.SetLen(len(r.notes))
	for _, row := range r.rows.All() {
		adder.AddWidget(row)
	}
	adder.AddWidget(&r.status)

	r.add.SetText("Add")
	r.add.OnDown(func(ctx *guigui.Context) {
		_, err := r.client.CreateNote(context.Background(), db.NoteNew().TitleSet(r.title.Value()))
		r.call("client.CreateNote", err)
		if err == nil {
			r.title.ForceSetValue("")
		}
	})
	for i, row := range r.rows.All() {
		n := r.notes[i]
		row.note = n
		row.toggle = func() {
			_, err := r.client.UpdateNote(context.Background(), n.ID, db.NoteNew().DoneSet(!n.Done))
			r.call(fmt.Sprintf("client.UpdateNote(%d)", n.ID), err)
		}
		row.remove = func() {
			r.call(fmt.Sprintf("client.DeleteNote(%d)", n.ID), r.client.DeleteNote(context.Background(), n.ID))
		}
	}
	r.status.SetValue(r.last)
	return nil
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(context)
	r.rowItems = slices.Delete(r.rowItems, 0, len(r.rowItems))
	r.rowItems = append(r.rowItems,
		guigui.LinearLayoutItem{Widget: &r.title, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.add, Size: guigui.FixedSize(4 * u)},
	)
	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.rowItems, Gap: u / 2,
	}})
	for _, row := range r.rows.All() {
		r.items = append(r.items, guigui.LinearLayoutItem{Widget: row, Size: guigui.FixedSize(u)})
	}
	r.items = append(r.items,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.status, Size: guigui.FixedSize(u)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.items, Gap: u / 2,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

type noteRow struct {
	guigui.DefaultWidget

	done   basicwidget.Checkbox
	title  basicwidget.Text
	delete basicwidget.Button

	note   db.Note
	toggle func()
	remove func()

	items []guigui.LinearLayoutItem
}

func (n *noteRow) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&n.done)
	adder.AddWidget(&n.title)
	adder.AddWidget(&n.delete)
	n.done.SetValue(n.note.Done)
	n.done.OnValueChanged(func(*guigui.Context, bool) { n.toggle() })
	n.title.SetValue(fmt.Sprintf("%d  %s", n.note.ID, n.note.Title))
	n.title.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	n.delete.SetText("Delete")
	n.delete.OnDown(func(*guigui.Context) { n.remove() })
	return nil
}

func (n *noteRow) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	n.items = slices.Delete(n.items, 0, len(n.items))
	n.items = append(n.items,
		guigui.LinearLayoutItem{Widget: &n.done, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &n.title, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &n.delete, Size: guigui.FixedSize(4 * u)},
	)
	(guigui.LinearLayout{Direction: guigui.LayoutDirectionHorizontal, Items: n.items, Gap: u / 2}).
		LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
