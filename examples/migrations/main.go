// Migrations shows hand-written migrations (db/migrations.volt) folding
// to the schema (db/schema.volt): Up and Down move the database through
// them, the window lists the ledger, and the posts table shows the
// column 0002 adds and its down drops.
package main

//go:generate go run github.com/Piechutowski/volt/cmd/volt gen --sql ./...

import (
	"context"
	_ "embed"
	"fmt"
	"image"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	_ "github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt/examples/migrations/app"
	"github.com/Piechutowski/volt/examples/migrations/app/client"
	"github.com/Piechutowski/volt/examples/migrations/db"
	"github.com/Piechutowski/volt/nao/rt"
	"github.com/Piechutowski/volt/nao/run"
)

var bg = context.Background()

func main() {
	d, base, err := serve(filepath.Join(os.TempDir(), "volt-example-migrations.db"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root := &Root{d: d, client: client.New(base)}
	root.refresh()
	if err := guigui.Run(root, &guigui.RunOptions{Title: "Volt: migrations", WindowMinSize: image.Pt(560, 400)}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(path string) (run.DB, string, error) {
	d, err := run.Open("sqlite3", path)
	if err != nil {
		return run.DB{}, "", err
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return run.DB{}, "", err
	}
	go http.Serve(ln, app.NewRouter(app.Controllers{DB: d}))
	return d, "http://" + ln.Addr().String(), nil
}

// Root is the window: the ledger with Up and Down, then the posts the
// generated client reads at the current schema.
type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	up         basicwidget.Button
	down       basicwidget.Button
	ledger     guigui.WidgetSlice[*basicwidget.Text]
	title      basicwidget.TextInput
	add        basicwidget.Button
	posts      guigui.WidgetSlice[*basicwidget.Text]
	status     basicwidget.Text

	d      run.DB
	client *client.Client
	lines  []string
	rows   []string
	last   string

	buttonItems []guigui.LinearLayoutItem
	addItems    []guigui.LinearLayoutItem
	items       []guigui.LinearLayoutItem
}

func (r *Root) refresh() {
	statuses, err := run.MigrateStatus(bg, r.d, db.Migrations)
	r.lines = r.lines[:0]
	if err != nil {
		r.lines = append(r.lines, "run.MigrateStatus: "+err.Error())
	}
	for _, s := range statuses {
		r.lines = append(r.lines, ledgerLine(s))
	}
	r.rows = r.rows[:0]
	posts, err := r.client.Posts(bg)
	if err != nil {
		r.rows = append(r.rows, "client.Posts: "+err.Error()+" (the generated queries expect every migration applied)")
	}
	for _, p := range posts {
		r.rows = append(r.rows, fmt.Sprintf("%d  %s  (views %d)", p.ID, p.Title, p.Views))
	}
}

func ledgerLine(s rt.MigrationStatus) string {
	if s.Applied {
		return "applied  " + s.Name + "  " + s.AppliedAt
	}
	return "pending  " + s.Name
}

func (r *Root) Build(gc *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.up)
	adder.AddWidget(&r.down)
	r.ledger.SetLen(len(r.lines))
	for _, t := range r.ledger.All() {
		adder.AddWidget(t)
	}
	adder.AddWidget(&r.title)
	adder.AddWidget(&r.add)
	r.posts.SetLen(len(r.rows))
	for _, t := range r.posts.All() {
		adder.AddWidget(t)
	}
	adder.AddWidget(&r.status)

	r.up.SetText("Up")
	r.up.OnDown(func(*guigui.Context) {
		applied, err := run.MigrateUp(bg, r.d, db.Migrations)
		r.last = fmt.Sprintf("run.MigrateUp applied %v", applied)
		if err != nil {
			r.last += ": " + err.Error()
		}
		r.refresh()
	})
	r.down.SetText("Down")
	r.down.OnDown(func(*guigui.Context) {
		name, err := run.MigrateDown(bg, r.d, db.Migrations)
		r.last = "run.MigrateDown reverted " + name
		if err != nil {
			r.last += ": " + err.Error()
		}
		r.refresh()
	})
	for i, t := range r.ledger.All() {
		t.SetValue(r.lines[i])
	}
	r.add.SetText("Add post")
	r.add.OnDown(func(*guigui.Context) {
		_, err := r.client.CreatePost(bg, db.PostNew().TitleSet(r.title.Value()))
		r.last = "client.CreatePost"
		if err != nil {
			r.last += ": " + err.Error()
		} else {
			r.title.ForceSetValue("")
		}
		r.refresh()
	})
	for i, t := range r.posts.All() {
		t.SetValue(r.rows[i])
	}
	r.status.SetValue(r.last)
	return nil
}

func (r *Root) Layout(gc *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(gc)
	r.buttonItems = slices.Delete(r.buttonItems, 0, len(r.buttonItems))
	r.buttonItems = append(r.buttonItems,
		guigui.LinearLayoutItem{Widget: &r.up, Size: guigui.FixedSize(4 * u)},
		guigui.LinearLayoutItem{Widget: &r.down, Size: guigui.FixedSize(4 * u)},
	)
	r.addItems = slices.Delete(r.addItems, 0, len(r.addItems))
	r.addItems = append(r.addItems,
		guigui.LinearLayoutItem{Widget: &r.title, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.add, Size: guigui.FixedSize(5 * u)},
	)
	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.buttonItems, Gap: u / 2,
	}})
	for _, t := range r.ledger.All() {
		r.items = append(r.items, guigui.LinearLayoutItem{Widget: t, Size: guigui.FixedSize(u)})
	}
	r.items = append(r.items, guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal, Items: r.addItems, Gap: u / 2,
	}})
	for _, t := range r.posts.All() {
		r.items = append(r.items, guigui.LinearLayoutItem{Widget: t, Size: guigui.FixedSize(u)})
	}
	r.items = append(r.items,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.status, Size: guigui.FixedSize(u)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.items, Gap: u / 2,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(gc, widgetBounds.Bounds(), layouter)
}
