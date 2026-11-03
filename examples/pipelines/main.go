// Pipelines shows named middleware lists (app/routes.volt): the api
// pipeline guards every route and the vault pipeline is added under
// /vault, so the reply's X-Plugs lists the plugs a request went
// through and RequireKey refuses a request without the key.
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

	"github.com/Piechutowski/volt/examples/pipelines/app"
	"github.com/Piechutowski/volt/examples/pipelines/app/client"
)

func main() {
	base, err := serve()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	root := &Root{}
	root.client = client.New(base)
	root.client.HTTP = &http.Client{Transport: keyed{root: root}}
	if err := guigui.Run(root, &guigui.RunOptions{Title: "Volt: pipelines", WindowMinSize: image.Pt(560, 360)}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve() (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go http.Serve(ln, app.NewRouter(app.Controllers{Pages: app.Pages{}, Vault: app.Vault{}}))
	return "http://" + ln.Addr().String(), nil
}

type keyed struct{ root *Root }

func (k keyed) RoundTrip(r *http.Request) (*http.Response, error) {
	if k.root.sendKey {
		r = r.Clone(r.Context())
		r.Header.Set("X-Key", "secret")
	}
	return http.DefaultTransport.RoundTrip(r)
}

// Root is the window: whether to send the key, a button per route and
// the last reply.
type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	key        basicwidget.Checkbox
	keyLabel   basicwidget.Text
	open       basicwidget.Button
	secret     basicwidget.Button
	reply      basicwidget.Text

	client  *client.Client
	sendKey bool
	last    string

	keyItems []guigui.LinearLayoutItem
	items    []guigui.LinearLayoutItem
}

func (r *Root) Build(gc *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.key)
	adder.AddWidget(&r.keyLabel)
	adder.AddWidget(&r.open)
	adder.AddWidget(&r.secret)
	adder.AddWidget(&r.reply)

	r.key.SetValue(r.sendKey)
	r.key.OnValueChanged(func(_ *guigui.Context, v bool) { r.sendKey = v })
	r.keyLabel.SetValue("send X-Key: secret")
	r.keyLabel.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	r.open.SetText("GET " + app.PathOpen())
	r.open.OnDown(func(*guigui.Context) {
		resp, err := r.client.Open(context.Background())
		r.last = replyText(app.PathOpen(), resp, err)
	})
	r.secret.SetText("GET " + app.PathVaultSecret())
	r.secret.OnDown(func(*guigui.Context) {
		resp, err := r.client.VaultSecret(context.Background())
		r.last = replyText(app.PathVaultSecret(), resp, err)
	})
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
	return strings.Join([]string{
		"GET " + path,
		"status " + resp.Status,
		"X-Request-Id " + resp.Header.Get("X-Request-Id"),
		"X-Plugs " + strings.Join(resp.Header.Values("X-Plugs"), ", "),
		strings.TrimSpace(string(body)),
	}, "\n")
}

func (r *Root) Layout(gc *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())
	u := basicwidget.UnitSize(gc)
	r.keyItems = slices.Delete(r.keyItems, 0, len(r.keyItems))
	r.keyItems = append(r.keyItems,
		guigui.LinearLayoutItem{Widget: &r.key, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &r.keyLabel, Size: guigui.FlexibleSize(1)},
	)
	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items,
		guigui.LinearLayoutItem{Size: guigui.FixedSize(u), Layout: &guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal, Items: r.keyItems, Gap: u / 2,
		}},
		guigui.LinearLayoutItem{Widget: &r.open, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &r.secret, Size: guigui.FixedSize(u)},
		guigui.LinearLayoutItem{Widget: &r.reply, Size: guigui.FlexibleSize(1)},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical, Items: r.items, Gap: u / 2,
		Padding: guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(gc, widgetBounds.Bounds(), layouter)
}
