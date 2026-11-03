// Validation shows a changeset refused before the database sees it
// (db/schema.volt): required and the Go validator EmailShape answer
// 422, unique answers 409, and the window puts each field error of the
// Problem the client receives under its field.
package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"image"
	"net"
	"net/http"
	"os"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	_ "github.com/mattn/go-sqlite3"

	"github.com/Piechutowski/volt"
	"github.com/Piechutowski/volt/examples/validation/app"
	"github.com/Piechutowski/volt/examples/validation/app/client"
	"github.com/Piechutowski/volt/examples/validation/db"
	"github.com/Piechutowski/volt/nao/run"
)

//go:embed db/volt_schema.sql
var schema string

var bg = context.Background()

func main() {
	base, err := serve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root := &Root{client: client.New(base)}
	root.reload()
	if err := guigui.Run(root, &guigui.RunOptions{Title: "Volt: validation", WindowMinSize: image.Pt(560, 400)}); err != nil {
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

// Root is the window: the form with an error line under each field, the
// members stored so far, and the last reply.
type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	email      basicwidget.TextInput
	emailErr   basicwidget.Text
	name       basicwidget.TextInput
	nameErr    basicwidget.Text
	create     basicwidget.Button
	members    guigui.WidgetSlice[*basicwidget.Text]
	status     basicwidget.Text

	client *client.Client
	rows   []string
	errs   map[string]string
	last   string

	items []guigui.LinearLayoutItem
}

func (r *Root) reload() {
	members, err := r.client.Members(bg)
	r.rows = r.rows[:0]
	if err != nil {
		r.rows = append(r.rows, "client.Members: "+err.Error())
	}
	for _, m := range members {
		r.rows = append(r.rows, fmt.Sprintf("%d  %s  %s", m.ID, m.Email, m.Name))
	}
}

func (r *Root) submit() {
	_, err := r.client.CreateMember(bg, db.MemberNew().EmailSet(r.email.Value()).NameSet(r.name.Value()))
	r.errs = map[string]string{}
	var pe *volt.ProblemError
	switch {
	case err == nil:
		r.last = "client.CreateMember: stored"
		r.email.ForceSetValue("")
		r.name.ForceSetValue("")
	case errors.As(err, &pe):
		r.last = fmt.Sprintf("client.CreateMember: %d, %d field error(s)", pe.Status, len(pe.Details()))
		for _, d := range pe.Details() {
			for _, col := range d.Columns {
				if r.errs[col] != "" {
					r.errs[col] += "; "
				}
				r.errs[col] += d.Check + ": " + d.Message
			}
		}
	default:
		r.last = "client.CreateMember: " + err.Error()
	}
	r.reload()
}

func (r *Root) Build(gc *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.email)
	adder.AddWidget(&r.emailErr)
	adder.AddWidget(&r.name)
	adder.AddWidget(&r.nameErr)
	adder.AddWidget(&r.create)
	r.members.SetLen(len(r.rows))
	for _, t := range r.members.All() {
		adder.AddWidget(t)
	}
	adder.AddWidget(&r.status)

	r.email.SetPlaceholder("email")
	r.name.SetPlaceholder("name")
	r.emailErr.SetValue(r.errs["email"])
	r.nameErr.SetValue(r.errs["name"])
	r.create.SetText("Create member")
	r.create.OnDown(func(*guigui.Context) { r.submit() })
	for i, t := range r.members.All() {
		t.SetValue(r.rows[i])
	}
	r.status.SetValue(r.last)
	return nil
}

func (r *Root) Layout(gc *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(gc)
	r.items = slices.Delete(r.items, 0, len(r.items))
	for _, w := range []guigui.Widget{&r.email, &r.emailErr, &r.name, &r.nameErr, &r.create} {
		r.items = append(r.items, guigui.LinearLayoutItem{Widget: w, Size: guigui.FixedSize(u)})
	}
	for _, t := range r.members.All() {
		r.items = append(r.items, guigui.LinearLayoutItem{Widget: t, Size: guigui.FixedSize(u)})
	}
	r.items = append(r.items,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &r.status, Size: guigui.FixedSize(u)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.items, Gap: u / 4,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(gc, widgetBounds.Bounds(), layouter)
}
