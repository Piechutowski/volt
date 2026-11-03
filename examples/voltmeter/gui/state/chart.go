package state

import (
	"math"
	"slices"
	"strconv"

	"github.com/Piechutowski/volt/examples/voltmeter/app"
	"github.com/Piechutowski/volt/examples/voltmeter/db"
	"github.com/Piechutowski/volt/examples/voltmeter/units"
)

// Chart is a line chart as data: named series of points on one x and
// one y axis, with the x ticks its builder chose. The chart widget
// draws it through Frame.
type Chart struct {
	XLabel string
	YLabel string
	XTicks []Tick
	Series []Series
}

// Series is one line of a chart.
type Series struct {
	Name   string
	Points []Point
}

// Point is one value of a series.
type Point struct{ X, Y float64 }

// Tick is a labelled value on an axis.
type Tick struct {
	Value float64
	Label string
}

// Frame is where a chart's values fall: the ranges both axes span and
// the y ticks, round numbers from zero to above the largest value.
type Frame struct {
	XMin, XMax float64
	YMax       float64
	XTicks     []Tick
	YTicks     []Tick
}

// Frame is the frame that holds every point of c, or the unit frame
// for a chart without points.
func (c Chart) Frame() Frame {
	f := Frame{XMin: math.Inf(1), XMax: math.Inf(-1), XTicks: c.XTicks}
	for _, s := range c.Series {
		for _, p := range s.Points {
			f.XMin, f.XMax = min(f.XMin, p.X), max(f.XMax, p.X)
			f.YMax = max(f.YMax, p.Y)
		}
	}
	if math.IsInf(f.XMin, 1) {
		f.XMin, f.XMax = 0, 1
	}
	if f.XMin == f.XMax {
		f.XMin, f.XMax = f.XMin-1, f.XMax+1
	}
	step := tickStep(f.YMax)
	f.YMax = step * math.Ceil(f.YMax/step)
	if f.YMax == 0 {
		f.YMax = step
	}
	places := decimals(step)
	for i := 0; float64(i)*step <= f.YMax+step/2; i++ {
		v := float64(i) * step
		f.YTicks = append(f.YTicks, Tick{Value: v, Label: strconv.FormatFloat(v, 'f', places, 64)})
	}
	return f
}

// At is where p falls in a plot of width w and height h, with y
// growing downwards as on a screen.
func (f Frame) At(p Point, w, h float64) (float64, float64) {
	x := (p.X - f.XMin) / (f.XMax - f.XMin) * w
	y := h - p.Y/f.YMax*h
	return x, y
}

func decimals(step float64) int {
	for d := 0; d < 9; d++ {
		scaled := step * math.Pow(10, float64(d))
		if math.Abs(scaled-math.Round(scaled)) < 1e-9 {
			return d
		}
	}
	return 9
}

func tickStep(top float64) float64 {
	if top <= 0 {
		return 1
	}
	raw := top / 4
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 2.5, 5, 10} {
		if raw <= m*mag {
			return m * mag
		}
	}
	return 10 * mag
}

// RunChart is a run's wall time against the table count, one series
// per phase in the order the sweep runs them.
func RunChart(d app.RunDetail) Chart {
	c := Chart{XLabel: "tables", YLabel: "wall ms"}
	var sizes []int
	for _, ph := range db.Phases {
		s := Series{Name: string(ph)}
		for _, r := range d.Phases {
			if r.Phase != ph {
				continue
			}
			s.Points = append(s.Points, Point{X: float64(r.Tables), Y: float64(r.WallMS)})
			if !slices.Contains(sizes, r.Tables) {
				sizes = append(sizes, r.Tables)
			}
		}
		if len(s.Points) > 0 {
			c.Series = append(c.Series, s)
		}
	}
	slices.Sort(sizes)
	for _, n := range sizes {
		c.XTicks = append(c.XTicks, Tick{Value: float64(n), Label: strconv.Itoa(n)})
	}
	return c
}

// TrendChart is a spot's wall time across runs, by run id.
func TrendChart(spot Spot, points []TrendPoint) Chart {
	c := Chart{XLabel: "run", YLabel: "wall ms"}
	s := Series{Name: string(spot.Phase) + " at " + strconv.Itoa(spot.Tables) + " tables"}
	for _, p := range points {
		s.Points = append(s.Points, Point{X: float64(p.RunID), Y: float64(p.WallMS)})
	}
	c.Series = []Series{s}
	c.XTicks = ticksSpread(len(points), func(i int) Tick {
		return Tick{Value: float64(points[i].RunID), Label: strconv.Itoa(points[i].RunID)}
	})
	return c
}

// ThesisChart is a fixture's lines per commit, oldest first: the Volt
// lines a person writes, the Go lines a person writes, and the Go lines
// volt gen writes.
func ThesisChart(rows []db.Thesis) Chart {
	c := Chart{XLabel: "commit", YLabel: "lines"}
	series := []Series{{Name: "volt"}, {Name: "hand"}, {Name: "generated"}}
	for i, r := range rows {
		x := float64(i)
		series[0].Points = append(series[0].Points, Point{X: x, Y: float64(r.VoltLines)})
		series[1].Points = append(series[1].Points, Point{X: x, Y: float64(r.HandLines)})
		series[2].Points = append(series[2].Points, Point{X: x, Y: float64(r.GenLines)})
	}
	if len(rows) > 0 {
		c.Series = series
	}
	c.XTicks = ticksSpread(len(rows), func(i int) Tick {
		return Tick{Value: float64(i), Label: units.Commit(rows[i].Sha).Short()}
	})
	return c
}

func ticksSpread(n int, tick func(int) Tick) []Tick {
	if n == 0 {
		return nil
	}
	step := max(1, (n+5)/6)
	var at []int
	for i := 0; i < n; i += step {
		at = append(at, i)
	}
	if last := n - 1; at[len(at)-1] != last {
		if last-at[len(at)-1] < step && len(at) > 1 {
			at = at[:len(at)-1]
		}
		at = append(at, last)
	}
	out := make([]Tick, 0, len(at))
	for _, i := range at {
		out = append(out, tick(i))
	}
	return out
}
