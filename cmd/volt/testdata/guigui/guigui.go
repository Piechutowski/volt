package guigui

import "image"

type uniqueKey struct {
	v int64
}

type EnvKey uniqueKey

func GenerateEnvKey() EnvKey {
	return EnvKey(uniqueKey{1})
}

type EnvSource struct {
	Origin Widget
	Child  Widget
}

type Context struct{}

func (context *Context) Env(widget Widget, key EnvKey) (any, bool) {
	return nil, false
}

type ChildAdder struct{}

func (adder *ChildAdder) AddWidget(widget Widget) {}

type ChildLayouter struct{}

func (layouter *ChildLayouter) LayoutWidget(widget Widget, bounds image.Rectangle) {}

type WidgetBounds struct {
	bounds image.Rectangle
}

func (widgetBounds *WidgetBounds) Bounds() image.Rectangle {
	return widgetBounds.bounds
}

type StateKeyWriter struct{}

func (writer *StateKeyWriter) WriteInt(value int) {}

type Widget interface {
	Env(context *Context, key EnvKey, source *EnvSource) (any, bool)
	Build(context *Context, adder *ChildAdder) error
	Layout(context *Context, widgetBounds *WidgetBounds, layouter *ChildLayouter)
	WriteStateKey(context *Context, writer *StateKeyWriter)
	widgetState() *widgetState
}

type widgetState struct {
	bounds image.Rectangle
}

type DefaultWidget struct {
	state widgetState
}

func (*DefaultWidget) Env(context *Context, key EnvKey, source *EnvSource) (any, bool) {
	return nil, false
}

func (*DefaultWidget) Build(context *Context, adder *ChildAdder) error {
	return nil
}

func (*DefaultWidget) Layout(context *Context, widgetBounds *WidgetBounds, layouter *ChildLayouter) {}

func (*DefaultWidget) WriteStateKey(context *Context, writer *StateKeyWriter) {}

func (widget *DefaultWidget) widgetState() *widgetState {
	return &widget.state
}
