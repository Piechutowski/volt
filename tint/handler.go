package tint

import (
	"context"
	"io"
	"log/slog"
	"os"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	Black uint8 = iota
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
	BlackBright
	RedBright
	GreenBright
	YellowBright
	BlueBright
	MagentaBright
	CyanBright
	WhiteBright
)

const (
	levelDefault      = slog.LevelInfo
	timeFormatDefault = time.StampMilli

	errorKey = "err"

	// TODO: Limit is to be verified
	attrDepthMax = 64
	// TODO: Limit is to be verified
	attrCountMax = 1 << 16
)

type Options struct {
	AddSource   bool
	Level       slog.Leveler
	ReplaceAttr func(groups []string, attr slog.Attr) slog.Attr
	TimeFormat  string
	NoColor     bool
}

func (options Options) defaulted() Options {
	if options.Level == nil {
		options.Level = levelDefault
	}
	if options.TimeFormat == "" {
		options.TimeFormat = timeFormatDefault
	}
	return options
}

func NewTextHandler(writer io.Writer, options Options) slog.Handler {
	options = options.defaulted()
	if !isColorable(writer) {
		options.NoColor = true
	}
	return textHandler{mutex: &sync.Mutex{}, writer: writer, options: options}
}

func isColorable(writer io.Writer) bool {
	file, ok := writer.(*os.File)
	if !ok {
		return true
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func Err(err error) slog.Attr {
	return Attr(RedBright, slog.Any(errorKey, err))
}

func Attr(color uint8, attr slog.Attr) slog.Attr {
	attr.Value = slog.AnyValue(tintValue{attr.Value, color})
	return attr
}

type tintValue struct {
	slog.Value
	Color uint8
}

func (value tintValue) LogValue() slog.Value {
	return value.Value
}

type colorOptional struct {
	code  uint8
	valid bool
}

type textHandler struct {
	attrsPrefix []string
	groupPrefix string
	groups      []string

	mutex  *sync.Mutex
	writer io.Writer

	options Options
}

func (handler textHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= handler.options.Level.Level()
}

func (handler textHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(attr slog.Attr) bool {
		attrs = append(attrs, attr)
		return true
	})
	recordFields := handler.attrFields(attrs, handler.groupPrefix, handler.groups)
	fields := slices.Concat(handler.headerFields(record), handler.attrsPrefix, recordFields)
	line := strings.Join(fields, " ") + "\n"

	handler.mutex.Lock()
	defer handler.mutex.Unlock()
	_, err := io.WriteString(handler.writer, line)
	return err
}

func (handler textHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return handler
	}
	fields := handler.attrFields(attrs, handler.groupPrefix, handler.groups)
	derived := handler
	derived.attrsPrefix = slices.Concat(handler.attrsPrefix, fields)
	return derived
}

func (handler textHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return handler
	}
	derived := handler
	derived.groupPrefix = handler.groupPrefix + name + "."
	derived.groups = append(slices.Clone(handler.groups), name)
	return derived
}

func (handler textHandler) headerFields(record slog.Record) []string {
	fields := make([]string, 0, 4)
	if !record.Time.IsZero() {
		if field, kept := handler.timeField(record.Time); kept {
			fields = append(fields, field)
		}
	}
	if field, kept := handler.levelField(record.Level); kept {
		fields = append(fields, field)
	}
	if handler.options.AddSource {
		if field, kept := handler.sourceField(record.PC); kept {
			fields = append(fields, field)
		}
	}
	if field, kept := handler.messageField(record.Message); kept {
		fields = append(fields, field)
	}
	return fields
}

func (handler textHandler) timeField(moment time.Time) (string, bool) {
	replace := handler.options.ReplaceAttr
	if replace == nil {
		return handler.timeText(moment, colorOptional{}), true
	}
	attr := replace(nil, slog.Time(slog.TimeKey, moment.Round(0)))
	if attr.Key == "" {
		return "", false
	}
	value, color := handler.resolved(attr.Value, colorOptional{})
	if value.Kind() == slog.KindTime {
		return handler.timeText(value.Time(), color), true
	}
	return handler.valueTinted(value, color, true), true
}

func (handler textHandler) levelField(level slog.Level) (string, bool) {
	replace := handler.options.ReplaceAttr
	if replace == nil {
		return handler.levelText(level, colorOptional{}), true
	}
	attr := replace(nil, slog.Any(slog.LevelKey, level))
	if attr.Key == "" {
		return "", false
	}
	value, color := handler.resolved(attr.Value, colorOptional{})
	if levelReplaced, ok := value.Any().(slog.Level); ok && value.Kind() == slog.KindAny {
		return handler.levelText(levelReplaced, color), true
	}
	return handler.valueTinted(value, color, false), true
}

func (handler textHandler) sourceField(counter uintptr) (string, bool) {
	frame, _ := runtime.CallersFrames([]uintptr{counter}).Next()
	if frame.File == "" {
		return "", false
	}
	source := &slog.Source{Function: frame.Function, File: frame.File, Line: frame.Line}
	replace := handler.options.ReplaceAttr
	if replace == nil {
		if handler.options.NoColor {
			return sourceText(source), true
		}
		return ansiFaint + sourceText(source) + ansiReset, true
	}
	attr := replace(nil, slog.Any(slog.SourceKey, source))
	if attr.Key == "" {
		return "", false
	}
	value, color := handler.resolved(attr.Value, colorOptional{})
	return handler.valueTinted(value, color, true), true
}

func (handler textHandler) messageField(message string) (string, bool) {
	replace := handler.options.ReplaceAttr
	if replace == nil {
		return message, true
	}
	attr := replace(nil, slog.String(slog.MessageKey, message))
	if attr.Key == "" {
		return "", false
	}
	value, color := handler.resolved(attr.Value, colorOptional{})
	return handler.valueTinted(value, color, false), true
}

func (handler textHandler) resolved(value slog.Value, inherited colorOptional) (slog.Value, colorOptional) {
	if !handler.options.NoColor && value.Kind() == slog.KindLogValuer {
		if tinted, ok := value.Any().(tintValue); ok {
			return tinted.Value.Resolve(), colorOptional{code: tinted.Color, valid: true}
		}
	}
	return value.Resolve(), inherited
}
