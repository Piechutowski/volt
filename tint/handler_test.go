package tint

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/Piechutowski/volt/assert"
)

var momentPublished = time.Date(2026, 11, 19, 18, 20, 47, 123456789, time.UTC)

type slug struct{ text string }

func (value *slug) MarshalText() ([]byte, error) {
	if value.text == "" {
		return nil, errors.New("empty slug")
	}
	return []byte(value.text), nil
}

func TestHandlerPassesSlogtest(t *testing.T) {
	var output bytes.Buffer
	handlerNew := func(t *testing.T) slog.Handler {
		output.Reset()
		return NewTextHandler(&output, Options{NoColor: true, TimeFormat: time.RFC3339Nano})
	}
	result := func(t *testing.T) map[string]any {
		return slogtestLineParsed(t, strings.TrimSuffix(output.String(), "\n"))
	}
	slogtest.Run(t, handlerNew, result)
}

func slogtestLineParsed(t *testing.T, line string) map[string]any {
	tokens := strings.Split(line, " ")
	parsed := map[string]any{}
	if _, err := time.Parse(time.RFC3339Nano, tokens[0]); err == nil {
		parsed[slog.TimeKey] = tokens[0]
		tokens = tokens[1:]
	}
	if len(tokens) < 2 {
		t.Fatalf("line %q holds no level and message", line)
	}
	parsed[slog.LevelKey] = tokens[0]
	parsed[slog.MessageKey] = tokens[1]
	for _, token := range tokens[2:] {
		key, value, found := strings.Cut(token, "=")
		if !found {
			t.Fatalf("token %q of line %q is not key=value", token, line)
		}
		slogtestAttrPlaced(t, parsed, strings.Split(key, "."), slogtestValueUnquoted(value))
	}
	return parsed
}

func slogtestAttrPlaced(t *testing.T, parsed map[string]any, path []string, value string) {
	group := parsed
	for _, name := range path[:len(path)-1] {
		child, ok := group[name].(map[string]any)
		if !ok {
			child = map[string]any{}
			group[name] = child
		}
		group = child
	}
	group[path[len(path)-1]] = value
}

func slogtestValueUnquoted(value string) string {
	unquoted, err := strconv.Unquote(value)
	if err != nil {
		return value
	}
	return unquoted
}

func handlerLine(t *testing.T, options Options, record slog.Record) string {
	t.Helper()
	var output bytes.Buffer
	handler := NewTextHandler(&output, options)
	if err := handler.Handle(context.Background(), record); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	return output.String()
}

func recordPublished(level slog.Level, message string, args ...any) slog.Record {
	record := slog.NewRecord(momentPublished, level, message, 0)
	record.Add(args...)
	return record
}

func TestHandlerWritesFields(t *testing.T) {
	cases := []struct {
		name   string
		record slog.Record
		want   string
	}{
		{"plain", recordPublished(slog.LevelInfo, "post published", "slug", "hello-world", "views", 42),
			"Nov 19 18:20:47.123 INF post published slug=hello-world views=42\n"},
		{"debug below", recordPublished(slog.LevelDebug-2, "cache miss"), "Nov 19 18:20:47.123 DBG-2 cache miss\n"},
		{"warn above", recordPublished(slog.LevelWarn+1, "slow query"), "Nov 19 18:20:47.123 WRN+1 slow query\n"},
		{"error", recordPublished(slog.LevelError, "write failed"), "Nov 19 18:20:47.123 ERR write failed\n"},
		{"empty message", recordPublished(slog.LevelInfo, ""), "Nov 19 18:20:47.123 INF \n"},
		{"quoted", recordPublished(slog.LevelInfo, "post", "title", "Hello world", "tag", "", "body", `a"b`),
			`Nov 19 18:20:47.123 INF post title="Hello world" tag="" body="a\"b"` + "\n"},
		{"group", recordPublished(slog.LevelInfo, "request", slog.Group("timing", "total", 5*time.Millisecond)),
			"Nov 19 18:20:47.123 INF request timing.total=5ms\n"},
		{"inline group", recordPublished(slog.LevelInfo, "request", slog.Group("", "views", 7)),
			"Nov 19 18:20:47.123 INF request views=7\n"},
		{"time value", recordPublished(slog.LevelInfo, "post", "at", momentPublished),
			"Nov 19 18:20:47.123 INF post at=2026-11-19T18:20:47.123Z\n"},
		{"bytes", recordPublished(slog.LevelInfo, "post", "digest", []byte("abc")),
			`Nov 19 18:20:47.123 INF post digest="abc"` + "\n"},
		{"escapes removed", recordPublished(slog.LevelInfo, "post", "title", "\u001b[31mred\u001b[0m title"),
			`Nov 19 18:20:47.123 INF post title="red title"` + "\n"},
		{"invalid utf-8 kept", recordPublished(slog.LevelInfo, "post", "title", "a\xffb", "tag", "c�d"),
			"Nov 19 18:20:47.123 INF post title=\"a�b\" tag=\"c�d\"\n"},
		{"text marshaler", recordPublished(slog.LevelInfo, "post", "slug", &slug{"hello-world"}),
			"Nov 19 18:20:47.123 INF post slug=hello-world\n"},
		{"nil text marshaler", recordPublished(slog.LevelInfo, "post", "slug", (*slug)(nil)),
			"Nov 19 18:20:47.123 INF post slug=<nil>\n"},
		{"failing text marshaler", recordPublished(slog.LevelInfo, "post", "slug", &slug{}),
			`Nov 19 18:20:47.123 INF post slug="!ERROR:empty slug"` + "\n"},
		{"zero time", slog.NewRecord(time.Time{}, slog.LevelInfo, "boot", 0), "INF boot\n"},
	}
	for _, testCase := range cases {
		if got := handlerLine(t, Options{NoColor: true}, testCase.record); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestHandlerWritesColors(t *testing.T) {
	const (
		timeFaint = "\u001b[2mNov 19 18:20:47.123\u001b[0m "
		infoGreen = "\u001b[92mINF\u001b[0m "
	)
	cases := []struct {
		name   string
		record slog.Record
		want   string
	}{
		{"info", recordPublished(slog.LevelInfo, "post published", "slug", "hello-world"),
			timeFaint + infoGreen + "post published \u001b[2mslug=\u001b[0mhello-world\n"},
		{"debug", recordPublished(slog.LevelDebug, "cache miss"), timeFaint + "DBG cache miss\n"},
		{"warn", recordPublished(slog.LevelWarn, "slow"), timeFaint + "\u001b[93mWRN\u001b[0m slow\n"},
		{"error", recordPublished(slog.LevelError, "failed", Err(errors.New("disk full"))),
			timeFaint + "\u001b[91mERR\u001b[0m failed \u001b[2;91merr=\u001b[22m\"disk full\"\u001b[0m\n"},
		{"tinted group", recordPublished(slog.LevelInfo, "request", Attr(Yellow, slog.Group("timing", "total", 5))),
			timeFaint + infoGreen + "request \u001b[2;33mtiming.total=\u001b[22m5\u001b[0m\n"},
		{"cube color", recordPublished(slog.LevelInfo, "metric", Attr(200, slog.Int("cpu", 93))),
			timeFaint + infoGreen + "metric \u001b[2;38;5;200mcpu=\u001b[22m93\u001b[0m\n"},
	}
	for _, testCase := range cases {
		if got := handlerLine(t, Options{}, testCase.record); got != testCase.want {
			t.Errorf("%s: got %q, want %q", testCase.name, got, testCase.want)
		}
	}
}

func TestHandlerDerivesAttrsAndGroups(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(NewTextHandler(&output, Options{NoColor: true}))
	logger.With("blog", "metrics").WithGroup("request").With("method", "GET").Info("served", "status", 200)
	logger.WithGroup("request").WithGroup("").Info("served")
	got := output.String()
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), got)
	}
	if !strings.HasSuffix(lines[0], " INF served blog=metrics request.method=GET request.status=200") {
		t.Errorf("derived line: got %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], " INF served") {
		t.Errorf("empty group line: got %q", lines[1])
	}
}

func TestHandlerReplacesAttrs(t *testing.T) {
	replace := func(groups []string, attr slog.Attr) slog.Attr {
		switch {
		case attr.Key == slog.TimeKey && len(groups) == 0:
			return slog.Attr{}
		case attr.Key == slog.LevelKey && len(groups) == 0:
			return Attr(MagentaBright, slog.String(attr.Key, "TRC"))
		case attr.Key == "secret":
			return slog.Attr{}
		case attr.Key == "views" && len(groups) == 1 && groups[0] == "post":
			return slog.Int64("views", attr.Value.Int64()+1)
		}
		return attr
	}
	record := recordPublished(slog.LevelInfo, "post", "secret", "x", slog.Group("post", "views", 41))
	got := handlerLine(t, Options{NoColor: true, ReplaceAttr: replace}, record)
	if want := "TRC post post.views=42\n"; got != want {
		t.Errorf("no color: got %q, want %q", got, want)
	}
	got = handlerLine(t, Options{ReplaceAttr: replace}, record)
	if want := "\u001b[95mTRC\u001b[0m post \u001b[2mpost.views=\u001b[0m42\n"; got != want {
		t.Errorf("color: got %q, want %q", got, want)
	}
}

func TestHandlerAddsSource(t *testing.T) {
	var output bytes.Buffer
	slog.New(NewTextHandler(&output, Options{NoColor: true, AddSource: true})).Info("post published")
	if got := output.String(); !strings.Contains(got, " tint/handler_test.go:") {
		t.Errorf("got %q, want the source tint/handler_test.go", got)
	}
}

func TestHandlerColorsOnlyATerminal(t *testing.T) {
	file, err := os.Create(filepath.Join(t.TempDir(), "blog.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	slog.New(NewTextHandler(file, Options{})).Info("post published", Err(errors.New("disk full")))
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(file.Name())
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(string(written), ansiEscape) {
		t.Errorf("a file that is not a terminal got colors: %q", written)
	}
	if !strings.HasSuffix(string(written), ` INF post published err="disk full"`+"\n") {
		t.Errorf("file line: got %q", written)
	}
}

func TestHandlerDefaultLevelIsInfo(t *testing.T) {
	if !NewTextHandler(&bytes.Buffer{}, Options{}).Enabled(context.Background(), slog.LevelInfo) {
		t.Error("the default level does not enable info")
	}
	if NewTextHandler(&bytes.Buffer{}, Options{}).Enabled(context.Background(), slog.LevelDebug) {
		t.Error("the default level enables debug")
	}
}

func attrNested(depth int) slog.Attr {
	attr := slog.Int("views", 1)
	for level := 0; level < depth; level++ {
		attr = slog.Group("g", attr)
	}
	return attr
}

func handleFailed(handler slog.Handler, record slog.Record) (failed bool) {
	defer func() {
		failed = assert.IsFailure(recover())
	}()
	_ = handler.Handle(context.Background(), record)
	return false
}

func withAttrsFailed(handler slog.Handler, attrs []slog.Attr) (failed bool) {
	defer func() {
		failed = assert.IsFailure(recover())
	}()
	handler.WithAttrs(attrs)
	return false
}

func TestHandlerAssertsGroupDepth(t *testing.T) {
	handler := NewTextHandler(&bytes.Buffer{}, Options{NoColor: true})
	deepest := recordPublished(slog.LevelInfo, "post")
	deepest.AddAttrs(attrNested(attrDepthMax))
	if handleFailed(handler, deepest) {
		t.Errorf("groups %d deep: the assertion failed", attrDepthMax)
	}
	deeper := recordPublished(slog.LevelInfo, "post")
	deeper.AddAttrs(attrNested(attrDepthMax + 1))
	if !handleFailed(handler, deeper) {
		t.Errorf("groups %d deep: no assertion failed", attrDepthMax+1)
	}
	if !withAttrsFailed(handler, []slog.Attr{attrNested(attrDepthMax + 1)}) {
		t.Errorf("WithAttrs of groups %d deep: no assertion failed", attrDepthMax+1)
	}
}

func TestHandlerAssertsAttrCount(t *testing.T) {
	handler := NewTextHandler(&bytes.Buffer{}, Options{NoColor: true})
	attrs := make([]slog.Attr, 0, attrCountMax+1)
	for index := 0; index < attrCountMax; index++ {
		attrs = append(attrs, slog.Int("views", index))
	}
	most := recordPublished(slog.LevelInfo, "metrics")
	most.AddAttrs(attrs...)
	if handleFailed(handler, most) {
		t.Errorf("%d attributes: the assertion failed", attrCountMax)
	}
	more := recordPublished(slog.LevelInfo, "metrics")
	more.AddAttrs(append(attrs, slog.Int("views", attrCountMax))...)
	if !handleFailed(handler, more) {
		t.Errorf("%d attributes: no assertion failed", attrCountMax+1)
	}
}
