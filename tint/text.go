package tint

import (
	"encoding"
	"fmt"
	"log/slog"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	ansiEscape       = '\u001b'
	ansiReset        = "\u001b[0m"
	ansiFaint        = "\u001b[2m"
	ansiFaintReset   = "\u001b[22m"
	ansiRedBright    = "\u001b[91m"
	ansiGreenBright  = "\u001b[92m"
	ansiYellowBright = "\u001b[93m"
)

func (handler textHandler) timeText(moment time.Time, color colorOptional) string {
	text := moment.Format(handler.options.TimeFormat)
	if handler.options.NoColor {
		return text
	}
	if color.valid {
		return ansiCode(color.code, true) + text + ansiReset
	}
	return ansiFaint + text + ansiReset
}

func (handler textHandler) levelText(level slog.Level, color colorOptional) string {
	text := levelName(level)
	if handler.options.NoColor {
		return text
	}
	if color.valid {
		return ansiCode(color.code, false) + text + ansiReset
	}
	switch {
	case level < slog.LevelInfo:
		return text
	case level < slog.LevelWarn:
		return ansiGreenBright + text + ansiReset
	case level < slog.LevelError:
		return ansiYellowBright + text + ansiReset
	}
	return ansiRedBright + text + ansiReset
}

func levelName(level slog.Level) string {
	switch {
	case level < slog.LevelInfo:
		return levelNameOffset("DBG", level-slog.LevelDebug)
	case level < slog.LevelWarn:
		return levelNameOffset("INF", level-slog.LevelInfo)
	case level < slog.LevelError:
		return levelNameOffset("WRN", level-slog.LevelWarn)
	}
	return levelNameOffset("ERR", level-slog.LevelError)
}

func levelNameOffset(base string, offset slog.Level) string {
	if offset == 0 {
		return base
	}
	if offset > 0 {
		return base + "+" + strconv.Itoa(int(offset))
	}
	return base + strconv.Itoa(int(offset))
}

func sourceText(source *slog.Source) string {
	directory, file := filepath.Split(source.File)
	return filepath.Join(filepath.Base(directory), file) + ":" + strconv.Itoa(source.Line)
}

func (handler textHandler) valueTinted(value slog.Value, color colorOptional, faint bool) string {
	text := handler.valueText(value, false)
	if handler.options.NoColor {
		return text
	}
	if color.valid {
		return ansiCode(color.code, faint) + text + ansiReset
	}
	if faint {
		return ansiFaint + text + ansiReset
	}
	return text
}

func (handler textHandler) valueText(value slog.Value, quote bool) string {
	color := !handler.options.NoColor
	switch value.Kind() {
	case slog.KindString:
		return stringText(value.String(), quote, color)
	case slog.KindInt64:
		return strconv.FormatInt(value.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(value.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(value.Float64(), 'g', -1, 64)
	case slog.KindBool:
		return strconv.FormatBool(value.Bool())
	case slog.KindDuration:
		return stringText(value.Duration().String(), quote, color)
	case slog.KindTime:
		return timeRFC3339Millis(value.Time())
	case slog.KindAny:
		return anyText(value.Any(), quote, color)
	}
	return ""
}

func anyText(value any, quote bool, color bool) string {
	switch typed := value.(type) {
	case encoding.TextMarshaler:
		if isNilPointer(typed) {
			return "<nil>"
		}
		data, err := typed.MarshalText()
		if err != nil {
			return stringText("!ERROR:"+err.Error(), true, color)
		}
		return stringText(string(data), quote, color)
	case *slog.Source:
		if typed == nil {
			return "<nil>"
		}
		return sourceText(typed)
	}
	if data, ok := byteSliceOf(value); ok {
		if quote {
			return strconv.Quote(string(data))
		}
		return string(data)
	}
	return stringText(fmt.Sprintf("%+v", value), quote, color)
}

func isNilPointer(value any) bool {
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}

func byteSliceOf(value any) ([]byte, bool) {
	if data, ok := value.([]byte); ok {
		return data, true
	}
	reflected := reflect.TypeOf(value)
	if reflected != nil && reflected.Kind() == reflect.Slice && reflected.Elem().Kind() == reflect.Uint8 {
		return reflect.ValueOf(value).Bytes(), true
	}
	return nil, false
}

func timeRFC3339Millis(moment time.Time) string {
	const prefixLength = len("2006-01-02T15:04:05.000")
	text := moment.Truncate(time.Millisecond).Add(time.Millisecond / 10).Format(time.RFC3339Nano)
	return text[:prefixLength] + text[prefixLength+1:]
}

func ansiCode(code uint8, faint bool) string {
	prefix := "\u001b["
	if faint {
		prefix += "2;"
	}
	switch {
	case code < 8:
		return prefix + strconv.Itoa(int(code)+30) + "m"
	case code < 16:
		return prefix + strconv.Itoa(int(code)+82) + "m"
	}
	return prefix + "38;5;" + strconv.Itoa(int(code)) + "m"
}

func stringText(text string, quote bool, color bool) string {
	visible := text
	if quote && !color {
		visible = escapesRemoved(text)
	}
	if !quote || !needsQuoting(visible) {
		return visible
	}
	if !color {
		return strconv.Quote(visible)
	}
	return strings.ReplaceAll(strconv.Quote(visible), `\x1b`, string(ansiEscape))
}

func escapesRemoved(text string) string {
	var builder strings.Builder
	escaping := false
	for _, character := range text {
		if character == ansiEscape {
			escaping = true
			continue
		}
		if escaping {
			escaping = !unicode.IsLetter(character)
			continue
		}
		builder.WriteRune(character)
	}
	return builder.String()
}

func needsQuoting(text string) bool {
	if text == "" {
		return true
	}
	for _, character := range text {
		if character < utf8.RuneSelf {
			if asciiNeedsQuoting(character) {
				return true
			}
			continue
		}
		if character == utf8.RuneError || unicode.IsSpace(character) || !unicode.IsPrint(character) {
			return true
		}
	}
	return false
}

func asciiNeedsQuoting(character rune) bool {
	switch character {
	case ' ', '=', '"':
		return true
	case ansiEscape, '\u007f':
		return false
	}
	return character < ' '
}
