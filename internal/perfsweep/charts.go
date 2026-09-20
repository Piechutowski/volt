package perfsweep

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// The chart's colors: a dark surface and four categorical hues stepped
// for it, one per phase in a fixed order, validated for color-vision
// deficiency and contrast (worst adjacent pair well clear of the
// floor); text wears text tones, never a series color.
const (
	chartSurface = "#1a1a19"
	chartText    = "#ffffff"
	chartMuted   = "#c3c2b7"
	chartGrid    = "#3a3a38"
)

var phaseColors = map[string]string{"load": "#3987e5", "check": "#d95926", "vet": "#199e70", "generate": "#c98500"}

// Charts draws one SVG per measure, wall time, bytes allocated, heap in
// use and allocations, with a line per phase over the table counts,
// into dir; it returns the file names in the order drawn.
func Charts(dir string, rows []Row, columns int) ([]string, error) {
	xs := Sizes(rows)
	var names []string
	for _, m := range []struct {
		name, title, unit string
		pick              func(Row) float64
	}{
		{"wall", "Wall time per phase", "ms", func(r Row) float64 { return r.WallMS }},
		{"alloc", "Bytes allocated per run", "MB", func(r Row) float64 { return r.AllocMB }},
		{"heap", "Heap in use after the run", "MB", func(r Row) float64 { return r.HeapMB }},
		{"allocs", "Allocations per run", "thousands", func(r Row) float64 { return float64(r.Allocs) / 1000 }},
	} {
		series := map[string][]float64{}
		for _, r := range rows {
			series[r.Phase] = append(series[r.Phase], m.pick(r))
		}
		svg := LineChart(m.title, m.unit, fmt.Sprintf("tables (%d columns each, one file)", columns), xs, Phases, series, phaseColors)
		name := m.name + ".svg"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(svg), 0o644); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// LineChart renders a line chart on the dark surface: x is the table
// count, one line per named series in the order given, thin lines and
// round markers, recessive grid and ticks, a legend and a label at
// each line's end for two series or more.
func LineChart(title, unit, xlabel string, xs []int, names []string, series map[string][]float64, colors map[string]string) string {
	const w, h, left, right, top, bottom = 760, 420, 70, 90, 50, 50
	maxY := 0.0
	for _, name := range names {
		for _, v := range series[name] {
			maxY = max(maxY, v)
		}
	}
	if maxY == 0 {
		maxY = 1
	}
	maxX := float64(xs[len(xs)-1])
	px := func(x int) float64 { return left + float64(x)/maxX*(w-left-right) }
	py := func(v float64) float64 { return h - bottom - v/maxY*(h-top-bottom) }
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="ui-monospace, SFMono-Regular, Menlo, monospace" font-size="12">`+"\n", w, h, w, h)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="%s"/>`+"\n", w, h, chartSurface)
	fmt.Fprintf(&b, `<text x="%d" y="24" fill="%s" font-size="16">%s</text>`+"\n", left, chartText, title)
	// Grid and y ticks.
	for i := 0; i <= 5; i++ {
		v := maxY * float64(i) / 5
		y := py(v)
		fmt.Fprintf(&b, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="%s"/>`+"\n", left, y, w-right, y, chartGrid)
		fmt.Fprintf(&b, `<text x="%d" y="%.1f" fill="%s" text-anchor="end">%s</text>`+"\n", left-8, y+4, chartMuted, fmtNum(v))
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" transform="rotate(-90 14 %d)">%s</text>`+"\n", 14, (h-bottom+top)/2, chartMuted, (h-bottom+top)/2, unit)
	// x ticks at every measured size.
	for _, x := range xs {
		fmt.Fprintf(&b, `<line x1="%.1f" y1="%d" x2="%.1f" y2="%d" stroke="%s"/>`+"\n", px(x), top, px(x), h-bottom, chartGrid)
		fmt.Fprintf(&b, `<text x="%.1f" y="%d" fill="%s" text-anchor="middle">%d</text>`+"\n", px(x), h-bottom+18, chartMuted, x)
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" fill="%s" text-anchor="middle">%s</text>`+"\n", (w+left-right)/2, h-8, chartMuted, xlabel)
	// Lines, markers, a legend and end labels.
	for i, name := range names {
		vals := series[name]
		if len(vals) == 0 {
			continue
		}
		var pts []string
		for j, v := range vals {
			pts = append(pts, fmt.Sprintf("%.1f,%.1f", px(xs[j]), py(v)))
		}
		fmt.Fprintf(&b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="2" stroke-linejoin="round"/>`+"\n", strings.Join(pts, " "), colors[name])
		for j, v := range vals {
			fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="4" fill="%s" stroke="%s" stroke-width="2"/>`+"\n", px(xs[j]), py(v), colors[name], chartSurface)
		}
		if len(names) > 1 {
			last := len(vals) - 1
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" fill="%s">%s</text>`+"\n", px(xs[last])+8, py(vals[last])+4, chartMuted, name)
			lx := left + 8
			ly := top + 16*i
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="10" height="10" rx="2" fill="%s"/><text x="%d" y="%d" fill="%s">%s</text>`+"\n", lx, ly-9, colors[name], lx+16, ly, chartText, name)
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}

func fmtNum(v float64) string {
	switch {
	case v >= 100:
		return fmt.Sprintf("%.0f", v)
	case v >= 10:
		return fmt.Sprintf("%.1f", v)
	default:
		return fmt.Sprintf("%.2f", v)
	}
}
