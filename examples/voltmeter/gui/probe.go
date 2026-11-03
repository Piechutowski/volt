package main

import (
	"bytes"
	"encoding/json"
	"image"
	"io"
	"maps"
	"strconv"

	"github.com/hajimehoshi/ebiten/v2"
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
	Notice  string            `json:"notice"`
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
		Notice:  s.Notice.Text,
		Busy:    m.inflight > 0 || len(m.results) > 0 || len(m.events) > 0,
		Targets: maps.Clone(p.targets),
	}
	line.Window[0], line.Window[1] = ebiten.WindowPosition()
	for f, v := range s.Fields {
		line.Fields[string(f)] = v
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
