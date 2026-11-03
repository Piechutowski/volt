package main

import (
	"bytes"
	"encoding/json"
	"image"
	"io"
	"maps"
	"strconv"

	"github.com/guigui-gui/guigui"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/Piechutowski/volt/examples/voltmeter/db"
)

type probe struct {
	out     io.Writer
	last    []byte
	targets map[string][4]int
}

type probeLine struct {
	Screen  string            `json:"screen"`
	Run     int               `json:"run"`
	Loaded  bool              `json:"loaded"`
	Latency int               `json:"latency"`
	Fields  map[string]string `json:"fields"`
	Errors  map[string]string `json:"errors"`
	Notice  string            `json:"notice"`
	Board   int               `json:"board"`
	Widgets int               `json:"widgets"`
	Budget  int               `json:"budget"`
	Breach  int               `json:"breached"`
	Trend   string            `json:"trend"`
	Points  int               `json:"points"`
	Fixture string            `json:"fixture"`
	Counts  int               `json:"counts"`
	Busy    bool              `json:"busy"`
	Window  [2]int            `json:"window"`
	Targets map[string][4]int `json:"targets"`
}

func (p *probe) place(name string, r image.Rectangle) {
	if p == nil || r.Empty() {
		return
	}
	if p.targets == nil {
		p.targets = map[string][4]int{}
	}
	p.targets[name] = [4]int{r.Min.X, r.Min.Y, r.Max.X, r.Max.Y}
}

func (p *probe) emit(m *model) {
	if p == nil {
		return
	}
	s := m.s
	line := probeLine{
		Screen:  s.Screen.String(),
		Run:     s.Run.ID,
		Loaded:  s.Run.Loaded,
		Latency: s.Run.Detail.Latency.N,
		Fields:  map[string]string{},
		Errors:  map[string]string{},
		Notice:  s.Notice.Text,
		Board:   s.Boards.Selected,
		Budget:  s.Budgets.Selected,
		Trend:   string(s.Trend.Phase) + "@" + strconv.Itoa(s.Trend.Tables),
		Points:  len(s.Trends[s.Trend]),
		Fixture: s.Thesis.Fixture,
		Counts:  len(s.Thesis.Rows),
		Busy:    m.inflight > 0 || len(m.results) > 0 || len(m.events) > 0,
		Targets: maps.Clone(p.targets),
	}
	if b, ok := s.BoardSelected(); ok {
		line.Widgets = len(b.Widgets)
	}
	for _, b := range s.Budgets.List {
		if b.State == db.HealthBreached {
			line.Breach++
		}
	}
	line.Window[0], line.Window[1] = ebiten.WindowPosition()
	for f, v := range s.Fields {
		line.Fields[string(f)] = v
	}
	for f, v := range s.Errors {
		line.Errors[string(f)] = v
	}
	b, err := json.Marshal(line)
	if err != nil || bytes.Equal(b, p.last) {
		return
	}
	p.last = b
	p.out.Write(append(b, '\n'))
	p.targets = nil
}

func probeRow(prefix string, id int) string { return prefix + ":" + strconv.Itoa(id) }

type spots struct {
	at    map[string]image.Rectangle
	names map[guigui.Widget]string
}

type spotLayouter struct {
	inner guigui.WidgetLayouter
	s     *spots
}

func (l spotLayouter) LayoutWidget(w guigui.Widget, r image.Rectangle) {
	if name, ok := l.s.names[w]; ok {
		l.s.at[name] = r
	}
	l.inner.LayoutWidget(w, r)
}

func (s *spots) recorded(m *model, inner guigui.WidgetLayouter, named func(name func(guigui.Widget, string))) guigui.WidgetLayouter {
	if m == nil || m.probe == nil {
		return inner
	}
	if s.at == nil {
		s.at, s.names = map[string]image.Rectangle{}, map[guigui.Widget]string{}
	}
	clear(s.at)
	clear(s.names)
	named(func(w guigui.Widget, name string) { s.names[w] = name })
	return spotLayouter{inner: inner, s: s}
}

func (s *spots) placed(m *model) {
	if m == nil || m.probe == nil {
		return
	}
	for name, r := range s.at {
		m.probe.place(name, r)
	}
}

func (s *spots) segmentsPlaced(m *model, name string, labels []string) {
	r, ok := s.at[name]
	if m == nil || m.probe == nil || !ok || len(labels) == 0 {
		return
	}
	w := r.Dx() / len(labels)
	for i, l := range labels {
		seg := r
		seg.Min.X += i * w
		seg.Max.X = seg.Min.X + w
		m.probe.place(name+":"+l, seg)
	}
}
