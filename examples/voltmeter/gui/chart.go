package main

import (
	"image"
	"image/color"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/Piechutowski/volt/examples/voltmeter/gui/state"
)

var (
	seriesLight = []color.RGBA{{0x2a, 0x78, 0xd6, 0xff}, {0xeb, 0x68, 0x34, 0xff}, {0x1b, 0xaf, 0x7a, 0xff}, {0xed, 0xa1, 0x00, 0xff}}
	seriesDark  = []color.RGBA{{0x39, 0x87, 0xe5, 0xff}, {0xd9, 0x59, 0x26, 0xff}, {0x19, 0x9e, 0x70, 0xff}, {0xc9, 0x85, 0x00, 0xff}}
)

func seriesColor(context *guigui.Context, i int) color.Color {
	if context.ColorMode() == ebiten.ColorModeDark {
		return seriesDark[i%len(seriesDark)]
	}
	return seriesLight[i%len(seriesLight)]
}

type chartView struct {
	guigui.DefaultWidget

	title   basicwidget.Text
	empty   basicwidget.Text
	yLabels guigui.WidgetSlice[*basicwidget.Text]
	xLabels guigui.WidgetSlice[*basicwidget.Text]
	legend  guigui.WidgetSlice[*basicwidget.Text]

	chart   state.Chart
	frame   state.Frame
	compact bool
}

func (c *chartView) SetChart(chart state.Chart, compact bool) {
	c.chart, c.frame, c.compact = chart, chart.Frame(), compact
}

func (c *chartView) hasPoints() bool {
	for _, s := range c.chart.Series {
		if len(s.Points) > 0 {
			return true
		}
	}
	return false
}

func (c *chartView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if !c.hasPoints() {
		adder.AddWidget(&c.empty)
		caption(&c.empty, "No measurements yet", basicwidgetdraw.TextColor(context.ColorMode(), false))
		c.empty.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		return nil
	}
	if c.compact {
		return nil
	}
	adder.AddWidget(&c.title)
	caption(&c.title, c.chart.YLabel+" by "+c.chart.XLabel, basicwidgetdraw.TextColor(context.ColorMode(), false))
	c.yLabels.SetLen(len(c.frame.YTicks))
	c.xLabels.SetLen(len(c.frame.XTicks))
	n := 0
	if len(c.chart.Series) > 1 {
		n = len(c.chart.Series)
	}
	c.legend.SetLen(n)
	for _, t := range c.yLabels.All() {
		adder.AddWidget(t)
	}
	for _, t := range c.xLabels.All() {
		adder.AddWidget(t)
	}
	for _, t := range c.legend.All() {
		adder.AddWidget(t)
	}
	for i, t := range c.yLabels.All() {
		caption(t, c.frame.YTicks[i].Label, nil)
		t.SetHorizontalAlign(basicwidget.HorizontalAlignRight)
	}
	for i, t := range c.xLabels.All() {
		caption(t, c.frame.XTicks[i].Label, nil)
		t.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	}
	for i, t := range c.legend.All() {
		caption(t, c.chart.Series[i].Name, nil)
	}
	return nil
}

func (c *chartView) geometry(context *guigui.Context, b image.Rectangle) (plot, top image.Rectangle, entry int) {
	u := basicwidget.UnitSize(context)
	if c.compact {
		return b.Inset(u / 4), image.Rectangle{}, 0
	}
	top = image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+u)
	plot = image.Rect(b.Min.X+3*u, top.Max.Y+u/2, b.Max.X-u, b.Max.Y-u*3/2)
	return plot, top, 4 * u
}

func (c *chartView) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	if !c.hasPoints() {
		layouter.LayoutWidget(&c.empty, b)
		return
	}
	if c.compact {
		return
	}
	u := basicwidget.UnitSize(context)
	plot, top, entry := c.geometry(context, b)
	layouter.LayoutWidget(&c.title, image.Rect(top.Min.X, top.Min.Y, top.Max.X-entry*c.legend.Len(), top.Max.Y))
	for i, t := range c.legend.All() {
		x := top.Max.X - entry*(c.legend.Len()-i) + u*3/4
		layouter.LayoutWidget(t, image.Rect(x, top.Min.Y, x+entry-u*3/4, top.Max.Y))
	}
	w, h := float64(plot.Dx()), float64(plot.Dy())
	for i, t := range c.yLabels.All() {
		_, y := c.frame.At(state.Point{X: c.frame.XMin, Y: c.frame.YTicks[i].Value}, w, h)
		cy := plot.Min.Y + int(y)
		layouter.LayoutWidget(t, image.Rect(b.Min.X, cy-u/2, plot.Min.X-u/3, cy+u/2))
	}
	for i, t := range c.xLabels.All() {
		x, _ := c.frame.At(state.Point{X: c.frame.XTicks[i].Value}, w, h)
		cx := plot.Min.X + int(x)
		layouter.LayoutWidget(t, image.Rect(cx-2*u, plot.Max.Y+u/4, cx+2*u, plot.Max.Y+u*5/4))
	}
}

func (c *chartView) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	if !c.hasPoints() {
		return
	}
	u := float32(basicwidget.UnitSize(context))
	mode := context.ColorMode()
	plot, top, entry := c.geometry(context, widgetBounds.Bounds())
	grid, _ := basicwidgetdraw.BorderColors(mode, basicwidgetdraw.RoundedRectBorderTypeRegular)
	axis := basicwidgetdraw.TextColor(mode, false)
	w, h := float64(plot.Dx()), float64(plot.Dy())
	at := func(p state.Point) (float32, float32) {
		x, y := c.frame.At(p, w, h)
		return float32(plot.Min.X) + float32(x), float32(plot.Min.Y) + float32(y)
	}
	if !c.compact {
		for _, t := range c.frame.YTicks {
			x0, y := at(state.Point{X: c.frame.XMin, Y: t.Value})
			x1, _ := at(state.Point{X: c.frame.XMax, Y: t.Value})
			vector.StrokeLine(dst, x0, y, x1, y, 1, grid, false)
		}
		x0, y0 := at(state.Point{X: c.frame.XMin, Y: 0})
		x1, _ := at(state.Point{X: c.frame.XMax, Y: 0})
		vector.StrokeLine(dst, x0, y0, x1, y0, 1, axis, false)
		for _, t := range c.frame.XTicks {
			x, y := at(state.Point{X: t.Value})
			vector.StrokeLine(dst, x, y, x, y+u/6, 1, axis, false)
		}
		for i := range c.legend.Len() {
			x := float32(top.Max.X - entry*(c.legend.Len()-i))
			cy := float32(top.Min.Y) + u/2
			vector.StrokeLine(dst, x, cy, x+u/2, cy, 2*float32(context.Scale()), seriesColor(context, i), true)
		}
	}
	stroke := 2 * float32(context.Scale())
	dot := 4 * float32(context.Scale())
	if c.compact {
		dot = 2 * float32(context.Scale())
	}
	for i, s := range c.chart.Series {
		clr := seriesColor(context, i)
		for j := 1; j < len(s.Points); j++ {
			xa, ya := at(s.Points[j-1])
			xb, yb := at(s.Points[j])
			vector.StrokeLine(dst, xa, ya, xb, yb, stroke, clr, true)
		}
		for _, p := range s.Points {
			x, y := at(p)
			vector.FillCircle(dst, x, y, dot, clr, true)
		}
	}
}
