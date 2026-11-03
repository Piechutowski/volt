// Scopes shows nested scopes (app/routes.volt): a path prefix, a name
// that prefixes the generated helpers and client methods, and the
// nearest error handler answering a failure. Each button calls one
// route through the generated client and shows what came back.
package main

//go:generate go run github.com/Piechutowski/volt/cmd/volt gen --sql ./...

import (
	"context"
	"fmt"
	"image"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/Piechutowski/volt/examples/scopes/app"
	"github.com/Piechutowski/volt/examples/scopes/app/client"
)

func main() {
	base, err := serve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := guigui.Run(&Root{client: client.New(base)}, &guigui.RunOptions{Title: "Volt: scopes", WindowMinSize: image.Pt(560, 360)}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go http.Serve(ln, app.NewRouter(app.Controllers{Pages: app.Pages{}, Admin: app.Admin{}}))
	return "http://" + ln.Addr().String(), nil
}

type route struct {
	path string
	call func(*client.Client) (*http.Response, error)
}

var routes = []route{
	{app.PathHello(), func(c *client.Client) (*http.Response, error) { return c.Hello(context.Background()) }},
	{app.PathBroken(), func(c *client.Client) (*http.Response, error) { return c.Broken(context.Background()) }},
	{app.PathAdminStats(), func(c *client.Client) (*http.Response, error) { return c.AdminStats(context.Background()) }},
	{app.PathAdminBroken(), func(c *client.Client) (*http.Response, error) { return c.AdminBroken(context.Background()) }},
}

// Root is the window: a button per route and the last reply.
type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	buttons    guigui.WidgetSlice[*basicwidget.Button]
	reply      basicwidget.Text

	client *client.Client
	last   string

	items []guigui.LinearLayoutItem
}

func (r *Root) Build(gc *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	r.buttons.SetLen(len(routes))
	for _, b := range r.buttons.All() {
		adder.AddWidget(b)
	}
	adder.AddWidget(&r.reply)
	for i, b := range r.buttons.All() {
		rt := routes[i]
		b.SetText("GET " + rt.path)
		b.OnDown(func(*guigui.Context) {
			resp, err := rt.call(r.client)
			r.last = replyText(rt.path, resp, err)
		})
	}
	r.reply.SetMultiline(true)
	r.reply.SetValue(r.last)
	return nil
}

func replyText(path string, resp *http.Response, err error) string {
	if err != nil {
		return path + ": " + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	lines := []string{"GET " + path, "status " + resp.Status}
	if h := resp.Header.Get("X-Error-Handler"); h != "" {
		lines = append(lines, "answered by "+h)
	}
	return strings.Join(append(lines, strings.TrimSpace(string(body))), "\n")
}

func (r *Root) Layout(gc *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(gc)
	r.items = slices.Delete(r.items, 0, len(r.items))
	for _, b := range r.buttons.All() {
		r.items = append(r.items, guigui.LinearLayoutItem{Widget: b, Size: guigui.FixedSize(u)})
	}
	r.items = append(r.items, guigui.LinearLayoutItem{Widget: &r.reply, Size: guigui.FlexibleSize(1)})
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.items, Gap: u / 2,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(gc, widgetBounds.Bounds(), layouter)
}
