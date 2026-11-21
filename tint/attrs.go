package tint

import (
	"log/slog"
	"slices"

	"github.com/Piechutowski/volt/assert"
)

type attrFrame struct {
	attr        slog.Attr
	groupPrefix string
	groups      []string
	color       colorOptional
	depth       int
}

func (handler textHandler) attrFields(attrs []slog.Attr, groupPrefix string, groups []string) []string {
	fields := make([]string, 0, len(attrs))
	stack := attrFramesReversed(attrs, attrFrame{groupPrefix: groupPrefix, groups: groups})
	for step := 0; step < attrCountMax && len(stack) > 0; step++ {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		attr, color, kept := handler.attrResolved(frame)
		if !kept {
			continue
		}
		if attr.Value.Kind() != slog.KindGroup {
			fields = append(fields, handler.attrText(attr, frame.groupPrefix, color))
			continue
		}
		assert.That(frame.depth < attrDepthMax, "tint: attribute groups nest deeper than attrDepthMax")
		stack = append(stack, attrFramesReversed(attr.Value.Group(), frame.child(attr.Key, color))...)
	}
	assert.That(len(stack) == 0, "tint: a record holds more attributes than attrCountMax")
	return fields
}

func attrFramesReversed(attrs []slog.Attr, parent attrFrame) []attrFrame {
	frames := make([]attrFrame, 0, len(attrs))
	for index := len(attrs) - 1; index >= 0; index-- {
		frame := parent
		frame.attr = attrs[index]
		frames = append(frames, frame)
	}
	return frames
}

func (frame attrFrame) child(key string, color colorOptional) attrFrame {
	child := attrFrame{groupPrefix: frame.groupPrefix, groups: frame.groups, color: color, depth: frame.depth + 1}
	if key != "" {
		child.groupPrefix = frame.groupPrefix + key + "."
		child.groups = append(slices.Clone(frame.groups), key)
	}
	return child
}

func (handler textHandler) attrResolved(frame attrFrame) (slog.Attr, colorOptional, bool) {
	attr := frame.attr
	value, color := handler.resolved(attr.Value, frame.color)
	attr.Value = value
	if replace := handler.options.ReplaceAttr; replace != nil && attr.Value.Kind() != slog.KindGroup {
		attr = replace(frame.groups, attr)
		attr.Value, color = handler.resolved(attr.Value, color)
	}
	return attr, color, !attr.Equal(slog.Attr{})
}

func (handler textHandler) attrText(attr slog.Attr, groupPrefix string, color colorOptional) string {
	key := stringText(groupPrefix+attr.Key, true, !handler.options.NoColor) + "="
	value := handler.valueText(attr.Value, true)
	if handler.options.NoColor {
		return key + value
	}
	if color.valid {
		return ansiCode(color.code, true) + key + ansiFaintReset + value + ansiReset
	}
	return ansiFaint + key + ansiReset + value
}
